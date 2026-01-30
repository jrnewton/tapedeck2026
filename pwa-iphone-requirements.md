# iOS/iPhone PWA Requirements

A comprehensive guide to making a website installable as a Progressive Web App on iOS devices, including icon customization and audio playback with lock screen/CarPlay support.

---

## Core Requirements

### 1. Web App Manifest

Link a JSON manifest file in your HTML head:

```html
<link rel="manifest" href="/manifest.json">
```

**Minimum manifest fields:**

```json
{
  "name": "My App",
  "short_name": "App",
  "start_url": "/",
  "display": "standalone",
  "icons": [
    { "src": "/icon-192.png", "sizes": "192x192", "type": "image/png" },
    { "src": "/icon-512.png", "sizes": "512x512", "type": "image/png" }
  ]
}
```

> **Note:** iOS only supports `standalone` display mode. `fullscreen` falls back to `standalone`, and `minimal-ui` falls back to a browser shortcut.

### 2. HTTPS

The site must be served over HTTPS (localhost is exempt for development).

### 3. Service Worker

A registered service worker with a `fetch` event handler:

```javascript
// sw.js
self.addEventListener('fetch', (event) => {
  event.respondWith(fetch(event.request));
});
```

Register it:

```javascript
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js');
}
```

### 4. Apple-Specific Meta Tag

This is **critical** for iOS to recognize the app as installable:

```html
<meta name="apple-mobile-web-app-capable" content="yes">
```

---

## Customizing the Home Screen Icon (iOS)

**Important:** Safari on iOS ignores manifest icons and uses `apple-touch-icon` instead. If both are present, `apple-touch-icon` takes precedence.

### Required HTML Tags

```html
<!-- Primary icon (180x180 is recommended for modern devices) -->
<link rel="apple-touch-icon" href="/apple-touch-icon.png">

<!-- Multiple sizes for different devices -->
<link rel="apple-touch-icon" sizes="120x120" href="/apple-touch-icon-120x120.png">
<link rel="apple-touch-icon" sizes="152x152" href="/apple-touch-icon-152x152.png">
<link rel="apple-touch-icon" sizes="167x167" href="/apple-touch-icon-167x167.png">
<link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon-180x180.png">
```

### Icon Size Guidelines

| Size    | Device                                        |
|---------|-----------------------------------------------|
| 120×120 | iPhone (older)                                |
| 152×152 | iPad                                          |
| 167×167 | iPad Pro                                      |
| 180×180 | iPhone (Retina) - **recommended default**     |

### Icon Design Tips

- Icons should be **square with no transparency** (iOS adds the rounded corners automatically)
- Add ~20px padding around your logo with a solid background color
- Do not include the rounded corner mask—iOS applies it
- Use PNG format

### App Title

```html
<meta name="apple-mobile-web-app-title" content="My App">
```

### Status Bar Style

```html
<meta name="apple-mobile-web-app-status-bar-style" content="default">
<!-- Options: default, black, black-translucent -->
```

---

## Complete iOS PWA HTML Head Example

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>My App</title>
  
  <!-- PWA Manifest -->
  <link rel="manifest" href="/manifest.json">
  
  <!-- iOS PWA Support -->
  <meta name="apple-mobile-web-app-capable" content="yes">
  <meta name="apple-mobile-web-app-title" content="My App">
  <meta name="apple-mobile-web-app-status-bar-style" content="default">
  
  <!-- iOS Icons (these override manifest icons on Safari) -->
  <link rel="apple-touch-icon" href="/apple-touch-icon.png">
  <link rel="apple-touch-icon" sizes="152x152" href="/apple-touch-icon-152x152.png">
  <link rel="apple-touch-icon" sizes="167x167" href="/apple-touch-icon-167x167.png">
  <link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon-180x180.png">
  
  <!-- Theme Color -->
  <meta name="theme-color" content="#ffffff">
</head>
<body>
  <!-- App content -->
