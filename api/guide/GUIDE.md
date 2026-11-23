# Guide de Microdash

Tout ce que fait Microdash, page par page. Ce guide est tenu à jour à chaque nouvelle fonctionnalité : si quelque chose n'y est pas, c'est que Microdash ne le fait pas encore.

## Ce soir

Ta page d'accueil : ce qui t'attend ce soir, toutes sources confondues.

- **Ta soirée** : les tâches et les rendez-vous du jour, avec le temps total quand les tâches ont une durée.
- **À traiter** : échéances proches, plafonds, opérations Qonto à valider, mails importants, tâches arrivées à échéance. Du plus pressant au moins pressant.
- **Fait ou plus tard** : chaque élément se marque « Fait », ou revient demain, samedi ou lundi.
- **Tâche** : le bouton range un élément dans tes tâches. Cocher la tâche coche aussi l'échéance ou archive le mail.

## Tâches

Tes tâches par projet : Educari, SaaS, Microdash, perso.

- **Par dates** (du … au …) : la tâche est à faire chaque soir de sa période. Elle s'affiche ces jours-là dans l'agenda et dans « Ta soirée ».
- **Par semaine** (du lundi au dimanche) ou **Plus tard** (sans date).
- **Cocher une tâche** (page Tâches ou « Ta soirée ») : un trait barre la tâche, puis elle quitte la liste. Tu la retrouves dans l'**Historique**. Reclique pendant le trait pour l'arrêter, ou touche **Annuler** dans le bandeau qui reste 5 secondes.
- **Durée** : écris-la à la fin du titre (« Relancer Paul 15 min », « Compta 1 h 30 ») ; elle compte dans « Ta soirée ».
- **Tâches récurrentes** : dans le formulaire d'ajout, liste « Dates », choisis « Répéter » : chaque jour, chaque semaine (le même jour que la date choisie) ou chaque mois (le même quantième, ou le dernier jour du mois). Une tâche d'un soir est créée toute seule à chaque date : le jour même pour « chaque jour », une semaine à l'avance sinon. Elles se listent dans la carte « Récurrentes » de la page Tâches ; « Arrêter » efface les prochaines pas encore faites et garde les autres.
- **Historique** (en bas de la page Tâches) : un bilan par semaine, du lundi au dimanche, fait tout seul. Pour chaque semaine : les tâches faites, celles pas faites à temps (avec la date si tu les as faites plus tard), le temps passé et la répartition par projet. La semaine en cours est en haut ; « Semaines d'avant » remonte plus loin.
- **Capture rapide** : depuis une autre app (navigateur, Gmail, YouTube…), bouton Partager › **Microdash** : le lien, le titre ou le texte partagé devient une tâche, à ranger pour ce soir, cette semaine ou plus tard. Marche avec l'app installée sur Android (et Chrome sur ordinateur). Après la mise à jour, Microdash peut mettre jusqu'à un jour à apparaître dans le menu Partager ; sinon, désinstalle et réinstalle l'app.
- **Capture sur iPhone** : iOS ne propose pas les apps web dans Partager. Crée un raccourci (app Raccourcis) : « Recevoir les URL et le texte depuis la feuille de partage », puis « Encoder l'URL » sur l'entrée, puis « Ouvrir les URL » avec `https://ton-site/partager?text=` suivi du texte encodé. Il apparaît ensuite dans Partager.

## Agenda

Tes rendez-vous, avec les tâches datées et les échéances, en vue Semaine ou Mois.

