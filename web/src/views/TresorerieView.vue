<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Pencil, RefreshCw, Trash2 } from 'lucide-vue-next'
import { api } from '../api.js'
import { demanderConfirmation } from '../confirmation.js'
import { aujourdhui, formatDate, formatEuros, posteURSSAF, postesDepense, versCentimes } from '../format.js'
import GrapheTresorerie from '../components/GrapheTresorerie.vue'

const donnees = ref(null)
const erreur = ref('')
const chargement = ref(true)

async function charger() {
  erreur.value = ''
  try {
    donnees.value = await api.get('/tresorerie')
  } catch (e) {
    erreur.value = e.message
  } finally {
    chargement.value = false
  }
}
onMounted(charger)

const tr = computed(() => donnees.value?.tresorerie)

// Vue du graphe, gardée sur cet appareil.
const lireVue = () => {
  try {
    return localStorage.getItem('microdash.tresorerie.vue') === 'cumul' ? 'cumul' : 'mois'
  } catch {
    return 'mois'
  }
}
const vueGraphe = ref(lireVue())
function choisirVue(v) {
  vueGraphe.value = v
  try {
    localStorage.setItem('microdash.tresorerie.vue', v)
  } catch {
    // stockage indisponible : la vue reste choisie jusqu'au rechargement
  }
}
const mois = computed(() => tr.value?.mois || [])
const prevus = computed(() => mois.value.filter((m) => m.prevision))
const dernierPrevu = computed(() => prevus.value[prevus.value.length - 1])
const stripe = computed(() => donnees.value?.stripe)
const signe = (c) => `${c < 0 ? '−' : '+'} ${formatEuros(Math.abs(c))}`
const majuscule = (s) => (s ? s.charAt(0).toUpperCase() + s.slice(1) : '')
const libelleMois = (cle) => new Date(`${cle}-01T00:00:00`).toLocaleDateString('fr-FR', { month: 'long', year: 'numeric' })

const methode = computed(() => {
  if (!tr.value) return ''
  const r = formatEuros(tr.value.recettes_prevues)
  if (tr.value.methode === 'mrr') return `recettes au revenu mensuel récurrent Stripe, ${r} par mois`
  const n = tr.value.mois_moyenne
  if (n === 0) return "pas encore de recettes prévues, faute d'un mois complet d'activité"
  return n === 1
    ? `recettes égales à ton dernier mois complet, ${r} par mois`
    : `recettes à la moyenne de tes ${n} derniers mois complets, ${r} par mois`
})
const tauxCharges = computed(() => (tr.value ? Math.round(tr.value.taux_charges * 1000) / 10 : 0))

// --- Solde du compte ---
const solde = reactive({ ouvert: false, montant: '', au: aujourdhui(), erreur: '' })
function ouvrirSolde() {
  const s = donnees.value?.solde
  Object.assign(solde, {
    ouvert: true,
    montant: s ? (s.centimes / 100).toFixed(2).replace('.', ',') : '',
    au: s?.au || aujourdhui(),
    erreur: '',
  })
}
async function enregistrerSolde(effacer = false) {
  solde.erreur = ''
  let corps = { centimes: null }
  if (!effacer) {
    const centimes = versCentimes(solde.montant)
    if (Number.isNaN(centimes) || String(solde.montant).trim() === '') {
      solde.erreur = 'Montant invalide.'
      return
    }
    corps = { centimes, au: solde.au }
  }
  try {
    donnees.value = await api.put('/tresorerie/solde', corps)
    solde.ouvert = false
  } catch (e) {
    solde.erreur = e.message
  }
}

