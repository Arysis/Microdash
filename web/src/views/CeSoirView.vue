<script setup>
import { computed, onMounted, ref, watchEffect } from 'vue'
import { api } from '../api.js'
import { dateCourte, dateLongue } from '../echeances.js'
import { moduleActif } from '../session.js'
import { duree, listes, nomProjet } from '../taches.js'

const donnees = ref(null)
const erreur = ref('')
const enCours = ref('')
// Élément dont le choix « Plus tard » ou « Tâche » est ouvert : { cle, choix }.
const choixOuvert = ref(null)
// Tâches rangées dans « Ce soir » (null : module Tâches désactivé) et rendez-vous du jour.
const soiree = ref(null)
const rendezVous = ref([])

const reports = [
  { quand: 'demain', libelle: 'Demain' },
  { quand: 'week-end', libelle: 'Samedi' },
  { quand: 'semaine', libelle: 'Lundi' },
]

async function charger() {
  erreur.value = ''
  try {
    const cs = await api.get('/ce-soir')
    const j = cs.aujourdhui
    const [ts, cal] = await Promise.all([
      moduleActif('taches') ? api.get('/taches') : null,
      moduleActif('agenda') ? api.get(`/calendrier?du=${j}&au=${j}`).catch(() => null) : null,
    ])
    donnees.value = cs
    soiree.value = ts && ts.filter((t) => t.quand === 'ce_soir')
    rendezVous.value = (cal?.evenements || []).filter((e) => e.type === 'evenement')
  } catch (e) {
    erreur.value = e.message
  }
}
onMounted(charger)

const elements = computed(() => donnees.value?.elements || [])
const sources = computed(() => donnees.value?.sources || [])
const nomSource = (code) => sources.value.find((s) => s.code === code)?.nom || code
const cle = (e) => `${e.source}/${e.ref}`
const externe = (lien) => /^https?:\/\//.test(lien)

const tachesRestantes = computed(() => (soiree.value || []).filter((t) => !t.faite))
const minutesSoiree = computed(() => tachesRestantes.value.reduce((n, t) => n + (t.duree_minutes || 0), 0))

const resume = computed(() => {
  if (!donnees.value) return ''
  const jour = dateLongue(donnees.value.aujourdhui)
  const n = elements.value.length
  if (!n) return `${jour}. Rien à traiter.`
  return `${jour}. ${n} ${n > 1 ? 'choses' : 'chose'} à traiter.`
})

// Pastille sur l'icône de l'app installée : ce qui reste à traiter ou à faire ce soir.
watchEffect(() => {
  if (!donnees.value || !('setAppBadge' in navigator)) return
  const n = elements.value.length + tachesRestantes.value.length
  ;(n ? navigator.setAppBadge(n) : navigator.clearAppBadge()).catch(() => {})
})

function etiquette(e) {
  if (e.importance >= 2) return 'Urgent'
  if (e.importance === 1) return 'Important'
  return ''
}

function quandLibelle(e) {
  if (!e.date) return ''
  const d = donnees.value.aujourdhui
  if (e.date === d) return "Aujourd'hui"
  if (!e.limite) return `Reçu le ${dateCourte(e.date, Number(d.slice(0, 4)))}`
  if (e.date < d) return 'En retard'
  return `Avant le ${dateLongue(e.date).toLowerCase()}`
}
const enRetard = (e) => e.limite && e.date < donnees.value.aujourdhui
const ouvert = (e, choix) => choixOuvert.value?.cle === cle(e) && choixOuvert.value.choix === choix

// Retire l'élément de la liste tout de suite ; le remet si l'API refuse.
async function decider(e, chemin, corps, apres) {
  enCours.value = cle(e)
  erreur.value = ''
  const avant = donnees.value
  donnees.value = {
    ...avant,
    elements: avant.elements.filter((x) => cle(x) !== cle(e)),
    sources: avant.sources.map((s) => (s.code === e.source ? { ...s, nombre: s.nombre - 1 } : s)),
  }
  try {
    const res = await api.post(chemin, { source: e.source, ref: e.ref, ...corps })
    apres?.(res)
  } catch (err) {
    donnees.value = avant
    erreur.value = err.message
  } finally {
    enCours.value = ''
    choixOuvert.value = null
  }
}

