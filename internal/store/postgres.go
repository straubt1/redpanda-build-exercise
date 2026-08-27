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
	EventID      string     `json:"event_id"`
	Repo         string     `json:"repo"`
	PRNumber     int        `json:"pr_number"`
	Title        string     `json:"title"`
	PRURL        string     `json:"pr_url"`
	Author       string     `json:"author"`
	Action       string     `json:"action"`
	Category     string     `json:"category"`
	Source       string     `json:"source"`
	Rationale    string     `json:"rationale"`
	Error        string     `json:"error"`
	Confidence   *float64   `json:"confidence"`
	AffectedArea string     `json:"affected_area"`
	ReceivedAt   *time.Time `json:"received_at"`
	ClassifiedAt time.Time  `json:"classified_at"`
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

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

var sortColumns = map[string]string{
	"classified_at": "classified_at",
	"category":      "category",
	"repo":          "repo",
	"title":         "title",
	"confidence":    "confidence",
	"source":        "source",
	"pr_number":     "pr_number",
	"author":        "author",
	"event_id":      "event_id",
}

const (
	DefaultSort = "classified_at"
	DefaultDir  = "desc"
)

// NormalizeOrder allowlists sort identifiers. ok is false if sort is set but unknown.
func NormalizeOrder(sort, dir string) (col, direction string, ok bool) {
	if sort == "" {
		sort = DefaultSort
	}
	col, ok = sortColumns[sort]
	if !ok {
		return DefaultSort, DefaultDir, false
	}
	switch dir {
	case "asc", "desc":
		direction = dir
	default:
		direction = DefaultDir
	}
	return col, direction, true
}

func (s *Store) List(ctx context.Context, sortCol, dir string) ([]Row, error) {
	col, direction, ok := NormalizeOrder(sortCol, dir)
	if !ok {
		col, direction = DefaultSort, DefaultDir
	}
	q := fmt.Sprintf(`
SELECT event_id, repo, pr_number, title, pr_url, author, action,
  category, source, rationale, error, confidence, affected_area, received_at, classified_at
FROM pr_triages
ORDER BY %s %s NULLS LAST
LIMIT 200
`, col, direction)
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		var r Row
		if err := rows.Scan(
			&r.EventID, &r.Repo, &r.PRNumber, &r.Title, &r.PRURL, &r.Author, &r.Action,
			&r.Category, &r.Source, &r.Rationale, &r.Error, &r.Confidence, &r.AffectedArea,
			&r.ReceivedAt, &r.ClassifiedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
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
