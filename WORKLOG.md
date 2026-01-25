DONE: create DESIGN.md with high level architecture details.
DONE: setup local dev environment.
DONE: Implement backend CLI for WMBR
  - pkg/tapedeck/adapter.go - Adapter interface with Archive struct
  - internal/m3u/m3u.go - M3U parser with tests
  - internal/db/db.go - SQLite database layer with tests
    - Relational schema: stations, shows, archives, downloads
    - Show/archive caching with TTL (1 hour default)
    - Download status tracking (pending/downloading/completed/failed)
    - Duplicate detection (FindDownload)
  - internal/adapters/wmbr/wmbr.go - WMBR adapter with tests
    - URL decoding for show names
  - pkg/tapedeck/tapedeck.go - Core library with adapter registry
  - cmd/tapedeck-cli/main.go - CLI commands:
    - list-shows <STATION> - with caching
    - list-downloads [STATION]
    - download-show <STATION> <SHOW> [--latest|--date] - blocking, with duplicate detection
    - download-status [ID] - show progress/status
TODO: Implement backend CLI for remaining stations (WHRB, WUMB, WOMR, WCUW)
TODO: Build frontend GUI
