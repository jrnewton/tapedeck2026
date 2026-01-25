package m3u

import (
	"io"
	"testing"
)

func TestParse_ExtendedM3U(t *testing.T) {
	content := `#EXTM3U
#EXTINF:123,Sample Title
http://example.com/stream1.mp3
#EXTINF:456,Another Title
https://example.com/stream2.mp3
`
	urls, err := ParseString(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}

	expected := []string{
		"http://example.com/stream1.mp3",
		"https://example.com/stream2.mp3",
	}

	for i, url := range urls {
		if url != expected[i] {
			t.Errorf("URL %d: expected %q, got %q", i, expected[i], url)
		}
	}
}

func TestParse_SimpleM3U(t *testing.T) {
	content := `http://example.com/stream.mp3
https://example.com/stream2.mp3`

	urls, err := ParseString(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}
}

func TestParse_EmptyLines(t *testing.T) {
	content := `#EXTM3U

http://example.com/stream.mp3

`
	urls, err := ParseString(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
}

func TestParse_EmptyFile(t *testing.T) {
	urls, err := ParseString("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 0 {
		t.Fatalf("expected 0 URLs, got %d", len(urls))
	}
}

func TestParse_OnlyComments(t *testing.T) {
	content := `#EXTM3U
#EXTINF:123,Title`

	urls, err := ParseString(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 0 {
		t.Fatalf("expected 0 URLs, got %d", len(urls))
	}
}

func TestParse_ReaderError(t *testing.T) {
	// Test with a reader that returns an error
	r := &errorReader{}
	_, err := Parse(r)
	// Scanner should handle most errors gracefully
	if err == nil {
		t.Error("expected error from errorReader")
	}
}

type errorReader struct{}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}
