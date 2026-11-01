self.addEventListener('install', () => self.skipWaiting())
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()))
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = new URL(event.notification.data?.href || '/', self.location.origin)
  if (url.origin !== self.location.origin) return
  event.waitUntil(self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(async (clients) => {
    const client = clients[0]
    if (client) { await client.navigate(url.href); return client.focus() }
    return self.clients.openWindow(url.href)
  }))
})
