package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jnewton/tapedeck/internal/db"
)

// Server holds API dependencies.
type Server struct {
	DB           *db.DB
	DownloadsDir string
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

// handleListShows returns shows for a station.
func (s *Server) handleListShows(w http.ResponseWriter, r *http.Request) {
	callSign := r.PathValue("call")
	if callSign == "" {
		http.Error(w, "station call sign required", http.StatusBadRequest)
		return
	}

	station, err := s.DB.GetStation(callSign)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	shows, _, err := s.DB.GetCachedShows(station.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

	// Security: validate filepath is within downloads directory
	absPath, err := filepath.Abs(download.Filepath)
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

	// Serve file with Range support
	http.ServeFile(w, r, absPath)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
