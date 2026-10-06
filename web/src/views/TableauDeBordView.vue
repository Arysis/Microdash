<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api } from '../api.js'
import { formatDate, formatEuros } from '../format.js'
import { ChevronRight } from 'lucide-vue-next'
import { dans, joursRestants, jourDuMois, libelleSansAnnee, moisCourt, prochaine } from '../echeances.js'

const maintenant = new Date()
const annee = ref(maintenant.getFullYear())
const vue = ref('annee')
const index = ref(0)
const donnees = ref(null)
const profil = ref(null)
const erreur = ref('')
const dernieres = ref([])
const echeance = ref(null)
const aujourdhuiAgenda = ref('')

// Prochaine échéance de l'agenda de l'année en cours ; une erreur ici ne bloque pas le tableau de bord.
async function chargerEcheance() {
  try {
    const a = await api.get('/agenda')
    aujourdhuiAgenda.value = a.aujourdhui
    echeance.value = prochaine(a.echeances, a.aujourdhui)
  } catch {
    echeance.value = null
  }
}

async function charger() {
  erreur.value = ''
  try {
    const [d, t] = await Promise.all([
      api.get(`/tableau-de-bord?annee=${annee.value}`),
      api.get(`/transactions?du=${annee.value}-01-01&au=${annee.value}-12-31`),
    ])
    donnees.value = d
    // L'API renvoie les saisies de la plus récente à la plus ancienne.
    dernieres.value = (t || []).slice(0, 3)
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
  // Requête interrompue (changement de page pendant le chargement) : rien à afficher.
  if (!profil.value) return
  vue.value = profil.value.periodicite === 'mensuelle' ? 'mois' : 'trimestre'
  index.value = vue.value === 'mois' ? maintenant.getMonth() : Math.floor(maintenant.getMonth() / 3)
  await Promise.all([charger(), chargerEcheance()])
})
watch(annee, charger)

const periodes = computed(() => {
  if (!donnees.value) return []
  return vue.value === 'mois' ? donnees.value.mois : vue.value === 'trimestre' ? donnees.value.trimestres : []
})
const totaux = computed(() => {
  if (!donnees.value) return null
  return vue.value === 'annee' ? donnees.value.total : periodes.value[index.value]
})
const libellePeriode = computed(() => {
  if (vue.value === 'annee') return `Année ${annee.value}`
  const p = periodes.value[index.value]
  return p ? `${p.libelle} ${annee.value}` : ''
})
// Part du chiffre d'affaires qui reste après cotisations et dépenses, en pourcentage entier.
const partNet = computed(() => (totaux.value?.ca > 0 ? Math.round((totaux.value.net / totaux.value.ca) * 100) : null))
// Plafonds à signaler en haut de l'écran : 80 % passés ou seuil dépassé.
const alertesPlafonds = computed(() => (donnees.value?.plafonds || []).filter((p) => p.niveau !== 'ok'))
function texteAlerte(p) {
  const a = donnees.value.annee
  const tva = p.code.startsWith('tva_')
  if (tva && p.majore && p.ca > p.majore) {
    return { titre: 'Seuil majoré de franchise de TVA dépassé', texte: `Au-delà de ${formatEuros(p.majore)}, la TVA est due dès le jour du dépassement. Vérifie ta situation sur impots.gouv.fr.` }
  }
  if (p.niveau === 'depasse') {
    return tva
      ? { titre: `Seuil de franchise de TVA dépassé en ${a}`, texte: `Tu factureras la TVA à partir du 1er janvier ${a + 1}.${p.majore ? ` Au-delà de ${formatEuros(p.majore)}, elle est due dès le jour du dépassement.` : ''} Vérifie ta situation sur impots.gouv.fr.` }
      : { titre: `Plafond micro-entreprise dépassé en ${a}`, texte: `Si cela se répète en ${a + 1}, tu sors du régime micro au 1er janvier ${a + 2}. Vérifie ta situation sur urssaf.fr.` }
  }
  return tva
    ? { titre: 'Tu approches du seuil de franchise de TVA', texte: `${Math.floor(p.ratio * 100)} % du seuil de ${formatEuros(p.plafond)}. Garde un œil sur tes prochaines recettes.` }
    : { titre: 'Tu approches du plafond micro-entreprise', texte: `${Math.floor(p.ratio * 100)} % du plafond de ${formatEuros(p.plafond)}. Garde un œil sur tes prochaines recettes.` }
}
const impotLibelle = computed(() => (profil.value?.versement_liberatoire ? 'Impôt (versement libératoire)' : null))

