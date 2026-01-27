package main

import (
	"os"
	"testing"
)

func TestScheduleDownload_MissingArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no args", []string{}},
		{"only station", []string{"WMBR"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := cmdScheduleDownload(tc.args)
			if err == nil {
				t.Error("expected error for missing arguments")
			}
		})
	}
}

func TestFormatCronTime(t *testing.T) {
	tests := []struct {
		cron     string
		expected string
	}{
		{"30 4 * * 0", "04:30"},
		{"0 22 * * 2", "22:00"},
		{"5 9 * * 1", "09:05"},
		{"invalid", "invalid"},
	}

	for _, tc := range tests {
		t.Run(tc.cron, func(t *testing.T) {
			got := formatCronTime(tc.cron)
			if got != tc.expected {
				t.Errorf("formatCronTime(%q) = %q, want %q", tc.cron, got, tc.expected)
			}
		})
	}
}

func TestDescribeCron(t *testing.T) {
	tests := []struct {
		cron     string
		expected string
	}{
		{"30 4 * * 0", "Sundays at 04:30"},
		{"0 22 * * 2", "Tuesdays at 22:00"},
		{"30 12 * * 6", "Saturdays at 12:30"},
		{"5 9 * * 1", "Mondays at 09:05"},
		{"0 0 * * *", "Every day at 00:00"},
		{"invalid", "invalid"},
	}

	for _, tc := range tests {
		t.Run(tc.cron, func(t *testing.T) {
			got := describeCron(tc.cron)
			if got != tc.expected {
				t.Errorf("describeCron(%q) = %q, want %q", tc.cron, got, tc.expected)
			}
		})
	}
}

func TestListDownloads_UnknownStation(t *testing.T) {
	err := cmdListDownloads([]string{"XKJF"})
	if err == nil {
		t.Error("expected error for unknown station")
	}
	if err.Error() != "unknown station: XKJF" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDownloadShow_MissingArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no args", []string{}},
		{"only station", []string{"WMBR"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := cmdDownloadShow(tc.args)
			if err == nil {
				t.Error("expected error for missing arguments")
			}
		})
	}
}

func TestDownloadShow_UnknownStation(t *testing.T) {
	err := cmdDownloadShow([]string{"XKJF", "SomeShow"})
	if err == nil {
		t.Error("expected error for unknown station")
	}
}

func TestServerURLFromEnv(t *testing.T) {
	// Test default
	os.Unsetenv("TAPEDECK_SERVER_URL")
	url := os.Getenv("TAPEDECK_SERVER_URL")
	if url == "" {
		url = "http://localhost:8080"
	}
	if url != "http://localhost:8080" {
		t.Errorf("expected default URL, got %q", url)
	}

	// Test custom
	os.Setenv("TAPEDECK_SERVER_URL", "http://custom:9000")
	defer os.Unsetenv("TAPEDECK_SERVER_URL")
	url = os.Getenv("TAPEDECK_SERVER_URL")
	if url == "" {
		url = "http://localhost:8080"
	}
	if url != "http://custom:9000" {
		t.Errorf("expected custom URL, got %q", url)
	}
}
