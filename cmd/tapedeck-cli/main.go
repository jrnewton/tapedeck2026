package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jnewton/tapedeck/internal/db"
	"github.com/jnewton/tapedeck/pkg/tapedeck"

	// Register adapters
	_ "github.com/jnewton/tapedeck/internal/adapters/wmbr"
)

const (
	defaultDataDir = "./data"
	dbFilename     = "tapedeck.db"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "list-shows":
		err = cmdListShows(args)
	case "list-downloads":
		err = cmdListDownloads(args)
	case "download-show":
		err = cmdDownloadShow(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`tapedeck-cli - Download radio archive streams

Usage:
  tapedeck-cli <command> [arguments]

Commands:
  list-shows <STATION>                         List available shows from station archive
  list-downloads [STATION]                     List downloaded files from database
  download-show <STATION> <SHOW> [options]     Download archive and record in DB

Options for download-show:
  --latest            Download the most recent archive (default)
  --date YYYYMMDD     Download archive from specific date
  --output DIR        Output directory (default: ./data/downloads)

Supported Stations:
  WMBR

Examples:
  tapedeck-cli list-shows WMBR
  tapedeck-cli download-show WMBR "Lost and Found" --latest
  tapedeck-cli download-show WMBR Backwoods --date 20260120
  tapedeck-cli list-downloads
  tapedeck-cli list-downloads WMBR`)
}

func cmdListShows(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: list-shows <STATION>")
	}

	station := strings.ToUpper(args[0])
	adapter, err := tapedeck.GetAdapter(station)
	if err != nil {
		return err
	}

	shows, err := adapter.ListShows()
	if err != nil {
		return fmt.Errorf("list shows: %w", err)
	}

	fmt.Printf("Shows available on %s (%d):\n", station, len(shows))
	for _, show := range shows {
		fmt.Printf("  %s\n", show)
	}

	return nil
}

func cmdListDownloads(args []string) error {
	var station string
	if len(args) > 0 {
		station = strings.ToUpper(args[0])
	}

	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	downloads, err := database.ListDownloads(station)
	if err != nil {
		return fmt.Errorf("list downloads: %w", err)
	}

	if len(downloads) == 0 {
		if station != "" {
			fmt.Printf("No downloads found for station %s\n", station)
		} else {
			fmt.Println("No downloads found")
		}
		return nil
	}

	fmt.Printf("Downloads (%d):\n", len(downloads))
	for _, d := range downloads {
		fmt.Printf("  [%s] %s - %s (%s)\n", d.Station, d.Show, d.Date.Format("2006-01-02"), d.Filepath)
	}

	return nil
}

func cmdDownloadShow(args []string) error {
	fs := flag.NewFlagSet("download-show", flag.ExitOnError)
	latest := fs.Bool("latest", false, "Download most recent archive")
	date := fs.String("date", "", "Download archive from specific date (YYYYMMDD)")
	output := fs.String("output", "", "Output directory")

	if len(args) < 2 {
		return fmt.Errorf("usage: download-show <STATION> <SHOW> [--latest | --date YYYYMMDD] [--output DIR]")
	}

	station := strings.ToUpper(args[0])
	show := args[1]

	if err := fs.Parse(args[2:]); err != nil {
		return err
	}

	// Default to latest if no date specified
	if !*latest && *date == "" {
		*latest = true
	}

	// Set output directory
	outputDir := *output
	if outputDir == "" {
		outputDir = filepath.Join(defaultDataDir, "downloads")
	}

	adapter, err := tapedeck.GetAdapter(station)
	if err != nil {
		return err
	}

	var archive *tapedeck.Archive

	if *latest {
		archive, err = adapter.GetLatestArchive(show)
		if err != nil {
			return fmt.Errorf("get latest archive: %w", err)
		}
	} else {
		// Parse date and find matching archive
		targetDate, err := time.Parse("20060102", *date)
		if err != nil {
			return fmt.Errorf("invalid date format (use YYYYMMDD): %w", err)
		}

		archives, err := adapter.ListArchives(show)
		if err != nil {
			return fmt.Errorf("list archives: %w", err)
		}

		for i, a := range archives {
			if a.Date.Year() == targetDate.Year() &&
				a.Date.Month() == targetDate.Month() &&
				a.Date.Day() == targetDate.Day() {
				archive = &archives[i]
				break
			}
		}

		if archive == nil {
			return fmt.Errorf("no archive found for %s on %s", show, *date)
		}
	}

	fmt.Printf("Downloading %s - %s (%s)...\n", station, archive.ShowName, archive.Date.Format("2006-01-02"))

	filepath, err := adapter.DownloadArchive(archive, outputDir)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}

	fmt.Printf("Downloaded to: %s\n", filepath)

	// Record in database
	database, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not record download in database: %v\n", err)
		return nil
	}
	defer database.Close()

	_, err = database.InsertDownload(&db.Download{
		Station:  station,
		Show:     archive.ShowName,
		Date:     archive.Date,
		Filepath: filepath,
		Status:   "completed",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not record download in database: %v\n", err)
	}

	return nil
}

func openDB() (*db.DB, error) {
	dbPath := filepath.Join(defaultDataDir, dbFilename)
	if err := os.MkdirAll(defaultDataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	return db.Open(dbPath)
}
