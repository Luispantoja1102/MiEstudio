# Reglas de trabajo

## 1. Piensa antes de hacer
- Antes de escribir código, di en pocas líneas qué vas a cambiar y por qué.
- Si algo no está claro, pregunta una sola vez. No adivines.

## 2. Hazlo simple
- Elige la solución más corta que funcione. Sin opciones ni código "por si acaso".

## 3. No dupliques
- Antes de crear una función, componente, estilo o ruta, busca si ya existe algo parecido.
- Si existe, reutilízalo. Crea algo nuevo solo si no hay nada que sirva y di por qué.

## 4. Toca solo lo que pido
- Cambia solo lo necesario para la petición. Si ves otro problema, avísame sin arreglarlo.
- No cambies diseño, textos ni formato de lo que no se mencionó.

## 5. Prueba antes de hacer commit
- Corre pruebas y compilación antes de cada commit. Si algo falla, arréglalo o dime qué falló.
- Si el cambio se ve en pantalla, revísalo en pantalla.

## 6. Sincroniza y fusiona después de que funcione
- `git fetch` antes de modificar y otra vez antes de fusionar. Si otro participante subió cambios, mézclalos sin perder trabajo de nadie y vuelve a probar.
- No subas a `main` sin que yo lo diga.

## Conocimiento del proyecto (vault)
El conocimiento vive en `.claude/vault/` (local, no se versiona).
- Antes de empezar una tarea lee `.claude/vault/AGENTS.md` y `.claude/vault/Barbella/Barbella.md`.
- Al terminar una sesión con cambios relevantes, agrega una nota con `Plantillas/Sesion.md` en `Barbella/Sesiones/` y enlázala desde `Barbella.md`.
- Decisiones importantes: `Plantillas/Decision.md` en `Barbella/Decisiones/`.
- Si una nota contradice el código, manda el código: deja la contradicción visible.
