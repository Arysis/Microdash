<script setup>
import { computed, onMounted, ref } from 'vue'
import { BellRing, Trash2 } from 'lucide-vue-next'
import { api } from '../api.js'
import { abonnementActuel, activer, desactiver, etatAppareil } from '../notifications.js'
import { formatDate } from '../format.js'

const icone = { size: 20, 'stroke-width': 1.75, 'aria-hidden': 'true' }
const avances = [
  { v: 0, nom: "À l'heure du rendez-vous" },
  { v: 5, nom: '5 min avant' },
  { v: 10, nom: '10 min avant' },
  { v: 15, nom: '15 min avant' },
  { v: 30, nom: '30 min avant' },
  { v: 60, nom: '1 h avant' },
  { v: 120, nom: '2 h avant' },
]

const infos = ref(null)
const etat = ref(etatAppareil())
const ici = ref(false) // cet appareil est abonné
const enCours = ref(false)
const erreur = ref('')
const message = ref('')

const appareils = computed(() => infos.value?.appareils || [])

async function charger() {
  try {
    infos.value = await api.get('/notifications')
    ici.value = Boolean(await abonnementActuel().catch(() => null))
  } catch (e) {
    erreur.value = e.message
  }
}
onMounted(charger)

async function appel(f) {
  enCours.value = true
  erreur.value = ''
  message.value = ''
  try {
    await f()
  } catch (e) {
    erreur.value = e.message
  } finally {
    enCours.value = false
    etat.value = etatAppareil()
  }
}

async function changerIci(caseACocher) {
  await appel(async () => {
    if (caseACocher.checked) {
      await activer(infos.value.cle_publique)
      message.value = 'Notifications activées sur cet appareil.'
    } else {
      await desactiver()
    }
    await charger()
  })
  caseACocher.checked = ici.value // en cas d'échec, l'interrupteur revient à l'état réel
}

const changerPreference = (cle, valeur) => appel(async () => {
  const p = { ...infos.value.preferences, [cle]: valeur }
  infos.value.preferences = await api.put('/notifications/preferences', p)
})

const essai = () => appel(async () => {
  const r = await api.post('/notifications/essai')
  message.value = r.appareils === 1 ? 'Essai envoyé à 1 appareil.' : `Essai envoyé à ${r.appareils} appareils.`
  await charger()
})

const retirer = (a) => appel(async () => {
  await api.del(`/notifications/abonnements/${a.id}`)
  await charger()
})
</script>

