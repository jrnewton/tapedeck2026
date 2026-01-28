package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"local/tapedeck/internal/db"
	"local/tapedeck/pkg/tapedeck"
)

// ScheduleResponse extends db.Schedule with pre-formatted display strings.
type ScheduleResponse struct {
	db.Schedule
	CronDescription string `json:"CronDescription"` // e.g., "Wednesdays at 09:30"
	LastRunDisplay  string `json:"LastRunDisplay"`  // e.g., "2026-01-28 09:30"
	NextRunDisplay  string `json:"NextRunDisplay"`  // e.g., "2026-02-04 09:30"
}

// Scheduler interface for schedule management.
type Scheduler interface {
	AddSchedule(stationID, showID int64, cronExpr string) (*db.Schedule, error)
	RemoveSchedule(id int64) error
	ListSchedules() ([]db.Schedule, error)
	GetSchedule(id int64) (*db.Schedule, error)
	SetEnabled(id int64, enabled bool) error
}

// Server holds API dependencies.
type Server struct {
	DB           *db.DB
	DownloadsDir string
	Scheduler    Scheduler
}

// NewServer creates a new API server.
func NewServer(database *db.DB, downloadsDir string) *Server {
	return &Server{
		DB:           database,
		DownloadsDir: downloadsDir,
	}
}

// RegisterRoutes registers all API routes on the given mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/stations", s.handleListStations)
	mux.HandleFunc("GET /api/stations/{call}/shows", s.handleListShows)
	mux.HandleFunc("GET /api/stations/{call}/allshows", s.handleListAllShows)
	mux.HandleFunc("GET /api/downloads", s.handleListDownloads)
	mux.HandleFunc("POST /api/downloads", s.handleQueueDownload)
	mux.HandleFunc("GET /api/downloads/{id}", s.handleGetDownload)
	mux.HandleFunc("GET /api/shows/{id}/downloads", s.handleListShowDownloads)
	mux.HandleFunc("GET /api/audio/{id}", s.handleStreamAudio)

	// Schedule endpoints
	mux.HandleFunc("GET /api/schedules", s.handleListSchedules)
	mux.HandleFunc("POST /api/schedules", s.handleCreateSchedule)
	mux.HandleFunc("GET /api/schedules/{id}", s.handleGetSchedule)
	mux.HandleFunc("DELETE /api/schedules/{id}", s.handleDeleteSchedule)
	mux.HandleFunc("PATCH /api/schedules/{id}", s.handleUpdateSchedule)
}

// handleListStations returns all registered stations.
func (s *Server) handleListStations(w http.ResponseWriter, r *http.Request) {
	stations, err := s.DB.ListStations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, stations)
}

