package db

import (
	"context"
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// DefaultCacheTTL is the default cache time-to-live.
const DefaultCacheTTL = 1 * time.Hour

// Station represents a radio station.
type Station struct {
	ID         int64
	CallSign   string
	Name       string
	ArchiveURL string
}

// Show represents a cached show from a station.
type Show struct {
	ID        int64
	StationID int64
	Name      string
	CachedAt  time.Time
}

// Archive represents a cached archive entry.
type Archive struct {
	ID       int64
	ShowID   int64
	Date     time.Time
	M3UURL   string
	CachedAt time.Time
}

// Download status constants.
const (
	StatusPending     = "pending"
	StatusDownloading = "downloading"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
)

// Download represents a downloaded archive record.
type Download struct {
	ID          int64
	StationID   int64
	ShowID      *int64 // nullable
	ArchiveDate time.Time
	M3UURL      string
	Filepath    string // empty until download completes
	Status      string
	Error       string // error message if failed
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// Denormalized fields for display
	Station string
	Show    string
}

// DB wraps a SQLite connection pool.
type DB struct {
	pool     *sqlitex.Pool
	CacheTTL time.Duration
}

// Open opens or creates a SQLite database at the given path.
// Use ":memory:" for an in-memory database.
func Open(path string) (*DB, error) {
	if path == ":memory:" {
		path = "file::memory:?mode=memory&cache=shared"
	}

	pool, err := sqlitex.NewPool(path, sqlitex.PoolOptions{
		PoolSize: 1,
		Flags:    sqlite.OpenReadWrite | sqlite.OpenCreate | sqlite.OpenURI,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db := &DB{pool: pool, CacheTTL: DefaultCacheTTL}
	if err := db.migrate(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// Close closes the database connection pool.
func (db *DB) Close() error {
	return db.pool.Close()
}

func (db *DB) migrate() error {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer db.pool.Put(conn)

	const schema = `
		CREATE TABLE IF NOT EXISTS stations (
			id INTEGER PRIMARY KEY,
			call_sign TEXT UNIQUE NOT NULL,
			name TEXT,
			archive_url TEXT
		);

		CREATE TABLE IF NOT EXISTS shows (
			id INTEGER PRIMARY KEY,
			station_id INTEGER NOT NULL REFERENCES stations(id),
			name TEXT NOT NULL,
			cached_at TEXT NOT NULL,
			UNIQUE(station_id, name)
		);

		CREATE TABLE IF NOT EXISTS archives (
			id INTEGER PRIMARY KEY,
			show_id INTEGER NOT NULL REFERENCES shows(id),
			date TEXT NOT NULL,
			m3u_url TEXT NOT NULL,
			cached_at TEXT NOT NULL,
			UNIQUE(show_id, date)
		);

		CREATE TABLE IF NOT EXISTS downloads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			station_id INTEGER NOT NULL REFERENCES stations(id),
			show_id INTEGER REFERENCES shows(id),
			archive_date TEXT NOT NULL,
			m3u_url TEXT NOT NULL,
			filepath TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			error TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_shows_station ON shows(station_id);
		CREATE INDEX IF NOT EXISTS idx_archives_show ON archives(show_id);
		CREATE INDEX IF NOT EXISTS idx_downloads_station ON downloads(station_id);
		CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
	`
	return sqlitex.ExecuteScript(conn, schema, nil)
}

// GetOrCreateStation gets a station by call sign, creating it if it doesn't exist.
func (db *DB) GetOrCreateStation(callSign, name, archiveURL string) (*Station, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	// Try to get existing station
	var station *Station
	err = sqlitex.Execute(conn, `SELECT id, call_sign, name, archive_url FROM stations WHERE call_sign = ?`, &sqlitex.ExecOptions{
		Args: []any{callSign},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			station = &Station{
				ID:         stmt.ColumnInt64(0),
				CallSign:   stmt.ColumnText(1),
				Name:       stmt.ColumnText(2),
				ArchiveURL: stmt.ColumnText(3),
			}
			return nil
		},
	})
	if err != nil {
		return nil, err
	}

	if station != nil {
		return station, nil
	}

	// Create new station
	err = sqlitex.Execute(conn, `INSERT INTO stations (call_sign, name, archive_url) VALUES (?, ?, ?)`, &sqlitex.ExecOptions{
		Args: []any{callSign, name, archiveURL},
	})
	if err != nil {
		return nil, err
	}

	return &Station{
		ID:         conn.LastInsertRowID(),
		CallSign:   callSign,
		Name:       name,
		ArchiveURL: archiveURL,
	}, nil
}

// GetStation gets a station by call sign.
func (db *DB) GetStation(callSign string) (*Station, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	var station *Station
	err = sqlitex.Execute(conn, `SELECT id, call_sign, name, archive_url FROM stations WHERE call_sign = ?`, &sqlitex.ExecOptions{
		Args: []any{callSign},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			station = &Station{
				ID:         stmt.ColumnInt64(0),
				CallSign:   stmt.ColumnText(1),
				Name:       stmt.ColumnText(2),
				ArchiveURL: stmt.ColumnText(3),
			}
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	if station == nil {
		return nil, fmt.Errorf("station not found: %s", callSign)
	}
	return station, nil
}

// GetCachedShows returns cached shows for a station if still valid.
func (db *DB) GetCachedShows(stationID int64) ([]Show, bool, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, false, err
	}
	defer db.pool.Put(conn)

	var shows []Show
	var oldestCache time.Time

	err = sqlitex.Execute(conn, `SELECT id, station_id, name, cached_at FROM shows WHERE station_id = ? ORDER BY name`, &sqlitex.ExecOptions{
		Args: []any{stationID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			cachedAt, _ := time.Parse(time.RFC3339, stmt.ColumnText(3))
			if oldestCache.IsZero() || cachedAt.Before(oldestCache) {
				oldestCache = cachedAt
			}
			shows = append(shows, Show{
				ID:        stmt.ColumnInt64(0),
				StationID: stmt.ColumnInt64(1),
				Name:      stmt.ColumnText(2),
				CachedAt:  cachedAt,
			})
			return nil
		},
	})
	if err != nil {
		return nil, false, err
	}

	if len(shows) == 0 {
		return nil, false, nil
	}

	// Check if cache is still valid
	valid := time.Since(oldestCache) < db.CacheTTL
	return shows, valid, nil
}

// CacheShows caches shows for a station, clearing old entries first.
func (db *DB) CacheShows(stationID int64, showNames []string) error {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer db.pool.Put(conn)

	now := time.Now().Format(time.RFC3339)

	// Delete old archives for this station's shows
	err = sqlitex.Execute(conn, `DELETE FROM archives WHERE show_id IN (SELECT id FROM shows WHERE station_id = ?)`, &sqlitex.ExecOptions{
		Args: []any{stationID},
	})
	if err != nil {
		return err
	}

	// Delete old shows
	err = sqlitex.Execute(conn, `DELETE FROM shows WHERE station_id = ?`, &sqlitex.ExecOptions{
		Args: []any{stationID},
	})
	if err != nil {
		return err
	}

	// Insert new shows
	for _, name := range showNames {
		err = sqlitex.Execute(conn, `INSERT INTO shows (station_id, name, cached_at) VALUES (?, ?, ?)`, &sqlitex.ExecOptions{
			Args: []any{stationID, name, now},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// GetShowByName gets a show by station ID and name.
func (db *DB) GetShowByName(stationID int64, name string) (*Show, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	var show *Show
	err = sqlitex.Execute(conn, `SELECT id, station_id, name, cached_at FROM shows WHERE station_id = ? AND name = ?`, &sqlitex.ExecOptions{
		Args: []any{stationID, name},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			cachedAt, _ := time.Parse(time.RFC3339, stmt.ColumnText(3))
			show = &Show{
				ID:        stmt.ColumnInt64(0),
				StationID: stmt.ColumnInt64(1),
				Name:      stmt.ColumnText(2),
				CachedAt:  cachedAt,
			}
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	return show, nil
}

// GetCachedArchives returns cached archives for a show if still valid.
func (db *DB) GetCachedArchives(showID int64) ([]Archive, bool, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, false, err
	}
	defer db.pool.Put(conn)

	var archives []Archive
	var oldestCache time.Time

	err = sqlitex.Execute(conn, `SELECT id, show_id, date, m3u_url, cached_at FROM archives WHERE show_id = ? ORDER BY date DESC`, &sqlitex.ExecOptions{
		Args: []any{showID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			date, _ := time.Parse("2006-01-02", stmt.ColumnText(2))
			cachedAt, _ := time.Parse(time.RFC3339, stmt.ColumnText(4))
			if oldestCache.IsZero() || cachedAt.Before(oldestCache) {
				oldestCache = cachedAt
			}
			archives = append(archives, Archive{
				ID:       stmt.ColumnInt64(0),
				ShowID:   stmt.ColumnInt64(1),
				Date:     date,
				M3UURL:   stmt.ColumnText(3),
				CachedAt: cachedAt,
			})
			return nil
		},
	})
	if err != nil {
		return nil, false, err
	}

	if len(archives) == 0 {
		return nil, false, nil
	}

	valid := time.Since(oldestCache) < db.CacheTTL
	return archives, valid, nil
}

// CacheArchives caches archives for a show.
func (db *DB) CacheArchives(showID int64, archives []Archive) error {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer db.pool.Put(conn)

	now := time.Now().Format(time.RFC3339)

	// Delete old archives for this show
	err = sqlitex.Execute(conn, `DELETE FROM archives WHERE show_id = ?`, &sqlitex.ExecOptions{
		Args: []any{showID},
	})
	if err != nil {
		return err
	}

	// Insert new archives
	for _, a := range archives {
		err = sqlitex.Execute(conn, `INSERT INTO archives (show_id, date, m3u_url, cached_at) VALUES (?, ?, ?, ?)`, &sqlitex.ExecOptions{
			Args: []any{showID, a.Date.Format("2006-01-02"), a.M3UURL, now},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// InsertDownload inserts a new download record.
func (db *DB) InsertDownload(d *Download) (int64, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return 0, err
	}
	defer db.pool.Put(conn)

	status := d.Status
	if status == "" {
		status = StatusPending
	}
	createdAt := d.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	// filepath can be empty string (NULL in DB)
	var filepath any = nil
	if d.Filepath != "" {
		filepath = d.Filepath
	}

	err = sqlitex.Execute(conn, `INSERT INTO downloads (station_id, show_id, archive_date, m3u_url, filepath, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, &sqlitex.ExecOptions{
		Args: []any{
			d.StationID,
			d.ShowID,
			d.ArchiveDate.Format("2006-01-02"),
			d.M3UURL,
			filepath,
			status,
			createdAt.Format(time.RFC3339),
		},
	})
	if err != nil {
		return 0, err
	}

	return conn.LastInsertRowID(), nil
}

// ListDownloads returns all downloads, optionally filtered by station call sign.
func (db *DB) ListDownloads(callSign string) ([]Download, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	var query string
	var args []any

	if callSign == "" {
		query = `
			SELECT d.id, d.station_id, d.show_id, d.archive_date, d.m3u_url, d.filepath, d.status, d.error, d.created_at, d.updated_at,
			       s.call_sign, COALESCE(sh.name, '')
			FROM downloads d
			JOIN stations s ON d.station_id = s.id
			LEFT JOIN shows sh ON d.show_id = sh.id
			ORDER BY d.created_at DESC`
	} else {
		query = `
			SELECT d.id, d.station_id, d.show_id, d.archive_date, d.m3u_url, d.filepath, d.status, d.error, d.created_at, d.updated_at,
			       s.call_sign, COALESCE(sh.name, '')
			FROM downloads d
			JOIN stations s ON d.station_id = s.id
			LEFT JOIN shows sh ON d.show_id = sh.id
			WHERE s.call_sign = ?
			ORDER BY d.created_at DESC`
		args = []any{callSign}
	}

	return db.queryDownloads(conn, query, args)
}

// GetDownload returns a download by ID.
func (db *DB) GetDownload(id int64) (*Download, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	query := `
		SELECT d.id, d.station_id, d.show_id, d.archive_date, d.m3u_url, d.filepath, d.status, d.error, d.created_at, d.updated_at,
		       s.call_sign, COALESCE(sh.name, '')
		FROM downloads d
		JOIN stations s ON d.station_id = s.id
		LEFT JOIN shows sh ON d.show_id = sh.id
		WHERE d.id = ?`

	downloads, err := db.queryDownloads(conn, query, []any{id})
	if err != nil {
		return nil, err
	}
	if len(downloads) == 0 {
		return nil, fmt.Errorf("download not found: %d", id)
	}
	return &downloads[0], nil
}

// ListDownloadsByStatus returns downloads filtered by status values.
func (db *DB) ListDownloadsByStatus(statuses ...string) ([]Download, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	if len(statuses) == 0 {
		return nil, nil
	}

	// Build placeholders
	placeholders := ""
	args := make([]any, len(statuses))
	for i, s := range statuses {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args[i] = s
	}

	query := fmt.Sprintf(`
		SELECT d.id, d.station_id, d.show_id, d.archive_date, d.m3u_url, d.filepath, d.status, d.error, d.created_at, d.updated_at,
		       s.call_sign, COALESCE(sh.name, '')
		FROM downloads d
		JOIN stations s ON d.station_id = s.id
		LEFT JOIN shows sh ON d.show_id = sh.id
		WHERE d.status IN (%s)
		ORDER BY d.created_at DESC`, placeholders)

	return db.queryDownloads(conn, query, args)
}

// FindDownload finds an existing download for a station and archive date.
func (db *DB) FindDownload(stationID int64, archiveDate time.Time) (*Download, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	query := `
		SELECT d.id, d.station_id, d.show_id, d.archive_date, d.m3u_url, d.filepath, d.status, d.error, d.created_at, d.updated_at,
		       s.call_sign, COALESCE(sh.name, '')
		FROM downloads d
		JOIN stations s ON d.station_id = s.id
		LEFT JOIN shows sh ON d.show_id = sh.id
		WHERE d.station_id = ? AND d.archive_date = ?`

	downloads, err := db.queryDownloads(conn, query, []any{stationID, archiveDate.Format("2006-01-02")})
	if err != nil {
		return nil, err
	}
	if len(downloads) == 0 {
		return nil, nil
	}
	return &downloads[0], nil
}

// UpdateDownloadStatus updates the status, filepath, and error of a download.
func (db *DB) UpdateDownloadStatus(id int64, status, filepath, errMsg string) error {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer db.pool.Put(conn)

	now := time.Now().Format(time.RFC3339)

	var fp any = nil
	if filepath != "" {
		fp = filepath
	}

	var errVal any = nil
	if errMsg != "" {
		errVal = errMsg
	}

	return sqlitex.Execute(conn, `UPDATE downloads SET status = ?, filepath = ?, error = ?, updated_at = ? WHERE id = ?`, &sqlitex.ExecOptions{
		Args: []any{status, fp, errVal, now, id},
	})
}

// queryDownloads executes a download query and returns results.
func (db *DB) queryDownloads(conn *sqlite.Conn, query string, args []any) ([]Download, error) {
	var downloads []Download
	err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
		Args: args,
		ResultFunc: func(stmt *sqlite.Stmt) error {
			d := Download{
				ID:        stmt.ColumnInt64(0),
				StationID: stmt.ColumnInt64(1),
				M3UURL:    stmt.ColumnText(4),
				Filepath:  stmt.ColumnText(5),
				Status:    stmt.ColumnText(6),
				Error:     stmt.ColumnText(7),
				Station:   stmt.ColumnText(10),
				Show:      stmt.ColumnText(11),
			}

			if stmt.ColumnType(2) != sqlite.TypeNull {
				showID := stmt.ColumnInt64(2)
				d.ShowID = &showID
			}

			if date, err := time.Parse("2006-01-02", stmt.ColumnText(3)); err == nil {
				d.ArchiveDate = date
			}
			if t, err := time.Parse(time.RFC3339, stmt.ColumnText(8)); err == nil {
				d.CreatedAt = t
			}
			if t, err := time.Parse(time.RFC3339, stmt.ColumnText(9)); err == nil {
				d.UpdatedAt = t
			}

			downloads = append(downloads, d)
			return nil
		},
	})

	return downloads, err
}
