DONE: create DESIGN.md with high level architecture details.
DONE: setup local dev environment.
DONE: Implement backend CLI for WMBR
  - pkg/tapedeck/adapter.go - Adapter interface with Archive struct
  - internal/m3u/m3u.go - M3U parser with tests
  - internal/db/db.go - SQLite database layer with tests
  - internal/adapters/wmbr/wmbr.go - WMBR adapter with tests
  - pkg/tapedeck/tapedeck.go - Core library with adapter registry
  - cmd/tapedeck-cli/main.go - CLI with list-shows, list-downloads, download-show
TODO: Implement backend CLI for remaining stations (WHRB, WUMB, WOMR, WCUW)
TODO: Build frontend GUI
