# Plan 07: CLI delete-schedule Command

## Summary
Add new CLI command `delete-schedule` to remove a scheduled download by ID.

## Design

### Usage

```
$ tapedeck-cli delete-schedule 1
Deleted schedule #1 (WMBR - Backwoods)
```

### Implementation

```go
func cmdDeleteSchedule(args []string) error {
    if len(args) < 1 {
        return fmt.Errorf("usage: delete-schedule <ID>")
    }

    id, err := strconv.ParseInt(args[0], 10, 64)
    if err != nil {
        return fmt.Errorf("invalid schedule ID: %s", args[0])
    }

    serverURL := os.Getenv("TAPEDECK_SERVER_URL")
    if serverURL == "" {
        serverURL = "http://localhost:8080"
    }

    // First get schedule details for confirmation message
    resp, err := http.Get(fmt.Sprintf("%s/api/schedules/%d", serverURL, id))
    // ... parse response to get station/show name

    // Delete via API
    req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/schedules/%d", serverURL, id), nil)
    resp, err = http.DefaultClient.Do(req)
    // ... handle response

    fmt.Printf("Deleted schedule #%d (%s - %s)\n", id, station, show)
    return nil
}
```

### Error Handling

| Scenario            | Message                                                            |
|---------------------|--------------------------------------------------------------------|
| Missing ID          | "usage: delete-schedule <ID>"                                      |
| Invalid ID          | "invalid schedule ID: {input}"                                     |
| Server unreachable  | "cannot connect to server at {url}. Is the server running?"        |
| Schedule not found  | "schedule not found: {id}"                                         |

## Example API Calls

```bash
# Get schedule details first
curl http://localhost:8080/api/schedules/1

# Delete schedule
curl -X DELETE http://localhost:8080/api/schedules/1
```

## Proposed Tests

### Unit Tests

1. **TestDeleteSchedule_MissingID** - No ID provided returns error
2. **TestDeleteSchedule_InvalidID** - Non-numeric ID returns error

### Integration Tests

1. **TestDeleteScheduleE2E** - Create schedule, delete it, verify gone
2. **TestDeleteSchedule_NotFound** - Delete non-existent ID returns appropriate error

## Files to Modify

| File                        | Changes                                      |
|-----------------------------|----------------------------------------------|
| `cmd/tapedeck-cli/main.go`  | Add `cmdDeleteSchedule()`, update help text  |
