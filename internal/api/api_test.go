package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"local/tapedeck/internal/db"

	// Register WMBR adapter for allshows test
	_ "local/tapedeck/internal/adapters/wmbr"
)

func setupTestServer(t *testing.T) (*Server, *db.DB) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	tmpDir := t.TempDir()
	server := NewServer(database, tmpDir)
	return server, database
}

func TestListStations(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	// Add stations
	database.GetOrCreateStation("WMBR", "MIT Radio", "https://wmbr.org")
	database.GetOrCreateStation("WHRB", "Harvard Radio", "https://whrb.org")

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/stations", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var stations []db.Station
	if err := json.Unmarshal(w.Body.Bytes(), &stations); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(stations) != 2 {
		t.Errorf("expected 2 stations, got %d", len(stations))
	}
}

func TestListStations_Empty(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/stations", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var stations []db.Station
	if err := json.Unmarshal(w.Body.Bytes(), &stations); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(stations) != 0 {
		t.Errorf("expected 0 stations, got %d", len(stations))
	}
}

func TestListShows(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Show1")
	database.InsertShow(station.ID, "Show2")
	database.InsertShow(station.ID, "Show3")
	show1, _ := database.GetShowByName(station.ID, "Show1")
	show2, _ := database.GetShowByName(station.ID, "Show2")
	// Show3 has no downloads, so it should not appear

	// Add downloads for Show1 and Show2
	database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show1.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://1.m3u",
	})
	database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show2.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://2.m3u",
	})

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/stations/WMBR/shows", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var shows []db.Show
	if err := json.Unmarshal(w.Body.Bytes(), &shows); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Only shows with downloads should be returned
	if len(shows) != 2 {
		t.Errorf("expected 2 shows with downloads, got %d", len(shows))
	}
}

func TestListShows_NotFound(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/stations/WXYZ/shows", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestListAllShows(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// WMBR is registered in the adapter registry, so it should return show names
	// Note: This makes a real HTTP request to WMBR's archive page
	// In a production test suite, you would mock the HTTP client
	req := httptest.NewRequest("GET", "/api/stations/WMBR/allshows", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var shows []string
	if err := json.Unmarshal(w.Body.Bytes(), &shows); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should return at least some shows
	if len(shows) == 0 {
		t.Error("expected at least some shows from WMBR")
	}
}

func TestListAllShows_UnknownStation(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/stations/WXYZ/allshows", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestListDownloads(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Show1")
	show, _ := database.GetShowByName(station.ID, "Show1")
	database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://test.m3u",
	})

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/downloads", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var downloads []db.Download
	if err := json.Unmarshal(w.Body.Bytes(), &downloads); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(downloads) != 1 {
		t.Errorf("expected 1 download, got %d", len(downloads))
	}
}

func TestListDownloads_FilterByStatus(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Show1")
	show, _ := database.GetShowByName(station.ID, "Show1")

	id1, _ := database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://1.m3u",
	})
	database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://2.m3u",
	})
	database.UpdateDownloadStatus(id1, db.StatusCompleted, "show1.mp3", "")

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/downloads?status=completed", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var downloads []db.Download
	if err := json.Unmarshal(w.Body.Bytes(), &downloads); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(downloads) != 1 {
		t.Errorf("expected 1 completed download, got %d", len(downloads))
	}
}

func TestGetDownload(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Show1")
	show, _ := database.GetShowByName(station.ID, "Show1")
	id, _ := database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://test.m3u",
	})

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/downloads/"+string(rune(id+'0')), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetDownload_NotFound(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/downloads/999", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestListShowDownloads(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Show1")
	database.InsertShow(station.ID, "Show2")
	show1, _ := database.GetShowByName(station.ID, "Show1")
	show2, _ := database.GetShowByName(station.ID, "Show2")

	database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show1.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://1.m3u",
	})
	database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show2.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://2.m3u",
	})

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/shows/"+string(rune(show1.ID+'0'))+"/downloads", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var downloads []db.Download
	if err := json.Unmarshal(w.Body.Bytes(), &downloads); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(downloads) != 1 {
		t.Errorf("expected 1 download for show1, got %d", len(downloads))
	}
}

func TestStreamAudio(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	// Create a test audio file
	audioContent := []byte("fake audio content for testing")
	audioFilename := "test.mp3"
	audioPath := filepath.Join(server.DownloadsDir, audioFilename)
	if err := os.WriteFile(audioPath, audioContent, 0644); err != nil {
		t.Fatalf("failed to create test audio file: %v", err)
	}

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Show1")
	show, _ := database.GetShowByName(station.ID, "Show1")
	id, _ := database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://test.m3u",
	})
	// Store just the filename (not full path) - this is the new behavior
	database.UpdateDownloadStatus(id, db.StatusCompleted, audioFilename, "")

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/audio/1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if w.Body.String() != string(audioContent) {
		t.Errorf("expected audio content, got %s", w.Body.String())
	}
}

