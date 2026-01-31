package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSPAHandler(t *testing.T) {
	// Create a mock filesystem with some files
	mockFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>index</html>")},
		"app.js":     &fstest.MapFile{Data: []byte("console.log('app')")},
		"style.css":  &fstest.MapFile{Data: []byte("body { margin: 0; }")},
	}

	handler := spaHandler(mockFS)
	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name         string
		path         string
		wantStatus   int
		wantContains string
	}{
		{
			name:         "root path returns index.html",
			path:         "/",
			wantStatus:   http.StatusOK,
			wantContains: "<html>index</html>",
		},
		{
			name:         "existing JS file is served",
			path:         "/app.js",
			wantStatus:   http.StatusOK,
			wantContains: "console.log('app')",
		},
		{
			name:         "existing CSS file is served",
			path:         "/style.css",
			wantStatus:   http.StatusOK,
			wantContains: "body { margin: 0; }",
		},
		{
			name:         "non-existent path falls back to index.html",
			path:         "/some/spa/route",
			wantStatus:   http.StatusOK,
			wantContains: "<html>index</html>",
		},
		{
			name:         "query parameters work with fallback",
			path:         "/?station=WMBR&show=42",
			wantStatus:   http.StatusOK,
			wantContains: "<html>index</html>",
		},
		{
			name:         "non-existent file falls back to index.html",
			path:         "/nonexistent.js",
			wantStatus:   http.StatusOK,
			wantContains: "<html>index</html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read body: %v", err)
			}

			if string(body) != tt.wantContains {
				t.Errorf("got body %q, want %q", string(body), tt.wantContains)
			}
		})
	}
}

func TestCacheHeaders(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>index</html>")},
		"app.js":     &fstest.MapFile{Data: []byte("console.log('app')")},
		"style.css":  &fstest.MapFile{Data: []byte("body { margin: 0; }")},
	}

	handler := spaHandler(mockFS)
	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		path        string
		wantCache   string
	}{
		{"/app.js", "public, max-age=31536000, immutable"},
		{"/style.css", "public, max-age=31536000, immutable"},
		{"/", "no-cache, no-store, must-revalidate"},
		{"/nonexistent", "no-cache, no-store, must-revalidate"}, // fallback to index.html
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := http.Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			got := resp.Header.Get("Cache-Control")
			if got != tt.wantCache {
				t.Errorf("Cache-Control for %s: got %q, want %q", tt.path, got, tt.wantCache)
			}
		})
	}
}
