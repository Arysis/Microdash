<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RefreshCw, RotateCcw } from 'lucide-vue-next'
import { api } from '../api.js'
import { declarationsURSSAF } from '../echeances.js'
import { formatDate, formatEuros, posteURSSAF, postesDepense, versCentimes } from '../format.js'

// Opérations lues chez Qonto qui attendent une décision : créer la saisie, ou l'ignorer.
const props = defineProps({
  categoriesProfil: { type: Array, default: () => [] },
  categories: { type: Object, default: () => ({}) },
})
const emit = defineEmits(['saisie'])

const etat = ref(null)
const ops = ref([])
const ignorees = ref(null)
const erreur = ref('')
const message = ref('')
const synchro = ref(false)
const ouverte = ref(null)
const envoi = ref(false)
const form = reactive({ type: 'recette', montant: '', date: '', categorie: '', poste: '', echeance: '', libelle: '', tiers: '', retenir: true })

const visible = computed(() => etat.value && (etat.value.connecte || ops.value.length))
const estURSSAF = computed(() => form.type === 'depense' && form.poste === posteURSSAF)
const operation = computed(() => ops.value.find((o) => o.id === ouverte.value))
const viaStripe = computed(() => form.type === 'recette' && /stripe/i.test(operation.value?.nom || ''))
const synchroLe = computed(() => {
  const v = etat.value?.synchro_le
  if (!v) return ''
  const d = new Date(v)
  return `${d.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })} à ${d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}`
})

async function charger() {
  try {
    const [e, o] = await Promise.all([api.get('/qonto'), api.get('/qonto/operations')])
    etat.value = e
    ops.value = o
    if (ignorees.value) ignorees.value = await api.get('/qonto/operations?statut=ignoree')
  } catch (e) {
    erreur.value = e.message
  }
}
onMounted(charger)

function bilan(b) {
  const parties = []
  if (b.regles) parties.push(`${b.regles} ajoutée${b.regles > 1 ? 's' : ''} selon tes choix`)
  if (b.rapprochees) parties.push(`${b.rapprochees} déjà saisie${b.rapprochees > 1 ? 's' : ''}`)
  if (b.ignorees) parties.push(`${b.ignorees} ignorée${b.ignorees > 1 ? 's' : ''}`)
  if (b.a_valider) parties.push(`${b.a_valider} à valider`)
  if (!b.nouvelles) return 'Aucune nouvelle opération.'
  return `${b.nouvelles} nouvelle${b.nouvelles > 1 ? 's' : ''} opération${b.nouvelles > 1 ? 's' : ''} : ${parties.join(', ')}.`
}

async function synchroniser() {
  erreur.value = ''
  message.value = ''
  synchro.value = true
  try {
    const r = await api.post('/qonto/synchroniser')
    message.value = bilan(r.bilan)
    await charger()
    if (r.bilan.regles || r.bilan.rapprochees) emit('saisie')
  } catch (e) {
    erreur.value = e.message
    await charger()
  } finally {
    synchro.value = false
  }
}

const declarations = ref([])
async function chargerDeclarations() {
  try {
    declarations.value = await declarationsURSSAF(form.date)
  } catch {
    declarations.value = []
  }
  if (!declarations.value.some((e) => e.code === form.echeance)) {
    form.echeance = declarations.value.find((e) => e.periode_fin < form.date)?.code || declarations.value[0]?.code || ''
  }
}
watch(() => [estURSSAF.value, form.date], () => estURSSAF.value && chargerDeclarations())

function ouvrir(o) {
  erreur.value = ''
  message.value = ''
  ouverte.value = o.id
  Object.assign(form, {
    type: o.type,
    montant: (o.centimes / 100).toFixed(2).replace('.', ','),
    date: o.date,
    categorie: props.categoriesProfil[0] || '',
    poste: o.poste_propose || postesDepense[0],
    echeance: o.echeance_proposee,
    libelle: o.nom,
    tiers: '',
    retenir: true,
  })
}

async function valider() {
  erreur.value = ''
  const centimes = versCentimes(form.montant)
  if (!(centimes > 0)) {
    erreur.value = 'Montant invalide.'
    return
  }
  if (estURSSAF.value && !form.echeance) {
    erreur.value = 'Choisis la déclaration que ce paiement règle.'
    return
  }
  envoi.value = true
  try {
    await api.post(`/qonto/operations/${ouverte.value}/valider`, {
      type: form.type,
      date: form.date,
      centimes,
      categorie: form.type === 'recette' ? form.categorie : '',
      poste: form.type === 'depense' ? form.poste : '',
      libelle: form.libelle,
      tiers: form.tiers,
      echeance: estURSSAF.value ? form.echeance : '',
      retenir: form.retenir,
    })
    ouverte.value = null
    await charger()
    emit('saisie')
  } catch (e) {
    erreur.value = e.message
  } finally {
    envoi.value = false
  }
}

async function ignorer() {
  erreur.value = ''
  envoi.value = true
  try {
    await api.post(`/qonto/operations/${ouverte.value}/ignorer`, { retenir: form.retenir })
    ouverte.value = null
    await charger()
  } catch (e) {
    erreur.value = e.message
  } finally {
    envoi.value = false
  }
}

async function voirIgnorees() {
  if (ignorees.value) {
    ignorees.value = null
    return
  }
  try {
    ignorees.value = await api.get('/qonto/operations?statut=ignoree')
  } catch (e) {
    erreur.value = e.message
  }
}