func TestStreamAudio_NotCompleted(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Show1")
	show, _ := database.GetShowByName(station.ID, "Show1")
	database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://test.m3u",
		Status:      db.StatusPending,
	})

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/audio/1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestStreamAudio_PathTraversal(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	// Try to access file outside downloads directory using path traversal
	// The database stores filenames, but a malicious entry could try "../../etc/passwd"
	maliciousFilename := "../../etc/passwd"

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Show1")
	show, _ := database.GetShowByName(station.ID, "Show1")
	id, _ := database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://test.m3u",
	})
	database.UpdateDownloadStatus(id, db.StatusCompleted, maliciousFilename, "")

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/audio/1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestQueueDownload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server, database := setupTestServer(t)
	defer database.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Queue a download for Late Risers' Club (a real WMBR show)
	body := `{"station":"WMBR","show":"Late Risers' Club","date":"latest"}`
	req := httptest.NewRequest("POST", "/api/downloads", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var download db.Download
	if err := json.Unmarshal(w.Body.Bytes(), &download); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if download.Station != "WMBR" {
		t.Errorf("expected station WMBR, got %s", download.Station)
	}
	if download.Show != "Late Risers' Club" {
		t.Errorf("expected show Late Risers' Club, got %s", download.Show)
	}
	if download.Status != db.StatusPending {
		t.Errorf("expected status pending, got %s", download.Status)
	}
}

