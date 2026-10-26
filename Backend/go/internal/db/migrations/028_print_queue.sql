-- Cola de impresión entre dispositivos: el celular cobra y manda el ticket; la computadora que tiene la impresora
-- (la «estación») lo recoge y lo imprime. Cada sucursal tiene su estación.
CREATE TABLE print_stations (
  business_id  BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  station_id   TEXT NOT NULL,
  branch_id    BIGINT NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  name         TEXT NOT NULL DEFAULT '',
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (business_id, station_id)
);
CREATE INDEX print_stations_branch ON print_stations (branch_id, last_seen_at DESC);

CREATE TABLE print_jobs (
  id          BIGSERIAL PRIMARY KEY,
  business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  branch_id   BIGINT NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  sale_id     BIGINT NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
  open_drawer BOOLEAN NOT NULL DEFAULT false,
  status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'claimed', 'done', 'failed')),
  station_id  TEXT NOT NULL DEFAULT '',
  error       TEXT NOT NULL DEFAULT '',
  created_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  claimed_at  TIMESTAMPTZ
);
CREATE INDEX print_jobs_open ON print_jobs (business_id, branch_id, status, id);
