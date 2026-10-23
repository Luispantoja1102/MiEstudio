-- Plan Individual (1 persona) y anticipos separados por método: transferencia (Deposits) y Mercado Pago (DepositsMP).
DO $$
DECLARE c text;
BEGIN
  FOR c IN SELECT conname FROM pg_constraint WHERE conrelid = 'businesses'::regclass AND contype = 'c' AND pg_get_constraintdef(oid) LIKE '%plan%basico%' LOOP
    EXECUTE 'ALTER TABLE businesses DROP CONSTRAINT ' || quote_ident(c);
  END LOOP;
  FOR c IN SELECT conname FROM pg_constraint WHERE conrelid = 'plan_settings'::regclass AND contype = 'c' AND pg_get_constraintdef(oid) LIKE '%plan_id%' LOOP
    EXECUTE 'ALTER TABLE plan_settings DROP CONSTRAINT ' || quote_ident(c);
  END LOOP;
END $$;
ALTER TABLE businesses ADD CONSTRAINT businesses_plan_check CHECK (plan IN ('individual', 'basico', 'crecimiento', 'pro'));
ALTER TABLE plan_settings ADD CONSTRAINT plan_settings_plan_id_check CHECK (plan_id IN ('individual', 'basico', 'crecimiento', 'pro'));

-- Hasta ahora "anticipos" incluía Mercado Pago y transferencia: lo ya guardado conserva ese alcance.
ALTER TABLE plan_settings ADD COLUMN deposits_mp BOOLEAN;
UPDATE plan_settings SET deposits_mp = deposits;
ALTER TABLE plan_settings ALTER COLUMN deposits_mp SET NOT NULL, ALTER COLUMN deposits_mp SET DEFAULT false;
