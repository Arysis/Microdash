<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api.js'
import { session } from '../session.js'

const modules = ref(null)
const erreur = ref('')

// Modules annoncés mais pas encore livrés : affichés sans interrupteur.
const enPreparation = ['Mails', 'Support', 'CRM']

const aides = {
  finances: 'Échéances URSSAF, plafonds et opérations Qonto à valider dans « Ce soir ». Onglet Finances.',
  agenda: 'Tes rendez-vous, avec les tâches datées et les échéances. Lien iCal pour Home Assistant.',
  taches: 'Tes listes Ce soir, Cette semaine et Plus tard, par projet. Les tâches arrivées à échéance passent dans « Ce soir ».',
}

onMounted(async () => {
  try {
    modules.value = await api.get('/modules')
  } catch (e) {
    erreur.value = e.message
  }
})

async function changer(m, actif) {
  erreur.value = ''
  const avant = m.actif
  m.actif = actif
  try {
    await api.put(`/modules/${m.code}`, { actif })
    session.modules = modules.value.map((x) => ({ ...x }))
  } catch (e) {
    m.actif = avant
    erreur.value = e.message
  }
}
</script>

<template>
  <h1>Réglages</h1>

  <section class="carte" aria-labelledby="titre-modules">
    <h2 id="titre-modules">Modules</h2>
    <p class="aide">Un module désactivé disparaît de « Ce soir » et du menu. Ses données restent.</p>
    <label v-for="m in modules || []" :key="m.code" class="interrupteur">
      <span>{{ m.nom }}<small class="aide">{{ aides[m.code] }}</small></span>
      <input type="checkbox" role="switch" :checked="m.actif" @change="changer(m, $event.target.checked)" />
    </label>
    <div v-for="nom in enPreparation" :key="nom" class="interrupteur prochain">
      <span>{{ nom }}</span>
      <small class="aide">En préparation</small>
    </div>
    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
  </section>
</template>

<style scoped>
h1 { margin-bottom: var(--e5); }
h2 { margin-bottom: var(--e1); }
.carte > .aide { margin-bottom: var(--e2); }
.interrupteur {
  display: flex; justify-content: space-between; align-items: center; gap: var(--e3);
  min-height: 52px; padding: var(--e2) 0; border-bottom: 1px solid var(--lin);
  font-weight: 500;
}
.interrupteur span { display: grid; gap: 2px; }
.interrupteur small { font-weight: 400; }
.interrupteur input { width: 20px; height: 20px; margin: 0; flex-shrink: 0; accent-color: var(--safran); }
.prochain { color: var(--pierre); }
.prochain:last-of-type { border-bottom: 0; }
</style>
