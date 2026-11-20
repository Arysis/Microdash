<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api.js'
import { demanderConfirmation } from '../confirmation.js'
import { dateCourte } from '../echeances.js'
import { projets, nomProjet, listes, duree, lireDuree, aujourdhuiLocal, lundi, semaines, enCours as prevueLe, periode, intervalle, nomRythme, rythmes } from '../taches.js'

const taches = ref(null)
const erreur = ref('')
const filtre = ref('')
const jour = aujourdhuiLocal()
const nouvelle = reactive({ texte: '', quand: 'semaine', debut: jour, fin: jour, semaine: lundi(jour), repeter: '' })
const ajout = ref(false)
const plusTardOuvert = ref(false)
// Tâche en cours de modification, et sa copie modifiable.
const edition = ref(null)
const brouillon = reactive({})
const enCours = ref(false)

async function charger() {
  try {
    ;[taches.value, recurrentes.value] = await Promise.all([api.get('/taches'), api.get('/taches/recurrentes')])
  } catch (e) {
    erreur.value = e.message
  }
}
onMounted(() => {
  charger()
  chargerBilan()
})

// --- Tâches récurrentes ---
const recurrentes = ref([])

async function arreter(r) {
  const ok = await demanderConfirmation({
    titre: 'Arrêter cette tâche récurrente ?',
    message: `« ${r.titre} » ne reviendra plus. Les prochaines pas encore faites sont effacées, les autres restent.`,
    action: 'Arrêter',
  })
  if (!ok) return
  try {
    await api.del(`/taches/recurrentes/${r.id}`)
    await charger()
  } catch (e) {
    erreur.value = e.message
  }
}

// --- Historique : bilan de chaque semaine, calculé par l'API ---
const bilan = ref(null)
const nbSemaines = ref(8)
const erreurBilan = ref('')

async function chargerBilan() {
  try {
    bilan.value = (await api.get(`/taches/bilan?semaines=${nbSemaines.value}`)).semaines
  } catch (e) {
    erreurBilan.value = e.message
  }
}

function plusDeSemaines() {
  nbSemaines.value = Math.min(nbSemaines.value + 8, 104)
  chargerBilan()
}

function resumeSemaine(s) {
  const morceaux = [`${s.faites.length} ${s.faites.length > 1 ? 'faites' : 'faite'}`]
  if (s.non_faites.length) morceaux.push(`${s.non_faites.length} ${s.en_cours ? 'à finir' : 'pas faites'}`)
  if (s.minutes) morceaux.push(duree(s.minutes))
  return morceaux.join(' · ')
}

const repartition = (s) => Object.entries(s.par_projet).map(([p, n]) => `${nomProjet(p) || 'Sans projet'} : ${n}`).join(' · ')

const annee = new Date().getFullYear()
const visibles = computed(() => (taches.value || []).filter((t) => !filtre.value || t.projet === filtre.value))
const parListe = (quand) => visibles.value.filter((t) => t.quand === quand)
const aFaire = computed(() => visibles.value.filter((t) => !t.faite).length)
const minutes = (ts) => ts.filter((t) => !t.faite).reduce((n, t) => n + (t.duree_minutes || 0), 0)

const resume = computed(() => {
  if (!taches.value) return ''
  const n = aFaire.value
  const soir = minutes(visibles.value.filter((t) => prevueLe(t, jour)))
  const texte = n ? `${n} ${n > 1 ? 'tâches' : 'tâche'} à faire` : 'Rien à faire'
  return soir ? `${texte}. Ce soir : ${duree(soir)}.` : `${texte}.`
})

function meta(t) {
  const morceaux = []
  if (!filtre.value && t.projet) morceaux.push(nomProjet(t.projet))
  if (t.debut) morceaux.push(periode(t, jour))
  if (t.duree_minutes) morceaux.push(duree(t.duree_minutes))
  if (t.echeance) morceaux.push(`avant le ${dateCourte(t.echeance, annee)}`)
  if (t.source === 'recurrente') morceaux.push('répétée')
  return morceaux.join(' · ')
}

async function ajouter() {
  const { titre, minutes: m } = lireDuree(nouvelle.texte)
  if (!titre) return
  ajout.value = true
  erreur.value = ''
  try {
    if (nouvelle.quand === 'dates' && nouvelle.repeter) {
      await api.post('/taches/recurrentes', { titre, duree_minutes: m, projet: filtre.value, rythme: nouvelle.repeter, depuis: nouvelle.debut })
      await charger()
    } else {
      const t = await api.post('/taches', { titre, duree_minutes: m, projet: filtre.value, ...periodeDe(nouvelle) })
      taches.value = [...taches.value, t]
    }
    nouvelle.texte = ''
  } catch (e) {
    erreur.value = e.message
  } finally {
    ajout.value = false
  }
}

