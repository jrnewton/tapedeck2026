# Summary
You are building a simple web application that is used to download radio station archive streams (typically stored in mp3 files).

# WORKLOG
WORKLOG.md contains all work items one per line in the following format:
<STATUS>: <ITEM>, where STATUS is TODO, PROG, DONE and ITEM is a short description of the work.

Indented under each work item can be more details as needed.

Eg
DONE: create DESIGN.MD
  determine languages to use
  deployment architecture
PROG: implement frontend
TODO: implement backend

# Design
See DESIGN.md for living document of the project's technical design.
The target OS and architecture is a docker container running under a Linux host system.
The UI is mobile first, prioritizing ios/safari.

# Git
You are working in a Git repository.  Pls commit after each iteration is done and tested.

# Go Code
- Project structure must be as simple as possible.
- Always ask when introducing third party code.

# Makefile
Use `make` for common tasks:
- `make build` - Build binaries
- `make run` - Run server via Docker Compose (port 8080)
- `make stop` - Stop server
- `make logs` - View server logs
- `make test` - Run unit tests
- `make test-e2e` - Run E2E tests (requires Docker with Chromium)
- `make clean` - Remove build artifacts

# Frontend Debugging
All console output (log, warn, error) and alerts in app.js must be controlled by the debug mode toggle (DBG button). Use these helper functions instead of console.* directly:
- `debugLog(...args)` - for info-level logging
- `debugWarn(...args)` - for warning-level logging
- `debugError(...args)` - for error-level logging
- `debugAlert(message)` - for user-visible alerts (also logs)

Debug mode is toggled via the DBG button in the UI and persisted in localStorage.

# Offline/PWA Versioning
The app uses two separate version numbers for offline functionality:

- **IndexedDB version** (`DB_VERSION` in `offline.js`): Controls the database schema. Only increment when you change the structure (add/remove object stores, change indexes). Existing saved audio blobs remain valid across schema-compatible versions.
- **SW Cache version** (`CACHE_VERSION` in `sw.js`): Controls which static files (HTML/JS/CSS) are cached. Increment when app code changes so users get the updated files.
