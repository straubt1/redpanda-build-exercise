package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

type Row struct {
	EventID      string
	Repo         string
	PRNumber     int
	Title        string
	PRURL        string
	Author       string
	Action       string
	Category     string
	Source       string
	Rationale    string
	Error        string
	Confidence   *float64
	AffectedArea string
	ReceivedAt   *time.Time
}

func Connect(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Upsert(ctx context.Context, row Row) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO pr_triages (
  event_id, repo, pr_number, title, pr_url, author, action,
  category, source, rationale, error, confidence, affected_area, received_at, classified_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14, now())
ON CONFLICT (event_id) DO UPDATE SET
  repo = EXCLUDED.repo,
  pr_number = EXCLUDED.pr_number,
  title = EXCLUDED.title,
  pr_url = EXCLUDED.pr_url,
  author = EXCLUDED.author,
  action = EXCLUDED.action,
  category = EXCLUDED.category,
  source = EXCLUDED.source,
  rationale = EXCLUDED.rationale,
  error = EXCLUDED.error,
  confidence = EXCLUDED.confidence,
  affected_area = EXCLUDED.affected_area,
  received_at = EXCLUDED.received_at,
  classified_at = now()
`, row.EventID, row.Repo, row.PRNumber, row.Title, row.PRURL, row.Author, row.Action,
		row.Category, row.Source, row.Rationale, row.Error, row.Confidence, row.AffectedArea, row.ReceivedAt)
	return err
}
