#!/usr/bin/env bash
# Met Microdash à jour sur le serveur. C'est la seule commande que la clé SSH de GitHub peut
# lancer (« command= » dans authorized_keys, voir le README) : même volée, elle ne donne ni
# terminal ni accès aux fichiers.
#
#   deployer <version>   ex. deployer sha-1a2b3c4 : sauvegarde la base, installe cette version,
#                        vérifie que le site répond, revient à la précédente sinon.
#   etat                 affiche la version en ligne.
#
# Sur l'entrée : une ligne facultative avec un jeton de lecture de ghcr.io (celui du workflow,
# valable quelques minutes), pour tirer les images si elles sont privées.
#
# Le script vit dans <dossier>/deploiement/ ; <dossier> contient compose.yaml et .env.
set -euo pipefail

DOSSIER="$(cd "$(dirname "$(readlink -f "$0")")/.." && pwd)"
SAUVEGARDES="$DOSSIER/sauvegardes"
GARDER=10 # sauvegardes gardées
cd "$DOSSIER"

# Appelé par SSH : la commande demandée est dans SSH_ORIGINAL_COMMAND ; à la main : les arguments.
read -r -a demande <<< "${SSH_ORIGINAL_COMMAND:-$*}"
action="${demande[0]:-}"
version="${demande[1]:-}"

journal() { printf '[%s] %s\n' "$(date '+%H:%M:%S')" "$*"; }
echec() { printf '::error::%s\n' "$*" >&2; exit 1; }

valeur_env() { grep -E "^$1=" .env | tail -n 1 | cut -d= -f2- || true; }
fixer_version() {
  if grep -qE '^MICRODASH_VERSION=' .env; then
    sed -i "s/^MICRODASH_VERSION=.*/MICRODASH_VERSION=$1/" .env
  else
    printf '\nMICRODASH_VERSION=%s\n' "$1" >> .env
  fi
}

[ -f compose.yaml ] && [ -f .env ] || echec "compose.yaml ou .env manquant dans $DOSSIER"

case "$action" in
  etat)
    echo "Version en ligne : $(valeur_env MICRODASH_VERSION)"
    docker compose ps --format '{{.Service}} {{.Status}}'
    exit 0
    ;;
  deployer) ;;
  *) echec "commande refusée : « ${SSH_ORIGINAL_COMMAND:-$*} » (deployer <version> ou etat)" ;;
esac

[[ "$version" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] || echec "version invalide : « $version »"

# Un seul déploiement à la fois.
exec 9> "$DOSSIER/.deploiement.lock"
flock -n 9 || echec "un autre déploiement est en cours"

# Jeton facultatif sur l'entrée, pour ghcr.io.
jeton=""
if [ ! -t 0 ]; then IFS= read -r -t 5 jeton || true; fi
if [ -n "$jeton" ]; then
  printf '%s' "$jeton" | docker login ghcr.io -u github --password-stdin > /dev/null
  trap 'docker logout ghcr.io > /dev/null 2>&1 || true' EXIT
fi

precedente="$(valeur_env MICRODASH_VERSION)"
journal "Version en ligne : ${precedente:-aucune} → nouvelle : $version"

journal "Téléchargement des images…"
MICRODASH_VERSION="$version" docker compose pull --quiet api web

if [ -n "$(docker compose ps --status running -q db 2> /dev/null)" ]; then
  mkdir -p "$SAUVEGARDES"
  fichier="$SAUVEGARDES/$(date +%Y%m%d-%H%M%S)-avant-$version.sql.gz"
  journal "Sauvegarde de la base : $(basename "$fichier")"
  docker compose exec -T db pg_dump -U microdash microdash | gzip > "$fichier"
  ls -1t "$SAUVEGARDES"/*.sql.gz | tail -n +$((GARDER + 1)) | xargs -r rm -f --
fi

fixer_version "$version"
journal "Démarrage de $version…"
docker compose up -d --remove-orphans

# Le site répond-il ? L'API passe par le front, comme pour un visiteur.
port="$(valeur_env WEB_PORT)"
url="http://127.0.0.1:${port:-8080}/api/health"
for _ in $(seq 1 30); do
  if curl -fsS -o /dev/null "$url"; then
    journal "Le site répond : $version est en ligne."
    docker image prune -f > /dev/null
    exit 0
  fi
  sleep 2
done

journal "Le site ne répond pas après 60 s."
docker compose logs --tail 30 api >&2 || true
if [ -n "$precedente" ] && [ "$precedente" != "$version" ]; then
  journal "Retour à $precedente."
  fixer_version "$precedente"
  docker compose up -d --remove-orphans
  echec "$version ne démarre pas : $precedente est remise. Si une migration de la base a tourné, la sauvegarde d'avant est dans sauvegardes/."
fi
echec "$version ne démarre pas."
