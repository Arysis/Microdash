<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api.js'
import { dateLongue } from '../echeances.js'

const donnees = ref(null)
const erreur = ref('')
const enCours = ref('')
// Élément dont le choix « Plus tard » est ouvert.
const reportOuvert = ref('')

const reports = [
  { quand: 'demain', libelle: 'Demain' },
  { quand: 'week-end', libelle: 'Samedi' },
  { quand: 'semaine', libelle: 'Lundi' },
]

async function charger() {
  erreur.value = ''
  try {
    donnees.value = await api.get('/ce-soir')
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

const resume = computed(() => {
  if (!donnees.value) return ''
  const jour = dateLongue(donnees.value.aujourdhui)
  const n = elements.value.length
  if (!n) return `${jour}. Rien à traiter.`
  return `${jour}. ${n} ${n > 1 ? 'choses' : 'chose'} à traiter.`
})

function etiquette(e) {
  if (e.importance >= 2) return 'Urgent'
  if (e.importance === 1) return 'Important'
  return ''
}

function quandLibelle(e) {
  if (!e.date) return ''
  const d = donnees.value.aujourdhui
  if (e.date < d) return 'En retard'
  if (e.date === d) return "Aujourd'hui"
  return `Avant le ${dateLongue(e.date).toLowerCase()}`
}

// Retire l'élément de la liste tout de suite ; le remet si l'API refuse.
async function decider(e, chemin, corps) {
  enCours.value = cle(e)
  erreur.value = ''
  const avant = donnees.value
  donnees.value = {
    ...avant,
    elements: avant.elements.filter((x) => cle(x) !== cle(e)),
    sources: avant.sources.map((s) => (s.code === e.source ? { ...s, nombre: s.nombre - 1 } : s)),
  }
  try {
    await api.post(chemin, { source: e.source, ref: e.ref, ...corps })
  } catch (err) {
    donnees.value = avant
    erreur.value = err.message
  } finally {
    enCours.value = ''
    reportOuvert.value = ''
  }
}

const fait = (e) => decider(e, '/ce-soir/fait')
const plusTard = (e, quand) => decider(e, '/ce-soir/plus-tard', { quand })
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
            <span v-if="quandLibelle(e)" :class="{ retard: e.date < donnees.aujourdhui }">{{ quandLibelle(e) }}</span>
          </div>
          <p class="titre">{{ e.titre }}</p>
          <p v-if="e.detail" class="aide">{{ e.detail }}</p>

          <div v-if="reportOuvert === cle(e)" class="actions" role="group" :aria-label="`Remettre « ${e.titre} » à plus tard`">
            <button v-for="r in reports" :key="r.quand" class="bouton secondaire" :disabled="enCours === cle(e)" @click="plusTard(e, r.quand)">{{ r.libelle }}</button>
            <button class="bouton secondaire" @click="reportOuvert = ''">Annuler</button>
          </div>
          <div v-else class="actions">
            <template v-if="e.lien">
              <a v-if="externe(e.lien)" class="bouton secondaire" :href="e.lien" target="_blank" rel="noopener">Ouvrir</a>
              <RouterLink v-else class="bouton secondaire" :to="e.lien">Ouvrir</RouterLink>
            </template>
            <button class="bouton secondaire" :disabled="enCours === cle(e)" @click="reportOuvert = cle(e)">Plus tard</button>
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
.actions .bouton { flex: 1 1 0; min-width: 88px; padding: 0 var(--e3); font-size: 14px; }
</style>
