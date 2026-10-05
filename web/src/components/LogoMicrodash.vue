<script setup>
// Le signe Microdash : un « m » blanc sur un carré safran. Dessiné en traits, il ne dépend d'aucune police.
defineProps({ avecNom: { type: Boolean, default: true } })
</script>

<template>
  <span class="logo">
    <svg width="28" height="28" viewBox="0 0 32 32" aria-hidden="true">
      <rect width="32" height="32" rx="6" fill="var(--safran)" />
      <path d="M9 22v-7.5a3.5 3.5 0 0 1 7 0V22m0-7.5a3.5 3.5 0 0 1 7 0V22" fill="none" stroke="var(--blanc)" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
    <span v-if="avecNom" class="nom">Microdash</span>
  </span>
</template>

<style scoped>
.logo { display: inline-flex; align-items: center; gap: var(--e2); }
.nom { font: 700 19px var(--police-titre); color: var(--encre); letter-spacing: -0.01em; }
</style>
