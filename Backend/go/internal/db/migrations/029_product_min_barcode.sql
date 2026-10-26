-- Existencia mínima (aviso de «poco stock») y código de barras del producto (para buscar y cobrar con lector).
ALTER TABLE products ADD COLUMN min_stock INT NOT NULL DEFAULT 0 CHECK (min_stock >= 0);
ALTER TABLE products ADD COLUMN barcode TEXT NOT NULL DEFAULT '' CHECK (length(barcode) <= 64);
CREATE UNIQUE INDEX products_barcode_key ON products (business_id, barcode) WHERE barcode <> '';
