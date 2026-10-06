<script setup>
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api.js'

const route = useRoute()
const etat = ref('attente') // attente, fait, erreur
const message = ref('')

// La page confirme d'un clic : un lien ouvert par un antivirus ou un aperçu ne désinscrit personne.
async function desinscrire() {
  etat.value = 'envoi'
  try {
    await api.post('/alertes/desinscription', { jeton: String(route.query.jeton || '') })
    etat.value = 'fait'
  } catch (e) {
    etat.value = 'erreur'
    message.value = e.message
  }
}

onMounted(() => {
  if (!route.query.jeton) {
    etat.value = 'erreur'
    message.value = 'Ce lien est incomplet. Ouvre celui de ton dernier e-mail, ou coupe les rappels dans ton compte.'
  }
})
</script>

<template>
  <article class="texte-legal largeur-lecture">
    <h1>Rappels par e-mail</h1>
    <template v-if="etat === 'fait'">
      <p class="succes" role="status">C'est fait : tu ne recevras plus aucun rappel de Microdash.</p>
      <p>Tu peux les réactiver un par un dans ton compte, écran Compte.</p>
    </template>
    <template v-else-if="etat === 'erreur'">
      <p class="erreur" role="alert">{{ message }}</p>
      <p><RouterLink to="/connexion">Se connecter pour gérer tes rappels</RouterLink></p>
    </template>
    <template v-else>
      <p>Tu ne recevras plus aucun rappel par e-mail : ni échéances, ni plafonds, ni CFE.</p>
      <div><button class="bouton principal" :disabled="etat === 'envoi'" @click="desinscrire">Ne plus recevoir de rappels</button></div>
    </template>
  </article>
</template>
