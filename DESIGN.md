# Tapedeck.us - Design Document

## Overview

A web application for downloading radio show archives (mp3 files) from station archive APIs.

## Concepts

- **Station**: A radio station identified by its call sign (e.g., WMBR, KEXP)
- **Show**: A radio program that airs on a station (e.g., "Backwoods", "Morning Show")
- **Archive**: A recorded episode of a show, identified by date

## Tech Stack

| Component | Technology |
|-----------|------------|
| Backend   | Go         |
| Database  | SQLite     |
| Frontend  | HTML/CSS/Vanilla JS |
| Container | Docker     |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                       Docker Host                            │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                  Docker Container                      │  │
│  │                                                        │  │
│  │  ┌──────────────────────────────────────────────────┐ │  │
│  │  │              REST API (HTTP Server)              │ │  │
│  │  │         Serves web frontend + JSON API           │ │  │
│  │  └──────────────────────┬───────────────────────────┘ │  │
│  │                         │                              │  │
│  │  ┌──────────────────────▼───────────────────────────┐ │  │
│  │  │           Core Library (tapedeck pkg)            │ │  │
│  │  │      Headless, testable, cron-compatible         │ │  │
│  │  │  ┌─────────────┐  ┌────────────┐  ┌──────────┐  │ │  │
│  │  │  │  Scheduler  │  │  Recorder  │  │ Adapters │  │ │  │
│  │  │  └─────────────┘  └────────────┘  └──────────┘  │ │  │
│  │  │                         │                        │ │  │
│  │  │                  ┌──────▼──────┐                 │ │  │
│  │  │                  │   SQLite    │                 │ │  │
│  │  │                  └─────────────┘                 │ │  │
│  │  └──────────────────────────────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────┘  │
│                              │                               │
│                    ┌─────────▼─────────┐                     │
│                    │  Volume: /data    │                     │
│                    │  - tapedeck.db    │                     │
│                    │  - downloads/     │                     │
│                    └───────────────────┘                     │
└─────────────────────────────────────────────────────────────┘
```

## Components

### Core Library (`tapedeck` package)

The core library is fully headless and testable. It can be used:
- Programmatically from Go code
- Via CLI for cron-scheduled recordings
- Via REST API for web frontend

1. **Scheduler**
   - Manages download schedules
   - Manages live record capture schedules (when no archives exist)
   - Triggers downloads and recordings at specified times
   - Cron-compatible for automated capture

2. **Recorder**
   - Downloads/captures streams
   - Job queue for managing active recordings
   - Progress tracking

3. **Station Adapters**
   - Pluggable adapters (one per radio station archive source)
   - Each adapter handles: listing available shows, downloading archives

### REST API (HTTP Server)

Thin layer over the core library:
- Serves static frontend files (HTML/CSS/JS)
- JSON API for frontend communication
- Single binary deployment

### Database (SQLite)

Tables:
- `stations` - radio stations (call sign, name, adapter type)
- `shows` - shows per station (name, schedule info)
- `downloads` - download history and status
- `jobs` - pending/active download queue

### Frontend (HTML/CSS/Vanilla JS)

- Single page application
- Views: Dashboard, Downloads, Settings
- No build step required

## REST API Endpoints

```
GET  /api/stations              - List configured stations
GET  /api/stations/:call/shows  - List available shows for a station

POST /api/downloads             - Queue a download
GET  /api/downloads             - List download history
GET  /api/downloads/:id         - Get download status
DELETE /api/downloads/:id       - Cancel/remove download

GET  /api/schedules             - List scheduled downloads
POST /api/schedules             - Create a schedule
DELETE /api/schedules/:id       - Remove a schedule
```

## CLI Usage

The CLI provides headless access to the core library for cron jobs:

```bash
# List available shows for a station
tapedeck-cli list WMBR

# Download latest archive of a show
tapedeck-cli download WMBR backwoods --latest

# Download archive from a specific date
tapedeck-cli download WMBR backwoods --date 20260112

# Example cron entry (download latest every Monday at 6am)
0 6 * * 1 tapedeck-cli download WMBR backwoods --latest
```

## Docker Setup

### Project Structure

```
td23/
├── Dockerfile
├── docker-compose.yml
├── cmd/
│   ├── tapedeck/           # Web server + REST API
│   │   └── main.go
│   └── tapedeck-cli/       # CLI for cron/headless use
│       └── main.go
├── pkg/
│   └── tapedeck/           # Core library (public API)
│       ├── tapedeck.go     # Main entry point
│       ├── scheduler.go
│       ├── recorder.go
│       └── adapters/
│           └── adapter.go  # Adapter interface
├── internal/
│   ├── api/                # REST API handlers
│   ├── db/                 # SQLite operations
│   └── adapters/           # Station adapter implementations
├── web/
│   ├── index.html
│   ├── style.css
│   └── app.js
└── data/                   # Docker volume mount point
    ├── tapedeck.db
    └── downloads/
```

### Running

```bash
# Development
docker compose up --build

# Production
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

## Configuration

Environment variables:
- `TAPEDECK_PORT` - HTTP port (default: 8080)
- `TAPEDECK_DATA_DIR` - Data directory (default: /data)

## Deployment

The same Docker image works for both local and server deployment:
- Local: `docker compose up`
- Server: Push image to registry, pull and run with appropriate volume mounts

## Authentication

- No authentication for now
- Future: NGINX OAuth proxy in front of the application

## Supported Stations

See [STATIONS.md](STATIONS.md) for full details. Implementation priority:

| Station | Archive Format    | Index By         | Retention  |
|---------|-------------------|------------------|------------|
| WMBR    | m3u               | Show name        | 2 weeks    |
| WHRB    | m3u8 (1hr chunks) | Date/time (UTC)  | 2 weeks    |
| WUMB    | mp3 (1hr chunks)  | Show + date      | 2 weekends |
| WOMR    | aac               | Show + timestamp | 2 weeks    |
| WCUW    | Spinitron         | TBD              | 2 weeks    |
