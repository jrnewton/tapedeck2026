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
DONE: Implement "download to device" option in GUI that will save audio file in browser local storage for offline listening.
    - Service Worker (sw.js) caches app shell for true PWA offline experience
    - IndexedDB wrapper (offline.js) stores audio blobs for offline playback
    - Download button on each tape spine with states: download arrow, spinner, checkmark (saved)
    - Offline-first playback: checks IndexedDB before fetching from network
    - Cache versioning: increment CACHE_VERSION in sw.js when deploying code changes
DONE: Improve API caching for faster offline loading.
    - Cache-first strategy: return cached data immediately, refresh in background
    - Eliminates offline delay from waiting for network timeouts
    - Auto-refresh UI: when background refresh detects new data, re-renders affected component
DONE: Fix path handling for cross-context database compatibility.
    - Database now stores only filenames (e.g., WMBR_ShowName_20260124.mp3)
    - Full paths constructed at runtime using TAPEDECK_DATA_DIR
    - Works across host CLI, Docker CLI, and Docker Web contexts
    - fix-downloads command migrates existing paths to filenames
DONE: Review deployment requirements and deploy to DigitalOcean.
    - Evaluated Fly.io vs DigitalOcean (DO cheaper long-term)
    - Created droplet: tapedeck (s-1vcpu-1gb, nyc1, $6/mo)
    - Deployed Docker container with existing data (630MB, 7 shows)
    - Configured Caddy reverse proxy with automatic Let's Encrypt HTTPS
    - Configured UFW firewall (22, 80, 443 only)
    - Docker bound to localhost:8080 (not exposed externally)
    - Live at: https://tapedeck.us
    - Deployment commands documented in README.md
    - Added make deploy target for one-command deployment
DONE: Fix iOS Safari audio playback.
    - Set Content-Type: audio/mpeg for MP3 files (Alpine's mime detection fails)
    - Bypass service worker for /api/audio/ requests (iOS redirect error)
    - Added debug mode toggle (DBG button) for error alerts
    - Improved IndexedDB error handling for offline storage
DONE: Fix offline mode UI not updating on show change.
    - Render functions now called in catch blocks when API fetch fails
    - Previously showed stale data when changing shows while offline with uncached data
DONE: Adjust page title based on station/show/download.
    - Title format: "Tapedeck - <context>" where context is station, show, or download
    - Updates dynamically when selecting station, show, or playing a download
    - Works with URL state restoration (direct links)
DONE: Add favicon.
    - Using cassette-tape.png from tapedeck2 repo
    - Cached by service worker for offline use
DONE: Adjust layout and add About modal.
    - Header now: TAPEDECK | About | DBG
    - Station and Show selectors in separate rows below header
    - About button opens mobile-friendly modal overlay with site description
    - Tap outside modal or X to close
DONE: Improve deployment speed.
    - Cross-compile Go binaries locally (CGO_ENABLED=0 for Alpine compatibility)
    - Deploy pre-built binaries instead of building on server
    - Reduced deploy time from ~2 minutes to ~10 seconds
DONE: Fix selected download decoration inconsistency on iOS.
    - Hover effects (shadow/slide) now only apply on devices with hover capability
    - Uses @media (hover: hover) to prevent sticky hover state on touch devices
    - Active class explicitly resets transform/box-shadow for clean state
DONE: UI polish for iOS Safari.
    - Tape spine accent color changed from red to original orange
    - Audio player progress bar thumb changed from orange to neutral grey
    - Play button icon fixed to render consistently with other transport buttons (text variation selector)
TODO: Have a "share" button that copies URL to current playing song.
TODO: Change scheduler implementation so user can schedule via GUI.
TODO: Review all test cases and test coverage.
TODO: Implement backend CLI for WHRB
TODO: Implement backend CLI for WUMB
TODO: Implement backend CLI for WOMR
TODO: Implement backend CLI for WCUW
