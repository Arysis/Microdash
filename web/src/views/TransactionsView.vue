<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { Pencil, Trash2 } from 'lucide-vue-next'
import { api } from '../api.js'
import { demanderConfirmation } from '../confirmation.js'
import { aujourdhui, formatDate, formatEuros, posteURSSAF, postesDepense, versCentimes } from '../format.js'

const anneeEnCours = new Date().getFullYear()
const annee = ref(anneeEnCours)
const transactions = ref([])
const profil = ref(null)
const categories = ref({})
const erreur = ref('')
const filtre = ref('tout')
const enEdition = ref(null)

const vide = () => ({ type: 'recette', date: aujourdhui(), montant: '', categorie: '', poste: postesDepense[0], libelle: '', tiers: '', echeance: '' })
const saisie = reactive(vide())

const categoriesProfil = computed(() =>
  profil.value ? [profil.value.categorie, profil.value.categorie_secondaire].filter(Boolean) : [],
)
const visibles = computed(() =>
  filtre.value === 'tout' ? transactions.value : transactions.value.filter((t) => t.type === filtre.value),
)

// --- Paiement URSSAF : la déclaration qu'il règle ---
const declarations = ref([])
const estURSSAF = computed(() => saisie.type === 'depense' && saisie.poste === posteURSSAF)

// Déclarations URSSAF de l'année du paiement et de la précédente, la plus récente en premier.
async function chargerDeclarations() {
  const a = Number(saisie.date.slice(0, 4))
  if (!a) return
  try {
    const listes = await Promise.all([a, a - 1].map((x) => api.get(`/agenda?annee=${x}`).catch(() => ({ echeances: [] }))))
    const vues = new Set()
    declarations.value = listes
      .flatMap((l) => l.echeances)
      .filter((e) => e.type === 'urssaf' && e.periode_debut <= saisie.date && !vues.has(e.code) && vues.add(e.code))
      .sort((x, y) => (x.periode_fin < y.periode_fin ? 1 : -1))
  } catch {
    declarations.value = []
  }
  // Par défaut : la dernière période terminée avant le paiement.
  if (!declarations.value.some((e) => e.code === saisie.echeance)) {
    saisie.echeance = declarations.value.find((e) => e.periode_fin < saisie.date)?.code || declarations.value[0]?.code || ''
  }
}
watch(() => [estURSSAF.value, saisie.date], () => estURSSAF.value && chargerDeclarations())

async function charger() {
  erreur.value = ''
  try {
    transactions.value = await api.get(`/transactions?du=${annee.value}-01-01&au=${annee.value}-12-31`)
  } catch (e) {
    erreur.value = e.message
  }
}

onMounted(async () => {
  const [p, b] = await Promise.all([api.get('/profil'), api.get(`/baremes/${annee.value}`)])
  profil.value = p
  categories.value = Object.fromEntries(b.categories.map((c) => [c.code, c.libelle]))
  saisie.categorie = p.categorie
  await charger()
})

function reinitialiser() {
  Object.assign(saisie, vide(), { categorie: profil.value?.categorie ?? '' })
  enEdition.value = null
}

