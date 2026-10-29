/* Chỉ lưu tài nguyên công khai; không lưu trang HTML, dữ liệu truyện, API hay phiên đăng nhập. */
const STATIC_CACHE = 'ainovel-mobile-static-v3'
const APP_ASSETS = [
  '/manifest.webmanifest', '/icons/icon-192.png', '/icons/icon-512.png', '/icons/icon-maskable-512.png',
  '/icons/apple-touch-icon.png', '/icons/favicon.svg',
]
const MIME_TYPES = {
  js: ['text/javascript', 'application/javascript'],
  mjs: ['text/javascript', 'application/javascript'],
  css: ['text/css'],
  svg: ['image/svg+xml'],
  png: ['image/png'],
  jpg: ['image/jpeg'],
  jpeg: ['image/jpeg'],
  webp: ['image/webp'],
  ico: ['image/x-icon', 'image/vnd.microsoft.icon'],
  woff: ['font/woff', 'application/font-woff'],
  woff2: ['font/woff2'],
}

function expectedTypes(url) {
  if (url.origin !== self.location.origin || url.search || /^\/(api|auth)(\/|$)/i.test(url.pathname)) return null
  if (url.pathname === '/manifest.webmanifest') return ['application/manifest+json', 'application/json']
  if (/^\/icons\/[a-z0-9-]+\.png$/.test(url.pathname)) return MIME_TYPES.png
  if (url.pathname === '/icons/favicon.ico') return MIME_TYPES.ico
  if (url.pathname === '/icons/favicon.svg') return MIME_TYPES.svg
  const asset = /^\/assets\/[a-zA-Z0-9_-]+\.([a-z0-9]+)$/.exec(url.pathname)
  return asset ? MIME_TYPES[asset[1]] ?? null : null
}

function cacheable(response, types) {
  const mime = (response.headers.get('Content-Type') ?? '').split(';')[0].trim().toLowerCase()
  return response.ok && response.type === 'basic' && !response.redirected && types.includes(mime)
    && !/\b(no-store|private)\b/i.test(response.headers.get('Cache-Control') ?? '')
}

self.addEventListener('install', (event) => {
  event.waitUntil((async () => {
    const cache = await caches.open(STATIC_CACHE)
    await Promise.all(APP_ASSETS.map(async (path) => {
      const response = await fetch(path, { credentials: 'omit', cache: 'reload' })
      const types = expectedTypes(new URL(path, self.location.origin))
      if (cacheable(response, types)) await cache.put(path, response)
    }))
    await self.skipWaiting()
  })())
})

self.addEventListener('activate', (event) => {
  event.waitUntil((async () => {
    const names = await caches.keys()
    await Promise.all(names.filter((name) => name.startsWith('ainovel-mobile-static-') && name !== STATIC_CACHE).map((name) => caches.delete(name)))
    await self.clients.claim()
  })())
})

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET' || event.request.mode === 'navigate') return
  const types = expectedTypes(new URL(event.request.url))
  if (!types) return
  event.respondWith((async () => {
    const cache = await caches.open(STATIC_CACHE)
    const cached = await cache.match(event.request)
    if (cached) return cached
    const response = await fetch(event.request)
    if (cacheable(response, types)) await cache.put(event.request, response.clone())
    return response
  })())
})

// Web Push không mang dữ liệu phiên và không đi qua cache tài nguyên.
function notificationURL(path) {
  if (typeof path !== 'string' || !path.startsWith('/') || path.startsWith('//')) return '/notifications'
  return path
}

self.addEventListener('push', (event) => {
  event.waitUntil((async () => {
    let payload
    try { payload = event.data?.json() } catch { return }
    if (!payload || typeof payload.title !== 'string') return
    const actions = Array.isArray(payload.actions) ? payload.actions
      .filter((action) => typeof action.action === 'string' && typeof action.title === 'string')
      .map(({ action, title }) => ({ action, title })) : []
    await self.registration.showNotification(payload.title, {
      body: typeof payload.body === 'string' ? payload.body : '',
      icon: '/icons/icon-192.png', badge: '/icons/icon-192.png',
      tag: payload.id || `ainovel-${payload.type || 'notification'}`,
      data: { ...payload, url: notificationURL(payload.url) },
      actions, requireInteraction: payload.type === 'gate' || payload.type === 'error',
    })
    const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
    windows.forEach((client) => client.postMessage({ type: 'ainovel.notification', id: payload.id }))
  })())
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  event.waitUntil((async () => {
    const payload = event.notification.data || {}
    let url = notificationURL(payload.url)
    let expired = false
    let actionError = ''
    if (event.action && payload.id) {
      try {
        const response = await fetch(`/api/notifications/${encodeURIComponent(payload.id)}/action`, {
          method: 'POST', credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ action: event.action }),
        })
        expired = response.status === 401
        const result = await response.json().catch(() => ({}))
        if (!response.ok && !expired) actionError = result.error || 'Không thực hiện được thao tác. Mở Noverse để thử lại.'
        if (response.ok && result.url) url = notificationURL(result.url)
      } catch { actionError = 'Mất kết nối. Thao tác chưa được gửi; mở Noverse để thử lại.' }
    }
    if (!expired) {
      try { expired = (await fetch('/api/me', { credentials: 'same-origin', cache: 'no-store' })).status === 401 } catch { /* Mất mạng vẫn mở đúng trang đích. */ }
    }
    if (actionError) await self.registration.showNotification('Chưa thực hiện được thao tác', {
      body: actionError, icon: '/icons/icon-192.png', data: { url },
    })
    // Liên kết tải tệp (/api/…) không phải route của app: mở thẳng. Phiên hết hạn thì vào hộp thư để đăng nhập.
    if (url.startsWith('/api/')) {
      if (!expired) { await self.clients.openWindow(new URL(url, self.location.origin).href); return }
      url = '/notifications'
    }
    const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
    const existing = windows.find((client) => new URL(client.url).origin === self.location.origin)
    if (existing) {
      await existing.focus()
      existing.postMessage({ type: 'ainovel.navigate', url, expired, action: actionError ? '' : event.action })
    } else {
      // Bootstrap kiểm tra phiên; hash đích được giữ qua bước đăng nhập.
      const target = new URL('/', self.location.origin)
      target.hash = `#${url}${event.action === 'share' && !actionError ? `${url.includes('?') ? '&' : '?'}share=1` : ''}`
      await self.clients.openWindow(target.href)
    }
  })())
})
