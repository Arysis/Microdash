<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import { session } from '../session.js'

const router = useRouter()
const mode = ref('connexion')
const email = ref('')
const motDePasse = ref('')
const erreur = ref('')
const envoi = ref(false)

async function valider() {
  erreur.value = ''
  envoi.value = true
  try {
    const chemin = mode.value === 'connexion' ? '/auth/login' : '/auth/register'
    session.user = await api.post(chemin, { email: email.value, mot_de_passe: motDePasse.value })
    router.push(session.user.profil_complet ? '/' : '/profil')
  } catch (e) {
    erreur.value = e.message
  } finally {
    envoi.value = false
  }
}
</script>

<template>
  <section class="carte etroite">
    <h1>{{ mode === 'connexion' ? 'Connexion' : 'Créer un compte' }}</h1>
    <form @submit.prevent="valider">
      <label>E-mail <input v-model="email" type="email" autocomplete="email" required /></label>
      <label>
        Mot de passe
        <input
          v-model="motDePasse"
          type="password"
          :autocomplete="mode === 'connexion' ? 'current-password' : 'new-password'"
          :minlength="mode === 'connexion' ? undefined : 10"
          required
        />
      </label>
      <p v-if="mode === 'inscription'" class="aide">10 caractères minimum.</p>
      <p v-if="erreur" class="erreur">{{ erreur }}</p>
      <button class="principal" :disabled="envoi">
        {{ mode === 'connexion' ? 'Se connecter' : 'Créer mon compte' }}
      </button>
    </form>
    <button class="lien" @click="mode = mode === 'connexion' ? 'inscription' : 'connexion'">
      {{ mode === 'connexion' ? 'Pas encore de compte ? Inscris-toi' : 'Déjà un compte ? Connecte-toi' }}
    </button>
  </section>
</template>