function changerVue(v) {
  vue.value = v
  index.value = v === 'mois' ? maintenant.getMonth() : Math.floor(maintenant.getMonth() / 3)
}
</script>

<template>
  <div class="bandeau">
    <div class="titre-ligne">
      <h1>Tableau de bord</h1>
      <select v-model="annee" aria-label="Année">
        <option v-for="a in [maintenant.getFullYear() + 1, maintenant.getFullYear(), maintenant.getFullYear() - 1, maintenant.getFullYear() - 2]" :key="a" :value="a">{{ a }}</option>
      </select>
    </div>
    <div class="bascule">
      <button :class="{ actif: vue === 'mois' }" @click="changerVue('mois')">Mois</button>
      <button :class="{ actif: vue === 'trimestre' }" @click="changerVue('trimestre')">Trimestre</button>
      <button :class="{ actif: vue === 'annee' }" @click="changerVue('annee')">Année</button>
    </div>
    <select v-if="vue !== 'annee'" v-model="index" aria-label="Période">
      <option v-for="(p, i) in periodes" :key="p.debut" :value="i">{{ p.libelle }} {{ annee }}</option>
    </select>
  </div>

  <section class="carte pile carte-net">
    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
    <div v-for="p in alertesPlafonds" :key="p.code" :class="['alerte-plafond', p.niveau]" role="status">
      <strong>{{ texteAlerte(p).titre }}</strong>
      <span>{{ texteAlerte(p).texte }}</span>
    </div>
    <p v-if="donnees && !donnees.bareme_exact" class="encart">
      Pas encore de barème {{ annee }} : les calculs utilisent les taux {{ donnees.bareme_annee }}.
    </p>

    <template v-if="totaux">
      <div class="net">
        <span>Revenu net estimé · {{ libellePeriode }}</span>
        <strong class="chiffres-tab">{{ formatEuros(totaux.net) }}</strong>
        <small v-if="partNet !== null">{{ partNet }} % de ton chiffre d'affaires</small>
      </div>
      <div v-if="totaux.ca > 0" class="repartition" role="img" :aria-label="`Sur ${formatEuros(totaux.ca)} encaissés, il te reste ${formatEuros(totaux.net)}`">
        <div :style="{ flex: Math.max(totaux.net, 0), background: 'var(--sarcelle)' }"></div>
        <div :style="{ flex: totaux.cotisations + totaux.cfp + totaux.impot_vl, background: 'var(--safran)' }"></div>
        <div :style="{ flex: totaux.depenses, background: 'var(--pierre)' }"></div>
      </div>
      <dl class="chiffres">
        <div><dt>Chiffre d'affaires encaissé</dt><dd><strong>{{ formatEuros(totaux.ca) }}</strong></dd></div>
        <div><dt><span class="pastille" style="background: var(--safran)"></span>Cotisations sociales</dt><dd>− {{ formatEuros(totaux.cotisations) }}</dd></div>
        <div><dt><span class="pastille" style="background: var(--safran)"></span>Formation professionnelle (CFP)</dt><dd>− {{ formatEuros(totaux.cfp) }}</dd></div>
        <div v-if="impotLibelle"><dt><span class="pastille" style="background: var(--safran)"></span>{{ impotLibelle }}</dt><dd>− {{ formatEuros(totaux.impot_vl) }}</dd></div>
        <div><dt><span class="pastille" style="background: var(--pierre)"></span>Dépenses</dt><dd>− {{ formatEuros(totaux.depenses) }}</dd></div>
      </dl>
      <p v-if="vue === 'annee' && !profil?.versement_liberatoire" class="aide">
        Revenu imposable de l'activité en {{ annee }} : <strong>{{ formatEuros(donnees.revenu_imposable) }}</strong>.
        C'est ton chiffre d'affaires après abattement. Il s'ajoute à tes autres revenus dans ta déclaration d'impôt.
      </p>
      <p v-if="donnees.fin_acre" class="aide">ACRE appliquée jusqu'au {{ formatDate(donnees.fin_acre) }}.</p>
    </template>
  </section>

  <RouterLink v-if="echeance" to="/agenda" class="carte carte-echeance">
    <span class="tuile" aria-hidden="true"><span>{{ moisCourt(echeance.date) }}</span><strong>{{ jourDuMois(echeance.date) }}</strong></span>
    <span class="texte">
      <span class="surtitre">Prochaine échéance · {{ dans(joursRestants(echeance.date, aujourdhuiAgenda)) }}</span>
      <span class="libelle">{{ libelleSansAnnee(echeance) }}</span>
      <span v-if="echeance.type === 'urssaf'" class="aide">Chiffre d'affaires à déclarer : {{ formatEuros(echeance.ca) }}</span>
      <span v-else-if="echeance.type === 'cfe'" class="aide">À confirmer pour ton cas.</span>
    </span>
    <ChevronRight :size="20" :stroke-width="1.75" aria-hidden="true" class="chevron" />
  </RouterLink>

  <section v-if="donnees" class="carte pile">
    <h2>Plafonds {{ annee }}</h2>
    <div v-for="p in donnees.plafonds" :key="p.code" class="jauge">
      <div class="titre-ligne">
        <span>{{ p.libelle }}</span>
        <small class="chiffres-tab">{{ Math.round(p.ratio * 100) }} %</small>
      </div>
      <div class="barre" :class="p.niveau">
        <div :style="{ width: Math.min(100, p.ratio * 100) + '%' }"></div>
      </div>
      <small class="chiffres-tab">{{ formatEuros(p.ca) }} sur {{ formatEuros(p.plafond) }}</small>
      <small v-if="p.niveau === 'attention'" class="attention"> · Tu as passé 80 % du seuil.</small>
      <small v-if="p.niveau === 'depasse'" class="depasse"> · Seuil dépassé<template v-if="p.majore">, seuil majoré : {{ formatEuros(p.majore) }}</template>.</small>
    </div>
    <p class="aide">Estimations faites à partir de tes saisies et du barème de l'année. Seule ta déclaration URSSAF fait foi.</p>
  </section>

  <section v-if="dernieres.length" class="carte">
    <div class="titre-ligne">
      <h2>Dernières saisies</h2>
      <RouterLink class="tout-voir" to="/saisies">Tout voir</RouterLink>
    </div>
    <ul class="liste">
      <li v-for="t in dernieres" :key="t.id">
        <div>
          <strong>{{ t.libelle || (t.type === 'recette' ? 'Recette' : t.poste) }}</strong>
          <small>{{ formatDate(t.date) }}<template v-if="t.type === 'depense' && t.libelle"> · {{ t.poste }}</template></small>
        </div>
        <span :class="['montant', t.type]">{{ t.type === 'depense' ? '−' : '+' }} {{ formatEuros(t.centimes) }}</span>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.net { display: grid; gap: var(--e1); }
