// Bancos y servicios de pago donde se reciben transferencias en México; "Otro" deja escribir el nombre.
export const BANKS = [
  'BBVA', 'Banorte', 'Santander', 'Citibanamex', 'HSBC', 'Scotiabank', 'Inbursa', 'Banco Azteca', 'BanCoppel', 'Afirme',
  'BanBajío', 'Banregio', 'Banco del Bienestar', 'Multiva', 'Invex', 'Mifel', 'Nu México', 'Mercado Pago', 'Hey Banco',
  'Spin by OXXO', 'Klar', 'Stori', 'Ualá', 'Revolut', 'STP'
];
export const clabeSpaced = (c) => (c ?? '').replace(/(\d{4})(?=\d)/g, '$1 ');
