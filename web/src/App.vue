<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { House, List, CircleUser } from 'lucide-vue-next'
import { session } from './session.js'
import LogoMicrodash from './components/LogoMicrodash.vue'
import FenetreConfirmation from './components/FenetreConfirmation.vue'

const route = useRoute()
const miseEnPageApp = computed(() => route.meta.acces === 'membre')
const annee = new Date().getFullYear()
const icone = { size: 24, 'stroke-width': 1.75, 'aria-hidden': 'true' }
</script>

<template>
  <template v-if="miseEnPageApp">
    <header class="entete-app">
      <RouterLink to="/tableau-de-bord" aria-label="Microdash, tableau de bord"><LogoMicrodash /></RouterLink>
    </header>
    <main class="contenu-app">
      <RouterView />
    </main>
    <nav v-if="session.user?.profil_complet" class="onglets" aria-label="Navigation principale">
      <RouterLink to="/tableau-de-bord"><House v-bind="icone" />Accueil</RouterLink>
      <RouterLink to="/saisies"><List v-bind="icone" />Saisies</RouterLink>
      <RouterLink to="/compte"><CircleUser v-bind="icone" />Compte</RouterLink>
    </nav>
  </template>

  <template v-else>
    <header class="entete-public">
      <div class="largeur">
        <RouterLink to="/" aria-label="Microdash, accueil"><LogoMicrodash /></RouterLink>
        <div class="liens-entete">
          <template v-if="session.user">
            <RouterLink class="bouton principal" to="/tableau-de-bord">Ouvrir l'app</RouterLink>
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
.entete-app a { text-decoration: none; display: inline-flex; }
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
.entete-public a:first-child { text-decoration: none; display: inline-flex; }
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
