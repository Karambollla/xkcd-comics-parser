package db

import (
	"context"
	"log/slog"

	"github.com/Karambollla/course/update/core"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type DB struct {
	log  *slog.Logger
	conn *sqlx.DB
}

func New(log *slog.Logger, address string) (*DB, error) {

	db, err := sqlx.Connect("pgx", address)
	if err != nil {
		log.Error("connection problem", "address", address, "error", err)
		return nil, err
	}

	return &DB{
		log:  log,
		conn: db,
	}, nil
}

func (db *DB) Add(ctx context.Context, comics core.Comics) error {
	comic := core.Comics{
		ID:    comics.ID,
		URL:   comics.URL,
		Words: comics.Words,
	}
	_, err := db.conn.NamedExecContext(ctx,
		"INSERT INTO comics (id, url, words) VALUES (:id, :url, :words)", comic)
	return err
}

func (db *DB) Stats(ctx context.Context) (core.DBStats, error) {
	var stats core.DBStats
	db.log.Info("starting fetching stats | db adapter")

	query := `
		SELECT 
			COUNT(*) AS comics_fetched,
			COALESCE(SUM(cardinality(words)), 0) AS words_total,
			COALESCE((SELECT COUNT(DISTINCT w) FROM comics, unnest(words) AS w), 0) AS words_unique
		FROM comics
	`

	err := db.conn.GetContext(ctx, &stats, query)
	if err != nil {
		db.log.Error("couldnt fetch stats", "error", err)
		return core.DBStats{}, err
	}
	db.log.Info("successful stat fetch | db adapter")
	return stats, nil
}

func (db *DB) IDs(ctx context.Context) ([]int, error) {
	var ids []int
	err := db.conn.SelectContext(ctx, &ids, "SELECT id FROM comics ORDER BY id")
	return ids, err
}

func (db *DB) Drop(ctx context.Context) error {
	_, err := db.conn.ExecContext(ctx, "TRUNCATE TABLE comics")
	return err
}
