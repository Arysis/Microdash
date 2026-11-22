<script setup>
// Capture rapide : ce qui est partagé depuis une autre app (Partager › Microdash) devient une tâche.
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api.js'
import { projets, lireDuree } from '../taches.js'

const route = useRoute()
const router = useRouter()
const erreur = ref('')
const enCours = ref(false)
const tache = reactive({ titre: '', notes: '', lien: '', projet: '', quand: 'dates' })
const quands = [
  { quand: 'dates', nom: 'Ce soir' },
  { quand: 'semaine', nom: 'Cette semaine' },
  { quand: 'plus_tard', nom: 'Plus tard' },
]

const premier = (v) => (Array.isArray(v) ? v[0] : v) || ''

onMounted(() => {
  const titre = premier(route.query.title).trim()
  let texte = premier(route.query.text).trim()
  let lien = premier(route.query.url).trim()
  // Android met souvent l'adresse dans le texte : on la sort.
  const trouve = texte.match(/https?:\/\/\S+/)
  if (!lien && trouve) lien = trouve[0]
  if (lien) texte = texte.replace(lien, '').trim()
  if (!/^https?:\/\//.test(lien)) lien = ''
  let t = titre || texte.split('\n')[0] || ''
  if (!t && lien) {
    try {
      t = new URL(lien).hostname.replace(/^www\./, '')
    } catch { /* adresse illisible : titre vide */ }
  }
  tache.titre = t.slice(0, 200)
  tache.notes = (texte && texte !== t ? texte : '').slice(0, 4000)
  tache.lien = lien
})

async function enregistrer() {
  const { titre, minutes } = lireDuree(tache.titre)
  if (!titre) return
  enCours.value = true
  erreur.value = ''
  try {
    await api.post('/taches', { titre, duree_minutes: minutes, notes: tache.notes, projet: tache.projet, quand: tache.quand, lien: tache.lien })
    router.replace('/taches')
  } catch (e) {
    erreur.value = e.message
  } finally {
    enCours.value = false
  }
}
</script>

<template>
  <h1>Nouvelle tâche</h1>
  <form class="carte pile" @submit.prevent="enregistrer">
    <label>Tâche <input v-model="tache.titre" maxlength="220" required autofocus placeholder="Ex. : Lire cet article 15 min" /></label>
    <p v-if="tache.lien" class="aide lien">Lien gardé : <a :href="tache.lien" target="_blank" rel="noopener">{{ tache.lien }}</a></p>
    <div class="bascule" role="group" aria-label="Quand">
      <button v-for="q in quands" :key="q.quand" type="button" :class="{ actif: tache.quand === q.quand }" :aria-pressed="tache.quand === q.quand" @click="tache.quand = q.quand">{{ q.nom }}</button>
    </div>
    <label>Projet
      <select v-model="tache.projet">
        <option value="">Aucun</option>
        <option v-for="p in projets" :key="p.code" :value="p.code">{{ p.nom }}</option>
      </select>
    </label>
    <label>Notes <textarea v-model="tache.notes" rows="3" maxlength="4000" /></label>
    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
    <div class="boutons-form">
      <RouterLink class="bouton secondaire" to="/taches">Annuler</RouterLink>
      <button class="bouton principal" :disabled="enCours || !tache.titre.trim()">Ajouter la tâche</button>
    </div>
  </form>
</template>

<style scoped>
h1 { margin-bottom: var(--e5); }
.lien { overflow-wrap: anywhere; }
.boutons-form { display: flex; gap: var(--e2); justify-content: flex-end; flex-wrap: wrap; }
</style>
