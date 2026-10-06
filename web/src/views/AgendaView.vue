<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api } from '../api.js'
import { formatEuros } from '../format.js'
import {
  dans, dateCourte, dateLongue, joursRestants, jourDuMois, libelleCourt, libelleSansAnnee,
  lienDeclaration, moisCourt, prochaine, raisonReport,
} from '../echeances.js'

const anneeEnCours = new Date().getFullYear()
const annee = ref(anneeEnCours)
const donnees = ref(null)
const profil = ref(null)
const erreur = ref('')
const enCours = ref('')

async function charger() {
  erreur.value = ''
  try {
    donnees.value = await api.get(`/agenda?annee=${annee.value}`)
  } catch (e) {
    erreur.value = e.message
  }
}

onMounted(async () => {
  try {
    profil.value = await api.get('/profil')
  } catch (e) {
    erreur.value = e.message
    return
  }
  await charger()
})
watch(annee, charger)

const echeances = computed(() => donnees.value?.echeances || [])
const aujourdhui = computed(() => donnees.value?.aujourdhui || '')
const suivante = computed(() => prochaine(echeances.value, aujourdhui.value))
const periodicite = computed(() => (profil.value?.periodicite === 'mensuelle' ? 'Déclaration mensuelle' : 'Déclaration trimestrielle'))

const restants = (e) => joursRestants(e.date, aujourdhui.value)
const passee = (e) => e.faite || (e.date && restants(e) < 0)
const majuscule = (s) => s.charAt(0).toUpperCase() + s.slice(1)

function detail(e) {
  if (e.type === 'cfe') return 'À confirmer pour ton cas.'
  if (e.type === 'revenus') return e.date ? 'Vérifie la date de ton département.' : 'Date selon ton département.'
  const n = restants(e)
  if (n < 0 || e.faite) return raisonReport(e)
  const parties = []
  if (n <= 60) parties.push(majuscule(dans(n)))
  parties.push(`${formatEuros(e.ca)} à déclarer`)
  if (n > 60) parties.push(`${formatEuros(e.a_payer)} de cotisations estimées`)
  return parties.join(' · ')
}

function sousTitre(e) {
  const raison = raisonReport(e)
  return raison ? `${dateLongue(e.date)}. ${raison}` : `${dateLongue(e.date)}.`
}

async function cocher(e, faite) {
  enCours.value = e.code
  erreur.value = ''
  const avant = e.faite
  e.faite = faite
  try {
    await api.put(`/agenda/${e.code}`, { faite })
  } catch (err) {
    e.faite = avant
    erreur.value = err.message
  } finally {
    enCours.value = ''
  }
}
</script>

