import { reactive } from 'vue'
import { api, ApiError } from './api.js'

// État partagé de l'utilisateur connecté.
export const session = reactive({ charge: false, user: null })

export async function chargerSession() {
  try {
    session.user = await api.get('/me')
  } catch (e) {
    if (!(e instanceof ApiError) || e.status !== 401) console.error(e)
    session.user = null
  }
  session.charge = true
}

// La copie hors ligne des réponses de l'API (cache « api » du service worker) contient
// des données financières : on l'efface dès que la personne quitte son compte.
async function viderCopieLocale() {
  if ('caches' in window) await caches.delete('api').catch(() => {})
}

export async function deconnecter() {
  await api.post('/auth/logout')
  await viderCopieLocale()
  session.user = null
}

export async function supprimerCompte() {
  await api.del('/me')
  await viderCopieLocale()
  session.user = null
}
