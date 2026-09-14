const CACHE_NAME = 'gardarr-v4';
const STATIC_ASSETS = [
  '/logo.ico',
  '/logo-192.png',
  '/logo-512.png',
  '/logo-512-maskable.png',
  '/apple-touch-icon.png',
  '/shortcut-96.png',
  '/manifest.json'
];

// Install event - cache static assets. Do NOT skipWaiting here: the new worker
// stays in "waiting" so the app can surface an update prompt and activate it on
// demand via the SKIP_WAITING message below.
globalThis.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then((cache) => {
        return cache.addAll(STATIC_ASSETS);
      })
  );
});

// Allow the page to trigger activation of the waiting worker.
// Same-origin check: a service worker can receive messages from any client
// its scope covers, so verify the sender before acting on it.
globalThis.addEventListener('message', (event) => {
  if (event.origin !== globalThis.location.origin) return;
  if (event.data && event.data.type === 'SKIP_WAITING') {
    globalThis.skipWaiting();
  }
});

// Activate event - cleanup old caches
globalThis.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames
          .filter((name) => name !== CACHE_NAME)
          .map((name) => caches.delete(name))
      );
    })
  );
  globalThis.clients.claim();
});

// Fetch event - network first for API, cache first for assets
globalThis.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Skip non-GET requests
  if (request.method !== 'GET') return;

  // Skip chrome-extension and other non-http(s) requests
  if (!url.protocol.startsWith('http')) return;

  // The HTML shell must always come from the network so a deployed page is
  // never held by the service worker. Keep caching versioned assets below.
  if (request.mode === 'navigate' || url.pathname === '/' || url.pathname === '/index.html') {
    event.respondWith(fetch(request));
    return;
  }

  // API requests - network first
  if (url.pathname.startsWith('/v1/')) {
    event.respondWith(
      fetch(request)
        .then((response) => {
          // Clone and cache successful responses
          if (response.ok) {
            const responseClone = response.clone();
            caches.open(CACHE_NAME).then((cache) => {
              cache.put(request, responseClone);
            });
          }
          return response;
        })
        .catch(() => {
          // Fallback to cache if network fails
          return caches.match(request).then((cachedResponse) => {
            if (cachedResponse) {
              return cachedResponse;
            }
            // No cache available - return 503 Service Unavailable
            return new Response(
              JSON.stringify({ error: 'Service unavailable - offline and no cached data' }),
              {
                status: 503,
                statusText: 'Service Unavailable',
                headers: { 'Content-Type': 'application/json' }
              }
            );
          });
        })
    );
    return;
  }

  // Static assets - cache first, then network
  event.respondWith(
    caches.match(request).then((cachedResponse) => {
      if (cachedResponse) {
        // Return cached version and update cache in background
        fetch(request).then((response) => {
          if (response.ok) {
            caches.open(CACHE_NAME).then((cache) => {
              cache.put(request, response);
            });
          }
        }).catch(() => {});
        return cachedResponse;
      }

      // Not in cache - fetch from network
      return fetch(request).then((response) => {
        if (response.ok) {
          const responseClone = response.clone();
          caches.open(CACHE_NAME).then((cache) => {
            cache.put(request, responseClone);
          });
        }
        return response;
      });
    })
  );
});
