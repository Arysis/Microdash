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
    if (etaitNouveau) router.push('/tableau-de-bord')
    else message.value = 'Profil enregistré.'
  } catch (e) {
    erreur.value = e.message
  } finally {
    envoi.value = false
  }
}
</script>

<template>
  <section class="carte pile">
    <h1>Ton activité</h1>
    <p v-if="premiereFois" class="aide">
      Tes réponses fixent tes taux de cotisation et tes plafonds. Tu pourras les changer plus tard.
    </p>
    <form @submit.prevent="enregistrer">
      <label>
        <span>Type d'activité <InfoBulle k="typeActivite" /></span>
        <select v-model="profil.categorie" required>
          <option disabled value="">Choisis…</option>
          <option v-for="c in categories" :key="c.code" :value="c.code">{{ c.libelle }}</option>
        </select>
      </label>

      <label class="case"><input v-model="mixte" type="checkbox" /> <span>Activité mixte (vente et services) <InfoBulle k="activiteMixte" /></span></label>
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
        <span>Nature de l'activité (formation professionnelle) <InfoBulle k="natureCFP" /></span>
        <select v-model="profil.nature_cfp" required>
          <option value="commercant">Commerçant</option>
          <option value="artisan">Artisan</option>
          <option value="liberal">Libéral</option>
        </select>
      </label>

      <label><span>Date de début d'activité <InfoBulle k="debutActivite" /></span> <input v-model="profil.debut_activite" type="date" required /></label>

      <label>
        <span>Périodicité des déclarations URSSAF <InfoBulle k="periodicite" /></span>
        <select v-model="profil.periodicite">
          <option value="mensuelle">Mensuelle</option>
          <option value="trimestrielle">Trimestrielle</option>
        </select>
      </label>

      <label class="case"><input v-model="profil.acre" type="checkbox" /> <span>Je bénéficie de l'ACRE <InfoBulle k="acre" /></span></label>
      <label class="case">
        <input v-model="profil.versement_liberatoire" type="checkbox" /> <span>J'ai opté pour le versement libératoire de l'impôt <InfoBulle k="versementLiberatoire" /></span>
      </label>

      <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
      <p v-if="message" class="succes">{{ message }}</p>
      <button class="bouton principal large" :disabled="envoi">{{ premiereFois ? 'Continuer' : 'Enregistrer' }}</button>
    </form>
  </section>
</template>
