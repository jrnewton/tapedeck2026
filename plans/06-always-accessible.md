# Plan 06: Downloads Page Always Accessible

## Summary
The record button and downloads page should always be accessible, providing both the ability to set up new downloads and view status of current activity.

## Design

### Visibility Requirements

The record button should be visible:
1. On the main page (in player bar)
2. When no audio is loaded
3. When audio is playing
4. On any station/show selection state

### Navigation States

| From | Action | To |
|------|--------|-----|
| Main page (any state) | Click record | Downloads page |
| Downloads page | Click back | Main page (preserved state) |
| Downloads page | Browser back | Main page (preserved state) |

### URL Handling

```javascript
// URL patterns
// Main page: /?station=WMBR&show=1&play=1
// Downloads:  /?page=downloads

function handleNavigation() {
    const params = new URLSearchParams(window.location.search);

    if (params.get('page') === 'downloads') {
        showDownloadsPage();
    } else {
        showMainPage();
        // Restore station/show/play state from params
    }
}

window.addEventListener('popstate', handleNavigation);
```

### State Preservation

When navigating to downloads page, preserve main page state:
```javascript
let savedMainState = null;

function showDownloadsPage() {
    // Save current state
    savedMainState = {
        station: currentStation,
        show: currentShow,
        playing: isPlaying,
        position: audioElement.currentTime
    };

    // Update URL without losing state
    const url = new URL(window.location);
    url.searchParams.set('page', 'downloads');
    history.pushState({page: 'downloads'}, '', url);

    // Render downloads page
    renderDownloadsPage();
}

function showMainPage() {
    // Update URL
    const url = new URL(window.location);
    url.searchParams.delete('page');
    if (savedMainState) {
        url.searchParams.set('station', savedMainState.station);
        // ... restore other params
    }
    history.pushState({page: 'main'}, '', url);

    // Render main page
    renderMainPage();
}
```

### Downloads Page Features

The downloads page serves as a dashboard:

1. **Setup new downloads**
   - Ad-hoc single episode downloads
   - Recurring scheduled downloads

2. **View status**
   - Pending downloads
   - In-progress downloads (with progress if available)
   - Recent completed/failed downloads
   - Scheduled downloads with last/next run times

3. **Manage**
   - Cancel pending downloads (future enhancement)
   - Delete schedules
   - Enable/disable schedules (future enhancement)

### Auto-Refresh

Downloads page should auto-refresh status:
```javascript
let statusRefreshInterval = null;

function showDownloadsPage() {
    // ... setup page ...

    // Start polling for status updates
    statusRefreshInterval = setInterval(refreshDownloadStatus, 5000);
}

function hideDownloadsPage() {
    if (statusRefreshInterval) {
        clearInterval(statusRefreshInterval);
        statusRefreshInterval = null;
    }
}

async function refreshDownloadStatus() {
    const downloads = await fetchJSON('/api/downloads?limit=10');
    const schedules = await fetchJSON('/api/schedules');
    updateDownloadStatusUI(downloads);
    updateScheduleStatusUI(schedules);
}
```

## Proposed Tests

### E2E Tests

1. **TestRecordButtonAlwaysVisible** - Check visibility in various states
2. **TestNavigateToDownloadsPreservesState** - Go to downloads, return, state intact
3. **TestBrowserBackFromDownloads** - Browser back button works
4. **TestDirectURLToDownloads** - Load `?page=downloads` directly
5. **TestStatusAutoRefresh** - Status updates without manual refresh

## Files to Modify

| File | Changes |
|------|---------|
| `cmd/tapedeck/web/app.js` | Navigation state management, auto-refresh |
| `cmd/tapedeck/web/style.css` | Ensure record button always visible |
