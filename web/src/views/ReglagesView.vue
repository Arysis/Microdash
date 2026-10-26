<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api.js'
import { demanderConfirmation } from '../confirmation.js'
import { session, moduleActif } from '../session.js'

const modules = ref(null)
const erreur = ref('')

// Modules annoncés mais pas encore livrés : affichés sans interrupteur.
const enPreparation = ['Support', 'CRM']

const aides = {
  finances: 'Échéances URSSAF, plafonds et opérations Qonto à valider dans « Ce soir ». Onglet Finances.',
  agenda: 'Tes rendez-vous, avec les tâches datées et les échéances. Lien iCal pour Home Assistant.',
  mails: 'Les mails importants de ta boîte Gmail dans « Ce soir ». « Fait » les archive.',
  taches: 'Tes listes Ce soir, Cette semaine et Plus tard, par projet. Les tâches arrivées à échéance passent dans « Ce soir ».',
}

onMounted(async () => {
  try {
    modules.value = await api.get('/modules')
  } catch (e) {
    erreur.value = e.message
  }
  if (moduleActif('mails')) chargerMails()
})

// --- Gmail ---
const gmail = ref(null)
const connexion = reactive({ email: '', mot_de_passe: '', important_gmail: true })
const regle = reactive({ motif: '', libelle: '' })
const erreurGmail = ref('')
const enCours = ref(false)

async function chargerMails() {
  try {
    gmail.value = await api.get('/mails')
  } catch (e) {
    erreurGmail.value = e.message
  }
}

async function appel(f) {
  enCours.value = true
  erreurGmail.value = ''
  try {
    await f()
  } catch (e) {
    erreurGmail.value = e.message
  } finally {
    enCours.value = false
  }
}

const connecter = () => appel(async () => {
  gmail.value = await api.put('/mails/connexion', connexion)
  connexion.mot_de_passe = ''
})

const changerImportantGmail = (actif) => appel(async () => {
  gmail.value = await api.put('/mails/important-gmail', { actif })
})

async function deconnecter() {
  const ok = await demanderConfirmation({
    titre: 'Déconnecter Gmail ?',
    message: 'Le copilote oublie ton mot de passe d\'application et la copie des mails. Tes règles restent.',
    action: 'Déconnecter',
  })
  if (ok) await appel(async () => {
    await api.del('/mails/connexion')
    await chargerMails()
  })
}

const ajouterRegle = () => appel(async () => {
  const r = await api.post('/mails/regles', regle)
  gmail.value.regles = [...gmail.value.regles.filter((x) => x.id !== r.id), r].sort((a, b) => a.motif.localeCompare(b.motif))
  Object.assign(regle, { motif: '', libelle: '' })
})

const supprimerRegle = (r) => appel(async () => {
  await api.del(`/mails/regles/${r.id}`)
  gmail.value.regles = gmail.value.regles.filter((x) => x.id !== r.id)
})

