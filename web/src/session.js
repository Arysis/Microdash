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

export async function deconnecter() {
  await api.post('/auth/logout')
  session.user = null
}
