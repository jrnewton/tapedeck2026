package scheduler

import (
	"log"
	"math"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"local/tapedeck/internal/db"
	"local/tapedeck/pkg/tapedeck"
)

// Config holds scheduler configuration.
type Config struct {
	MaxConcurrent int           // Maximum concurrent downloads (default: 2)
	MaxRetries    int           // Maximum retry attempts (default: 5)
	RetryBackoff  float64       // Retry backoff multiplier (default: 1.3)
	BaseDelay     time.Duration // Base delay for retries (default: 1 minute)
	TickInterval  time.Duration // How often to check for due schedules (default: 1 minute)
}

// DefaultConfig returns the default scheduler configuration.
func DefaultConfig() Config {
	return Config{
		MaxConcurrent: 2,
		MaxRetries:    5,
		RetryBackoff:  1.3,
		BaseDelay:     1 * time.Minute,
		TickInterval:  1 * time.Minute,
	}
}

// Scheduler manages scheduled download jobs.
type Scheduler struct {
	db           *db.DB
	downloadsDir string
	config       Config
	parser       cron.Parser
	semaphore    chan struct{}
	ticker       *time.Ticker
	done         chan bool
	wg           sync.WaitGroup
	mu           sync.Mutex
	running      bool
}

// New creates a new scheduler.
func New(database *db.DB, downloadsDir string, cfg Config) *Scheduler {
	// Set defaults for zero values
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 2
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 5
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = 1.3
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 1 * time.Minute
	}
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = 1 * time.Minute
	}

	return &Scheduler{
		db:           database,
		downloadsDir: downloadsDir,
		config:       cfg,
		parser:       cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		semaphore:    make(chan struct{}, cfg.MaxConcurrent),
		done:         make(chan bool),
	}
}

// Start begins the scheduler loop.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.ticker = time.NewTicker(s.config.TickInterval)
	s.mu.Unlock()

	log.Printf("Scheduler started (tick interval: %v, max concurrent: %d)", s.config.TickInterval, s.config.MaxConcurrent)

	go func() {
		// Run immediately on start
		s.runDue()

		for {
			select {
			case <-s.done:
				return
			case <-s.ticker.C:
				s.runDue()
			}
		}
	}()
}

// Stop stops the scheduler and waits for running jobs to complete.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.ticker.Stop()
	s.mu.Unlock()

	close(s.done)
	s.wg.Wait()
	log.Println("Scheduler stopped")
}

// runDue checks for and executes due schedules.
func (s *Scheduler) runDue() {
	now := time.Now()

	// Check for retries first
	retries, err := s.db.ListRetrySchedules(now)
	if err != nil {
		log.Printf("Scheduler: error listing retry schedules: %v", err)
	} else {
		for _, sched := range retries {
			s.executeSchedule(sched, true)
		}
	}

	// Check for regular due schedules
	due, err := s.db.ListDueSchedules(now)
	if err != nil {
		log.Printf("Scheduler: error listing due schedules: %v", err)
		return
	}

	for _, sched := range due {
		s.executeSchedule(sched, false)
	}
}

// executeSchedule runs a single schedule in a goroutine.
func (s *Scheduler) executeSchedule(sched db.Schedule, isRetry bool) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Acquire semaphore
		s.semaphore <- struct{}{}
		defer func() { <-s.semaphore }()

		s.runScheduledDownload(sched, isRetry)
	}()
}

// runScheduledDownload performs the actual download for a schedule.
func (s *Scheduler) runScheduledDownload(sched db.Schedule, isRetry bool) {
	log.Printf("Scheduler: running %s - %s (ID: %d, retry: %v)", sched.Station, sched.Show, sched.ID, isRetry)

	// Get adapter
	adapter, err := tapedeck.GetAdapter(sched.Station)
	if err != nil {
		s.handleFailure(sched, err, isRetry)
		return
	}

	// Get latest archive from adapter
	archive, err := adapter.GetLatestArchive(sched.Show)
	if err != nil {
		s.handleFailure(sched, err, isRetry)
		return
	}

	// Check if already downloaded
	existing, err := s.db.FindDownload(sched.StationID, &sched.ShowID, archive.Date)
	if err != nil {
		s.handleFailure(sched, err, isRetry)
		return
	}

	if existing != nil && existing.Status == db.StatusCompleted {
		// Already downloaded, mark as skipped
		log.Printf("Scheduler: skipped %s - %s (already downloaded: %s)", sched.Station, sched.Show, archive.Date.Format("2006-01-02"))
		nextRun := s.calculateNextRun(sched.CronExpression)
		if err := s.db.UpdateScheduleStatus(sched.ID, db.ScheduleStatusSkipped, "", nextRun, nil, 0); err != nil {
			log.Printf("Scheduler: error updating schedule status: %v", err)
		}
		return
	}

	// Create download record if it doesn't exist
	var downloadID int64
	if existing != nil {
		downloadID = existing.ID
	} else {
		downloadID, err = s.db.InsertDownload(&db.Download{
			StationID:   sched.StationID,
			ShowID:      &sched.ShowID,
			ArchiveDate: archive.Date,
			M3UURL:      archive.M3UURL,
			Status:      db.StatusPending,
		})
		if err != nil {
			s.handleFailure(sched, err, isRetry)
			return
		}
	}

	// Update download status to downloading
	if err := s.db.UpdateDownloadStatus(downloadID, db.StatusDownloading, "", ""); err != nil {
		s.handleFailure(sched, err, isRetry)
		return
	}

	// Perform the download
	destPath, err := adapter.DownloadArchive(archive, s.downloadsDir)
	if err != nil {
		// Mark download as failed
		s.db.UpdateDownloadStatus(downloadID, db.StatusFailed, "", err.Error())
		s.handleFailure(sched, err, isRetry)
		return
	}

	// Update download status to completed (store only filename)
	filename := filepath.Base(destPath)
	if err := s.db.UpdateDownloadStatus(downloadID, db.StatusCompleted, filename, ""); err != nil {
		log.Printf("Scheduler: error updating download status: %v", err)
	}

	// Mark schedule as successful
	log.Printf("Scheduler: completed %s - %s (%s)", sched.Station, sched.Show, archive.Date.Format("2006-01-02"))
	nextRun := s.calculateNextRun(sched.CronExpression)
	if err := s.db.UpdateScheduleStatus(sched.ID, db.ScheduleStatusSuccess, "", nextRun, nil, 0); err != nil {
		log.Printf("Scheduler: error updating schedule status: %v", err)
	}
}

