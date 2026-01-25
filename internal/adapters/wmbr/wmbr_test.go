package wmbr

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jnewton/tapedeck/pkg/tapedeck"
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
