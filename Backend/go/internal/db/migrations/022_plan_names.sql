-- Planes nuevos: Básico (solo citas, sin cobros), Crecimiento (todo, una sucursal) y Pro (varias sucursales).
-- Esencial → Básico, Profesional → Crecimiento, Estudio → Pro.
DO $$
DECLARE c text;
BEGIN
  SELECT conname INTO c FROM pg_constraint
   WHERE conrelid = 'businesses'::regclass AND contype = 'c' AND pg_get_constraintdef(oid) LIKE '%esencial%';
  IF c IS NOT NULL THEN EXECUTE 'ALTER TABLE businesses DROP CONSTRAINT ' || quote_ident(c); END IF;
END $$;

UPDATE businesses SET plan = CASE plan WHEN 'esencial' THEN 'basico' WHEN 'profesional' THEN 'crecimiento' WHEN 'estudio' THEN 'pro' ELSE plan END;
UPDATE payments SET plan = CASE plan WHEN 'esencial' THEN 'basico' WHEN 'profesional' THEN 'crecimiento' WHEN 'estudio' THEN 'pro' ELSE plan END;
UPDATE subscription_checkouts SET plan = CASE plan WHEN 'esencial' THEN 'basico' WHEN 'profesional' THEN 'crecimiento' WHEN 'estudio' THEN 'pro' ELSE plan END;

ALTER TABLE businesses ALTER COLUMN plan SET DEFAULT 'crecimiento';
ALTER TABLE businesses ADD CONSTRAINT businesses_plan_check CHECK (plan IN ('basico', 'crecimiento', 'pro'));
