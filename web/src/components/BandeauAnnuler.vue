<script setup>
// Bandeau « Faite : … Annuler », affiché quelques secondes après la rature.
defineProps({ tache: { type: Object, default: null } })
defineEmits(['annuler'])
</script>

<template>
  <div class="bandeau-annuler" role="status" aria-live="polite">
    <template v-if="tache">
      <span class="texte">Faite : « {{ tache.titre }} »</span>
      <button type="button" class="annuler" @click="$emit('annuler')">Annuler</button>
    </template>
  </div>
</template>

<style scoped>
.bandeau-annuler {
  position: fixed; z-index: 3; left: 50%; transform: translateX(-50%);
  bottom: calc(72px + env(safe-area-inset-bottom));
  width: min(608px, calc(100% - 2 * var(--e4)));
  display: flex; align-items: center; gap: var(--e3);
  pointer-events: none;
}
.bandeau-annuler:not(:empty) {
  padding: var(--e2) var(--e2) var(--e2) var(--e4);
  background: var(--encre); color: var(--blanc); border-radius: var(--rayon); box-shadow: var(--ombre);
  pointer-events: auto;
}
.texte { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.annuler {
  flex-shrink: 0; min-height: 44px; padding: 0 var(--e3);
  background: none; border: 0; border-radius: var(--rayon); font: 600 15px var(--police-texte); color: var(--blanc); cursor: pointer;
  transition: background var(--duree) ease-out;
}
.annuler:hover { background: var(--sur-sarcelle); }
</style>
