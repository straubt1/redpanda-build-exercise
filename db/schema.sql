-- Applied on first Postgres boot only (empty volume).
CREATE TABLE IF NOT EXISTS pr_triages (
  event_id        TEXT PRIMARY KEY,
  repo            TEXT NOT NULL,
  pr_number       INTEGER NOT NULL,
  title           TEXT NOT NULL DEFAULT '',
  pr_url          TEXT NOT NULL DEFAULT '',
  author          TEXT NOT NULL DEFAULT '',
  action          TEXT NOT NULL DEFAULT 'opened',
  category        TEXT NOT NULL,
  confidence      DOUBLE PRECISION,
  rationale       TEXT NOT NULL DEFAULT '',
  affected_area   TEXT NOT NULL DEFAULT '',
  summary         TEXT NOT NULL DEFAULT '',
  source          TEXT NOT NULL DEFAULT 'unknown',
  error           TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ,
  classified_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS pr_triages_classified_at ON pr_triages (classified_at DESC);
