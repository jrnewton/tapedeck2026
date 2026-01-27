# Plan 02: CLI schedule-download Server Integration

## Summary
Modify `tapedeck-cli schedule-download` to connect to the running server and create a schedule via API, instead of outputting a cron line.

## Current State
- `cmdScheduleDownload()` in `cmd/tapedeck-cli/main.go` (lines 452-492)
- Analyzes show broadcast history using adapter's `GetShowSchedule()`
- Outputs a cron line for manual installation
- No server communication

## Design

### New Behavior

```
$ tapedeck-cli schedule-download WMBR "Lost Highway"
Analyzing broadcast history for Lost Highway...
Detected schedule: Sundays ~02:00, archive available ~04:30
Created schedule #1: 30 4 * * 0
Next run: 2026-02-02 04:30:00
```

### Server Communication

The CLI needs the server URL. Options:
1. Environment variable: `TAPEDECK_SERVER_URL` (default: `http://localhost:8080`)
2. Command flag: `--server http://localhost:8080`

Recommended: Environment variable for consistency with `TAPEDECK_DATA_DIR`.

### Implementation

```go
func cmdScheduleDownload(args []string) {
    // ... existing validation ...

    // Get schedule recommendation from adapter (existing logic)
    schedule, err := adapter.GetShowSchedule(showName)

    // Connect to server API
    serverURL := os.Getenv("TAPEDECK_SERVER_URL")
    if serverURL == "" {
        serverURL = "http://localhost:8080"
    }

    // Create schedule via API
    payload := map[string]string{
        "station": callSign,
        "show":    showName,
        "cron":    schedule.CronLine,
    }

    resp, err := http.Post(serverURL+"/api/schedules",
        "application/json",
        jsonBody(payload))

    // Parse response, display confirmation
    var result ScheduleResponse
    json.NewDecoder(resp.Body).Decode(&result)

    fmt.Printf("Created schedule #%d: %s\n", result.ID, result.CronExpression)
    fmt.Printf("Next run: %s\n", result.NextRunAt.Format(time.RFC3339))
}
```

### Error Handling

| Scenario | Behavior |
|----------|----------|
| Server unreachable | Error: "Cannot connect to server at {url}. Is the server running?" |
| Schedule exists | Error: "Schedule already exists for {show}. Use list-schedules to view." |
| Invalid show | Error: "Show '{show}' not found for station {station}" |
| Insufficient history | Warning + proceed: "Low confidence schedule (few archives). Monitor for accuracy." |

### Backward Compatibility

Add `--cron-only` flag to preserve old behavior:
```
$ tapedeck-cli schedule-download --cron-only WMBR "Lost Highway"
30 4 * * 0 tapedeck-cli download-show WMBR "Lost Highway"
```

## Example API Calls

**CLI makes this request:**
```bash
curl -X POST http://localhost:8080/api/schedules \
  -H "Content-Type: application/json" \
  -d '{"station": "WMBR", "show": "Lost Highway", "cron": "30 4 * * 0"}'
```

**Server responds:**
```json
{
  "id": 1,
  "station": "WMBR",
  "show": "Lost Highway",
  "cron_expression": "30 4 * * 0",
  "enabled": true,
  "next_run_at": "2026-02-02T04:30:00Z"
}
```

## Proposed Tests

### Unit Tests (`cmd/tapedeck-cli/main_test.go`)

1. **TestScheduleDownloadServerURL** - Uses env var, falls back to default
2. **TestScheduleDownloadCronOnly** - `--cron-only` outputs cron line only
3. **TestScheduleDownloadParseResponse** - Parses API response correctly

### Integration Tests

1. **TestScheduleDownloadE2E** - Start server, run CLI, verify schedule created
2. **TestScheduleDownloadServerDown** - Server offline, appropriate error message
3. **TestScheduleDownloadDuplicate** - Schedule exists, appropriate error

## Files to Modify

| File | Changes |
|------|---------|
| `cmd/tapedeck-cli/main.go` | Modify `cmdScheduleDownload()`, add HTTP client, add `--cron-only` flag |
