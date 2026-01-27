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
