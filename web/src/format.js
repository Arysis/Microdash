const euros = new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR' })

export const formatEuros = (centimes) => euros.format((centimes || 0) / 100)

export const formatDate = (iso) =>
  new Date(`${iso}T00:00:00`).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' })

export const aujourdhui = () => {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// "12,50" ou "12.5" → 1250 centimes ; NaN si la saisie est invalide.
export const versCentimes = (saisie) => {
  const n = Number(String(saisie).replace(/\s/g, '').replace(',', '.'))
  return Number.isFinite(n) ? Math.round(n * 100) : NaN
}

export const postesDepense = [
  'Matériel',
  'Logiciels et abonnements',
  'Déplacements',
  'Formation',
  'Frais bancaires',
  'Assurance',
  'URSSAF (cotisations et impôt)',
  'Marketing',
  'Autre',
]

export const posteURSSAF = 'URSSAF (cotisations et impôt)'
