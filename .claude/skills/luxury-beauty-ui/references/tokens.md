# Tokens

## CSS
```css
:root {
  --linen: #F8F6F2;        /* lienzo (alternativa: #FBF8F5) */
  --charcoal: #1F1E1C;     /* barra lateral; alternativas: espresso #2D2522, salvia #1E2822 */
  --surface: #FFFFFF;
  --line: #E6E1DA;
  --shadow: 0 1px 2px rgba(0,0,0,.03), 0 8px 24px -12px rgba(0,0,0,.05);
  --terracotta: #C07A65;   /* rellenos, puntos, bordes */
  --terracotta-deep: #A8614C;  /* botones con texto blanco */
  --terracotta-ink: #8A4A38;   /* texto sobre tinte */
  --terracotta-tint: #F4E4DE;
  --olive: #9A9E8B;  --olive-tint: #ECEDE6;  --olive-ink: #5F634F;
  --rose: #D2A08B;   --rose-tint: #F6E8E1;
  --ink: #1F1E1C;    --muted: #6B665F;
}
```

## Tailwind
```js
theme: { extend: { colors: {
  linen: '#F8F6F2', cream: '#FBF8F5',
  charcoal: '#1F1E1C', espresso: '#2D2522', sage: '#1E2822',
  line: '#E6E1DA',
  terracotta: { DEFAULT: '#C07A65', deep: '#A8614C', ink: '#8A4A38', tint: '#F4E4DE' },
  olive: { DEFAULT: '#9A9E8B', tint: '#ECEDE6', ink: '#5F634F' },
  rose: { DEFAULT: '#D2A08B', tint: '#F6E8E1' },
}, boxShadow: { soft: '0 1px 2px rgba(0,0,0,.03), 0 8px 24px -12px rgba(0,0,0,.05)' },
fontFamily: { sans: ['Inter','sans-serif'], serif: ['Playfair Display','Cormorant Garamond','serif'] } } }
```
