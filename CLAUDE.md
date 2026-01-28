# Summary
You are building a simple web application that is used to download radio station archive streams (typically stored in mp3 files).

# WORKLOG
WORKLOG.md contains all work items one per line in the following format:
<STATUS>: <ITEM>

Where STATUS is TODO, PROG, DONE and ITEM is a short description of the work.
Indented under each work item can be more details as needed.

# Design
DESIGN.md is the living document of the project's technical design. Keep it updated.
Target architecture is a docker container running under a Linux host system.
The UI is mobile first, prioritizing Safari on IOS and PWA.

# Git
You are working in a Git repository.  Pls commit after each iteration is done and tested.  Prefix your commit message with the robot emojii Eg "🤖 fixed foobar" 

# Go Code
- Project structure must be as simple as possible.
- Always ask when introducing third party code.

# Makefile
Use `make` for common tasks.

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
