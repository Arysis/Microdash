import { createRouter, createWebHistory } from 'vue-router'
import { session, chargerSession } from './session.js'
import ConnexionView from './views/ConnexionView.vue'
import TableauDeBordView from './views/TableauDeBordView.vue'
import TransactionsView from './views/TransactionsView.vue'
import ProfilView from './views/ProfilView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/connexion', component: ConnexionView, meta: { public: true } },
    { path: '/', component: TableauDeBordView },
    { path: '/transactions', component: TransactionsView },
    { path: '/profil', component: ProfilView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  if (!session.charge) await chargerSession()
  if (to.meta.public) return session.user ? '/' : true
  if (!session.user) return '/connexion'
  // Tant que le profil n'est pas renseigné, aucun calcul n'est possible.
  if (!session.user.profil_complet && to.path !== '/profil') return '/profil'
  return true
})

export default router
