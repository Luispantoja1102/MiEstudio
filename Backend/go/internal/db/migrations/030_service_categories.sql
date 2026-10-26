-- Categorías de servicios (Cabello, Barba, Uñas…) para ordenar el catálogo. Un servicio sin categoría queda «Sin categoría».
CREATE TABLE service_categories (
  id          BIGSERIAL PRIMARY KEY,
  business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  name        TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 40),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX service_categories_name_key ON service_categories (business_id, lower(name));
ALTER TABLE services ADD COLUMN category_id BIGINT REFERENCES service_categories(id) ON DELETE SET NULL;
CREATE INDEX services_category ON services (category_id);
