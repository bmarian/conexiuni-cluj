// Imported by the generated service worker (workbox.importScripts in vite.config.ts).

self.addEventListener('activate', (event) => {
  event.waitUntil(Promise.all(['map-tiles', 'map-tiles-v2', 'leaflet-assets'].map((name) => caches.delete(name))))
})

self.addEventListener('push', (event) => {
  let message
  try {
    message = event.data?.json()
  } catch {
    return
  }
  if (!message?.title) return
  event.waitUntil((async () => {
    // Marked by retractReminder in pushSubscription.ts.
    const cancelled = message.tag && await caches.match(`/cancelled/${message.tag}`, {cacheName: 'cancelled-reminders'})
    if (cancelled && Number(await cancelled.text()) >= Date.now()) return
    await self.registration.showNotification(message.title, {
      body: message.body,
      tag: message.tag,
      renotify: Boolean(message.tag),
      requireInteraction: Boolean(message.alert),
      timestamp: message.timestamp,
      icon: '/pwa-192x192.png',
      data: {url: message.url || '/'},
    })
  })())
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = event.notification.data?.url || '/'
  event.waitUntil((async () => {
    const windows = await self.clients.matchAll({type: 'window', includeUncontrolled: true})
    const client = windows.find((c) => c.focused) ?? windows[0]
    if (client) {
      try {
        await client.focus()
        client.postMessage({type: 'open-url', url})
        return
      } catch {}
    }
    await self.clients.openWindow(url)
  })())
})
