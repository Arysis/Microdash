<script setup>
// Barres du solde de chaque mois (entrées moins sorties) : vers le haut en sarcelle quand il reste
// de l'argent, vers le bas en safran quand il en part plus qu'il n'en entre. Les mois prévus sont hachurés.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { formatEuros } from '../format.js'

const props = defineProps({ mois: { type: Array, required: true } })

const boite = ref(null)
const largeur = ref(320)
let observateur
onMounted(() => {
  observateur = new ResizeObserver(([e]) => { largeur.value = Math.max(240, Math.round(e.contentRect.width)) })
  observateur.observe(boite.value)
})
onBeforeUnmount(() => observateur?.disconnect())

const HAUTEUR = 220
const HAUT = 12
const BAS = 40 // libellés des mois et des années
const GAUCHE = 48 // libellés de l'axe

const compact = new Intl.NumberFormat('fr-FR', { notation: 'compact', style: 'currency', currency: 'EUR', maximumFractionDigits: 1 })
const initiales = ['J', 'F', 'M', 'A', 'M', 'J', 'J', 'A', 'S', 'O', 'N', 'D']
const courts = ['janv.', 'févr.', 'mars', 'avr.', 'mai', 'juin', 'juil.', 'août', 'sept.', 'oct.', 'nov.', 'déc.']

// Graduation « ronde » (1, 2 ou 5 × 10^n euros) couvrant les valeurs et zéro.
const echelle = computed(() => {
  const flux = props.mois.map((m) => m.flux / 100)
  let max = Math.max(0, ...flux)
  let min = Math.min(0, ...flux)
  if (max === min) max = 100
  const brut = (max - min) / 4
  const p = 10 ** Math.floor(Math.log10(brut))
  const pas = [1, 2, 5, 10].map((k) => k * p).find((v) => v >= brut)
  const haut = Math.ceil(max / pas) * pas
  const bas = Math.floor(min / pas) * pas
  const graduations = []
  for (let v = bas; v <= haut + pas / 2; v += pas) graduations.push(Math.round(v))
  const y = (euros) => HAUT + ((haut - euros) / (haut - bas)) * (HAUTEUR - HAUT - BAS)
  return { y, graduations }
})

const geo = computed(() => {
  const n = props.mois.length || 1
  const zone = largeur.value - GAUCHE - 4
  const fente = zone / n
  const barre = Math.max(4, Math.min(24, fente - 4))
  return { fente, barre, x: (i) => GAUCHE + i * fente + (fente - barre) / 2, centre: (i) => GAUCHE + i * fente + fente / 2 }
})

// Rectangle aux coins arrondis (4 px) du côté de la valeur, droit contre la ligne de zéro.
function chemin(i, m) {
  const { y } = echelle.value
  const { x, barre: w } = geo.value
  const x0 = x(i)
  const y0 = y(0)
  const y1 = y(m.flux / 100)
  const h = Math.abs(y1 - y0)
  if (h < 0.5) return ''
  const r = Math.min(4, h, w / 2)
  if (y1 < y0) {
    return `M${x0},${y0}V${y1 + r}Q${x0},${y1} ${x0 + r},${y1}H${x0 + w - r}Q${x0 + w},${y1} ${x0 + w},${y1 + r}V${y0}Z`
  }
  return `M${x0},${y0}V${y1 - r}Q${x0},${y1} ${x0 + r},${y1}H${x0 + w - r}Q${x0 + w},${y1} ${x0 + w},${y1 - r}V${y0}Z`
}

const moisDe = (m) => Number(m.mois.slice(5, 7)) - 1
const libelleX = (m) => (geo.value.fente >= 34 ? courts[moisDe(m)] : initiales[moisDe(m)])
// L'année sous chaque janvier, et sous le premier mois s'il reste la place avant le janvier suivant.
const anneeX = (m, i) => {
  if (moisDe(m) === 0) return m.mois.slice(0, 4)
  return i === 0 && (12 - moisDe(m)) * geo.value.fente >= 36 ? m.mois.slice(0, 4) : ''
}
const remplissage = (m) => {
  const neg = m.flux < 0
  if (m.prevision) return neg ? 'url(#hachure-negatif)' : 'url(#hachure-positif)'
  return neg ? 'var(--safran)' : 'var(--sarcelle)'
}

