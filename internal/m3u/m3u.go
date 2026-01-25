package m3u

import (
	"bufio"
	"io"
	"strings"
)

// Parse reads an M3U file and returns all stream URLs found.
// It handles both extended M3U format (#EXTM3U) and simple URL lists.
func Parse(r io.Reader) ([]string, error) {
	var urls []string
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Skip M3U header and metadata lines
		if strings.HasPrefix(line, "#") {
			continue
		}
		// Collect URLs (http/https)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			urls = append(urls, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return urls, nil
}

// ParseString parses M3U content from a string.
func ParseString(content string) ([]string, error) {
	return Parse(strings.NewReader(content))
}
