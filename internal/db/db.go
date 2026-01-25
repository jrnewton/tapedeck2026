package db

import (
	"context"
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// Download represents a downloaded archive record.
type Download struct {
	ID        int64
	Station   string
	Show      string
	Date      time.Time
	Filepath  string
	Status    string
	CreatedAt time.Time
}

// DB wraps a SQLite connection pool.
type DB struct {
	pool *sqlitex.Pool
}

// Open opens or creates a SQLite database at the given path.
// Use ":memory:" for an in-memory database.
func Open(path string) (*DB, error) {
	// Handle in-memory database
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
		CREATE TABLE IF NOT EXISTS downloads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			station TEXT NOT NULL,
			show TEXT NOT NULL,
			date TEXT NOT NULL,
			filepath TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'completed',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_downloads_station ON downloads(station);
		CREATE INDEX IF NOT EXISTS idx_downloads_show ON downloads(show);
	`
	return sqlitex.ExecuteScript(conn, schema, nil)
}

// InsertDownload inserts a new download record.
func (db *DB) InsertDownload(d *Download) (int64, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return 0, err
	}
	defer db.pool.Put(conn)

	const query = `
		INSERT INTO downloads (station, show, date, filepath, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	status := d.Status
	if status == "" {
		status = "completed"
	}
	createdAt := d.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	err = sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
		Args: []any{
			d.Station,
			d.Show,
			d.Date.Format("2006-01-02"),
			d.Filepath,
			status,
			createdAt.Format(time.RFC3339),
		},
	})
	if err != nil {
		return 0, err
	}

	return conn.LastInsertRowID(), nil
}

// ListDownloads returns all downloads, optionally filtered by station.
func (db *DB) ListDownloads(station string) ([]Download, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	var query string
	var args []any

	if station == "" {
		query = `SELECT id, station, show, date, filepath, status, created_at FROM downloads ORDER BY created_at DESC`
	} else {
		query = `SELECT id, station, show, date, filepath, status, created_at FROM downloads WHERE station = ? ORDER BY created_at DESC`
		args = []any{station}
	}

	var downloads []Download
	err = sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
		Args: args,
		ResultFunc: func(stmt *sqlite.Stmt) error {
			d := Download{
				ID:       stmt.ColumnInt64(0),
				Station:  stmt.ColumnText(1),
				Show:     stmt.ColumnText(2),
				Filepath: stmt.ColumnText(4),
				Status:   stmt.ColumnText(5),
			}

			dateStr := stmt.ColumnText(3)
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				d.Date = t
			}

			createdStr := stmt.ColumnText(6)
			if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
				d.CreatedAt = t
			}

			downloads = append(downloads, d)
			return nil
		},
	})

	return downloads, err
}

// GetDownload returns a download by ID.
func (db *DB) GetDownload(id int64) (*Download, error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer db.pool.Put(conn)

	const query = `SELECT id, station, show, date, filepath, status, created_at FROM downloads WHERE id = ?`

	var d *Download
	err = sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
		Args: []any{id},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			d = &Download{
				ID:       stmt.ColumnInt64(0),
				Station:  stmt.ColumnText(1),
				Show:     stmt.ColumnText(2),
				Filepath: stmt.ColumnText(4),
				Status:   stmt.ColumnText(5),
			}

			dateStr := stmt.ColumnText(3)
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				d.Date = t
			}

			createdStr := stmt.ColumnText(6)
			if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
				d.CreatedAt = t
			}

			return nil
		},
	})

	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, fmt.Errorf("download not found: %d", id)
	}

	return d, nil
}
