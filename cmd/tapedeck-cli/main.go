package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	case "download-status":
		err = cmdDownloadStatus(args)
	case "schedule-download":
		err = cmdScheduleDownload(args)
	case "fix-downloads":
		err = cmdFixDownloads(args)
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
  download-show <STATION> <SHOW> [options]     Queue archive download (returns ID)
  download-status [ID]                         Show download status (all pending if no ID)
  schedule-download <STATION> <SHOW>           Generate crontab line for automated downloads

Options for download-show:
  --latest            Download the most recent archive (default)
  --date YYYYMMDD     Download archive from specific date
  --output DIR        Output directory (default: ./data/downloads)

Supported Stations:
  WMBR

Examples:
  tapedeck-cli list-shows WMBR
  tapedeck-cli download-show WMBR "Lost and Found" --latest
  tapedeck-cli download-status 42
  tapedeck-cli download-status
  tapedeck-cli list-downloads WMBR
  tapedeck-cli schedule-download WMBR Backwoods`)
}

func cmdListShows(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: list-shows <STATION>")
	}

	callSign := strings.ToUpper(args[0])

	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	// Get or create station
	station, err := database.GetOrCreateStation(callSign, "", "")
	if err != nil {
		return err
	}

	// Check cache first
	cachedShows, valid, err := database.GetCachedShows(station.ID)
	if err != nil {
		return err
	}

	if valid && len(cachedShows) > 0 {
		fmt.Printf("Shows available on %s (%d) [cached]:\n", callSign, len(cachedShows))
		for _, show := range cachedShows {
			fmt.Printf("  %s\n", show.Name)
		}
		return nil
	}

	// Fetch from adapter
	adapter, err := tapedeck.GetAdapter(callSign)
	if err != nil {
		return err
	}

	shows, err := adapter.ListShows()
	if err != nil {
		return fmt.Errorf("list shows: %w", err)
	}

	// Cache the results
	if err := database.CacheShows(station.ID, shows); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not cache shows: %v\n", err)
	}

	fmt.Printf("Shows available on %s (%d):\n", callSign, len(shows))
	for _, show := range shows {
		fmt.Printf("  %s\n", show)
	}

	return nil
}

func cmdListDownloads(args []string) error {
	var callSign string
	if len(args) > 0 {
		callSign = strings.ToUpper(args[0])
	}

	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	downloads, err := database.ListDownloads(callSign)
	if err != nil {
		return fmt.Errorf("list downloads: %w", err)
	}

	if len(downloads) == 0 {
		if callSign != "" {
			fmt.Printf("No downloads found for station %s\n", callSign)
		} else {
			fmt.Println("No downloads found")
		}
		return nil
	}

	fmt.Printf("Downloads (%d):\n", len(downloads))
	for _, d := range downloads {
		statusStr := formatStatus(d.Status)
		if d.Filepath != "" {
			fmt.Printf("  [%d] %s - %s (%s) - %s - %s\n", d.ID, d.Station, d.Show, d.ArchiveDate.Format("2006-01-02"), statusStr, d.Filepath)
		} else {
			fmt.Printf("  [%d] %s - %s (%s) - %s\n", d.ID, d.Station, d.Show, d.ArchiveDate.Format("2006-01-02"), statusStr)
		}
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

	callSign := strings.ToUpper(args[0])
	showName := args[1]

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

	adapter, err := tapedeck.GetAdapter(callSign)
	if err != nil {
		return err
	}

	var archive *tapedeck.Archive

	if *latest {
		archive, err = adapter.GetLatestArchive(showName)
		if err != nil {
			return fmt.Errorf("get latest archive: %w", err)
		}
	} else {
		// Parse date and find matching archive
		targetDate, err := time.Parse("20060102", *date)
		if err != nil {
			return fmt.Errorf("invalid date format (use YYYYMMDD): %w", err)
		}

		archives, err := adapter.ListArchives(showName)
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
			return fmt.Errorf("no archive found for %s on %s", showName, *date)
		}
	}

	// Open database and create download record
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	// Get or create station
	station, err := database.GetOrCreateStation(callSign, "", "")
	if err != nil {
		return err
	}

	// Ensure shows are cached, then get show ID
	var showID *int64
	shows, valid, _ := database.GetCachedShows(station.ID)
	if !valid || len(shows) == 0 {
		// Cache shows from adapter
		showNames, err := adapter.ListShows()
		if err == nil {
			database.CacheShows(station.ID, showNames)
		}
	}
	show, err := database.GetShowByName(station.ID, archive.ShowName)
	if err == nil && show != nil {
		showID = &show.ID
	}

	// Check for existing download of same show/date
	existing, err := database.FindDownload(station.ID, archive.Date)
	if err != nil {
		return fmt.Errorf("check existing download: %w", err)
	}
	if existing != nil {
		fmt.Printf("Download already exists: ID %d (%s)\n", existing.ID, existing.Status)
		if existing.Status == db.StatusCompleted && existing.Filepath != "" {
			fmt.Printf("  File: %s\n", existing.Filepath)
		}
		return nil
	}

	// Insert pending download record
	downloadID, err := database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      showID,
		ArchiveDate: archive.Date,
		M3UURL:      archive.M3UURL,
		Status:      db.StatusPending,
	})
	if err != nil {
		return fmt.Errorf("create download record: %w", err)
	}

	fmt.Printf("Download started: ID %d\n", downloadID)
	fmt.Printf("  %s - %s (%s)\n", callSign, archive.ShowName, archive.Date.Format("2006-01-02"))

	// Run download synchronously
	runDownload(downloadID, adapter, archive, outputDir)

	// Show final status
	d, err := database.GetDownload(downloadID)
	if err != nil {
		return fmt.Errorf("get download status: %w", err)
	}

	if d.Status == db.StatusCompleted {
		fmt.Printf("Download completed: %s\n", d.Filepath)
	} else if d.Status == db.StatusFailed {
		fmt.Printf("Download failed: %s\n", d.Error)
	}

	return nil
}

func runDownload(downloadID int64, adapter tapedeck.Adapter, archive *tapedeck.Archive, outputDir string) {
	database, err := openDB()
	if err != nil {
		return
	}
	defer database.Close()

	// Update status to downloading
	database.UpdateDownloadStatus(downloadID, db.StatusDownloading, "", "")

	// Perform the download
	destPath, err := adapter.DownloadArchive(archive, outputDir)
	if err != nil {
		database.UpdateDownloadStatus(downloadID, db.StatusFailed, "", err.Error())
		return
	}

	// Update status to completed
	database.UpdateDownloadStatus(downloadID, db.StatusCompleted, destPath, "")
}

func cmdDownloadStatus(args []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	// If ID provided, show specific download
	if len(args) > 0 {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid download ID: %s", args[0])
		}

		d, err := database.GetDownload(id)
		if err != nil {
			return err
		}

		printDownloadDetail(d)
		return nil
	}

	// Show all pending/downloading
	downloads, err := database.ListDownloadsByStatus(db.StatusPending, db.StatusDownloading)
	if err != nil {
		return err
	}

	if len(downloads) == 0 {
		fmt.Println("No pending or in-progress downloads")
		return nil
	}

	fmt.Printf("Pending/In-Progress Downloads (%d):\n", len(downloads))
	for _, d := range downloads {
		fmt.Printf("  [%d] %s - %s (%s) - %s\n",
			d.ID, d.Station, d.Show, d.ArchiveDate.Format("2006-01-02"), formatStatus(d.Status))
	}

	return nil
}

func printDownloadDetail(d *db.Download) {
	fmt.Printf("[%d] %s - %s (%s)\n", d.ID, d.Station, d.Show, d.ArchiveDate.Format("2006-01-02"))
	fmt.Printf("  Status:  %s\n", formatStatus(d.Status))
	fmt.Printf("  Started: %s\n", d.CreatedAt.Format("2006-01-02 15:04:05"))

	if !d.UpdatedAt.IsZero() {
		fmt.Printf("  Updated: %s\n", d.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	if d.Filepath != "" {
		fmt.Printf("  File:    %s\n", d.Filepath)
	}

	if d.Error != "" {
		fmt.Printf("  Error:   %s\n", d.Error)
	}
}

func formatStatus(status string) string {
	switch status {
	case db.StatusPending:
		return "pending"
	case db.StatusDownloading:
		return "downloading"
	case db.StatusCompleted:
		return "completed"
	case db.StatusFailed:
		return "failed"
	default:
		return status
	}
}

func openDB() (*db.DB, error) {
	dataDir := os.Getenv("TAPEDECK_DATA_DIR")
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	dbPath := filepath.Join(dataDir, dbFilename)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	return db.Open(dbPath)
}

func cmdScheduleDownload(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: schedule-download <STATION> <SHOW>")
	}

	callSign := strings.ToUpper(args[0])
	showName := args[1]

	adapter, err := tapedeck.GetAdapter(callSign)
	if err != nil {
		return err
	}

	schedule, err := adapter.GetShowSchedule(showName)
	if err != nil {
		return fmt.Errorf("get schedule: %w", err)
	}

	// Format the show name for the crontab command
	quotedShow := showName
	if strings.Contains(showName, " ") {
		quotedShow = fmt.Sprintf("%q", showName)
	}

	// Build the cron line
	cronLine := fmt.Sprintf("%s docker exec tapedeck tapedeck-cli download-show %s %s --latest",
		schedule.RecommendedCron, callSign, quotedShow)

	// Output the crontab line with comments
	fmt.Printf("# %s on %s\n", showName, callSign)
	fmt.Printf("# Airs: %s at %s\n", schedule.DayOfWeek, schedule.StartTime)
	fmt.Printf("# Confidence: %s\n", schedule.Confidence)
	if schedule.Notes != "" {
		fmt.Printf("# Note: %s\n", schedule.Notes)
	}
	fmt.Printf("%s\n", cronLine)
	fmt.Printf("\n# To install:\n")
	fmt.Printf("(crontab -l 2>/dev/null; echo '%s') | crontab -\n", cronLine)

	return nil
}

func cmdFixDownloads(args []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer database.Close()

	// Get all downloads
	downloads, err := database.ListDownloads("")
	if err != nil {
		return err
	}

	fixed := 0
	for _, d := range downloads {
		// Skip if already has a show_id
		if d.ShowID != nil {
			continue
		}

		// Try to extract show name from M3U URL or filepath
		// URL format: https://wmbr.org/m3u/ShowName_YYYYMMDD_HHMM.m3u
		// Filepath format: data/downloads/STATION_ShowName_YYYYMMDD.mp3
		showName := extractShowName(d.M3UURL, d.Filepath)
		if showName == "" {
			fmt.Printf("Could not extract show name for download %d\n", d.ID)
			continue
		}

		// Ensure shows are cached for this station
		station, err := database.GetStation(d.Station)
		if err != nil {
			continue
		}

		shows, valid, _ := database.GetCachedShows(station.ID)
		if !valid || len(shows) == 0 {
			adapter, err := tapedeck.GetAdapter(d.Station)
			if err == nil {
				showNames, err := adapter.ListShows()
				if err == nil {
					database.CacheShows(station.ID, showNames)
				}
			}
		}

		// Find the show
		show, err := database.GetShowByName(station.ID, showName)
		if err != nil || show == nil {
			fmt.Printf("Show not found for download %d: %s\n", d.ID, showName)
			continue
		}

		// Link the download to the show
		err = database.LinkDownloadToShow(d.ID, show.ID)
		if err != nil {
			fmt.Printf("Failed to link download %d to show %s: %v\n", d.ID, showName, err)
			continue
		}

		fmt.Printf("Fixed download %d: linked to show '%s'\n", d.ID, showName)
		fixed++
	}

	fmt.Printf("\nFixed %d downloads\n", fixed)
	return nil
}

func extractShowName(m3uURL, filepath string) string {
	// Try M3U URL first: https://wmbr.org/m3u/ShowName_YYYYMMDD_HHMM.m3u
	if m3uURL != "" {
		parts := strings.Split(m3uURL, "/")
		if len(parts) > 0 {
			filename := parts[len(parts)-1]
			// Remove .m3u extension
			filename = strings.TrimSuffix(filename, ".m3u")
			// Split by underscore and take the show name part
			parts := strings.Split(filename, "_")
			if len(parts) >= 1 {
				return parts[0]
			}
		}
	}

	// Try filepath: STATION_ShowName_YYYYMMDD.mp3
	if filepath != "" {
		parts := strings.Split(filepath, "/")
		if len(parts) > 0 {
			filename := parts[len(parts)-1]
			filename = strings.TrimSuffix(filename, ".mp3")
			parts := strings.Split(filename, "_")
			if len(parts) >= 2 {
				return parts[1] // Skip station prefix
			}
		}
	}

	return ""
}
