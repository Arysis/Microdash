<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { LogOut, Trash2, ChevronRight } from 'lucide-vue-next'
import { session, deconnecter, supprimerCompte } from '../session.js'
import { demanderConfirmation } from '../confirmation.js'

const router = useRouter()
const erreur = ref('')
const icone = { size: 20, 'stroke-width': 1.75, 'aria-hidden': 'true' }

async function quitter() {
  erreur.value = ''
  try {
    await deconnecter()
    router.push('/connexion')
  } catch (e) {
    erreur.value = e.message
  }
}

async function supprimer() {
  const ok = await demanderConfirmation({
    titre: 'Supprimer ton compte ?',
    message: 'Ton profil et toutes tes saisies sont effacés tout de suite. On ne peut pas les récupérer.',
    action: 'Supprimer mon compte',
  })
  if (!ok) return
  erreur.value = ''
  try {
    await supprimerCompte()
    router.push('/')
  } catch (e) {
    erreur.value = e.message
  }
}
</script>

<template>
  <h1>Compte</h1>

  <section class="carte pile">
    <div>
      <p class="aide">Adresse e-mail</p>
      <p>{{ session.user?.email }}</p>
    </div>
    <RouterLink class="ligne-lien" to="/profil">
      <span>
        Ton activité
        <small class="aide">Catégorie, ACRE, versement libératoire, périodicité</small>
      </span>
      <ChevronRight v-bind="icone" />
    </RouterLink>
  </section>

  <section class="carte pile">
    <button class="bouton secondaire" @click="quitter"><LogOut v-bind="icone" />Se déconnecter</button>
  </section>

  <section class="carte pile">
    <h2>Supprimer ton compte</h2>
    <p class="aide">Ton profil et toutes tes saisies sont effacés de nos serveurs. Pense à les noter ailleurs avant.</p>
    <button class="bouton secondaire danger" @click="supprimer"><Trash2 v-bind="icone" />Supprimer mon compte</button>
  </section>

  <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>

  <p class="liens-legaux aide">
    <RouterLink to="/confidentialite">Confidentialité</RouterLink>
    <RouterLink to="/mentions-legales">Mentions légales</RouterLink>
  </p>
</template>

<style scoped>
h1 { margin-bottom: var(--e5); }
.ligne-lien {
  display: flex; justify-content: space-between; align-items: center; gap: var(--e3);
  min-height: 44px; padding-top: var(--e4); border-top: 1px solid var(--lin);
  color: var(--encre); text-decoration: none; font-weight: 500;
}
.ligne-lien small { display: block; font-weight: 400; }
.ligne-lien:hover { color: var(--safran); }
.liens-legaux { display: flex; gap: var(--e5); margin-top: var(--e5); }
.liens-legaux a { color: var(--pierre); }
</style>
