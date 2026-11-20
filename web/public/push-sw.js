// Notifications push, chargé par le service worker de l'app (importScripts, voir vite.config.js).
// Le serveur envoie { titre, corps, lien, tag } ; un clic ouvre la page de l'app correspondante.
self.addEventListener('push', (event) => {
  let n = {}
  try {
    n = event.data ? event.data.json() : {}
  } catch {
    n = { corps: event.data ? event.data.text() : '' }
  }
  event.waitUntil(
    self.registration.showNotification(n.titre || 'Microdash', {
      body: n.corps || '',
      tag: n.tag || undefined,
      renotify: Boolean(n.tag),
      icon: '/icon-192.png',
      badge: '/icon-192.png',
      lang: 'fr',
      data: { lien: n.lien || '/ce-soir' },
    }),
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const lien = new URL(event.notification.data?.lien || '/ce-soir', self.location.origin)
  if (lien.origin !== self.location.origin) return
  event.waitUntil(
    (async () => {
      const fenetres = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      const ouverte = fenetres.find((c) => new URL(c.url).origin === self.location.origin)
      if (ouverte) {
        await ouverte.focus()
        if ('navigate' in ouverte) await ouverte.navigate(lien.href).catch(() => {})
        return
      }
      await self.clients.openWindow(lien.href)
    })(),
  )
})

// Le navigateur a renouvelé l'abonnement : on prévient le serveur avec le nouveau.
self.addEventListener('pushsubscriptionchange', (event) => {
  const options = event.oldSubscription?.options
  if (!options) return
  event.waitUntil(
    (async () => {
      const nouvel = event.newSubscription || (await self.registration.pushManager.subscribe(options))
      await fetch('/api/notifications/abonnements', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'microdash' },
        body: JSON.stringify({ ...nouvel.toJSON(), appareil: 'Appareil renouvelé' }),
      })
    })(),
  )
})
