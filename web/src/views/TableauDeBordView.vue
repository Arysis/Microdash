<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api } from '../api.js'
import { formatDate, formatEuros } from '../format.js'

const maintenant = new Date()
const annee = ref(maintenant.getFullYear())
const vue = ref('annee')
const index = ref(0)
const donnees = ref(null)
const profil = ref(null)
const erreur = ref('')

async function charger() {
  erreur.value = ''
  try {
    donnees.value = await api.get(`/tableau-de-bord?annee=${annee.value}`)
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
  await charger()
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
const impotLibelle = computed(() => (profil.value?.versement_liberatoire ? 'Impôt (versement libératoire)' : null))

function changerVue(v) {
  vue.value = v
  index.value = v === 'mois' ? maintenant.getMonth() : Math.floor(maintenant.getMonth() / 3)
}
</script>

<template>
  <section class="carte pile">
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
      <option v-for="(p, i) in periodes" :key="p.debut" :value="i">{{ p.libelle }}</option>
    </select>

    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
    <p v-if="donnees && !donnees.bareme_exact" class="encart">
      Pas encore de barème {{ annee }} : les calculs utilisent les taux {{ donnees.bareme_annee }}.
    </p>

    <template v-if="totaux">
      <div class="net">
        <span>Revenu net estimé · {{ libellePeriode }}</span>
        <strong class="chiffres-tab">{{ formatEuros(totaux.net) }}</strong>
      </div>
      <div v-if="totaux.ca > 0" class="repartition" role="img" :aria-label="`Sur ${formatEuros(totaux.ca)} encaissés, il te reste ${formatEuros(totaux.net)}`">
        <div :style="{ flex: Math.max(totaux.net, 0), background: 'var(--sarcelle)' }"></div>
        <div :style="{ flex: totaux.cotisations + totaux.cfp + totaux.impot_vl, background: 'var(--safran)' }"></div>
        <div :style="{ flex: totaux.depenses, background: 'var(--pierre)' }"></div>
      </div>
      <dl class="chiffres">
        <div><dt>Chiffre d'affaires encaissé</dt><dd>{{ formatEuros(totaux.ca) }}</dd></div>
        <div><dt><span class="pastille" style="background: var(--safran)"></span>Cotisations sociales</dt><dd>− {{ formatEuros(totaux.cotisations) }}</dd></div>
        <div><dt><span class="pastille" style="background: var(--safran)"></span>Formation professionnelle (CFP)</dt><dd>− {{ formatEuros(totaux.cfp) }}</dd></div>
        <div v-if="impotLibelle"><dt><span class="pastille" style="background: var(--safran)"></span>{{ impotLibelle }}</dt><dd>− {{ formatEuros(totaux.impot_vl) }}</dd></div>
        <div><dt><span class="pastille" style="background: var(--pierre)"></span>Dépenses</dt><dd>− {{ formatEuros(totaux.depenses) }}</dd></div>
        <div><dt><span class="pastille" style="background: var(--sarcelle)"></span>Revenu net</dt><dd><strong>{{ formatEuros(totaux.net) }}</strong></dd></div>
      </dl>
      <p v-if="vue === 'annee' && !profil?.versement_liberatoire" class="aide">
        Revenu imposable de l'activité en {{ annee }} : <strong>{{ formatEuros(donnees.revenu_imposable) }}</strong>.
        C'est ton chiffre d'affaires après abattement. Il s'ajoute à tes autres revenus dans ta déclaration d'impôt.
      </p>
      <p v-if="donnees.fin_acre" class="aide">ACRE appliquée jusqu'au {{ formatDate(donnees.fin_acre) }}.</p>
    </template>
  </section>

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
</template>

<style scoped>
.net { display: grid; gap: var(--e1); }
.net span { color: var(--pierre); font-size: 14px; }
.net strong { font-size: 36px; line-height: 1.1; font-weight: 600; }
</style>
