package db

import (
	"testing"
	"time"
)

func TestOpen_InMemory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	defer db.Close()
}

func TestGetOrCreateStation(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create station
	station, err := db.GetOrCreateStation("WMBR", "MIT Radio", "https://wmbr.org/cgi-bin/arch")
	if err != nil {
		t.Fatalf("failed to create station: %v", err)
	}

	if station.CallSign != "WMBR" {
		t.Errorf("expected call sign WMBR, got %s", station.CallSign)
	}
	if station.ID <= 0 {
		t.Errorf("expected positive ID, got %d", station.ID)
	}

	// Get same station again
	station2, err := db.GetOrCreateStation("WMBR", "", "")
	if err != nil {
		t.Fatalf("failed to get station: %v", err)
	}

	if station2.ID != station.ID {
		t.Errorf("expected same ID %d, got %d", station.ID, station2.ID)
	}
}

func TestGetStation(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create station first
	_, err = db.GetOrCreateStation("WMBR", "MIT Radio", "https://wmbr.org/cgi-bin/arch")
	if err != nil {
		t.Fatalf("failed to create station: %v", err)
	}

	// Get station
	station, err := db.GetStation("WMBR")
	if err != nil {
		t.Fatalf("failed to get station: %v", err)
	}

	if station.CallSign != "WMBR" {
		t.Errorf("expected call sign WMBR, got %s", station.CallSign)
	}
}

func TestGetStation_NotFound(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.GetStation("WXYZ")
	if err == nil {
		t.Error("expected error for non-existent station")
	}
}

func TestCacheShows(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, err := db.GetOrCreateStation("WMBR", "", "")
	if err != nil {
		t.Fatalf("failed to create station: %v", err)
	}

	// Cache shows
	shows := []string{"Lost and Found", "Backwoods", "Pipeline"}
	err = db.CacheShows(station.ID, shows)
	if err != nil {
		t.Fatalf("failed to cache shows: %v", err)
	}

	// Get cached shows
	cached, valid, err := db.GetCachedShows(station.ID)
	if err != nil {
		t.Fatalf("failed to get cached shows: %v", err)
	}

	if !valid {
		t.Error("expected cache to be valid")
	}

	if len(cached) != 3 {
		t.Errorf("expected 3 shows, got %d", len(cached))
	}
}

func TestCacheShows_Expiry(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Set very short TTL
	db.CacheTTL = 1 * time.Millisecond

	station, err := db.GetOrCreateStation("WMBR", "", "")
	if err != nil {
		t.Fatalf("failed to create station: %v", err)
	}

	err = db.CacheShows(station.ID, []string{"Show1"})
	if err != nil {
		t.Fatalf("failed to cache shows: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(5 * time.Millisecond)

	_, valid, err := db.GetCachedShows(station.ID)
	if err != nil {
		t.Fatalf("failed to get cached shows: %v", err)
	}

	if valid {
		t.Error("expected cache to be expired")
	}
}

func TestGetShowByName(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, err := db.GetOrCreateStation("WMBR", "", "")
	if err != nil {
		t.Fatalf("failed to create station: %v", err)
	}

	err = db.CacheShows(station.ID, []string{"Lost and Found", "Backwoods"})
	if err != nil {
		t.Fatalf("failed to cache shows: %v", err)
	}

	show, err := db.GetShowByName(station.ID, "Lost and Found")
	if err != nil {
		t.Fatalf("failed to get show: %v", err)
	}

	if show == nil {
		t.Fatal("expected show, got nil")
	}

	if show.Name != "Lost and Found" {
		t.Errorf("expected name 'Lost and Found', got %q", show.Name)
	}
}

func TestCacheArchives(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, err := db.GetOrCreateStation("WMBR", "", "")
	if err != nil {
		t.Fatalf("failed to create station: %v", err)
	}

	err = db.CacheShows(station.ID, []string{"Lost and Found"})
	if err != nil {
		t.Fatalf("failed to cache shows: %v", err)
	}

	show, err := db.GetShowByName(station.ID, "Lost and Found")
	if err != nil {
		t.Fatalf("failed to get show: %v", err)
	}

	// Cache archives
	archives := []Archive{
		{Date: time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC), M3UURL: "https://wmbr.org/m3u/test1.m3u"},
		{Date: time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC), M3UURL: "https://wmbr.org/m3u/test2.m3u"},
	}
	err = db.CacheArchives(show.ID, archives)
	if err != nil {
		t.Fatalf("failed to cache archives: %v", err)
	}

	// Get cached archives
	cached, valid, err := db.GetCachedArchives(show.ID)
	if err != nil {
		t.Fatalf("failed to get cached archives: %v", err)
	}

	if !valid {
		t.Error("expected cache to be valid")
	}

	if len(cached) != 2 {
		t.Errorf("expected 2 archives, got %d", len(cached))
	}

	// Should be sorted by date descending
	if cached[0].Date.Before(cached[1].Date) {
		t.Error("expected archives sorted by date descending")
	}
}

func TestInsertDownload(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, err := db.GetOrCreateStation("WMBR", "", "")
	if err != nil {
		t.Fatalf("failed to create station: %v", err)
	}

	err = db.CacheShows(station.ID, []string{"Lost and Found"})
	if err != nil {
		t.Fatalf("failed to cache shows: %v", err)
	}

	show, err := db.GetShowByName(station.ID, "Lost and Found")
	if err != nil {
		t.Fatalf("failed to get show: %v", err)
	}

	d := &Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC),
		Filepath:    "/data/downloads/WMBR_Lost_and_Found_20260125.mp3",
		Status:      "completed",
	}

	id, err := db.InsertDownload(d)
	if err != nil {
		t.Fatalf("failed to insert download: %v", err)
	}

	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}
}

func TestListDownloads(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create two stations
	wmbr, _ := db.GetOrCreateStation("WMBR", "", "")
	whrb, _ := db.GetOrCreateStation("WHRB", "", "")

	db.CacheShows(wmbr.ID, []string{"Show1"})
	db.CacheShows(whrb.ID, []string{"Show2"})

	show1, _ := db.GetShowByName(wmbr.ID, "Show1")
	show2, _ := db.GetShowByName(whrb.ID, "Show2")

	// Insert downloads
	db.InsertDownload(&Download{StationID: wmbr.ID, ShowID: &show1.ID, ArchiveDate: time.Now(), Filepath: "/path1.mp3"})
	db.InsertDownload(&Download{StationID: whrb.ID, ShowID: &show2.ID, ArchiveDate: time.Now(), Filepath: "/path2.mp3"})
	db.InsertDownload(&Download{StationID: wmbr.ID, ShowID: &show1.ID, ArchiveDate: time.Now(), Filepath: "/path3.mp3"})

	// List all
	all, err := db.ListDownloads("")
	if err != nil {
		t.Fatalf("failed to list downloads: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 downloads, got %d", len(all))
	}

	// List by station
	wmbrDownloads, err := db.ListDownloads("WMBR")
	if err != nil {
		t.Fatalf("failed to list downloads: %v", err)
	}
	if len(wmbrDownloads) != 2 {
		t.Errorf("expected 2 WMBR downloads, got %d", len(wmbrDownloads))
	}

	for _, d := range wmbrDownloads {
		if d.Station != "WMBR" {
			t.Errorf("expected station WMBR, got %s", d.Station)
		}
	}
}
