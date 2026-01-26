package api_test

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"local/tapedeck/internal/api"
	"local/tapedeck/internal/db"
)

//go:embed testdata/web/*
var testWebFiles embed.FS

//go:embed testdata/sample.mp3
var sampleMP3 []byte

// consoleMessage represents a browser console message
type consoleMessage struct {
	Level string
	Text  string
}

// findChromium finds the chromium/chrome executable on Linux
func findChromium() string {
	candidates := []string{
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
	}

	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// setupTestServer creates a test server with seeded data and a sample audio file
func setupTestServer(t *testing.T) (*httptest.Server, *db.DB, func()) {
	t.Helper()

	// Create temp directory for downloads
	tmpDir, err := os.MkdirTemp("", "tapedeck-e2e-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	downloadsDir := filepath.Join(tmpDir, "downloads")
	if err := os.MkdirAll(downloadsDir, 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create downloads dir: %v", err)
	}

	// Write sample audio file
	audioPath := filepath.Join(downloadsDir, "WMBR_Backwoods_20260124.mp3")
	if err := os.WriteFile(audioPath, sampleMP3, 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to write sample audio: %v", err)
	}

	// Create in-memory database
	database, err := db.Open(":memory:")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open database: %v", err)
	}

	// Seed test data
	station, err := database.GetOrCreateStation("WMBR", "MIT Radio", "")
	if err != nil {
		t.Fatalf("failed to create station: %v", err)
	}

	// Cache shows
	err = database.CacheShows(station.ID, []string{"Backwoods", "Lost Highway"})
	if err != nil {
		t.Fatalf("failed to cache shows: %v", err)
	}

	// Get show for download linking
	show, err := database.GetShowByName(station.ID, "Backwoods")
	if err != nil {
		t.Fatalf("failed to get show: %v", err)
	}

	// Create a completed download pointing to the sample audio file
	showID := show.ID
	_, err = database.InsertDownload(&db.Download{
		StationID:   station.ID,
		ShowID:      &showID,
		ArchiveDate: time.Date(2026, 1, 24, 0, 0, 0, 0, time.UTC),
		M3UURL:      "https://wmbr.org/m3u/Backwoods_20260124_1000.m3u",
		Filepath:    audioPath,
		Status:      db.StatusCompleted,
	})
	if err != nil {
		t.Fatalf("failed to insert download: %v", err)
	}

	// Create API server with the temp downloads directory
	apiServer := api.NewServer(database, downloadsDir)

	// Setup routes
	mux := http.NewServeMux()
	apiServer.RegisterRoutes(mux)

	// Serve web files from testdata
	webFS, err := fs.Sub(testWebFiles, "testdata/web")
	if err != nil {
		t.Fatalf("failed to create web filesystem: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	// Create test server
	server := httptest.NewServer(mux)

	cleanup := func() {
		server.Close()
		database.Close()
		os.RemoveAll(tmpDir)
	}

	return server, database, cleanup
}

// newBrowserContext creates a chromedp context for headless browser testing on Linux
func newBrowserContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	chromePath := findChromium()
	if chromePath == "" {
		t.Skip("Chromium/Chrome not found; install chromium to run E2E tests")
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-software-rasterizer", true),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)

	cleanup := func() {
		cancelTimeout()
		cancelCtx()
		cancelAlloc()
	}

	return ctx, cleanup
}

// consoleCollector collects browser console messages during tests
type consoleCollector struct {
	mu       sync.Mutex
	messages []consoleMessage
	t        *testing.T
}

func newConsoleCollector(t *testing.T) *consoleCollector {
	return &consoleCollector{t: t}
}

func (c *consoleCollector) listen(ctx context.Context) {
	chromedp.ListenTarget(ctx, func(ev any) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			c.mu.Lock()
			defer c.mu.Unlock()

			var text string
			for _, arg := range ev.Args {
				if arg.Value != nil {
					text += fmt.Sprintf("%v ", arg.Value)
				}
			}

			level := string(ev.Type)
			c.messages = append(c.messages, consoleMessage{
				Level: level,
				Text:  strings.TrimSpace(text),
			})

			if level == "warning" || level == "error" {
				c.t.Logf("Browser console [%s]: %s", level, text)
			}

		case *runtime.EventExceptionThrown:
			c.mu.Lock()
			defer c.mu.Unlock()

			text := ev.ExceptionDetails.Text
			if ev.ExceptionDetails.Exception != nil && ev.ExceptionDetails.Exception.Description != "" {
				text = ev.ExceptionDetails.Exception.Description
			}

			c.messages = append(c.messages, consoleMessage{
				Level: "exception",
				Text:  text,
			})
			c.t.Logf("Browser exception: %s", text)
		}
	})
}

func (c *consoleCollector) hasErrors() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, msg := range c.messages {
		if msg.Level == "error" || msg.Level == "exception" {
			return true
		}
	}
	return false
}

