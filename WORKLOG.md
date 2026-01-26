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
DONE: Implement schedule-download CLI command
DONE: Implement cron scheduler to download specific show archives each week
    - schedule-download <STATION> <SHOW> - generate crontab line for automated downloads
      - Analyzes archive history to infer broadcast schedule
      - Outputs cron line with 2.5hr delay for archive availability
      - Handles late-night rollover (e.g., 23:00 -> 01:30 next day)
      - Confidence levels based on schedule consistency
      - Includes ready-to-run crontab install command
DONE: Build frontend GUI that allow for download playback in browser
    - You select a station and a show (which has downloads) and then
      get a UI which plays the saved audio file on disk.
    - REST API: /api/stations, /api/stations/:call/shows, /api/downloads,
      /api/downloads/:id, /api/shows/:id/downloads, /api/audio/:id
    - Retro cassette tape design with spinning reels
    - HTML5 audio player with Range request support for seeking
    - Fixed: shows dropdown only shows shows with downloads
    - Fixed: CLI uses TAPEDECK_DATA_DIR env var for consistent DB and downloads path
    - Fixed: UTC timezone handling for date display in collection
    - Added: fix-downloads CLI command to repair unlinked downloads and relative paths
    - Added: E2E browser tests using chromedp (run in Docker with Chromium)
DONE: Adjust GUI design so UI choice are saved in URL
    - URL params: ?station=WMBR&show=1&play=1
    - Selecting station/show/track updates URL via history.pushState
    - Direct navigation to URL restores full UI state
    - Browser back/forward navigation supported via popstate
    - Track loads without autoplay when restoring from URL (avoids browser policy issues)
    - E2E tests for URL state updates and restoration
DONE: Adjust GUI design for mobile-first.
    - Replaced desktop UI with mobile-first design as default
    - Layout: compact header with station selector, full-width show dropdown,
      scrollable tape collection as main focus, sticky mini-player at bottom
    - Touch-friendly styles (44px min touch targets)
    - Subtle light pink highlight for selected tape spine (red/white base)
    - TAPEDECK header links to "/" for navigation
    - Removed redundant play button from tape spines
    - Optimized for iPhone 12 Pro viewport (390x844)
TODO: Implement "download to device" option in GUI that will save audio file in browser local storage for offline listening.
TODO: Implement "About" button
TODO: Review all test cases and test coverage.
TODO: Change scheduler implementation so user can schedule via GUI.
TODO: Review deployment requirements and begin planning hosting on cloudflare.
TODO: Implement backend CLI for WHRB
TODO: Implement backend CLI for WUMB
TODO: Implement backend CLI for WOMR
TODO: Implement backend CLI for WCUW
