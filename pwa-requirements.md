# PWA Installation Requirements for Android

A summary of the elements required to make a website installable as a Progressive Web App on Android devices.

---

## Required Elements

### 1. Web App Manifest

A JSON file linked in your HTML head:

```html
<link rel="manifest" href="/manifest.json">
```

**Minimum required fields:**

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

| Field | Requirement |
|-------|-------------|
| `name` or `short_name` | At least one is required |
| `start_url` | The URL that loads when the app launches |
| `display` | Must be `standalone`, `fullscreen`, or `minimal-ui` |
| `icons` | At least 192×192 and 512×512 PNG icons |

### 2. HTTPS

The site must be served over HTTPS. Localhost is exempt for development purposes.

### 3. Service Worker

Register a service worker with a `fetch` event handler.

**Minimal service worker (sw.js):**

```javascript
self.addEventListener('fetch', (event) => {
  event.respondWith(fetch(event.request));
});
```

**Registration (in your main JS):**

```javascript
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js');
}
```

---

## Optional but Recommended

| Element | Purpose |
|---------|---------|
| `theme_color` | Colors the browser toolbar and splash screen |
| `background_color` | Splash screen background color |
| `<meta name="theme-color">` | Browser toolbar color on supported browsers |
| Maskable icons | Adaptive icon support on Android |
| `id` field | Helps Chrome uniquely identify your app |

**Example with optional fields:**

```json
{
  "name": "My App",
  "short_name": "App",
  "start_url": "/",
  "display": "standalone",
  "theme_color": "#ffffff",
  "background_color": "#ffffff",
  "id": "/",
  "icons": [
    { "src": "/icon-192.png", "sizes": "192x192", "type": "image/png" },
    { "src": "/icon-512.png", "sizes": "512x512", "type": "image/png" },
    { "src": "/icon-maskable.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable" }
  ]
}
```

---

## Triggering the Install Prompt

Chrome shows the install prompt automatically after user engagement, or you can capture and trigger it programmatically:

```javascript
let deferredPrompt;

window.addEventListener('beforeinstallprompt', (e) => {
  e.preventDefault();
  deferredPrompt = e;
  // Show your custom install button
});

// When user clicks your install button:
deferredPrompt.prompt();
```