func (c *consoleCollector) errorCount() (errors, exceptions int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, msg := range c.messages {
		switch msg.Level {
		case "error":
			errors++
		case "exception":
			exceptions++
		}
	}
	return
}

// TestE2ESelectStationShowAndCollection tests the main user flow:
// 1. Select a station from dropdown
// 2. Select a show from dropdown
// 3. Verify collection items appear
func TestE2ESelectStationShowAndCollection(t *testing.T) {
	if os.Getenv("E2E_TEST") == "" {
		t.Skip("Skipping E2E test; set E2E_TEST=1 to run")
	}

	server, _, serverCleanup := setupTestServer(t)
	defer serverCleanup()

	ctx, browserCleanup := newBrowserContext(t)
	defer browserCleanup()

	// Set up console message collection
	collector := newConsoleCollector(t)
	collector.listen(ctx)

	// Run the test steps
	var stationOptions, showOptions, tapeSpines int

	err := chromedp.Run(ctx,
		// Navigate to the page
		chromedp.Navigate(server.URL),

		// Wait for page to load and stations to populate
		chromedp.WaitVisible(`#station-select`, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),

		// Count station options (should have at least WMBR + default)
		chromedp.Evaluate(`document.querySelectorAll('#station-select option').length`, &stationOptions),

		// Select WMBR station - triggers change event to load shows
		chromedp.SetValue(`#station-select`, "WMBR", chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('station-select').dispatchEvent(new Event('change'))`, nil),
		chromedp.Sleep(500*time.Millisecond),

		// Count show options
		chromedp.Evaluate(`document.querySelectorAll('#show-select option').length`, &showOptions),

		// Select Backwoods show (ID=1) - triggers change event to load downloads
		chromedp.SetValue(`#show-select`, "1", chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('show-select').dispatchEvent(new Event('change'))`, nil),
		chromedp.Sleep(500*time.Millisecond),

		// Count collection items
		chromedp.Evaluate(`document.querySelectorAll('.tape-spine').length`, &tapeSpines),
	)

	if err != nil {
		t.Fatalf("chromedp run failed: %v", err)
	}

	// Validate results
	if stationOptions < 2 {
		t.Errorf("expected at least 2 station options (default + WMBR), got %d", stationOptions)
	}

	if showOptions < 2 {
		t.Errorf("expected at least 2 show options (default + Backwoods), got %d", showOptions)
	}

	if tapeSpines < 1 {
		t.Errorf("expected at least 1 tape spine in collection, got %d", tapeSpines)
	}

	// Check for console errors
	errors, exceptions := collector.errorCount()
	if errors > 0 {
		t.Errorf("Browser console had %d errors", errors)
	}
	if exceptions > 0 {
		t.Errorf("Browser had %d unhandled exceptions", exceptions)
	}
}

// TestE2EClickCollectionItem tests clicking on a collection item to play audio
func TestE2EClickCollectionItem(t *testing.T) {
	if os.Getenv("E2E_TEST") == "" {
		t.Skip("Skipping E2E test; set E2E_TEST=1 to run")
	}

	server, _, serverCleanup := setupTestServer(t)
	defer serverCleanup()

	ctx, browserCleanup := newBrowserContext(t)
	defer browserCleanup()

	collector := newConsoleCollector(t)
	collector.listen(ctx)

	var nowPlayingText string
	var audioSrc string

	err := chromedp.Run(ctx,
		// Navigate and select station/show
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#station-select`, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),

		// Select station
		chromedp.SetValue(`#station-select`, "WMBR", chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('station-select').dispatchEvent(new Event('change'))`, nil),
		chromedp.Sleep(500*time.Millisecond),

		// Select show
		chromedp.SetValue(`#show-select`, "1", chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('show-select').dispatchEvent(new Event('change'))`, nil),
		chromedp.Sleep(500*time.Millisecond),

		// Wait for tape spines to appear
		chromedp.WaitVisible(`.tape-spine`, chromedp.ByQuery),

		// Click on the first tape spine
		chromedp.Click(`.tape-spine`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Check that now-playing is updated
		chromedp.Text(`#now-playing`, &nowPlayingText, chromedp.ByID),

		// Check that audio source is set
		chromedp.Evaluate(`document.getElementById('audio-player').src`, &audioSrc),
	)

	if err != nil {
		t.Fatalf("chromedp run failed: %v", err)
	}

	// Validate results
	if nowPlayingText == "" || nowPlayingText == "No tape loaded" {
		t.Errorf("expected now-playing to show track info, got: %q", nowPlayingText)
	}

	if audioSrc == "" {
		t.Errorf("expected audio player to have a source set")
	}

	if !strings.Contains(audioSrc, "/api/audio/") {
		t.Errorf("expected audio source to point to API, got: %s", audioSrc)
	}

	if collector.hasErrors() {
		t.Error("browser console had errors during test")
	}
}

// TestE2EConsoleClean verifies no console errors on initial page load
func TestE2EConsoleClean(t *testing.T) {
	if os.Getenv("E2E_TEST") == "" {
		t.Skip("Skipping E2E test; set E2E_TEST=1 to run")
	}

	server, _, serverCleanup := setupTestServer(t)
	defer serverCleanup()

	ctx, browserCleanup := newBrowserContext(t)
	defer browserCleanup()

	collector := newConsoleCollector(t)
	collector.listen(ctx)

	err := chromedp.Run(ctx,
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#station-select`, chromedp.ByID),
		chromedp.Sleep(1*time.Second), // Give time for any async errors
	)

	if err != nil {
		t.Fatalf("chromedp run failed: %v", err)
	}

	errors, exceptions := collector.errorCount()
	if errors > 0 || exceptions > 0 {
		t.Errorf("browser console had %d errors and %d exceptions on initial load", errors, exceptions)
	}
}

// TestE2EURLStateUpdates verifies that selecting station/show updates the URL
func TestE2EURLStateUpdates(t *testing.T) {
	if os.Getenv("E2E_TEST") == "" {
		t.Skip("Skipping E2E test; set E2E_TEST=1 to run")
	}

	server, _, serverCleanup := setupTestServer(t)
	defer serverCleanup()

	ctx, browserCleanup := newBrowserContext(t)
	defer browserCleanup()

	collector := newConsoleCollector(t)
	collector.listen(ctx)

	var urlAfterStation, urlAfterShow, urlAfterPlay string

	err := chromedp.Run(ctx,
		// Navigate to the page
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#station-select`, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),

		// Select station
		chromedp.SetValue(`#station-select`, "WMBR", chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('station-select').dispatchEvent(new Event('change'))`, nil),
		chromedp.Sleep(500*time.Millisecond),

		// Get URL after station selection
		chromedp.Evaluate(`window.location.search`, &urlAfterStation),

		// Select show
		chromedp.SetValue(`#show-select`, "1", chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('show-select').dispatchEvent(new Event('change'))`, nil),
		chromedp.Sleep(500*time.Millisecond),

		// Get URL after show selection
		chromedp.Evaluate(`window.location.search`, &urlAfterShow),

		// Click on tape to play
		chromedp.WaitVisible(`.tape-spine`, chromedp.ByQuery),
		chromedp.Click(`.tape-spine`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Get URL after play
		chromedp.Evaluate(`window.location.search`, &urlAfterPlay),
	)

	if err != nil {
		t.Fatalf("chromedp run failed: %v", err)
	}

	// Validate URL states
	if !strings.Contains(urlAfterStation, "station=WMBR") {
		t.Errorf("expected URL to contain station=WMBR after station selection, got: %s", urlAfterStation)
	}

	if !strings.Contains(urlAfterShow, "station=WMBR") || !strings.Contains(urlAfterShow, "show=1") {
		t.Errorf("expected URL to contain station=WMBR&show=1 after show selection, got: %s", urlAfterShow)
	}

	if !strings.Contains(urlAfterPlay, "play=") {
		t.Errorf("expected URL to contain play= after clicking tape, got: %s", urlAfterPlay)
	}

	if collector.hasErrors() {
		t.Error("browser console had errors during test")
	}
}

// TestE2EURLStateRestoration verifies that navigating directly to URL with params restores state
func TestE2EURLStateRestoration(t *testing.T) {
	if os.Getenv("E2E_TEST") == "" {
		t.Skip("Skipping E2E test; set E2E_TEST=1 to run")
	}

	server, _, serverCleanup := setupTestServer(t)
	defer serverCleanup()

	ctx, browserCleanup := newBrowserContext(t)
	defer browserCleanup()

	collector := newConsoleCollector(t)
	collector.listen(ctx)

	var stationValue, showValue string
	var tapeSpines int

	// Navigate directly to URL with station and show params
	err := chromedp.Run(ctx,
		chromedp.Navigate(server.URL+"/?station=WMBR&show=1"),
		chromedp.WaitVisible(`#station-select`, chromedp.ByID),
		chromedp.Sleep(1*time.Second), // Give time for state restoration

		// Check that station is selected
		chromedp.Evaluate(`document.getElementById('station-select').value`, &stationValue),

		// Check that show is selected
		chromedp.Evaluate(`document.getElementById('show-select').value`, &showValue),

		// Check that downloads are loaded
		chromedp.Evaluate(`document.querySelectorAll('.tape-spine').length`, &tapeSpines),
	)

	if err != nil {
		t.Fatalf("chromedp run failed: %v", err)
	}

	if stationValue != "WMBR" {
		t.Errorf("expected station to be WMBR, got: %s", stationValue)
	}

	if showValue != "1" {
		t.Errorf("expected show to be 1, got: %s", showValue)
	}

	if tapeSpines < 1 {
		t.Errorf("expected at least 1 tape spine after URL state restoration, got: %d", tapeSpines)
	}

	if collector.hasErrors() {
		t.Error("browser console had errors during test")
	}
}

// TestE2EURLStateRestorationWithPlay verifies that navigating to URL with play= param
// correctly loads the track and shows proper UI state (not falsely showing "playing")
func TestE2EURLStateRestorationWithPlay(t *testing.T) {
	if os.Getenv("E2E_TEST") == "" {
		t.Skip("Skipping E2E test; set E2E_TEST=1 to run")
	}

	server, _, serverCleanup := setupTestServer(t)
	defer serverCleanup()

	ctx, browserCleanup := newBrowserContext(t)
	defer browserCleanup()

	collector := newConsoleCollector(t)
	collector.listen(ctx)

	var nowPlayingText, playButtonIcon, audioSrc string
	var leftReelSpinning, rightReelSpinning bool

	// Navigate directly to URL with play parameter (no prior user interaction)
	err := chromedp.Run(ctx,
		chromedp.Navigate(server.URL+"/?station=WMBR&show=1&play=1"),
		chromedp.WaitVisible(`#station-select`, chromedp.ByID),
		chromedp.Sleep(1*time.Second), // Give time for state restoration and autoplay attempt

		// Check that now-playing shows the track info (track should be loaded)
		chromedp.Text(`#now-playing`, &nowPlayingText, chromedp.ByID),

		// Check that audio source is set (track should be loaded)
		chromedp.Evaluate(`document.getElementById('audio-player').src`, &audioSrc),

		// Check the play button state - should show play icon (not pause) since autoplay is blocked
		chromedp.Evaluate(`document.querySelector('#btn-play .icon').innerHTML`, &playButtonIcon),

		// Check reels are NOT spinning (since autoplay is blocked)
		chromedp.Evaluate(`document.querySelector('.left-reel').classList.contains('spinning')`, &leftReelSpinning),
		chromedp.Evaluate(`document.querySelector('.right-reel').classList.contains('spinning')`, &rightReelSpinning),
	)

	if err != nil {
		t.Fatalf("chromedp run failed: %v", err)
	}

	// Track should be loaded and displayed
	if nowPlayingText == "" || nowPlayingText == "No tape loaded" {
		t.Errorf("expected now-playing to show track info, got: %q", nowPlayingText)
	}

	if audioSrc == "" || !strings.Contains(audioSrc, "/api/audio/") {
		t.Errorf("expected audio source to be set to API endpoint, got: %s", audioSrc)
	}

	// UI should NOT show playing state (autoplay blocked without user interaction)
	// Play button should show play icon (▶ = &#9654;), not pause icon (❚❚)
	if strings.Contains(playButtonIcon, "9616") { // 9616 is the pause bar character
		t.Errorf("expected play button to show play icon (autoplay blocked), but got pause icon: %s", playButtonIcon)
	}

	if leftReelSpinning || rightReelSpinning {
		t.Errorf("expected reels to NOT be spinning (autoplay blocked), left=%v right=%v", leftReelSpinning, rightReelSpinning)
	}

	// Should not have console errors (we handle the autoplay rejection gracefully)
	if collector.hasErrors() {
		t.Error("browser console had errors during test")
	}
}
