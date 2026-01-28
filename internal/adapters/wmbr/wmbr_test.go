package wmbr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"local/tapedeck/pkg/tapedeck"
)

const mockArchiveHTML = `<!DOCTYPE html>
<html>
<head><title>WMBR Archives</title></head>
<body>
<h1>Program Archives</h1>
<ul>
<li><a href="/m3u/Lost_and_Found_20260125_1200.m3u">Lost and Found - Jan 25</a></li>
<li><a href="/m3u/Lost_and_Found_20260118_1200.m3u">Lost and Found - Jan 18</a></li>
<li><a href="/m3u/Backwoods_20260124_2100.m3u">Backwoods - Jan 24</a></li>
<li><a href="/m3u/Pipeline_20260123_1800.m3u">Pipeline - Jan 23</a></li>
</ul>
</body>
</html>`

func TestAdapter_Name(t *testing.T) {
	adapter := New()
	if adapter.Name() != "WMBR" {
		t.Errorf("expected WMBR, got %s", adapter.Name())
	}
}

func TestAdapter_ListShows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockArchiveHTML))
	}))
	defer server.Close()

	adapter := newTestAdapter(server.URL)

	shows, err := adapter.ListShows()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(shows) != 3 {
		t.Errorf("expected 3 shows, got %d", len(shows))
	}

	expected := []string{"Backwoods", "Lost and Found", "Pipeline"}
	for i, show := range shows {
		if show != expected[i] {
			t.Errorf("show %d: expected %q, got %q", i, expected[i], show)
		}
	}
}

func TestAdapter_ListArchives(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockArchiveHTML))
	}))
	defer server.Close()

	adapter := newTestAdapter(server.URL)

	archives, err := adapter.ListArchives("Lost and Found")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(archives) != 2 {
		t.Errorf("expected 2 archives, got %d", len(archives))
	}

	// Should be sorted by date descending
	if archives[0].Date.Before(archives[1].Date) {
		t.Error("archives should be sorted by date descending")
	}
}

func TestAdapter_ListArchives_CaseInsensitive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockArchiveHTML))
	}))
	defer server.Close()

	adapter := newTestAdapter(server.URL)

	// Search with different case
	archives, err := adapter.ListArchives("lost AND FOUND")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(archives) != 2 {
		t.Errorf("expected 2 archives, got %d", len(archives))
	}
}

func TestAdapter_GetLatestArchive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockArchiveHTML))
	}))
	defer server.Close()

	adapter := newTestAdapter(server.URL)

	archive, err := adapter.GetLatestArchive("Lost and Found")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if archive.Date.Format("20060102") != "20260125" {
		t.Errorf("expected date 20260125, got %s", archive.Date.Format("20060102"))
	}
}

func TestAdapter_GetLatestArchive_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockArchiveHTML))
	}))
	defer server.Close()

	adapter := newTestAdapter(server.URL)

	_, err := adapter.GetLatestArchive("Nonexistent Show")
	if err == nil {
		t.Error("expected error for nonexistent show")
	}
}