func TestQueueDownload_MissingFields(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Missing show
	body := `{"station":"WMBR"}`
	req := httptest.NewRequest("POST", "/api/downloads", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestQueueDownload_UnknownStation(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	body := `{"station":"WXYZ","show":"Test Show","date":"latest"}`
	req := httptest.NewRequest("POST", "/api/downloads", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestQueueDownload_InvalidDate(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	body := `{"station":"WMBR","show":"Lost Highway","date":"invalid-date"}`
	req := httptest.NewRequest("POST", "/api/downloads", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestQueueDownload_Duplicate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server, database := setupTestServer(t)
	defer database.Close()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// First request - should succeed
	body := `{"station":"WMBR","show":"Late Risers' Club","date":"latest"}`
	req1 := httptest.NewRequest("POST", "/api/downloads", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first request expected status 201, got %d: %s", w1.Code, w1.Body.String())
	}

	// Second request with same params - should return conflict
	req2 := httptest.NewRequest("POST", "/api/downloads", bytes.NewReader([]byte(body)))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("duplicate request expected status 409, got %d: %s", w2.Code, w2.Body.String())
	}
}

// mockScheduler implements the Scheduler interface for testing.
type mockScheduler struct {
	db        *db.DB
	schedules map[int64]*db.Schedule
	nextID    int64
}

func newMockScheduler(database *db.DB) *mockScheduler {
	return &mockScheduler{
		db:        database,
		schedules: make(map[int64]*db.Schedule),
		nextID:    1,
	}
}

func (m *mockScheduler) AddSchedule(stationID, showID int64, cronExpr string) (*db.Schedule, error) {
	sched := &db.Schedule{
		ID:             m.nextID,
		StationID:      stationID,
		ShowID:         showID,
		CronExpression: cronExpr,
		Enabled:        true,
	}
	m.schedules[m.nextID] = sched
	m.nextID++
	return sched, nil
}

func (m *mockScheduler) RemoveSchedule(id int64) error {
	delete(m.schedules, id)
	return nil
}

func (m *mockScheduler) ListSchedules() ([]db.Schedule, error) {
	result := make([]db.Schedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		result = append(result, *s)
	}
	return result, nil
}

func (m *mockScheduler) GetSchedule(id int64) (*db.Schedule, error) {
	s, ok := m.schedules[id]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (m *mockScheduler) SetEnabled(id int64, enabled bool) error {
	if s, ok := m.schedules[id]; ok {
		s.Enabled = enabled
	}
	return nil
}

func TestCreateScheduleAutoCron(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server, database := setupTestServer(t)
	defer database.Close()

	// Set up mock scheduler
	server.Scheduler = newMockScheduler(database)

	// Create station and show (required for schedule creation)
	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Late Risers' Club")

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Create schedule WITHOUT providing cron - should auto-determine
	body := `{"station":"WMBR","show":"Late Risers' Club"}`
	req := httptest.NewRequest("POST", "/api/schedules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var schedule db.Schedule
	if err := json.Unmarshal(w.Body.Bytes(), &schedule); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have auto-determined a cron expression
	if schedule.CronExpression == "" {
		t.Error("expected auto-determined cron expression, got empty string")
	}

	// Cron should be in format "MM HH * * D"
	if !strings.Contains(schedule.CronExpression, "*") {
		t.Errorf("expected cron with wildcards, got %s", schedule.CronExpression)
	}
}

func TestCreateScheduleWithCron(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	// Set up mock scheduler
	server.Scheduler = newMockScheduler(database)

	// Create station and show
	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.InsertShow(station.ID, "Test Show")

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Create schedule WITH explicit cron
	body := `{"station":"WMBR","show":"Test Show","cron":"0 12 * * 0"}`
	req := httptest.NewRequest("POST", "/api/schedules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var schedule db.Schedule
	if err := json.Unmarshal(w.Body.Bytes(), &schedule); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if schedule.CronExpression != "0 12 * * 0" {
		t.Errorf("expected cron '0 12 * * 0', got %s", schedule.CronExpression)
	}
}

// Tests for cron description

func TestDescribeCron(t *testing.T) {
	tests := []struct {
		name     string
		cronExpr string
		want     string
	}{
		{
			name:     "Saturday evening",
			cronExpr: "30 23 * * 6",
			want:     "Saturdays at 23:30",
		},
		{
			name:     "Sunday early morning",
			cronExpr: "30 1 * * 0",
			want:     "Sundays at 01:30",
		},
		{
			name:     "Monday afternoon",
			cronExpr: "30 16 * * 1",
			want:     "Mondays at 16:30",
		},
		{
			name:     "Every day",
			cronExpr: "0 12 * * *",
			want:     "Every day at 12:00",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := describeCron(tc.cronExpr)
			if got != tc.want {
				t.Errorf("describeCron(%q) = %q, want %q", tc.cronExpr, got, tc.want)
			}
		})
	}
}

func TestDescribeCron_InvalidCron(t *testing.T) {
	// Should return the original expression if it can't be parsed
	got := describeCron("invalid")
	if got != "invalid" {
		t.Errorf("expected 'invalid', got %q", got)
	}

	got = describeCron("1 2 3")
	if got != "1 2 3" {
		t.Errorf("expected '1 2 3', got %q", got)
	}
}

func TestFormatScheduleResponse(t *testing.T) {
	// Create a schedule with known values
	// All times are in local time (America/New_York)
	now := time.Date(2026, 1, 28, 9, 30, 0, 0, time.Local)
	nextRun := time.Date(2026, 2, 4, 9, 30, 0, 0, time.Local)

	sched := db.Schedule{
		ID:             1,
		StationID:      1,
		ShowID:         1,
		CronExpression: "30 9 * * 3", // Local time (EST)
		Enabled:        true,
		LastRunAt:      &now,
		NextRunAt:      &nextRun,
		Station:        "WMBR",
		Show:           "Test Show",
	}

	resp := formatScheduleResponse(sched)

	// Check that display strings are populated
	if resp.CronDescription == "" {
		t.Error("CronDescription should not be empty")
	}

	// CronDescription should show the time as-is (no timezone conversion)
	if !strings.Contains(resp.CronDescription, "09:30") {
		t.Errorf("CronDescription should show 09:30, got %q", resp.CronDescription)
	}

	if !strings.Contains(resp.CronDescription, "Wednesdays") {
		t.Errorf("CronDescription should show Wednesdays, got %q", resp.CronDescription)
	}

	// LastRunDisplay should show date in "Mon Jan 2" format
	if resp.LastRunDisplay == "-" {
		t.Error("LastRunDisplay should not be '-' when LastRunAt is set")
	}
	if resp.LastRunDisplay != "Wed Jan 28" {
		t.Errorf("LastRunDisplay should be 'Wed Jan 28', got %q", resp.LastRunDisplay)
	}

	// NextRunDisplay should show date in "Mon Jan 2" format
	if resp.NextRunDisplay == "-" {
		t.Error("NextRunDisplay should not be '-' when NextRunAt is set")
	}
	if resp.NextRunDisplay != "Wed Feb 4" {
		t.Errorf("NextRunDisplay should be 'Wed Feb 4', got %q", resp.NextRunDisplay)
	}
}

func TestFormatScheduleResponse_NilTimes(t *testing.T) {
	sched := db.Schedule{
		ID:             1,
		CronExpression: "30 9 * * 3",
		LastRunAt:      nil,
		NextRunAt:      nil,
		Station:        "WMBR",
		Show:           "Test Show",
	}

	resp := formatScheduleResponse(sched)

	if resp.LastRunDisplay != "-" {
		t.Errorf("LastRunDisplay should be '-' when LastRunAt is nil, got %q", resp.LastRunDisplay)
	}

	if resp.NextRunDisplay != "-" {
		t.Errorf("NextRunDisplay should be '-' when NextRunAt is nil, got %q", resp.NextRunDisplay)
	}
}
