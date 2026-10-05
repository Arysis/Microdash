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
  profil.value = await api.get('/profil')
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
  <section class="carte">
    <div class="titre-ligne">
      <h1>Tableau de bord</h1>
      <select v-model="annee">
        <option v-for="a in [maintenant.getFullYear() + 1, maintenant.getFullYear(), maintenant.getFullYear() - 1, maintenant.getFullYear() - 2]" :key="a" :value="a">{{ a }}</option>
      </select>
    </div>
    <div class="bascule petite">
      <button :class="{ actif: vue === 'mois' }" @click="changerVue('mois')">Mois</button>
      <button :class="{ actif: vue === 'trimestre' }" @click="changerVue('trimestre')">Trimestre</button>
      <button :class="{ actif: vue === 'annee' }" @click="changerVue('annee')">Année</button>
    </div>
    <select v-if="vue !== 'annee'" v-model="index" class="periode">
      <option v-for="(p, i) in periodes" :key="p.debut" :value="i">{{ p.libelle }}</option>
    </select>

    <p v-if="erreur" class="erreur">{{ erreur }}</p>
    <p v-if="donnees && !donnees.bareme_exact" class="alerte">
      Pas encore de barème {{ annee }} : les calculs utilisent les taux {{ donnees.bareme_annee }}.
    </p>

    <template v-if="totaux">
      <div class="net">
        <span>Revenu net · {{ libellePeriode }}</span>
        <strong>{{ formatEuros(totaux.net) }}</strong>
      </div>
      <dl class="chiffres">
        <div><dt>Chiffre d'affaires encaissé</dt><dd>{{ formatEuros(totaux.ca) }}</dd></div>
        <div><dt>Cotisations sociales</dt><dd>− {{ formatEuros(totaux.cotisations) }}</dd></div>
        <div><dt>Formation professionnelle (CFP)</dt><dd>− {{ formatEuros(totaux.cfp) }}</dd></div>
        <div v-if="impotLibelle"><dt>{{ impotLibelle }}</dt><dd>− {{ formatEuros(totaux.impot_vl) }}</dd></div>
        <div><dt>Dépenses</dt><dd>− {{ formatEuros(totaux.depenses) }}</dd></div>
      </dl>
      <p v-if="vue === 'annee' && !profil?.versement_liberatoire" class="aide">
        Revenu imposable de l'activité {{ annee }} : <strong>{{ formatEuros(donnees.revenu_imposable) }}</strong>
        (CA après abattement forfaitaire). Il s'ajoute à tes autres revenus dans ta déclaration ; l'impôt n'est pas déduit ici.
      </p>
      <p v-if="donnees.fin_acre" class="aide">ACRE appliquée jusqu'au {{ formatDate(donnees.fin_acre) }}.</p>
    </template>
  </section>

  <section v-if="donnees" class="carte">
    <h2>Plafonds {{ annee }}</h2>
    <div v-for="p in donnees.plafonds" :key="p.code" class="jauge">
      <div class="titre-ligne">
        <span>{{ p.libelle }}</span>
        <small>{{ formatEuros(p.ca) }} / {{ formatEuros(p.plafond) }}</small>
      </div>
      <div class="barre" :class="p.niveau">
        <div :style="{ width: Math.min(100, p.ratio * 100) + '%' }"></div>
      </div>
      <small v-if="p.niveau === 'attention'" class="avertissement">Plus de 80 % du seuil atteint.</small>
      <small v-if="p.niveau === 'depasse'" class="danger">Seuil dépassé<template v-if="p.majore"> (seuil majoré : {{ formatEuros(p.majore) }})</template>.</small>
    </div>
    <p class="aide">Estimations à partir de tes saisies et du barème en vigueur. Seule ta déclaration URSSAF fait foi.</p>
  </section>
</template>