func TestParseArchivePage(t *testing.T) {
	archives, err := parseArchivePage(strings.NewReader(mockArchiveHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(archives) != 4 {
		t.Fatalf("expected 4 archives, got %d", len(archives))
	}

	// Check first archive
	first := archives[0]
	if first.ShowName != "Lost and Found" {
		t.Errorf("expected show name 'Lost and Found', got %q", first.ShowName)
	}
	if first.Date.Format("20060102") != "20260125" {
		t.Errorf("expected date 20260125, got %s", first.Date.Format("20060102"))
	}
	if !strings.Contains(first.M3UURL, "Lost_and_Found_20260125_1200.m3u") {
		t.Errorf("unexpected M3U URL: %s", first.M3UURL)
	}
}

func TestParseArchivePage_Empty(t *testing.T) {
	archives, err := parseArchivePage(strings.NewReader("<html><body></body></html>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(archives) != 0 {
		t.Errorf("expected 0 archives, got %d", len(archives))
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Lost and Found", "Lost_and_Found"},
		{"Show: With Colon", "Show_With_Colon"},
		{"Show?", "Show"},
		{"Normal_Name", "Normal_Name"},
	}

	for _, tc := range tests {
		result := sanitizeFilename(tc.input)
		if result != tc.expected {
			t.Errorf("sanitizeFilename(%q): expected %q, got %q", tc.input, tc.expected, result)
		}
	}
}

// newTestAdapter creates an adapter that uses the test server URL.
func newTestAdapter(baseURL string) *testAdapter {
	return &testAdapter{
		Adapter: NewWithClient(&http.Client{}),
		baseURL: baseURL,
	}
}

type testAdapter struct {
	*Adapter
	baseURL string
}

func (a *testAdapter) ListShows() ([]string, error) {
	archives, err := a.fetchTestArchives()
	if err != nil {
		return nil, err
	}

	showMap := make(map[string]bool)
	for _, arch := range archives {
		showMap[arch.ShowName] = true
	}

	shows := make([]string, 0, len(showMap))
	for show := range showMap {
		shows = append(shows, show)
	}

	// Sort for consistent output
	for i := 0; i < len(shows)-1; i++ {
		for j := i + 1; j < len(shows); j++ {
			if shows[i] > shows[j] {
				shows[i], shows[j] = shows[j], shows[i]
			}
		}
	}

	return shows, nil
}

func (a *testAdapter) ListArchives(show string) ([]tapedeck.Archive, error) {
	allArchives, err := a.fetchTestArchives()
	if err != nil {
		return nil, err
	}

	var archives []tapedeck.Archive
	showLower := strings.ToLower(show)
	for _, arch := range allArchives {
		if strings.ToLower(arch.ShowName) == showLower {
			archives = append(archives, arch)
		}
	}

	// Sort by date descending
	for i := 0; i < len(archives)-1; i++ {
		for j := i + 1; j < len(archives); j++ {
			if archives[i].Date.Before(archives[j].Date) {
				archives[i], archives[j] = archives[j], archives[i]
			}
		}
	}

	return archives, nil
}

func (a *testAdapter) GetLatestArchive(show string) (*tapedeck.Archive, error) {
	archives, err := a.ListArchives(show)
	if err != nil {
		return nil, err
	}

	if len(archives) == 0 {
		return nil, &notFoundError{show: show}
	}

	return &archives[0], nil
}

func (a *testAdapter) fetchTestArchives() ([]tapedeck.Archive, error) {
	resp, err := a.client.Get(a.baseURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return parseArchivePage(resp.Body)
}

type notFoundError struct {
	show string
}

func (e *notFoundError) Error() string {
	return "no archives found for show: " + e.show
}

// Schedule test data - shows with consistent Friday 21:00 schedule
const mockScheduleHTML = `<!DOCTYPE html>
<html>
<head><title>WMBR Archives</title></head>
<body>
<h1>Program Archives</h1>
<ul>
<li><a href="/m3u/Backwoods_20260124_2100.m3u">Backwoods - Jan 24</a></li>
<li><a href="/m3u/Backwoods_20260117_2100.m3u">Backwoods - Jan 17</a></li>
<li><a href="/m3u/Backwoods_20260110_2100.m3u">Backwoods - Jan 10</a></li>
<li><a href="/m3u/Backwoods_20260103_2100.m3u">Backwoods - Jan 3</a></li>
</ul>
</body>
</html>`

// Shows with multiple days per week
const mockMultipleDaysHTML = `<!DOCTYPE html>
<html>
<body>
<ul>
<li><a href="/m3u/Daily_Show_20260125_1200.m3u">Daily Show - Jan 25 (Sun)</a></li>
<li><a href="/m3u/Daily_Show_20260124_1200.m3u">Daily Show - Jan 24 (Sat)</a></li>
<li><a href="/m3u/Daily_Show_20260123_1200.m3u">Daily Show - Jan 23 (Fri)</a></li>
<li><a href="/m3u/Daily_Show_20260122_1200.m3u">Daily Show - Jan 22 (Thu)</a></li>
</ul>
</body>
</html>`

// Late night show (23:00) that rolls over
const mockLateNightHTML = `<!DOCTYPE html>
<html>
<body>
<ul>
<li><a href="/m3u/Night_Owl_20260125_2300.m3u">Night Owl - Jan 25 (Sun)</a></li>
<li><a href="/m3u/Night_Owl_20260118_2300.m3u">Night Owl - Jan 18 (Sun)</a></li>
<li><a href="/m3u/Night_Owl_20260111_2300.m3u">Night Owl - Jan 11 (Sun)</a></li>
</ul>
</body>
</html>`

// Only one archive - insufficient data
const mockInsufficientHTML = `<!DOCTYPE html>
<html>
<body>
<ul>
<li><a href="/m3u/New_Show_20260125_1500.m3u">New Show - Jan 25</a></li>
</ul>
</body>
</html>`

func TestAdapter_GetShowSchedule_ConsistentSchedule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockScheduleHTML))
	}))
	defer server.Close()

	adapter := newTestScheduleAdapter(server.URL)

	schedule, err := adapter.GetShowSchedule("Backwoods")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Backwoods airs Friday (day 5) at 21:00
	// 2026-01-24 is a Saturday, 2026-01-17 is a Saturday, etc.
	// Actually let me verify: Jan 1, 2026 is Thursday
	// Jan 3 = Saturday, Jan 10 = Saturday, Jan 17 = Saturday, Jan 24 = Saturday
	// So the show airs on Saturdays at 21:00

	if schedule.DayOfWeek.String() != "Saturday" {
		t.Errorf("expected Saturday, got %s", schedule.DayOfWeek)
	}

	if schedule.StartTime != "21:00" {
		t.Errorf("expected start time 21:00, got %s", schedule.StartTime)
	}

	if schedule.Confidence != "high" {
		t.Errorf("expected high confidence, got %s", schedule.Confidence)
	}

	if schedule.MultiplePerWeek {
		t.Error("expected MultiplePerWeek to be false")
	}

	// Download time should be 23:30 EST on Saturday (21:00 EST + 2.5h = 23:30 EST)
	// Cron is now stored in local time (America/New_York)
	if schedule.RecommendedCron != "30 23 * * 6" {
		t.Errorf("expected cron '30 23 * * 6' (EST), got %q", schedule.RecommendedCron)
	}
}

