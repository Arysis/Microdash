<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api.js'
import { useRouter } from 'vue-router'
import { LogOut, Trash2, ChevronRight, FileDown, Copy, Unplug } from 'lucide-vue-next'
import { session, deconnecter, supprimerCompte } from '../session.js'
import { demanderConfirmation } from '../confirmation.js'
import { formatDate } from '../format.js'

const router = useRouter()
const erreur = ref('')
const icone = { size: 20, 'stroke-width': 1.75, 'aria-hidden': 'true' }

const rappels = [
  { cle: 'echeances', titre: 'Échéances', aide: 'URSSAF et impôts : 7 jours avant, puis la veille' },
  { cle: 'plafond', titre: 'Plafond micro-entreprise', aide: "À 80 %, puis s'il est dépassé" },
  { cle: 'tva', titre: 'Franchise de TVA', aide: "À 80 %, puis si elle est dépassée" },
  { cle: 'cfe', titre: 'CFE', aide: 'Rappel en décembre, à confirmer pour ton cas' },
]
const anneeEnCours = new Date().getFullYear()
const anneeExport = ref(anneeEnCours)
const preferences = ref(null)
const erreurRappels = ref('')

onMounted(async () => {
  try {
    preferences.value = await api.get('/alertes/preferences')
  } catch (e) {
    erreurRappels.value = e.message
  }
})

// Connecteur Claude : adresse à ajouter dans Claude, accès donnés et bouton pour les couper.
const claude = ref(null)
const erreurClaude = ref('')
const copie = ref(false)

async function chargerClaude() {
  try {
    claude.value = await api.get('/claude')
  } catch (e) {
    erreurClaude.value = e.message
  }
}
onMounted(chargerClaude)

async function copierAdresse() {
  try {
    await navigator.clipboard.writeText(claude.value.adresse)
    copie.value = true
    setTimeout(() => (copie.value = false), 2000)
  } catch {
    erreurClaude.value = "Copie impossible : sélectionne l'adresse à la main."
  }
}

async function couperClaude() {
  const ok = await demanderConfirmation({
    titre: "Couper l'accès de Claude ?",
    message: 'Claude ne pourra plus lire ni changer ton compte. Pour le reconnecter, il faudra l\'autoriser de nouveau.',
    action: "Couper l'accès",
  })
  if (!ok) return
  erreurClaude.value = ''
  try {
    await api.del('/claude')
    await chargerClaude()
  } catch (e) {
    erreurClaude.value = e.message
  }
}

async function changerRappel(cle, valeur) {
  erreurRappels.value = ''
  const avant = { ...preferences.value }
  preferences.value = { ...preferences.value, [cle]: valeur }
  try {
    preferences.value = await api.put('/alertes/preferences', preferences.value)
  } catch (e) {
    preferences.value = avant
    erreurRappels.value = e.message
  }
}

async function quitter() {
  erreur.value = ''
  try {
    await deconnecter()
    router.push('/connexion')
  } catch (e) {
    erreur.value = e.message
  }
}

async function supprimer() {
  const ok = await demanderConfirmation({
    titre: 'Supprimer ton compte ?',
    message: 'Ton profil et toutes tes saisies sont effacés tout de suite. On ne peut pas les récupérer.',
    action: 'Supprimer mon compte',
  })
  if (!ok) return
  erreur.value = ''
  try {
    await supprimerCompte()
    router.push('/')
  } catch (e) {
    erreur.value = e.message
  }
}
</script>

