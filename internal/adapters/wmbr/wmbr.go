package wmbr

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jnewton/tapedeck/internal/m3u"
	"github.com/jnewton/tapedeck/pkg/tapedeck"
	"golang.org/x/net/html"
)

const (
	archiveURL = "https://wmbr.org/cgi-bin/arch"
	m3uBaseURL = "https://wmbr.org/m3u/"
)

// Adapter implements the tapedeck.Adapter interface for WMBR.
type Adapter struct {
	client *http.Client
}

// New creates a new WMBR adapter.
func New() *Adapter {
	return &Adapter{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewWithClient creates a new WMBR adapter with a custom HTTP client.
func NewWithClient(client *http.Client) *Adapter {
	return &Adapter{client: client}
}

// Name returns the station call sign.
func (a *Adapter) Name() string {
	return "WMBR"
}

// ListShows returns a list of unique show names available in the archive.
func (a *Adapter) ListShows() ([]string, error) {
	archives, err := a.fetchArchives()
	if err != nil {
		return nil, err
	}

	// Extract unique show names
	showMap := make(map[string]bool)
	for _, arch := range archives {
		showMap[arch.ShowName] = true
	}

	shows := make([]string, 0, len(showMap))
	for show := range showMap {
		shows = append(shows, show)
	}
	sort.Strings(shows)

	return shows, nil
}

// ListArchives returns all archives for a given show name.
func (a *Adapter) ListArchives(show string) ([]tapedeck.Archive, error) {
	allArchives, err := a.fetchArchives()
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

	// Sort by date descending (most recent first)
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].Date.After(archives[j].Date)
	})

	return archives, nil
}

// GetLatestArchive returns the most recent archive for a given show.
func (a *Adapter) GetLatestArchive(show string) (*tapedeck.Archive, error) {
	archives, err := a.ListArchives(show)
	if err != nil {
		return nil, err
	}

	if len(archives) == 0 {
		return nil, fmt.Errorf("no archives found for show: %s", show)
	}

	return &archives[0], nil
}

// DownloadArchive downloads the archive to destDir and returns the filepath.
func (a *Adapter) DownloadArchive(archive *tapedeck.Archive, destDir string) (string, error) {
	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	// Fetch the m3u file
	resp, err := a.client.Get(archive.M3UURL)
	if err != nil {
		return "", fmt.Errorf("fetch m3u: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch m3u: status %d", resp.StatusCode)
	}

	m3uContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read m3u: %w", err)
	}

	// Parse m3u to get stream URL
	urls, err := m3u.ParseString(string(m3uContent))
	if err != nil {
		return "", fmt.Errorf("parse m3u: %w", err)
	}

	if len(urls) == 0 {
		return "", fmt.Errorf("no stream URLs found in m3u")
	}

	streamURL := urls[0]

	// Download the stream
	streamResp, err := a.client.Get(streamURL)
	if err != nil {
		return "", fmt.Errorf("fetch stream: %w", err)
	}
	defer streamResp.Body.Close()

	if streamResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch stream: status %d", streamResp.StatusCode)
	}

	// Generate output filename
	safeName := sanitizeFilename(archive.ShowName)
	filename := fmt.Sprintf("WMBR_%s_%s.mp3", safeName, archive.Date.Format("20060102"))
	destPath := filepath.Join(destDir, filename)

	// Write to file
	out, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, streamResp.Body); err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("write file: %w", err)
	}

	return destPath, nil
}

// fetchArchives fetches and parses the archive page.
func (a *Adapter) fetchArchives() ([]tapedeck.Archive, error) {
	resp, err := a.client.Get(archiveURL)
	if err != nil {
		return nil, fmt.Errorf("fetch archive page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch archive page: status %d", resp.StatusCode)
	}

	return parseArchivePage(resp.Body)
}

// parseArchivePage parses the WMBR archive HTML page.
func parseArchivePage(r io.Reader) ([]tapedeck.Archive, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var archives []tapedeck.Archive

	// Pattern for m3u URLs: /m3u/Show_Name_YYYYMMDD_HHMM.m3u
	m3uPattern := regexp.MustCompile(`/m3u/(.+)_(\d{8})_(\d{4})\.m3u`)

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					if matches := m3uPattern.FindStringSubmatch(attr.Val); matches != nil {
						showName := strings.ReplaceAll(matches[1], "_", " ")
						// Decode URL-encoded characters (e.g., %28 -> (, %26 -> &)
						if decoded, err := url.QueryUnescape(showName); err == nil {
							showName = decoded
						}
						dateStr := matches[2]

						date, err := time.Parse("20060102", dateStr)
						if err != nil {
							continue
						}

						m3uURL := m3uBaseURL + matches[1] + "_" + dateStr + "_" + matches[3] + ".m3u"

						archives = append(archives, tapedeck.Archive{
							ShowName: showName,
							Date:     date,
							M3UURL:   m3uURL,
						})
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)

	return archives, nil
}

