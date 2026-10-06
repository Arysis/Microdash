import { createRouter, createWebHistory } from 'vue-router'
import { session, chargerSession } from './session.js'
import { site } from '../theme.js'

// acces : « public » pour tout le monde, « invite » pour les visiteurs non connectés, « membre » pour l'app.
const routes = [
  { path: '/', component: () => import('./views/AccueilView.vue'), meta: { acces: 'public', titre: null, description: site.description } },
  { path: '/connexion', component: () => import('./views/ConnexionView.vue'), props: { mode: 'connexion' }, meta: { acces: 'invite', titre: 'Connexion', description: 'Connecte-toi à ton compte Microdash.' } },
  { path: '/inscription', component: () => import('./views/ConnexionView.vue'), props: { mode: 'inscription' }, meta: { acces: 'invite', titre: 'Créer un compte', description: 'Crée ton compte Microdash gratuit avec une adresse e-mail.' } },
  { path: '/mentions-legales', component: () => import('./views/MentionsLegalesView.vue'), meta: { acces: 'public', titre: 'Mentions légales', description: "Éditeur et hébergeur du site Microdash." } },
  { path: '/confidentialite', component: () => import('./views/ConfidentialiteView.vue'), meta: { acces: 'public', titre: 'Confidentialité', description: 'Les données que Microdash garde, pourquoi, combien de temps, et comment les effacer.' } },
  { path: '/tableau-de-bord', component: () => import('./views/TableauDeBordView.vue'), meta: { acces: 'membre', titre: 'Tableau de bord', relief: true } },
  { path: '/saisies', component: () => import('./views/TransactionsView.vue'), meta: { acces: 'membre', titre: 'Saisies', relief: true } },
  { path: '/tresorerie', component: () => import('./views/TresorerieView.vue'), meta: { acces: 'membre', titre: 'Trésorerie', relief: true } },
  { path: '/agenda', component: () => import('./views/AgendaView.vue'), meta: { acces: 'membre', titre: 'Agenda', relief: true } },
  { path: '/desinscription', component: () => import('./views/DesinscriptionView.vue'), meta: { acces: 'public', titre: 'Rappels par e-mail', noindex: true } },
  { path: '/profil', component: () => import('./views/ProfilView.vue'), meta: { acces: 'membre', titre: 'Ton activité' } },
  { path: '/compte', component: () => import('./views/CompteView.vue'), meta: { acces: 'membre', titre: 'Compte' } },
  // Anciennes adresses du socle.
  { path: '/transactions', redirect: '/saisies' },
  { path: '/:chemin(.*)*', component: () => import('./views/PageIntrouvableView.vue'), meta: { acces: 'public', titre: 'Page introuvable', noindex: true } },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach(async (to) => {
  if (!session.charge) await chargerSession()
  const connecte = Boolean(session.user)
  if (to.path === '/' && connecte) return '/tableau-de-bord'
  if (to.meta.acces === 'invite' && connecte) return '/tableau-de-bord'
  if (to.meta.acces === 'membre') {
    if (!connecte) return '/connexion'
    // Tant que le profil n'est pas renseigné, aucun calcul n'est possible.
    if (!session.user.profil_complet && to.path !== '/profil') return '/profil'
  }
  return true
})

function majBalise(selecteur, attribut, valeur) {
  document.querySelector(selecteur)?.setAttribute(attribut, valeur)
}

// Titre, description et adresse canonique propres à chaque page.
router.afterEach((to) => {
  document.title = to.meta.titre
    ? `${to.meta.titre} · ${site.nom}`
    : `${site.nom} : cotisations URSSAF et revenu net des micro-entrepreneurs`
  majBalise('meta[name="description"]', 'content', to.meta.description || site.description)
  majBalise('link[rel="canonical"]', 'href', `${location.origin}${to.path}`)
  // Le serveur renvoie l'app pour toute adresse : on dit aux moteurs de ne pas indexer la page 404.
  majBalise('meta[name="robots"]', 'content', to.meta.noindex ? 'noindex' : 'index, follow')
})

export default router
