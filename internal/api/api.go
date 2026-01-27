package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"local/tapedeck/internal/db"
)

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
	mux.HandleFunc("GET /api/downloads", s.handleListDownloads)
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

	// Ensure we return [] instead of null
	if schedules == nil {
		schedules = []db.Schedule{}
	}

	writeJSON(w, map[string]any{"schedules": schedules})
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

	// If no cron provided, we would need to infer it from adapter
	// For now, require it
	if req.Cron == "" {
		http.Error(w, "cron expression is required", http.StatusBadRequest)
		return
	}

	sched, err := s.Scheduler.AddSchedule(station.ID, show.ID, req.Cron)
	if err != nil {
		http.Error(w, "invalid cron expression: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, sched)
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

	writeJSON(w, sched)
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

	writeJSON(w, sched)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