.net span { color: var(--pierre); font-size: 14px; font-weight: 500; }
.net strong { font-size: 40px; line-height: 1.1; font-weight: 700; letter-spacing: -0.01em; }
.net small { color: var(--sarcelle); font-size: 13px; font-weight: 600; }
.carte-net .repartition { height: 12px; }
/* Dans la carte du revenu net, les lignes respirent sans séparateur, comme sur la maquette. */
.carte-net .chiffres div { border-bottom: 0; padding-bottom: 0; }
.alerte-plafond { display: grid; gap: var(--e1); border-radius: var(--rayon); padding: var(--e3) var(--e4); font-size: 15px; background: var(--safran-clair); }
.alerte-plafond strong { color: var(--safran-fonce); }
.alerte-plafond.depasse { background: var(--blanc); border: 1px solid var(--lin); border-left: 4px solid var(--brique); }
.alerte-plafond.depasse strong { color: var(--brique); }

.carte-echeance { display: flex; gap: var(--e4); align-items: center; padding: var(--e4) var(--e5); text-decoration: none; color: var(--encre); }
.carte-echeance:hover { color: var(--encre); }
.carte-echeance:hover .libelle { text-decoration: underline; }
.carte-echeance .texte { display: grid; gap: 2px; flex: 1; min-width: 0; }
.carte-echeance .surtitre { color: var(--safran-fonce); font-size: 13px; font-weight: 600; }
.carte-echeance .libelle { font-size: 15px; font-weight: 600; }
.carte-echeance .chevron { color: var(--pierre); flex-shrink: 0; }
.tuile {
  width: 56px; height: 60px; flex-shrink: 0; border-radius: var(--rayon); background: var(--safran-clair);
  display: flex; flex-direction: column; align-items: center; justify-content: center;
}
.tuile span { font-size: 12px; font-weight: 600; color: var(--safran-fonce); }
.tuile strong { font-size: 22px; line-height: 1; }
.tout-voir { font-weight: 600; text-decoration: none; min-height: 44px; display: inline-flex; align-items: center; }
</style>
