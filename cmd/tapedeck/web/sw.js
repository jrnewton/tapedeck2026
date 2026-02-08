// Tapedeck Service Worker
// Provides offline app shell caching for PWA experience

const CACHE_VERSION = 'v57';
const CACHE_NAME = `tapedeck-${CACHE_VERSION}`;

// App shell files to cache for offline use
const APP_SHELL = [
    '/',
    '/index.html',
    '/app.js',
    '/style.css',
    '/offline.js',
    '/favicon.png',
    '/apple-touch-icon.png'
];

// Install event - cache app shell
self.addEventListener('install', (event) => {
    event.waitUntil(
        caches.open(CACHE_NAME)
            .then((cache) => cache.addAll(APP_SHELL))
            .then(() => self.skipWaiting())
    );
});

// Activate event - clean up old caches
self.addEventListener('activate', (event) => {
    event.waitUntil(
        caches.keys()
            .then((cacheNames) => {
                return Promise.all(
                    cacheNames
                        .filter((name) => name.startsWith('tapedeck-') && name !== CACHE_NAME)
                        .map((name) => caches.delete(name))
                );
            })
            .then(() => self.clients.claim())
    );
});

// Fetch event - network-first for API, cache-first for app shell
self.addEventListener('fetch', (event) => {
    const url = new URL(event.request.url);

    // Only handle same-origin requests
    if (url.origin !== location.origin) {
        return;
    }

    // Audio API: don't intercept at all (iOS Safari has issues with SW-served audio)
    if (url.pathname.startsWith('/api/audio/')) {
        return;
    }

    // Other API calls: network-first (don't cache)
    if (url.pathname.startsWith('/api/')) {
        event.respondWith(
            fetch(event.request).catch(() => {
                return new Response(JSON.stringify({ error: 'Offline' }), {
                    status: 503,
                    headers: { 'Content-Type': 'application/json' }
                });
            })
        );
        return;
    }

    // App shell: cache-first, then network
    event.respondWith(
        caches.match(event.request, { ignoreSearch: true })
            .then((cachedResponse) => {
                // Don't serve redirected responses (causes Safari "has redirections" error)
                if (cachedResponse && !cachedResponse.redirected && cachedResponse.type !== 'opaqueredirect') {
                    return cachedResponse;
                }

                // For navigation requests, serve index.html for SPA routing
                if (event.request.mode === 'navigate') {
                    return caches.match('/index.html').then((indexResponse) => {
                        if (indexResponse && !indexResponse.redirected) {
                            return indexResponse;
                        }
                        return fetchAndCache(event.request);
                    });
                }

                return fetchAndCache(event.request);
            })
            .catch(() => {
                // Last resort: try to serve index.html for navigation
                if (event.request.mode === 'navigate') {
                    return caches.match('/index.html');
                }
                return new Response('Offline', { status: 503 });
            })
    );
});

// Helper to fetch and cache response
function fetchAndCache(request) {
    return fetch(request)
        .then((response) => {
            // Don't cache non-GET, failed, or redirected responses
            // (redirected responses cause Safari "has redirections" error)
            if (request.method !== 'GET' || !response.ok || response.redirected) {
                return response;
            }
            const responseToCache = response.clone();
            caches.open(CACHE_NAME).then((cache) => {
                cache.put(request, responseToCache);
            });
            return response;
        })
        .catch(() => {
            return new Response('Offline', { status: 503 });
        });
}
