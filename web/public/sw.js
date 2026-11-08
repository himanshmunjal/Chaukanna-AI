// Chaukanna's service worker. It makes the installed app open quickly and shows useful
// advice with no internet. It never touches /api, /ws or /audio: live call data and
// private details must always come from the server, and are never stored here.

const VERSION = 'v1'
const SHELL = `chaukanna-shell-${VERSION}`
const ASSETS = `chaukanna-assets-${VERSION}`
const OFFLINE_PAGE = '/offline.html'
const PRECACHE = [OFFLINE_PAGE, '/icon.svg', '/icon-192.png', '/icon-512.png']
const MAX_ASSETS = 40

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(SHELL).then((cache) => cache.addAll(PRECACHE)).then(() => self.skipWaiting()))
})

self.addEventListener('activate', (event) => {
  event.waitUntil((async () => {
    const names = await caches.keys()
    await Promise.all(names.filter((name) => name !== SHELL && name !== ASSETS).map((name) => caches.delete(name)))
    await self.clients.claim()
  })())
})

async function keepAsset(request, response) {
  const cache = await caches.open(ASSETS)
  await cache.put(request, response)
  const keys = await cache.keys()
  await Promise.all(keys.slice(0, Math.max(0, keys.length - MAX_ASSETS)).map((key) => cache.delete(key)))
}

self.addEventListener('fetch', (event) => {
  const { request } = event
  if (request.method !== 'GET') return
  const url = new URL(request.url)
  if (url.origin !== self.location.origin) return

  // Pages: always the newest from the network, or the offline advice.
  if (request.mode === 'navigate') {
    event.respondWith(fetch(request).catch(() => caches.match(OFFLINE_PAGE)))
    return
  }

  // The icons the offline page shows.
  if (PRECACHE.includes(url.pathname)) {
    event.respondWith(fetch(request).catch(() => caches.match(request)))
    return
  }

  // Built files have a content hash in their name, so a stored copy can't go stale.
  if (url.pathname.startsWith('/assets/')) {
    event.respondWith((async () => {
      const stored = await caches.match(request)
      if (stored) return stored
      const response = await fetch(request)
      if (response.ok) event.waitUntil(keepAsset(request, response.clone()))
      return response
    })())
  }
  // Everything else, including all live data, goes straight to the network as usual.
})
