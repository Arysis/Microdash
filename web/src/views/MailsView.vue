<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api.js'

const donnees = ref(null)
const erreur = ref('')
const onglet = ref('regarder')
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

// À regarder : tout ce qui n'est pas de la veille, les importants d'abord.
const aRegarder = computed(() => {
  const ms = (donnees.value?.mails || []).filter((m) => !m.veille)
  return [...ms.filter((m) => m.important), ...ms.filter((m) => !m.important)]
})
const veille = computed(() => (donnees.value?.mails || []).filter((m) => m.veille))
const importants = computed(() => aRegarder.value.filter((m) => m.important).length)

const resume = computed(() => {
  const d = donnees.value
  if (!d) return ''
  if (!d.connecte) return 'Gmail n\'est pas connecté.'
  const n = aRegarder.value.length
  let texte = n ? `${n} ${n > 1 ? 'mails non lus' : 'mail non lu'} à regarder` : 'Rien à regarder'
  if (importants.value) texte += `, dont ${importants.value} important${importants.value > 1 ? 's' : ''}`
  if (veille.value.length) texte += ` · ${veille.value.length} en veille`
  return `${texte}. ${depuis(d.synchro_le)}`
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

// --- Veille : texte à coller dans Claude, puis mails marqués lus ---
const copie = ref(null) // { texte, refs, copie: bool, lus: bool }
const message = ref('')

function jour(iso) {
  return new Date(iso).toLocaleDateString('fr-FR', { weekday: 'long', day: 'numeric', month: 'long' })
}

function texteVeille(ms) {
  const ordre = [...ms].sort((a, b) => a.recu_le.localeCompare(b.recu_le))
  const blocs = ordre.map((m, i) => [
    `--- Mail ${i + 1} ---`,
    `De : ${m.expediteur_nom ? `${m.expediteur_nom} <${m.expediteur}>` : m.expediteur}`,
    `Sujet : ${m.sujet}`,
    `Reçu le : ${jour(m.recu_le)}`,
    '',
    m.texte || m.extrait || '(texte indisponible)',
  ].join('\n'))
  return [
    `Voici ${ms.length} mail${ms.length > 1 ? 's' : ''} de ma veille technologique. Fais-m'en un résumé en français :`,
    '1. Les points essentiels à retenir, toutes sources confondues (5 à 10 puces).',
    '2. Pour chaque mail, une ou deux phrases sur ce qu\'il apporte de nouveau.',
    '3. Ce qui mérite que je lise le mail en entier, et pourquoi.',
    'Ignore les publicités, les offres d\'emploi et les liens de désinscription.',
    '',
    ...blocs,
  ].join('\n')
}

async function copierPourClaude() {
  erreur.value = ''
  message.value = ''
  const ms = veille.value
  copie.value = { texte: texteVeille(ms), refs: ms.map((m) => m.ref), copie: false, lus: false }
  try {
    await navigator.clipboard.writeText(copie.value.texte)
    copie.value.copie = true
  } catch {
    // Copie refusée par le navigateur : le texte reste affiché pour une copie à la main.
    return
  }
  await marquerLus()
}

async function marquerLus() {
  const c = copie.value
  if (!c) return
  enCours.value = 'veille'
  try {
    donnees.value = await api.post('/mails/lus', { refs: c.refs })
    c.lus = true
    const n = c.refs.length
    message.value = `${n} mail${n > 1 ? 's' : ''} ${c.copie ? `copié${n > 1 ? 's' : ''} et ` : ''}marqué${n > 1 ? 's' : ''} lu${n > 1 ? 's' : ''} dans Gmail. Colle le texte dans Claude.`
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
      <button :class="{ actif: onglet === 'regarder' }" :aria-pressed="onglet === 'regarder'" @click="onglet = 'regarder'">À regarder ({{ aRegarder.length }})</button>
      <button :class="{ actif: onglet === 'veille' }" :aria-pressed="onglet === 'veille'" @click="onglet = 'veille'">Veille ({{ veille.length }})</button>
    </div>
  </div>

  <div class="pile-relief">
    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>

    <section v-if="donnees && !donnees.connecte" class="carte">
      <h2>Connecte Gmail</h2>
      <p class="aide">Le copilote lit l'expéditeur, le sujet et le début des mails non lus des 14 derniers jours, et te montre ceux qui comptent dans « Ce soir ».</p>
      <RouterLink class="bouton principal" to="/reglages">Ouvrir les réglages</RouterLink>
    </section>

    <template v-else-if="donnees">
      <p v-if="donnees.erreur" class="carte alerte" role="status">
        <strong>Gmail ne répond plus.</strong> {{ donnees.erreur }} <RouterLink to="/reglages">Réglages</RouterLink>
      </p>

      <template v-if="onglet === 'regarder'">
        <section class="carte liste-mails" aria-label="Mails à regarder">
          <p v-if="!aRegarder.length" class="vide">Aucun mail non lu à regarder. Tu peux passer à la suite.</p>
          <ul>
            <li v-for="m in aRegarder" :key="m.ref">
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
          Seuls les mails non lus s'affichent. « Archiver » sort le mail de ta boîte de réception Gmail, comme dans Gmail.
          Ce qui est important se règle dans <RouterLink to="/reglages">Réglages</RouterLink>.
        </p>
      </template>

      <template v-else>
        <section class="carte resume-veille" aria-labelledby="titre-veille">
          <h2 id="titre-veille">Résumer la veille</h2>
          <p v-if="!donnees.veille.length" class="aide">
            Aucune règle de veille. Ajoute des mots-clés, adresses ou domaines dans <RouterLink to="/reglages">Réglages</RouterLink>.
          </p>
          <template v-else>
            <p class="aide">
              Copie le texte de tes {{ veille.length }} mail{{ veille.length > 1 ? 's' : '' }} de veille et colle-le dans Claude pour en avoir le résumé.
              Ils sont ensuite marqués lus dans Gmail.
            </p>
            <button class="bouton principal" :disabled="!veille.length || enCours === 'veille'" @click="copierPourClaude">
              {{ enCours === 'veille' ? 'Envoi à Gmail…' : 'Copier pour Claude' }}
            </button>
          </template>
          <p v-if="message" class="succes" role="status">{{ message }}</p>
          <template v-if="copie && !copie.lus">
            <p class="aide">Le navigateur n'a pas pu copier : sélectionne le texte, copie-le, puis marque les mails comme lus.</p>
            <textarea :value="copie.texte" readonly rows="8" @focus="$event.target.select()" />
            <button class="bouton secondaire" :disabled="enCours === 'veille'" @click="marquerLus">Marquer comme lus</button>
          </template>
          <details v-else-if="copie">
            <summary>Revoir le texte copié</summary>
            <textarea :value="copie.texte" readonly rows="8" @focus="$event.target.select()" />
          </details>
        </section>

        <section v-if="veille.length" class="carte liste-mails" aria-label="Mails de veille">
          <ul>
            <li v-for="m in veille" :key="m.ref">
              <div class="meta">
                <span class="de">{{ m.expediteur_nom || m.expediteur }}</span>
                <span>{{ quand(m.recu_le) }}</span>
              </div>
              <p class="sujet">{{ m.sujet }}</p>
              <p v-if="m.extrait" class="aide extrait">{{ m.extrait }}</p>
              <div class="actions">
                <a class="bouton secondaire" :href="m.lien" target="_blank" rel="noopener">Ouvrir</a>
              </div>
            </li>
          </ul>
        </section>
      </template>
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
.sujet { overflow-wrap: anywhere; font-weight: 600; }
.extrait { display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.badge { justify-self: start; background: var(--safran-clair); color: var(--safran-fonce); border-radius: var(--rayon); padding: 2px var(--e2); font-size: 13px; font-weight: 600; }
.actions { display: flex; flex-wrap: wrap; gap: var(--e2); margin-top: var(--e2); }
.actions .bouton { flex: 1 1 0; min-width: 96px; padding: 0 var(--e2); font-size: 14px; }
.note { padding: 0 var(--e1); }
.resume-veille { display: grid; gap: var(--e3); }
.resume-veille h2 { margin: 0; }
.resume-veille .bouton { justify-self: start; }
.resume-veille textarea { width: 100%; font: 13px/1.5 var(--police-texte); }
.resume-veille summary { cursor: pointer; color: var(--pierre); font-size: 14px; }
.resume-veille details textarea { margin-top: var(--e2); }
</style>
