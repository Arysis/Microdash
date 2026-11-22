# Microdash : consignes pour Claude

## Guide utilisateur : à tenir à jour, toujours

`api/guide/GUIDE.md` liste tout ce que fait Microdash. C'est la mémoire des fonctionnalités : l'utilisateur la relit, la page Aide de l'app l'affiche (`/aide`, via `/api/guide`) et Claude la lit avec l'outil MCP `lire_guide`.

- **Avant de coder une fonctionnalité demandée**, lis le guide : elle existe peut-être déjà. Si c'est le cas, dis-le et montre où elle se trouve.
- **Toute PR qui ajoute, change ou retire une fonctionnalité visible met à jour le guide** dans la même PR : où la trouver, ce qu'elle fait, ses réglages par défaut. Un outil MCP ajouté va dans la liste « Outils » du guide, dans `web/src/views/ClaudeView.vue` et dans le test des outils (`oauth_test.go`).
- Style du guide : tutoiement, phrases courtes, chemins de menus avec « › ». Seulement `##`, listes `-`, **gras** et `code` : la page Aide n'affiche rien d'autre.

## Autres règles

- Interface : suivre `DESIGN.md`. Textes en français, tutoiement.
- Tests : `cd api && gofmt -l . && go vet ./... && go test ./...` puis `cd web && npm run build`.
- Prod : le déploiement ne recopie ni `compose.yaml` ni `deploiement/deployer.sh` ni le `.env`. Éviter d'ajouter des variables d'environnement ; si c'est nécessaire, l'écrire clairement dans la PR.
- Le moins d'API payantes possible.
