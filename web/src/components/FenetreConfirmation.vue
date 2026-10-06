<script setup>
import { ref, watch } from 'vue'
import { confirmation, repondre } from '../confirmation.js'

const fenetre = ref(null)
watch(
  () => confirmation.ouverte,
  (ouverte) => {
    if (ouverte) fenetre.value?.showModal()
    else fenetre.value?.close()
  },
)
</script>

<template>
  <dialog ref="fenetre" class="confirmation" @cancel.prevent="repondre(false)">
    <h2>{{ confirmation.titre }}</h2>
    <p v-if="confirmation.message" class="message">{{ confirmation.message }}</p>
    <div class="actions">
      <button class="bouton secondaire" @click="repondre(false)">Annuler</button>
      <button class="bouton principal" @click="repondre(true)">{{ confirmation.action }}</button>
    </div>
  </dialog>
</template>

<style scoped>
.message { margin-top: var(--e2); color: var(--pierre); }
.actions { display: flex; justify-content: flex-end; gap: var(--e3); margin-top: var(--e5); }
</style>
