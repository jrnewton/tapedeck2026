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

func TestInsertDownload(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	d := &Download{
		Station:  "WMBR",
		Show:     "Lost and Found",
		Date:     time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC),
		Filepath: "/data/downloads/WMBR_Lost_and_Found_20260125.mp3",
		Status:   "completed",
	}

	id, err := db.InsertDownload(d)
	if err != nil {
		t.Fatalf("failed to insert download: %v", err)
	}

	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}
}

func TestListDownloads_All(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Insert test data
	downloads := []*Download{
		{Station: "WMBR", Show: "Show1", Date: time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC), Filepath: "/path1.mp3"},
		{Station: "WHRB", Show: "Show2", Date: time.Date(2026, 1, 24, 0, 0, 0, 0, time.UTC), Filepath: "/path2.mp3"},
		{Station: "WMBR", Show: "Show3", Date: time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC), Filepath: "/path3.mp3"},
	}

	for _, d := range downloads {
		if _, err := db.InsertDownload(d); err != nil {
			t.Fatalf("failed to insert download: %v", err)
		}
	}

	// List all
	all, err := db.ListDownloads("")
	if err != nil {
		t.Fatalf("failed to list downloads: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("expected 3 downloads, got %d", len(all))
	}
}

func TestListDownloads_ByStation(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Insert test data
	downloads := []*Download{
		{Station: "WMBR", Show: "Show1", Date: time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC), Filepath: "/path1.mp3"},
		{Station: "WHRB", Show: "Show2", Date: time.Date(2026, 1, 24, 0, 0, 0, 0, time.UTC), Filepath: "/path2.mp3"},
		{Station: "WMBR", Show: "Show3", Date: time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC), Filepath: "/path3.mp3"},
	}

	for _, d := range downloads {
		if _, err := db.InsertDownload(d); err != nil {
			t.Fatalf("failed to insert download: %v", err)
		}
	}

	// List by station
	wmbr, err := db.ListDownloads("WMBR")
	if err != nil {
		t.Fatalf("failed to list downloads: %v", err)
	}

	if len(wmbr) != 2 {
		t.Errorf("expected 2 WMBR downloads, got %d", len(wmbr))
	}

	for _, d := range wmbr {
		if d.Station != "WMBR" {
			t.Errorf("expected station WMBR, got %s", d.Station)
		}
	}
}

func TestGetDownload(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	d := &Download{
		Station:  "WMBR",
		Show:     "Test Show",
		Date:     time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC),
		Filepath: "/test/path.mp3",
		Status:   "completed",
	}

	id, err := db.InsertDownload(d)
	if err != nil {
		t.Fatalf("failed to insert download: %v", err)
	}

	// Get the download
	got, err := db.GetDownload(id)
	if err != nil {
		t.Fatalf("failed to get download: %v", err)
	}

	if got.Station != d.Station {
		t.Errorf("station: expected %s, got %s", d.Station, got.Station)
	}
	if got.Show != d.Show {
		t.Errorf("show: expected %s, got %s", d.Show, got.Show)
	}
	if got.Filepath != d.Filepath {
		t.Errorf("filepath: expected %s, got %s", d.Filepath, got.Filepath)
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
