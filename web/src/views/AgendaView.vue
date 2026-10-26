<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { api } from '../api.js'
import { demanderConfirmation } from '../confirmation.js'
import { dateCourte, dateLongue } from '../echeances.js'
import { duree } from '../taches.js'

const router = useRouter()
const icone = { size: 20, 'stroke-width': 1.75, 'aria-hidden': 'true' }
const moisNoms = ['janvier', 'février', 'mars', 'avril', 'mai', 'juin', 'juillet', 'août', 'septembre', 'octobre', 'novembre', 'décembre']
const joursCourts = ['Lun', 'Mar', 'Mer', 'Jeu', 'Ven', 'Sam', 'Dim']

// Dates en AAAA-MM-JJ, calculées en UTC pour éviter les décalages d'heure d'été.
const lire = (iso) => new Date(`${iso}T00:00:00Z`)
const ecrire = (d) => d.toISOString().slice(0, 10)
const ajouter = (iso, n) => {
  const d = lire(iso)
  d.setUTCDate(d.getUTCDate() + n)
  return ecrire(d)
}
const lundi = (iso) => ajouter(iso, -((lire(iso).getUTCDay() + 6) % 7))
const debutMois = (iso) => `${iso.slice(0, 8)}01`
const finMois = (iso) => {
  const d = lire(debutMois(iso))
  d.setUTCMonth(d.getUTCMonth() + 1, 0)
  return ecrire(d)
}
const localAujourdhui = () => {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

const vue = ref('semaine')
const aujourdhui = ref(localAujourdhui())
const ancre = ref(aujourdhui.value)
const jourChoisi = ref(aujourdhui.value)
const donnees = ref(null)
const erreur = ref('')

const periode = computed(() => {
  if (vue.value === 'semaine') {
    const du = lundi(ancre.value)
    return { du, au: ajouter(du, 6) }
  }
  const du = lundi(debutMois(ancre.value))
  return { du, au: ajouter(lundi(finMois(ancre.value)), 6) }
})

const jours = computed(() => {
  const res = []
  for (let j = periode.value.du; j <= periode.value.au; j = ajouter(j, 1)) res.push(j)
  return res
})

const titrePeriode = computed(() => {
  const d = lire(ancre.value)
  if (vue.value === 'mois') return `${moisNoms[d.getUTCMonth()]} ${d.getUTCFullYear()}`
  const du = lire(periode.value.du)
  const au = lire(periode.value.au)
  const debut = du.getUTCMonth() === au.getUTCMonth() ? du.getUTCDate() : `${du.getUTCDate()} ${moisNoms[du.getUTCMonth()]}`
  return `Du ${debut} au ${au.getUTCDate()} ${moisNoms[au.getUTCMonth()]}`
})

async function charger() {
  erreur.value = ''
  try {
    const { du, au } = periode.value
    donnees.value = await api.get(`/calendrier?du=${du}&au=${au}`)
    aujourdhui.value = donnees.value.aujourdhui
  } catch (e) {
    erreur.value = e.message
  }
}
watch(periode, charger)
onMounted(() => {
  charger()
  chargerFlux()
})

function decaler(sens) {
  if (vue.value === 'semaine') ancre.value = ajouter(ancre.value, 7 * sens)
  else {
    const d = lire(debutMois(ancre.value))
    d.setUTCMonth(d.getUTCMonth() + sens)
    ancre.value = ecrire(d)
  }
}
function revenir() {
  ancre.value = aujourdhui.value
  jourChoisi.value = aujourdhui.value
}

const parJour = computed(() => {
  const res = {}
  for (const e of donnees.value?.evenements || []) (res[e.date] ||= []).push(e)
  return res
})
const dansLeMois = (j) => j.slice(0, 7) === ancre.value.slice(0, 7)

const types = {
  evenement: { nom: 'Rendez-vous', couleur: 'var(--sarcelle)' },
  tache: { nom: 'Tâche', couleur: 'var(--bleu)' },
  echeance: { nom: 'Échéance', couleur: 'var(--safran)' },
}

function libelleJour(j) {
  const d = lire(j)
  const nom = joursCourts[(d.getUTCDay() + 6) % 7]
  return `${nom} ${dateCourte(j, d.getUTCFullYear())}`
}

function horaire(e) {
  if (!e.heure) return 'Journée'
  return e.duree ? `${e.heure} · ${duree(e.duree)}` : e.heure
}

// --- Ajouter ou modifier un rendez-vous ---
const formulaire = ref(null) // null, ou l'identifiant modifié (0 : nouveau)
const saisie = reactive({ titre: '', jour: '', heure: '', duree_minutes: 60, notes: '' })
const enCours = ref(false)

function nouveau(jour) {
  Object.assign(saisie, { titre: '', jour: jour || jourChoisi.value || aujourdhui.value, heure: '', duree_minutes: 60, notes: '' })
  formulaire.value = 0
}

function ouvrir(e) {
  if (e.type === 'evenement') {
    Object.assign(saisie, { titre: e.titre, jour: e.date, heure: e.heure || '', duree_minutes: e.duree || 60, notes: e.notes || '' })
    formulaire.value = Number(e.ref)
  } else if (e.lien) router.push(e.lien)
}

async function enregistrer() {
  enCours.value = true
  erreur.value = ''
  const corps = { ...saisie, duree_minutes: saisie.heure ? Number(saisie.duree_minutes) || 0 : 0 }
  try {
    if (formulaire.value) await api.put(`/evenements/${formulaire.value}`, corps)
    else await api.post('/evenements', corps)
    formulaire.value = null
    await charger()
  } catch (e) {
    erreur.value = e.message
  } finally {
    enCours.value = false
  }
}

async function supprimer() {
  const ok = await demanderConfirmation({ titre: 'Supprimer le rendez-vous ?', message: `« ${saisie.titre} » sera effacé.`, action: 'Supprimer' })
  if (!ok) return
  try {
    await api.del(`/evenements/${formulaire.value}`)
    formulaire.value = null
    await charger()
  } catch (e) {
    erreur.value = e.message
  }
}

// --- Flux iCal pour Home Assistant ---
const flux = ref(null)
const lienFlux = ref('')
const copie = ref(false)
const erreurFlux = ref('')

async function chargerFlux() {
  try {
    flux.value = await api.get('/calendrier/flux')
  } catch (e) {
    erreurFlux.value = e.message
  }
}

async function creerLien() {
  if (flux.value?.actif) {
    const ok = await demanderConfirmation({
      titre: 'Changer le lien ?',
      message: "L'ancien lien cessera de marcher : il faudra coller le nouveau dans Home Assistant.",
      action: 'Changer le lien',
    })
    if (!ok) return
  }
  erreurFlux.value = ''
  try {
    const r = await api.post('/calendrier/flux')
    flux.value = { actif: true }
    lienFlux.value = `${location.origin}${r.chemin}`
    copie.value = false
  } catch (e) {
    erreurFlux.value = e.message
  }
}

async function copier() {
  try {
    await navigator.clipboard.writeText(lienFlux.value)
    copie.value = true
  } catch {
    erreurFlux.value = 'Copie impossible : sélectionne le lien et copie-le à la main.'
  }
}

async function couperLien() {
  const ok = await demanderConfirmation({ titre: 'Couper le lien ?', message: 'Home Assistant ne verra plus ton agenda.', action: 'Couper le lien' })
  if (!ok) return
  try {
    await api.del('/calendrier/flux')
    flux.value = { actif: false }
    lienFlux.value = ''
  } catch (e) {
    erreurFlux.value = e.message
  }
}
</script>

<template>
  <div class="bandeau">
    <div class="titre-ligne">
      <h1>Agenda</h1>
      <button class="bouton secondaire ajouter" @click="nouveau()">Ajouter</button>
    </div>
    <div class="bascule" role="group" aria-label="Vue">
      <button :class="{ actif: vue === 'semaine' }" :aria-pressed="vue === 'semaine'" @click="vue = 'semaine'">Semaine</button>
      <button :class="{ actif: vue === 'mois' }" :aria-pressed="vue === 'mois'" @click="vue = 'mois'">Mois</button>
    </div>
    <div class="navigation">
      <button class="fleche" aria-label="Période précédente" @click="decaler(-1)"><ChevronLeft v-bind="icone" /></button>
      <span>{{ titrePeriode }}</span>
      <button class="fleche" aria-label="Période suivante" @click="decaler(1)"><ChevronRight v-bind="icone" /></button>
      <button v-if="!jours.includes(aujourdhui)" class="retour" @click="revenir">Aujourd'hui</button>
    </div>
  </div>

  <div class="pile-relief">
    <form v-if="formulaire !== null" class="carte" @submit.prevent="enregistrer">
      <h2>{{ formulaire ? 'Modifier le rendez-vous' : 'Nouveau rendez-vous' }}</h2>
      <label>Titre <input v-model="saisie.titre" maxlength="200" placeholder="Ex. : Appel avec le comptable" required /></label>
      <div class="ligne">
        <label>Jour <input v-model="saisie.jour" type="date" required /></label>
        <label>Heure (facultatif) <input v-model="saisie.heure" type="time" /></label>
      </div>
      <label v-if="saisie.heure">Durée (min) <input v-model="saisie.duree_minutes" type="number" min="1" max="1440" inputmode="numeric" /></label>
      <label>Notes <textarea v-model="saisie.notes" rows="2" maxlength="4000" /></label>
      <div class="boutons-form">
        <button v-if="formulaire" type="button" class="lien-bouton danger" @click="supprimer">Supprimer</button>
        <button type="button" class="bouton secondaire" @click="formulaire = null">Annuler</button>
        <button class="bouton principal" :disabled="enCours">Enregistrer</button>
      </div>
    </form>

    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
    <p v-if="donnees?.indisponibles?.length" class="aide">Indisponible pour le moment : {{ donnees.indisponibles.join(', ') }}.</p>

    <section v-if="vue === 'semaine'" class="carte semaine" aria-label="Semaine">
      <div v-for="j in jours" :key="j" :class="['jour', { aujourdhui: j === aujourdhui }]">
        <h2 class="nom-jour">{{ libelleJour(j) }}</h2>
        <ul v-if="parJour[j]">
          <li v-for="e in parJour[j]" :key="`${e.source}/${e.ref}`">
            <button :class="['entree', { fait: e.fait }]" @click="ouvrir(e)">
              <span class="pastille" :style="{ background: types[e.type]?.couleur }" aria-hidden="true" />
              <span class="horaire">{{ horaire(e) }}</span>
              <span class="titre">{{ e.titre }}<span class="sr-only"> ({{ types[e.type]?.nom }}{{ e.fait ? ', fait' : '' }})</span></span>
            </button>
          </li>
        </ul>
        <button v-else class="libre" :aria-label="`Ajouter un rendez-vous le ${dateLongue(j)}`" @click="nouveau(j)">Libre</button>
      </div>
    </section>

    <template v-else>
      <section class="carte mois" aria-label="Mois">
        <div class="grille">
          <span v-for="n in joursCourts" :key="n" class="entete-jour" aria-hidden="true">{{ n.slice(0, 1) }}</span>
          <button
            v-for="j in jours" :key="j"
            :class="['case-jour', { hors: !dansLeMois(j), aujourdhui: j === aujourdhui, choisi: j === jourChoisi }]"
            :aria-label="`${dateLongue(j)}, ${(parJour[j] || []).length} élément(s)`"
            :aria-pressed="j === jourChoisi"
            @click="jourChoisi = j"
          >
            <span>{{ Number(j.slice(8)) }}</span>
            <span class="points" aria-hidden="true">
              <span v-for="e in (parJour[j] || []).slice(0, 3)" :key="`${e.source}/${e.ref}`" class="point" :style="{ background: types[e.type]?.couleur }" />
            </span>
          </button>
        </div>
      </section>
      <section class="carte" aria-labelledby="titre-jour">
        <div class="titre-ligne">
          <h2 id="titre-jour">{{ dateLongue(jourChoisi) }}</h2>
          <button class="lien-bouton" @click="nouveau(jourChoisi)">Ajouter</button>
        </div>
        <p v-if="!parJour[jourChoisi]" class="aide">Rien ce jour-là.</p>
        <ul v-else class="jour">
          <li v-for="e in parJour[jourChoisi]" :key="`${e.source}/${e.ref}`">
            <button :class="['entree', { fait: e.fait }]" @click="ouvrir(e)">
              <span class="pastille" :style="{ background: types[e.type]?.couleur }" aria-hidden="true" />
              <span class="horaire">{{ horaire(e) }}</span>
              <span class="titre">{{ e.titre }}</span>
            </button>
          </li>
        </ul>
      </section>
    </template>

    <div class="legende" aria-label="Légende">
      <span v-for="(t, code) in types" :key="code"><span class="pastille" :style="{ background: t.couleur }" />{{ t.nom }}</span>
    </div>

    <section class="carte ha" aria-labelledby="titre-ha">
      <h2 id="titre-ha">Dans Home Assistant</h2>
      <p class="aide">
        Un lien secret donne ton agenda (rendez-vous, tâches datées, échéances) au format iCal.
        Dans Home Assistant : Paramètres, Appareils et services, Ajouter une intégration, « Remote Calendar », puis colle le lien.
        Il marche aussi avec le calendrier de ton téléphone.
      </p>
      <template v-if="lienFlux">
        <label>Ton lien (montré une seule fois)
          <input :value="lienFlux" readonly @focus="$event.target.select()" />
        </label>
        <div class="boutons-form">
          <button class="bouton principal" @click="copier">{{ copie ? 'Lien copié' : 'Copier le lien' }}</button>
        </div>
      </template>
      <template v-else-if="flux?.actif">
        <p>Le lien est actif. Il n'est montré qu'une fois : si tu l'as perdu, change-le.</p>
        <div class="boutons-form">
          <button class="lien-bouton danger" @click="couperLien">Couper le lien</button>
          <button class="bouton secondaire" @click="creerLien">Changer le lien</button>
        </div>
      </template>
      <div v-else-if="flux" class="boutons-form">
        <button class="bouton principal" @click="creerLien">Créer le lien</button>
      </div>
      <p v-if="erreurFlux" class="erreur" role="alert">{{ erreurFlux }}</p>
    </section>
  </div>
</template>

<style scoped>
.bandeau .titre-ligne { margin-bottom: var(--e3); }
.bandeau .ajouter { background: var(--blanc); color: var(--sarcelle); border-color: var(--blanc); min-height: 40px; }
.navigation { display: flex; align-items: center; gap: var(--e2); margin-top: var(--e3); color: var(--blanc); font-weight: 600; }
.navigation span { flex: 1; text-align: center; }
.fleche, .retour {
  min-width: 44px; min-height: 44px; display: inline-flex; align-items: center; justify-content: center;
  background: var(--sur-sarcelle); color: var(--blanc); border: 0; border-radius: var(--rayon); cursor: pointer;
  font: 500 14px var(--police-texte);
}
.retour { padding: 0 var(--e3); }
.pile-relief { display: grid; gap: var(--e4); }
.carte + .carte { margin-top: 0; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }

.semaine { padding-top: var(--e2); padding-bottom: var(--e2); }
.jour { list-style: none; margin: 0; padding: 0; }
.semaine .jour { display: grid; grid-template-columns: 76px 1fr; gap: var(--e2); padding: var(--e2) 0; border-bottom: 1px solid var(--lin); }
.semaine .jour:last-child { border-bottom: 0; }
.semaine ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 2px; }
.nom-jour { font: 600 14px var(--police-texte); color: var(--pierre); margin: 0; padding-top: 12px; }
.aujourdhui .nom-jour { color: var(--safran); }
.entree {
  width: 100%; min-height: 44px; display: grid; grid-template-columns: 10px auto 1fr; align-items: center; gap: var(--e2);
  background: none; border: 0; padding: var(--e1) 0; text-align: left; font: inherit; color: var(--encre); cursor: pointer;
}
.entree .horaire { font-size: 13px; color: var(--pierre); font-variant-numeric: tabular-nums; white-space: nowrap; }
.entree .titre { font-weight: 500; overflow-wrap: anywhere; }
.entree.fait .titre { text-decoration: line-through; color: var(--pierre); }
.libre { min-height: 44px; background: none; border: 0; text-align: left; color: var(--pierre); font: 400 14px var(--police-texte); cursor: pointer; }
.libre:hover { color: var(--safran); }

