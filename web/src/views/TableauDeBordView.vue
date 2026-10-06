<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api } from '../api.js'
import { formatDate, formatEuros } from '../format.js'
import { ChevronRight } from 'lucide-vue-next'
import { dans, dateCourte, joursRestants, jourDuMois, libelleCourt, libelleSansAnnee, moisCourt, prochaine } from '../echeances.js'

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
const declarations = ref([])
const aujourdhuiDeclarations = ref('')

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

// Déclarations URSSAF dont la période touche l'année affichée, pour la vue Année.
// Une erreur ici ne bloque pas le tableau de bord : le bloc garde ses totaux, sans le détail.
async function chargerDeclarations() {
  const a = annee.value
  try {
    const r = await api.get(`/agenda?annee=${a}`)
    if (a !== annee.value) return
    aujourdhuiDeclarations.value = r?.aujourdhui || ''
    declarations.value = (r?.echeances || []).filter(
      (e) => e.type === 'urssaf' && e.periode_fin >= `${a}-01-01` && e.periode_debut <= `${a}-12-31`,
    )
  } catch {
    if (a === annee.value) declarations.value = []
  }
}

async function charger() {
  erreur.value = ''
  chargerDeclarations()
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
// Un mois payé à l'URSSAF montre le montant réel sur une ligne ; l'estimation ne reste que
// pour les mois pas encore payés.
const estimationVisible = computed(() => {
  const t = totaux.value
  return !t.urssaf_paye || t.cotisations + t.cfp + t.impot_vl > 0
})
const suffixeEstime = computed(() => (totaux.value?.urssaf_paye ? ', estimé (pas encore payé)' : ''))
// Vue Année : l'URSSAF se règle déclaration par déclaration, pas en une fois.
const urssafAnnee = computed(() => {
  const t = totaux.value
  const payees = declarations.value.filter((e) => e.paye > 0).length
  return {
    paye: t.urssaf_paye,
    reste: t.cotisations + t.cfp + t.impot_vl,
    payees,
    aVenir: declarations.value.length - payees,
  }
})
const libelleRythme = computed(() => {
  const n = declarations.value.length
  const rythme = profil.value?.periodicite === 'mensuelle' ? 'mensuelle' : 'trimestrielle'
  if (!n) return `Réglé déclaration par déclaration (${rythme}).`
  return n === 1 ? `Réglé en 1 déclaration ${rythme}.` : `Réglé en ${n} déclarations ${rythme}s.`
})
function etatDeclaration(e) {
  if (e.paye > 0) return 'Payé'
  if (e.faite) return 'Marquée faite, paiement pas saisi'
  if (aujourdhuiDeclarations.value && e.date < aujourdhuiDeclarations.value) return `Date limite passée (${dateCourte(e.date, annee.value)}), paiement pas saisi`
  return `À payer avant le ${dateCourte(e.date, annee.value)}`
}
const pluriel = (n, mot) => `${n} ${mot}${n > 1 ? 's' : ''}`
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
        <span>Revenu net estimé · {{ libellePeriode }} <InfoBulle k="revenuNet" /></span>
        <strong class="chiffres-tab">{{ formatEuros(totaux.net) }}</strong>
        <small v-if="partNet !== null">{{ partNet }} % de ton chiffre d'affaires</small>
      </div>
      <div v-if="totaux.ca > 0" class="repartition" role="img" :aria-label="`Sur ${formatEuros(totaux.ca)} encaissés, il te reste ${formatEuros(totaux.net)}`">
        <div :style="{ flex: Math.max(totaux.net, 0), background: 'var(--sarcelle)' }"></div>
        <div :style="{ flex: totaux.cotisations + totaux.cfp + totaux.impot_vl + totaux.urssaf_paye, background: 'var(--safran)' }"></div>
        <div :style="{ flex: totaux.depenses, background: 'var(--pierre)' }"></div>
      </div>
      <dl class="chiffres">
        <div><dt>Chiffre d'affaires encaissé <InfoBulle k="caEncaisse" /></dt><dd><strong>{{ formatEuros(totaux.ca) }}</strong></dd></div>
        <div v-if="vue === 'annee'" class="bloc-urssaf">
          <dt>
            <span class="pastille" style="background: var(--safran)"></span>{{ profil?.versement_liberatoire ? "URSSAF et impôt de l'année" : "URSSAF de l'année" }} <InfoBulle k="urssafAnnee" />
            <small>{{ libelleRythme }}</small>
          </dt>
          <dd>− {{ formatEuros(urssafAnnee.paye + urssafAnnee.reste) }}</dd>
        </div>
        <template v-if="vue === 'annee'">
          <div class="sous-ligne">
            <dt>Déjà payé<template v-if="urssafAnnee.payees"> ({{ pluriel(urssafAnnee.payees, 'déclaration') }})</template></dt>
            <dd>{{ formatEuros(urssafAnnee.paye) }}</dd>
          </div>
          <div class="sous-ligne">
            <dt>Reste à payer, estimé<template v-if="urssafAnnee.aVenir"> ({{ pluriel(urssafAnnee.aVenir, 'déclaration') }})</template></dt>
            <dd>{{ formatEuros(urssafAnnee.reste) }}</dd>
          </div>
        </template>
        <template v-else>
          <div v-if="totaux.urssaf_paye > 0">
            <dt><span class="pastille" style="background: var(--safran)"></span>{{ profil?.versement_liberatoire ? 'URSSAF et impôt payés' : 'URSSAF payé' }} <InfoBulle k="urssafPaye" /></dt>
            <dd>− {{ formatEuros(totaux.urssaf_paye) }}</dd>
          </div>
          <template v-if="estimationVisible">
            <div><dt><span class="pastille" style="background: var(--safran)"></span>Cotisations sociales{{ suffixeEstime }} <InfoBulle k="cotisations" /></dt><dd>− {{ formatEuros(totaux.cotisations) }}</dd></div>
            <div><dt><span class="pastille" style="background: var(--safran)"></span>Formation professionnelle (CFP){{ suffixeEstime }} <InfoBulle k="cfp" /></dt><dd>− {{ formatEuros(totaux.cfp) }}</dd></div>
            <div v-if="impotLibelle"><dt><span class="pastille" style="background: var(--safran)"></span>{{ impotLibelle }}{{ suffixeEstime }} <InfoBulle k="versementLiberatoire" /></dt><dd>− {{ formatEuros(totaux.impot_vl) }}</dd></div>
          </template>
        </template>
        <div><dt><span class="pastille" style="background: var(--pierre)"></span>Dépenses <InfoBulle k="depenses" /></dt><dd>− {{ formatEuros(totaux.depenses) }}</dd></div>
      </dl>
      <details v-if="vue === 'annee' && declarations.length" class="detail-declarations">
        <summary>Voir le détail par déclaration</summary>
        <ul>
          <li v-for="e in declarations" :key="e.code">
            <div>
              <strong>{{ libelleCourt(e) }}</strong>
              <small>{{ etatDeclaration(e) }}</small>
            </div>
            <span class="chiffres-tab">{{ e.paye > 0 ? formatEuros(e.paye) : `${formatEuros(e.a_payer)}, estimé` }}</span>
          </li>
        </ul>
      </details>
      <p v-if="vue === 'annee' && !profil?.versement_liberatoire" class="aide">
        Revenu imposable de l'activité en {{ annee }} <InfoBulle k="revenuImposable" /> : <strong>{{ formatEuros(donnees.revenu_imposable) }}</strong>.
        C'est ton chiffre d'affaires après abattement. Il s'ajoute à tes autres revenus dans ta déclaration d'impôt.
      </p>
      <p v-if="donnees.fin_acre" class="aide">ACRE appliquée jusqu'au {{ formatDate(donnees.fin_acre) }}. <InfoBulle k="acre" /></p>
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
        <span>{{ p.libelle }} <InfoBulle :k="p.code.startsWith('tva_') ? 'seuilTVA' : 'plafondMicro'" /></span>
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
.bloc-urssaf dt { flex-wrap: wrap; row-gap: 2px; }
.bloc-urssaf dt small { flex-basis: 100%; color: var(--pierre); font-size: 13px; font-weight: 400; margin-left: 18px; }
.sous-ligne { padding-left: 18px; color: var(--pierre); font-size: 14px; }
.detail-declarations summary { cursor: pointer; font-weight: 600; min-height: 44px; display: flex; align-items: center; }
.detail-declarations ul { list-style: none; margin: 0; padding: 0; display: grid; }
.detail-declarations li { display: flex; justify-content: space-between; align-items: center; gap: var(--e3); padding: var(--e2) 0; border-top: 1px solid var(--lin); }
.detail-declarations li div { display: grid; gap: 2px; min-width: 0; }
.detail-declarations li small { color: var(--pierre); font-size: 13px; }
.detail-declarations li span { white-space: nowrap; font-size: 14px; }
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
