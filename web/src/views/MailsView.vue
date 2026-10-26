<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api.js'

const donnees = ref(null)
const erreur = ref('')
const onglet = ref('importants')
const synchro = ref(false)
const enCours = ref('')

async function charger() {
  try {
    donnees.value = await api.get('/mails')
  } catch (e) {
    erreur.value = e.message
  }
}
onMounted(charger)

async function actualiser() {
  synchro.value = true
  erreur.value = ''
  try {
    donnees.value = await api.post('/mails/synchroniser')
  } catch (e) {
    erreur.value = e.message
  } finally {
    synchro.value = false
  }
}

const importants = computed(() => (donnees.value?.mails || []).filter((m) => m.important))
const autres = computed(() => (donnees.value?.mails || []).filter((m) => !m.important))
const liste = computed(() => (onglet.value === 'importants' ? importants.value : autres.value))

const resume = computed(() => {
  const d = donnees.value
  if (!d) return ''
  if (!d.connecte) return 'Gmail n\'est pas connecté.'
  const n = importants.value.length
  const texte = n ? `${n} ${n > 1 ? 'mails importants' : 'mail important'}` : 'Aucun mail important'
  return `${texte} dans ta boîte de réception. ${depuis(d.synchro_le)}`
})

function depuis(iso) {
  if (!iso) return 'Première lecture en cours.'
  const min = Math.round((Date.now() - new Date(iso)) / 60000)
  if (min < 1) return "Lu à l'instant."
  if (min < 60) return `Lu il y a ${min} min.`
  return `Lu à ${new Date(iso).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}.`
}

function quand(iso) {
  const d = new Date(iso)
  if (d.toDateString() === new Date().toDateString()) return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })
}

// Retire le mail tout de suite ; le remet si Gmail refuse.
async function archiver(m) {
  enCours.value = m.ref
  erreur.value = ''
  const avant = donnees.value
  donnees.value = { ...avant, mails: avant.mails.filter((x) => x.ref !== m.ref) }
  try {
    await api.post(`/mails/${m.ref}/archiver`)
  } catch (e) {
    donnees.value = avant
    erreur.value = e.message
  } finally {
    enCours.value = ''
  }
}

async function toujoursImportant(m) {
  enCours.value = m.ref
  erreur.value = ''
  try {
    await api.post('/mails/regles', { motif: m.expediteur, libelle: m.expediteur_nom || '' })
    await charger()
  } catch (e) {
    erreur.value = e.message
  } finally {
    enCours.value = ''
  }
}
</script>

<template>
  <div class="bandeau">
    <div class="titre-ligne">
      <h1>Mails</h1>
      <button v-if="donnees?.connecte" class="bouton secondaire actualiser" :disabled="synchro" @click="actualiser">
        {{ synchro ? 'Lecture…' : 'Actualiser' }}
      </button>
    </div>
    <p>{{ resume }}</p>
    <div v-if="donnees?.connecte" class="bascule" role="group" aria-label="Mails à afficher">
      <button :class="{ actif: onglet === 'importants' }" :aria-pressed="onglet === 'importants'" @click="onglet = 'importants'">Importants ({{ importants.length }})</button>
      <button :class="{ actif: onglet === 'autres' }" :aria-pressed="onglet === 'autres'" @click="onglet = 'autres'">Les autres ({{ autres.length }})</button>
    </div>
  </div>

  <div class="pile-relief">
    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>

    <section v-if="donnees && !donnees.connecte" class="carte">
      <h2>Connecte Gmail</h2>
      <p class="aide">Le copilote lit l'expéditeur, le sujet et le début des mails des 14 derniers jours, et te montre ceux qui comptent dans « Ce soir ».</p>
      <RouterLink class="bouton principal" to="/reglages">Ouvrir les réglages</RouterLink>
    </section>

    <template v-else-if="donnees">
      <p v-if="donnees.erreur" class="carte alerte" role="status">
        <strong>Gmail ne répond plus.</strong> {{ donnees.erreur }} <RouterLink to="/reglages">Réglages</RouterLink>
      </p>
      <section class="carte liste-mails" :aria-label="onglet === 'importants' ? 'Mails importants' : 'Les autres mails'">
        <p v-if="!liste.length" class="vide">
          {{ onglet === 'importants' ? 'Rien d\'important. Tu peux passer à la suite.' : 'Aucun autre mail récent dans ta boîte de réception.' }}
        </p>
        <ul>
          <li v-for="m in liste" :key="m.ref" :class="{ 'non-lu': !m.lu }">
            <div class="meta">
              <span class="de">{{ m.expediteur_nom || m.expediteur }}</span>
              <span>{{ quand(m.recu_le) }}</span>
            </div>
            <p class="sujet">{{ m.sujet }}</p>
            <p v-if="m.extrait" class="aide extrait">{{ m.extrait }}</p>
            <span v-if="m.raison" class="badge">{{ m.raison }}</span>
            <div class="actions">
              <a class="bouton secondaire" :href="m.lien" target="_blank" rel="noopener">Ouvrir</a>
              <button v-if="!m.important" class="bouton secondaire" :disabled="enCours === m.ref" @click="toujoursImportant(m)">Toujours important</button>
              <button class="bouton principal" :disabled="enCours === m.ref" @click="archiver(m)">Archiver</button>
            </div>
          </li>
        </ul>
      </section>
      <p class="aide note">
        « Archiver » sort le mail de ta boîte de réception Gmail, comme dans Gmail.
        Ce qui est important se règle dans <RouterLink to="/reglages">Réglages</RouterLink>.
      </p>
    </template>
  </div>
</template>

<style scoped>
.bandeau p { color: var(--blanc); font-size: 15px; margin-bottom: var(--e4); }
.bandeau .actualiser { background: var(--blanc); color: var(--sarcelle); border-color: var(--blanc); min-height: 40px; }
.pile-relief { display: grid; gap: var(--e4); }
.carte + .carte { margin-top: 0; }
.alerte { border-left: 4px solid var(--brique); }
.liste-mails { padding-top: var(--e2); padding-bottom: var(--e2); }
.liste-mails ul { list-style: none; margin: 0; padding: 0; }
.liste-mails li { display: grid; gap: var(--e1); padding: var(--e3) 0; border-bottom: 1px solid var(--lin); }
.liste-mails li:last-child { border-bottom: 0; }
.vide { padding: var(--e3) 0; color: var(--pierre); }
.meta { display: flex; justify-content: space-between; gap: var(--e3); font-size: 13px; color: var(--pierre); }
.de { font-weight: 600; color: var(--encre); overflow-wrap: anywhere; }
.sujet { overflow-wrap: anywhere; }
.non-lu .sujet { font-weight: 600; }
.extrait { display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.badge { justify-self: start; background: var(--safran-clair); color: var(--safran-fonce); border-radius: var(--rayon); padding: 2px var(--e2); font-size: 13px; font-weight: 600; }
.actions { display: flex; flex-wrap: wrap; gap: var(--e2); margin-top: var(--e2); }
.actions .bouton { flex: 1 1 0; min-width: 96px; padding: 0 var(--e2); font-size: 14px; }
.note { padding: 0 var(--e1); }
</style>
