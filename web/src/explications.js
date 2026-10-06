// Explications des boutons « i » (composant InfoBulle). Une entrée par terme, un paragraphe par ligne.
// Les textes disent ce que calcule Microdash, sans chiffre de barème : les taux sont dans api/baremes.
// Phrases de 20 mots au plus, tutoiement (DESIGN.md).

const urssaf = { texte: 'En savoir plus sur autoentrepreneur.urssaf.fr', url: 'https://www.autoentrepreneur.urssaf.fr' }
const impots = { texte: 'En savoir plus sur impots.gouv.fr', url: 'https://www.impots.gouv.fr' }

export const explications = {
  // --- Tableau de bord ---
  revenuNet: {
    titre: 'Revenu net estimé',
    texte: [
      "Ce qui te reste : ton chiffre d'affaires encaissé, moins l'URSSAF (cotisations, CFP, impôt si versement libératoire) et tes dépenses.",
      "Sans versement libératoire, ton impôt sur le revenu n'est pas déduit : il dépend de tout ton foyer.",
    ],
  },
  caEncaisse: {
    titre: "Chiffre d'affaires encaissé",
    texte: [
      "La somme de tes recettes, à la date où l'argent est arrivé.",
      "En micro-entreprise, tu déclares ce que tu as encaissé, pas ce que tu as facturé.",
    ],
  },
  cotisations: {
    titre: 'Cotisations sociales',
    texte: [
      "Elles financent ta retraite, ta santé et tes allocations. C'est un pourcentage de ton chiffre d'affaires, qui dépend de ton activité.",
      "Sans chiffre d'affaires, tu ne paies rien.",
      'Tant que tu n\'as pas saisi le paiement, le montant affiché est une estimation.',
    ],
    lien: urssaf,
  },
  cfp: {
    titre: 'Formation professionnelle (CFP)',
    texte: [
      "Une petite contribution qui te donne droit à la formation professionnelle. Elle se paie à l'URSSAF, avec tes cotisations.",
      "Son taux dépend de la nature de ton activité : commerçant, artisan ou libéral.",
    ],
  },
  versementLiberatoire: {
    titre: 'Versement libératoire',
    texte: [
      "Une option : tu paies l'impôt sur le revenu de ton activité avec tes cotisations, en pourcentage de ton chiffre d'affaires.",
      "Elle n'est ouverte que sous une condition de revenus du foyer. Vérifie si tu y as droit.",
    ],
    lien: impots,
  },
  urssafAnnee: {
    titre: "URSSAF de l'année",
    texte: [
      "Tu ne paies pas cette somme en une fois. Tu la règles à chaque déclaration, chaque mois ou chaque trimestre.",
      '« Déjà payé » vient de tes saisies au poste URSSAF. Le reste est estimé à partir de ton chiffre d\'affaires.',
    ],
  },
  urssafPaye: {
    titre: 'URSSAF payé',
    texte: [
      "Ce que tu as saisi comme paiement URSSAF pour cette période. Il remplace l'estimation des cotisations.",
      'Il compte sur le mois déclaré, même si tu as payé le mois suivant.',
    ],
  },
  depenses: {
    titre: 'Dépenses',
    texte: [
      'Tes achats et frais pro saisis. Ils servent à calculer ce qui te reste vraiment.',
      'En micro-entreprise, ils ne baissent ni tes cotisations ni ton impôt.',
    ],
  },
  revenuImposable: {
    titre: 'Revenu imposable',
    texte: [
      "Ton chiffre d'affaires après un abattement forfaitaire, qui dépend de ton activité. Il représente tes frais, sans justificatif.",
      "C'est ce montant que l'impôt prend en compte, avec les autres revenus de ton foyer.",
    ],
    lien: impots,
  },
  acre: {
    titre: 'ACRE',
    texte: [
      "Une aide à la création : tes cotisations sociales sont réduites pendant ta première période d'activité.",
      "Microdash l'applique jusqu'à la date affichée sur le tableau de bord. Vérifie ton droit et sa durée.",
    ],
    lien: urssaf,
  },
  plafondMicro: {
    titre: 'Plafond micro-entreprise',
    texte: [
      "Le chiffre d'affaires maximum sur une année civile pour rester en micro-entreprise. Il dépend de ton activité.",
      "Le dépasser une année ne suffit pas à en sortir. C'est le cas s'il est dépassé deux années de suite.",
    ],
    lien: urssaf,
  },
  seuilTVA: {
    titre: 'Franchise de TVA',
    texte: [
      "Sous ce seuil, tu ne factures pas de TVA. Au-dessus, tu la factures à partir du 1er janvier suivant.",
      'Au-delà du seuil majoré, la TVA est due dès le jour du dépassement.',
    ],
    lien: impots,
  },

  // --- Saisies ---
  recetteDepense: {
    titre: 'Recette ou dépense',
    texte: [
      "Une recette, c'est de l'argent encaissé pour ton activité. Note-la à la date où il arrive sur ton compte.",
      'Une dépense, c\'est un achat ou un frais pro. Un paiement à l\'URSSAF se saisit aussi en dépense.',
    ],
  },
  poste: {
    titre: 'Poste de dépense',
    texte: [
      "Il range tes dépenses par type, pour les retrouver dans les exports. Il ne change pas tes cotisations.",
      'Seul le poste URSSAF a un effet : il remplace l\'estimation de la déclaration choisie.',
    ],
  },
  frequence: {
    titre: 'Ponctuelle ou tous les mois',
    texte: [
      'Ponctuelle : une seule saisie. Tous les mois : la dépense revient chaque mois, à la même date.',
      'Tu la retrouves dans Trésorerie, et tu peux l\'arrêter depuis la liste des saisies.',
    ],
  },
  declarationPayee: {
    titre: 'Pour la déclaration',
    texte: [
      "La période dont ce paiement règle les cotisations. Souvent, tu paies au début du mois qui suit.",
      'Le montant compte sur cette période, et plus comme une dépense.',
    ],
  },

  // --- Agenda ---
  dateLimite: {
    titre: 'Date limite',
    texte: [
      "Le dernier jour pour déclarer ton chiffre d'affaires et payer. C'est la fin du mois qui suit la période.",
      'Un week-end ou un jour férié la repousse au jour ouvré suivant.',
    ],
    lien: urssaf,
  },
  cotisationsEstimees: {
    titre: 'Cotisations estimées',
    texte: [
      "Le montant calculé à partir des recettes saisies sur la période. Seul le calcul de l'URSSAF fait foi.",
      'Les mois déjà payés en sont retirés.',
    ],
  },
  caseFaite: {
    titre: 'Fait',
    texte: [
      "Coche-la quand tu as déclaré. Tu ne recevras plus de rappel pour cette échéance.",
      'Elle se coche toute seule quand tu saisis le paiement URSSAF de la déclaration.',
    ],
  },
  premiereDeclaration: {
    titre: 'Première déclaration',
    texte: [
      "La première déclaration regroupe ta période de démarrage et la ou les suivantes.",
      "C'est pour ça qu'elle couvre plus d'un mois ou d'un trimestre.",
    ],
    lien: urssaf,
  },
  declarationRevenus: {
    titre: 'Déclaration de revenus',
    texte: [
      "Chaque printemps, tu reportes ton chiffre d'affaires de l'année dans ta déclaration d'impôt, même avec le versement libératoire.",
      'La date limite dépend de ton département.',
    ],
    lien: impots,
  },
  cfe: {
    titre: 'CFE',
    texte: [
      "La cotisation foncière des entreprises, un impôt local. Elle n'est pas due l'année de création.",
      'Son montant et ses exonérations dépendent de ta commune.',
    ],
    lien: impots,
  },

  // --- Trésorerie ---
  solde: {
    titre: 'Solde du compte',
    texte: [
      'Ce qu\'il y a sur ton compte pro à une date. Microdash part de ce chiffre et ajoute tes saisies.',
      'Mets-le à jour de temps en temps pour rester juste.',
    ],
  },
  vueGraphe: {
    titre: 'Par mois ou cumulée',
    texte: [
      'Par mois : ce que chaque mois a ajouté ou retiré à ton compte.',
      "Cumulée : ce qu'il y a sur le compte en fin de mois. En bleu, ce qui restait du mois d'avant.",
    ],
  },
  prevision: {
    titre: 'Prévision',
    texte: [
      "Les mois à venir, en hachuré. Ils comptent tes dépenses récurrentes, tes cotisations à leur date limite et des recettes estimées.",
      "C'est une estimation : elle bouge à chaque nouvelle saisie.",
    ],
  },
  sourcePrevision: {
    titre: 'Source des recettes prévues',
    texte: [
      'MRR Stripe : le revenu mensuel de tes abonnements Stripe en cours.',
      'Mes saisies : la moyenne de tes recettes des 3 derniers mois complets.',
    ],
  },
  cleStripe: {
    titre: 'Clé restreinte Stripe',
    texte: [
      'Une clé qui commence par rk_, créée dans Stripe avec des droits en lecture seule.',
      "Microdash la chiffre et ne te la réaffiche jamais. Les clés secrètes et publiques sont refusées.",
    ],
    lien: { texte: 'Créer une clé restreinte dans Stripe', url: 'https://dashboard.stripe.com/apikeys' },
  },
  recurrentes: {
    titre: 'Dépenses récurrentes',
    texte: [
      'Tes abonnements et frais fixes. Microdash crée une saisie à chaque échéance, et les compte dans la prévision.',
      'Supprimer ou arrêter une dépense récurrente garde les saisies déjà créées.',
    ],
  },

  // --- Profil ---
  typeActivite: {
    titre: "Type d'activité",
    texte: [
      'Vente : tu revends des marchandises. Services BIC : prestations commerciales ou artisanales. BNC : professions libérales.',
      'Il fixe ton taux de cotisations, ton abattement et tes plafonds. Ton espace URSSAF l\'indique.',
    ],
    lien: urssaf,
  },
  activiteMixte: {
    titre: 'Activité mixte',
    texte: [
      'Tu vends des marchandises et tu fais aussi des prestations de services.',
      'Chaque recette est alors rattachée à une des deux activités, avec son propre taux.',
    ],
  },
  natureCFP: {
    titre: "Nature de l'activité",
    texte: [
      "Elle fixe le taux de ta contribution à la formation professionnelle (CFP).",
      'Commerçant pour une activité commerciale, artisan pour une activité artisanale, libéral pour les autres.',
    ],
  },
  debutActivite: {
    titre: "Date de début d'activité",
    texte: [
      "La date de création de ta micro-entreprise. Elle sert pour l'ACRE, ta première déclaration et la CFE.",
    ],
  },
  periodicite: {
    titre: 'Périodicité des déclarations',
    texte: [
      "Le rythme de tes déclarations URSSAF : chaque mois ou chaque trimestre. Tu l'as choisi à ta création.",
      'Ton espace URSSAF l\'indique. Elle change les dates de ton agenda.',
    ],
    lien: urssaf,
  },

  // --- Compte ---
  rappels: {
    titre: 'Rappels par e-mail',
    texte: [
      "Un e-mail 7 jours avant chaque échéance, puis la veille. Un autre quand tu passes 80 % d'un plafond.",
      "Une échéance cochée « fait » ou déjà payée ne déclenche plus de rappel.",
    ],
  },
  exports: {
    titre: 'Exports',
    texte: [
      'CSV : toutes tes saisies de l\'année, à ouvrir dans un tableur.',
      'PDF : un récapitulatif avec tes totaux, ton URSSAF, tes plafonds et tes saisies.',
    ],
  },
}