function editer(t) {
  enEdition.value = t.id
  Object.assign(saisie, {
    type: t.type,
    date: t.date,
    montant: (t.centimes / 100).toFixed(2).replace('.', ','),
    categorie: t.categorie || profil.value.categorie,
    poste: t.poste || postesDepense[0],
    libelle: t.libelle,
    tiers: t.tiers,
    echeance: t.echeance || '',
  })
  window.scrollTo({ top: 0, behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' })
}

async function enregistrer() {
  erreur.value = ''
  const centimes = versCentimes(saisie.montant)
  if (!(centimes > 0)) {
    erreur.value = 'Montant invalide.'
    return
  }
  const corps = {
    type: saisie.type,
    date: saisie.date,
    centimes,
    categorie: saisie.type === 'recette' ? saisie.categorie : '',
    poste: saisie.type === 'depense' ? saisie.poste : '',
    libelle: saisie.libelle,
    tiers: saisie.tiers,
    echeance: estURSSAF.value ? saisie.echeance : '',
  }
  if (estURSSAF.value && !corps.echeance) {
    erreur.value = 'Choisis la déclaration que ce paiement règle.'
    return
  }
  try {
    if (enEdition.value) await api.put(`/transactions/${enEdition.value}`, corps)
    else await api.post('/transactions', corps)
    reinitialiser()
    await charger()
  } catch (e) {
    erreur.value = e.message
  }
}

// « urssaf-2026-09 » → « septembre 2026 », « urssaf-2026-t3 » → « 3e trimestre 2026 ».
const moisLongs = ['janvier', 'février', 'mars', 'avril', 'mai', 'juin', 'juillet', 'août', 'septembre', 'octobre', 'novembre', 'décembre']
function libelleEcheance(code) {
  const m = /^urssaf-(\d{4})-(t?)(\d+)$/.exec(code)
  if (!m) return code
  if (m[2]) return `pour le ${m[3] === '1' ? '1er' : `${m[3]}e`} trimestre ${m[1]}`
  return `pour ${moisLongs[Number(m[3]) - 1]} ${m[1]}`
}

async function supprimer(t) {
  const ok = await demanderConfirmation({
    titre: 'Supprimer cette saisie ?',
    message: `${t.libelle || (t.type === 'recette' ? 'Recette' : t.poste)} du ${formatDate(t.date)}, ${formatEuros(t.centimes)}. Elle disparaît de tes calculs.`,
    action: 'Supprimer',
  })
  if (!ok) return
  try {
    await api.del(`/transactions/${t.id}`)
    await charger()
  } catch (e) {
    erreur.value = e.message
  }
}
</script>

<template>
  <div class="bandeau">
    <h1>Saisies</h1>
  </div>

  <section class="carte pile">
    <h2>{{ enEdition ? 'Modifier la saisie' : 'Nouvelle saisie' }}</h2>
    <form @submit.prevent="enregistrer">
      <div class="bascule">
        <button type="button" :class="{ actif: saisie.type === 'recette' }" @click="saisie.type = 'recette'">Recette</button>
        <button type="button" :class="{ actif: saisie.type === 'depense' }" @click="saisie.type = 'depense'">Dépense</button>
      </div>
      <div class="ligne">
        <label>Montant (€) <input v-model="saisie.montant" inputmode="decimal" placeholder="0,00" required /></label>
        <label>{{ saisie.type === 'recette' ? 'Encaissé le' : 'Payé le' }} <input v-model="saisie.date" type="date" required /></label>
      </div>
      <label v-if="saisie.type === 'recette' && categoriesProfil.length > 1">
        Activité
        <select v-model="saisie.categorie">
          <option v-for="c in categoriesProfil" :key="c" :value="c">{{ categories[c] }}</option>
        </select>
      </label>
      <label v-if="saisie.type === 'depense'">
        Poste
        <select v-model="saisie.poste">
          <option v-for="p in postesDepense" :key="p">{{ p }}</option>
        </select>
      </label>
      <label v-if="estURSSAF">
        Pour la déclaration
        <select v-model="saisie.echeance" required>
          <option v-if="!declarations.length" value="" disabled>Aucune déclaration avant cette date</option>
          <option v-for="e in declarations" :key="e.code" :value="e.code">
            {{ e.libelle.replace('Déclaration URSSAF ', '') }} · {{ e.paye ? `payé ${formatEuros(e.paye)}` : `estimé ${formatEuros(e.a_payer)}` }}
          </option>
        </select>
      </label>
      <label>Libellé <input v-model="saisie.libelle" placeholder="Ex. : site vitrine" /></label>
      <label>{{ saisie.type === 'recette' ? 'Client' : 'Fournisseur' }} (facultatif) <input v-model="saisie.tiers" /></label>
      <p v-if="estURSSAF" class="aide">
        Ce paiement remplace l'estimation de la déclaration choisie, sur le mois qu'elle couvre. Il ne compte pas comme une dépense.
      </p>
      <p v-else-if="saisie.type === 'depense'" class="aide">
        En micro-entreprise, une dépense ne baisse ni tes cotisations ni ton impôt. Elle sert à suivre ton revenu net réel.
      </p>
      <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
      <div class="ligne">
        <button class="bouton principal">{{ enEdition ? 'Enregistrer' : 'Ajouter' }}</button>
        <button v-if="enEdition" type="button" class="bouton secondaire" @click="reinitialiser">Annuler</button>
      </div>
    </form>
  </section>

  <section class="carte pile">
    <div class="titre-ligne">
      <h2>Saisies {{ annee }}</h2>
      <select v-model="annee" aria-label="Année" @change="charger">
        <option v-for="a in [anneeEnCours + 1, anneeEnCours, anneeEnCours - 1, anneeEnCours - 2]" :key="a" :value="a">{{ a }}</option>
      </select>
    </div>
    <div class="bascule petite">
      <button :class="{ actif: filtre === 'tout' }" @click="filtre = 'tout'">Tout</button>
      <button :class="{ actif: filtre === 'recette' }" @click="filtre = 'recette'">Recettes</button>
      <button :class="{ actif: filtre === 'depense' }" @click="filtre = 'depense'">Dépenses</button>
    </div>
    <p v-if="!visibles.length" class="aide">Aucune saisie sur cette période. Ajoute ta première recette avec le formulaire ci-dessus.</p>
    <ul class="liste">
      <li v-for="t in visibles" :key="t.id">
        <div>
          <strong>{{ t.libelle || (t.type === 'recette' ? 'Recette' : t.poste) }}</strong>
          <small>{{ formatDate(t.date) }}<template v-if="t.tiers"> · {{ t.tiers }}</template><template v-if="t.type === 'depense' && t.libelle"> · {{ t.poste }}</template><template v-if="t.recurrente"> · récurrente</template><template v-if="t.echeance"> · {{ libelleEcheance(t.echeance) }}</template></small>
        </div>
        <span :class="['montant', t.type]">{{ t.type === 'depense' ? '−' : '+' }}{{ formatEuros(t.centimes) }}</span>
        <div class="actions">
          <button class="lien-bouton" @click="editer(t)"><Pencil :size="18" :stroke-width="1.75" aria-hidden="true" />Modifier</button>
          <button class="lien-bouton danger" @click="supprimer(t)"><Trash2 :size="18" :stroke-width="1.75" aria-hidden="true" />Supprimer</button>
        </div>
      </li>
    </ul>
  </section>
</template>
