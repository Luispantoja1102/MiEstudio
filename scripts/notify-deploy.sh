#!/usr/bin/env bash
# Manda el correo de estado del deploy. Corre EN EL VPS (lo llama deploy.yml por SSH),
# así que el correo sale desde la IP del servidor, no desde el runner de GitHub Actions.
# Credenciales en el .env del servidor (no se tocan desde fuera): MAIL_USERNAME, MAIL_PASSWORD
# (contraseña de aplicación de Gmail). Opcionales: MAIL_TO (varios separados por coma; por defecto
# los mismos tres de MiTiendita), MAIL_SERVER, PROD_URL.
#
# Uso: notify-deploy.sh <success|failure> "<mensaje commit>" "<sha>" "<autor>" "<rama>" "<commit_url>" "<run_url>"
set -euo pipefail
cd "$(dirname "$0")/.."
[ -f .env ] && set -a && source .env && set +a

RESULT="$1" MESSAGE="${2:-(sin mensaje)}" SHA="${3:-}" AUTHOR="${4:-desconocido}" BRANCH="${5:-main}" COMMIT_URL="${6:-}" RUN_URL="${7:-}"
SERVICE="Barbella"
SHORT="${SHA:0:7}"
MAIL_TO="${MAIL_TO:-mayra.bamaca09@gmail.com,madgrismad@gmail.com,luispantoja1102@gmail.com}"

if [ -z "${MAIL_USERNAME:-}" ] || [ -z "${MAIL_PASSWORD:-}" ]; then
  echo "Faltan MAIL_USERNAME/MAIL_PASSWORD en .env: no se manda correo."
  exit 0
fi

if [ "$RESULT" = success ]; then
  SUBJECT="✅ $SERVICE: Deploy publicado ($SHORT)"
  LEAD="La versión nueva de $SERVICE ya está publicada en el servidor y respondió bien."
  LINKS="<a href=\"${PROD_URL:-}\">Ver producción</a> · <a href=\"$COMMIT_URL\">Ver el commit</a>"
else
  SUBJECT="❌ $SERVICE: El deploy falló ($SHORT)"
  LEAD="No se pudo publicar $SERVICE. Producción puede seguir con la versión anterior: revisa el registro."
  LINKS="<a href=\"$RUN_URL\">Ver el registro del error</a> · <a href=\"$COMMIT_URL\">Ver el commit</a>"
fi

BODY="<p>$LEAD</p><p>Commit: $MESSAGE<br>SHA: $SHORT<br>Autor: $AUTHOR<br>Rama: $BRANCH</p><p>$LINKS</p><p>— $SERVICE</p>"

RCPT_ARGS=()
IFS=',' read -ra ADDRS <<< "$MAIL_TO"
for a in "${ADDRS[@]}"; do RCPT_ARGS+=(--mail-rcpt "$(echo "$a" | xargs)"); done

{
  printf 'From: %s <%s>\r\n' "$SERVICE" "$MAIL_USERNAME"
  printf 'To: %s\r\n' "$MAIL_TO"
  printf 'Subject: %s\r\n' "$SUBJECT"
  printf 'MIME-Version: 1.0\r\n'
  printf 'Content-Type: text/html; charset=UTF-8\r\n\r\n'
  printf '%s\r\n' "$BODY"
} | curl -s --url "smtps://${MAIL_SERVER:-smtp.gmail.com}:465" --ssl-reqd \
    --mail-from "$MAIL_USERNAME" "${RCPT_ARGS[@]}" \
    --user "$MAIL_USERNAME:$MAIL_PASSWORD" --upload-file -
