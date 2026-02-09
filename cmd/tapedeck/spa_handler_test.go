package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSPAHandler(t *testing.T) {
	// Create a mock filesystem with files and a subdirectory
	mockFS := fstest.MapFS{
		"index.html":          &fstest.MapFile{Data: []byte("<html>index</html>")},
		"app.js":              &fstest.MapFile{Data: []byte("console.log('app')")},
		"style.css":           &fstest.MapFile{Data: []byte("body { margin: 0; }")},
		"favicon.png":         &fstest.MapFile{Data: []byte("PNG")},
		"static/tos.html":     &fstest.MapFile{Data: []byte("<html>tos</html>")},
		"static/privacy.html": &fstest.MapFile{Data: []byte("<html>privacy</html>")},
	}

	handler := spaHandler(mockFS)
	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		// Root and query-param SPA routes → 200 index.html
		{
			name:       "root serves index.html",
			path:       "/",
			wantStatus: http.StatusOK,
			wantBody:   "<html>index</html>",
		},
		{
			name:       "query params on root serve index.html",
			path:       "/?page=downloads",
			wantStatus: http.StatusOK,
			wantBody:   "<html>index</html>",
		},
		{
			name:       "station query params serve index.html",
			path:       "/?station=WMBR&show=42",
			wantStatus: http.StatusOK,
			wantBody:   "<html>index</html>",
		},
		// Real files → 200
		{
			name:       "existing JS file is served",
			path:       "/app.js",
			wantStatus: http.StatusOK,
			wantBody:   "console.log('app')",
		},
		{
			name:       "existing CSS file is served",
			path:       "/style.css",
			wantStatus: http.StatusOK,
			wantBody:   "body { margin: 0; }",
		},
		{
			name:       "existing image is served",
			path:       "/favicon.png",
			wantStatus: http.StatusOK,
			wantBody:   "PNG",
		},
		{
			name:       "static subdirectory file is served",
			path:       "/static/tos.html",
			wantStatus: http.StatusOK,
			wantBody:   "<html>tos</html>",
		},
		{
			name:       "static privacy page is served",
			path:       "/static/privacy.html",
			wantStatus: http.StatusOK,
			wantBody:   "<html>privacy</html>",
		},
		// Directory listing → 403
		{
			name:       "directory path returns 403",
			path:       "/static/",
			wantStatus: http.StatusForbidden,
		},
		// Missing files → 404
		{
			name:       "missing file with extension returns 404",
			path:       "/nonexistent.js",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing file in static dir returns 404",
			path:       "/static/nonexistent.html",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "arbitrary path returns 404",
			path:       "/totally/bogus/path",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "deep nonexistent path returns 404",
			path:       "/some/spa/route",
			wantStatus: http.StatusNotFound,
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

			if tt.wantBody != "" {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("failed to read body: %v", err)
				}
				if string(body) != tt.wantBody {
					t.Errorf("got body %q, want %q", string(body), tt.wantBody)
				}
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
		path      string
		wantCache string
	}{
		{"/app.js", "public, max-age=31536000, immutable"},
		{"/style.css", "public, max-age=31536000, immutable"},
		{"/", "no-cache, no-store, must-revalidate"},
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
