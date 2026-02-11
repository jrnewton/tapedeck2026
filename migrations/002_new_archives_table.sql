-- Migration 002: Create archives table
-- Stores archive entries discovered during station refresh operations.
-- Each refresh creates one row per active show with its archive URL.

CREATE TABLE IF NOT EXISTS archives (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	show_id INTEGER NOT NULL REFERENCES shows(id),
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	archive_date TEXT NOT NULL,
	archive_url TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_archives_show ON archives(show_id);
