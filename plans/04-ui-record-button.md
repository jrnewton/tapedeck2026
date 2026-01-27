# Plan 04: UI Record Button and Download Page

## Summary
Add a "record" button to the audio player that navigates to a new download management page. This page has two sections: ad-hoc downloads (4a) and scheduled downloads (4b).

## Mockup

### Main Page - Record Button Location

```
+------------------------------------------+
|  TAPEDECK                          [DBG] |
+------------------------------------------+
|                                          |
|  [ Station Tapes ]                       |
|                                          |
|  +------------------------------------+  |
|  | WMBR  Lost Highway    Jan 26       |  |
|  +------------------------------------+  |
|  | WMBR  Pipeline!       Jan 25       |  |
|  +------------------------------------+  |
|                                          |
+------------------------------------------+
|  [<<] [|>]  Lost Highway     [REC] [O]  |
|  =====[====]========================     |
+------------------------------------------+

[REC] = Record button (red circle icon)
[O]   = Existing offline save button
```

### Download Page Layout

```
+------------------------------------------+
|  [<] DOWNLOADS                     [DBG] |
+------------------------------------------+
|                                          |
|  == DOWNLOAD EPISODE ==                  |
|                                          |
|  Station: [WMBR v]                       |
|  Show:    [Select show... v]             |
|  Date:    ( ) Latest  (o) Pick date      |
|           [2026-01-27]                   |
|                                          |
|  [DOWNLOAD]                              |
|                                          |
|  -- Recent Downloads --                  |
|  Lost Highway (Jan 26)     [completed]   |
|  Pipeline! (Jan 25)        [downloading] |
|  Dinnertime (Jan 21)       [failed]      |
|                                          |
+------------------------------------------+
|                                          |
|  == SCHEDULED DOWNLOADS ==               |
|                                          |
|  Station: [WMBR v]                       |
|  Show:    [Select show... v]             |
|                                          |
|  [SCHEDULE]                              |
|                                          |
|  -- Active Schedules --                  |
|  +------------------------------------+  |
|  | Lost Highway                       |  |
|  | Every Sunday ~04:30                |  |
|  | Last: Jan 26 (success)             |  |
|  | Next: Feb 2                  [DEL] |  |
|  +------------------------------------+  |
|  | Pipeline!                          |  |
|  | Every Saturday ~05:30              |  |
|  | Last: never                        |  |
|  | Next: Feb 1                  [DEL] |  |
|  +------------------------------------+  |
|                                          |
+------------------------------------------+
```

## Design

### Navigation

- Record button click: `window.location.hash = '#downloads'` or `?page=downloads`
- Back button on downloads page returns to main view
- URL state: `?page=downloads` (no station/show params needed)

### Section 4a: Ad-hoc Downloads

**Show List**: Must include ALL shows, not just those with existing downloads.

New API endpoint needed:
```
GET /api/stations/{call}/allshows
```

This differs from existing `/api/stations/{call}/shows` which only returns shows with downloads.

**Form Fields:**
- Station dropdown (populated from `/api/stations`)
- Show dropdown (populated from `/api/stations/{call}/allshows`)
- Date selection: radio buttons for "Latest" or "Pick date" + date input

**Download Action:**
```javascript
async function startDownload(station, show, date) {
    const resp = await fetch('/api/downloads', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({station, show, date: date || 'latest'})
    });
    // Refresh download status list
}
```

**Status Display:**
- Poll `/api/downloads?status=pending,downloading` every 5 seconds
- Show recent downloads (last 10) with status badges

### Section 4b: Scheduled Downloads

**Show List**: Same as 4a - ALL shows via `/api/stations/{call}/allshows`

**Schedule Action:**
```javascript
async function createSchedule(station, show) {
    const resp = await fetch('/api/schedules', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({station, show})  // Server determines cron
    });
    // Refresh schedule list
}
```

**Schedule Display:**
- Fetch from `/api/schedules`
- Show cron in human-readable format: "Every Sunday ~04:30"
- Last run with status
- Next run time
- Delete button per schedule

### Cron Human-Readable Format

```javascript
function formatCron(cronExpr) {
    // "30 4 * * 0" -> "Every Sunday ~04:30"
    const [min, hour, , , dow] = cronExpr.split(' ');
    const days = ['Sunday', 'Monday', 'Tuesday', 'Wednesday',
                  'Thursday', 'Friday', 'Saturday'];
    return `Every ${days[dow]} ~${hour.padStart(2,'0')}:${min.padStart(2,'0')}`;
}
```

## API Endpoints Needed

| Method | Endpoint | Purpose |
|--------|----------|---------|
| `GET` | `/api/stations/{call}/allshows` | List ALL shows (not just with downloads) |
| `POST` | `/api/downloads` | Queue ad-hoc download |
| `GET` | `/api/schedules` | List all schedules |
| `POST` | `/api/schedules` | Create new schedule |
| `DELETE` | `/api/schedules/{id}` | Remove schedule |

### Example API Calls

**Get all shows:**
```bash
curl http://localhost:8080/api/stations/WMBR/allshows
```
```json
{
  "shows": [
    {"name": "Lost Highway"},
    {"name": "Pipeline!"},
    {"name": "Dinnertime Sampler"},
    {"name": "Late Risers Club"},
    ...
  ]
}
```

**Queue download:**
```bash
curl -X POST http://localhost:8080/api/downloads \
  -H "Content-Type: application/json" \
  -d '{"station": "WMBR", "show": "Lost Highway", "date": "latest"}'
```
```json
{
  "id": 42,
  "station": "WMBR",
  "show": "Lost Highway",
  "status": "pending",
  "created_at": "2026-01-27T12:00:00Z"
}
```

**Create schedule:**
```bash
curl -X POST http://localhost:8080/api/schedules \
  -H "Content-Type: application/json" \
  -d '{"station": "WMBR", "show": "Lost Highway"}'
```
```json
{
  "id": 1,
  "cron_expression": "30 4 * * 0",
  "next_run_at": "2026-02-02T04:30:00Z"
}
```

## Proposed Tests

### Frontend Tests (E2E)

1. **TestRecordButtonNavigates** - Click record button, verify downloads page shown
2. **TestBackButtonReturns** - Click back, verify main page shown
3. **TestShowDropdownPopulated** - All shows appear (not just downloaded)
4. **TestDownloadLatest** - Select show, click download, verify pending status
5. **TestDownloadByDate** - Pick date, click download, verify correct date used
6. **TestCreateSchedule** - Select show, click schedule, verify appears in list
7. **TestDeleteSchedule** - Click delete, verify removed from list
8. **TestScheduleStatusRefresh** - Status updates after schedule runs

### API Tests

1. **TestAllShowsEndpoint** - Returns all shows, not filtered
2. **TestQueueDownload** - POST creates pending download
3. **TestQueueDownloadLatest** - "latest" resolves to most recent archive
4. **TestQueueDownloadByDate** - Specific date queued correctly

## Files to Create/Modify

| File | Changes |
|------|---------|
| `cmd/tapedeck/web/app.js` | Add record button, downloads page, all new UI |
| `cmd/tapedeck/web/style.css` | Styles for downloads page, record button |
| `cmd/tapedeck/web/index.html` | May need additional containers |
| `internal/api/api.go` | Add `/allshows`, `POST /downloads` endpoints |
