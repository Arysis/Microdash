<script setup>
// Commandes du connecteur Claude : ce que Claude peut faire dans Microdash, avec des phrases à lui dire.
// À tenir à jour avec les outils de api/internal/httpapi/mcp.go.
const groupes = [
  {
    titre: 'Ce soir et tâches',
    commandes: [
      { outil: 'ce_soir', titre: 'Voir « Ce soir »', aide: 'Tes tâches du soir, les échéances proches et les mails importants.',
        exemples: ["Qu'est-ce que j'ai à faire ce soir ?", 'Fais-moi le point du soir.'] },
      { outil: 'lister_taches', titre: 'Lister les tâches', aide: 'Toutes les tâches à faire, filtrables par projet.',
        exemples: ['Quelles tâches il me reste sur le projet saas ?', "Qu'est-ce que j'ai prévu cette semaine ?"] },
      { outil: 'ajouter_tache', titre: 'Ajouter une tâche', aide: 'Pour ce soir, un jour précis, la semaine ou plus tard.',
        exemples: ['Ajoute ce soir la tâche Relancer Paul.', 'Ajoute pour jeudi : préparer la facture, 30 minutes.'] },
      { outil: 'modifier_tache', titre: 'Modifier une tâche', aide: 'Change le titre, la date, le projet ou la durée.',
        exemples: ['Décale « Relancer Paul » à vendredi.'] },
      { outil: 'terminer_tache', titre: 'Cocher une tâche', aide: 'La marque comme faite.',
        exemples: ["C'est fait pour Relancer Paul."] },
      { outil: 'supprimer_tache', titre: 'Supprimer une tâche', aide: 'Claude te demande confirmation avant.',
        exemples: ['Supprime la tâche « Tester la démo ».'] },
    ],
  },
  {
    titre: 'Mails',
    note: 'Il faut que Gmail soit connecté dans Réglages.',
    commandes: [
      { outil: 'lire_veille', titre: 'Lire la veille', aide: 'Le texte de tes newsletters non lues, pour que Claude les résume. Pas de copier-coller.',
        exemples: ['Résume ma veille techno.', "Quoi de neuf dans ma veille aujourd'hui ?"] },
      { outil: 'lire_mails_non_lus', titre: 'Lire les mails non lus', aide: "Les autres mails non lus : expéditeur, sujet et début du texte, les importants d'abord.",
        exemples: ["Résume mes mails non lus.", "J'ai des mails importants ?"] },
      { outil: 'marquer_mails_lus', titre: 'Marquer des mails comme lus', aide: 'Dans Gmail. Claude ne marque que les mails résumés, ou ceux que tu lui désignes.',
        exemples: ['Résume ma veille, puis marque-la comme lue.', 'Marque comme lus les mails de LinkedIn.'] },
    ],
  },
  {
    titre: 'Agenda et échéances',
    commandes: [
      { outil: 'agenda', titre: "Lire l'agenda", aide: 'Rendez-vous, tâches datées et échéances URSSAF et impôts, entre deux dates.',
        exemples: ["Qu'est-ce que j'ai jeudi ?", 'Mes échéances du mois prochain ?'] },
      { outil: 'ajouter_rendez_vous', titre: 'Ajouter un rendez-vous', aide: 'À une heure précise ou pour la journée.',
        exemples: ['Ajoute un rendez-vous comptable mardi à 14 h, une heure.'] },
      { outil: 'supprimer_rendez_vous', titre: 'Supprimer un rendez-vous', aide: 'Claude te demande confirmation avant.',
        exemples: ['Annule le rendez-vous de mardi.'] },
      { outil: 'marquer_echeance', titre: 'Marquer une échéance', aide: 'Déclarée ou payée, ou de nouveau à faire.',
        exemples: ["J'ai déclaré l'URSSAF d'octobre."] },
    ],
  },
  {
    titre: 'Finances',
    commandes: [
      { outil: 'resume_finances', titre: 'Résumé des finances', aide: "Chiffre d'affaires, cotisations, dépenses, revenu net, plafonds et trésorerie de l'année.",
        exemples: ["Où j'en suis de mon chiffre d'affaires cette année ?", "Combien je dois mettre de côté pour l'URSSAF ?"] },
    ],
  },
]
</script>

<template>
  <h1>Commandes Claude</h1>
  <div class="pile">
    <section class="carte pile">
      <p>
        Avec le connecteur Microdash, tu parles à Claude, sur ton téléphone ou ailleurs, et il lit ou change ton compte pour toi.
        Pas besoin de nom de commande : écris normalement, comme dans les exemples.
      </p>
      <p class="aide">
        Pour l'ajouter, copie l'adresse du connecteur dans <RouterLink to="/compte">Compte</RouterLink>, carte « Claude ».
        Claude n'a accès ni à tes saisies, ni à Qonto, ni à ton mot de passe.
      </p>
    </section>

    <section v-for="g in groupes" :key="g.titre" class="carte" :aria-labelledby="`groupe-${g.commandes[0].outil}`">
      <h2 :id="`groupe-${g.commandes[0].outil}`">{{ g.titre }}</h2>
      <p v-if="g.note" class="aide note">{{ g.note }}</p>
      <ul class="commandes">
        <li v-for="c in g.commandes" :key="c.outil">
          <div class="titre-commande">
            <strong>{{ c.titre }}</strong>
            <code>{{ c.outil }}</code>
          </div>
          <p class="aide">{{ c.aide }}</p>
          <ul class="exemples" aria-label="Exemples">
            <li v-for="e in c.exemples" :key="e">« {{ e }} »</li>
          </ul>
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
h1 { margin-bottom: var(--e5); }
h2 { margin: 0; }
.note { margin: var(--e1) 0 0; }
.commandes { list-style: none; padding: 0; margin: var(--e3) 0 0; }
.commandes > li { display: grid; gap: var(--e1); padding: var(--e3) 0; border-bottom: 1px solid var(--lin); }
.commandes > li:last-child { border-bottom: 0; padding-bottom: 0; }
.commandes p { margin: 0; }
.titre-commande { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: var(--e1) var(--e3); }
.titre-commande code { font-size: 12px; color: var(--pierre); overflow-wrap: anywhere; }
.exemples { list-style: none; padding: 0; margin: var(--e1) 0 0; display: grid; gap: var(--e1); }
.exemples li { background: var(--safran-clair); border-radius: var(--rayon); padding: var(--e2) var(--e3); font-size: 15px; }
</style>
