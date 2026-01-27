package scheduler

import (
	"testing"
	"time"

	"local/tapedeck/internal/db"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxConcurrent != 2 {
		t.Errorf("expected MaxConcurrent=2, got %d", cfg.MaxConcurrent)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5, got %d", cfg.MaxRetries)
	}
	if cfg.RetryBackoff != 1.3 {
		t.Errorf("expected RetryBackoff=1.3, got %f", cfg.RetryBackoff)
	}
	if cfg.BaseDelay != 1*time.Minute {
		t.Errorf("expected BaseDelay=1m, got %v", cfg.BaseDelay)
	}
	if cfg.TickInterval != 1*time.Minute {
		t.Errorf("expected TickInterval=1m, got %v", cfg.TickInterval)
	}
}

func TestNew_DefaultsForZeroConfig(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Pass empty config - should use defaults
	s := New(database, "/tmp/downloads", Config{})

	if s.config.MaxConcurrent != 2 {
		t.Errorf("expected MaxConcurrent default=2, got %d", s.config.MaxConcurrent)
	}
	if s.config.MaxRetries != 5 {
		t.Errorf("expected MaxRetries default=5, got %d", s.config.MaxRetries)
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	s := New(database, "/tmp/downloads", Config{
		BaseDelay:    1 * time.Minute,
		RetryBackoff: 1.3,
	})

	tests := []struct {
		retryCount int
		expected   time.Duration
	}{
		{1, 1 * time.Minute},                                                  // 1 * 1.3^0 = 1
		{2, time.Duration(float64(1*time.Minute) * 1.3)},                      // 1 * 1.3^1 = 1.3
		{3, time.Duration(float64(1*time.Minute) * 1.3 * 1.3)},                // 1 * 1.3^2 = 1.69
		{4, time.Duration(float64(1*time.Minute) * 1.3 * 1.3 * 1.3)},          // 1 * 1.3^3 = 2.197
		{5, time.Duration(float64(1*time.Minute) * 1.3 * 1.3 * 1.3 * 1.3)},    // 1 * 1.3^4 = 2.856
	}

	for _, tc := range tests {
		got := s.calculateRetryDelay(tc.retryCount)
		// Allow small floating point tolerance
		diff := got - tc.expected
		if diff < 0 {
			diff = -diff
		}
		if diff > time.Millisecond {
			t.Errorf("retry %d: expected %v, got %v", tc.retryCount, tc.expected, got)
		}
	}
}

func TestCalculateNextRun(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	s := New(database, "/tmp/downloads", DefaultConfig())

	// Test valid cron expression
	next := s.calculateNextRun("30 4 * * 0") // Every Sunday at 4:30
	if next == nil {
		t.Fatal("expected next run time, got nil")
	}

	// Verify it's in the future
	if !next.After(time.Now()) {
		t.Errorf("expected next run to be in future, got %v", next)
	}

	// Verify it's on a Sunday
	if next.Weekday() != time.Sunday {
		t.Errorf("expected Sunday, got %v", next.Weekday())
	}

	// Verify hour and minute
	if next.Hour() != 4 || next.Minute() != 30 {
		t.Errorf("expected 04:30, got %02d:%02d", next.Hour(), next.Minute())
	}
}

func TestCalculateNextRun_InvalidCron(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	s := New(database, "/tmp/downloads", DefaultConfig())

	// Invalid cron expression should return nil
	next := s.calculateNextRun("invalid cron")
	if next != nil {
		t.Errorf("expected nil for invalid cron, got %v", next)
	}
}

func TestParseCronExpression(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	s := New(database, "/tmp/downloads", DefaultConfig())

	tests := []struct {
		expr    string
		wantErr bool
	}{
		{"30 4 * * 0", false},   // Valid: Sunday 4:30
		{"0 22 * * 2", false},   // Valid: Tuesday 22:00
		{"*/15 * * * *", false}, // Valid: Every 15 minutes
		{"invalid", true},       // Invalid
		{"60 4 * * 0", true},    // Invalid minute
		{"30 25 * * 0", true},   // Invalid hour
	}

	for _, tc := range tests {
		_, err := s.ParseCronExpression(tc.expr)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseCronExpression(%q): got err=%v, wantErr=%v", tc.expr, err, tc.wantErr)
		}
	}
}

func TestSchedulerStartStop(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	s := New(database, "/tmp/downloads", Config{
		TickInterval: 100 * time.Millisecond,
	})

	// Start scheduler
	s.Start()

	// Verify it's running
	if !s.running {
		t.Error("expected scheduler to be running")
	}

	// Starting again should be no-op
	s.Start()

	// Stop scheduler
	s.Stop()

	// Verify it's stopped
	if s.running {
		t.Error("expected scheduler to be stopped")
	}

	// Stopping again should be no-op
	s.Stop()
}