const indexCourant = computed(() => Math.max(0, props.mois.findIndex((m) => m.en_cours)))
const choisi = ref(null)
watch(() => props.mois, () => { choisi.value = null })
const actif = computed(() => props.mois[choisi.value ?? indexCourant.value])
const signe = (c) => `${c < 0 ? '−' : '+'} ${formatEuros(Math.abs(c))}`
const majuscule = (s) => s.charAt(0).toUpperCase() + s.slice(1)
const etat = (m) => (m.prevision ? 'prévision' : m.en_cours ? 'en cours' : 'réel')
const resume = (m) => `${majuscule(m.libelle)}, ${etat(m)} : ${signe(m.flux)}`

function clavier(e, i) {
  const n = props.mois.length
  if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
    e.preventDefault()
    const j = (i + (e.key === 'ArrowRight' ? 1 : -1) + n) % n
    choisi.value = j
    boite.value.querySelectorAll('.cible')[j]?.focus()
  }
}
</script>

<template>
  <figure class="graphe">
    <figcaption class="legende">
      <span><i class="pastille" style="background: var(--sarcelle)"></i>Il en reste</span>
      <span><i class="pastille" style="background: var(--safran)"></i>Il en part plus</span>
      <span><i class="pastille hachure"></i>Prévision</span>
    </figcaption>

    <div ref="boite" class="zone">
      <svg :width="largeur" :height="HAUTEUR" :viewBox="`0 0 ${largeur} ${HAUTEUR}`" role="group" aria-label="Solde de chaque mois, entrées moins sorties">
        <defs>
          <pattern id="hachure-positif" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
            <rect width="6" height="6" fill="var(--sarcelle-clair)" />
            <rect width="2.5" height="6" fill="var(--sarcelle)" />
          </pattern>
          <pattern id="hachure-negatif" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(135)">
            <rect width="6" height="6" fill="var(--safran-clair)" />
            <rect width="2.5" height="6" fill="var(--safran)" />
          </pattern>
        </defs>

        <g class="axe" aria-hidden="true">
          <template v-for="g in echelle.graduations" :key="g">
            <line :x1="GAUCHE" :x2="largeur" :y1="echelle.y(g)" :y2="echelle.y(g)" :class="{ zero: g === 0 }" />
            <text :x="GAUCHE - 6" :y="echelle.y(g) + 4" text-anchor="end">{{ g === 0 ? '0' : compact.format(g).replace('-', '−') }}</text>
          </template>
        </g>

        <rect
          v-if="actif"
          class="surlignage"
          :x="GAUCHE + (choisi ?? indexCourant) * geo.fente"
          :y="HAUT - 4"
          :width="geo.fente"
          :height="HAUTEUR - HAUT - BAS + 8"
          rx="4"
          aria-hidden="true"
        />

        <path v-for="(m, i) in mois" :key="m.mois" :d="chemin(i, m)" :fill="remplissage(m)" aria-hidden="true" />

        <g class="mois" aria-hidden="true">
          <template v-for="(m, i) in mois" :key="m.mois">
            <text :x="geo.centre(i)" :y="HAUTEUR - BAS + 16" text-anchor="middle" :class="{ courant: m.en_cours }">{{ libelleX(m) }}</text>
            <text v-if="anneeX(m, i)" :x="geo.x(i)" :y="HAUTEUR - 6" class="annee">{{ anneeX(m, i) }}</text>
          </template>
        </g>

        <rect
          v-for="(m, i) in mois"
          :key="`c-${m.mois}`"
          class="cible"
          :x="GAUCHE + i * geo.fente"
          y="0"
          :width="geo.fente"
          :height="HAUTEUR"
          :tabindex="i === (choisi ?? indexCourant) ? 0 : -1"
          role="img"
          :aria-label="resume(m)"
          @mouseenter="choisi = i"
          @click="choisi = i"
          @focus="choisi = i"
          @keydown="clavier($event, i)"
        />
      </svg>
    </div>

    <div v-if="actif" class="detail" aria-live="polite">
      <div class="titre-detail">
        <strong>{{ majuscule(actif.libelle) }}</strong>
        <span :class="['etat', etat(actif).replace(' ', '-')]">{{ majuscule(etat(actif)) }}</span>
      </div>
      <dl class="chiffres">
        <div><dt>Encaissé</dt><dd>{{ formatEuros(actif.encaisse) }}</dd></div>
        <div><dt>Dépenses</dt><dd>− {{ formatEuros(actif.depenses) }}</dd></div>
        <div><dt>Paiements URSSAF</dt><dd>− {{ formatEuros(actif.cotisations) }}</dd></div>
        <div class="total"><dt>Solde du mois</dt><dd>{{ signe(actif.flux) }}</dd></div>
        <div v-if="actif.solde_fin !== undefined"><dt>Sur le compte en fin de mois</dt><dd>{{ formatEuros(actif.solde_fin) }}</dd></div>
      </dl>
    </div>

    <details class="tableau">
      <summary>Voir en tableau</summary>
      <div class="defile">
        <table>
          <thead>
            <tr><th scope="col">Mois</th><th scope="col">Encaissé</th><th scope="col">Dépenses</th><th scope="col">URSSAF</th><th scope="col">Solde du mois</th><th scope="col">Fin de mois</th></tr>
          </thead>
          <tbody>
            <tr v-for="m in mois" :key="m.mois" :class="{ prevision: m.prevision }">
              <th scope="row">{{ majuscule(m.libelle) }}<small v-if="m.prevision || m.en_cours"> · {{ etat(m) }}</small></th>
              <td>{{ formatEuros(m.encaisse) }}</td>
              <td>{{ formatEuros(m.depenses) }}</td>
              <td>{{ formatEuros(m.cotisations) }}</td>
              <td>{{ signe(m.flux) }}</td>
              <td>{{ m.solde_fin !== undefined ? formatEuros(m.solde_fin) : '·' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </details>
  </figure>
</template>

<style scoped>
.graphe { margin: 0; display: grid; gap: var(--e3); }
.legende { display: flex; flex-wrap: wrap; gap: var(--e1) var(--e4); font-size: 13px; color: var(--pierre); }
.legende span { display: inline-flex; align-items: center; gap: var(--e2); }
.hachure { background: repeating-linear-gradient(45deg, var(--sarcelle) 0 2px, var(--sarcelle-clair) 2px 5px); }
.zone { width: 100%; overflow: hidden; }
svg { display: block; touch-action: pan-y; }
.axe line { stroke: var(--lin); stroke-width: 1; }
.axe line.zero { stroke: var(--pierre); }
.axe text, .mois text { font: 500 11px var(--police-texte); fill: var(--pierre); font-variant-numeric: tabular-nums; }
.mois text.courant { fill: var(--encre); font-weight: 700; }
.mois text.annee { font-weight: 600; }
.surlignage { fill: var(--papier); stroke: var(--lin); }
.cible { fill: transparent; cursor: pointer; outline: none; }
.cible:focus-visible { stroke: var(--safran); stroke-width: 2; }

.detail { background: var(--papier); border-radius: var(--rayon); padding: var(--e3) var(--e4); display: grid; gap: var(--e2); }
.titre-detail { display: flex; justify-content: space-between; align-items: center; gap: var(--e3); }
.etat { font-size: 12px; font-weight: 600; padding: 1px var(--e2); border-radius: var(--rayon); background: var(--lin); color: var(--encre); }
.etat.prevision { background: var(--sarcelle-clair); color: var(--sarcelle); }
.etat.en-cours { background: var(--safran-clair); color: var(--safran-fonce); }
.detail .chiffres { gap: var(--e1); font-size: 15px; }
.detail .chiffres div { border-bottom: 0; padding-bottom: 0; }
.detail .total { border-top: 1px solid var(--lin); padding-top: var(--e2); font-weight: 600; }
.detail .total dt { color: var(--encre); }

.tableau summary { cursor: pointer; color: var(--safran); font-weight: 500; font-size: 15px; min-height: 44px; display: flex; align-items: center; }
.defile { overflow-x: auto; }
table { border-collapse: collapse; width: 100%; font-size: 13px; font-variant-numeric: tabular-nums; }
th, td { padding: var(--e2); text-align: right; white-space: nowrap; border-bottom: 1px solid var(--lin); }
th:first-child { text-align: left; }
thead th { color: var(--pierre); font-weight: 600; }
tbody th { font-weight: 500; }
tbody th small { color: var(--pierre); }
tr.prevision { color: var(--pierre); }
</style>
