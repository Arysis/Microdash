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

Pour revenir en arrière, remets l'ancien tag dans `.env` et relance la commande. Place un reverse proxy HTTPS (par exemple Caddy) devant le port `WEB_PORT`. Si le dépôt est privé, connecte d'abord le serveur à ghcr.io avec `docker login ghcr.io`.

Sauvegarde de la base : `docker compose exec db pg_dump -U microdash microdash > sauvegarde.sql`.

## Organisation

| Dossier | Contenu |
| --- | --- |
| `api/` | API Go : comptes, profil, transactions, calculs (`internal/calc`) |
| `api/baremes/` | Taux, plafonds et règles par année (`2026.yaml`) |
| `web/` | PWA Vue 3 + Vite, servie par Nginx qui relaie `/api` vers l'API |
| `compose.yaml` | Les trois services : `web`, `api`, `db` (PostgreSQL 16) |

## Mettre à jour les taux

Les taux vivent dans `api/baremes/<année>.yaml`, embarqués dans l'image de l'API. Pour une nouvelle année, copie le fichier de l'année précédente, mets à jour les valeurs et reconstruis l'API. Pour tester des valeurs sans recompiler, monte un dossier de barèmes dans le conteneur et indique-le avec la variable `BAREMES_DIR`.

Une année sans fichier utilise le barème le plus récent, et le tableau de bord le signale.

## Tests

```sh
cd api && go test ./...
cd web && npm run build
```
