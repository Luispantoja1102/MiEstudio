# Deploy al VPS con correo de estado

- **Contexto:** MiTiendita y MiConsultorio publican en el VPS por SSH y avisan por correo; se copia el mismo esquema.
- **Decisión:** `.github/workflows/deploy.yml` corre cuando el CI de `main` termina en verde: sube el código (rsync), reconstruye con `docker compose` y comprueba `/health`. Un trabajo `notify` manda siempre un correo (éxito o fallo) armado por `scripts/deploy-mail.mjs`.
- **Secretos del repositorio** (nunca en el archivo, el repo es público): `VPS_SSH_KEY`, `VPS_KNOWN_HOSTS`, `VPS_HOST`, `PROD_URL`, `MAIL_USERNAME`, `MAIL_PASSWORD`, `MAIL_TO`. Opcionales: `VPS_PATH` (por defecto `~/barbella`), `VPS_DEPLOY_CMD`, `MAIL_SERVER`.
- **Diferencias con MiTiendita:** sin `rsync --delete` y sin `docker-compose.yml` en el repo, porque el montaje de Barbella en el VPS no está en el repositorio.
- **Probado:** el correo (éxito y fallo) con `node scripts/deploy-mail.mjs --preview` y el YAML. El workflow completo solo se puede probar en GitHub (se puede lanzar a mano con "Run workflow").
