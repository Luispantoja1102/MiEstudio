# Verificar el correo al registrarse

- **Contexto:** correos falsos y altas en masa. Ya se había hecho igual en MiTiendita (`MADGRISMAD/MiTiendita`, `email-verification.service.js`).
- **Decisión:** el registro pide un código de 6 dígitos que llega al correo. La cuenta solo se crea si lo escriben bien.
- **Reglas:** vale 10 min; 60 s entre envíos; 5 envíos por hora por correo; 5 intentos y se anula; el código se guarda con HMAC (solo el hash) y se gasta al usarlo.
- **Dónde:** `Backend/go/internal/api/signup_verify.go`, ruta `POST /api/auth/register/code`, tabla `signup_codes` (migración 026), pantalla `Frontend/src/routes/registro`.
- **Sin correo configurado** en el servidor no se exige (no hay cómo mandarlo). `SIGNUP_VERIFY=off` lo apaga.
- **Consecuencia:** si el alta falla después de gastar el código (por ejemplo, usuario repetido), hay que pedir otro.
