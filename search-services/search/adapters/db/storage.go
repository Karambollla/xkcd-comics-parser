package db

import (
	"context"
	"log/slog"

	"github.com/Karambollla/course/search/core"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type DB struct {
	log  *slog.Logger
	conn *sqlx.DB
}

func New(log *slog.Logger, dsn string) (*DB, error) {
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		log.Error("db connect failed", "dsn", dsn, "error", err)
		return nil, err
	}
	return &DB{log: log, conn: db}, nil
}

func (d *DB) Close() error {
	if d.conn == nil {
		return nil
	}
	return d.conn.Close()
}

func (d *DB) IDs(ctx context.Context, words []string, limit int) ([]core.Comics, error) {
	var out []core.Comics
	var total int
	countQuery := `
	SELECT COUNT(*)
	FROM comics c
	CROSS JOIN LATERAL (
		SELECT COUNT(*) AS matches
		FROM unnest(c.words) AS w
		WHERE w = ANY($1::text[])
	) m
	WHERE m.matches > 0
	`
	if err := d.conn.GetContext(ctx, &total, countQuery, words); err != nil {
		d.log.Error("failed to count comics by words", "error", err)
		return nil, err
	}

	// same thing
	query := `
	SELECT c.id, c.url
	FROM comics c
	CROSS JOIN LATERAL (
		SELECT COUNT(*) AS matches
		FROM unnest(c.words) AS w
		WHERE w = ANY($1::text[])
	) m
	WHERE m.matches > 0
	ORDER BY m.matches DESC, c.id ASC
	LIMIT $2
	`

	if err := d.conn.SelectContext(ctx, &out, query, words, limit); err != nil {
		d.log.Error("failed to select comics by words", "error", err)
		return nil, err
	}
	return out, nil
}

func (d *DB) AllComics(ctx context.Context) ([]core.Comics, error) {
	type row struct {
		ID    int            `db:"id"`
		URL   string         `db:"url"`
		Words pq.StringArray `db:"words"`
	}
	var rows []row
	query := `
	SELECT id, url, words
	FROM comics
	`
	if err := d.conn.SelectContext(ctx, &rows, query); err != nil {
		d.log.Error("failed to select all comics", "error", err)
		return nil, err
	}
	d.log.Info("fetched all comics", "count", len(rows))
	out := make([]core.Comics, 0, len(rows))
	for _, r := range rows {
		out = append(out, core.Comics{
			ID:    r.ID,
			URL:   r.URL,
			Words: []string(r.Words),
		})
	}
	return out, nil
}

func (d *DB) GetComicsByIDs(ctx context.Context, ids []int) ([]core.Comics, error) {
	if len(ids) == 0 {
		d.log.Info("no ids provided, returning empty result")
		return nil, nil
	}
	d.log.Info("len id", "len", len(ids))
	type row struct {
		ID    int            `db:"id"`
		URL   string         `db:"url"`
		Words pq.StringArray `db:"words"`
	}
	var rows []row
	query := `
	SELECT id, url, words
	FROM comics
	WHERE id = ANY($1)
	`
	if err := d.conn.SelectContext(ctx, &rows, query, ids); err != nil {
		d.log.Error("failed to select comics by ids", "error", err)
		return nil, err
	}
	out := make([]core.Comics, 0, len(rows))
	for _, r := range rows {
		out = append(out, core.Comics{
			ID:    r.ID,
			URL:   r.URL,
			Words: []string(r.Words),
		})
	}
	return out, nil
}
