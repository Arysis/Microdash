<script setup>
// Deux vues. « mois » : le solde de chaque mois (entrées moins sorties), vers le haut en sarcelle
// quand il reste de l'argent, vers le bas en safran quand il en part plus qu'il n'en entre.
// « cumul » : ce qu'il y a sur le compte en fin de mois, en barre empilée : en bleu ce qui reste
// du mois d'avant, en sarcelle ce que le mois a ajouté, en pointillés safran ce qu'il a retiré.
// Les mois prévus sont hachurés.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { formatEuros } from '../format.js'

const props = defineProps({
  mois: { type: Array, required: true },
  vue: { type: String, default: 'mois' }, // mois ou cumul
})
const cumul = computed(() => props.vue === 'cumul')

// Début et fin de chaque mois sur le compte, en centimes. Avec un solde noté, l'API donne la fin
// de chaque mois ; sans lui, le cumul part de 0 au début du premier mois affiché.
const sansSolde = computed(() => props.mois.some((m) => m.solde_fin === undefined || m.solde_fin === null))
const bornes = computed(() => {
  let fin = 0
  return props.mois.map((m) => {
    fin = sansSolde.value ? fin + m.flux : m.solde_fin
    return { debut: fin - m.flux, fin }
  })
})

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
  const flux = cumul.value
    ? bornes.value.flatMap((b) => [b.debut / 100, b.fin / 100])
    : props.mois.map((m) => m.flux / 100)
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

// Rectangle de la barre i entre deux ordonnées : y0 est le côté droit (vers zéro ou contre le
// segment voisin), y1 le côté arrondi (4 px) quand arrondi est vrai.
function rectangle(i, y0, y1, arrondi = true) {
  const { x, barre: w } = geo.value
  const x0 = x(i)
  const h = Math.abs(y1 - y0)
  if (h < 0.5) return ''
  const r = arrondi ? Math.min(4, h, w / 2) : 0
  if (y1 < y0) {
    return `M${x0},${y0}V${y1 + r}Q${x0},${y1} ${x0 + r},${y1}H${x0 + w - r}Q${x0 + w},${y1} ${x0 + w},${y1 + r}V${y0}Z`
  }
  return `M${x0},${y0}V${y1 - r}Q${x0},${y1} ${x0 + r},${y1}H${x0 + w - r}Q${x0 + w},${y1} ${x0 + w},${y1 - r}V${y0}Z`
}

// Vue mois : une barre du zéro au solde du mois.
const chemin = (i, m) => rectangle(i, echelle.value.y(0), echelle.value.y(m.flux / 100))

// Vue cumul. Le bleu va de zéro à ce qui reste du mois d'avant (la partie commune au début et
// à la fin du mois, du même côté de zéro). Au-delà, jusqu'à la fin du mois : ce qui s'est ajouté,
// en plein (sarcelle au-dessus de zéro, safran sous zéro, où il creuse le découvert). Jusqu'au
// début du mois : ce qui a disparu, en pointillés (safran au-dessus de zéro, sarcelle sous zéro,
// où c'est du découvert remboursé). 2 px séparent le bleu de l'ajout.
function segments(i) {
  const { y } = echelle.value
  const { debut: d, fin: f } = bornes.value[i]
  const k = d > 0 && f > 0 ? Math.min(d, f) : d < 0 && f < 0 ? Math.max(d, f) : 0
  const y0 = y(0)
  const yk = y(k / 100)
  const ecart = (v) => (k === 0 ? 0 : v > 0 ? -2 : 2) // vers le haut au-dessus de zéro
  const plein = Math.abs(f) > Math.abs(k) || Math.sign(f) !== Math.sign(k)
  const fantome = Math.abs(d) > Math.abs(k) || Math.sign(d) !== Math.sign(k)
  const fin = f !== 0 && plein && Math.abs(y(f / 100) - yk) >= 0.5
  return {
    reste: rectangle(i, y0, yk, !fin),
    plein: fin ? rectangle(i, yk + ecart(f), y(f / 100)) : '',
    pleinCouleur: f > 0 ? 'positif' : 'negatif',
    // Le pointillé suffit à le distinguer du bleu : pas d'écart, pour qu'un petit retrait reste visible.
    fantome: d !== 0 && fantome ? rectangle(i, yk, y(d / 100), false) : '',
    fantomeCouleur: d > 0 ? 'perdu' : 'rembourse',
  }
}
const decouvert = computed(() => cumul.value && bornes.value.some((b) => b.debut < 0 || b.fin < 0))

const moisDe = (m) => Number(m.mois.slice(5, 7)) - 1
const libelleX = (m) => (geo.value.fente >= 34 ? courts[moisDe(m)] : initiales[moisDe(m)])
// L'année sous chaque janvier, et sous le premier mois s'il reste la place avant le janvier suivant.
const anneeX = (m, i) => {
  if (moisDe(m) === 0) return m.mois.slice(0, 4)
  return i === 0 && (12 - moisDe(m)) * geo.value.fente >= 36 ? m.mois.slice(0, 4) : ''
}
const remplissageReste = (m) => (m.prevision ? 'url(#hachure-bleu)' : 'var(--bleu)')
const remplissagePlein = (m, c) =>
  c === 'positif'
    ? m.prevision ? 'url(#hachure-positif)' : 'var(--sarcelle)'
    : m.prevision ? 'url(#hachure-negatif)' : 'var(--safran)'
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
const resume = (m, i) =>
  cumul.value
    ? `${majuscule(m.libelle)}, ${etat(m)} : ${formatEuros(bornes.value[i].fin)} en fin de mois, ${signe(m.flux)} sur le mois`
    : `${majuscule(m.libelle)}, ${etat(m)} : ${signe(m.flux)}`
