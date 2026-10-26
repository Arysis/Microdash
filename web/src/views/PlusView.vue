<script setup>
import { computed } from 'vue'
import { ChevronRight } from 'lucide-vue-next'
import { moduleActif } from '../session.js'

const icone = { size: 20, 'stroke-width': 1.75, 'aria-hidden': 'true' }
const liens = computed(() => [
  ...(moduleActif('mails') ? [{ to: '/mails', titre: 'Mails', aide: 'Importants et les autres, de ta boîte Gmail' }] : []),
  { to: '/reglages', titre: 'Réglages', aide: 'Modules activés, Gmail, ce qui est important' },
  { to: '/compte', titre: 'Compte', aide: 'Rappels par e-mail, exports, déconnexion' },
  { to: '/profil', titre: 'Ton activité', aide: 'Catégorie, ACRE, versement libératoire, périodicité' },
])
</script>

<template>
  <h1>Plus</h1>
  <nav class="carte" aria-label="Autres pages">
    <RouterLink v-for="l in liens" :key="l.to" class="ligne-lien" :to="l.to">
      <span>{{ l.titre }}<small class="aide">{{ l.aide }}</small></span>
      <ChevronRight v-bind="icone" />
    </RouterLink>
  </nav>
</template>

<style scoped>
h1 { margin-bottom: var(--e5); }
.carte { padding-top: var(--e2); padding-bottom: var(--e2); }
.ligne-lien {
  display: flex; justify-content: space-between; align-items: center; gap: var(--e3);
  min-height: 56px; padding: var(--e2) 0; border-bottom: 1px solid var(--lin);
  color: var(--encre); text-decoration: none; font-weight: 500;
  transition: color var(--duree) ease-out;
}
.ligne-lien:last-child { border-bottom: 0; }
.ligne-lien span { display: grid; gap: 2px; }
.ligne-lien small { font-weight: 400; }
.ligne-lien:hover { color: var(--safran); }
</style>
