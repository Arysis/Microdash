<script setup>
import { session } from '../session.js'
</script>

<template>
  <section class="introuvable largeur-lecture">
    <p class="aide">Erreur 404</p>
    <h1>Cette page n'existe pas.</h1>
    <p>Le lien est peut-être ancien, ou l'adresse contient une faute de frappe.</p>
    <RouterLink v-if="session.user" class="bouton principal" to="/tableau-de-bord">Retour au tableau de bord</RouterLink>
    <RouterLink v-else class="bouton principal" to="/">Retour à l'accueil</RouterLink>
  </section>
</template>

<style scoped>
.introuvable { display: grid; gap: var(--e4); justify-items: start; padding-top: var(--e8); }
</style>
