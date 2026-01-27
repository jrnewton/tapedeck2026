# Plan 05: No Audio Player on Downloads Page

## Summary
The downloads page (from Plan 04) should not display the audio player footer that appears on the main page.

## Design

### Current State
The audio player is always visible at the bottom of the page:
```html
<div id="player-bar">
    <button id="prev-btn">...</button>
    <button id="play-btn">...</button>
    <button id="next-btn">...</button>
    <span id="now-playing">...</span>
    ...
</div>
```

### Implementation

**Option A: CSS Hide (Recommended)**

Add class to body when on downloads page:
```javascript
function showDownloadsPage() {
    document.body.classList.add('downloads-page');
    // ... render downloads UI
}

function showMainPage() {
    document.body.classList.remove('downloads-page');
    // ... render main UI
}
```

CSS:
```css
body.downloads-page #player-bar {
    display: none;
}
```

**Option B: Conditional Render**

Only render player bar when not on downloads page:
```javascript
function render() {
    if (currentPage === 'downloads') {
        renderDownloadsPage();
        // Don't render player
    } else {
        renderMainPage();
        renderPlayer();
    }
}
```

### Recommendation

Option A (CSS Hide) is simpler and preserves audio playback state. If user is playing audio, navigates to downloads, then returns - audio continues.

Option B would require pausing audio and managing state restoration.

## Proposed Tests

### E2E Tests

1. **TestPlayerHiddenOnDownloads** - Navigate to downloads, verify player not visible
2. **TestPlayerShownOnMain** - Navigate back to main, verify player visible
3. **TestAudioContinuesWhileOnDownloads** - Start playback, go to downloads, return, verify still playing

## Files to Modify

| File | Changes |
|------|---------|
| `cmd/tapedeck/web/app.js` | Add body class toggle on page navigation |
| `cmd/tapedeck/web/style.css` | Add `.downloads-page #player-bar { display: none; }` |
