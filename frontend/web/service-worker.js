const CACHE_VERSION = 'himate-public-v1';
const PUBLIC_CACHE = CACHE_VERSION + '-assets';

const PUBLIC_ASSETS = [
  '/landing.html',
  '/manifest.json',
  '/himate-brand-r4.css',
  '/brand.css',
  '/site.js',
  '/brand/himate_identity_favicon_32.png',
  '/brand/himate_identity_icon_192.webp',
  '/brand/himate_identity_wordmark_2026.webp'
];

const PROTECTED_PREFIXES = [
  '/api/',
  '/partner/',
  '/app',
  '/login',
  '/logout'
];

function isProtected(pathname) {
  return PROTECTED_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(prefix));
}

function responseMayBeCached(response) {
  if (!response || !response.ok) return false;
  const policy = (response.headers.get('Cache-Control') || '').toLowerCase();
  return !policy.includes('no-store') && !policy.includes('private');
}

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(PUBLIC_CACHE)
      .then((cache) => Promise.all(PUBLIC_ASSETS.map(async (asset) => {
        try {
          const response = await fetch(asset, {cache: 'reload'});
          if (responseMayBeCached(response)) {
            await cache.put(asset, response.clone());
          }
        } catch (_) {
          // An individual public asset must not prevent worker installation.
        }
      })))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((key) => key !== PUBLIC_CACHE).map((key) => caches.delete(key))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', (event) => {
  const request = event.request;
  if (request.method !== 'GET') return;

  const url = new URL(request.url);
  if (url.origin !== self.location.origin || isProtected(url.pathname)) return;

  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request).catch(async () => {
        if (url.pathname === '/') {
          return new Response(
            '<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>HIMATE Offline</title></head><body><main><h1>HIMATE</h1><p>The public site is temporarily offline. Reconnect to continue.</p></main></body></html>',
            {status: 200, headers: {'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store'}}
          );
        }
        return Response.error();
      })
    );
    return;
  }

  const cacheableAsset =
    url.pathname.startsWith('/brand/') ||
    url.pathname.startsWith('/art/') ||
    url.pathname.endsWith('.css') ||
    url.pathname.endsWith('.js') ||
    url.pathname === '/manifest.json';

  if (!cacheableAsset) return;

  event.respondWith(
    caches.match(request).then(async (cached) => {
      if (cached) return cached;
      const response = await fetch(request);
      if (responseMayBeCached(response)) {
        const cache = await caches.open(PUBLIC_CACHE);
        await cache.put(request, response.clone());
      }
      return response;
    })
  );
});
