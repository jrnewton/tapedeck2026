package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"local/tapedeck/internal/db"
	"local/tapedeck/pkg/tapedeck"

	// Register adapters
	_ "local/tapedeck/internal/adapters/wmbr"
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
	case "list-schedules":
		err = cmdListSchedules(args)
	case "delete-schedule":
		err = cmdDeleteSchedule(args)
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
  schedule-download <STATION> <SHOW> [options] Create scheduled download on server
  list-schedules [STATION]                     List all scheduled downloads
  delete-schedule <ID>                         Delete a scheduled download

Options for download-show:
  --latest            Download the most recent archive (default)
  --date YYYYMMDD     Download archive from specific date
  --output DIR        Output directory (default: ./data/downloads)

Options for schedule-download:
  --dryrun            Show what would be created without creating it
  --cron-only         Output crontab line only (legacy mode, no server)

Environment Variables:
  TAPEDECK_SERVER_URL  Server URL (default: http://localhost:8080)
  TAPEDECK_DATA_DIR    Data directory (default: ./data)

Supported Stations:
  WMBR

Examples:
  tapedeck-cli list-shows WMBR
  tapedeck-cli download-show WMBR "Lost and Found" --latest
  tapedeck-cli download-status 42
  tapedeck-cli download-status
  tapedeck-cli list-downloads WMBR
  tapedeck-cli schedule-download WMBR Backwoods
  tapedeck-cli schedule-download WMBR Backwoods --cron-only
  tapedeck-cli schedule-download WMBR Backwoods --dryrun
  TAPEDECK_SERVER_URL=http://host:8080 tapedeck-cli schedule-download WMBR Backwoods
  tapedeck-cli list-schedules
  tapedeck-cli list-schedules WMBR
  tapedeck-cli delete-schedule 1`)
}

func cmdListShows(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: list-shows <STATION>")
	}

	callSign := strings.ToUpper(args[0])

	// Fetch from adapter (always live, no caching)
	adapter, err := tapedeck.GetAdapter(callSign)
	if err != nil {
		return err
	}

	shows, err := adapter.ListShows()
	if err != nil {
		return fmt.Errorf("list shows: %w", err)
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

		// Validate station exists
		_, err := tapedeck.GetAdapter(callSign)
		if err != nil {
			return fmt.Errorf("unknown station: %s", callSign)
		}
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

	// Set output directory - use TAPEDECK_DATA_DIR for consistency with web server
	outputDir := *output
	if outputDir == "" {
		dataDir := os.Getenv("TAPEDECK_DATA_DIR")
		if dataDir == "" {
			dataDir = defaultDataDir
		}
		outputDir = filepath.Join(dataDir, "downloads")
	}

	adapter, err := tapedeck.GetAdapter(callSign)
	if err != nil {
		return err
	}

	// Validate show name exists
	availableShows, err := adapter.ListShows()
	if err != nil {
		return fmt.Errorf("list shows: %w", err)
	}
	showFound := false
	for _, s := range availableShows {
		if s == showName {
			showFound = true
			break
		}
	}
	if !showFound {
		return fmt.Errorf("unknown show: %s", showName)
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

	// Get show ID if it exists in DB
	var showID *int64
	show, err := database.GetShowByName(station.ID, archive.ShowName)
	if err == nil && show != nil {
		showID = &show.ID
	}

	// Check for existing download of same station/show/date
	existing, err := database.FindDownload(station.ID, showID, archive.Date)
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
		// Reconstruct full path for display
		fullPath := filepath.Join(outputDir, d.Filepath)
		fmt.Printf("Download completed: %s\n", fullPath)
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

	// Update status to completed - store only the filename (not full path)
	// so it works across host CLI, Docker CLI, and Docker Web contexts
	filename := filepath.Base(destPath)
	database.UpdateDownloadStatus(downloadID, db.StatusCompleted, filename, "")
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
	dataDir := os.Getenv("TAPEDECK_DATA_DIR")
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	downloadsDir := filepath.Join(dataDir, "downloads")

	fmt.Printf("[%d] %s - %s (%s)\n", d.ID, d.Station, d.Show, d.ArchiveDate.Format("2006-01-02"))
	fmt.Printf("  Status:  %s\n", formatStatus(d.Status))
	fmt.Printf("  Started: %s\n", d.CreatedAt.Format("2006-01-02 15:04:05"))

	if !d.UpdatedAt.IsZero() {
		fmt.Printf("  Updated: %s\n", d.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	if d.Filepath != "" {
		// Reconstruct full path from stored filename
		fullPath := filepath.Join(downloadsDir, d.Filepath)
		fmt.Printf("  File:    %s\n", fullPath)
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
		panic(fmt.Sprintf("unknown status: %s", status))
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
	// Manually extract flags from any position
	cronOnly := false
	dryRun := false
	var positionalArgs []string
	for _, arg := range args {
		switch arg {
		case "--cron-only", "-cron-only":
			cronOnly = true
		case "--dryrun", "-dryrun", "--dry-run", "-dry-run":
			dryRun = true
		default:
			positionalArgs = append(positionalArgs, arg)
		}
	}

	if len(positionalArgs) < 2 {
		return fmt.Errorf("usage: schedule-download <STATION> <SHOW> [--cron-only | --dryrun]")
	}

	callSign := strings.ToUpper(positionalArgs[0])
	showName := positionalArgs[1]

	adapter, err := tapedeck.GetAdapter(callSign)
	if err != nil {
		return err
	}

	// Validate show name exists
	shows, err := adapter.ListShows()
	if err != nil {
		return fmt.Errorf("list shows: %w", err)
	}

	showFound := false
	for _, s := range shows {
		if s == showName {
			showFound = true
			break
		}
	}
	if !showFound {
		return fmt.Errorf("unknown show: %s", showName)
	}

	fmt.Printf("Analyzing broadcast history for %s...\n", showName)

	schedule, err := adapter.GetShowSchedule(showName)
	if err != nil {
		return fmt.Errorf("get schedule: %w", err)
	}

	fmt.Printf("Detected schedule: %ss ~%s, archive available ~%s\n",
		schedule.DayOfWeek, schedule.StartTime, formatCronTime(schedule.RecommendedCron))

	if schedule.Confidence == "low" {
		fmt.Printf("Warning: Low confidence schedule (few archives). Monitor for accuracy.\n")
	}

	// Legacy mode: output crontab line only
	if cronOnly {
		return outputCronLine(callSign, showName, schedule)
	}

	// Dry run mode: show what would be created without creating it
	if dryRun {
		fmt.Printf("\n[Dry run - no schedule created]\n")
		fmt.Printf("Would create schedule:\n")
		fmt.Printf("  Station:   %s\n", callSign)
		fmt.Printf("  Show:      %s\n", showName)
		fmt.Printf("  Schedule:  %s (%s)\n", describeCron(schedule.RecommendedCron), schedule.RecommendedCron)
		if schedule.Notes != "" {
			fmt.Printf("  Note:      %s\n", schedule.Notes)
		}
		return nil
	}

	// Server mode: create schedule via API
	serverURL := os.Getenv("TAPEDECK_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// Create schedule via API
	payload := map[string]string{
		"station": callSign,
		"show":    showName,
		"cron":    schedule.RecommendedCron,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	resp, err := http.Post(serverURL+"/api/schedules", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("cannot connect to server at %s. Is the server running?", serverURL)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("schedule already exists for %s. Use list-schedules to view", showName)
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("show '%s' not found for station %s. Run a download first to register the show", showName, callSign)
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		ID             int64     `json:"ID"`
		CronExpression string    `json:"CronExpression"`
		NextRunAt      time.Time `json:"NextRunAt"`
		Station        string    `json:"Station"`
		Show           string    `json:"Show"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Printf("Created schedule #%d: %s\n", result.ID, result.CronExpression)
	if !result.NextRunAt.IsZero() {
		fmt.Printf("Next run: %s\n", result.NextRunAt.Local().Format("2006-01-02 15:04:05"))
	}

	return nil
}