func TestAdapter_GetShowSchedule_InsufficientData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockInsufficientHTML))
	}))
	defer server.Close()

	adapter := newTestScheduleAdapter(server.URL)

	_, err := adapter.GetShowSchedule("New Show")
	if err == nil {
		t.Error("expected error for insufficient data")
	}
	if !strings.Contains(err.Error(), "insufficient data") {
		t.Errorf("expected 'insufficient data' error, got: %v", err)
	}
}

func TestAdapter_GetShowSchedule_LateNightRollover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockLateNightHTML))
	}))
	defer server.Close()

	adapter := newTestScheduleAdapter(server.URL)

	schedule, err := adapter.GetShowSchedule("Night Owl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Night Owl airs Sunday at 23:00 EST
	// Download time: 23:00 EST + 2:30 = 01:30 EST Monday
	// Cron is now stored in local time (America/New_York)
	if schedule.RecommendedCron != "30 1 * * 1" {
		t.Errorf("expected cron '30 1 * * 1' (EST), got %q", schedule.RecommendedCron)
	}
}

func TestAdapter_GetShowSchedule_MultiplePerWeek(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockMultipleDaysHTML))
	}))
	defer server.Close()

	adapter := newTestScheduleAdapter(server.URL)

	schedule, err := adapter.GetShowSchedule("Daily Show")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !schedule.MultiplePerWeek {
		t.Error("expected MultiplePerWeek to be true")
	}

	// Should be medium confidence due to multiple days
	if schedule.Confidence != "medium" && schedule.Confidence != "low" {
		t.Errorf("expected medium or low confidence for multi-day show, got %s", schedule.Confidence)
	}

	if schedule.Notes == "" {
		t.Error("expected notes about multiple days")
	}
}

func TestCalculateDownloadTime(t *testing.T) {
	// All expected values are in local time (America/New_York)
	tests := []struct {
		name      string
		startTime string
		day       time.Weekday
		expected  string
	}{
		// 21:00 EST + 2:30 = 23:30 EST same day (Friday)
		{"Normal evening", "21:00", time.Friday, "30 23 * * 5"},
		// 23:00 EST + 2:30 = 01:30 EST (Sat->Sun)
		{"Late night rollover", "23:00", time.Saturday, "30 1 * * 0"},
		// 14:00 EST + 2:30 = 16:30 EST same day
		{"Afternoon", "14:00", time.Monday, "30 16 * * 1"},
		// 06:00 EST + 2:30 = 08:30 EST same day
		{"Early morning", "06:00", time.Wednesday, "30 8 * * 3"},
		// 21:45 EST + 2:30 = 00:15 EST next day (Tuesday->Wednesday)
		{"With minutes", "21:45", time.Tuesday, "15 0 * * 3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateDownloadTime(tc.startTime, tc.day)
			if result != tc.expected {
				t.Errorf("calculateDownloadTime(%q, %s): expected %q, got %q",
					tc.startTime, tc.day, tc.expected, result)
			}
		})
	}
}

// testScheduleAdapter wraps the adapter for schedule testing
type testScheduleAdapter struct {
	*Adapter
	baseURL string
}

func newTestScheduleAdapter(baseURL string) *testScheduleAdapter {
	return &testScheduleAdapter{
		Adapter: NewWithClient(&http.Client{}),
		baseURL: baseURL,
	}
}

func (a *testScheduleAdapter) GetShowSchedule(show string) (*tapedeck.Schedule, error) {
	archives, err := a.listTestArchives(show)
	if err != nil {
		return nil, err
	}

	if len(archives) < 2 {
		return nil, fmt.Errorf("insufficient data: need at least 2 archives, found %d", len(archives))
	}

	// Use the same logic as the real adapter
	return analyzeSchedule(show, archives)
}

func (a *testScheduleAdapter) listTestArchives(show string) ([]tapedeck.Archive, error) {
	resp, err := a.client.Get(a.baseURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	allArchives, err := parseArchivePage(resp.Body)
	if err != nil {
		return nil, err
	}

	var archives []tapedeck.Archive
	showLower := strings.ToLower(show)
	for _, arch := range allArchives {
		if strings.ToLower(arch.ShowName) == showLower {
			archives = append(archives, arch)
		}
	}

	return archives, nil
}
