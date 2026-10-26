import { reactive } from 'vue'
import { api, ApiError } from './api.js'

// État partagé de l'utilisateur connecté. modules : modules du copilote et leur activation.
export const session = reactive({ charge: false, user: null, modules: null })

export async function chargerModules() {
  try {
    session.modules = await api.get('/modules')
  } catch (e) {
    console.error(e)
    session.modules = []
  }
}

// Un module est actif tant que la liste n'est pas chargée, pour ne pas faire clignoter le menu.
export const moduleActif = (code) => session.modules?.find((m) => m.code === code)?.actif ?? true

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
  session.modules = null
}

export async function supprimerCompte() {
  await api.del('/me')
  await viderCopieLocale()
  session.user = null
  session.modules = null
}
