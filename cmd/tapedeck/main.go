package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"local/tapedeck/internal/api"
	"local/tapedeck/internal/db"

	// Register adapters
	_ "local/tapedeck/internal/adapters/wmbr"
)

//go:embed web/*
var webFiles embed.FS

// spaHandler returns an HTTP handler that serves static files from the embedded
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
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("TAPEDECK_PORT")
	if port == "" {
		port = "8080"
	}

	dataDir := os.Getenv("TAPEDECK_DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
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

	// Create API server
	apiServer := api.NewServer(database, downloadsDir)

	// Register routes
	mux := http.NewServeMux()
	apiServer.RegisterRoutes(mux)

	// Serve static files from embedded filesystem with SPA fallback
	webFS, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatalf("failed to create web filesystem: %v", err)
	}
	mux.Handle("/", spaHandler(webFS))

	log.Printf("Starting server on :%s", port)
	log.Printf("Data directory: %s", dataDir)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
