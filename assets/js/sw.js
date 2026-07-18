/* Service Worker: static cache-first, pages network-first with offline fallback. */
const CACHE_VERSION = 'blog-v1';
const STATIC_CACHE = CACHE_VERSION + '-static';
const PAGE_CACHE = CACHE_VERSION + '-pages';

const STATIC_PREFIX = '/assets/';
const BYPASS_PREFIXES = ['/admin', '/api/'];

function shouldBypass(request) {
  if (request.method !== 'GET') {
    return true;
  }
  var path = new URL(request.url).pathname;
  for (var i = 0; i < BYPASS_PREFIXES.length; i++) {
    var prefix = BYPASS_PREFIXES[i];
    if (path === prefix || path.indexOf(prefix + '/') === 0) {
      return true;
    }
  }
  return false;
}

function isHTMLRequest(request) {
  if (request.mode === 'navigate') {
    return true;
  }
  var accept = request.headers.get('accept');
  return accept && accept.indexOf('text/html') !== -1;
}

self.addEventListener('install', function (event) {
  self.skipWaiting();
  event.waitUntil(Promise.resolve());
});

self.addEventListener('activate', function (event) {
  event.waitUntil(
    caches.keys().then(function (keys) {
      return Promise.all(
        keys
          .filter(function (key) {
            return key !== STATIC_CACHE && key !== PAGE_CACHE;
          })
          .map(function (key) {
            return caches.delete(key);
          })
      );
    }).then(function () {
      return self.clients.claim();
    })
  );
});

self.addEventListener('fetch', function (event) {
  var request = event.request;
  if (shouldBypass(request)) {
    return;
  }

  var path = new URL(request.url).pathname;
  if (path.indexOf(STATIC_PREFIX) === 0) {
    event.respondWith(cacheFirst(request, STATIC_CACHE));
    return;
  }

  if (isHTMLRequest(request)) {
    event.respondWith(networkFirst(request, PAGE_CACHE));
  }
});

function cacheFirst(request, cacheName) {
  return caches.open(cacheName).then(function (cache) {
    return cache.match(request).then(function (cached) {
      if (cached) {
        return cached;
      }
      return fetch(request).then(function (response) {
        if (response && response.ok) {
          cache.put(request, response.clone());
        }
        return response;
      });
    });
  });
}

function networkFirst(request, cacheName) {
  return caches.open(cacheName).then(function (cache) {
    return fetch(request)
      .then(function (response) {
        if (response && response.ok) {
          cache.put(request, response.clone());
        }
        return response;
      })
      .catch(function () {
        return cache.match(request).then(function (cached) {
          if (cached) {
            return cached;
          }
          throw new Error('Offline and no cached page');
        });
      });
  });
}
