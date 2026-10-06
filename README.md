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

## Rappels par e-mail (Resend)

Sans `RESEND_API_KEY`, l'app marche normalement et n'envoie aucun e-mail. Pour activer les rappels :

1. Crée une clé d'API sur [resend.com](https://resend.com) et ajoute ton domaine dans Resend (enregistrements DNS à créer chez ton registraire).
2. Dans le `.env`, renseigne `RESEND_API_KEY`, `ALERTES_SECRET` (au moins 32 caractères, par exemple `openssl rand -hex 32`), `ALERTES_EXPEDITEUR` (par exemple `Microdash <rappels@mondomaine.fr>`) et `SITE_URL`.
3. Tant que le domaine n'est pas vérifié, Resend n'envoie qu'à l'adresse de ton compte Resend : mets-la dans `ALERTES_LIMITER_A` pour que l'app n'écrive qu'à elle. Vide la variable une fois le domaine vérifié.
4. `docker compose up -d`. Les journaux de l'API indiquent « rappels par e-mail activés ».

L'API envoie les rappels chaque matin à 8 h (heure de Paris), et au démarrage s'il est plus tard. Chaque e-mail ne part qu'une fois : la table `alertes_envoyees` garde la trace des envois.

## Organisation

| Dossier | Contenu |
| --- | --- |
| `api/` | API Go : comptes, profil, transactions, calculs et agenda (`internal/calc`), rappels par e-mail (`internal/alertes`) |
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