const fait = (e) => decider(e, '/ce-soir/fait')
const plusTard = (e, quand) => decider(e, '/ce-soir/plus-tard', { quand })
// Range l'élément dans les tâches ; une tâche pour ce soir rejoint « Ta soirée ».
const enTache = (e, quand) => decider(e, '/ce-soir/tache', { quand }, (t) => {
  if (t.quand === 'ce_soir' && soiree.value) soiree.value = [...soiree.value, t]
})

async function cocher(t) {
  erreur.value = ''
  const maj = (x) => (soiree.value = soiree.value.map((y) => (y.id === x.id ? x : y)))
  maj({ ...t, faite: !t.faite })
  try {
    maj(await api.put(`/taches/${t.id}`, {
      titre: t.titre, notes: t.notes, projet: t.projet, duree_minutes: t.duree_minutes || 0,
      quand: t.quand, echeance: t.echeance || '', faite: !t.faite,
    }))
  } catch (err) {
    maj(t)
    erreur.value = err.message
  }
}
const metaTache = (t) => [nomProjet(t.projet), duree(t.duree_minutes)].filter(Boolean).join(' · ')
</script>

<template>
  <div class="bandeau">
    <h1>Ce soir</h1>
    <p>{{ resume }}</p>
  </div>

  <div class="pile-relief">
    <div v-if="sources.length" class="compteurs">
      <div v-for="s in sources" :key="s.code" class="carte compteur">
        <span class="aide">{{ s.nom }}</span>
        <strong :class="{ actif: s.nombre > 0 }">{{ s.nombre }}</strong>
      </div>
    </div>

    <section v-if="soiree || rendezVous.length" class="carte liste-soir" aria-labelledby="titre-soiree">
      <div class="titre-ligne">
        <h2 id="titre-soiree">Ta soirée</h2>
        <span v-if="minutesSoiree" class="aide">{{ duree(minutesSoiree) }}</span>
      </div>
      <ul v-if="rendezVous.length" class="rendez-vous">
        <li v-for="e in rendezVous" :key="e.ref">
          <RouterLink to="/agenda" class="rdv">
            <span class="heure">{{ e.heure || 'Journée' }}</span>
            <span class="titre">{{ e.titre }}</span>
          </RouterLink>
        </li>
      </ul>
      <p v-if="soiree && !soiree.length" class="vide">
        Aucune tâche pour ce soir. <RouterLink to="/taches">Choisir dans tes tâches</RouterLink>
      </p>
      <ul v-if="soiree">
        <li v-for="t in soiree" :key="t.id" :class="['tache', { faite: t.faite }]">
          <label class="case-tache">
            <input type="checkbox" :checked="t.faite" @change="cocher(t)" />
            <span>
              <span class="titre">{{ t.titre }}</span>
              <small v-if="metaTache(t)" class="aide">{{ metaTache(t) }}</small>
            </span>
          </label>
        </li>
      </ul>
    </section>

    <section class="carte liste-soir" aria-labelledby="titre-traiter">
      <h2 id="titre-traiter">À traiter</h2>
      <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
      <p v-if="donnees?.indisponibles?.length" class="aide">
        Indisponible pour le moment : {{ donnees.indisponibles.join(', ') }}.
      </p>
      <p v-if="donnees && !elements.length" class="vide">
        Tout est à jour. Tu peux fermer l'app.
      </p>
      <ul>
        <li v-for="e in elements" :key="cle(e)">
          <div class="meta">
            <span :class="['badge', { fort: e.importance > 0 }]">{{ nomSource(e.source) }}<template v-if="etiquette(e)"> · {{ etiquette(e) }}</template></span>
            <span v-if="quandLibelle(e)" :class="{ retard: enRetard(e) }">{{ quandLibelle(e) }}</span>
          </div>
          <p class="titre">{{ e.titre }}</p>
          <p v-if="e.detail" class="aide">{{ e.detail }}</p>

          <div v-if="ouvert(e, 'report')" class="actions" role="group" :aria-label="`Remettre « ${e.titre} » à plus tard`">
            <button v-for="r in reports" :key="r.quand" class="bouton secondaire" :disabled="enCours === cle(e)" @click="plusTard(e, r.quand)">{{ r.libelle }}</button>
            <button class="bouton secondaire" @click="choixOuvert = null">Annuler</button>
          </div>
          <div v-else-if="ouvert(e, 'tache')" class="actions" role="group" :aria-label="`Ranger « ${e.titre} » dans les tâches`">
            <button v-for="l in listes" :key="l.quand" class="bouton secondaire" :disabled="enCours === cle(e)" @click="enTache(e, l.quand)">{{ l.nom }}</button>
            <button class="bouton secondaire" @click="choixOuvert = null">Annuler</button>
          </div>
          <div v-else class="actions">
            <template v-if="e.lien">
              <a v-if="externe(e.lien)" class="bouton secondaire" :href="e.lien" target="_blank" rel="noopener">Ouvrir</a>
              <RouterLink v-else class="bouton secondaire" :to="e.lien">Ouvrir</RouterLink>
            </template>
            <button class="bouton secondaire" :disabled="enCours === cle(e)" @click="choixOuvert = { cle: cle(e), choix: 'report' }">Plus tard</button>
            <button v-if="soiree && e.source !== 'taches'" class="bouton secondaire" :disabled="enCours === cle(e)" @click="choixOuvert = { cle: cle(e), choix: 'tache' }">Tâche</button>
            <button v-if="e.finir" class="bouton principal" :disabled="enCours === cle(e)" @click="fait(e)">Fait</button>
          </div>
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.bandeau p { color: var(--blanc); font-size: 15px; }
.pile-relief { display: grid; gap: var(--e4); }
.compteurs { display: grid; grid-template-columns: repeat(auto-fill, minmax(104px, 1fr)); gap: var(--e2); }
.compteur { padding: var(--e3); display: grid; gap: 2px; }
.carte + .carte.compteur { margin-top: 0; }
.compteur strong { font-size: 28px; line-height: 1.2; font-variant-numeric: tabular-nums; }
.compteur strong.actif { color: var(--safran); }