async function remettre(o) {
  erreur.value = ''
  try {
    await api.post(`/qonto/operations/${o.id}/remettre`)
    await charger()
  } catch (e) {
    erreur.value = e.message
  }
}

const motifs = { interne: 'virement entre tes comptes', regle: 'selon ton choix', manuel: 'ignorée à la main' }
const signe = (o) => `${o.type === 'depense' ? '−' : '+'}${formatEuros(o.centimes)}`
</script>

<template>
  <section v-if="visible" class="carte pile" aria-labelledby="titre-qonto">
    <div class="titre-ligne">
      <h2 id="titre-qonto">À valider<template v-if="ops.length"> · {{ ops.length }}</template></h2>
      <button v-if="etat.connecte" class="lien-bouton" :disabled="synchro" @click="synchroniser">
        <RefreshCw :size="18" :stroke-width="1.75" aria-hidden="true" />{{ synchro ? 'Synchronisation…' : 'Synchroniser Qonto' }}
      </button>
    </div>
    <p class="aide">
      Les opérations de ton compte Qonto. Elles ne comptent dans tes calculs qu'une fois validées.
      <template v-if="synchroLe"> Dernière synchro le {{ synchroLe }}.</template>
    </p>
    <p v-if="etat.erreur" class="erreur" role="alert">Dernière synchro impossible : {{ etat.erreur }}</p>
    <p v-if="message" class="succes" role="status">{{ message }}</p>
    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
    <p v-if="!ops.length" class="aide">Rien à valider.</p>

    <ul class="liste">
      <li v-for="o in ops" :key="o.id">
        <div>
          <strong>{{ o.nom || 'Opération' }}</strong>
          <small>{{ formatDate(o.date) }}<template v-if="o.reference"> · {{ o.reference }}</template></small>
        </div>
        <span :class="['montant', o.type]">{{ signe(o) }}</span>
        <div v-if="ouverte !== o.id" class="actions">
          <button class="lien-bouton" @click="ouvrir(o)">Traiter</button>
        </div>
        <form v-else class="traitement" @submit.prevent="valider">
          <div class="bascule petite">
            <button type="button" :class="{ actif: form.type === 'recette' }" @click="form.type = 'recette'">Recette</button>
            <button type="button" :class="{ actif: form.type === 'depense' }" @click="form.type = 'depense'">Dépense</button>
          </div>
          <div class="ligne">
            <label>Montant (€) <input v-model="form.montant" inputmode="decimal" required /></label>
            <label>{{ form.type === 'recette' ? 'Encaissé le' : 'Payé le' }} <input v-model="form.date" type="date" required /></label>
          </div>
          <label v-if="form.type === 'recette' && categoriesProfil.length > 1">
            Activité
            <select v-model="form.categorie">
              <option v-for="c in categoriesProfil" :key="c" :value="c">{{ categories[c] }}</option>
            </select>
          </label>
          <label v-if="form.type === 'depense'">
            Poste
            <select v-model="form.poste">
              <option v-for="p in postesDepense" :key="p">{{ p }}</option>
            </select>
          </label>
          <label v-if="estURSSAF">
            Pour la déclaration
            <select v-model="form.echeance" required>
              <option v-if="!declarations.length" value="" disabled>Aucune déclaration avant cette date</option>
              <option v-for="e in declarations" :key="e.code" :value="e.code">
                {{ e.libelle.replace('Déclaration URSSAF ', '') }} · {{ e.paye ? `payé ${formatEuros(e.paye)}` : `estimé ${formatEuros(e.a_payer)}` }}
              </option>
            </select>
          </label>
          <label>Libellé <input v-model="form.libelle" /></label>
          <p v-if="viaStripe" class="aide">
            Un virement Stripe arrive net des frais Stripe. Ton chiffre d'affaires à déclarer est le montant brut payé par tes clients :
            corrige le montant si besoin (il est indiqué dans Stripe).
          </p>
          <label class="case">
            <input v-model="form.retenir" type="checkbox" />
            Faire pareil pour les prochaines opérations « {{ o.nom }} »
          </label>
          <div class="ligne">
            <button class="bouton principal" :disabled="envoi">Créer la saisie</button>
            <button type="button" class="bouton secondaire" :disabled="envoi" @click="ignorer">Ignorer</button>
            <button type="button" class="bouton secondaire" @click="ouverte = null">Annuler</button>
          </div>
        </form>
      </li>
    </ul>

    <button class="lien-bouton" @click="voirIgnorees">{{ ignorees ? 'Masquer les opérations ignorées' : 'Voir les opérations ignorées' }}</button>
    <ul v-if="ignorees" class="liste">
      <li v-if="!ignorees.length"><small>Aucune opération ignorée.</small></li>
      <li v-for="o in ignorees" :key="o.id">
        <div>
          <strong>{{ o.nom || 'Opération' }}</strong>
          <small>{{ formatDate(o.date) }} · {{ motifs[o.motif] || 'ignorée' }}</small>
        </div>
        <span class="montant">{{ signe(o) }}</span>
        <div class="actions">
          <button class="lien-bouton" @click="remettre(o)"><RotateCcw :size="18" :stroke-width="1.75" aria-hidden="true" />Remettre à valider</button>
        </div>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.traitement { grid-column: 1 / -1; display: grid; gap: var(--e3); background: var(--papier); border-radius: var(--rayon); padding: var(--e4); }
.titre-ligne .lien-bouton, .pile > .lien-bouton { min-height: 44px; justify-self: start; }
</style>
