<script setup>
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Check, X } from 'lucide-vue-next'
import { api } from '../api.js'
import { session } from '../session.js'

// Page d'autorisation OAuth du connecteur Claude : Claude y envoie la personne, qui accepte ou refuse.
const route = useRoute()
const champs = ['client_id', 'redirect_uri', 'response_type', 'code_challenge', 'code_challenge_method', 'state', 'scope', 'resource']
const demande = Object.fromEntries(champs.map((c) => [c, typeof route.query[c] === 'string' ? route.query[c] : '']))

const client = ref('')
const erreur = ref('')
const charge = ref(false)
const envoi = ref(false)
const icone = { size: 20, 'stroke-width': 1.75, 'aria-hidden': 'true' }

const peut = [
  'Lire ta liste « Ce soir », tes tâches et ton agenda',
  'Ajouter, modifier, cocher et supprimer des tâches et des rendez-vous',
  'Marquer une échéance URSSAF déclarée ou payée',
  "Lire le résumé de tes finances de l'année",
]
const nePeutPas = [
  'Toucher à tes saisies, à Qonto, à Stripe ou à tes mails',
  'Changer ton mot de passe ou supprimer ton compte',
]

onMounted(async () => {
  try {
    client.value = (await api.get('/oauth/demande?' + new URLSearchParams(demande))).client
  } catch (e) {
    erreur.value = e.message
  } finally {
    charge.value = true
  }
})

async function repondre(autoriser) {
  erreur.value = ''
  envoi.value = true
  try {
    const { redirection } = await api.post('/oauth/autoriser', { ...demande, autoriser })
    window.location.assign(redirection)
  } catch (e) {
    erreur.value = e.message
    envoi.value = false
  }
}
</script>

<template>
  <section class="cadre">
    <div class="carte pile">
      <template v-if="charge && client">
        <div class="pile-serree">
          <h1>Autoriser {{ client }} ?</h1>
          <p class="aide">{{ client }} pourra agir dans ton compte Microdash ({{ session.user?.email }}) quand tu le lui demandes.</p>
        </div>
        <div class="pile-serree">
          <h2>Ce que {{ client }} pourra faire</h2>
          <ul class="droits">
            <li v-for="p in peut" :key="p"><Check v-bind="icone" class="oui" />{{ p }}</li>
          </ul>
        </div>
        <div class="pile-serree">
          <h2>Ce qu'il ne pourra pas faire</h2>
          <ul class="droits">
            <li v-for="p in nePeutPas" :key="p"><X v-bind="icone" class="non" />{{ p }}</li>
          </ul>
        </div>
        <p class="aide">Tu peux couper cet accès à tout moment dans Compte.</p>
        <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
        <div class="boutons">
          <button class="bouton secondaire" :disabled="envoi" @click="repondre(false)">Refuser</button>
          <button class="bouton principal" :disabled="envoi" @click="repondre(true)">Autoriser</button>
        </div>
      </template>
      <template v-else-if="charge">
        <h1>Connexion impossible</h1>
        <p class="erreur" role="alert">{{ erreur }}</p>
        <RouterLink to="/ce-soir">Retour à Microdash</RouterLink>
      </template>
    </div>
  </section>
</template>

<style scoped>
.cadre { max-width: 460px; margin: var(--e7) auto 0; padding: 0 var(--e4); }
.pile-serree { display: grid; gap: var(--e2); }
h2 { font-size: 16px; }
.droits { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--e3); }
.droits li { display: flex; gap: var(--e3); align-items: flex-start; }
.droits svg { flex-shrink: 0; margin-top: 2px; }
.oui { color: var(--sarcelle); }
.non { color: var(--brique); }
.boutons { display: grid; grid-template-columns: 1fr 1fr; gap: var(--e3); }
</style>
