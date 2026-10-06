<script setup>
import { nextTick, onBeforeUnmount, ref } from 'vue'
import { Info } from 'lucide-vue-next'
import { explications } from '../explications.js'

// Petit bouton « i » qui ouvre une explication courte sous lui. Les textes sont dans explications.js.
const props = defineProps({ k: { type: String, required: true } })
const e = explications[props.k]
if (!e) console.error(`explication inconnue : ${props.k}`)

const ouvert = ref(false)
const bouton = ref(null)
const bulle = ref(null)
const position = ref({})
const id = `info-${props.k}-${Math.random().toString(36).slice(2, 8)}`

// La bulle est placée sous le bouton, ou au-dessus s'il n'y a pas la place, sans sortir de l'écran
// ni passer sous la barre d'onglets. Elle suit le bouton quand la page défile.
function placer() {
  const r = bouton.value?.getBoundingClientRect()
  if (!r || !bulle.value) return
  const largeur = Math.min(320, window.innerWidth - 32)
  const gauche = Math.min(Math.max(16, r.left + r.width / 2 - largeur / 2), window.innerWidth - 16 - largeur)
  bulle.value.style.width = `${largeur}px`
  const hauteur = bulle.value.offsetHeight
  const onglets = document.querySelector('.onglets')?.getBoundingClientRect()
  const bas = Math.min(window.innerHeight, onglets && onglets.top > 0 ? onglets.top : window.innerHeight) - 8
  let haut = r.bottom + 6
  if (haut + hauteur > bas) haut = r.top - 6 - hauteur >= 8 ? r.top - 6 - hauteur : Math.max(8, bas - hauteur)
  position.value = { left: `${gauche}px`, top: `${haut}px`, width: `${largeur}px` }
}

function fermer() {
  ouvert.value = false
  document.removeEventListener('pointerdown', dehors, true)
  document.removeEventListener('keydown', echap)
  window.removeEventListener('scroll', placer, true)
  window.removeEventListener('resize', placer)
}
function dehors(ev) {
  if (!bulle.value?.contains(ev.target) && !bouton.value?.contains(ev.target)) fermer()
}
function echap(ev) {
  if (ev.key === 'Escape') {
    fermer()
    bouton.value?.focus()
  }
}
async function basculer() {
  if (ouvert.value) return fermer()
  ouvert.value = true
  await nextTick()
  placer()
  document.addEventListener('pointerdown', dehors, true)
  document.addEventListener('keydown', echap)
  window.addEventListener('scroll', placer, true)
  window.addEventListener('resize', placer)
}
onBeforeUnmount(fermer)
</script>

<template>
  <span v-if="e" class="info">
    <button
      ref="bouton"
      type="button"
      class="info-bouton"
      :aria-expanded="ouvert"
      :aria-controls="id"
      :aria-label="`Explication : ${e.titre}`"
      @click.stop.prevent="basculer"
    >
      <Info :size="16" :stroke-width="1.75" aria-hidden="true" />
    </button>
    <Teleport to="body">
      <span v-if="ouvert" :id="id" ref="bulle" class="info-bulle" role="note" :style="position">
        <strong>{{ e.titre }}</strong>
        <span v-for="(p, i) in e.texte" :key="i" class="info-para">{{ p }}</span>
        <a v-if="e.lien" :href="e.lien.url" target="_blank" rel="noopener">{{ e.lien.texte }}</a>
      </span>
    </Teleport>
  </span>
</template>

<style scoped>
.info { display: inline-flex; vertical-align: middle; }
/* La zone de clic fait 32 px pour le doigt ; l'icône reste discrète. */
.info-bouton {
  width: 32px; height: 32px; margin: -8px -6px -8px -2px; padding: 0; border: 0; background: none;
  display: inline-flex; align-items: center; justify-content: center; color: var(--pierre); cursor: pointer;
  border-radius: var(--rayon); transition: color 150ms ease-out;
}
.info-bouton:hover, .info-bouton[aria-expanded='true'] { color: var(--safran); }
.info-bulle {
  position: fixed; z-index: 50; display: grid; gap: var(--e2); padding: var(--e3) var(--e4);
  background: var(--blanc); color: var(--encre); border: 1px solid var(--lin); border-radius: var(--rayon);
  box-shadow: var(--ombre-carte); font-size: 14px; line-height: 1.45; font-weight: 400; text-align: left;
  white-space: normal; letter-spacing: 0;
}
.info-bulle strong { font-size: 14px; font-weight: 600; }
.info-bulle a { font-weight: 600; }
@media (prefers-reduced-motion: reduce) { .info-bouton { transition: none; } }
</style>
