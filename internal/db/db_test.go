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

func TestListStations(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Initially empty
	stations, err := db.ListStations()
	if err != nil {
		t.Fatalf("failed to list stations: %v", err)
	}
	if len(stations) != 0 {
		t.Errorf("expected 0 stations, got %d", len(stations))
	}

	// Add some stations
	db.GetOrCreateStation("WMBR", "MIT Radio", "https://wmbr.org")
	db.GetOrCreateStation("WHRB", "Harvard Radio", "https://whrb.org")

	stations, err = db.ListStations()
	if err != nil {
		t.Fatalf("failed to list stations: %v", err)
	}
	if len(stations) != 2 {
		t.Errorf("expected 2 stations, got %d", len(stations))
	}

	// Should be sorted by call sign
	if stations[0].CallSign != "WHRB" || stations[1].CallSign != "WMBR" {
		t.Errorf("expected stations sorted by call sign, got %s, %s", stations[0].CallSign, stations[1].CallSign)
	}
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
		M3UURL:      "https://wmbr.org/m3u/test.m3u",
		Status:      StatusPending,
	}

	id, err := db.InsertDownload(d)
	if err != nil {
		t.Fatalf("failed to insert download: %v", err)
	}

	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	// Verify status defaults to pending
	got, err := db.GetDownload(id)
	if err != nil {
		t.Fatalf("failed to get download: %v", err)
	}
	if got.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, got.Status)
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
	db.InsertDownload(&Download{StationID: wmbr.ID, ShowID: &show1.ID, ArchiveDate: time.Now(), M3UURL: "http://test/1.m3u"})
	db.InsertDownload(&Download{StationID: whrb.ID, ShowID: &show2.ID, ArchiveDate: time.Now(), M3UURL: "http://test/2.m3u"})
	db.InsertDownload(&Download{StationID: wmbr.ID, ShowID: &show1.ID, ArchiveDate: time.Now(), M3UURL: "http://test/3.m3u"})

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

func TestUpdateDownloadStatus(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Show1"})
	show, _ := db.GetShowByName(station.ID, "Show1")

	// Insert pending download
	id, err := db.InsertDownload(&Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://test.m3u",
	})
	if err != nil {
		t.Fatalf("failed to insert download: %v", err)
	}

	// Update to downloading
	err = db.UpdateDownloadStatus(id, StatusDownloading, "", "")
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	d, _ := db.GetDownload(id)
	if d.Status != StatusDownloading {
		t.Errorf("expected status %q, got %q", StatusDownloading, d.Status)
	}

	// Update to completed with filepath
	err = db.UpdateDownloadStatus(id, StatusCompleted, "/path/to/file.mp3", "")
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	d, _ = db.GetDownload(id)
	if d.Status != StatusCompleted {
		t.Errorf("expected status %q, got %q", StatusCompleted, d.Status)
	}
	if d.Filepath != "/path/to/file.mp3" {
		t.Errorf("expected filepath %q, got %q", "/path/to/file.mp3", d.Filepath)
	}
}

func TestUpdateDownloadStatus_Failed(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Show1"})
	show, _ := db.GetShowByName(station.ID, "Show1")

	id, _ := db.InsertDownload(&Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Now(),
		M3UURL:      "http://test.m3u",
	})

	// Update to failed with error
	err = db.UpdateDownloadStatus(id, StatusFailed, "", "connection timeout")
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	d, _ := db.GetDownload(id)
	if d.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, d.Status)
	}
	if d.Error != "connection timeout" {
		t.Errorf("expected error %q, got %q", "connection timeout", d.Error)
	}
}