const heure = (iso) => new Date(iso).toLocaleString('fr-FR', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

async function changer(m, actif) {
  erreur.value = ''
  const avant = m.actif
  m.actif = actif
  try {
    await api.put(`/modules/${m.code}`, { actif })
    session.modules = modules.value.map((x) => ({ ...x }))
  } catch (e) {
    m.actif = avant
    erreur.value = e.message
  }
}
</script>

<template>
  <h1>Réglages</h1>

  <section class="carte" aria-labelledby="titre-modules">
    <h2 id="titre-modules">Modules</h2>
    <p class="aide">Un module désactivé disparaît de « Ce soir » et du menu. Ses données restent.</p>
    <label v-for="m in modules || []" :key="m.code" class="interrupteur">
      <span>{{ m.nom }}<small class="aide">{{ aides[m.code] }}</small></span>
      <input type="checkbox" role="switch" :checked="m.actif" @change="changer(m, $event.target.checked)" />
    </label>
    <div v-for="nom in enPreparation" :key="nom" class="interrupteur prochain">
      <span>{{ nom }}</span>
      <small class="aide">En préparation</small>
    </div>
    <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>
  </section>

  <template v-if="gmail && moduleActif('mails')">
    <section class="carte" aria-labelledby="titre-gmail">
      <h2 id="titre-gmail">Gmail</h2>
      <p v-if="!gmail.disponible" class="aide">
        La connexion Gmail n'est pas activée sur ce serveur : il manque CLE_CHIFFREMENT (voir le README).
      </p>
      <template v-else-if="gmail.connecte">
        <p>Connecté à <strong>{{ gmail.email }}</strong>.</p>
        <p class="aide">{{ gmail.synchro_le ? `Dernière lecture : ${heure(gmail.synchro_le)}. Relu toutes les 5 minutes.` : 'Première lecture en cours.' }}</p>
        <p v-if="gmail.erreur" class="erreur" role="alert">{{ gmail.erreur }}</p>
        <label class="interrupteur">
          <span>Suivre le tri « Important » de Gmail<small class="aide">Sinon, seules tes règles comptent.</small></span>
          <input type="checkbox" role="switch" :checked="gmail.important_gmail" :disabled="enCours" @change="changerImportantGmail($event.target.checked)" />
        </label>
        <button class="lien-bouton danger" :disabled="enCours" @click="deconnecter">Déconnecter Gmail</button>
      </template>
      <form v-else class="form-gmail" @submit.prevent="connecter">
        <p class="aide">
          Gmail demande un mot de passe d'application (la validation en deux étapes doit être activée sur ton compte Google).
          Crée-le sur <a href="https://myaccount.google.com/apppasswords" target="_blank" rel="noopener">myaccount.google.com/apppasswords</a>, nomme-le « Microdash », puis colle-le ici.
          Il est gardé chiffré. Le copilote lit l'expéditeur, le sujet et le début des mails des 14 derniers jours.
        </p>
        <label>Adresse Gmail <input v-model="connexion.email" type="email" autocomplete="email" required /></label>
        <label>Mot de passe d'application <input v-model="connexion.mot_de_passe" autocomplete="off" placeholder="abcd efgh ijkl mnop" required /></label>
        <label class="case"><input v-model="connexion.important_gmail" type="checkbox" /> Suivre le tri « Important » de Gmail</label>
        <button class="bouton principal" :disabled="enCours">{{ enCours ? 'Vérification…' : 'Connecter Gmail' }}</button>
      </form>
      <p v-if="erreurGmail" class="erreur" role="alert">{{ erreurGmail }}</p>
    </section>

    <section class="carte" aria-labelledby="titre-regles">
      <h2 id="titre-regles">Ce qui est important</h2>
      <p class="aide">Un mail de ces adresses ou domaines passe toujours dans « Ce soir ».</p>
      <ul v-if="gmail.regles.length" class="regles">
        <li v-for="r in gmail.regles" :key="r.id">
          <span>{{ r.motif }}<small v-if="r.libelle" class="aide">{{ r.libelle }}</small></span>
          <button class="lien-bouton danger" :aria-label="`Retirer ${r.motif}`" :disabled="enCours" @click="supprimerRegle(r)">Retirer</button>
        </li>
      </ul>
      <form class="form-regle" @submit.prevent="ajouterRegle">
        <div class="ligne">
          <label>Adresse ou domaine <input v-model="regle.motif" placeholder="@educari.fr" required /></label>
          <label>Nom (facultatif) <input v-model="regle.libelle" maxlength="60" placeholder="Ex. : Client" /></label>
        </div>
        <button class="bouton secondaire" :disabled="enCours">Ajouter</button>
      </form>
    </section>
  </template>
</template>

<style scoped>
h1 { margin-bottom: var(--e5); }
h2 { margin-bottom: var(--e1); }
.carte > .aide { margin-bottom: var(--e2); }
.interrupteur {
  display: flex; justify-content: space-between; align-items: center; gap: var(--e3);
  min-height: 52px; padding: var(--e2) 0; border-bottom: 1px solid var(--lin);
  font-weight: 500;
}
.interrupteur span { display: grid; gap: 2px; }
.interrupteur small { font-weight: 400; }
.interrupteur input { width: 20px; height: 20px; margin: 0; flex-shrink: 0; accent-color: var(--safran); }
.prochain { color: var(--pierre); }
.prochain:last-of-type { border-bottom: 0; }
.carte + .carte { margin-top: var(--e4); }
.form-gmail, .form-regle { margin-top: var(--e3); gap: var(--e3); }
.regles { list-style: none; margin: var(--e2) 0 0; padding: 0; }
.regles li { display: flex; justify-content: space-between; align-items: center; gap: var(--e3); border-bottom: 1px solid var(--lin); }
.regles span { display: grid; gap: 2px; font-weight: 500; overflow-wrap: anywhere; }
.regles small { font-weight: 400; }
</style>