const indexActif = computed(() => choisi.value ?? indexCourant.value)
const premierMois = computed(() => props.mois[0]?.libelle || '')

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
    <figcaption v-if="cumul" class="legende">
      <span><i class="pastille" style="background: var(--bleu)"></i>Reste du mois d'avant</span>
      <span><i class="pastille" style="background: var(--sarcelle)"></i>Ajouté ce mois-ci</span>
      <span><i class="pastille fantome perdu"></i>Parti ce mois-ci</span>
      <template v-if="decouvert">
        <span><i class="pastille" style="background: var(--safran)"></i>Découvert en plus</span>
        <span><i class="pastille fantome rembourse"></i>Découvert remboursé</span>
      </template>
      <span><i class="pastille hachure"></i>Prévision</span>
    </figcaption>
    <figcaption v-else class="legende">
      <span><i class="pastille" style="background: var(--sarcelle)"></i>Il en reste</span>
      <span><i class="pastille" style="background: var(--safran)"></i>Il en part plus</span>
      <span><i class="pastille hachure"></i>Prévision</span>
    </figcaption>

    <div ref="boite" class="zone">
      <svg
        :width="largeur"
        :height="HAUTEUR"
        :viewBox="`0 0 ${largeur} ${HAUTEUR}`"
        role="group"
        :aria-label="cumul ? 'Argent sur le compte en fin de mois' : 'Solde de chaque mois, entrées moins sorties'"
      >
        <defs>
          <pattern id="hachure-positif" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
            <rect width="6" height="6" fill="var(--sarcelle-clair)" />
            <rect width="2.5" height="6" fill="var(--sarcelle)" />
          </pattern>
          <pattern id="hachure-negatif" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(135)">
            <rect width="6" height="6" fill="var(--safran-clair)" />
            <rect width="2.5" height="6" fill="var(--safran)" />
          </pattern>
          <pattern id="hachure-bleu" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
            <rect width="6" height="6" fill="var(--bleu-clair)" />
            <rect width="2.5" height="6" fill="var(--bleu)" />
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

        <template v-if="cumul">
          <g v-for="(m, i) in mois" :key="m.mois" aria-hidden="true">
            <path :d="segments(i).reste" :fill="remplissageReste(m)" />
            <path :d="segments(i).fantome" :class="['fantome', segments(i).fantomeCouleur]" />
            <path :d="segments(i).plein" :fill="remplissagePlein(m, segments(i).pleinCouleur)" />
          </g>
        </template>
        <template v-else>
          <path v-for="(m, i) in mois" :key="m.mois" :d="chemin(i, m)" :fill="remplissage(m)" aria-hidden="true" />
        </template>

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
          :aria-label="resume(m, i)"
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
      <dl v-if="cumul" class="chiffres">
        <div><dt>Sur le compte en début de mois</dt><dd>{{ formatEuros(bornes[indexActif].debut) }}</dd></div>
        <div><dt>Encaissé</dt><dd>+ {{ formatEuros(actif.encaisse) }}</dd></div>
        <div><dt>Dépenses</dt><dd>− {{ formatEuros(actif.depenses) }}</dd></div>
        <div><dt>Paiements URSSAF</dt><dd>− {{ formatEuros(actif.cotisations) }}</dd></div>
        <div class="total"><dt>Sur le compte en fin de mois</dt><dd>{{ formatEuros(bornes[indexActif].fin) }}</dd></div>
      </dl>
      <dl v-else class="chiffres">
        <div><dt>Encaissé</dt><dd>{{ formatEuros(actif.encaisse) }}</dd></div>
        <div><dt>Dépenses</dt><dd>− {{ formatEuros(actif.depenses) }}</dd></div>
        <div><dt>Paiements URSSAF</dt><dd>− {{ formatEuros(actif.cotisations) }}</dd></div>
        <div class="total"><dt>Solde du mois</dt><dd>{{ signe(actif.flux) }}</dd></div>
        <div v-if="actif.solde_fin !== undefined"><dt>Sur le compte en fin de mois</dt><dd>{{ formatEuros(actif.solde_fin) }}</dd></div>
      </dl>
    </div>

    <p v-if="cumul && sansSolde" class="aide">
      Sans solde noté, le cumul part de 0 € au début de {{ premierMois }}. Note ton solde pour voir le vrai montant sur ton compte.
    </p>

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
              <td>{{ m.solde_fin !== undefined ? formatEuros(m.solde_fin) : cumul ? formatEuros(bornes[mois.indexOf(m)].fin) : '·' }}</td>
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
.pastille.fantome { border: 1.5px dashed; }
.pastille.perdu { background: var(--safran-clair); border-color: var(--safran); }
.pastille.rembourse { background: var(--sarcelle-clair); border-color: var(--sarcelle); }
path.fantome { stroke-width: 1.5; stroke-dasharray: 3 2; }
path.perdu { fill: var(--safran-clair); stroke: var(--safran); }
path.rembourse { fill: var(--sarcelle-clair); stroke: var(--sarcelle); }
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
