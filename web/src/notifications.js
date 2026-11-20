// Notifications push sur cet appareil : abonnement du navigateur et enregistrement côté serveur.
import { api } from './api.js'

const ios = () => /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
const installee = () => window.matchMedia?.('(display-mode: standalone)').matches || navigator.standalone === true

// etatAppareil dit ce que cet appareil peut faire :
// « ok », « installer » (iPhone : ajouter l'app à l'écran d'accueil d'abord), « refuse » (bloqué
// dans le navigateur) ou « impossible » (navigateur sans notifications push).
export function etatAppareil() {
  if (ios() && !installee()) return 'installer'
  if (!('serviceWorker' in navigator) || !('PushManager' in window) || !('Notification' in window)) return 'impossible'
  if (Notification.permission === 'denied') return 'refuse'
  return 'ok'
}

// Le service worker n'existe qu'une fois l'app construite (pas en « npm run dev »).
async function serviceWorker() {
  const reg = await Promise.race([
    navigator.serviceWorker.ready,
    new Promise((resolve) => setTimeout(() => resolve(null), 8000)),
  ])
  if (!reg) throw new Error("Le service worker de l'app n'est pas prêt : recharge la page et réessaie.")
  return reg
}

export async function abonnementActuel() {
  if (etatAppareil() !== 'ok') return null
  const reg = await navigator.serviceWorker.getRegistration()
  return reg ? reg.pushManager.getSubscription() : null
}

function cleBinaire(base64url) {
  const b64 = base64url.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (base64url.length % 4)) % 4)
  return Uint8Array.from(atob(b64), (c) => c.charCodeAt(0))
}

// nomAppareil donne un nom lisible dans la liste des appareils, sans rien d'identifiant.
export function nomAppareil() {
  const ua = navigator.userAgent
  const navigateur = /Edg\//.test(ua) ? 'Edge' : /Firefox\//.test(ua) ? 'Firefox' : /OPR\//.test(ua) ? 'Opera' : /Chrome\//.test(ua) ? 'Chrome' : /Safari\//.test(ua) ? 'Safari' : 'Navigateur'
  const systeme = /Android/.test(ua) ? 'Android' : ios() ? 'iPhone' : /Windows/.test(ua) ? 'Windows' : /Mac OS X/.test(ua) ? 'Mac' : /Linux/.test(ua) ? 'Linux' : ''
  const nom = systeme ? `${navigateur} sur ${systeme}` : navigateur
  return installee() ? `${nom} (app installée)` : nom
}

// activer demande la permission, abonne le navigateur et l'enregistre sur le serveur.
export async function activer(clePublique) {
  const permission = await Notification.requestPermission()
  if (permission !== 'granted') {
    throw new Error(permission === 'denied'
      ? 'Les notifications sont bloquées pour Microdash dans ce navigateur : autorise-les dans les réglages du site.'
      : 'Notifications non autorisées.')
  }
  const reg = await serviceWorker()
  const cle = cleBinaire(clePublique)
  let abonnement = await reg.pushManager.getSubscription()
  // Abonnement fait avec une autre clé du serveur : on le refait.
  const ancienne = abonnement?.options?.applicationServerKey
  if (abonnement && ancienne && new Uint8Array(ancienne).toString() !== cle.toString()) {
    await abonnement.unsubscribe()
    abonnement = null
  }
  if (!abonnement) {
    try {
      abonnement = await Promise.race([
        reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: cle }),
        new Promise((_, refuser) => setTimeout(() => refuser(new Error('pas de réponse du service de notifications')), 20000)),
      ])
    } catch (e) {
      // Cas connu : la navigation privée de Chrome n'a pas de notifications push.
      throw new Error(`Le navigateur refuse l'abonnement (${e.message}). En navigation privée, ouvre une fenêtre normale.`)
    }
  }
  const json = abonnement.toJSON()
  await api.post('/notifications/abonnements', { endpoint: json.endpoint, keys: json.keys, appareil: nomAppareil() })
  return abonnement
}

export async function desactiver() {
  const abonnement = await abonnementActuel()
  if (!abonnement) return
  await api.post('/notifications/desabonner', { endpoint: abonnement.endpoint })
  await abonnement.unsubscribe()
}
