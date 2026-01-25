package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jnewton/tapedeck/internal/db"
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
	database.CacheShows(station.ID, []string{"Show1", "Show2"})

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

	if len(shows) != 2 {
		t.Errorf("expected 2 shows, got %d", len(shows))
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

func TestListDownloads(t *testing.T) {
	server, database := setupTestServer(t)
	defer database.Close()

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.CacheShows(station.ID, []string{"Show1"})
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
	database.CacheShows(station.ID, []string{"Show1"})
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
	database.UpdateDownloadStatus(id1, db.StatusCompleted, "/path.mp3", "")

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
	database.CacheShows(station.ID, []string{"Show1"})
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
	database.CacheShows(station.ID, []string{"Show1", "Show2"})
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
	audioPath := filepath.Join(server.DownloadsDir, "test.mp3")
	if err := os.WriteFile(audioPath, audioContent, 0644); err != nil {
		t.Fatalf("failed to create test audio file: %v", err)
	}

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.CacheShows(station.ID, []string{"Show1"})
	show, _ := database.GetShowByName(station.ID, "Show1")
	id, _ := database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://test.m3u",
	})
	database.UpdateDownloadStatus(id, db.StatusCompleted, audioPath, "")

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
	database.CacheShows(station.ID, []string{"Show1"})
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

	// Try to access file outside downloads directory
	outsidePath := "/etc/passwd"

	station, _ := database.GetOrCreateStation("WMBR", "", "")
	database.CacheShows(station.ID, []string{"Show1"})
	show, _ := database.GetShowByName(station.ID, "Show1")
	id, _ := database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://test.m3u",
	})
	database.UpdateDownloadStatus(id, db.StatusCompleted, outsidePath, "")

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/audio/1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}