.liste-soir { padding-bottom: var(--e2); }
.liste-soir h2 { margin-bottom: var(--e2); }
.liste-soir ul { list-style: none; margin: 0; padding: 0; }
.liste-soir li { display: grid; gap: var(--e1); padding: var(--e3) 0; border-top: 1px solid var(--lin); }
.vide { padding: var(--e3) 0; color: var(--pierre); }
.meta { display: flex; flex-wrap: wrap; gap: var(--e2); align-items: center; font-size: 13px; color: var(--pierre); }
.badge { background: var(--fond-relief); color: var(--pierre); border-radius: var(--rayon); padding: 2px var(--e2); font-weight: 600; }
.badge.fort { background: var(--safran-clair); color: var(--safran-fonce); }
.retard { color: var(--brique); font-weight: 600; }
.titre { font-weight: 600; }
.actions { display: flex; flex-wrap: wrap; gap: var(--e2); margin-top: var(--e2); }
.actions .bouton { flex: 1 1 0; min-width: 72px; padding: 0 var(--e2); font-size: 14px; }
.liste-soir .titre-ligne { margin-bottom: var(--e2); }
.liste-soir .titre-ligne h2 { margin: 0; }
.liste-soir li.tache { padding: 0; }
.case-tache { display: flex; align-items: center; gap: var(--e3); min-height: 52px; font-weight: 400; cursor: pointer; }
.case-tache input { width: 22px; height: 22px; min-height: 0; padding: 0; margin: 0 0 0 var(--e1); flex-shrink: 0; accent-color: var(--sarcelle); }
.case-tache > span { display: grid; gap: 2px; }
.faite .titre { text-decoration: line-through; color: var(--pierre); }
.rendez-vous li { padding: 0; }
.rdv { display: flex; align-items: center; gap: var(--e3); min-height: 48px; color: var(--encre); text-decoration: none; }
.rdv .heure { min-width: 56px; font-size: 13px; font-weight: 600; color: var(--sarcelle); font-variant-numeric: tabular-nums; }
</style>
