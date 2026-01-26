// Tapedeck Service Worker
// Provides offline app shell caching for PWA experience

const CACHE_VERSION = 'v1';
const CACHE_NAME = `tapedeck-${CACHE_VERSION}`;

// App shell files to cache for offline use
const APP_SHELL = [
    '/',
    '/index.html',
    '/app.js',
    '/style.css',
    '/offline.js'
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

    // API calls: network-first (don't cache)
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
        caches.match(event.request)
            .then((cachedResponse) => {
                if (cachedResponse) {
                    return cachedResponse;
                }
                return fetch(event.request).then((response) => {
                    // Don't cache non-GET requests or failed responses
                    if (event.request.method !== 'GET' || !response.ok) {
                        return response;
                    }
                    // Cache successful responses for app shell files
                    const responseToCache = response.clone();
                    caches.open(CACHE_NAME).then((cache) => {
                        cache.put(event.request, responseToCache);
                    });
                    return response;
                });
            })
    );
});
