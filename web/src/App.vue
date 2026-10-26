<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Moon, ListChecks, Calendar, Wallet, Ellipsis } from 'lucide-vue-next'
import { session, moduleActif } from './session.js'
import LogoMicrodash from './components/LogoMicrodash.vue'
import FenetreConfirmation from './components/FenetreConfirmation.vue'

const route = useRoute()
const miseEnPageApp = computed(() => route.meta.acces === 'membre')
// Les écrans principaux (meta.relief) ont un bandeau sarcelle et des cartes ombrées.
const relief = computed(() => Boolean(route.meta.relief))
const annee = new Date().getFullYear()
const icone = { size: 24, 'stroke-width': 1.75, 'aria-hidden': 'true' }
// Onglet allumé pour les pages d'un groupe (Finances : trois pages, Plus : compte et réglages).
const groupe = computed(() => route.meta.groupe)
</script>

<template>
  <div v-if="miseEnPageApp" :class="{ 'ecran-relief': relief }">
    <header :class="['entete-app', { relief }]">
      <RouterLink to="/ce-soir" aria-label="Microdash, ce soir"><LogoMicrodash /></RouterLink>
      <nav v-if="groupe === 'finances'" class="sous-onglets" aria-label="Finances">
        <RouterLink to="/tableau-de-bord">Tableau de bord</RouterLink>
        <RouterLink to="/saisies">Saisies</RouterLink>
        <RouterLink to="/tresorerie">Trésorerie</RouterLink>
        <RouterLink to="/echeances">Échéances</RouterLink>
      </nav>
    </header>
    <main class="contenu-app">
      <RouterView />
    </main>
    <nav v-if="session.user?.profil_complet" class="onglets" aria-label="Navigation principale">
      <RouterLink to="/ce-soir"><Moon v-bind="icone" />Ce soir</RouterLink>
      <RouterLink v-if="moduleActif('taches')" to="/taches"><ListChecks v-bind="icone" />Tâches</RouterLink>
      <RouterLink v-if="moduleActif('agenda')" to="/agenda"><Calendar v-bind="icone" />Agenda</RouterLink>
      <RouterLink v-if="moduleActif('finances')" to="/tableau-de-bord" :class="{ 'router-link-active': groupe === 'finances' }"><Wallet v-bind="icone" />Finances</RouterLink>
      <RouterLink to="/plus" :class="{ 'router-link-active': groupe === 'plus' }"><Ellipsis v-bind="icone" />Plus</RouterLink>
    </nav>
  </div>

  <template v-else>
    <header class="entete-public">
      <div class="largeur">
        <RouterLink to="/" aria-label="Microdash, accueil"><LogoMicrodash /></RouterLink>
        <div class="liens-entete">
          <template v-if="session.user">
            <RouterLink class="bouton principal" to="/ce-soir">Ouvrir l'app</RouterLink>
          </template>
          <template v-else>
            <RouterLink class="lien-entete" to="/connexion">Se connecter</RouterLink>
            <RouterLink class="bouton principal bouton-entete" to="/inscription">Créer mon compte</RouterLink>
          </template>
        </div>
      </div>
    </header>
    <main>
      <RouterView />
    </main>
    <footer class="pied">
      <div class="largeur">
        <span>© {{ annee }} Microdash</span>
        <nav aria-label="Informations légales">
          <RouterLink to="/mentions-legales">Mentions légales</RouterLink>
          <RouterLink to="/confidentialite">Confidentialité</RouterLink>
        </nav>
      </div>
    </footer>
  </template>

  <FenetreConfirmation />
</template>

<style scoped>
.entete-app {
  position: sticky; top: 0; z-index: 2;
  padding: calc(env(safe-area-inset-top) + var(--e3)) var(--e4) var(--e3);
  background: var(--papier); border-bottom: 1px solid var(--lin);
}
.entete-app a { text-decoration: none; display: inline-flex; color: var(--encre); }
.entete-app.relief { background: var(--sarcelle); border-bottom-color: var(--sarcelle); }
.entete-app.relief a { color: var(--blanc); }
.entete-app.relief :focus-visible { outline-color: var(--blanc); }
.sous-onglets { display: flex; gap: var(--e1); margin-top: var(--e2); overflow-x: auto; }
.entete-app .sous-onglets a {
  min-height: 36px; padding: 0 var(--e3); border-radius: var(--rayon); align-items: center;
  font-size: 14px; font-weight: 500; white-space: nowrap;
  transition: background-color var(--duree) ease-out;
}
.entete-app.relief .sous-onglets a { color: var(--blanc); background: var(--sur-sarcelle); }
.entete-app.relief .sous-onglets a.router-link-active { background: var(--blanc); color: var(--sarcelle); font-weight: 600; }
.contenu-app { max-width: 640px; margin: 0 auto; padding: var(--e5) var(--e4) calc(96px + env(safe-area-inset-bottom)); }

.onglets {
  position: fixed; bottom: 0; left: 0; right: 0; z-index: 2;
  display: flex; background: var(--blanc); border-top: 1px solid var(--lin);
  padding-bottom: env(safe-area-inset-bottom);
}
.onglets a {
  flex: 1; display: flex; flex-direction: column; align-items: center; gap: var(--e1);
  padding: var(--e2) var(--e1) var(--e3); min-height: 56px;
  color: var(--pierre); text-decoration: none; font-size: 12px; font-weight: 500;
  transition: color var(--duree) ease-out;
}
.onglets a:hover { color: var(--encre); }
.onglets a.router-link-active { color: var(--safran); font-weight: 600; }

.largeur { max-width: 1080px; margin: 0 auto; padding: 0 var(--e4); }
.entete-public { border-bottom: 1px solid var(--lin); background: var(--papier); }
.entete-public .largeur { display: flex; justify-content: space-between; align-items: center; gap: var(--e3); min-height: 64px; }
.entete-public a:first-child { text-decoration: none; display: inline-flex; color: var(--encre); }
.liens-entete { display: flex; align-items: center; gap: var(--e4); }
.lien-entete { color: var(--encre); font-weight: 500; text-decoration: none; min-height: 44px; display: inline-flex; align-items: center; }
.lien-entete:hover { color: var(--safran); }
/* Sur petit écran, le bouton d'inscription reste dans le haut de la page d'accueil. */
@media (max-width: 420px) { .bouton-entete { display: none; } }

.pied { border-top: 1px solid var(--lin); margin-top: var(--e8); padding: var(--e5) 0 calc(var(--e5) + env(safe-area-inset-bottom)); color: var(--pierre); font-size: 14px; }
.pied .largeur { display: flex; flex-wrap: wrap; justify-content: space-between; gap: var(--e3); }
.pied nav { display: flex; gap: var(--e5); }
.pied a { color: var(--pierre); min-height: 44px; display: inline-flex; align-items: center; }
.pied a:hover { color: var(--encre); }
</style>
