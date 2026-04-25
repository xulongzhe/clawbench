// Smart Workbench Service Worker
const CACHE_NAME = 'smartworkbench-v5';
const PRECACHE_ASSETS = [
  '/',
  '/index.html',
  '/manifest.json',
  '/assets/favicon.png',
  '/assets/logo-180.png',
  '/assets/logo-512.png',
  '/assets/logo.png'
];

// Install event - precache assets, keep old cache until new one is ready
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      console.log('[SW] Precaching assets');
      return cache.addAll(PRECACHE_ASSETS);
    })
  );
  self.skipWaiting();
});

// Activate event - clean up old caches only after new one is ready
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames
          .filter((name) => name !== CACHE_NAME)
          .map((name) => caches.delete(name))
      );
    })
  );
  self.clients.claim();
});

// Fetch event
self.addEventListener('fetch', (event) => {
  // Skip non-GET requests
  if (event.request.method !== 'GET') return;

  const url = new URL(event.request.url);

  // API requests: network-only, return 503 when offline
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(
      fetch(event.request).catch(() => {
        return new Response(JSON.stringify({ error: 'Offline' }), {
          status: 503,
          statusText: 'Service Unavailable',
          headers: { 'Content-Type': 'application/json' }
        });
      })
    );
    return;
  }

  // All other requests: network-first with cache fallback
  event.respondWith(
    fetch(event.request)
      .then((response) => {
        // Cache successful responses in background
        if (response.ok) {
          const responseClone = response.clone();
          caches.open(CACHE_NAME).then((cache) => {
            cache.put(event.request, responseClone);
          });
        }
        return response;
      })
      .catch(() => {
        // Network failed, try cache
        return caches.match(event.request).then((cachedResponse) => {
          if (cachedResponse) return cachedResponse;
          // For HTML navigation requests, fallback to cached index.html
          if (event.request.mode === 'navigate') {
            return caches.match('/index.html');
          }
          return undefined;
        });
      })
  );
});

// Handle PWA installation prompt
self.addEventListener('beforeinstallprompt', (event) => {
  event.preventDefault();
  self.deferredInstallPrompt = event;
  console.log('[SW] Install prompt ready');
});