// Début et fin envoyés selon la liste : deux dates, le lundi de la semaine choisie, ou rien.
function periodeDe(f) {
  if (f.quand === 'dates') return { quand: 'dates', debut: f.debut, fin: f.fin < f.debut ? f.debut : f.fin }
  if (f.quand === 'semaine') return { quand: 'semaine', debut: f.semaine }
  return { quand: 'plus_tard' }
}

const corps = (t) => ({
  titre: t.titre, notes: t.notes, projet: t.projet, duree_minutes: Number(t.duree_minutes) || 0,
  echeance: t.echeance || '', faite: t.faite, ...periodeDe(t),
})

// Brouillon d'édition : la semaine est son lundi ; une tâche sans dates part d'aujourd'hui.
const versBrouillon = (t) => ({
  ...t, debut: t.debut || jour, fin: t.fin || t.debut || jour, semaine: t.debut ? lundi(t.debut) : lundi(jour),
})

function remplacer(t) {
  taches.value = taches.value.map((x) => (x.id === t.id ? t : x))
}

// Coche tout de suite ; décoche si l'API refuse.
async function cocher(t) {
  erreur.value = ''
  const avant = t
  remplacer({ ...t, faite: !t.faite })
  try {
    remplacer(await api.put(`/taches/${t.id}`, { ...corps(versBrouillon(t)), faite: !t.faite }))
    chargerBilan()
  } catch (e) {
    remplacer(avant)
    erreur.value = e.message
  }
}

function modifier(t) {
  edition.value = t.id
  Object.assign(brouillon, versBrouillon(t))
}

async function enregistrer() {
  enCours.value = true
  erreur.value = ''
  try {
    remplacer(await api.put(`/taches/${edition.value}`, corps(brouillon)))
    edition.value = null
  } catch (e) {
    erreur.value = e.message
  } finally {
    enCours.value = false
  }
}

async function supprimer(t) {
  const ok = await demanderConfirmation({ titre: 'Supprimer la tâche ?', message: `« ${t.titre} » sera effacée.`, action: 'Supprimer' })
  if (!ok) return
  try {
    await api.del(`/taches/${t.id}`)
    taches.value = taches.value.filter((x) => x.id !== t.id)
    edition.value = null
  } catch (e) {
    erreur.value = e.message
  }
}
</script>

