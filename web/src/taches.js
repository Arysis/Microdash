// Aides du module Tâches : projets, listes et durées.
export const projets = [
  { code: 'educari', nom: 'Educari' },
  { code: 'saas', nom: 'SaaS' },
  { code: 'microdash', nom: 'Microdash' },
  { code: 'perso', nom: 'Perso' },
]
export const nomProjet = (code) => projets.find((p) => p.code === code)?.nom || ''

export const listes = [
  { quand: 'ce_soir', nom: 'Ce soir' },
  { quand: 'semaine', nom: 'Cette semaine', court: 'Semaine' },
  { quand: 'plus_tard', nom: 'Plus tard' },
]

// « 15 min », « 1 h », « 1 h 30 »
export function duree(min) {
  if (!min) return ''
  const h = Math.floor(min / 60)
  const m = min % 60
  if (!h) return `${m} min`
  if (!m) return `${h} h`
  return `${h} h ${String(m).padStart(2, '0')}`
}

// Sépare la durée écrite à la fin d'une tâche : « Relancer Paul 15 min » donne
// { titre: 'Relancer Paul', minutes: 15 }. Formes lues : 15 min, 15mn, 1 h, 1h30, 1 h 30, 2h.
export function lireDuree(texte) {
  const t = texte.trim()
  const m = t.match(/\s+(?:(\d{1,2})\s*h(?:\s*(\d{1,2}))?|(\d{1,4})\s*(?:min|mn|m))\s*$/i)
  if (!m) return { titre: t, minutes: 0 }
  const minutes = m[3] ? Number(m[3]) : Number(m[1]) * 60 + Number(m[2] || 0)
  if (!minutes || minutes > 1440) return { titre: t, minutes: 0 }
  return { titre: t.slice(0, m.index).trim(), minutes }
}
