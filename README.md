# Microdash

Application web (PWA) pour gérer une micro-entreprise française au même endroit : saisie des recettes et dépenses, calcul du chiffre d'affaires, des cotisations URSSAF, de l'impôt et du revenu net, suivi des plafonds. L'agenda administratif et les exports arrivent dans les étapes suivantes.

Les montants sont des **estimations** : seule la déclaration URSSAF fait foi.

## Lancer en local

Prérequis : Docker avec Docker Compose.

```sh
cp .env.example .env        # puis choisis un mot de passe pour POSTGRES_PASSWORD
docker compose up --build
```

L'app est sur http://localhost:8080. Après une modification du code, relance `docker compose up --build` pour reconstruire les images.

Pour développer le front avec rechargement à chaud, garde la pile lancée puis exécute `cd web && npm install && npm run dev` : Vite (http://localhost:5173) relaie `/api` vers http://localhost:8080.

## Mettre en production (VPS avec Docker)

1. Pousse un tag de version, par exemple `git tag v0.1.0 && git push origin v0.1.0`. La CI publie les images `ghcr.io/arysis/microdash-api` et `ghcr.io/arysis/microdash-web` avec ce tag.
2. Sur le serveur, copie `compose.yaml` et un `.env` avec `MICRODASH_VERSION=v0.1.0`, `COOKIE_SECURE=true` et un vrai mot de passe de base.
3. Lance `docker compose pull && docker compose up -d`.

Pour revenir en arrière, remets l'ancien tag dans `.env` et relance la commande. Si le dépôt est privé, connecte d'abord le serveur à ghcr.io avec `docker login ghcr.io`.

### Domaine et HTTPS

1. Achète le domaine, puis crée deux enregistrements DNS de type A, `mondomaine.fr` et `www.mondomaine.fr`, vers l'adresse IP du VPS.
2. Dans les variables du dépôt GitHub (Settings > Secrets and variables > Actions > Variables), crée `SITE_URL` avec `https://www.mondomaine.fr`. Les images publiées par la CI s'en servent pour l'image de partage et l'adresse canonique. Pousse ensuite un nouveau tag.
3. Installe Caddy sur le VPS. Il obtient et renouvelle seul les certificats HTTPS. Exemple de `Caddyfile`, qui redirige le domaine nu vers `www` :

```
mondomaine.fr {
    redir https://www.mondomaine.fr{uri} permanent
}

www.mondomaine.fr {
    reverse_proxy localhost:8080
}
```

Remplace `8080` par ton `WEB_PORT`, et garde `COOKIE_SECURE=true` dans le `.env`.

Sauvegarde de la base : `docker compose exec db pg_dump -U microdash microdash > sauvegarde.sql`.

## Ce soir et les modules

La page d'accueil de l'app est « Ce soir » : une seule liste de ce qui est à traiter, toutes sources confondues, triée du plus pressant au moins pressant. Chaque élément se marque « Fait » ou se remet à plus tard (demain, samedi ou lundi). Un élément fait ne revient pas ; un élément reporté revient le jour choisi.

Chaque source est un module (`api/internal/modules`). Un module donne son code, son nom et, s'il le veut, ses éléments à traiter (`Source`) et ce qu'il place dans l'agenda (`Calendrier`) ; le registre est monté dans `cmd/server/main.go`. Les modules :

- **Finances** : les échéances URSSAF et fiscales 14 jours avant leur date (et jusqu'à 60 jours de retard), les plafonds à 80 % ou dépassés, et les opérations Qonto à valider. « Fait » sur une échéance la coche aussi dans la page Échéances.
- **Tâches** : listes Ce soir, Cette semaine et Plus tard, par projet (Educari, SaaS, Microdash, perso). Une durée écrite à la fin (« Relancer Paul 15 min », « Compta 1 h 30 ») est comptée dans « Ta soirée ». Une tâche arrivée à échéance passe dans « À traiter ». Le bouton « Tâche » d'un élément de « Ce soir » le range dans les tâches ; cocher la tâche prévient son module d'origine (l'échéance est cochée, le mail archivé).
- **Agenda** : rendez-vous, avec les tâches datées et les échéances, en vue Semaine ou Mois. Les rendez-vous du jour s'affichent dans « Ta soirée ».
- **Mails** : les mails importants de la boîte de réception Gmail (voir plus bas). « Fait » archive le mail.

Chacun peut désactiver un module dans Plus › Réglages : il quitte « Ce soir » et le menu, ses données restent.

### Agenda dans Home Assistant

Dans l'app, Agenda › « Dans Home Assistant » › Créer le lien. Le lien (secret, montré une seule fois) sert un flux iCal : rendez-vous, tâches datées et échéances, de deux mois en arrière à un an en avant. Dans Home Assistant : Paramètres › Appareils et services › Ajouter une intégration › **Remote Calendar**, et coller le lien. Le même lien s'ajoute au calendrier du téléphone (abonnement par URL). « Changer le lien » coupe l'ancien.

### Mails (Gmail)

Le module lit Gmail en IMAP avec un **mot de passe d'application** Google, pas avec le mot de passe du compte :

1. Activer la validation en deux étapes sur le compte Google (obligatoire pour les mots de passe d'application).
2. Vérifier que l'IMAP est activé : Gmail › Paramètres › Transfert et POP/IMAP › Activer IMAP.
3. Créer le mot de passe sur [myaccount.google.com/apppasswords](https://myaccount.google.com/apppasswords) (nom : « Microdash ») et copier les 16 lettres.
4. Dans l'app : Plus › Réglages › Gmail, coller l'adresse et le mot de passe. Il est vérifié auprès de Gmail, puis chiffré avec `CLE_CHIFFREMENT` (obligatoire pour ce module).

L'API relit la boîte de réception toutes les 5 minutes (et sur « Actualiser ») : expéditeur, sujet, date et les 200 premiers caractères des mails des 14 derniers jours. Les messages complets ne sont jamais copiés. Un mail est important s'il vient d'une adresse ou d'un domaine listé dans « Ce qui est important », ou si Gmail l'a rangé dans « Important » (désactivable). « Fait » ou « Archiver » sort le mail de la boîte de réception, comme le bouton Archiver de Gmail. Pour couper l'accès : déconnecter Gmail dans Réglages et supprimer le mot de passe d'application chez Google.

## Rappels par e-mail (Resend)

Sans `RESEND_API_KEY`, l'app marche normalement et n'envoie aucun e-mail. Pour activer les rappels :

1. Crée une clé d'API sur [resend.com](https://resend.com) et ajoute ton domaine dans Resend (enregistrements DNS à créer chez ton registraire).
2. Dans le `.env`, renseigne `RESEND_API_KEY`, `ALERTES_SECRET` (au moins 32 caractères, par exemple `openssl rand -hex 32`), `ALERTES_EXPEDITEUR` (par exemple `Microdash <rappels@mondomaine.fr>`) et `SITE_URL`.
3. Tant que le domaine n'est pas vérifié, Resend n'envoie qu'à l'adresse de ton compte Resend : mets-la dans `ALERTES_LIMITER_A` pour que l'app n'écrive qu'à elle. Vide la variable une fois le domaine vérifié.
4. `docker compose up -d`. Les journaux de l'API indiquent « rappels par e-mail activés ».

L'API envoie les rappels chaque matin à 8 h (heure de Paris), et au démarrage s'il est plus tard. Chaque e-mail ne part qu'une fois : la table `alertes_envoyees` garde la trace des envois.

## Trésorerie et Stripe

La page Trésorerie montre, mois par mois, l'argent qui entre et qui sort : recettes, dépenses (dont les dépenses récurrentes, saisies automatiquement à leur date) et cotisations URSSAF au mois de leur date limite. Elle prévoit les 6 mois suivants.

Un paiement à l'URSSAF se saisit comme une dépense au poste « URSSAF (cotisations et impôt) », avec la déclaration qu'il règle. Il remplace alors l'estimation de cette période : le tableau de bord montre le montant payé sur le mois déclaré (au prorata du chiffre d'affaires en trimestriel), la trésorerie le compte le mois où il sort du compte, et l'agenda coche l'échéance. Le graphe a deux vues : le solde de chaque mois, ou le cumul de ce qu'il y a sur le compte en fin de mois (en bleu ce qui reste du mois d'avant, en vert ce que le mois a ajouté).

Chaque compte peut y connecter son propre compte Stripe pour voir son revenu mensuel récurrent (MRR) et ses encaissements. Aucune clé ne va dans le `.env` : chacun colle sa clé dans l'app, elle est vérifiée auprès de Stripe, chiffrée (AES-256-GCM) avec `CLE_CHIFFREMENT` et liée à son compte. Elle n'est jamais renvoyée au navigateur ; seuls ses 4 derniers caractères sont affichés.

Côté serveur, une seule chose à faire : mettre `CLE_CHIFFREMENT` dans le `.env` (`openssl rand -hex 32`), puis `docker compose up -d`. La garder précieusement : si elle change, les clés enregistrées ne sont plus lisibles et chacun doit recoller la sienne.

Côté utilisateur, il faut une clé restreinte (elle commence par `rk_`) : Stripe › Développeurs › Clés API › Créer une clé restreinte, avec la lecture des abonnements (Subscriptions : Read) et du solde (Balance : Read). Les clés secrètes `sk_` sont refusées, car elles donnent tous les droits sur le compte. Si une permission manque, le message d'erreur de Stripe la nomme.

## Synchro Qonto

Chaque compte peut aussi connecter son compte Qonto (carte « Qonto » de la page Trésorerie) avec l'identifiant et la clé secrète de Qonto › Paramètres › Intégrations et partenariats › Clé API. La clé est chiffrée comme celle de Stripe, avec la même `CLE_CHIFFREMENT` ; Microdash ne fait que lire (organisation, comptes, opérations).

Chaque synchro lit les opérations réglées depuis le 1er janvier (puis seulement celles modifiées depuis la synchro précédente) et note le solde des comptes comme solde de départ de la prévision. Une opération n'est jamais lue deux fois. Chacune est rangée ainsi :

- virement entre deux comptes de l'organisation : ignoré ;
- saisie déjà présente, manuelle ou récurrente, du même type et du même montant à 5 jours près : rattachée, rien n'est créé ;
- libellé pour lequel la personne a demandé de « faire pareil » : saisie créée (ou opération ignorée) comme la fois d'avant ;
- sinon : liste « À valider » de la page Saisies, où la personne crée la saisie (montant, poste, activité modifiables) ou l'ignore.

Un débit dont le libellé contient « URSSAF » est proposé au poste URSSAF, pour la déclaration de la période précédant le paiement. Une opération ne compte dans les calculs qu'une fois devenue une saisie.

## Organisation

| Dossier | Contenu |
| --- | --- |
| `api/` | API Go : comptes, profil, transactions, calculs et agenda (`internal/calc`), rappels par e-mail (`internal/alertes`), exports CSV et PDF (`internal/exports`, police IBM Plex Sans sous licence OFL) |
| `api/baremes/` | Taux, plafonds, règles et dates de l'agenda par année (`2026.yaml`) |
| `web/` | PWA Vue 3 + Vite, servie par Nginx qui relaie `/api` vers l'API |
| `DESIGN.md` | Couleurs, polices, formes, icônes, mouvement et ton des textes : à suivre pour tout changement d'interface |
| `compose.yaml` | Les trois services : `web`, `api`, `db` (PostgreSQL 16) |

## Mettre à jour les taux

Les taux vivent dans `api/baremes/<année>.yaml`, embarqués dans l'image de l'API. Pour une nouvelle année, copie le fichier de l'année précédente, mets à jour les valeurs et reconstruis l'API. Pour tester des valeurs sans recompiler, monte un dossier de barèmes dans le conteneur et indique-le avec la variable `BAREMES_DIR`.

Une année sans fichier utilise le barème le plus récent, et le tableau de bord le signale.

La section `echeances` du fichier donne la date de la CFE et celle de la déclaration de revenus. Laisse `declaration_revenus: null` tant que la date n'est pas publiée : l'agenda affiche alors « fin mai ou début juin ».

## Tests

```sh
cd api && go test ./...
cd web && npm run build
```