<template>
  <div class="bandeau">
    <div class="titre-ligne">
      <h1>Agenda</h1>
      <select v-model="annee" aria-label="Année">
        <option v-for="a in [anneeEnCours + 1, anneeEnCours, anneeEnCours - 1, anneeEnCours - 2]" :key="a" :value="a">{{ a }}</option>
      </select>
    </div>
    <p v-if="profil">{{ periodicite }}. Coche une échéance quand c'est fait.</p>
  </div>

  <section v-if="suivante" class="carte pile prochaine" aria-labelledby="titre-prochaine">
    <span class="surtitre">Prochaine échéance · {{ dans(restants(suivante)) }}</span>
    <div class="entete-echeance">
      <span class="tuile" aria-hidden="true"><span>{{ moisCourt(suivante.date) }}</span><strong>{{ jourDuMois(suivante.date) }}</strong></span>
      <div>
        <h2 id="titre-prochaine">{{ libelleSansAnnee(suivante) }}</h2>
        <span class="aide">{{ sousTitre(suivante) }}</span>
      </div>
    </div>
    <dl v-if="suivante.type === 'urssaf'" class="montants">
      <div><dt>Chiffre d'affaires à déclarer</dt><dd><strong>{{ formatEuros(suivante.ca) }}</strong></dd></div>
      <div><dt>Cotisations estimées</dt><dd>{{ formatEuros(suivante.a_payer) }}</dd></div>
    </dl>
    <p v-if="suivante.note" class="aide">{{ suivante.note }}</p>
    <p v-if="suivante.type === 'urssaf' && !suivante.bareme_exact" class="aide">Estimation faite avec les taux d'une autre année.</p>
    <div class="actions">
      <a class="bouton principal" :href="lienDeclaration(suivante)" target="_blank" rel="noopener">
        {{ suivante.type === 'urssaf' ? "Déclarer sur l'URSSAF" : 'Aller sur impots.gouv.fr' }}
      </a>
      <label class="case fait">
        <input type="checkbox" :checked="suivante.faite" :disabled="enCours === suivante.code" @change="cocher(suivante, $event.target.checked)" />
        C'est fait
      </label>
    </div>
  </section>

  <section class="carte">
    <h2>Toutes les échéances {{ annee }}</h2>
    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
    <p v-else-if="donnees && !echeances.length" class="aide vide">Aucune échéance cette année-là : ton activité n'avait pas commencé.</p>
    <ul class="echeances">
      <li v-for="e in echeances" :key="e.code" :class="{ passee: passee(e), faite: e.faite, suivante: e === suivante }">
        <span class="date">{{ e.date ? dateCourte(e.date, annee) : 'mai-juin' }}</span>
        <span class="texte">
          <span class="libelle">{{ libelleCourt(e) }}</span>
          <small v-if="detail(e)">{{ detail(e) }}</small>
        </span>
        <label class="coche">
          <input type="checkbox" :checked="e.faite" :disabled="enCours === e.code" :aria-label="`${libelleCourt(e)} : c'est fait`" @change="cocher(e, $event.target.checked)" />
        </label>
      </li>
    </ul>
    <p class="aide">Dates tirées du calendrier publié par l'URSSAF. Montants estimés à partir de tes saisies : seule ta déclaration fait foi.</p>
  </section>
</template>

<style scoped>
.bandeau p { color: var(--blanc); font-size: 15px; }
.prochaine { border-top: 4px solid var(--safran); gap: var(--e3); }
.surtitre { color: var(--safran-fonce); font-size: 13px; font-weight: 600; }
.entete-echeance { display: flex; gap: var(--e4); align-items: center; }
.entete-echeance h2 { font-size: 18px; }
.entete-echeance > div { display: grid; gap: 2px; min-width: 0; }
.tuile {
  width: 56px; height: 60px; flex-shrink: 0; border-radius: var(--rayon); background: var(--safran-clair);
  display: flex; flex-direction: column; align-items: center; justify-content: center;
}
.tuile span { font-size: 12px; font-weight: 600; color: var(--safran-fonce); }
.tuile strong { font-size: 22px; line-height: 1; }
.montants { margin: 0; display: grid; gap: var(--e2); font-size: 15px; }
.montants div { display: flex; justify-content: space-between; gap: var(--e3); }
.montants dt { color: var(--pierre); }
.montants dd { margin: 0; font-variant-numeric: tabular-nums; white-space: nowrap; }
.actions { display: flex; flex-wrap: wrap; align-items: center; gap: var(--e3) var(--e5); }
label.fait { align-items: center; min-height: 44px; font-weight: 500; }
label.fait input { margin: 0; }

.carte h2 { margin-bottom: var(--e2); }
.vide { padding: var(--e3) 0; }
.echeances { list-style: none; margin: 0 0 var(--e3); padding: 0; }
.echeances li {
  display: grid; grid-template-columns: 64px minmax(0, 1fr) auto; gap: var(--e3); align-items: center;
  padding: var(--e3) 0; border-bottom: 1px solid var(--lin);
}
.echeances li:last-child { border-bottom: 0; }
.date { font-size: 13px; font-weight: 600; font-variant-numeric: tabular-nums; }
.texte { display: grid; }
.libelle { font-weight: 600; }
.texte small { color: var(--pierre); font-size: 13px; }
.passee { color: var(--pierre); }
.passee .libelle { font-weight: 500; }
.faite .libelle { text-decoration: line-through; }
.suivante .date { color: var(--safran-fonce); font-weight: 700; }
.coche { min-width: 44px; min-height: 44px; display: inline-flex; align-items: center; justify-content: center; }
.coche input { width: 20px; height: 20px; margin: 0; accent-color: var(--safran); }
</style>
