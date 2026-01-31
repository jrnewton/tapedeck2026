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
// filesystem, falling back to index.html for paths that don't match a file.
// This enables client-side routing for the single-page application.
func spaHandler(webFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(webFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file directly
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if file exists (strip leading slash for fs.Stat)
		if _, err := fs.Stat(webFS, path[1:]); err == nil {
			setCacheHeaders(w, path)
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA routing
		setCacheHeaders(w, "/index.html")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

// setCacheHeaders sets appropriate Cache-Control headers based on file type.
func setCacheHeaders(w http.ResponseWriter, path string) {
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
