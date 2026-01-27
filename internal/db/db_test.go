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

	if !show.Active {
		t.Error("expected show to be active")
	}
}

func TestCacheShows_PreservesIDs(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")

	// Initial cache
	err = db.CacheShows(station.ID, []string{"Backwoods", "Pipeline"})
	if err != nil {
		t.Fatalf("failed to cache shows: %v", err)
	}

	show1, _ := db.GetShowByName(station.ID, "Backwoods")
	originalID := show1.ID

	// Re-cache with same shows (simulating cache refresh)
	err = db.CacheShows(station.ID, []string{"Backwoods", "Pipeline"})
	if err != nil {
		t.Fatalf("failed to re-cache shows: %v", err)
	}

	show2, _ := db.GetShowByName(station.ID, "Backwoods")

	// ID should be preserved
	if show2.ID != originalID {
		t.Errorf("expected ID %d to be preserved, got %d", originalID, show2.ID)
	}
}

func TestCacheShows_MarksInactive(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")

	// Cache initial shows
	err = db.CacheShows(station.ID, []string{"Backwoods", "Pipeline", "OldShow"})
	if err != nil {
		t.Fatalf("failed to cache shows: %v", err)
	}

	oldShow, _ := db.GetShowByName(station.ID, "OldShow")
	oldShowID := oldShow.ID

	// Re-cache without OldShow
	err = db.CacheShows(station.ID, []string{"Backwoods", "Pipeline"})
	if err != nil {
		t.Fatalf("failed to re-cache shows: %v", err)
	}

	// GetCachedShows should only return active shows
	cached, _, _ := db.GetCachedShows(station.ID)
	if len(cached) != 2 {
		t.Errorf("expected 2 active shows, got %d", len(cached))
	}
	for _, s := range cached {
		if s.Name == "OldShow" {
			t.Error("OldShow should not appear in active shows")
		}
	}

	// But GetShowByName should still find the inactive show
	oldShowAfter, _ := db.GetShowByName(station.ID, "OldShow")
	if oldShowAfter == nil {
		t.Fatal("expected to find inactive show by name")
	}
	if oldShowAfter.ID != oldShowID {
		t.Errorf("expected ID %d, got %d", oldShowID, oldShowAfter.ID)
	}
	if oldShowAfter.Active {
		t.Error("expected show to be inactive")
	}
}

func TestCacheShows_PreservesForeignKeys(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")

	// Cache shows
	err = db.CacheShows(station.ID, []string{"Backwoods"})
	if err != nil {
		t.Fatalf("failed to cache shows: %v", err)
	}

	show, _ := db.GetShowByName(station.ID, "Backwoods")
	originalShowID := show.ID

	// Create a download referencing the show
	downloadID, err := db.InsertDownload(&Download{
		StationID:   station.ID,
		ShowID:      &show.ID,
		ArchiveDate: time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC),
		M3UURL:      "http://test.m3u",
	})
	if err != nil {
		t.Fatalf("failed to insert download: %v", err)
	}

	// Create a schedule referencing the show
	scheduleID, err := db.InsertSchedule(&Schedule{
		StationID:      station.ID,
		ShowID:         show.ID,
		CronExpression: "30 4 * * 0",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("failed to insert schedule: %v", err)
	}

	// Re-cache shows (simulating hourly refresh)
	err = db.CacheShows(station.ID, []string{"Backwoods"})
	if err != nil {
		t.Fatalf("failed to re-cache shows: %v", err)
	}

	// Verify show ID is preserved
	showAfter, _ := db.GetShowByName(station.ID, "Backwoods")
	if showAfter.ID != originalShowID {
		t.Errorf("show ID changed from %d to %d", originalShowID, showAfter.ID)
	}

	// Verify download still references the correct show
	download, err := db.GetDownload(downloadID)
	if err != nil {
		t.Fatalf("failed to get download: %v", err)
	}
	if download.ShowID == nil || *download.ShowID != originalShowID {
		t.Error("download show_id reference was corrupted")
	}
	if download.Show != "Backwoods" {
		t.Errorf("expected show name 'Backwoods', got %q", download.Show)
	}

	// Verify schedule still references the correct show
	schedule, err := db.GetSchedule(scheduleID)
	if err != nil {
		t.Fatalf("failed to get schedule: %v", err)
	}
	if schedule.ShowID != originalShowID {
		t.Error("schedule show_id reference was corrupted")
	}
	if schedule.Show != "Backwoods" {
		t.Errorf("expected show name 'Backwoods', got %q", schedule.Show)
	}
}