<template>
  <h1>Compte</h1>

  <section class="carte pile">
    <div>
      <p class="aide">Adresse e-mail</p>
      <p>{{ session.user?.email }}</p>
    </div>
    <RouterLink class="ligne-lien" to="/profil">
      <span>
        Ton activité
        <small class="aide">Catégorie, ACRE, versement libératoire, périodicité</small>
      </span>
      <ChevronRight v-bind="icone" />
    </RouterLink>
  </section>

  <section class="carte rappels" aria-labelledby="titre-rappels">
    <h2 id="titre-rappels">Rappels par e-mail <InfoBulle k="rappels" /></h2>
    <template v-if="preferences">
      <label v-for="r in rappels" :key="r.cle" class="interrupteur">
        <span>{{ r.titre }}<small class="aide">{{ r.aide }}</small></span>
        <input type="checkbox" role="switch" :checked="preferences[r.cle]" @change="changerRappel(r.cle, $event.target.checked)" />
      </label>
    </template>
    <p v-if="erreurRappels" class="erreur" role="alert">{{ erreurRappels }}</p>
    <p class="aide">Envoyés à {{ session.user?.email }}. Chaque e-mail contient un lien pour te désinscrire.</p>
  </section>

  <section class="carte pile" aria-labelledby="titre-claude">
    <h2 id="titre-claude">Claude</h2>
    <template v-if="claude">
      <p class="aide">
        Pilote Microdash en écrivant à Claude : « ajoute ce soir la tâche Relancer Paul », « qu'est-ce que j'ai jeudi ? ».
        Dans Claude, ouvre Paramètres, Connecteurs, Ajouter un connecteur personnalisé, et colle cette adresse :
      </p>
      <div class="adresse">
        <code>{{ claude.adresse }}</code>
        <button class="bouton secondaire" @click="copierAdresse"><Copy v-bind="icone" />{{ copie ? 'Copiée' : 'Copier' }}</button>
      </div>
      <template v-if="claude.acces.length">
        <ul class="acces">
          <li v-for="a in claude.acces" :key="a.id">
            {{ a.client }} : autorisé le {{ formatDate(a.cree_le.slice(0, 10)) }}<template v-if="a.utilise_le">, dernier usage le {{ formatDate(a.utilise_le.slice(0, 10)) }}</template>
          </li>
        </ul>
        <button class="bouton secondaire danger" @click="couperClaude"><Unplug v-bind="icone" />Couper l'accès de Claude</button>
      </template>
      <p v-else class="aide">Claude n'a pas accès à ton compte pour le moment.</p>
    </template>
    <p v-if="erreurClaude" class="erreur" role="alert">{{ erreurClaude }}</p>
  </section>

  <section class="carte pile" aria-labelledby="titre-exports">
    <div class="titre-ligne">
      <h2 id="titre-exports">Exporter <InfoBulle k="exports" /></h2>
      <select v-model="anneeExport" aria-label="Année à exporter">
        <option v-for="a in [anneeEnCours, anneeEnCours - 1, anneeEnCours - 2]" :key="a" :value="a">{{ a }}</option>
      </select>
    </div>
    <a class="bouton secondaire" :href="`/api/exports/recapitulatif-${anneeExport}.pdf`" download>
      <FileDown v-bind="icone" />Récapitulatif {{ anneeExport }} en PDF
    </a>
    <a class="bouton secondaire" :href="`/api/exports/saisies-${anneeExport}.csv`" download>
      <FileDown v-bind="icone" />Saisies {{ anneeExport }} en CSV
    </a>
    <p class="aide">Le PDF reprend tes totaux, tes plafonds et toutes tes saisies de l'année. Le CSV s'ouvre dans Excel ou LibreOffice.</p>
  </section>

  <section class="carte pile">
    <button class="bouton secondaire" @click="quitter"><LogOut v-bind="icone" />Se déconnecter</button>
  </section>

  <section class="carte pile">
    <h2>Supprimer ton compte</h2>
    <p class="aide">Ton profil et toutes tes saisies sont effacés de nos serveurs. Pense à les noter ailleurs avant.</p>
    <button class="bouton secondaire danger" @click="supprimer"><Trash2 v-bind="icone" />Supprimer mon compte</button>
  </section>

  <p v-if="erreur" class="erreur" role="alert">{{ erreur }}</p>

  <p class="liens-legaux aide">
    <RouterLink to="/confidentialite">Confidentialité</RouterLink>
    <RouterLink to="/mentions-legales">Mentions légales</RouterLink>
  </p>
</template>

<style scoped>
h1 { margin-bottom: var(--e5); }
.ligne-lien {
  display: flex; justify-content: space-between; align-items: center; gap: var(--e3);
  min-height: 44px; padding-top: var(--e4); border-top: 1px solid var(--lin);
  color: var(--encre); text-decoration: none; font-weight: 500;
}
.ligne-lien small { display: block; font-weight: 400; }
.ligne-lien:hover { color: var(--safran); }
.rappels h2 { margin-bottom: var(--e2); }
.interrupteur {
  display: flex; justify-content: space-between; align-items: center; gap: var(--e3);
  min-height: 52px; padding: var(--e2) 0; border-bottom: 1px solid var(--lin);
}
.interrupteur:last-of-type { border-bottom: 0; }
.interrupteur span { display: grid; gap: 2px; }
.interrupteur small { font-weight: 400; }
.interrupteur input { width: 20px; height: 20px; margin: 0; flex-shrink: 0; accent-color: var(--safran); }
.rappels > .aide { margin-top: var(--e2); }
.adresse { display: flex; gap: var(--e3); align-items: center; flex-wrap: wrap; }
.adresse code { flex: 1 1 220px; padding: var(--e2) var(--e3); background: var(--papier); border: 1px solid var(--lin); border-radius: var(--rayon); font-size: 14px; overflow-wrap: anywhere; }
.acces { margin: 0; padding-left: var(--e5); }
.liens-legaux { display: flex; gap: var(--e5); margin-top: var(--e5); }
.liens-legaux a { color: var(--pierre); }
</style>
