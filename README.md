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

Tout se pilote depuis GitHub, téléphone compris, sans se connecter au serveur :

1. Une fonctionnalité arrive sur une branche, avec sa **pull request**. La CI la vérifie (tests Go, build du front).
2. Tu relis la PR et tu la **fusionnes** dans `main` (app GitHub : Pull requests › Merge).
3. Le workflow **Déployer** vérifie à nouveau, publie les images `ghcr.io/arysis/microdash-api` et `-web` (version `sha-xxxxxxx`), puis **attend ton feu vert**. Tu reçois une notification : Actions › Déployer › **Review deployments** › production › **Approve and deploy**. Tant que tu n'approuves pas, rien ne change sur le serveur ; « Reject » abandonne cette version.
4. Le serveur sauvegarde la base (10 dernières gardées dans `sauvegardes/`), installe la version, vérifie que le site répond, et revient tout seul à la précédente sinon. Le résultat s'affiche dans GitHub.

**Revenir en arrière** : Actions › Déployer › Run workflow (branche `main`), avec dans « version » une version déjà publiée (ex. `sha-1a2b3c4`, visible dans l'historique des déploiements), puis approuver.

**Sécurité** : GitHub se connecte avec une clé SSH qui ne peut lancer qu'une commande, `deploiement/deployer.sh` (« command= » dans `authorized_keys`) : même volée, elle ne donne ni terminal ni accès aux fichiers. Active la double authentification sur ton compte GitHub : c'est elle qui protège le bouton « Approve ».

### Installation, une seule fois

Exemple pour un VPS Ubuntu ou Debian ; les commandes `sudo` se lancent sur le VPS.

1. **Docker** :

   ```sh
   curl -fsSL https://get.docker.com | sudo sh
   ```

2. **Un utilisateur dédié** et le dossier de l'app :

   ```sh
   sudo adduser --disabled-password --gecos "" microdash
   sudo usermod -aG docker microdash
   sudo mkdir -p /opt/microdash/deploiement && sudo chown -R microdash: /opt/microdash
   ```

3. **Les fichiers de l'app**, dans `/opt/microdash` (copiés du dépôt) : `compose.yaml`, `deploiement/deployer.sh` (le rendre exécutable : `chmod +x`), et un `.env` fait à partir de `.env.example` avec `COOKIE_SECURE=true`, un vrai `POSTGRES_PASSWORD`, `CLE_CHIFFREMENT` (`openssl rand -hex 32`) et `SITE_URL`. `MICRODASH_VERSION` est rempli par le premier déploiement. Garde une copie du `.env` en lieu sûr.

   Ces deux fichiers ne sont pas mis à jour par les déploiements (la clé de GitHub ne peut pas écrire sur le serveur) : si une PR modifie `compose.yaml` ou `deployer.sh`, recopie-les à la main.

4. **La clé de déploiement**, créée sur ton ordinateur :

   ```sh
   ssh-keygen -t ed25519 -N "" -C github-microdash -f microdash-deploiement
   ```

   Sur le VPS, mets la clé publique (`microdash-deploiement.pub`) dans `/home/microdash/.ssh/authorized_keys`, sur une seule ligne, précédée de ses restrictions :

   ```
   restrict,command="/opt/microdash/deploiement/deployer.sh" ssh-ed25519 AAAA… github-microdash
   ```

   Puis `sudo chmod 700 /home/microdash/.ssh && sudo chmod 600 /home/microdash/.ssh/authorized_keys && sudo chown -R microdash: /home/microdash/.ssh`. Test depuis ton ordinateur : `ssh -i microdash-deploiement microdash@IP_DU_VPS etat` affiche la version en ligne, et toute autre commande est refusée.

5. **L'environnement GitHub** : Settings › Environments › New environment › `production`.
   - Deployment protection rules : coche **Required reviewers** et ajoute-toi.
   - Deployment branches and tags : **Selected branches**, ajoute `main`.
   - Environment secrets : `DEPLOY_SSH_KEY` (tout le contenu du fichier `microdash-deploiement`, la clé privée) et `DEPLOY_KNOWN_HOSTS` (le résultat de `ssh-keyscan -t ed25519 IP_DU_VPS`, lancé depuis Git Bash ou Linux : celui de Windows est trop ancien pour les serveurs récents et ne renvoie rien).
   - Environment variables : `DEPLOY_HOST` (IP ou nom du VPS), `DEPLOY_USER` (`microdash`), et `DEPLOY_PORT` si SSH n'écoute pas sur 22.

   Supprime ensuite la clé privée de ton ordinateur : elle n'est plus utile qu'à GitHub.

6. **Protéger `main`** : Settings › Rules › Rulesets › New branch ruleset, cible `main`, coche « Require a pull request before merging » et « Require status checks to pass » (`api`, `web`). Plus rien n'arrive sur `main`, donc sur le serveur, sans passer par une PR.

7. **Premier déploiement** : Actions › Déployer › Run workflow sur `main`, puis approuver.

### Domaine et HTTPS

1. Achète le domaine, puis crée deux enregistrements DNS de type A, `mondomaine.fr` et `www.mondomaine.fr`, vers l'adresse IP du VPS.
2. Dans les variables du dépôt GitHub (Settings > Secrets and variables > Actions > Variables), crée `SITE_URL` avec `https://www.mondomaine.fr`. Les images publiées par la CI s'en servent pour l'image de partage et l'adresse canonique. Lance ensuite un nouveau déploiement.
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

Sauvegarde de la base : chaque déploiement en fait une dans `/opt/microdash/sauvegardes/`. À la main : `docker compose exec db pg_dump -U microdash microdash > sauvegarde.sql`. Pour restaurer : `gunzip -c sauvegardes/FICHIER.sql.gz | docker compose exec -T db psql -U microdash microdash` (sur une base vide).

## Ce soir et les modules

La page d'accueil de l'app est « Ce soir » : une seule liste de ce qui est à traiter, toutes sources confondues, triée du plus pressant au moins pressant. Chaque élément se marque « Fait » ou se remet à plus tard (demain, samedi ou lundi). Un élément fait ne revient pas ; un élément reporté revient le jour choisi.

Chaque source est un module (`api/internal/modules`). Un module donne son code, son nom et, s'il le veut, ses éléments à traiter (`Source`) et ce qu'il place dans l'agenda (`Calendrier`) ; le registre est monté dans `cmd/server/main.go`. Les modules :

- **Finances** : les échéances URSSAF et fiscales 14 jours avant leur date (et jusqu'à 60 jours de retard), les plafonds à 80 % ou dépassés, et les opérations Qonto à valider. « Fait » sur une échéance la coche aussi dans la page Échéances.
- **Tâches** : par dates (du … au …), par semaine (du lundi au dimanche) ou Plus tard (sans date), par projet (Educari, SaaS, Microdash, perso). Une tâche datée est à faire chaque soir de sa période : elle s'affiche ces jours-là dans l'agenda et dans « Ta soirée ». Une durée écrite à la fin (« Relancer Paul 15 min », « Compta 1 h 30 ») est comptée dans « Ta soirée ». Une tâche arrivée à échéance passe dans « À traiter ». Le bouton « Tâche » d'un élément de « Ce soir » le range dans les tâches ; cocher la tâche prévient son module d'origine (l'échéance est cochée, le mail archivé).
- **Agenda** : rendez-vous, avec les tâches datées et les échéances, en vue Semaine ou Mois. Les rendez-vous du jour s'affichent dans « Ta soirée ».
- **Mails** : les mails non lus de la boîte de réception Gmail (voir plus bas). Les importants vont dans « Ce soir » ; « Fait » archive le mail. La veille est mise à part pour être résumée.

Chacun peut désactiver un module dans Plus › Réglages : il quitte « Ce soir » et le menu, ses données restent.

### Agenda dans Home Assistant

Dans l'app, Agenda › « Dans Home Assistant » › Créer le lien. Le lien (secret, montré une seule fois) sert un flux iCal : rendez-vous, tâches datées et échéances, de deux mois en arrière à un an en avant. Dans Home Assistant : Paramètres › Appareils et services › Ajouter une intégration › **Remote Calendar**, et coller le lien. Le même lien s'ajoute au calendrier du téléphone (abonnement par URL). « Changer le lien » coupe l'ancien.

### Mails (Gmail)

Le module lit Gmail en IMAP avec un **mot de passe d'application** Google, pas avec le mot de passe du compte :

1. Activer la validation en deux étapes sur le compte Google (obligatoire pour les mots de passe d'application).
2. Vérifier que l'IMAP est activé : Gmail › Paramètres › Transfert et POP/IMAP › Activer IMAP.
3. Créer le mot de passe sur [myaccount.google.com/apppasswords](https://myaccount.google.com/apppasswords) (nom : « Microdash ») et copier les 16 lettres.
4. Dans l'app : Plus › Réglages › Gmail, coller l'adresse et le mot de passe. Il est vérifié auprès de Gmail, puis chiffré avec `CLE_CHIFFREMENT` (obligatoire pour ce module).

L'API relit la boîte de réception toutes les 5 minutes (et sur « Actualiser ») : expéditeur, sujet, date et les 200 premiers caractères des mails **non lus** des 14 derniers jours. Un mail lu dans Gmail disparaît de Microdash à la lecture suivante. Les messages complets ne sont jamais copiés. Un mail est important s'il vient d'une adresse ou d'un domaine listé dans « Ce qui est important », ou si Gmail l'a rangé dans « Important » (désactivable). « Fait » ou « Archiver » sort le mail de la boîte de réception, comme le bouton Archiver de Gmail. Pour couper l'accès : déconnecter Gmail dans Réglages et supprimer le mot de passe d'application chez Google.

**Veille.** Dans Réglages › Veille, des mots-clés (cherchés dans l'expéditeur et le sujet), des adresses ou des domaines. Les mails qui correspondent quittent « À regarder » et « Ce soir » pour l'onglet Veille, et eux seuls gardent le début de leur texte (4 000 caractères). « Copier pour Claude » copie ces mails avec une consigne de résumé, à coller dans Claude, puis les marque lus dans Gmail : ils sortent de Microdash.

À venir : le résumé fait directement par l'API Claude, sans copier-coller.

## Mot de passe oublié (temporaire, sans e-mail)

En attendant les e-mails, la page « Mot de passe oublié ? » permet de choisir un nouveau mot de passe avec seulement l'adresse du compte. **N'importe qui connaissant l'adresse pourrait le faire**, donc la page n'est ouverte qu'aux adresses listées dans `MDP_OUBLIE_EMAILS` :

- vide (par défaut) : désactivée, le lien n'apparaît pas ;
- `moi@exemple.fr,associe@exemple.fr` : seulement ces comptes ;
- `*` : tous les comptes (à éviter en production).

Après un changement, `docker compose up -d` suffit. Le nouveau mot de passe ferme toutes les sessions ouvertes du compte. Pense à vider la variable une fois ton accès récupéré.

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
