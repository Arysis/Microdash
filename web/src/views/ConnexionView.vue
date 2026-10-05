<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import { session } from '../session.js'

const props = defineProps({ mode: { type: String, required: true } })
const router = useRouter()
const email = ref('')
const motDePasse = ref('')
const erreur = ref('')
const envoi = ref(false)

async function valider() {
  erreur.value = ''
  envoi.value = true
  try {
    const chemin = props.mode === 'connexion' ? '/auth/login' : '/auth/register'
    session.user = await api.post(chemin, { email: email.value, mot_de_passe: motDePasse.value })
    router.push(session.user.profil_complet ? '/tableau-de-bord' : '/profil')
  } catch (e) {
    erreur.value = e.message
  } finally {
    envoi.value = false
  }
}
</script>

<template>
  <section class="cadre">
    <div class="carte pile">
      <div class="pile-serree">
        <h1>{{ mode === 'connexion' ? 'Connexion' : 'Créer ton compte' }}</h1>
        <p v-if="mode === 'inscription'" class="aide">Gratuit. Il te faut seulement une adresse e-mail.</p>
      </div>
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
        <p v-if="mode === 'inscription'" class="aide">10 caractères au moins.</p>
        <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
        <button class="bouton principal large" :disabled="envoi">
          {{ mode === 'connexion' ? 'Se connecter' : 'Créer mon compte' }}
        </button>
      </form>
      <p class="bascule-mode">
        <template v-if="mode === 'connexion'">Pas encore de compte ? <RouterLink to="/inscription">Crée-le ici</RouterLink></template>
        <template v-else>Tu as déjà un compte ? <RouterLink to="/connexion">Connecte-toi</RouterLink></template>
      </p>
    </div>
  </section>
</template>

<style scoped>
.cadre { max-width: 420px; margin: var(--e7) auto 0; padding: 0 var(--e4); }
.pile-serree { display: grid; gap: var(--e2); }
.bascule-mode { font-size: 15px; color: var(--pierre); }
</style>