// handleFailure handles a failed schedule execution with retry logic.
func (s *Scheduler) handleFailure(sched db.Schedule, err error, isRetry bool) {
	retryCount := sched.RetryCount
	if !isRetry {
		retryCount = 0
	}
	retryCount++

	if retryCount >= s.config.MaxRetries {
		// Max retries reached, mark as failed and schedule next regular run
		log.Printf("Scheduler: failed %s - %s after %d retries: %v", sched.Station, sched.Show, retryCount, err)
		nextRun := s.calculateNextRun(sched.CronExpression)
		if err := s.db.UpdateScheduleStatus(sched.ID, db.ScheduleStatusFailed, err.Error(), nextRun, nil, 0); err != nil {
			log.Printf("Scheduler: error updating schedule status: %v", err)
		}
		return
	}

	// Calculate retry delay with exponential backoff
	delay := s.calculateRetryDelay(retryCount)
	nextRetry := time.Now().Add(delay)

	log.Printf("Scheduler: retry %d/%d for %s - %s in %v: %v", retryCount, s.config.MaxRetries, sched.Station, sched.Show, delay, err)

	// Keep existing next_run_at, set next_retry_at
	if err := s.db.UpdateScheduleStatus(sched.ID, db.ScheduleStatusRetrying, err.Error(), sched.NextRunAt, &nextRetry, retryCount); err != nil {
		log.Printf("Scheduler: error updating schedule status: %v", err)
	}
}

// calculateRetryDelay calculates the delay for a retry attempt.
func (s *Scheduler) calculateRetryDelay(retryCount int) time.Duration {
	// Exponential backoff: baseDelay * (backoff ^ (retryCount - 1))
	multiplier := math.Pow(s.config.RetryBackoff, float64(retryCount-1))
	return time.Duration(float64(s.config.BaseDelay) * multiplier)
}

// calculateNextRun calculates the next run time for a cron expression.
// Cron expressions are in local time (America/New_York via TZ env var).
func (s *Scheduler) calculateNextRun(cronExpr string) *time.Time {
	schedule, err := s.parser.Parse(cronExpr)
	if err != nil {
		log.Printf("Scheduler: error parsing cron expression %q: %v", cronExpr, err)
		return nil
	}
	next := schedule.Next(time.Now())
	return &next
}

// AddSchedule creates a new schedule for a station/show.
func (s *Scheduler) AddSchedule(stationID, showID int64, cronExpr string) (*db.Schedule, error) {
	// Validate cron expression
	_, err := s.parser.Parse(cronExpr)
	if err != nil {
		return nil, err
	}

	// Calculate first run time
	nextRun := s.calculateNextRun(cronExpr)

	sched := &db.Schedule{
		StationID:      stationID,
		ShowID:         showID,
		CronExpression: cronExpr,
		Enabled:        true,
		NextRunAt:      nextRun,
	}

	id, err := s.db.InsertSchedule(sched)
	if err != nil {
		return nil, err
	}

	return s.db.GetSchedule(id)
}

// RemoveSchedule deletes a schedule by ID.
func (s *Scheduler) RemoveSchedule(id int64) error {
	return s.db.DeleteSchedule(id)
}

// ListSchedules returns all schedules.
func (s *Scheduler) ListSchedules() ([]db.Schedule, error) {
	return s.db.ListSchedules()
}

// GetSchedule returns a schedule by ID.
func (s *Scheduler) GetSchedule(id int64) (*db.Schedule, error) {
	return s.db.GetSchedule(id)
}

// SetEnabled enables or disables a schedule.
func (s *Scheduler) SetEnabled(id int64, enabled bool) error {
	return s.db.UpdateScheduleEnabled(id, enabled)
}

// ParseCronExpression validates and parses a cron expression.
func (s *Scheduler) ParseCronExpression(cronExpr string) (cron.Schedule, error) {
	return s.parser.Parse(cronExpr)
}