- **Rendez-vous** : à une heure précise ou pour toute la journée, avec des notes.
- **Abonnement calendrier** (en bas de l'agenda) : un lien secret au format iCal, montré une seule fois. Il donne tes rendez-vous, tes tâches datées et tes échéances. Boutons « Google Agenda » et « Ouvrir dans mon calendrier » (iPhone, Mac, Outlook). « Changer le lien » coupe l'ancien.
- **Home Assistant** : Paramètres › Appareils et services › Ajouter une intégration › « Remote Calendar », puis colle le lien de l'abonnement.

## Notifications

Sur ton PC et ton téléphone, même app fermée. **Tout est coupé par défaut.**

- **Activer** : Compte › Notifications › « Sur cet appareil », sur chaque appareil. Sur iPhone et iPad, installe d'abord l'app sur l'écran d'accueil (iOS 16.4 ou plus).
- **Ta soirée** : un résumé chaque jour à l'heure choisie, seulement s'il y a quelque chose.
- **Avant un rendez-vous** : un rappel de 0 à 2 h avant chaque rendez-vous à heure fixe.
- **Essai et appareils** : « Envoyer un essai » vérifie que ça marche ; la liste permet de retirer un appareil.

## Finances

Ton activité de micro-entrepreneur. Les montants sont des estimations : seule la déclaration URSSAF fait foi.

- **Tableau de bord** : chiffre d'affaires, cotisations URSSAF, impôt, revenu net et plafonds de l'année.
- **Saisies** : recettes et dépenses. Hors ligne, elles attendent le retour du réseau. Un paiement à l'URSSAF se saisit au poste « URSSAF (cotisations et impôt) ».
- **Échéances** : déclarations URSSAF, impôt, CFE, à cocher une fois faites.
- **Trésorerie** : l'argent qui entre et qui sort, mois par mois, avec 6 mois de prévision et les dépenses récurrentes (saisies toutes seules à leur date).
- **Stripe** : ton revenu mensuel récurrent et tes encaissements (clé restreinte `rk_`, lecture seule).
- **Qonto** : tes opérations bancaires, rangées toutes seules ou proposées « À valider » dans Saisies (lecture seule).

## Mails

Tes mails Gmail non lus des 14 derniers jours, triés.

- **Connexion** : Plus › Réglages › Gmail, avec un mot de passe d'application Google. Relu toutes les 5 minutes.
- **Ce qui est important** : les adresses ou domaines que tu listes passent dans « Ce soir ».
- **Veille** : tes newsletters à part. « Copier pour Claude » les copie avec une consigne de résumé, ou demande à Claude « Résume ma veille ».
- **Fait ou Archiver** : archive le mail dans Gmail.

## Claude (connecteur MCP)

Tu parles à Claude et il lit ou change ton Microdash pour toi.

- **Ajouter** : copie l'adresse dans Compte › carte « Claude », puis dans Claude : Paramètres › Connecteurs › Ajouter un connecteur personnalisé.
- **Outils** : `ce_soir`, `lister_taches`, `ajouter_tache`, `modifier_tache`, `terminer_tache`, `supprimer_tache`, `bilan_taches` (bilan des semaines), `agenda`, `ajouter_rendez_vous`, `supprimer_rendez_vous`, `marquer_echeance`, `resume_finances`, `lire_veille`, `lire_mails_non_lus`, `marquer_mails_lus`, `lire_guide` (ce guide).
- **Exemples** : la page Plus › Commandes Claude donne des phrases à dire.
- **Couper** : Compte › carte « Claude » › « Couper l'accès de Claude ».

## Compte et réglages

- **Modules** : Plus › Réglages. Désactive Finances, Tâches, Agenda ou Mails : le module quitte le menu et « Ce soir », ses données restent.
- **Rappels par e-mail** : échéances, plafond, TVA, CFE, dans Compte (si le serveur envoie des e-mails).
- **Exports** : Compte › Exporter, tes saisies en CSV et un récapitulatif en PDF, par année.
- **Ton activité** : Plus › Ton activité. Catégorie, ACRE, versement libératoire, périodicité : les calculs en dépendent.
- **Mot de passe** : Compte › Changer le mot de passe. Tes autres appareils sont déconnectés.
- **Installer l'app** : dans le navigateur, « Installer » ou « Sur l'écran d'accueil » : Microdash s'ouvre comme une app.
