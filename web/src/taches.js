// Aides du module Tâches : projets, listes, périodes et durées.
export const projets = [
  { code: 'educari', nom: 'Educari' },
  { code: 'saas', nom: 'SaaS' },
  { code: 'microdash', nom: 'Microdash' },
  { code: 'perso', nom: 'Perso' },
]
export const nomProjet = (code) => projets.find((p) => p.code === code)?.nom || ''

// Une tâche est à faire chaque soir de sa période : deux dates, ou une semaine (lundi-dimanche).
export const listes = [
  { quand: 'dates', nom: 'Par dates', court: 'Dates' },
  { quand: 'semaine', nom: 'Par semaine', court: 'Semaine' },
  { quand: 'plus_tard', nom: 'Plus tard' },
]

const moisCourts = ['janv.', 'févr.', 'mars', 'avr.', 'mai', 'juin', 'juil.', 'août', 'sept.', 'oct.', 'nov.', 'déc.']
// Dates en AAAA-MM-JJ, calculées en UTC pour éviter les décalages d'heure d'été.
const lire = (iso) => new Date(`${iso}T00:00:00Z`)
const ecrire = (d) => d.toISOString().slice(0, 10)
export const ajouterJours = (iso, n) => {
  const d = lire(iso)
  d.setUTCDate(d.getUTCDate() + n)
  return ecrire(d)
}
export const lundi = (iso) => ajouterJours(iso, -((lire(iso).getUTCDay() + 6) % 7))
export function aujourdhuiLocal() {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// « 3 nov. »
function jourMois(iso) {
  const d = lire(iso)
  return `${d.getUTCDate() === 1 ? '1er' : d.getUTCDate()} ${moisCourts[d.getUTCMonth()]}`
}
// « du 2 au 8 nov. », « du 30 oct. au 5 nov. », « le 3 nov. »
export function intervalle(debut, fin) {
  if (debut === fin) return `le ${jourMois(debut)}`
  const memeMois = debut.slice(0, 7) === fin.slice(0, 7)
  return `du ${memeMois ? jourMois(debut).split(' ')[0] : jourMois(debut)} au ${jourMois(fin)}`
}

// Semaines proposées : celle en cours et les suivantes, plus celle déjà choisie si elle est passée.
export function semaines(aujourdhui, combien = 8, choisie = '') {
  const premier = lundi(aujourdhui)
  const res = Array.from({ length: combien }, (_, i) => ajouterJours(premier, 7 * i))
  if (choisie && !res.includes(choisie)) res.push(choisie)
  return res.sort().map((l) => {
    const ecart = Math.round((lire(l) - lire(premier)) / (7 * 864e5))
    const nom = ecart === 0 ? 'Cette semaine' : ecart === 1 ? 'Semaine prochaine' : ecart < 0 ? 'Semaine passée' : 'Semaine'
    return { debut: l, libelle: `${nom}, ${intervalle(l, ajouterJours(l, 6))}` }
  })
}

// La tâche est à faire le soir du jour donné.
export const enCours = (t, jour) => Boolean(t.debut) && t.debut <= jour && jour <= t.fin

// « du 2 au 8 nov. », avec « en cours » ou « dépassé ».
export function periode(t, jour) {
  if (!t.debut) return ''
  const texte = t.quand === 'semaine' ? `semaine ${intervalle(t.debut, t.fin)}` : intervalle(t.debut, t.fin)
  if (!t.faite && t.fin < jour) return `${texte}, dépassé`
  return texte
}

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
