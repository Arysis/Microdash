import { reactive } from 'vue'

// Fenêtre de confirmation de l'app, à la place de confirm() du navigateur.
export const confirmation = reactive({ ouverte: false, titre: '', message: '', action: '', resoudre: null })

export function demanderConfirmation({ titre, message = '', action = 'Confirmer' }) {
  return new Promise((resoudre) => {
    Object.assign(confirmation, { ouverte: true, titre, message, action, resoudre })
  })
}

export function repondre(ok) {
  const r = confirmation.resoudre
  Object.assign(confirmation, { ouverte: false, resoudre: null })
  r?.(ok)
}