// --- Stripe ---
const sourceErreur = ref('')
async function choisirSource(mrr) {
  if (stripe.value.prevision_mrr === mrr) return
  sourceErreur.value = ''
  try {
    donnees.value = await api.put('/stripe/prevision', { mrr })
  } catch (e) {
    sourceErreur.value = e.message
  }
}
const cleStripe = ref('')
const stripeErreur = ref('')
const stripeAction = ref('')
const devises = (d) => new Intl.NumberFormat('fr-FR', { style: 'currency', currency: d.toUpperCase() })
const autresDevises = computed(() =>
  Object.entries(stripe.value?.mrr?.par_devise || {}).filter(([d]) => d !== 'eur'),
)
const moisStripe = computed(() => (stripe.value?.mois || []).slice(-3).reverse())
const luLe = computed(() => {
  const v = stripe.value?.lu_le
  if (!v) return ''
  const d = new Date(v)
  return `${d.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })} à ${d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}`
})

async function connecterStripe() {
  stripeErreur.value = ''
  stripeAction.value = 'connexion'
  try {
    await api.put('/stripe', { cle: cleStripe.value.trim() })
    cleStripe.value = ''
    await charger()
  } catch (e) {
    stripeErreur.value = e.message
  } finally {
    stripeAction.value = ''
  }
}
async function actualiserStripe() {
  stripeErreur.value = ''
  stripeAction.value = 'actualiser'
  try {
    await api.post('/stripe/actualiser')
    await charger()
  } catch (e) {
    stripeErreur.value = e.message
  } finally {
    stripeAction.value = ''
  }
}
async function deconnecterStripe() {
  const ok = await demanderConfirmation({
    titre: 'Déconnecter Stripe ?',
    message: "Microdash efface ta clé et les chiffres lus. Pense aussi à supprimer la clé dans Stripe si tu ne t'en sers plus.",
    action: 'Déconnecter',
  })
  if (!ok) return
  try {
    await api.del('/stripe')
    await charger()
  } catch (e) {
    stripeErreur.value = e.message
  }
}

// --- Dépenses récurrentes ---
// Un paiement URSSAF varie et se rattache à une déclaration : il se saisit à la main.
const postesRecurrents = postesDepense.filter((p) => p !== posteURSSAF)
const recurrentes = computed(() => donnees.value?.recurrentes || [])
const vide = () => ({ libelle: '', montant: '', frequence: 'mensuelle', poste: 'Logiciels et abonnements', debut: aujourdhui(), fin: '' })
const rec = reactive(vide())
const recEdition = ref(null)
const recErreur = ref('')
const recMessage = ref('')
const formRec = ref(null)

function rythme(r) {
  const d = new Date(`${r.debut}T00:00:00`)
  return r.frequence === 'annuelle'
    ? `chaque année le ${d.toLocaleDateString('fr-FR', { day: 'numeric', month: 'long' })}`
    : `chaque mois le ${d.getDate()}`
}
function editerRec(r) {
  recEdition.value = r.id
  recMessage.value = ''
  Object.assign(rec, {
    libelle: r.libelle,
    montant: (r.centimes / 100).toFixed(2).replace('.', ','),
    frequence: r.frequence,
    poste: r.poste,
    debut: r.debut,
    fin: r.fin || '',
  })
  formRec.value?.scrollIntoView({ behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'start' })
}
function annulerRec() {
  Object.assign(rec, vide())
  recEdition.value = null
  recErreur.value = ''
}
async function enregistrerRec() {
  recErreur.value = ''
  recMessage.value = ''
  const centimes = versCentimes(rec.montant)
  if (!(centimes > 0)) {
    recErreur.value = 'Montant invalide.'
    return
  }
  const corps = { libelle: rec.libelle, poste: rec.poste, centimes, frequence: rec.frequence, debut: rec.debut, fin: rec.fin }
  try {
    const res = recEdition.value
      ? await api.put(`/depenses-recurrentes/${recEdition.value}`, corps)
      : await api.post('/depenses-recurrentes', corps)
    const n = res.saisies_creees
    recMessage.value = n === 0 ? 'Enregistré.' : `Enregistré. ${n} saisie${n > 1 ? 's' : ''} ajoutée${n > 1 ? 's' : ''} à tes dépenses.`
    annulerRec()
    await charger()
  } catch (e) {
    recErreur.value = e.message
  }
}
async function supprimerRec(r) {
  const ok = await demanderConfirmation({
    titre: 'Arrêter cette dépense récurrente ?',
    message: `${r.libelle}, ${formatEuros(r.centimes)}. Plus aucune saisie ne sera créée ; celles déjà créées restent dans tes saisies.`,
    action: 'Arrêter',
  })
  if (!ok) return
  try {
    await api.del(`/depenses-recurrentes/${r.id}`)
    if (recEdition.value === r.id) annulerRec()
    await charger()
  } catch (e) {
    recErreur.value = e.message
  }
}

