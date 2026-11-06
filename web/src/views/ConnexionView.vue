<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import { session } from '../session.js'

// mode : connexion, inscription ou oubli (nouveau mot de passe sans e-mail, tant que le serveur l'autorise).
const props = defineProps({ mode: { type: String, required: true } })
const router = useRouter()
const email = ref('')
const motDePasse = ref('')
const confirmation = ref('')
const erreur = ref('')
const envoi = ref(false)
const oubliActif = ref(false)

const nouveau = computed(() => props.mode !== 'connexion')
const textes = {
  connexion: { titre: 'Connexion', bouton: 'Se connecter', chemin: '/auth/login' },
  inscription: { titre: 'Créer ton compte', bouton: 'Créer mon compte', chemin: '/auth/register' },
  oubli: { titre: 'Mot de passe oublié', bouton: 'Changer mon mot de passe', chemin: '/auth/mot-de-passe-oublie' },
}
const texte = computed(() => textes[props.mode])

onMounted(async () => {
  if (props.mode === 'inscription') return
  try {
    oubliActif.value = (await api.get('/auth/mot-de-passe-oublie')).actif
  } catch {
    oubliActif.value = false
  }
})

async function valider() {
  erreur.value = ''
  if (nouveau.value && motDePasse.value !== confirmation.value) {
    erreur.value = 'Les deux mots de passe ne sont pas identiques.'
    return
  }
  envoi.value = true
  try {
    session.user = await api.post(texte.value.chemin, { email: email.value, mot_de_passe: motDePasse.value })
    router.push(session.user.profil_complet ? '/ce-soir' : '/profil')
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
        <h1>{{ texte.titre }}</h1>
        <p v-if="mode === 'inscription'" class="aide">Gratuit. Il te faut seulement une adresse e-mail.</p>
        <p v-else-if="mode === 'oubli'" class="aide">Indique l'e-mail de ton compte et choisis un nouveau mot de passe. Tes autres appareils seront déconnectés.</p>
      </div>
      <p v-if="mode === 'oubli' && !oubliActif" class="aide">
        La réinitialisation du mot de passe n'est pas ouverte pour le moment. Contacte l'administrateur du site.
      </p>
      <form v-else @submit.prevent="valider">
        <label>E-mail <input v-model="email" type="email" autocomplete="email" required /></label>
        <label>
          {{ mode === 'oubli' ? 'Nouveau mot de passe' : 'Mot de passe' }}
          <input
            v-model="motDePasse"
            type="password"
            :autocomplete="nouveau ? 'new-password' : 'current-password'"
            :minlength="nouveau ? 10 : undefined"
            required
          />
        </label>
        <label v-if="nouveau">
          Confirme le mot de passe
          <input v-model="confirmation" type="password" autocomplete="new-password" minlength="10" required />
        </label>
        <p v-if="nouveau" class="aide">10 caractères au moins.</p>
        <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
        <button class="bouton principal large" :disabled="envoi">{{ texte.bouton }}</button>
      </form>
      <p v-if="mode === 'connexion' && oubliActif" class="bascule-mode">
        <RouterLink to="/mot-de-passe-oublie">Mot de passe oublié ?</RouterLink>
      </p>
      <p class="bascule-mode">
        <template v-if="mode === 'inscription'">Tu as déjà un compte ? <RouterLink to="/connexion">Connecte-toi</RouterLink></template>
        <template v-else-if="mode === 'oubli'">Ça te revient ? <RouterLink to="/connexion">Connecte-toi</RouterLink></template>
        <template v-else>Pas encore de compte ? <RouterLink to="/inscription">Crée-le ici</RouterLink></template>
      </p>
    </div>
  </section>
</template>

<style scoped>
.cadre { max-width: 420px; margin: var(--e7) auto 0; padding: 0 var(--e4); }
.pile-serree { display: grid; gap: var(--e2); }
.bascule-mode { font-size: 15px; color: var(--pierre); }
</style>