// outputCronLine outputs a crontab line for manual installation (legacy mode).
func outputCronLine(callSign, showName string, schedule *tapedeck.Schedule) error {
	// Format the show name for the crontab command
	quotedShow := showName
	if strings.Contains(showName, " ") {
		quotedShow = fmt.Sprintf("%q", showName)
	}

	// Build the cron line
	cronLine := fmt.Sprintf("%s docker exec tapedeck tapedeck-cli download-show %s %s --latest",
		schedule.RecommendedCron, callSign, quotedShow)

	// Output the crontab line with comments
	fmt.Printf("\n# %s on %s\n", showName, callSign)
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

// formatCronTime extracts HH:MM from a cron expression like "30 4 * * 0".
func formatCronTime(cron string) string {
	parts := strings.Fields(cron)
	if len(parts) >= 2 {
		min := parts[0]
		hour := parts[1]
		if len(min) == 1 {
			min = "0" + min
		}
		if len(hour) == 1 {
			hour = "0" + hour
		}
		return hour + ":" + min
	}
	return cron
}

// describeCron converts a cron expression like "30 4 * * 0" to English like "Sundays at 04:30".
func describeCron(cron string) string {
	parts := strings.Fields(cron)
	if len(parts) < 5 {
		return cron
	}

	min := parts[0]
	hour := parts[1]
	dow := parts[4]

	// Format time
	if len(min) == 1 {
		min = "0" + min
	}
	if len(hour) == 1 {
		hour = "0" + hour
	}
	timeStr := hour + ":" + min

	// Day of week
	days := map[string]string{
		"0": "Sundays",
		"1": "Mondays",
		"2": "Tuesdays",
		"3": "Wednesdays",
		"4": "Thursdays",
		"5": "Fridays",
		"6": "Saturdays",
		"7": "Sundays", // Some systems use 7 for Sunday
		"*": "Every day",
	}

	dayStr, ok := days[dow]
	if !ok {
		dayStr = "day " + dow
	}

	if dow == "*" {
		return fmt.Sprintf("%s at %s", dayStr, timeStr)
	}
	return fmt.Sprintf("%s at %s", dayStr, timeStr)
}

func cmdListSchedules(args []string) error {
	var callSign string
	if len(args) > 0 {
		callSign = strings.ToUpper(args[0])

		// Validate station exists
		_, err := tapedeck.GetAdapter(callSign)
		if err != nil {
			return fmt.Errorf("unknown station: %s", callSign)
		}
	}

	serverURL := os.Getenv("TAPEDECK_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	resp, err := http.Get(serverURL + "/api/schedules")
	if err != nil {
		return fmt.Errorf("cannot connect to server at %s. Is the server running?", serverURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Schedules []struct {
			ID              int64      `json:"ID"`
			Station         string     `json:"Station"`
			Show            string     `json:"Show"`
			CronExpression  string     `json:"CronExpression"`
			CronDescription string     `json:"CronDescription"` // Pre-formatted from backend
			Enabled         bool       `json:"Enabled"`
			LastRunAt       *time.Time `json:"LastRunAt"`
			LastRunDisplay  string     `json:"LastRunDisplay"` // Pre-formatted from backend
			LastStatus      string     `json:"LastStatus"`
			NextRunAt       *time.Time `json:"NextRunAt"`
			NextRunDisplay  string     `json:"NextRunDisplay"` // Pre-formatted from backend
		} `json:"schedules"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	// Filter by station if specified
	var schedules []struct {
		ID              int64      `json:"ID"`
		Station         string     `json:"Station"`
		Show            string     `json:"Show"`
		CronExpression  string     `json:"CronExpression"`
		CronDescription string     `json:"CronDescription"`
		Enabled         bool       `json:"Enabled"`
		LastRunAt       *time.Time `json:"LastRunAt"`
		LastRunDisplay  string     `json:"LastRunDisplay"`
		LastStatus      string     `json:"LastStatus"`
		NextRunAt       *time.Time `json:"NextRunAt"`
		NextRunDisplay  string     `json:"NextRunDisplay"`
	}
	for _, s := range result.Schedules {
		if callSign == "" || s.Station == callSign {
			schedules = append(schedules, s)
		}
	}

	if len(schedules) == 0 {
		if callSign != "" {
			fmt.Printf("No schedules found for station %s\n", callSign)
		} else {
			fmt.Println("No schedules configured.")
		}
		return nil
	}

	// Print each schedule
	for _, s := range schedules {
		// Use backend-provided display strings with fallbacks
		lastRunDisplay := s.LastRunDisplay
		if lastRunDisplay == "" || lastRunDisplay == "-" {
			lastRunDisplay = "(never)"
		}

		status := "-"
		if s.LastStatus != "" {
			status = s.LastStatus
		}
		if !s.Enabled {
			status = "disabled"
		}

		nextRunDisplay := s.NextRunDisplay
		if nextRunDisplay == "" {
			nextRunDisplay = "-"
		}

		// Use CronDescription from backend, fallback to local describeCron
		cronDesc := s.CronDescription
		if cronDesc == "" {
			cronDesc = describeCron(s.CronExpression)
		}

		fmt.Printf("[%d] %s - %s\n", s.ID, s.Station, s.Show)
		fmt.Printf("    Schedule:  %s (%s)\n", cronDesc, s.CronExpression)
		fmt.Printf("    Last run:  %s (%s)\n", lastRunDisplay, status)
		fmt.Printf("    Next run:  %s\n", nextRunDisplay)
		fmt.Println()
	}

	return nil
}

func cmdDeleteSchedule(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: delete-schedule <ID>")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid schedule ID: %s", args[0])
	}

	serverURL := os.Getenv("TAPEDECK_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// First get schedule details for confirmation message
	resp, err := http.Get(fmt.Sprintf("%s/api/schedules/%d", serverURL, id))
	if err != nil {
		return fmt.Errorf("cannot connect to server at %s. Is the server running?", serverURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("schedule not found: %d", id)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	var schedule struct {
		Station string `json:"Station"`
		Show    string `json:"Show"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&schedule); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	// Delete via API
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/schedules/%d", serverURL, id), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode == http.StatusNotFound {
		return fmt.Errorf("schedule not found: %d", id)
	}

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("server error (%d): %s", resp2.StatusCode, string(body))
	}

	fmt.Printf("Deleted schedule #%d (%s - %s)\n", id, schedule.Station, schedule.Show)
	return nil
}

func cmdFixDownloads(_ []string) error {
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
	pathsFixed := 0
	for _, d := range downloads {
		// Migrate absolute paths or paths with directories to just filenames
		// This ensures paths work across host CLI, Docker CLI, and Docker Web contexts
		if d.Filepath != "" {
			filename := filepath.Base(d.Filepath)
			if filename != d.Filepath {
				// Path contains directory components - migrate to just filename
				if err := database.UpdateDownloadStatus(d.ID, d.Status, filename, d.Error); err != nil {
					fmt.Printf("Failed to fix filepath for download %d: %v\n", d.ID, err)
				} else {
					fmt.Printf("Fixed filepath for download %d: %s -> %s\n", d.ID, d.Filepath, filename)
					pathsFixed++
				}
			}
		}

		// Skip show linking if already has a show_id
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

		// Get station to find show
		station, err := database.GetStation(d.Station)
		if err != nil {
			continue
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

	fmt.Printf("\nFixed %d show links, %d filepaths\n", fixed, pathsFixed)
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