<template>
  <div class="bandeau">
    <h1>Tâches</h1>
    <p>{{ resume }}</p>
    <div class="bascule filtres" role="group" aria-label="Projet">
      <button :class="{ actif: !filtre }" :aria-pressed="!filtre" @click="filtre = ''">Tout</button>
      <button v-for="p in projets" :key="p.code" :class="{ actif: filtre === p.code }" :aria-pressed="filtre === p.code" @click="filtre = p.code">{{ p.nom }}</button>
    </div>
  </div>

  <div class="pile-relief">
    <form class="carte ajout" @submit.prevent="ajouter">
      <label for="nouvelle-tache" class="sr-only">Nouvelle tâche</label>
      <div class="champ-ajout">
        <input id="nouvelle-tache" v-model="nouvelle.texte" maxlength="220" placeholder="Ex. : Relancer Paul 15 min" autocomplete="off" />
        <button class="bouton principal" :disabled="ajout || !nouvelle.texte.trim()">Ajouter</button>
      </div>
      <div class="bascule" role="group" aria-label="Liste de la nouvelle tâche">
        <button v-for="l in listes" :key="l.quand" type="button" :class="{ actif: nouvelle.quand === l.quand }" :aria-pressed="nouvelle.quand === l.quand" @click="nouvelle.quand = l.quand">{{ l.court || l.nom }}</button>
      </div>
      <div v-if="nouvelle.quand === 'dates'" class="ligne">
        <label>{{ nouvelle.repeter ? 'À partir du' : 'Du' }} <input v-model="nouvelle.debut" type="date" required /></label>
        <label v-if="!nouvelle.repeter">Au <input v-model="nouvelle.fin" type="date" :min="nouvelle.debut" required /></label>
        <label>Répéter
          <select v-model="nouvelle.repeter">
            <option value="">Jamais</option>
            <option v-for="r in rythmes" :key="r.code" :value="r.code">{{ nomRythme(r.code, nouvelle.debut) }}</option>
          </select>
        </label>
      </div>
      <label v-else-if="nouvelle.quand === 'semaine'">Semaine
        <select v-model="nouvelle.semaine">
          <option v-for="s in semaines(jour)" :key="s.debut" :value="s.debut">{{ s.libelle }}</option>
        </select>
      </label>
      <p class="aide">
        La tâche est à faire chaque soir de sa période et s'affiche ces jours-là dans l'agenda. Une durée à la fin (15 min, 1 h 30) est comptée dans ta soirée.<template v-if="filtre"> La tâche va dans {{ nomProjet(filtre) }}.</template>
      </p>
    </form>

    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>

    <section v-for="l in listes" :key="l.quand" class="carte liste-taches" :aria-labelledby="`liste-${l.quand}`">
      <div class="titre-ligne">
        <h2 :id="`liste-${l.quand}`">{{ l.nom }}</h2>
        <span class="aide">
          <template v-if="minutes(parListe(l.quand))">{{ duree(minutes(parListe(l.quand))) }}</template>
          <button v-if="l.quand === 'plus_tard'" class="lien-bouton" :aria-expanded="plusTardOuvert" @click="plusTardOuvert = !plusTardOuvert">
            {{ plusTardOuvert ? 'Replier' : `Voir (${parListe(l.quand).length})` }}
          </button>
        </span>
      </div>
      <template v-if="l.quand !== 'plus_tard' || plusTardOuvert">
        <p v-if="taches && !parListe(l.quand).length" class="vide">Rien ici.</p>
        <ul>
          <li v-for="t in parListe(l.quand)" :key="t.id" :class="{ faite: t.faite }">
            <form v-if="edition === t.id" class="edition" @submit.prevent="enregistrer">
              <label>Tâche <input v-model="brouillon.titre" maxlength="200" required /></label>
              <div class="ligne">
                <label>Projet
                  <select v-model="brouillon.projet">
                    <option value="">Aucun</option>
                    <option v-for="p in projets" :key="p.code" :value="p.code">{{ p.nom }}</option>
                  </select>
                </label>
                <label>Liste
                  <select v-model="brouillon.quand">
                    <option v-for="x in listes" :key="x.quand" :value="x.quand">{{ x.nom }}</option>
                  </select>
                </label>
              </div>
              <div v-if="brouillon.quand === 'dates'" class="ligne">
                <label>Du <input v-model="brouillon.debut" type="date" required /></label>
                <label>Au <input v-model="brouillon.fin" type="date" :min="brouillon.debut" required /></label>
              </div>
              <label v-else-if="brouillon.quand === 'semaine'">Semaine
                <select v-model="brouillon.semaine">
                  <option v-for="s in semaines(jour, 8, brouillon.semaine)" :key="s.debut" :value="s.debut">{{ s.libelle }}</option>
                </select>
              </label>
              <div class="ligne">
                <label>Durée (min) <input v-model="brouillon.duree_minutes" type="number" min="0" max="1440" inputmode="numeric" /></label>
                <label>Avant le <input v-model="brouillon.echeance" type="date" /></label>
              </div>
              <label>Notes <textarea v-model="brouillon.notes" rows="3" maxlength="4000" /></label>
              <div class="boutons-edition">
                <button type="button" class="lien-bouton danger" @click="supprimer(t)">Supprimer</button>
                <button type="button" class="bouton secondaire" @click="edition = null">Annuler</button>
                <button class="bouton principal" :disabled="enCours">Enregistrer</button>
              </div>
            </form>
            <template v-else>
              <input type="checkbox" class="coche" :checked="t.faite" :aria-label="`${t.faite ? 'Décocher' : 'Cocher'} « ${t.titre} »`" @change="cocher(t)" />
              <button class="corps" :aria-label="`Modifier « ${t.titre} »`" @click="modifier(t)">
                <span class="titre">{{ t.titre }}</span>
                <small v-if="meta(t)">{{ meta(t) }}</small>
              </button>
              <a v-if="t.lien && /^https?:/.test(t.lien)" class="lien-bouton ouvrir" :href="t.lien" target="_blank" rel="noopener">Ouvrir</a>
              <RouterLink v-else-if="t.lien" class="lien-bouton ouvrir" :to="t.lien">Ouvrir</RouterLink>
            </template>
          </li>
        </ul>
      </template>
    </section>

    <section v-if="recurrentes.length" class="carte liste-taches" aria-labelledby="titre-recurrentes">
      <div class="titre-ligne"><h2 id="titre-recurrentes">Récurrentes</h2></div>
      <ul>
        <li v-for="r in recurrentes" :key="r.id">
          <span class="corps statique">
            <span class="titre">{{ r.titre }}</span>
            <small>{{ [nomRythme(r.rythme, r.depuis), nomProjet(r.projet), duree(r.duree_minutes)].filter(Boolean).join(' · ') }}</small>
          </span>
          <button class="lien-bouton danger" :aria-label="`Arrêter « ${r.titre} »`" @click="arreter(r)">Arrêter</button>
        </li>
      </ul>
    </section>

    <section class="carte historique" aria-labelledby="titre-historique">
      <h2 id="titre-historique">Historique</h2>
      <p class="aide">Le bilan de chaque semaine se fait tout seul : ce que tu as fait, et ce qui n'a pas été fait à temps.</p>
      <p v-if="erreurBilan" class="erreur" role="alert">{{ erreurBilan }}</p>
      <details v-for="s in bilan || []" :key="s.lundi" :open="s.en_cours">
        <summary>
          <span class="titre">{{ s.en_cours ? 'Cette semaine' : `Semaine ${intervalle(s.lundi, s.dimanche)}` }}</span>
          <small>{{ resumeSemaine(s) }}</small>
        </summary>
        <p v-if="Object.keys(s.par_projet).length" class="aide">{{ repartition(s) }}</p>
        <h3 v-if="s.faites.length">Faites</h3>
        <ul v-if="s.faites.length" class="bilan">
          <li v-for="t in s.faites" :key="`f${t.id}`">{{ t.titre }}<small v-if="t.projet || t.duree_minutes"> · {{ [nomProjet(t.projet), duree(t.duree_minutes)].filter(Boolean).join(' · ') }}</small></li>
        </ul>
        <h3 v-if="s.non_faites.length">{{ s.en_cours ? 'À finir cette semaine' : 'Pas faites à temps' }}</h3>
        <ul v-if="s.non_faites.length" class="bilan pas-faites">
          <li v-for="t in s.non_faites" :key="`n${t.id}`">{{ t.titre }}<small> · {{ t.faite_le ? `faite le ${dateCourte(t.faite_le, annee)}` : `prévue jusqu'au ${dateCourte(t.fin, annee)}` }}</small></li>
        </ul>
        <p v-if="!s.faites.length && !s.non_faites.length" class="vide">Rien pour l'instant.</p>
      </details>
      <button v-if="bilan && nbSemaines < 104" class="lien-bouton" @click="plusDeSemaines">Semaines d'avant</button>
    </section>
  </div>
