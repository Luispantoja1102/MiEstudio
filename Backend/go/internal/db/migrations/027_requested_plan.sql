-- El plan que la persona pidió al registrarse. Mientras la plataforma no lo aprueba, el negocio prueba el plan Básico.
-- Al activar ese plan desde la plataforma, la solicitud se borra.
ALTER TABLE businesses ADD COLUMN requested_plan TEXT CHECK (requested_plan IN ('individual', 'basico', 'crecimiento', 'pro'));
