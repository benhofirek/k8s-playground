package main

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

const schema = `
CREATE TABLE IF NOT EXISTS services (
	id               BIGSERIAL PRIMARY KEY,
	name             TEXT        NOT NULL,
	url              TEXT        NOT NULL,
	interval_seconds INT         NOT NULL DEFAULT 30,
	expected_status  INT         NOT NULL DEFAULT 200,
	created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS checks (
	id          BIGSERIAL PRIMARY KEY,
	service_id  BIGINT      NOT NULL REFERENCES services(id) ON DELETE CASCADE,
	checked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
	ok          BOOLEAN     NOT NULL,
	status_code INT         NOT NULL DEFAULT 0,
	latency_ms  INT         NOT NULL DEFAULT 0,
	error       TEXT        NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS checks_service_time_idx ON checks (service_id, checked_at DESC);
`

type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

func (s *Store) Ping(ctx context.Context) error { return s.db.Ping(ctx) }

// Migrate creates the schema. The advisory lock keeps concurrent instances from racing.
func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(727274)"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, schema); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const serviceCols = "id, name, url, interval_seconds, expected_status, created_at"

func scanService(row pgx.Row, v *Service) error {
	return row.Scan(&v.ID, &v.Name, &v.URL, &v.IntervalSeconds, &v.ExpectedStatus, &v.CreatedAt)
}

func (s *Store) CreateService(ctx context.Context, v *Service) error {
	return s.db.QueryRow(ctx,
		`INSERT INTO services (name, url, interval_seconds, expected_status)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		v.Name, v.URL, v.IntervalSeconds, v.ExpectedStatus,
	).Scan(&v.ID, &v.CreatedAt)
}

func (s *Store) ListServices(ctx context.Context) ([]Service, error) {
	rows, err := s.db.Query(ctx, "SELECT "+serviceCols+" FROM services ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Service{}
	for rows.Next() {
		var v Service
		if err := scanService(rows, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) GetService(ctx context.Context, id int64) (*Service, error) {
	var v Service
	err := scanService(s.db.QueryRow(ctx, "SELECT "+serviceCols+" FROM services WHERE id = $1", id), &v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Store) DeleteService(ctx context.Context, id int64) error {
	tag, err := s.db.Exec(ctx, "DELETE FROM services WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) InsertCheck(ctx context.Context, c *Check) error {
	return s.db.QueryRow(ctx,
		`INSERT INTO checks (service_id, checked_at, ok, status_code, latency_ms, error)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		c.ServiceID, c.CheckedAt, c.OK, c.StatusCode, c.LatencyMS, c.Error,
	).Scan(&c.ID)
}

// ListChecks returns the most recent checks, newest first.
func (s *Store) ListChecks(ctx context.Context, serviceID int64, limit int) ([]Check, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, service_id, checked_at, ok, status_code, latency_ms, error
		 FROM checks WHERE service_id = $1 ORDER BY checked_at DESC LIMIT $2`,
		serviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Check{}
	for rows.Next() {
		var c Check
		if err := rows.Scan(&c.ID, &c.ServiceID, &c.CheckedAt, &c.OK, &c.StatusCode, &c.LatencyMS, &c.Error); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// LatestCheck returns nil (and no error) when the service has no checks yet.
func (s *Store) LatestCheck(ctx context.Context, serviceID int64) (*Check, error) {
	cs, err := s.ListChecks(ctx, serviceID, 1)
	if err != nil || len(cs) == 0 {
		return nil, err
	}
	return &cs[0], nil
}

// Uptime24h returns the percentage of successful checks in the last 24 hours.
func (s *Store) Uptime24h(ctx context.Context, serviceID int64) (pct float64, total int, err error) {
	var up int
	err = s.db.QueryRow(ctx,
		`SELECT count(*), count(*) FILTER (WHERE ok)
		 FROM checks WHERE service_id = $1 AND checked_at > now() - interval '24 hours'`,
		serviceID).Scan(&total, &up)
	if err != nil || total == 0 {
		return 0, total, err
	}
	return math.Round(float64(up)/float64(total)*10000) / 100, total, nil
}

func (s *Store) PruneChecks(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := s.db.Exec(ctx,
		"DELETE FROM checks WHERE checked_at < now() - make_interval(secs => $1)",
		olderThan.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
