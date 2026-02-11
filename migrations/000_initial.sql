		CREATE TABLE IF NOT EXISTS stations (
			id INTEGER PRIMARY KEY,
			call_sign TEXT UNIQUE NOT NULL,
			name TEXT,
			archive_url TEXT,
			timezone TEXT NOT NULL DEFAULT 'America/New_York'
		);

		CREATE TABLE IF NOT EXISTS shows (
			id INTEGER PRIMARY KEY,
			station_id INTEGER NOT NULL REFERENCES stations(id),
			name TEXT NOT NULL,
			cached_at TEXT NOT NULL,
			active INTEGER NOT NULL DEFAULT 1,
			UNIQUE(station_id, name)
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
		CREATE INDEX IF NOT EXISTS idx_downloads_station ON downloads(station_id);
		CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
		CREATE INDEX IF NOT EXISTS idx_schedules_next_run ON schedules(next_run_at);
		CREATE INDEX IF NOT EXISTS idx_schedules_enabled ON schedules(enabled);