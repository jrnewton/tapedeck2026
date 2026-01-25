package tapedeck

import "time"

// Archive represents a single archived show recording.
type Archive struct {
	ShowName string
	Date     time.Time
	M3UURL   string
}

// Adapter is the interface that station adapters must implement.
type Adapter interface {
	// Name returns the station call sign (e.g., "WMBR").
	Name() string

	// ListShows returns a list of unique show names available in the archive.
	ListShows() ([]string, error)

	// ListArchives returns all archives for a given show name.
	ListArchives(show string) ([]Archive, error)

	// GetLatestArchive returns the most recent archive for a given show.
	GetLatestArchive(show string) (*Archive, error)

	// DownloadArchive downloads the archive to destDir and returns the filepath.
	DownloadArchive(archive *Archive, destDir string) (string, error)
}