</template>

<style scoped>
.bandeau p { color: var(--blanc); font-size: 15px; }
.filtres { margin-top: var(--e4); overflow-x: auto; }
.filtres button { flex: 1 0 auto; padding: 0 var(--e3); }
.pile-relief { display: grid; gap: var(--e4); }
.carte + .carte { margin-top: 0; }
.ajout { display: grid; gap: var(--e3); }
.champ-ajout { display: flex; gap: var(--e2); }
.champ-ajout input { flex: 1; }
.champ-ajout .bouton { flex-shrink: 0; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }

.liste-taches { padding-bottom: var(--e2); }
.liste-taches h2 { margin: 0; }
.liste-taches .titre-ligne { margin-bottom: var(--e2); }
.liste-taches .titre-ligne .aide { display: flex; gap: var(--e3); align-items: center; }
.liste-taches ul { list-style: none; margin: 0; padding: 0; }
.liste-taches li { display: flex; align-items: center; gap: var(--e3); min-height: 52px; border-top: 1px solid var(--lin); }
.vide { padding: var(--e2) 0 var(--e3); color: var(--pierre); }
.coche { width: 22px; height: 22px; min-height: 0; padding: 0; margin: 11px; flex-shrink: 0; accent-color: var(--sarcelle); }
.corps {
  flex: 1; min-width: 0; display: grid; gap: 2px; padding: var(--e3) 0; text-align: left;
  background: none; border: 0; font: inherit; color: var(--encre); cursor: pointer;
}
.corps small { color: var(--pierre); font-size: 13px; }
.titre { font-weight: 500; overflow-wrap: anywhere; }
.faite .titre { text-decoration: line-through; color: var(--pierre); }
.ouvrir { flex-shrink: 0; }
.statique { cursor: default; }
.historique { display: grid; gap: var(--e2); }
.historique h2 { margin: 0; }
.historique details { border-top: 1px solid var(--lin); padding: var(--e2) 0; }
.historique summary { display: grid; gap: 2px; min-height: 44px; align-content: center; cursor: pointer; list-style-position: inside; }
.historique summary small { color: var(--pierre); font-size: 13px; padding-left: var(--e4); }
.historique h3 { font: 600 14px var(--police-texte); margin: var(--e3) 0 var(--e1); color: var(--pierre); }
.bilan { margin: 0; padding-left: var(--e5); display: grid; gap: var(--e1); }
.bilan small { color: var(--pierre); }
.edition { flex: 1; display: grid; gap: var(--e3); padding: var(--e3) 0; }
.boutons-edition { display: flex; gap: var(--e2); align-items: center; }
.boutons-edition .lien-bouton { margin-right: auto; }
</style>