<template>
  <section class="carte notifications" aria-labelledby="titre-notifications">
    <h2 id="titre-notifications">Notifications</h2>
    <p v-if="infos && !infos.disponible" class="aide">Les notifications ne sont pas activées sur ce serveur.</p>
    <template v-else-if="infos">
      <p class="aide">
        Sur ton PC et ton téléphone, même app fermée : le résumé de ta soirée et un rappel avant tes rendez-vous.
        Tout est coupé par défaut : active-les sur chaque appareil, puis choisis ce que tu veux recevoir.
      </p>

      <label v-if="etat === 'ok'" class="interrupteur">
        <span>Sur cet appareil<small class="aide">{{ ici ? 'Activées' : 'Le navigateur te demandera ton accord.' }}</small></span>
        <input type="checkbox" role="switch" :checked="ici" :disabled="enCours" @change="changerIci($event.target)" />
      </label>
      <p v-else-if="etat === 'installer'" class="encart">
        Sur iPhone et iPad, installe d'abord l'app : dans Safari, bouton Partager, puis « Sur l'écran d'accueil ».
        Ouvre Microdash depuis l'écran d'accueil et reviens ici (iOS 16.4 ou plus récent).
      </p>
      <p v-else-if="etat === 'refuse'" class="encart">
        Les notifications sont bloquées pour Microdash dans ce navigateur. Autorise-les dans les réglages du site (le cadenas à gauche de l'adresse), puis recharge la page.
      </p>
      <p v-else class="encart">Ce navigateur ne reçoit pas de notifications push. Essaie Chrome, Edge, Firefox ou Safari à jour.</p>
      <p v-if="etat === 'ok' && ici && !infos.preferences.soiree && !infos.preferences.rendez_vous" class="encart">
        Cet appareil est prêt, mais rien n'est choisi : coche ci-dessous ce que tu veux recevoir.
      </p>

      <label class="interrupteur">
        <span>Ta soirée<small class="aide">Tes tâches, échéances et rendez-vous du soir, chaque jour s'il y en a.</small></span>
        <input type="checkbox" role="switch" :checked="infos.preferences.soiree" :disabled="enCours" @change="changerPreference('soiree', $event.target.checked)" />
      </label>
      <label v-if="infos.preferences.soiree" class="reglage">
        À quelle heure
        <input type="time" step="300" :value="infos.preferences.heure_soiree" :disabled="enCours"
          @change="$event.target.value && changerPreference('heure_soiree', $event.target.value)" />
      </label>
      <label class="interrupteur">
        <span>Avant un rendez-vous<small class="aide">Pour les rendez-vous à heure fixe de l'agenda.</small></span>
        <input type="checkbox" role="switch" :checked="infos.preferences.rendez_vous" :disabled="enCours" @change="changerPreference('rendez_vous', $event.target.checked)" />
      </label>
      <label v-if="infos.preferences.rendez_vous" class="reglage">
        Quand
        <select :value="infos.preferences.avance" :disabled="enCours" @change="changerPreference('avance', Number($event.target.value))">
          <option v-for="a in avances" :key="a.v" :value="a.v">{{ a.nom }}</option>
        </select>
      </label>

      <template v-if="appareils.length">
        <h3>Appareils qui reçoivent tes notifications</h3>
        <ul class="appareils">
          <li v-for="a in appareils" :key="a.id">
            <span>{{ a.appareil }}<small class="aide">Depuis le {{ formatDate(a.cree_le.slice(0, 10)) }}</small>
              <small v-if="a.erreur" class="erreur">Dernier envoi en échec</small></span>
            <button class="lien-bouton danger" :aria-label="`Retirer ${a.appareil}`" :disabled="enCours" @click="retirer(a)"><Trash2 v-bind="icone" /></button>
          </li>
        </ul>
        <button class="bouton secondaire" :disabled="enCours" @click="essai"><BellRing v-bind="icone" />Envoyer un essai</button>
      </template>
      <p v-if="message" class="aide" role="status">{{ message }}</p>
    </template>
    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
  </section>
</template>

<style scoped>
.notifications h2 { margin-bottom: var(--e2); }
.notifications h3 { font: 600 15px var(--police-texte); margin: var(--e4) 0 var(--e1); }
.interrupteur {
  display: flex; justify-content: space-between; align-items: center; gap: var(--e3);
  min-height: 52px; padding: var(--e2) 0; border-bottom: 1px solid var(--lin);
}
.interrupteur span { display: grid; gap: 2px; }
.interrupteur small { font-weight: 400; }
.interrupteur input { width: 20px; height: 20px; margin: 0; flex-shrink: 0; accent-color: var(--safran); }
.reglage {
  display: flex; justify-content: space-between; align-items: center; gap: var(--e3);
  padding: var(--e2) 0; border-bottom: 1px solid var(--lin); font-weight: 400; color: var(--pierre);
}
.reglage input, .reglage select { width: auto; min-width: 120px; }
.encart { margin: var(--e2) 0; }
.appareils { list-style: none; margin: 0 0 var(--e3); padding: 0; }
.appareils li { display: flex; justify-content: space-between; align-items: center; gap: var(--e3); min-height: 52px; border-bottom: 1px solid var(--lin); }
.appareils span { display: grid; gap: 2px; font-weight: 500; overflow-wrap: anywhere; }
.appareils small { font-weight: 400; }
.notifications > .aide { margin-bottom: var(--e2); }
</style>