func TestListDownloadsByStatus(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Show1"})
	show, _ := db.GetShowByName(station.ID, "Show1")

	// Insert downloads with different statuses
	id1, _ := db.InsertDownload(&Download{StationID: station.ID, ShowID: &show.ID, ArchiveDate: time.Now(), M3UURL: "http://1.m3u"})
	id2, _ := db.InsertDownload(&Download{StationID: station.ID, ShowID: &show.ID, ArchiveDate: time.Now(), M3UURL: "http://2.m3u"})
	id3, _ := db.InsertDownload(&Download{StationID: station.ID, ShowID: &show.ID, ArchiveDate: time.Now(), M3UURL: "http://3.m3u"})

	db.UpdateDownloadStatus(id1, StatusDownloading, "", "")
	db.UpdateDownloadStatus(id2, StatusCompleted, "/path.mp3", "")
	// id3 stays pending

	// List pending and downloading
	active, err := db.ListDownloadsByStatus(StatusPending, StatusDownloading)
	if err != nil {
		t.Fatalf("failed to list by status: %v", err)
	}

	if len(active) != 2 {
		t.Errorf("expected 2 active downloads, got %d", len(active))
	}

	// Verify id2 (completed) is not in the list
	for _, d := range active {
		if d.ID == id2 {
			t.Error("completed download should not be in active list")
		}
	}

	// Verify id3 (pending) is in the list
	found := false
	for _, d := range active {
		if d.ID == id3 {
			found = true
			break
		}
	}
	if !found {
		t.Error("pending download should be in active list")
	}
}

func TestGetDownload_NotFound(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.GetDownload(999)
	if err == nil {
		t.Error("expected error for non-existent download")
	}
}

func TestFindDownload(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Show1"})
	show, _ := db.GetShowByName(station.ID, "Show1")

	archiveDate := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)

	// No download exists yet
	found, err := db.FindDownload(station.ID, archiveDate)
	if err != nil {
		t.Fatalf("failed to find download: %v", err)
	}
	if found != nil {
		t.Error("expected nil for non-existent download")
	}

	// Insert a download
	db.InsertDownload(&Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: archiveDate,
		M3UURL:      "http://test.m3u",
	})

	// Now it should be found
	found, err = db.FindDownload(station.ID, archiveDate)
	if err != nil {
		t.Fatalf("failed to find download: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find download")
	}
	if found.StationID != station.ID {
		t.Errorf("expected station ID %d, got %d", station.ID, found.StationID)
	}
}

func TestListDownloadsByShowID(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Show1", "Show2"})
	show1, _ := db.GetShowByName(station.ID, "Show1")
	show2, _ := db.GetShowByName(station.ID, "Show2")

	// Insert downloads for different shows
	id1, _ := db.InsertDownload(&Download{StationID: station.ID, ShowID: &show1.ID, ArchiveDate: time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC), M3UURL: "http://1.m3u"})
	_, _ = db.InsertDownload(&Download{StationID: station.ID, ShowID: &show1.ID, ArchiveDate: time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC), M3UURL: "http://2.m3u"}) // stays pending
	db.InsertDownload(&Download{StationID: station.ID, ShowID: &show2.ID, ArchiveDate: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC), M3UURL: "http://3.m3u"})

	// Update one to completed
	db.UpdateDownloadStatus(id1, StatusCompleted, "/path.mp3", "")

	// List all downloads for show1
	downloads, err := db.ListDownloadsByShowID(show1.ID, "")
	if err != nil {
		t.Fatalf("failed to list downloads: %v", err)
	}
	if len(downloads) != 2 {
		t.Errorf("expected 2 downloads for show1, got %d", len(downloads))
	}

	// Should be sorted by archive_date descending
	if downloads[0].ArchiveDate.Before(downloads[1].ArchiveDate) {
		t.Error("expected downloads sorted by date descending")
	}

	// List only completed downloads for show1
	completed, err := db.ListDownloadsByShowID(show1.ID, StatusCompleted)
	if err != nil {
		t.Fatalf("failed to list completed downloads: %v", err)
	}
	if len(completed) != 1 {
		t.Errorf("expected 1 completed download, got %d", len(completed))
	}
	if completed[0].ID != id1 {
		t.Errorf("expected download ID %d, got %d", id1, completed[0].ID)
	}

	// List downloads for show2
	show2Downloads, err := db.ListDownloadsByShowID(show2.ID, "")
	if err != nil {
		t.Fatalf("failed to list downloads: %v", err)
	}
	if len(show2Downloads) != 1 {
		t.Errorf("expected 1 download for show2, got %d", len(show2Downloads))
	}
}
