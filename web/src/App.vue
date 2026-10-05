<script setup>
import { useRouter } from 'vue-router'
import { session, deconnecter } from './session.js'

const router = useRouter()

async function sortir() {
  await deconnecter()
  router.push('/connexion')
}
</script>

<template>
  <header class="entete">
    <span class="marque">Microdash</span>
    <button v-if="session.user" class="lien" @click="sortir">Se déconnecter</button>
  </header>
  <main class="contenu">
    <RouterView />
  </main>
  <nav v-if="session.user?.profil_complet" class="onglets">
    <RouterLink to="/">Tableau de bord</RouterLink>
    <RouterLink to="/transactions">Saisies</RouterLink>
    <RouterLink to="/profil">Profil</RouterLink>
  </nav>
</template>
