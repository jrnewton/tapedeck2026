# Plan 01: Server Native Scheduler

## Summary
Replace reliance on host cron with a native Go scheduler in the server. The server will manage scheduled download jobs internally, executing them at configured times.

## Current State
- `tapedeck-cli schedule-download` analyzes show history and outputs a cron line
- Downloads are synchronous (`runDownload()` blocks until complete)
- No persistent schedule storage in database

## Design

### Database Schema Changes

Add new `schedules` table:

```sql
CREATE TABLE schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    station_id INTEGER NOT NULL REFERENCES stations(id),
    show_id INTEGER NOT NULL REFERENCES shows(id),
    cron_expression TEXT NOT NULL,  -- "MM HH * * D" format
    enabled INTEGER NOT NULL DEFAULT 1,
    last_run_at DATETIME,
    last_status TEXT,  -- "success", "failed", "skipped"
    last_error TEXT,
    next_run_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(station_id, show_id)
);
```

### Scheduler Component

New package: `internal/scheduler/scheduler.go`

```go
type Scheduler struct {
    db          *db.DB
    adapters    map[string]tapedeck.Adapter
    downloadsDir string
    ticker      *time.Ticker
    done        chan bool
}

func New(db *db.DB, adapters map[string]tapedeck.Adapter, dir string) *Scheduler
func (s *Scheduler) Start()
func (s *Scheduler) Stop()
func (s *Scheduler) AddSchedule(stationID, showID int64, cronExpr string) (int64, error)
func (s *Scheduler) RemoveSchedule(id int64) error
func (s *Scheduler) ListSchedules() ([]Schedule, error)
func (s *Scheduler) runDue() // checks and executes due jobs
```

### Cron Expression Handling

Use simple cron format: `MM HH * * D` (minute, hour, *, *, day-of-week)
- Parse and calculate next run time
- Store `next_run_at` in database for efficient querying
- Option: Use stdlib only, or ask about `robfig/cron` library

### Download Execution

When a schedule is due:
1. Get latest archive for show via adapter
2. Check if already downloaded (via `FindDownload`)
3. If not, create download record and execute
4. Update schedule's `last_run_at`, `last_status`, `next_run_at`

### Server Integration

In `cmd/tapedeck/main.go`:
```go
scheduler := scheduler.New(database, adapters, downloadsDir)
scheduler.Start()
defer scheduler.Stop()
```

## API Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| `POST` | `/api/schedules` | Create new schedule |
| `GET` | `/api/schedules` | List all schedules |
| `GET` | `/api/schedules/{id}` | Get schedule details |
| `DELETE` | `/api/schedules/{id}` | Remove schedule |
| `PATCH` | `/api/schedules/{id}` | Update (enable/disable) |

### Example API Calls

**Create schedule:**
```bash
curl -X POST http://localhost:8080/api/schedules \
  -H "Content-Type: application/json" \
  -d '{"station": "WMBR", "show": "Lost Highway", "cron": "30 4 * * 1"}'
```

Response:
```json
{
  "id": 1,
  "station": "WMBR",
  "show": "Lost Highway",
  "cron_expression": "30 4 * * 1",
  "enabled": true,
  "next_run_at": "2026-02-02T04:30:00Z",
  "created_at": "2026-01-27T12:00:00Z"
}
```

**List schedules:**
```bash
curl http://localhost:8080/api/schedules
```

Response:
```json
{
  "schedules": [
    {
      "id": 1,
      "station": "WMBR",
      "show": "Lost Highway",
      "cron_expression": "30 4 * * 1",
      "enabled": true,
      "last_run_at": "2026-01-20T04:30:00Z",
      "last_status": "success",
      "next_run_at": "2026-02-02T04:30:00Z"
    }
  ]
}
```

## Proposed Tests

### Unit Tests (`internal/scheduler/scheduler_test.go`)

1. **TestParseCronExpression** - Parse valid/invalid cron expressions
2. **TestCalculateNextRun** - Given current time and cron, calculate next run
3. **TestScheduleDue** - Schedule with past next_run_at is identified as due
4. **TestSkipDuplicate** - Download already exists, schedule marks "skipped"
5. **TestRunDownload** - Successfully queues and executes download

### Integration Tests (`internal/scheduler/integration_test.go`)

1. **TestSchedulerStartStop** - Scheduler starts, ticks, stops cleanly
2. **TestAddRemoveSchedule** - CRUD operations on schedules
3. **TestScheduleExecution** - End-to-end: create schedule, advance time, verify download

### Database Tests (`internal/db/db_test.go`)

1. **TestInsertSchedule** - Insert new schedule
2. **TestListSchedules** - List with/without filters
3. **TestUpdateScheduleStatus** - Update last_run fields
4. **TestScheduleUniqueConstraint** - Duplicate station+show rejected

## Open Questions

1. **Third-party cron library**: Use `robfig/cron` for robust parsing, or implement simple parser in stdlib only?
2. **Tick interval**: Check schedules every minute, or more/less frequently?
3. **Concurrent downloads**: Allow multiple simultaneous downloads, or serialize?
4. **Retry logic**: Retry failed downloads? How many times?

## Files to Create/Modify

| File | Action |
|------|--------|
| `internal/scheduler/scheduler.go` | Create |
| `internal/scheduler/scheduler_test.go` | Create |
| `internal/scheduler/cron.go` | Create (cron parsing) |
| `internal/scheduler/cron_test.go` | Create |
| `internal/db/db.go` | Add schedules table + methods |
| `internal/db/db_test.go` | Add schedule tests |
| `internal/api/api.go` | Add schedule endpoints |
| `cmd/tapedeck/main.go` | Initialize scheduler |
