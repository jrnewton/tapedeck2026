# Plan 03: CLI list-schedules Command

## Summary
Add new CLI command `list-schedules` to display all scheduled downloads with their status, last run, and next run times.

## Design

### Usage

```
$ tapedeck-cli list-schedules

ID  Station  Show                 Schedule      Last Run             Status   Next Run
--  -------  -------------------  ------------  -------------------  -------  -------------------
1   WMBR     Lost Highway         30 4 * * 0    2026-01-26 04:30:00  success  2026-02-02 04:30:00
2   WMBR     Dinnertime Sampler   0 22 * * 2    2026-01-21 22:00:00  failed   2026-01-28 22:00:00
3   WMBR     Pipeline!            30 5 * * 6    (never)              -        2026-02-01 05:30:00
```

### Implementation

```go
func cmdListSchedules(args []string) {
    serverURL := os.Getenv("TAPEDECK_SERVER_URL")
    if serverURL == "" {
        serverURL = "http://localhost:8080"
    }

    resp, err := http.Get(serverURL + "/api/schedules")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: Cannot connect to server at %s\n", serverURL)
        os.Exit(1)
    }
    defer resp.Body.Close()

    var result struct {
        Schedules []Schedule `json:"schedules"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    if len(result.Schedules) == 0 {
        fmt.Println("No schedules configured.")
        return
    }

    // Print table header
    fmt.Printf("%-3s  %-7s  %-20s  %-12s  %-20s  %-7s  %-20s\n",
        "ID", "Station", "Show", "Schedule", "Last Run", "Status", "Next Run")
    fmt.Println(strings.Repeat("-", 95))

    for _, s := range result.Schedules {
        lastRun := "(never)"
        if s.LastRunAt != nil {
            lastRun = s.LastRunAt.Format("2006-01-02 15:04:05")
        }
        status := "-"
        if s.LastStatus != "" {
            status = s.LastStatus
        }

        fmt.Printf("%-3d  %-7s  %-20s  %-12s  %-20s  %-7s  %-20s\n",
            s.ID, s.Station, truncate(s.Show, 20), s.CronExpression,
            lastRun, status, s.NextRunAt.Format("2006-01-02 15:04:05"))
    }
}
```

### Options

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON instead of table |
| `--station CALL` | Filter by station |

### Error Messages

| Scenario | Message |
|----------|---------|
| Server unreachable | "Error: Cannot connect to server at {url}. Is the server running?" |
| No schedules | "No schedules configured." |

## Example API Call

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
      "cron_expression": "30 4 * * 0",
      "enabled": true,
      "last_run_at": "2026-01-26T04:30:00Z",
      "last_status": "success",
      "next_run_at": "2026-02-02T04:30:00Z"
    }
  ]
}
```

## Proposed Tests

### Unit Tests

1. **TestListSchedulesEmpty** - No schedules returns appropriate message
2. **TestListSchedulesFormat** - Table formatting is correct
3. **TestListSchedulesJSON** - `--json` flag outputs valid JSON
4. **TestListSchedulesFilter** - `--station` filters correctly

### Integration Tests

1. **TestListSchedulesE2E** - Create schedule, list shows it
2. **TestListSchedulesServerDown** - Server offline, appropriate error

## Files to Modify

| File | Changes |
|------|---------|
| `cmd/tapedeck-cli/main.go` | Add `cmdListSchedules()`, update help text |