// sanitizeFilename replaces spaces with underscores and removes problematic characters.
func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, " ", "_")
	// Remove characters that are problematic in filenames
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	return re.ReplaceAllString(name, "")
}

// GetShowSchedule analyzes archive history to infer the broadcast schedule.
func (a *Adapter) GetShowSchedule(show string) (*tapedeck.Schedule, error) {
	archives, err := a.ListArchives(show)
	if err != nil {
		return nil, err
	}

	if len(archives) < 2 {
		return nil, fmt.Errorf("insufficient data: need at least 2 archives, found %d", len(archives))
	}

	return analyzeSchedule(show, archives)
}

// analyzeSchedule extracts schedule patterns from a list of archives.
func analyzeSchedule(show string, archives []tapedeck.Archive) (*tapedeck.Schedule, error) {
	// Extract start times from M3U URLs and analyze patterns
	// Pattern: _YYYYMMDD_HHMM.m3u
	timePattern := regexp.MustCompile(`_(\d{8})_(\d{4})\.m3u$`)

	type broadcastInfo struct {
		dayOfWeek time.Weekday
		startTime string // "HH:MM"
	}

	var broadcasts []broadcastInfo

	for _, arch := range archives {
		matches := timePattern.FindStringSubmatch(arch.M3UURL)
		if matches == nil {
			continue
		}

		dateStr := matches[1]
		timeStr := matches[2]

		date, err := time.Parse("20060102", dateStr)
		if err != nil {
			continue
		}

		startTime := fmt.Sprintf("%s:%s", timeStr[:2], timeStr[2:])
		broadcasts = append(broadcasts, broadcastInfo{
			dayOfWeek: date.Weekday(),
			startTime: startTime,
		})
	}

	if len(broadcasts) < 2 {
		return nil, fmt.Errorf("insufficient data: could not parse schedule from archives")
	}

	// Count occurrences of each day/time combination
	type dayTime struct {
		day  time.Weekday
		time string
	}
	counts := make(map[dayTime]int)
	for _, b := range broadcasts {
		dt := dayTime{day: b.dayOfWeek, time: b.startTime}
		counts[dt]++
	}

	// Find the most common day/time
	var dominantDT dayTime
	maxCount := 0
	for dt, count := range counts {
		if count > maxCount {
			maxCount = count
			dominantDT = dt
		}
	}

	// Check for multiple airings per week
	daysUsed := make(map[time.Weekday]bool)
	for _, b := range broadcasts {
		daysUsed[b.dayOfWeek] = true
	}
	multiplePerWeek := len(daysUsed) > 1

	// Determine confidence
	confidence := "high"
	consistencyRatio := float64(maxCount) / float64(len(broadcasts))
	if consistencyRatio < 0.5 {
		confidence = "low"
	} else if consistencyRatio < 0.8 || multiplePerWeek {
		confidence = "medium"
	}

	// Calculate recommended download time
	// Archive delay is ~2 hours, add 30 min buffer = 2.5 hours after broadcast start
	downloadTime := calculateDownloadTime(dominantDT.time, dominantDT.day)

	// Build notes
	var notes string
	if multiplePerWeek {
		days := make([]string, 0, len(daysUsed))
		for d := range daysUsed {
			days = append(days, d.String())
		}
		sort.Strings(days)
		notes = fmt.Sprintf("Show airs on multiple days: %s", strings.Join(days, ", "))
	}

	return &tapedeck.Schedule{
		ShowName:        show,
		DayOfWeek:       dominantDT.day,
		StartTime:       dominantDT.time,
		RecommendedCron: downloadTime,
		Confidence:      confidence,
		MultiplePerWeek: multiplePerWeek,
		Notes:           notes,
	}, nil
}

// calculateDownloadTime adds 2.5 hours to the broadcast start time and returns cron format.
// Handles late-night rollover to next day.
func calculateDownloadTime(startTime string, day time.Weekday) string {
	// Parse start time
	parts := strings.Split(startTime, ":")
	if len(parts) != 2 {
		return "30 23 * * " + fmt.Sprintf("%d", day) // fallback
	}

	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])

	// Add 2.5 hours (2 hours archive delay + 30 min buffer)
	minute += 30
	hour += 2
	if minute >= 60 {
		minute -= 60
		hour++
	}

	downloadDay := day
	if hour >= 24 {
		hour -= 24
		downloadDay = (day + 1) % 7
	}

	return fmt.Sprintf("%d %d * * %d", minute, hour, downloadDay)
}
