---
name: luxury-beauty-ui
description: Dirección de diseño UI/UX de lujo para software SaaS de salones, spas, clínicas estéticas y estudios de belleza (agenda, caja, clientes). Úsala siempre que el usuario pida diseñar, rediseñar, elegir paleta, tipografía, layout, CSS/Tailwind o prompts de imagen (Midjourney, v0, DALL-E) para una plataforma de belleza o bienestar con estética editorial tipo Aesop, Chanel Beauty, Le Labo o Glossier, o cuando quiera alejarse del aspecto de ERP/Office/administrativo; incluso si solo dice "que se vea más elegante, premium o de lujo" o "mockup del dashboard".
---

# Luxury Beauty UI

Rol: director de UI/UX y de marca para software de belleza y bienestar de gama alta. Objetivo: que el producto se sienta como una marca de cosmética de lujo (editorial, sereno, cálido) y no como un panel administrativo.

## Por qué estas reglas
Un software de belleza lo ven a diario dueños y clientes de negocios que venden *atmósfera*. Blanco clínico, azul corporativo y verde tipo hoja de cálculo comunican "oficina". Los tonos cálidos, el aire entre elementos y la serif en los saludos comunican cuidado. Cada regla abajo existe para sostener esa percepción.

## Paleta
Valores exactos (úsalos tal cual; ver `references/tokens.md` para CSS y Tailwind):
- **Elementos oscuros** (barra lateral, cabecera): carbón cálido `#1F1E1C`, espresso `#2D2522` o salvia oscura `#1E2822`. Nunca azul marino plano ni negro frío.
- **Lienzo**: lino `#F8F6F2` o crema `#FBF8F5`. Nunca `#FFFFFF` como fondo de página.
- **Acentos** (apagados, sin neón): terracota suave `#C07A65`, oliva suave `#9A9E8B`, rosa polvo `#D2A08B`.
- **Superficies**: `#FFFFFF`, borde `#E6E1DA`, sombra ambiental `rgba(0,0,0,0.03)`.

Contraste: `#C07A65` con texto blanco queda en ~3.2:1 (no pasa AA para texto normal). Mantén `#C07A65` para rellenos, puntos, bordes y fondos de insignia; para botones con texto blanco o texto de insignia usa su versión profunda `#A8614C` (≈5:1) y para texto sobre fondo tinte `#8A4A38`. Así se conserva el color de marca y el texto sigue legible.

## Layout y componentes
- **Espacio**: generoso. Filas de 88–104 px, padding de tarjeta 24–32 px, separación entre bloques 32–48 px. Si se siente apretado, está mal.
- **Insignias**: píldora mínima, fondo tintado suave, texto de alto contraste (nunca texto del mismo tono que el fondo).
- **Botones**: píldora o `rounded-lg`, transición corta (150–200 ms). Primario en acento cálido; secundario con contorno suave. Estado `:active` con escala 0.97.
- **Tipografía**: sans limpia (Inter, Plus Jakarta Sans u Outfit) para interfaz y serif de alto contraste (Playfair Display o Cormorant Garamond) solo para saludos, títulos de página y cifras de tiempo. Cuerpo ≥ 15 px, microtexto ≥ 12 px.
- **Íconos**: trazo fino (1.25–1.5 px), un solo set, nunca emoji.
- **Una sola forma de elevación**: borde sutil + sombra ambiental muy baja; nunca borde fuerte con sombra pesada.
- Evita: acento lateral grueso en tarjetas, degradados en texto, tarjetas idénticas con ícono + título + texto como estructura de página, rejillas de métricas gigantes ("hero metric").

## Cómo trabajar
1. Parte de `assets/dashboard-mock.html` (dashboard de ejemplo con todos los tokens ya aplicados) y adapta contenido real del producto. Se abre solo, sin build.
2. Para mostrar el resultado, renderiza con Playwright a 1920×1080 y entrega la imagen. Si el usuario dijo "no lo apliques", no toques el código de la aplicación: el mockup vive aparte.
3. Si piden Tailwind, entrega `tailwind.config` con los hex exactos de `references/tokens.md`.
4. Si piden un prompt para Midjourney/v0/DALL·E, usa la plantilla de `references/prompts.md`; esos generadores no producen UI exacta, así que ofrécelo como complemento y no como sustituto del mockup en HTML.
5. Verifica antes de entregar: contraste de texto (≥ 4.5:1), ningún `#FFFFFF` de fondo de página, respiración en filas, un solo acento dominante por pantalla.
