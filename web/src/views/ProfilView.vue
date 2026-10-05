<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import { session } from '../session.js'

const router = useRouter()
const categories = ref([])
const erreur = ref('')
const message = ref('')
const envoi = ref(false)
const mixte = ref(false)
const profil = reactive({
  categorie: '',
  categorie_secondaire: '',
  nature_cfp: 'liberal',
  debut_activite: '',
  acre: false,
  versement_liberatoire: false,
  periodicite: 'trimestrielle',
})

const premiereFois = computed(() => !session.user?.profil_complet)

onMounted(async () => {
  const bareme = await api.get(`/baremes/${new Date().getFullYear()}`)
  categories.value = bareme.categories
  if (session.user?.profil_complet) {
    Object.assign(profil, await api.get('/profil'))
    mixte.value = profil.categorie_secondaire !== ''
  }
})

async function enregistrer() {
  erreur.value = ''
  message.value = ''
  envoi.value = true
  try {
    await api.put('/profil', { ...profil, categorie_secondaire: mixte.value ? profil.categorie_secondaire : '' })
    const etaitNouveau = premiereFois.value
    session.user.profil_complet = true
    if (etaitNouveau) router.push('/')
    else message.value = 'Profil enregistré.'
  } catch (e) {
    erreur.value = e.message
  } finally {
    envoi.value = false
  }
}
</script>

<template>
  <section class="carte">
    <h1>{{ premiereFois ? 'Ton activité' : 'Profil' }}</h1>
    <p v-if="premiereFois" class="aide">
      Ces informations servent à calculer tes cotisations, ton impôt et tes plafonds.
    </p>
    <form @submit.prevent="enregistrer">
      <label>
        Type d'activité
        <select v-model="profil.categorie" required>
          <option disabled value="">Choisis…</option>
          <option v-for="c in categories" :key="c.code" :value="c.code">{{ c.libelle }}</option>
        </select>
      </label>

      <label class="case"><input v-model="mixte" type="checkbox" /> Activité mixte (vente et services)</label>
      <label v-if="mixte">
        Seconde activité
        <select v-model="profil.categorie_secondaire" required>
          <option disabled value="">Choisis…</option>
          <option
            v-for="c in categories.filter((c) => c.code !== profil.categorie)"
            :key="c.code"
            :value="c.code"
          >{{ c.libelle }}</option>
        </select>
      </label>

      <label>
        Nature de l'activité (formation professionnelle)
        <select v-model="profil.nature_cfp" required>
          <option value="commercant">Commerçant</option>
          <option value="artisan">Artisan</option>
          <option value="liberal">Libéral</option>
        </select>
      </label>

      <label>Date de début d'activité <input v-model="profil.debut_activite" type="date" required /></label>

      <label>
        Périodicité des déclarations URSSAF
        <select v-model="profil.periodicite">
          <option value="mensuelle">Mensuelle</option>
          <option value="trimestrielle">Trimestrielle</option>
        </select>
      </label>

      <label class="case"><input v-model="profil.acre" type="checkbox" /> Je bénéficie de l'ACRE</label>
      <label class="case">
        <input v-model="profil.versement_liberatoire" type="checkbox" /> J'ai opté pour le versement libératoire de l'impôt
      </label>

      <p v-if="erreur" class="erreur">{{ erreur }}</p>
      <p v-if="message" class="succes">{{ message }}</p>
      <button class="principal" :disabled="envoi">{{ premiereFois ? 'Continuer' : 'Enregistrer' }}</button>
    </form>
  </section>
</template>