// Relit les données quand l'onglet redevient visible (une saisie a pu être faite ailleurs).
const auRetour = () => document.visibilityState === 'visible' && donnees.value && charger()
onMounted(() => document.addEventListener('visibilitychange', auRetour))
onBeforeUnmount(() => document.removeEventListener('visibilitychange', auRetour))
</script>

<template>
  <div class="bandeau">
    <h1>Trésorerie</h1>
    <p>L'argent qui entre et qui sort, mois par mois, et ce qui s'annonce.</p>
  </div>

  <section class="carte pile" aria-labelledby="titre-prevision">
    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
    <p v-else-if="chargement" class="aide">Chargement…</p>
    <template v-if="tr">
      <div class="hero">
        <template v-if="tr.solde_prevu !== undefined && tr.solde_prevu !== null">
          <span id="titre-prevision">Solde prévu fin {{ dernierPrevu?.libelle }}</span>
          <strong class="chiffres-tab">{{ formatEuros(tr.solde_prevu) }}</strong>
          <small :class="tr.flux_prevu < 0 ? 'baisse' : 'hausse'">{{ signe(tr.flux_prevu) }} sur les 6 prochains mois</small>
        </template>
        <template v-else>
          <span id="titre-prevision">Prévu sur les 6 prochains mois</span>
          <strong class="chiffres-tab">{{ signe(tr.flux_prevu) }}</strong>
          <small class="neutre">Entrées moins sorties, cotisations comprises.</small>
        </template>
      </div>

      <div v-if="!solde.ouvert" class="ligne-solde">
        <span v-if="donnees.solde" class="aide">
          Solde du compte noté le {{ formatDate(donnees.solde.au) }} : {{ formatEuros(donnees.solde.centimes) }}.
        </span>
        <span v-else class="aide">Note le solde de ton compte pro pour voir ce qu'il restera chaque mois.</span>
        <button class="lien-bouton" @click="ouvrirSolde">{{ donnees.solde ? 'Modifier' : 'Noter mon solde' }}</button>
      </div>
      <form v-else class="form-solde" @submit.prevent="enregistrerSolde()">
        <div class="ligne">
          <label>Solde du compte (€) <input v-model="solde.montant" inputmode="decimal" placeholder="0,00" required /></label>
          <label>Au <input v-model="solde.au" type="date" :max="aujourdhui()" required /></label>
        </div>
        <p class="aide">Microdash y ajoute ce que tu saisis après cette date, puis les entrées et sorties de chaque mois.</p>
        <p v-if="solde.erreur" class="erreur" role="alert">{{ solde.erreur }}</p>
        <div class="ligne">
          <button class="bouton principal">Enregistrer</button>
          <button type="button" class="bouton secondaire" @click="solde.ouvert = false">Annuler</button>
          <button v-if="donnees.solde" type="button" class="bouton secondaire danger" @click="enregistrerSolde(true)">Effacer</button>
        </div>
      </form>

      <div class="bascule" role="group" aria-label="Vue du graphe">
        <button type="button" :class="{ actif: vueGraphe === 'mois' }" :aria-pressed="vueGraphe === 'mois'" @click="choisirVue('mois')">Par mois</button>
        <button type="button" :class="{ actif: vueGraphe === 'cumul' }" :aria-pressed="vueGraphe === 'cumul'" @click="choisirVue('cumul')">Cumulée</button>
      </div>
      <GrapheTresorerie :mois="mois" :vue="vueGraphe" />

      <div v-if="stripe?.connecte && stripe.mrr" class="source">
        <span id="titre-source">Recettes prévues</span>
        <div class="bascule" role="group" aria-labelledby="titre-source">
          <button type="button" :class="{ actif: stripe.prevision_mrr }" :aria-pressed="stripe.prevision_mrr" @click="choisirSource(true)">
            MRR Stripe · {{ formatEuros(stripe.mrr.par_devise.eur || 0) }}
          </button>
          <button type="button" :class="{ actif: !stripe.prevision_mrr }" :aria-pressed="!stripe.prevision_mrr" @click="choisirSource(false)">
            Mes saisies · {{ formatEuros(tr.moyenne) }}
          </button>
        </div>
        <p v-if="sourceErreur" class="erreur" role="alert">{{ sourceErreur }}</p>
      </div>

      <p class="aide">
        Prévision : {{ methode }} ; dépenses récurrentes à leurs dates ; paiements URSSAF au mois de leur date limite
        ({{ tauxCharges.toLocaleString('fr-FR') }} % des recettes prévues). Le mois en cours compte tes saisies, plus les dépenses récurrentes et paiements URSSAF d'ici sa fin.
      </p>
    </template>
  </section>

  <section v-if="stripe" class="carte pile" aria-labelledby="titre-stripe">
    <div class="titre-ligne">
      <h2 id="titre-stripe">Stripe</h2>
      <span v-if="stripe.connecte && stripe.mode === 'test'" class="badge">Mode test</span>
    </div>

    <p v-if="!stripe.disponible" class="aide">
      La connexion Stripe n'est pas encore activée sur ce serveur. La personne qui l'héberge doit renseigner CLE_CHIFFREMENT.
    </p>

    <template v-else-if="!stripe.connecte">
      <p>Connecte ton compte Stripe pour voir le revenu mensuel récurrent de tes abonnements et t'en servir dans la prévision.</p>
      <ol class="etapes">
        <li>Dans Stripe, ouvre <a href="https://dashboard.stripe.com/apikeys" target="_blank" rel="noopener">Développeurs › Clés API</a> et crée une <strong>clé restreinte</strong>.</li>
        <li>Mets « Lecture » sur <strong>Subscriptions</strong> (abonnements) et <strong>Balance</strong> (solde). Laisse le reste sur « Aucun ».</li>
        <li>Copie la clé (elle commence par rk_live_, ou rk_test_ pour essayer) et colle-la ici.</li>
      </ol>
      <form @submit.prevent="connecterStripe">
        <label>
          Clé restreinte
          <input v-model="cleStripe" type="password" autocomplete="off" spellcheck="false" placeholder="rk_live_…" required />
        </label>
        <p class="aide">Ta clé est chiffrée et liée à ton compte Microdash. Elle ne lit rien d'autre et ne peut rien modifier ; tu peux la supprimer dans Stripe à tout moment.</p>
        <p v-if="stripeErreur" class="erreur" role="alert">{{ stripeErreur }}</p>
        <button class="bouton principal" :disabled="stripeAction === 'connexion'">
          {{ stripeAction === 'connexion' ? 'Vérification…' : 'Connecter Stripe' }}
        </button>
      </form>
    </template>

    <template v-else>
      <p class="aide">
        {{ stripe.compte_nom || stripe.compte_id || 'Compte Stripe' }}<template v-if="stripe.compte_nom && stripe.compte_id"> · {{ stripe.compte_id }}</template>
        · clé …{{ stripe.cle_fin }} · connecté le {{ formatDate(stripe.depuis) }}
      </p>
      <p v-if="stripe.erreur" class="erreur" role="alert">
        Dernière lecture impossible : {{ stripe.erreur }}<template v-if="stripe.mrr"> Les chiffres ci-dessous datent de la lecture précédente.</template>
      </p>
      <template v-if="stripe.mrr">
        <div class="mrr">
          <span>Revenu mensuel récurrent (MRR)</span>
          <strong class="chiffres-tab">{{ formatEuros(stripe.mrr.par_devise.eur || 0) }}</strong>
          <small v-for="[d, c] in autresDevises" :key="d">et {{ devises(d).format(c / 100) }} par mois, pas compté dans la prévision</small>
        </div>
        <p class="aide">
          {{ stripe.mrr.actifs }} abonnement{{ stripe.mrr.actifs > 1 ? 's' : '' }} actif{{ stripe.mrr.actifs > 1 ? 's' : '' }}<template v-if="stripe.mrr.en_essai">,
          {{ stripe.mrr.en_essai }} en essai (pas encore comptés)</template>.
          <template v-if="stripe.mrr.non_calcules"> {{ stripe.mrr.non_calcules }} avec un tarif à l'usage ou par paliers, compté sans cette partie.</template>
          <template v-if="stripe.mrr.remises"> {{ stripe.mrr.remises }} avec plusieurs remises : seule la première est déduite.</template>
        </p>
        <div v-if="moisStripe.length">
          <h3>Encaissé sur Stripe</h3>
          <dl class="chiffres">
            <div v-for="m in moisStripe" :key="m.mois">
              <dt>{{ majuscule(libelleMois(m.mois)) }}</dt>
              <dd>
                <strong>{{ formatEuros(m.encaisse) }}</strong>
                <small>frais {{ formatEuros(m.frais) }}<template v-if="m.rembourse"> · remboursé {{ formatEuros(m.rembourse) }}</template><template v-if="m.litiges"> · litiges {{ formatEuros(m.litiges) }}</template></small>
              </dd>
            </div>
          </dl>
        </div>
        <p class="aide">
          Les encaissements Stripe ne sont pas ajoutés à tes saisies : le chiffre d'affaires à déclarer reste celui que tu saisis.
          <template v-if="stripe.tronque"> Ton compte a beaucoup de mouvements : seule une partie a été lue.</template>
        </p>
      </template>
      <p v-if="stripeErreur" class="erreur" role="alert">{{ stripeErreur }}</p>
      <div class="actions-stripe">
        <span class="aide">Lu le {{ luLe }}</span>
        <button class="lien-bouton" :disabled="stripeAction === 'actualiser'" @click="actualiserStripe">
          <RefreshCw :size="18" :stroke-width="1.75" aria-hidden="true" />{{ stripeAction === 'actualiser' ? 'Lecture…' : 'Actualiser' }}
        </button>
        <button class="lien-bouton danger" @click="deconnecterStripe">Déconnecter</button>
      </div>
    </template>
  </section>

  <section v-if="donnees" class="carte pile" aria-labelledby="titre-recurrentes">
    <h2 id="titre-recurrentes">Dépenses récurrentes</h2>
    <p class="aide">Tes abonnements et frais fixes. Microdash les ajoute à tes saisies à chaque échéance et les compte dans la prévision.</p>
    <ul v-if="recurrentes.length" class="liste">
      <li v-for="r in recurrentes" :key="r.id">
        <div>
          <strong>{{ r.libelle }}</strong>
          <small>{{ majuscule(rythme(r)) }}<template v-if="r.fin"> jusqu'au {{ formatDate(r.fin) }}</template> · {{ r.poste }}</small>
        </div>
        <span class="montant">{{ formatEuros(r.centimes) }}</span>
        <div class="actions">
          <button class="lien-bouton" @click="editerRec(r)"><Pencil :size="18" :stroke-width="1.75" aria-hidden="true" />Modifier</button>
          <button class="lien-bouton danger" @click="supprimerRec(r)"><Trash2 :size="18" :stroke-width="1.75" aria-hidden="true" />Arrêter</button>
        </div>
      </li>
    </ul>

    <form ref="formRec" @submit.prevent="enregistrerRec">
      <h3>{{ recEdition ? 'Modifier la dépense' : 'Ajouter une dépense récurrente' }}</h3>
      <div class="bascule">
        <button type="button" :class="{ actif: rec.frequence === 'mensuelle' }" @click="rec.frequence = 'mensuelle'">Chaque mois</button>
        <button type="button" :class="{ actif: rec.frequence === 'annuelle' }" @click="rec.frequence = 'annuelle'">Chaque année</button>
      </div>
      <div class="ligne">
        <label>Nom <input v-model="rec.libelle" placeholder="Ex. : Figma" required /></label>
        <label>Montant (€) <input v-model="rec.montant" inputmode="decimal" placeholder="0,00" required /></label>
      </div>
      <label>
        Poste
        <select v-model="rec.poste">
          <option v-for="p in postesRecurrents" :key="p">{{ p }}</option>
        </select>
      </label>
      <div class="ligne">
        <label>Première échéance <input v-model="rec.debut" type="date" required /></label>
        <label>Dernière (facultatif) <input v-model="rec.fin" type="date" :min="rec.debut" /></label>
      </div>
      <p class="aide">
        <template v-if="recEdition">Les saisies déjà créées ne changent pas : seules les prochaines suivent la modification.</template>
        <template v-else>Si la première échéance est passée, les saisies jusqu'à aujourd'hui sont créées tout de suite.</template>
      </p>
      <p v-if="recErreur" class="erreur" role="alert">{{ recErreur }}</p>
      <p v-if="recMessage" class="succes" role="status">{{ recMessage }}</p>
      <div class="ligne">
        <button class="bouton principal">{{ recEdition ? 'Enregistrer' : 'Ajouter' }}</button>
        <button v-if="recEdition" type="button" class="bouton secondaire" @click="annulerRec">Annuler</button>
      </div>
    </form>
  </section>