func TestAddSchedule(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Create station and show
	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Test Show")
	show, _ := database.GetShowByName(station.ID, "Test Show")

	s := New(database, "/tmp/downloads", DefaultConfig())

	// Add a schedule
	sched, err := s.AddSchedule(station.ID, show.ID, "30 4 * * 0")
	if err != nil {
		t.Fatalf("failed to add schedule: %v", err)
	}

	if sched.ID <= 0 {
		t.Errorf("expected positive ID, got %d", sched.ID)
	}
	if sched.CronExpression != "30 4 * * 0" {
		t.Errorf("expected cron '30 4 * * 0', got %q", sched.CronExpression)
	}
	if !sched.Enabled {
		t.Error("expected schedule to be enabled")
	}
	if sched.NextRunAt == nil {
		t.Error("expected next_run_at to be set")
	}
	if sched.Station != "WMBR" {
		t.Errorf("expected station 'WMBR', got %q", sched.Station)
	}
	if sched.Show != "Test Show" {
		t.Errorf("expected show 'Test Show', got %q", sched.Show)
	}
}

func TestAddSchedule_InvalidCron(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Test Show")
	show, _ := database.GetShowByName(station.ID, "Test Show")

	s := New(database, "/tmp/downloads", DefaultConfig())

	// Invalid cron expression should fail
	_, err = s.AddSchedule(station.ID, show.ID, "invalid")
	if err == nil {
		t.Error("expected error for invalid cron")
	}
}

func TestAddSchedule_Duplicate(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Test Show")
	show, _ := database.GetShowByName(station.ID, "Test Show")

	s := New(database, "/tmp/downloads", DefaultConfig())

	// Add first schedule
	_, err = s.AddSchedule(station.ID, show.ID, "30 4 * * 0")
	if err != nil {
		t.Fatalf("failed to add first schedule: %v", err)
	}

	// Adding duplicate should fail (unique constraint)
	_, err = s.AddSchedule(station.ID, show.ID, "0 5 * * 1")
	if err == nil {
		t.Error("expected error for duplicate schedule")
	}
}

func TestListSchedules(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Show A")
	database.InsertShow(station.ID, "Show B")
	showA, _ := database.GetShowByName(station.ID, "Show A")
	showB, _ := database.GetShowByName(station.ID, "Show B")

	s := New(database, "/tmp/downloads", DefaultConfig())

	// Initially empty
	schedules, err := s.ListSchedules()
	if err != nil {
		t.Fatalf("failed to list schedules: %v", err)
	}
	if len(schedules) != 0 {
		t.Errorf("expected 0 schedules, got %d", len(schedules))
	}

	// Add schedules
	s.AddSchedule(station.ID, showA.ID, "30 4 * * 0")
	s.AddSchedule(station.ID, showB.ID, "0 5 * * 1")

	schedules, err = s.ListSchedules()
	if err != nil {
		t.Fatalf("failed to list schedules: %v", err)
	}
	if len(schedules) != 2 {
		t.Errorf("expected 2 schedules, got %d", len(schedules))
	}
}

func TestRemoveSchedule(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Test Show")
	show, _ := database.GetShowByName(station.ID, "Test Show")

	s := New(database, "/tmp/downloads", DefaultConfig())

	// Add schedule
	sched, _ := s.AddSchedule(station.ID, show.ID, "30 4 * * 0")

	// Remove it
	err = s.RemoveSchedule(sched.ID)
	if err != nil {
		t.Fatalf("failed to remove schedule: %v", err)
	}

	// Verify it's gone
	schedules, _ := s.ListSchedules()
	if len(schedules) != 0 {
		t.Errorf("expected 0 schedules after removal, got %d", len(schedules))
	}
}

func TestSetEnabled(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Test Show")
	show, _ := database.GetShowByName(station.ID, "Test Show")

	s := New(database, "/tmp/downloads", DefaultConfig())

	// Add schedule (enabled by default)
	sched, _ := s.AddSchedule(station.ID, show.ID, "30 4 * * 0")

	// Disable it
	err = s.SetEnabled(sched.ID, false)
	if err != nil {
		t.Fatalf("failed to disable schedule: %v", err)
	}

	// Verify it's disabled
	got, _ := s.GetSchedule(sched.ID)
	if got.Enabled {
		t.Error("expected schedule to be disabled")
	}

	// Re-enable it
	err = s.SetEnabled(sched.ID, true)
	if err != nil {
		t.Fatalf("failed to enable schedule: %v", err)
	}

	got, _ = s.GetSchedule(sched.ID)
	if !got.Enabled {
		t.Error("expected schedule to be enabled")
	}
}

func TestConcurrencyLimit(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	s := New(database, "/tmp/downloads", Config{
		MaxConcurrent: 3,
	})

	// Verify semaphore capacity
	if cap(s.semaphore) != 3 {
		t.Errorf("expected semaphore capacity=3, got %d", cap(s.semaphore))
	}
}