</body>
</html>
```

---

## Audio Playback: Lock Screen & Control Center

iOS supports the **Media Session API** for displaying audio metadata on the lock screen and Control Center. This is also what populates the "Now Playing" information that CarPlay can access.

### Basic Media Session Implementation

```javascript
if ('mediaSession' in navigator) {
  navigator.mediaSession.metadata = new MediaMetadata({
    title: 'Episode Title',
    artist: 'Artist or Podcast Name',
    album: 'Album or Show Name',
    artwork: [
      { src: '/artwork-96.png', sizes: '96x96', type: 'image/png' },
      { src: '/artwork-128.png', sizes: '128x128', type: 'image/png' },
      { src: '/artwork-256.png', sizes: '256x256', type: 'image/png' },
      { src: '/artwork-512.png', sizes: '512x512', type: 'image/png' }
    ]
  });

  // Action handlers
  navigator.mediaSession.setActionHandler('play', () => {
    audio.play();
    navigator.mediaSession.playbackState = 'playing';
  });

  navigator.mediaSession.setActionHandler('pause', () => {
    audio.pause();
    navigator.mediaSession.playbackState = 'paused';
  });

  navigator.mediaSession.setActionHandler('seekbackward', (details) => {
    audio.currentTime = Math.max(audio.currentTime - (details.seekOffset || 10), 0);
  });

  navigator.mediaSession.setActionHandler('seekforward', (details) => {
    audio.currentTime = Math.min(audio.currentTime + (details.seekOffset || 10), audio.duration);
  });
}
```

### iOS-Specific Media Session Quirks

⚠️ **Critical limitations on iOS Safari:**

| Issue                                         | Workaround                                                                                                       |
|-----------------------------------------------|------------------------------------------------------------------------------------------------------------------|
| **Artwork max size ~128×128**                 | Images larger than 128×128 may show as a grey box. Resize images dynamically (see below).                       |
| **Only first artwork used**                   | iOS picks one size (usually the first) and uses it everywhere, including blurred in Control Center.              |
| **Album hidden if artist set**                | If you provide `artist`, the `album` field won't display. Choose one or the other.                              |
| **seekbackward/seekforward hides prev/next**  | If you set handlers for seek actions, the Previous/Next track buttons disappear.                                |

### Workaround: Resize Large Artwork for iOS

```javascript
function setMediaSessionWithResizedArtwork(originalSrc, title, artist) {
  const image = new Image();
  image.crossOrigin = 'anonymous';
  image.src = originalSrc;
  
  image.addEventListener('load', () => {
    const canvas = document.createElement('canvas');
    canvas.width = 128;
    canvas.height = 128;
    const ctx = canvas.getContext('2d');
    ctx.drawImage(image, 0, 0, 128, 128);
    
    canvas.toBlob((blob) => {
      if (!blob) return;
      const blobURL = URL.createObjectURL(blob);
      
      navigator.mediaSession.metadata = new MediaMetadata({
        title: title,
        artist: artist,
        artwork: [{ src: blobURL, sizes: '128x128', type: blob.type }]
      });
    });
  });
}
```

### Position State (Progress Bar)

```javascript
function updatePositionState() {
  if ('setPositionState' in navigator.mediaSession) {
    navigator.mediaSession.setPositionState({
      duration: audio.duration,
      playbackRate: audio.playbackRate,
      position: audio.currentTime
    });
  }
}

// Call when playback starts and on seek
audio.addEventListener('playing', updatePositionState);
audio.addEventListener('seeked', updatePositionState);
audio.addEventListener('ratechange', updatePositionState);
```

---

## CarPlay Support

### Can PWAs Directly Support CarPlay?

**No.** CarPlay requires native iOS app development with specific Apple frameworks:

- `CarPlay Framework` (CPTemplate APIs)
- `MPNowPlayingInfoCenter`
- `MPRemoteCommandCenter`

PWAs cannot directly integrate with CarPlay's interface or display custom UI in the car.

### What PWAs *Can* Do

PWAs can populate the **system-wide "Now Playing" information** via the Media Session API. This metadata (title, artist, artwork) will appear in:

- iOS Lock Screen
- Control Center
- **CarPlay's Now Playing screen** (when your PWA is the active audio source)

When a user plays audio from your PWA and connects to CarPlay, the Now Playing screen will display your Media Session metadata, including artwork.

### How It Works

```
PWA Audio Playback
       ↓
Media Session API (title, artist, artwork)
       ↓
iOS System "Now Playing" Info
       ↓
CarPlay Now Playing Screen (reads system info)
```

### Limitations

| Feature                            | PWA Support                        |
|------------------------------------|------------------------------------|
| Now Playing artwork in CarPlay     | ✅ Yes (via Media Session API)     |
| Now Playing title/artist           | ✅ Yes                             |
| Play/Pause controls                | ✅ Yes                             |
| Custom CarPlay UI/templates        | ❌ No (requires native app)        |
| CarPlay app icon on dashboard      | ❌ No (requires native app)        |
| Browse content in CarPlay          | ❌ No (requires native app)        |

### For Full CarPlay Integration

If you need a dedicated CarPlay app with browsable content, playlists, or custom UI, you must build a **native iOS app** with:

1. CarPlay entitlement from Apple
2. CarPlay Framework implementation
3. App Store distribution

Consider using a PWA-to-native wrapper (like Capacitor or PWABuilder) as a starting point, then add native CarPlay code.

---

## iOS PWA Limitations Summary

| Feature                  | iOS Support                                   |
|--------------------------|-----------------------------------------------|
| Add to Home Screen       | ✅ Manual only (no auto-prompt)               |
| Push Notifications       | ✅ iOS 16.4+ (must be installed first)        |
| Offline Support          | ✅ Via Service Workers                        |
| Background Audio         | ⚠️ Limited                                    |
| Storage Persistence      | ⚠️ May be cleared after 7 days of inactivity  |
| Badging                  | ❌ Not supported                              |
| App Shortcuts            | ❌ Not supported                              |
| Full CarPlay Integration | ❌ Requires native app                        |

---

## Installation Instructions for Users

Since iOS doesn't show automatic install prompts, you may want to guide users:

1. Open your site in **Safari** (required—other browsers won't work)
2. Tap the **Share** button (square with arrow)
3. Scroll down and tap **"Add to Home Screen"**
4. Customize the name if desired
5. Tap **"Add"**

Consider showing an in-app banner with these instructions for iOS Safari users.

---

## Resources

- [MDN: Making PWAs Installable](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Making_PWAs_installable)
- [MDN: Media Session API](https://developer.mozilla.org/en-US/docs/Web/API/Media_Session_API)
- [firt.dev: iOS PWA Compatibility](https://firt.dev/notes/pwa-ios/)
- [Apple: CarPlay Developer Guide](https://developer.apple.com/carplay/)
- [web.dev: Media Session](https://web.dev/articles/media-session)
