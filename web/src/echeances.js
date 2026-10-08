// Mise en forme des échéances de l'agenda renvoyées par /api/agenda.
import { api } from './api.js'

const mois = ['janvier', 'février', 'mars', 'avril', 'mai', 'juin', 'juillet', 'août', 'septembre', 'octobre', 'novembre', 'décembre']
const moisCourts = ['janv.', 'févr.', 'mars', 'avr.', 'mai', 'juin', 'juil.', 'août', 'sept.', 'oct.', 'nov.', 'déc.']
const jours = ['dimanche', 'lundi', 'mardi', 'mercredi', 'jeudi', 'vendredi', 'samedi']
const trimestres = ['1er', '2e', '3e', '4e']

const lire = (iso) => {
  const [a, m, j] = iso.split('-').map(Number)
  return new Date(Date.UTC(a, m - 1, j))
}
const numero = (d) => (d.getUTCDate() === 1 ? '1er' : String(d.getUTCDate()))

// « 2 nov. », avec l'année si elle diffère de celle affichée.
export function dateCourte(iso, annee) {
  const d = lire(iso)
  const texte = `${numero(d)} ${moisCourts[d.getUTCMonth()]}`
  return d.getUTCFullYear() === annee ? texte : `${texte} ${d.getUTCFullYear()}`
}

// « Lundi 2 novembre »
export function dateLongue(iso) {
  const d = lire(iso)
  const j = jours[d.getUTCDay()]
  return `${j[0].toUpperCase()}${j.slice(1)} ${numero(d)} ${mois[d.getUTCMonth()]}`
}

export const moisCourt = (iso) => moisCourts[lire(iso).getUTCMonth()]
export const jourDuMois = (iso) => lire(iso).getUTCDate()

// Nombre de jours entre aujourd'hui et la date limite (négatif si elle est passée).
export const joursRestants = (iso, aujourdhui) => Math.round((lire(iso) - lire(aujourdhui)) / 86400000)

export function dans(n) {
  if (n === 0) return "aujourd'hui"
  if (n === 1) return 'demain'
  return `dans ${n} jours`
}

// Pourquoi la date limite a été reportée : « Le 31 octobre tombe un samedi. »
export function raisonReport(e) {
  if (!e.date_legale) return ''
  const d = lire(e.date_legale)
  const date = `${numero(d)} ${mois[d.getUTCMonth()]}`
  const jour = d.getUTCDay()
  return jour === 0 || jour === 6 ? `Le ${date} tombe un ${jours[jour]}.` : `Le ${date} est férié.`
}

// Libellé court pour la liste : « URSSAF, 3e trimestre 2026 », « URSSAF, juin 2026 ».
export function libelleCourt(e) {
  const m = /^urssaf-(\d{4})-(t([1-4])|(\d{2}))$/.exec(e.code)
  if (!m || e.libelle.startsWith('Première')) return e.libelle
  return m[3] ? `URSSAF, ${trimestres[m[3] - 1]} trimestre ${m[1]}` : `URSSAF, ${mois[Number(m[4]) - 1]} ${m[1]}`
}

// Titre de la prochaine échéance, sans l'année quand c'est celle de la date limite :
// « Déclaration URSSAF du 3e trimestre » (2 novembre 2026), mais « … du 4e trimestre 2026 » (1er février 2027).
export function libelleSansAnnee(e) {
  const m = / (\d{4})$/.exec(e.libelle)
  return m && e.date?.startsWith(m[1]) ? e.libelle.slice(0, m.index) : e.libelle
}

// Prochaine échéance datée, pas encore faite, à partir d'aujourd'hui.
export const prochaine = (echeances, aujourdhui) =>
  echeances.find((e) => e.date && !e.faite && e.date >= aujourdhui) || null

export const lienDeclaration = (e) =>
  e.type === 'urssaf' ? 'https://www.autoentrepreneur.urssaf.fr' : 'https://www.impots.gouv.fr'

// Déclarations URSSAF qu'un paiement du jour donné peut régler (année du paiement et précédente),
// la plus récente en premier.
export async function declarationsURSSAF(date) {
  const a = Number(String(date).slice(0, 4))
  if (!a) return []
  const listes = await Promise.all([a, a - 1].map((x) => api.get(`/agenda?annee=${x}`).catch(() => ({ echeances: [] }))))
  const vues = new Set()
  return listes
    .flatMap((l) => l.echeances)
    .filter((e) => e.type === 'urssaf' && e.periode_debut <= date && !vues.has(e.code) && vues.add(e.code))
    .sort((x, y) => (x.periode_fin < y.periode_fin ? 1 : -1))
}
