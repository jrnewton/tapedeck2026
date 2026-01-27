package db

import (
	"context"
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

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
	Active    bool
	// Denormalized archive data (current and previous)
	ArchiveCurrentDate    *time.Time
	ArchiveCurrentM3UURL  string
	ArchivePreviousDate   *time.Time
	ArchivePreviousM3UURL string
}


// Download status constants.
const (
	StatusPending     = "pending"
	StatusDownloading = "downloading"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
)

// Schedule status constants.
const (
	ScheduleStatusSuccess  = "success"
	ScheduleStatusFailed   = "failed"
	ScheduleStatusSkipped  = "skipped"
	ScheduleStatusRetrying = "retrying"
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

// Schedule represents a scheduled download job.
type Schedule struct {
	ID             int64
	StationID      int64
	ShowID         int64
	CronExpression string
	Enabled        bool
	LastRunAt      *time.Time
	LastStatus     string
	LastError      string
	RetryCount     int
	NextRetryAt    *time.Time
	NextRunAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// Denormalized fields for display
	Station string
	Show    string
}

// DB wraps a SQLite connection pool.
type DB struct {
	pool *sqlitex.Pool
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

	db := &DB{pool: pool}
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

		CREATE TABLE IF NOT EXISTS schedules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			station_id INTEGER NOT NULL REFERENCES stations(id),
			show_id INTEGER NOT NULL REFERENCES shows(id),
			cron_expression TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			last_run_at TEXT,
			last_status TEXT,
			last_error TEXT,
			retry_count INTEGER NOT NULL DEFAULT 0,
			next_retry_at TEXT,
			next_run_at TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(station_id, show_id)
		);

		CREATE INDEX IF NOT EXISTS idx_shows_station ON shows(station_id);
		CREATE INDEX IF NOT EXISTS idx_archives_show ON archives(show_id);
		CREATE INDEX IF NOT EXISTS idx_downloads_station ON downloads(station_id);
		CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
		CREATE INDEX IF NOT EXISTS idx_schedules_next_run ON schedules(next_run_at);
		CREATE INDEX IF NOT EXISTS idx_schedules_enabled ON schedules(enabled);
	`
	if err := sqlitex.ExecuteScript(conn, schema, nil); err != nil {
		return err
	}

	// Migration: add active column to shows if not exists
	// SQLite doesn't support IF NOT EXISTS for ALTER TABLE, so we check first
	var hasActive bool
	err = sqlitex.Execute(conn, `SELECT 1 FROM pragma_table_info('shows') WHERE name='active'`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			hasActive = true
			return nil
		},
	})
	if err != nil {
		return err
	}
	if !hasActive {
		if err := sqlitex.Execute(conn, `ALTER TABLE shows ADD COLUMN active INTEGER NOT NULL DEFAULT 1`, nil); err != nil {
			return err
		}
	}

	// Migration: add denormalized archive columns to shows
	var hasArchiveCurrent bool
	err = sqlitex.Execute(conn, `SELECT 1 FROM pragma_table_info('shows') WHERE name='archive_current_date'`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			hasArchiveCurrent = true
			return nil
		},
	})
	if err != nil {
		return err
	}
	if !hasArchiveCurrent {
		// Add all 4 archive columns
		archiveColumns := []string{
			`ALTER TABLE shows ADD COLUMN archive_current_date TEXT`,
			`ALTER TABLE shows ADD COLUMN archive_current_m3u_url TEXT`,
			`ALTER TABLE shows ADD COLUMN archive_previous_date TEXT`,
			`ALTER TABLE shows ADD COLUMN archive_previous_m3u_url TEXT`,
		}
		for _, sql := range archiveColumns {
			if err := sqlitex.Execute(conn, sql, nil); err != nil {
				return err
			}
		}
	}

	// Migration: drop unused archives table
	if err := sqlitex.Execute(conn, `DROP TABLE IF EXISTS archives`, nil); err != nil {
		return err
	}
	if err := sqlitex.Execute(conn, `DROP INDEX IF EXISTS idx_archives_show`, nil); err != nil {
		return err
	}

	return nil
}

// ListStations returns all registered stations.
func (db *DB) ListStations() ([]Station, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	var stations []Station
	err = sqlitex.Execute(conn, `SELECT id, call_sign, name, archive_url FROM stations ORDER BY call_sign`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			stations = append(stations, Station{
				ID:         stmt.ColumnInt64(0),
				CallSign:   stmt.ColumnText(1),
				Name:       stmt.ColumnText(2),
				ArchiveURL: stmt.ColumnText(3),
			})
			return nil
		},
	})
	return stations, err
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

// GetShows returns active shows for a station.
// Only returns active shows (shows currently available from the station).
func (db *DB) GetShows(stationID int64) ([]Show, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	var shows []Show

	err = sqlitex.Execute(conn, `SELECT id, station_id, name, cached_at, active, archive_current_date, archive_current_m3u_url, archive_previous_date, archive_previous_m3u_url FROM shows WHERE station_id = ? AND active = 1 ORDER BY name`, &sqlitex.ExecOptions{
		Args: []any{stationID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			cachedAt, _ := time.Parse(time.RFC3339, stmt.ColumnText(3))
			show := Show{
				ID:                    stmt.ColumnInt64(0),
				StationID:             stmt.ColumnInt64(1),
				Name:                  stmt.ColumnText(2),
				CachedAt:              cachedAt,
				Active:                stmt.ColumnInt(4) == 1,
				ArchiveCurrentM3UURL:  stmt.ColumnText(6),
				ArchivePreviousM3UURL: stmt.ColumnText(8),
			}
			if stmt.ColumnType(5) != sqlite.TypeNull {
				if d, parseErr := time.Parse("2006-01-02", stmt.ColumnText(5)); parseErr == nil {
					show.ArchiveCurrentDate = &d
				}
			}
			if stmt.ColumnType(7) != sqlite.TypeNull {
				if d, parseErr := time.Parse("2006-01-02", stmt.ColumnText(7)); parseErr == nil {
					show.ArchivePreviousDate = &d
				}
			}
			shows = append(shows, show)
			return nil
		},
	})
	if err != nil {
		return nil, err
	}

	return shows, nil
}

// ListShowsWithDownloads returns shows for a station that have at least one download.
// Returns both active and inactive shows (inactive shows may still have valid downloads).
func (db *DB) ListShowsWithDownloads(stationID int64) ([]Show, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	var shows []Show
	err = sqlitex.Execute(conn, `
		SELECT DISTINCT s.id, s.station_id, s.name, s.cached_at, s.active,
			s.archive_current_date, s.archive_current_m3u_url, s.archive_previous_date, s.archive_previous_m3u_url
		FROM shows s
		INNER JOIN downloads d ON d.show_id = s.id
		WHERE s.station_id = ?
		ORDER BY s.name`, &sqlitex.ExecOptions{
		Args: []any{stationID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			cachedAt, _ := time.Parse(time.RFC3339, stmt.ColumnText(3))
			show := Show{
				ID:                    stmt.ColumnInt64(0),
				StationID:             stmt.ColumnInt64(1),
				Name:                  stmt.ColumnText(2),
				CachedAt:              cachedAt,
				Active:                stmt.ColumnInt(4) == 1,
				ArchiveCurrentM3UURL:  stmt.ColumnText(6),
				ArchivePreviousM3UURL: stmt.ColumnText(8),
			}
			if stmt.ColumnType(5) != sqlite.TypeNull {
				if d, parseErr := time.Parse("2006-01-02", stmt.ColumnText(5)); parseErr == nil {
					show.ArchiveCurrentDate = &d
				}
			}
			if stmt.ColumnType(7) != sqlite.TypeNull {
				if d, parseErr := time.Parse("2006-01-02", stmt.ColumnText(7)); parseErr == nil {
					show.ArchivePreviousDate = &d
				}
			}
			shows = append(shows, show)
			return nil
		},
	})
	return shows, err
}

// InsertShow inserts a new show record, returning the show ID.
func (db *DB) InsertShow(stationID int64, name string) (int64, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return 0, err
	}
	defer db.pool.Put(conn)

	now := time.Now().Format(time.RFC3339)

	err = sqlitex.Execute(conn, `
		INSERT INTO shows (station_id, name, cached_at, active)
		VALUES (?, ?, ?, 1)
		ON CONFLICT(station_id, name) DO UPDATE SET
			cached_at = excluded.cached_at,
			active = 1`, &sqlitex.ExecOptions{
		Args: []any{stationID, name, now},
	})
	if err != nil {
		return 0, err
	}

	return conn.LastInsertRowID(), nil
}

// GetShowByID gets a show by ID.
func (db *DB) GetShowByID(showID int64) (*Show, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	var show *Show
	err = sqlitex.Execute(conn, `SELECT id, station_id, name, cached_at, active, archive_current_date, archive_current_m3u_url, archive_previous_date, archive_previous_m3u_url FROM shows WHERE id = ?`, &sqlitex.ExecOptions{
		Args: []any{showID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			cachedAt, _ := time.Parse(time.RFC3339, stmt.ColumnText(3))
			show = &Show{
				ID:                    stmt.ColumnInt64(0),
				StationID:             stmt.ColumnInt64(1),
				Name:                  stmt.ColumnText(2),
				CachedAt:              cachedAt,
				Active:                stmt.ColumnInt(4) == 1,
				ArchiveCurrentM3UURL:  stmt.ColumnText(6),
				ArchivePreviousM3UURL: stmt.ColumnText(8),
			}
			if stmt.ColumnType(5) != sqlite.TypeNull {
				if d, parseErr := time.Parse("2006-01-02", stmt.ColumnText(5)); parseErr == nil {
					show.ArchiveCurrentDate = &d
				}
			}
			if stmt.ColumnType(7) != sqlite.TypeNull {
				if d, parseErr := time.Parse("2006-01-02", stmt.ColumnText(7)); parseErr == nil {
					show.ArchivePreviousDate = &d
				}
			}
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	return show, nil
}

// GetShowByName gets a show by station ID and name.
// Returns the show regardless of active status.
func (db *DB) GetShowByName(stationID int64, name string) (*Show, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	var show *Show
	err = sqlitex.Execute(conn, `SELECT id, station_id, name, cached_at, active, archive_current_date, archive_current_m3u_url, archive_previous_date, archive_previous_m3u_url FROM shows WHERE station_id = ? AND name = ?`, &sqlitex.ExecOptions{
		Args: []any{stationID, name},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			cachedAt, _ := time.Parse(time.RFC3339, stmt.ColumnText(3))
			show = &Show{
				ID:                    stmt.ColumnInt64(0),
				StationID:             stmt.ColumnInt64(1),
				Name:                  stmt.ColumnText(2),
				CachedAt:              cachedAt,
				Active:                stmt.ColumnInt(4) == 1,
				ArchiveCurrentM3UURL:  stmt.ColumnText(6),
				ArchivePreviousM3UURL: stmt.ColumnText(8),
			}
			if stmt.ColumnType(5) != sqlite.TypeNull {
				if d, parseErr := time.Parse("2006-01-02", stmt.ColumnText(5)); parseErr == nil {
					show.ArchiveCurrentDate = &d
				}
			}
			if stmt.ColumnType(7) != sqlite.TypeNull {
				if d, parseErr := time.Parse("2006-01-02", stmt.ColumnText(7)); parseErr == nil {
					show.ArchivePreviousDate = &d
				}
			}
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	return show, nil
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

	// show_id is nullable
	var showID any = nil
	if d.ShowID != nil {
		showID = *d.ShowID
	}

	err = sqlitex.Execute(conn, `INSERT INTO downloads (station_id, show_id, archive_date, m3u_url, filepath, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, &sqlitex.ExecOptions{
		Args: []any{
			d.StationID,
			showID,
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

// LinkDownloadToShow updates a download's show_id.
func (db *DB) LinkDownloadToShow(downloadID int64, showID int64) error {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer db.pool.Put(conn)

	return sqlitex.Execute(conn, `UPDATE downloads SET show_id = ?, updated_at = ? WHERE id = ?`, &sqlitex.ExecOptions{
		Args: []any{showID, time.Now().Format(time.RFC3339), downloadID},
	})
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

// ListDownloadsByShowID returns downloads for a specific show, optionally filtered by status.
func (db *DB) ListDownloadsByShowID(showID int64, status string) ([]Download, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	var query string
	var args []any

	if status == "" {
		query = `
			SELECT d.id, d.station_id, d.show_id, d.archive_date, d.m3u_url, d.filepath, d.status, d.error, d.created_at, d.updated_at,
			       s.call_sign, COALESCE(sh.name, '')
			FROM downloads d
			JOIN stations s ON d.station_id = s.id
			LEFT JOIN shows sh ON d.show_id = sh.id
			WHERE d.show_id = ?
			ORDER BY d.archive_date DESC`
		args = []any{showID}
	} else {
		query = `
			SELECT d.id, d.station_id, d.show_id, d.archive_date, d.m3u_url, d.filepath, d.status, d.error, d.created_at, d.updated_at,
			       s.call_sign, COALESCE(sh.name, '')
			FROM downloads d
			JOIN stations s ON d.station_id = s.id
			LEFT JOIN shows sh ON d.show_id = sh.id
			WHERE d.show_id = ? AND d.status = ?
			ORDER BY d.archive_date DESC`
		args = []any{showID, status}
	}

	return db.queryDownloads(conn, query, args)
}

// FindDownload finds an existing download for a station, show, and archive date.
func (db *DB) FindDownload(stationID int64, showID *int64, archiveDate time.Time) (*Download, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	var query string
	var args []any

	if showID != nil {
		query = `
			SELECT d.id, d.station_id, d.show_id, d.archive_date, d.m3u_url, d.filepath, d.status, d.error, d.created_at, d.updated_at,
			       s.call_sign, COALESCE(sh.name, '')
			FROM downloads d
			JOIN stations s ON d.station_id = s.id
			LEFT JOIN shows sh ON d.show_id = sh.id
			WHERE d.station_id = ? AND d.show_id = ? AND d.archive_date = ?`
		args = []any{stationID, *showID, archiveDate.Format("2006-01-02")}
	} else {
		query = `
			SELECT d.id, d.station_id, d.show_id, d.archive_date, d.m3u_url, d.filepath, d.status, d.error, d.created_at, d.updated_at,
			       s.call_sign, COALESCE(sh.name, '')
			FROM downloads d
			JOIN stations s ON d.station_id = s.id
			LEFT JOIN shows sh ON d.show_id = sh.id
			WHERE d.station_id = ? AND d.show_id IS NULL AND d.archive_date = ?`
		args = []any{stationID, archiveDate.Format("2006-01-02")}
	}

	downloads, err := db.queryDownloads(conn, query, args)
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

// InsertSchedule inserts a new schedule record.
func (db *DB) InsertSchedule(s *Schedule) (int64, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return 0, err
	}
	defer db.pool.Put(conn)

	now := time.Now().Format(time.RFC3339)

	var nextRunAt any = nil
	if s.NextRunAt != nil {
		nextRunAt = s.NextRunAt.Format(time.RFC3339)
	}

	enabled := 1
	if !s.Enabled {
		enabled = 0
	}

	err = sqlitex.Execute(conn, `INSERT INTO schedules (station_id, show_id, cron_expression, enabled, next_run_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, &sqlitex.ExecOptions{
		Args: []any{
			s.StationID,
			s.ShowID,
			s.CronExpression,
			enabled,
			nextRunAt,
			now,
			now,
		},
	})
	if err != nil {
		return 0, err
	}

	return conn.LastInsertRowID(), nil
}

// GetSchedule returns a schedule by ID.
func (db *DB) GetSchedule(id int64) (*Schedule, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	query := `
		SELECT s.id, s.station_id, s.show_id, s.cron_expression, s.enabled,
		       s.last_run_at, s.last_status, s.last_error, s.retry_count, s.next_retry_at, s.next_run_at,
		       s.created_at, s.updated_at, st.call_sign, sh.name
		FROM schedules s
		JOIN stations st ON s.station_id = st.id
		JOIN shows sh ON s.show_id = sh.id
		WHERE s.id = ?`

	schedules, err := db.querySchedules(conn, query, []any{id})
	if err != nil {
		return nil, err
	}
	if len(schedules) == 0 {
		return nil, fmt.Errorf("schedule not found: %d", id)
	}
	return &schedules[0], nil
}

// ListSchedules returns all schedules.
func (db *DB) ListSchedules() ([]Schedule, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	query := `
		SELECT s.id, s.station_id, s.show_id, s.cron_expression, s.enabled,
		       s.last_run_at, s.last_status, s.last_error, s.retry_count, s.next_retry_at, s.next_run_at,
		       s.created_at, s.updated_at, st.call_sign, sh.name
		FROM schedules s
		JOIN stations st ON s.station_id = st.id
		JOIN shows sh ON s.show_id = sh.id
		ORDER BY sh.name`

	return db.querySchedules(conn, query, nil)
}

// ListDueSchedules returns schedules that are due to run (next_run_at <= now and enabled).
func (db *DB) ListDueSchedules(now time.Time) ([]Schedule, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	query := `
		SELECT s.id, s.station_id, s.show_id, s.cron_expression, s.enabled,
		       s.last_run_at, s.last_status, s.last_error, s.retry_count, s.next_retry_at, s.next_run_at,
		       s.created_at, s.updated_at, st.call_sign, sh.name
		FROM schedules s
		JOIN stations st ON s.station_id = st.id
		JOIN shows sh ON s.show_id = sh.id
		WHERE s.enabled = 1 AND s.next_run_at IS NOT NULL AND s.next_run_at <= ?
		ORDER BY s.next_run_at`

	return db.querySchedules(conn, query, []any{now.Format(time.RFC3339)})
}

// ListRetrySchedules returns schedules that are due for retry (next_retry_at <= now and enabled).
func (db *DB) ListRetrySchedules(now time.Time) ([]Schedule, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	query := `
		SELECT s.id, s.station_id, s.show_id, s.cron_expression, s.enabled,
		       s.last_run_at, s.last_status, s.last_error, s.retry_count, s.next_retry_at, s.next_run_at,
		       s.created_at, s.updated_at, st.call_sign, sh.name
		FROM schedules s
		JOIN stations st ON s.station_id = st.id
		JOIN shows sh ON s.show_id = sh.id
		WHERE s.enabled = 1 AND s.next_retry_at IS NOT NULL AND s.next_retry_at <= ?
		ORDER BY s.next_retry_at`

	return db.querySchedules(conn, query, []any{now.Format(time.RFC3339)})
}

// UpdateScheduleStatus updates the status fields of a schedule after a run.
func (db *DB) UpdateScheduleStatus(id int64, status, errMsg string, nextRunAt, nextRetryAt *time.Time, retryCount int) error {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer db.pool.Put(conn)

	now := time.Now()

	var nextRun any = nil
	if nextRunAt != nil {
		nextRun = nextRunAt.Format(time.RFC3339)
	}

	var nextRetry any = nil
	if nextRetryAt != nil {
		nextRetry = nextRetryAt.Format(time.RFC3339)
	}

	var errVal any = nil
	if errMsg != "" {
		errVal = errMsg
	}

	return sqlitex.Execute(conn, `UPDATE schedules SET last_run_at = ?, last_status = ?, last_error = ?, retry_count = ?, next_retry_at = ?, next_run_at = ?, updated_at = ? WHERE id = ?`, &sqlitex.ExecOptions{
		Args: []any{now.Format(time.RFC3339), status, errVal, retryCount, nextRetry, nextRun, now.Format(time.RFC3339), id},
	})
}

// UpdateScheduleEnabled updates the enabled flag of a schedule.
func (db *DB) UpdateScheduleEnabled(id int64, enabled bool) error {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer db.pool.Put(conn)

	enabledVal := 0
	if enabled {
		enabledVal = 1
	}

	return sqlitex.Execute(conn, `UPDATE schedules SET enabled = ?, updated_at = ? WHERE id = ?`, &sqlitex.ExecOptions{
		Args: []any{enabledVal, time.Now().Format(time.RFC3339), id},
	})
}

// DeleteSchedule removes a schedule by ID.
func (db *DB) DeleteSchedule(id int64) error {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer db.pool.Put(conn)

	return sqlitex.Execute(conn, `DELETE FROM schedules WHERE id = ?`, &sqlitex.ExecOptions{
		Args: []any{id},
	})
}

// FindScheduleByShowID finds a schedule for a specific station and show.
func (db *DB) FindScheduleByShowID(stationID, showID int64) (*Schedule, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	query := `
		SELECT s.id, s.station_id, s.show_id, s.cron_expression, s.enabled,
		       s.last_run_at, s.last_status, s.last_error, s.retry_count, s.next_retry_at, s.next_run_at,
		       s.created_at, s.updated_at, st.call_sign, sh.name
		FROM schedules s
		JOIN stations st ON s.station_id = st.id
		JOIN shows sh ON s.show_id = sh.id
		WHERE s.station_id = ? AND s.show_id = ?`

	schedules, err := db.querySchedules(conn, query, []any{stationID, showID})
	if err != nil {
		return nil, err
	}
	if len(schedules) == 0 {
		return nil, nil
	}
	return &schedules[0], nil
}

// querySchedules executes a schedule query and returns results.
func (db *DB) querySchedules(conn *sqlite.Conn, query string, args []any) ([]Schedule, error) {
	var schedules []Schedule
	err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
		Args: args,
		ResultFunc: func(stmt *sqlite.Stmt) error {
			s := Schedule{
				ID:             stmt.ColumnInt64(0),
				StationID:      stmt.ColumnInt64(1),
				ShowID:         stmt.ColumnInt64(2),
				CronExpression: stmt.ColumnText(3),
				Enabled:        stmt.ColumnInt(4) == 1,
				LastStatus:     stmt.ColumnText(6),
				LastError:      stmt.ColumnText(7),
				RetryCount:     stmt.ColumnInt(8),
				Station:        stmt.ColumnText(13),
				Show:           stmt.ColumnText(14),
			}

			if stmt.ColumnType(5) != sqlite.TypeNull {
				if t, err := time.Parse(time.RFC3339, stmt.ColumnText(5)); err == nil {
					s.LastRunAt = &t
				}
			}
			if stmt.ColumnType(9) != sqlite.TypeNull {
				if t, err := time.Parse(time.RFC3339, stmt.ColumnText(9)); err == nil {
					s.NextRetryAt = &t
				}
			}
			if stmt.ColumnType(10) != sqlite.TypeNull {
				if t, err := time.Parse(time.RFC3339, stmt.ColumnText(10)); err == nil {
					s.NextRunAt = &t
				}
			}
			if t, err := time.Parse(time.RFC3339, stmt.ColumnText(11)); err == nil {
				s.CreatedAt = t
			}
			if t, err := time.Parse(time.RFC3339, stmt.ColumnText(12)); err == nil {
				s.UpdatedAt = t
			}

			schedules = append(schedules, s)
			return nil
		},
	})

	return schedules, err
}