func TestCacheShows_ReactivatesShow(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")

	// Initial cache with show
	db.CacheShows(station.ID, []string{"Backwoods"})
	show, _ := db.GetShowByName(station.ID, "Backwoods")
	originalID := show.ID

	// Remove show (mark inactive)
	db.CacheShows(station.ID, []string{"Pipeline"})
	show, _ = db.GetShowByName(station.ID, "Backwoods")
	if show.Active {
		t.Error("expected show to be inactive after removal")
	}

	// Re-add show (should reactivate with same ID)
	db.CacheShows(station.ID, []string{"Backwoods", "Pipeline"})
	show, _ = db.GetShowByName(station.ID, "Backwoods")

	if show.ID != originalID {
		t.Errorf("expected ID %d to be preserved, got %d", originalID, show.ID)
	}
	if !show.Active {
		t.Error("expected show to be reactivated")
	}
}

func TestUpdateShowArchive(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Test Show"})
	show, _ := db.GetShowByName(station.ID, "Test Show")

	// Initially no archive data
	if show.ArchiveCurrentDate != nil {
		t.Error("expected nil archive date initially")
	}

	// Add first archive
	date1 := time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC)
	err = db.UpdateShowArchive(show.ID, date1, "https://wmbr.org/m3u/test1.m3u")
	if err != nil {
		t.Fatalf("failed to update archive: %v", err)
	}

	// Verify archive is stored
	show, _ = db.GetShowByName(station.ID, "Test Show")
	if show.ArchiveCurrentDate == nil {
		t.Fatal("expected archive date to be set")
	}
	if !show.ArchiveCurrentDate.Equal(date1) {
		t.Errorf("expected date %v, got %v", date1, *show.ArchiveCurrentDate)
	}
	if show.ArchiveCurrentM3UURL != "https://wmbr.org/m3u/test1.m3u" {
		t.Errorf("expected m3u URL test1.m3u, got %s", show.ArchiveCurrentM3UURL)
	}
	if show.ArchivePreviousDate != nil {
		t.Error("expected no previous archive yet")
	}

	// Add second archive (should rotate)
	date2 := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)
	err = db.UpdateShowArchive(show.ID, date2, "https://wmbr.org/m3u/test2.m3u")
	if err != nil {
		t.Fatalf("failed to update archive: %v", err)
	}

	// Verify rotation happened
	show, _ = db.GetShowByName(station.ID, "Test Show")
	if !show.ArchiveCurrentDate.Equal(date2) {
		t.Errorf("expected current date %v, got %v", date2, *show.ArchiveCurrentDate)
	}
	if show.ArchiveCurrentM3UURL != "https://wmbr.org/m3u/test2.m3u" {
		t.Errorf("expected current m3u URL test2.m3u, got %s", show.ArchiveCurrentM3UURL)
	}
	if show.ArchivePreviousDate == nil {
		t.Fatal("expected previous date to be set")
	}
	if !show.ArchivePreviousDate.Equal(date1) {
		t.Errorf("expected previous date %v, got %v", date1, *show.ArchivePreviousDate)
	}
	if show.ArchivePreviousM3UURL != "https://wmbr.org/m3u/test1.m3u" {
		t.Errorf("expected previous m3u URL test1.m3u, got %s", show.ArchivePreviousM3UURL)
	}
}