.grille { display: grid; grid-template-columns: repeat(7, 1fr); gap: 2px; }
.entete-jour { text-align: center; font-size: 12px; color: var(--pierre); font-weight: 600; padding-bottom: var(--e1); }
.case-jour {
  min-height: 48px; display: grid; justify-items: center; align-content: start; gap: 2px; padding-top: var(--e1);
  background: none; border: 1px solid transparent; border-radius: var(--rayon); font: 500 15px var(--police-texte); color: var(--encre); cursor: pointer;
}
.case-jour.hors { color: var(--pierre); opacity: 0.5; }
.case-jour.aujourdhui span:first-child { color: var(--safran); font-weight: 700; }
.case-jour.choisi { border-color: var(--sarcelle); background: var(--sarcelle-clair); }
.points { display: flex; gap: 2px; min-height: 6px; }
.point { width: 6px; height: 6px; border-radius: 50%; }

.legende { display: flex; flex-wrap: wrap; gap: var(--e4); font-size: 13px; color: var(--pierre); padding: 0 var(--e1); }
.legende > span { display: inline-flex; align-items: center; gap: var(--e1); }
.ha { display: grid; gap: var(--e3); }
.ha h2 { margin: 0; }
.boutons-form { display: flex; gap: var(--e2); align-items: center; justify-content: flex-end; }
.boutons-form .lien-bouton { margin-right: auto; }
</style>
