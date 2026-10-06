<script setup>
import { Receipt, Calculator, Wallet, Gauge, Smartphone, CalendarClock, Mail, FileDown, ChartColumn } from 'lucide-vue-next'

const icone = { size: 20, 'stroke-width': 1.75, 'aria-hidden': 'true' }

// Uniquement ce que l'app fait déjà. Le reste va dans « Ce qui arrive ensuite ».
const fonctions = [
  {
    icone: Receipt,
    titre: 'Noter tes recettes et tes dépenses',
    texte: 'Une saisie par encaissement ou par achat : montant, date, libellé.',
  },
  {
    icone: Calculator,
    titre: 'Estimer tes cotisations URSSAF',
    texte: "Selon ta catégorie : vente, prestations BIC, libéral SSI ou CIPAV. L'ACRE et le versement libératoire sont pris en compte.",
  },
  {
    icone: Wallet,
    titre: 'Voir ton revenu net',
    texte: "Ce qui te reste après cotisations et dépenses, au mois, au trimestre ou à l'année. Avec ton revenu imposable estimé.",
  },
  {
    icone: Gauge,
    titre: 'Surveiller tes plafonds',
    texte: "Une jauge pour le plafond de la micro-entreprise, une pour la franchise de TVA. Elles changent de couleur à 80 %, et un message te dit ce qu'implique un dépassement.",
  },
  {
    icone: CalendarClock,
    titre: 'Ne rater aucune échéance',
    texte: "Un agenda de tes déclarations URSSAF, de ta déclaration de revenus et de la CFE, avec le montant à déclarer. Tu coches quand c'est fait.",
  },
  {
    icone: ChartColumn,
    titre: 'Prévoir ta trésorerie',
    texte: "Ce qui entre et sort chaque mois, et une prévision sur 6 mois avec tes dépenses récurrentes et tes paiements URSSAF. Tu peux y connecter ton compte Stripe pour suivre le revenu de tes abonnements.",
  },
  {
    icone: FileDown,
    titre: 'Exporter tes chiffres',
    texte: "Un récapitulatif de l'année en PDF, et toutes tes saisies en CSV pour ton tableur.",
  },
  {
    icone: Smartphone,
    titre: "L'avoir sur ton téléphone",
    texte: "Microdash s'ouvre dans ton navigateur et s'ajoute à ton écran d'accueil comme une app.",
  },
]

const ensuite = [
  { icone: Mail, texte: 'Un rappel par e-mail avant chaque échéance.' },
]
</script>

<template>
  <section class="haut largeur">
    <div class="haut-texte">
      <p class="surtitre">Pour les micro-entrepreneurs</p>
      <h1>Sache ce qu'il te reste après l'URSSAF.</h1>
      <p class="sous-titre">
        Note ce que tu encaisses. Microdash estime tes cotisations et ton revenu net, au mois, au trimestre et à l'année.
      </p>
      <div class="actions">
        <RouterLink class="bouton principal" to="/inscription">Créer mon compte</RouterLink>
        <RouterLink class="bouton secondaire" to="/connexion">Se connecter</RouterLink>
      </div>
      <p class="aide">Gratuit. Une adresse e-mail suffit.</p>
    </div>
    <figure class="capture">
      <img
        src="/capture-tableau-de-bord.webp"
        width="390"
        height="844"
        alt="Tableau de bord Microdash sur un téléphone : revenu net estimé du trimestre en gros, barre de répartition entre revenu net, cotisations et dépenses, puis le détail des montants."
      />
      <figcaption class="aide">Exemple de tableau de bord. Les montants sont fictifs.</figcaption>
    </figure>
  </section>

  <section class="bloc largeur">
    <h2>Ce que tu peux faire aujourd'hui</h2>
    <ul class="fonctions">
      <li v-for="f in fonctions" :key="f.titre">
        <component :is="f.icone" v-bind="icone" class="icone" />
        <h3>{{ f.titre }}</h3>
        <p>{{ f.texte }}</p>
      </li>
    </ul>
  </section>

  <section class="bloc largeur deux-colonnes">
    <div>
      <h2>Ce qui arrive ensuite</h2>
      <p class="aide">Pas encore disponible : ces fonctions sont en préparation.</p>
      <ul class="ensuite">
        <li v-for="e in ensuite" :key="e.texte"><component :is="e.icone" v-bind="icone" class="icone" />{{ e.texte }}</li>
      </ul>
    </div>
    <div>
      <h2>Des estimations, pas ta déclaration</h2>
      <p>
        Microdash applique les taux de l'année à tes saisies. Tu déclares toujours ton chiffre d'affaires sur
        <a href="https://autoentrepreneur.urssaf.fr" rel="noopener">autoentrepreneur.urssaf.fr</a>.
      </p>
      <p>Les montants servent à anticiper. Ils ne remplacent pas ceux de l'URSSAF ni des impôts.</p>
    </div>
  </section>

  <section class="bloc largeur fin">
    <h2>Commence par ta première recette.</h2>
    <p>Crée ton compte, réponds à quelques questions sur ton activité, puis note ce que tu as encaissé.</p>
    <RouterLink class="bouton principal" to="/inscription">Créer mon compte</RouterLink>
  </section>
</template>

<style scoped>
.largeur { max-width: 1080px; margin: 0 auto; padding: 0 var(--e4); }

.haut { display: grid; gap: var(--e7); padding-top: var(--e7); align-items: center; }
.haut-texte { display: grid; gap: var(--e4); justify-items: start; }
.surtitre { color: var(--safran-fonce); font-weight: 600; font-size: 15px; }
h1 { font-size: 32px; line-height: 1.1; max-width: 14ch; }
.sous-titre { font-size: 18px; color: var(--pierre); max-width: 34em; }
.actions { display: flex; flex-wrap: wrap; gap: var(--e3); margin-top: var(--e2); }

.capture { margin: 0; justify-self: center; width: 100%; max-width: 320px; display: grid; gap: var(--e2); }
.capture img { border: 1px solid var(--lin); background: var(--blanc); }

.bloc { padding-top: var(--e8); display: grid; gap: var(--e4); align-content: start; }
.icone { color: var(--safran); flex-shrink: 0; }

.fonctions { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--e5); }
.fonctions li { display: grid; gap: var(--e2); padding-top: var(--e4); border-top: 1px solid var(--lin); }
.fonctions p { color: var(--pierre); }

.deux-colonnes { gap: var(--e7); }
.deux-colonnes > div { display: grid; gap: var(--e3); align-content: start; }
.ensuite { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--e3); }
.ensuite li { display: flex; gap: var(--e3); align-items: flex-start; }

.fin { justify-items: start; padding-bottom: var(--e4); }

@media (min-width: 600px) {
  h1 { font-size: 44px; }
  .fonctions { grid-template-columns: repeat(2, 1fr); }
}
@media (min-width: 900px) {
  .haut { grid-template-columns: 1fr 320px; padding-top: var(--e8); }
  .fonctions { grid-template-columns: repeat(3, 1fr); }
  .deux-colonnes { grid-template-columns: 1fr 1fr; }
}
</style>