func TestUpdateShowArchive_SameDate_NoOp(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Test Show"})
	show, _ := db.GetShowByName(station.ID, "Test Show")

	// Add first archive
	date := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)
	db.UpdateShowArchive(show.ID, date, "https://wmbr.org/m3u/test.m3u")

	// Add second archive with different URL but same date
	date2 := time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC)
	db.UpdateShowArchive(show.ID, date2, "https://wmbr.org/m3u/test2.m3u")

	// Now call with same date again - should not rotate
	db.UpdateShowArchive(show.ID, date2, "https://wmbr.org/m3u/test2_updated.m3u")

	// Verify no rotation happened (previous should still be date)
	show, _ = db.GetShowByName(station.ID, "Test Show")
	if show.ArchivePreviousDate == nil {
		t.Fatal("expected previous date to be set")
	}
	if !show.ArchivePreviousDate.Equal(date) {
		t.Errorf("expected previous date to remain %v, got %v", date, *show.ArchivePreviousDate)
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

	// Update to completed with filename (not full path - paths constructed at runtime)
	err = db.UpdateDownloadStatus(id, StatusCompleted, "WMBR_Show1_20260125.mp3", "")
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	d, _ = db.GetDownload(id)
	if d.Status != StatusCompleted {
		t.Errorf("expected status %q, got %q", StatusCompleted, d.Status)
	}
	if d.Filepath != "WMBR_Show1_20260125.mp3" {
		t.Errorf("expected filepath %q, got %q", "WMBR_Show1_20260125.mp3", d.Filepath)
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
	db.UpdateDownloadStatus(id2, StatusCompleted, "show.mp3", "")
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
	found, err := db.FindDownload(station.ID, &show.ID, archiveDate)
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
	found, err = db.FindDownload(station.ID, &show.ID, archiveDate)
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

func TestFindDownload_DifferentShowsSameDate(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Backwoods", "Aural Fixation"})
	show1, _ := db.GetShowByName(station.ID, "Backwoods")
	show2, _ := db.GetShowByName(station.ID, "Aural Fixation")

	archiveDate := time.Date(2026, 1, 24, 0, 0, 0, 0, time.UTC)

	// Insert download for show1 (Backwoods)
	db.InsertDownload(&Download{
		StationID:   station.ID,
		ShowID:      &show1.ID,
		ArchiveDate: archiveDate,
		M3UURL:      "http://test1.m3u",
		Status:      StatusCompleted,
	})

	// Searching for show1 should find it
	found, err := db.FindDownload(station.ID, &show1.ID, archiveDate)
	if err != nil {
		t.Fatalf("failed to find download: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find download for show1")
	}
	if found.Show != "Backwoods" {
		t.Errorf("expected show name 'Backwoods', got '%s'", found.Show)
	}

	// Searching for show2 (Aural Fixation) should NOT find it
	found, err = db.FindDownload(station.ID, &show2.ID, archiveDate)
	if err != nil {
		t.Fatalf("failed to find download: %v", err)
	}
	if found != nil {
		t.Error("expected nil for show2, but found existing download - this was the bug!")
	}

	// Now insert download for show2
	db.InsertDownload(&Download{
		StationID:   station.ID,
		ShowID:      &show2.ID,
		ArchiveDate: archiveDate,
		M3UURL:      "http://test2.m3u",
		Status:      StatusPending,
	})

	// Now searching for show2 should find it
	found, err = db.FindDownload(station.ID, &show2.ID, archiveDate)
	if err != nil {
		t.Fatalf("failed to find download: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find download for show2")
	}
	if found.Show != "Aural Fixation" {
		t.Errorf("expected show name 'Aural Fixation', got '%s'", found.Show)
	}
}

func TestFindDownload_NilShowID(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	archiveDate := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)

	// Insert download with nil ShowID (legacy record)
	db.InsertDownload(&Download{
		StationID:   station.ID,
		ShowID:      nil,
		ArchiveDate: archiveDate,
		M3UURL:      "http://test.m3u",
	})

	// Searching with nil showID should find it
	found, err := db.FindDownload(station.ID, nil, archiveDate)
	if err != nil {
		t.Fatalf("failed to find download: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find download with nil showID")
	}

	// Searching with a specific showID should NOT find the nil record
	db.CacheShows(station.ID, []string{"SomeShow"})
	show, _ := db.GetShowByName(station.ID, "SomeShow")
	found, err = db.FindDownload(station.ID, &show.ID, archiveDate)
	if err != nil {
		t.Fatalf("failed to find download: %v", err)
	}
	if found != nil {
		t.Error("expected nil when searching for specific show but only nil-show record exists")
	}
}

func TestListShowsWithDownloads(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Show1", "Show2", "Show3"})
	show1, _ := db.GetShowByName(station.ID, "Show1")
	show2, _ := db.GetShowByName(station.ID, "Show2")
	// show3 has no downloads

	// Add downloads only for show1 and show2
	db.InsertDownload(&Download{StationID: station.ID, ShowID: &show1.ID, ArchiveDate: time.Now(), M3UURL: "http://1.m3u"})
	db.InsertDownload(&Download{StationID: station.ID, ShowID: &show2.ID, ArchiveDate: time.Now(), M3UURL: "http://2.m3u"})

	shows, err := db.ListShowsWithDownloads(station.ID)
	if err != nil {
		t.Fatalf("failed to list shows: %v", err)
	}

	if len(shows) != 2 {
		t.Errorf("expected 2 shows with downloads, got %d", len(shows))
	}

	// Verify show3 is not in the list
	for _, s := range shows {
		if s.Name == "Show3" {
			t.Error("Show3 should not be in the list (no downloads)")
		}
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
	db.UpdateDownloadStatus(id1, StatusCompleted, "show1.mp3", "")

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

// Schedule tests

func TestInsertSchedule(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Test Show"})
	show, _ := db.GetShowByName(station.ID, "Test Show")

	nextRun := time.Date(2026, 2, 2, 4, 30, 0, 0, time.UTC)
	s := &Schedule{
		StationID:      station.ID,
		ShowID:         show.ID,
		CronExpression: "30 4 * * 0",
		Enabled:        true,
		NextRunAt:      &nextRun,
	}

	id, err := db.InsertSchedule(s)
	if err != nil {
		t.Fatalf("failed to insert schedule: %v", err)
	}

	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	// Verify we can retrieve it
	got, err := db.GetSchedule(id)
	if err != nil {
		t.Fatalf("failed to get schedule: %v", err)
	}
	if got.CronExpression != "30 4 * * 0" {
		t.Errorf("expected cron '30 4 * * 0', got %q", got.CronExpression)
	}
	if !got.Enabled {
		t.Error("expected schedule to be enabled")
	}
	if got.Station != "WMBR" {
		t.Errorf("expected station 'WMBR', got %q", got.Station)
	}
	if got.Show != "Test Show" {
		t.Errorf("expected show 'Test Show', got %q", got.Show)
	}
}

func TestListSchedules(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Show A", "Show B"})
	showA, _ := db.GetShowByName(station.ID, "Show A")
	showB, _ := db.GetShowByName(station.ID, "Show B")

	// Initially empty
	schedules, err := db.ListSchedules()
	if err != nil {
		t.Fatalf("failed to list schedules: %v", err)
	}
	if len(schedules) != 0 {
		t.Errorf("expected 0 schedules, got %d", len(schedules))
	}

	// Add schedules
	db.InsertSchedule(&Schedule{StationID: station.ID, ShowID: showA.ID, CronExpression: "30 4 * * 0", Enabled: true})
	db.InsertSchedule(&Schedule{StationID: station.ID, ShowID: showB.ID, CronExpression: "0 5 * * 1", Enabled: true})

	schedules, err = db.ListSchedules()
	if err != nil {
		t.Fatalf("failed to list schedules: %v", err)
	}
	if len(schedules) != 2 {
		t.Errorf("expected 2 schedules, got %d", len(schedules))
	}
}

func TestScheduleUniqueConstraint(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Test Show"})
	show, _ := db.GetShowByName(station.ID, "Test Show")

	// Insert first schedule
	_, err = db.InsertSchedule(&Schedule{StationID: station.ID, ShowID: show.ID, CronExpression: "30 4 * * 0", Enabled: true})
	if err != nil {
		t.Fatalf("failed to insert first schedule: %v", err)
	}

	// Duplicate should fail
	_, err = db.InsertSchedule(&Schedule{StationID: station.ID, ShowID: show.ID, CronExpression: "0 5 * * 1", Enabled: true})
	if err == nil {
		t.Error("expected error for duplicate station+show")
	}
}

func TestUpdateScheduleStatus(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Test Show"})
	show, _ := db.GetShowByName(station.ID, "Test Show")

	nextRun := time.Date(2026, 2, 2, 4, 30, 0, 0, time.UTC)
	id, _ := db.InsertSchedule(&Schedule{
		StationID:      station.ID,
		ShowID:         show.ID,
		CronExpression: "30 4 * * 0",
		Enabled:        true,
		NextRunAt:      &nextRun,
	})

	// Update to success
	newNextRun := time.Date(2026, 2, 9, 4, 30, 0, 0, time.UTC)
	err = db.UpdateScheduleStatus(id, ScheduleStatusSuccess, "", &newNextRun, nil, 0)
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	got, _ := db.GetSchedule(id)
	if got.LastStatus != ScheduleStatusSuccess {
		t.Errorf("expected status 'success', got %q", got.LastStatus)
	}
	if got.LastRunAt == nil {
		t.Error("expected last_run_at to be set")
	}
	if got.RetryCount != 0 {
		t.Errorf("expected retry_count=0, got %d", got.RetryCount)
	}
}

func TestUpdateScheduleStatus_Retry(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Test Show"})
	show, _ := db.GetShowByName(station.ID, "Test Show")

	nextRun := time.Date(2026, 2, 2, 4, 30, 0, 0, time.UTC)
	id, _ := db.InsertSchedule(&Schedule{
		StationID:      station.ID,
		ShowID:         show.ID,
		CronExpression: "30 4 * * 0",
		Enabled:        true,
		NextRunAt:      &nextRun,
	})

	// Update to retrying
	nextRetry := time.Now().Add(1 * time.Minute)
	err = db.UpdateScheduleStatus(id, ScheduleStatusRetrying, "connection timeout", &nextRun, &nextRetry, 1)
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	got, _ := db.GetSchedule(id)
	if got.LastStatus != ScheduleStatusRetrying {
		t.Errorf("expected status 'retrying', got %q", got.LastStatus)
	}
	if got.LastError != "connection timeout" {
		t.Errorf("expected error 'connection timeout', got %q", got.LastError)
	}
	if got.RetryCount != 1 {
		t.Errorf("expected retry_count=1, got %d", got.RetryCount)
	}
	if got.NextRetryAt == nil {
		t.Error("expected next_retry_at to be set")
	}
}

func TestUpdateScheduleEnabled(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Test Show"})
	show, _ := db.GetShowByName(station.ID, "Test Show")

	id, _ := db.InsertSchedule(&Schedule{
		StationID:      station.ID,
		ShowID:         show.ID,
		CronExpression: "30 4 * * 0",
		Enabled:        true,
	})

	// Disable
	err = db.UpdateScheduleEnabled(id, false)
	if err != nil {
		t.Fatalf("failed to disable: %v", err)
	}

	got, _ := db.GetSchedule(id)
	if got.Enabled {
		t.Error("expected disabled")
	}

	// Re-enable
	err = db.UpdateScheduleEnabled(id, true)
	if err != nil {
		t.Fatalf("failed to enable: %v", err)
	}

	got, _ = db.GetSchedule(id)
	if !got.Enabled {
		t.Error("expected enabled")
	}
}

func TestDeleteSchedule(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Test Show"})
	show, _ := db.GetShowByName(station.ID, "Test Show")

	id, _ := db.InsertSchedule(&Schedule{
		StationID:      station.ID,
		ShowID:         show.ID,
		CronExpression: "30 4 * * 0",
		Enabled:        true,
	})

	err = db.DeleteSchedule(id)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Verify it's gone
	_, err = db.GetSchedule(id)
	if err == nil {
		t.Error("expected error for deleted schedule")
	}
}

func TestListDueSchedules(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Show A", "Show B", "Show C"})
	showA, _ := db.GetShowByName(station.ID, "Show A")
	showB, _ := db.GetShowByName(station.ID, "Show B")
	showC, _ := db.GetShowByName(station.ID, "Show C")

	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	// Due (past next_run_at)
	db.InsertSchedule(&Schedule{StationID: station.ID, ShowID: showA.ID, CronExpression: "30 4 * * 0", Enabled: true, NextRunAt: &past})
	// Not due (future next_run_at)
	db.InsertSchedule(&Schedule{StationID: station.ID, ShowID: showB.ID, CronExpression: "0 5 * * 1", Enabled: true, NextRunAt: &future})
	// Disabled (should not be returned)
	db.InsertSchedule(&Schedule{StationID: station.ID, ShowID: showC.ID, CronExpression: "0 6 * * 2", Enabled: false, NextRunAt: &past})

	due, err := db.ListDueSchedules(now)
	if err != nil {
		t.Fatalf("failed to list due: %v", err)
	}

	if len(due) != 1 {
		t.Errorf("expected 1 due schedule, got %d", len(due))
	}
	if len(due) > 0 && due[0].Show != "Show A" {
		t.Errorf("expected 'Show A', got %q", due[0].Show)
	}
}

func TestFindScheduleByShowID(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	station, _ := db.GetOrCreateStation("WMBR", "", "")
	db.CacheShows(station.ID, []string{"Show A", "Show B"})
	showA, _ := db.GetShowByName(station.ID, "Show A")
	showB, _ := db.GetShowByName(station.ID, "Show B")

	db.InsertSchedule(&Schedule{StationID: station.ID, ShowID: showA.ID, CronExpression: "30 4 * * 0", Enabled: true})

	// Find existing
	found, err := db.FindScheduleByShowID(station.ID, showA.ID)
	if err != nil {
		t.Fatalf("failed to find: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find schedule")
	}
	if found.Show != "Show A" {
		t.Errorf("expected 'Show A', got %q", found.Show)
	}

	// Not found
	found, err = db.FindScheduleByShowID(station.ID, showB.ID)
	if err != nil {
		t.Fatalf("failed to find: %v", err)
	}
	if found != nil {
		t.Error("expected nil for non-existent schedule")
	}
}
