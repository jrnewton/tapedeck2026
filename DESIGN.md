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
┌───────────────────────────────────────────────────────────────────┐
│                           Docker Host                             │
│                                                                   │
│  ┌────────────┐                                                   │
│  │ Host Cron  │───── docker exec ─────┐                           │
│  └────────────┘                       │                           │
│                                       ▼                           │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │           Docker Container (restart: unless-stopped)        │  │
│  │                                                             │  │
│  │  ┌───────────────────────────────────────────────────────┐  │  │
│  │  │              REST API (HTTP Server)                   │  │  │
│  │  │         Serves web frontend + JSON API                │  │  │
│  │  └────────────────────────┬──────────────────────────────┘  │  │
│  │                           │                                 │  │
│  │  ┌────────────────────────▼──────────────────────────────┐  │  │
│  │  │            Core Library (tapedeck pkg)                │  │  │
│  │  │           Headless, testable, CLI-compatible          │  │  │
│  │  │       ┌──────────────┐       ┌──────────────┐         │  │  │
│  │  │       │  Downloader  │       │   Adapters   │         │  │  │
│  │  │       └──────────────┘       └──────────────┘         │  │  │
│  │  │                    │                                  │  │  │
│  │  │             ┌──────▼──────┐                           │  │  │
│  │  │             │   SQLite    │                           │  │  │
│  │  │             └─────────────┘                           │  │  │
│  │  └───────────────────────────────────────────────────────┘  │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                               │                                   │
│                     ┌─────────▼─────────┐                         │
│                     │  Volume: /data    │                         │
│                     │  - tapedeck.db    │                         │
│                     │  - downloads/     │                         │
│                     └───────────────────┘                         │
└───────────────────────────────────────────────────────────────────┘
```

## Components

### Core Library (`tapedeck` package)

The core library is fully headless and testable. It can be used:
- Programmatically from Go code
- Via CLI (invoked by host cron for scheduled downloads)
- Via REST API for web frontend

1. **Downloader**
   - Downloads archive streams
   - Job queue for managing active downloads
   - Progress tracking

2. **Station Adapters**
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
- `shows` - shows per station (name, metadata)
- `downloads` - download history and status

### Frontend (HTML/CSS/Vanilla JS)

- Single page application
- Views: Stations/Shows browser, Downloads history
- No build step required

## REST API Endpoints

```
GET  /api/stations              - List configured stations
GET  /api/stations/:call/shows  - List available shows for a station

POST /api/downloads             - Queue a download
GET  /api/downloads             - List download history
GET  /api/downloads/:id         - Get download status
DELETE /api/downloads/:id       - Cancel/remove download
```

## CLI Usage

The CLI provides headless access to the core library. It runs inside the Docker container and is invoked via `docker exec`:

```bash
# List available shows for a station
docker exec tapedeck tapedeck-cli list WMBR

# Download latest archive of a show
docker exec tapedeck tapedeck-cli download WMBR backwoods --latest

# Download archive from a specific date
docker exec tapedeck tapedeck-cli download WMBR backwoods --date 20260112
```

## Scheduling with Host Cron

Scheduled downloads are managed via the host system's cron. The container runs with `restart: unless-stopped` to ensure availability.

```bash
# Example: download latest every Monday at 6am
0 6 * * 1 docker exec tapedeck tapedeck-cli download WMBR backwoods --latest

# Example: download specific show every Sunday at noon
0 12 * * 0 docker exec tapedeck tapedeck-cli download WHRB jazztime --latest
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
│       ├── downloader.go
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
