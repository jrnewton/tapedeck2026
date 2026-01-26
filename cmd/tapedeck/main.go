package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jnewton/tapedeck/internal/api"
	"github.com/jnewton/tapedeck/internal/db"

	// Register adapters
	_ "github.com/jnewton/tapedeck/internal/adapters/wmbr"
)

//go:embed web/*
var webFiles embed.FS

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

	// Serve static files from embedded filesystem
	webFS, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatalf("failed to create web filesystem: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	log.Printf("Starting server on :%s", port)
	log.Printf("Data directory: %s", dataDir)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