</template>

<style scoped>
.bandeau p { color: var(--blanc); font-size: 15px; }
.hero, .mrr { display: grid; gap: var(--e1); }
.hero span, .mrr span { color: var(--pierre); font-size: 14px; font-weight: 500; }
.hero strong { font-size: 40px; line-height: 1.1; font-weight: 700; letter-spacing: -0.01em; }
.mrr strong { font-size: 28px; line-height: 1.2; font-weight: 700; }
.hero small, .mrr small { font-size: 13px; font-weight: 600; color: var(--pierre); }
.hero small.hausse { color: var(--sarcelle); }
.hero small.baisse { color: var(--safran-fonce); }
.hero small.neutre { font-weight: 400; }
.source { display: grid; gap: var(--e2); }
.source > span { font-size: 14px; font-weight: 500; color: var(--pierre); }
.source .bascule button { font-variant-numeric: tabular-nums; }
.ligne-solde { display: flex; flex-wrap: wrap; align-items: center; gap: 0 var(--e3); }
.ligne-solde .lien-bouton { min-height: 44px; }
.form-solde { background: var(--papier); border-radius: var(--rayon); padding: var(--e4); }
.badge { background: var(--safran-clair); color: var(--safran-fonce); font-size: 13px; font-weight: 600; padding: 2px var(--e2); border-radius: var(--rayon); }
.etapes { margin: 0; padding-left: var(--e5); display: grid; gap: var(--e2); font-size: 15px; }
h3 { margin-bottom: var(--e2); }
form h3 { margin: var(--e2) 0 0; }
.chiffres dd { display: grid; justify-items: end; }
.chiffres dd small { color: var(--pierre); font-size: 13px; }
.actions-stripe { display: flex; flex-wrap: wrap; align-items: center; gap: 0 var(--e4); }
.actions-stripe .aide { flex: 1 1 100%; }
.actions-stripe .lien-bouton { min-height: 44px; }
</style>
