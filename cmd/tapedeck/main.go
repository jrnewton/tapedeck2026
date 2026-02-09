package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"local/tapedeck/internal/api"
	"local/tapedeck/internal/db"
	"local/tapedeck/internal/scheduler"

	// Register adapters
	_ "local/tapedeck/internal/adapters/wmbr"
)

// spaHandler returns an HTTP handler that serves static files from a
// filesystem. The root path "/" serves index.html (the SPA uses query-param
// routing). Real directories return 403 (no directory listing). Missing
// files return 404.
func spaHandler(webFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(webFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Root path serves index.html (SPA uses query-param routing)
		if path == "/" {
			setCacheHeaders(w, "/index.html")
			fileServer.ServeHTTP(w, r)
			return
		}

		// Strip leading slash and trailing slash for fs operations
		fsPath := strings.TrimSuffix(path[1:], "/")

		// Check if path is a real directory — return 403 (no listing)
		if info, err := fs.Stat(webFS, fsPath); err == nil && info.IsDir() {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}

		// Check if path is a real file — serve it
		if info, err := fs.Stat(webFS, fsPath); err == nil && !info.IsDir() {
			setCacheHeaders(w, path)
			fileServer.ServeHTTP(w, r)
			return
		}

		// Everything else is 404
		http.NotFound(w, r)
	})
}

// setCacheHeaders sets appropriate Cache-Control headers based on file type.
func setCacheHeaders(w http.ResponseWriter, path string) {
	// Service worker must never be aggressively cached — browser needs to
	// fetch a fresh copy to detect version changes and trigger updates.
	if strings.HasSuffix(path, "sw.js") {
		w.Header().Set("Cache-Control", "no-cache")
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".css":
		// Static assets: cache for 1 year (SW version handles updates)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".webp", ".woff", ".woff2":
		// Images and fonts: cache for 1 year
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case ".html":
		// HTML: no cache to ensure fresh content
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	case ".json":
		// Manifest: short cache
		w.Header().Set("Cache-Control", "public, max-age=3600")
	default:
		// Default: short cache
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
}

func main() {
	port := os.Getenv("TAPEDECK_PORT")
	if port == "" {
		port = "8080"
	}

	dataDir := os.Getenv("TAPEDECK_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	downloadsDir := filepath.Join(dataDir, "downloads")
	if err := os.MkdirAll(downloadsDir, 0755); err != nil {
		log.Fatalf("failed to create downloads directory: %v", err)
	}

	// Open database
	dbPath := filepath.Join(dataDir, "tapedeck.db")
	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Configure and start scheduler
	schedulerCfg := scheduler.DefaultConfig()
	if maxConcurrent := os.Getenv("TAPEDECK_MAX_CONCURRENT"); maxConcurrent != "" {
		if n, err := strconv.Atoi(maxConcurrent); err == nil && n > 0 {
			schedulerCfg.MaxConcurrent = n
		}
	}
	sched := scheduler.New(database, downloadsDir, schedulerCfg)
	sched.Start()
	defer sched.Stop()

	// Create API server
	apiServer := api.NewServer(database, downloadsDir)
	apiServer.Scheduler = sched

	// Register routes
	mux := http.NewServeMux()
	apiServer.RegisterRoutes(mux)

	// Serve static files from filesystem with SPA fallback
	webFS := os.DirFS("./web")
	mux.Handle("/", spaHandler(webFS))

	log.Printf("Starting server on :%s", port)
	log.Printf("Data directory: %s", dataDir)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