// handleListShows returns shows for a station that have at least one download.
func (s *Server) handleListShows(w http.ResponseWriter, r *http.Request) {
	callSign := strings.ToUpper(r.PathValue("call"))
	if callSign == "" {
		http.Error(w, "station call sign required", http.StatusBadRequest)
		return
	}

	// Get station
	station, err := s.DB.GetStation(callSign)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Return only shows that have downloads
	shows, err := s.DB.ListShowsWithDownloads(station.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return [] instead of null for empty results
	if shows == nil {
		shows = []db.Show{}
	}

	writeJSON(w, shows)
}

// handleListAllShows returns ALL shows from the station adapter (not just shows with downloads).
func (s *Server) handleListAllShows(w http.ResponseWriter, r *http.Request) {
	callSign := strings.ToUpper(r.PathValue("call"))
	if callSign == "" {
		http.Error(w, "station call sign required", http.StatusBadRequest)
		return
	}

	adapter, err := tapedeck.GetAdapter(callSign)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	shows, err := adapter.ListShows()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return [] instead of null for empty results
	if shows == nil {
		shows = []string{}
	}

	writeJSON(w, shows)
}

// handleListDownloads returns all downloads, optionally filtered by status.
func (s *Server) handleListDownloads(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	var downloads []db.Download
	var err error

	if status != "" {
		downloads, err = s.DB.ListDownloadsByStatus(status)
	} else {
		downloads, err = s.DB.ListDownloads("")
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, downloads)
}

// handleQueueDownload queues an ad-hoc download request.
func (s *Server) handleQueueDownload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Station string `json:"station"`
		Show    string `json:"show"`
		Date    string `json:"date"` // "latest" or "YYYY-MM-DD"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Station == "" || req.Show == "" {
		http.Error(w, "station and show are required", http.StatusBadRequest)
		return
	}

	if req.Date == "" {
		req.Date = "latest"
	}

	// Get adapter
	adapter, err := tapedeck.GetAdapter(strings.ToUpper(req.Station))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Get archive (latest or by date)
	var archive *tapedeck.Archive
	if req.Date == "latest" {
		archive, err = adapter.GetLatestArchive(req.Show)
		if err != nil {
			http.Error(w, "failed to get latest archive: "+err.Error(), http.StatusNotFound)
			return
		}
	} else {
		// Parse date and find archive
		targetDate, parseErr := time.Parse("2006-01-02", req.Date)
		if parseErr != nil {
			http.Error(w, "invalid date format, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}

		archives, listErr := adapter.ListArchives(req.Show)
		if listErr != nil {
			http.Error(w, "failed to list archives: "+listErr.Error(), http.StatusInternalServerError)
			return
		}

		for i := range archives {
			if archives[i].Date.Year() == targetDate.Year() &&
				archives[i].Date.Month() == targetDate.Month() &&
				archives[i].Date.Day() == targetDate.Day() {
				archive = &archives[i]
				break
			}
		}
		if archive == nil {
			http.Error(w, "no archive found for date: "+req.Date, http.StatusNotFound)
			return
		}
	}

	// Get or create station in DB
	station, err := s.DB.GetOrCreateStation(strings.ToUpper(req.Station), "", "")
	if err != nil {
		http.Error(w, "failed to get station: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get or create show in DB
	_, err = s.DB.InsertShow(station.ID, req.Show)
	if err != nil {
		http.Error(w, "failed to create show: "+err.Error(), http.StatusInternalServerError)
		return
	}
	show, err := s.DB.GetShowByName(station.ID, req.Show)
	if err != nil {
		http.Error(w, "failed to get show: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check for existing download (duplicate detection)
	existing, err := s.DB.FindDownload(station.ID, &show.ID, archive.Date)
	if err != nil {
		http.Error(w, "failed to check existing download: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if existing != nil {
		// Return existing download with conflict status
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, existing)
		return
	}

	// Create download record with pending status
	download := &db.Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: archive.Date,
		M3UURL:      archive.M3UURL,
		Status:      db.StatusPending,
	}

	id, err := s.DB.InsertDownload(download)
	if err != nil {
		http.Error(w, "failed to create download: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Start background download
	go s.processDownload(id, adapter, archive)

	// Return created download
	createdDownload, err := s.DB.GetDownload(id)
	if err != nil {
		http.Error(w, "failed to get created download: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, createdDownload)
}

// processDownload runs in background to download the archive.
func (s *Server) processDownload(id int64, adapter tapedeck.Adapter, archive *tapedeck.Archive) {
	// Update status to downloading
	_ = s.DB.UpdateDownloadStatus(id, db.StatusDownloading, "", "")

	// Download the archive
	filepath, err := adapter.DownloadArchive(archive, s.DownloadsDir)
	if err != nil {
		_ = s.DB.UpdateDownloadStatus(id, db.StatusFailed, "", err.Error())
		return
	}

	// Store only the filename (not full path)
	filename := filepath[strings.LastIndex(filepath, "/")+1:]
	_ = s.DB.UpdateDownloadStatus(id, db.StatusCompleted, filename, "")
}

// handleGetDownload returns a single download by ID.
func (s *Server) handleGetDownload(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid download id", http.StatusBadRequest)
		return
	}

	download, err := s.DB.GetDownload(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, download)
}

// handleListShowDownloads returns downloads for a specific show.
func (s *Server) handleListShowDownloads(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid show id", http.StatusBadRequest)
		return
	}

	status := r.URL.Query().Get("status")
	downloads, err := s.DB.ListDownloadsByShowID(id, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, downloads)
}

// handleStreamAudio streams an audio file for a download.
func (s *Server) handleStreamAudio(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid download id", http.StatusBadRequest)
		return
	}

	download, err := s.DB.GetDownload(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if download.Filepath == "" || download.Status != db.StatusCompleted {
		http.Error(w, "download not completed", http.StatusNotFound)
		return
	}

	// Construct full path from stored filename
	// Database stores only filenames (e.g., "WMBR_ShowName_20260124.mp3")
	// so paths work across host CLI, Docker CLI, and Docker Web contexts
	fullPath := filepath.Join(s.DownloadsDir, download.Filepath)

	// Security: validate constructed path is within downloads directory (defense in depth)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, "invalid filepath", http.StatusBadRequest)
		return
	}

	absDownloadsDir, err := filepath.Abs(s.DownloadsDir)
	if err != nil {
		http.Error(w, "server configuration error", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(absPath, absDownloadsDir) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	// Check file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Set correct MIME type for MP3 files (Go's mime detection may not work on Alpine)
	if strings.HasSuffix(strings.ToLower(absPath), ".mp3") {
		w.Header().Set("Content-Type", "audio/mpeg")
	}

	// Serve file with Range support
	http.ServeFile(w, r, absPath)
}

// handleListSchedules returns all schedules.
func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	if s.Scheduler == nil {
		http.Error(w, "scheduler not configured", http.StatusServiceUnavailable)
		return
	}

	schedules, err := s.Scheduler.ListSchedules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format with display strings
	responses := make([]ScheduleResponse, 0, len(schedules))
	for _, sched := range schedules {
		responses = append(responses, formatScheduleResponse(sched))
	}

	writeJSON(w, map[string]any{"schedules": responses})
}

// handleCreateSchedule creates a new schedule.
func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	if s.Scheduler == nil {
		http.Error(w, "scheduler not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Station string `json:"station"`
		Show    string `json:"show"`
		Cron    string `json:"cron"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Station == "" || req.Show == "" {
		http.Error(w, "station and show are required", http.StatusBadRequest)
		return
	}

	// Get station
	station, err := s.DB.GetStation(strings.ToUpper(req.Station))
	if err != nil {
		http.Error(w, "station not found: "+req.Station, http.StatusNotFound)
		return
	}

	// Get show
	show, err := s.DB.GetShowByName(station.ID, req.Show)
	if err != nil || show == nil {
		http.Error(w, "show not found: "+req.Show, http.StatusNotFound)
		return
	}

	// Check if schedule already exists
	existing, err := s.DB.FindScheduleByShowID(station.ID, show.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, "schedule already exists for this show", http.StatusConflict)
		return
	}

	// If no cron provided, auto-determine from adapter
	if req.Cron == "" {
		adapter, adapterErr := tapedeck.GetAdapter(station.CallSign)
		if adapterErr != nil {
			http.Error(w, "cannot determine schedule: unknown station", http.StatusBadRequest)
			return
		}
		schedule, schedErr := adapter.GetShowSchedule(req.Show)
		if schedErr != nil {
			http.Error(w, "cannot determine schedule: "+schedErr.Error(), http.StatusBadRequest)
			return
		}
		req.Cron = schedule.RecommendedCron
	}

	sched, err := s.Scheduler.AddSchedule(station.ID, show.ID, req.Cron)
	if err != nil {
		http.Error(w, "invalid cron expression: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, formatScheduleResponse(*sched))
}

// handleGetSchedule returns a single schedule by ID.
func (s *Server) handleGetSchedule(w http.ResponseWriter, r *http.Request) {
	if s.Scheduler == nil {
		http.Error(w, "scheduler not configured", http.StatusServiceUnavailable)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid schedule id", http.StatusBadRequest)
		return
	}

	sched, err := s.Scheduler.GetSchedule(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, formatScheduleResponse(*sched))
}

// handleDeleteSchedule removes a schedule.
func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	if s.Scheduler == nil {
		http.Error(w, "scheduler not configured", http.StatusServiceUnavailable)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid schedule id", http.StatusBadRequest)
		return
	}

	err = s.Scheduler.RemoveSchedule(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateSchedule updates a schedule (enable/disable).
func (s *Server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	if s.Scheduler == nil {
		http.Error(w, "scheduler not configured", http.StatusServiceUnavailable)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid schedule id", http.StatusBadRequest)
		return
	}

	var req struct {
		Enabled *bool `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Enabled != nil {
		if err := s.Scheduler.SetEnabled(id, *req.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Return updated schedule
	sched, err := s.Scheduler.GetSchedule(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, formatScheduleResponse(*sched))
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// formatScheduleResponse converts a db.Schedule to ScheduleResponse with display strings.
// All times are in America/New_York (server timezone via TZ env var).
func formatScheduleResponse(sched db.Schedule) ScheduleResponse {
	resp := ScheduleResponse{
		Schedule:        sched,
		CronDescription: describeCron(sched.CronExpression),
		LastRunDisplay:  "-",
		NextRunDisplay:  "-",
	}

	if sched.LastRunAt != nil {
		resp.LastRunDisplay = sched.LastRunAt.Format("2006-01-02 15:04")
	}

	if sched.NextRunAt != nil {
		resp.NextRunDisplay = sched.NextRunAt.Format("2006-01-02 15:04")
	}

	return resp
}

// describeCron converts a cron expression to a human-readable description.
// Example: "30 23 * * 6" -> "Saturdays at 23:30"
func describeCron(cronExpr string) string {
	parts := strings.Fields(cronExpr)
	if len(parts) < 5 {
		return cronExpr
	}

	min, _ := strconv.Atoi(parts[0])
	hour, _ := strconv.Atoi(parts[1])
	dow := parts[4]

	// Format time
	timeStr := fmt.Sprintf("%02d:%02d", hour, min)

	// Day of week names
	days := map[string]string{
		"0": "Sundays",
		"1": "Mondays",
		"2": "Tuesdays",
		"3": "Wednesdays",
		"4": "Thursdays",
		"5": "Fridays",
		"6": "Saturdays",
		"7": "Sundays", // Some systems use 7 for Sunday
		"*": "Every day",
	}

	dayStr, ok := days[dow]
	if !ok {
		dayStr = "day " + dow
	}

	return fmt.Sprintf("%s at %s", dayStr, timeStr)
}
