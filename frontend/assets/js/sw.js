/* Service Worker: static cache-first, pages network-first with offline fallback. */
// __BLOG_SW_VERSION__ 由 /sw.js 路由替换为 settings site 段 sw_cache_version（缺省 blog-v3）。
const CACHE_VERSION = '__BLOG_SW_VERSION__';
const STATIC_CACHE = CACHE_VERSION + '-static';
const PAGE_CACHE = CACHE_VERSION + '-pages';

const STATIC_PREFIX = '/assets/';
const BYPASS_PREFIXES = ['/admin', '/api/'];
// P2-12：离线兜底页（install 时预缓存），离线且无缓存时返回而非白屏。
const OFFLINE_URL = '/assets/offline.html';

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
  event.waitUntil(
    caches.open(STATIC_CACHE).then(function (cache) {
      return cache.add(OFFLINE_URL);
    })
  );
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
          // P2-12：离线且无页面缓存时回退到预缓存的离线兜底页，避免白屏。
          return caches.match(OFFLINE_URL);
        });
      });
  });
}
