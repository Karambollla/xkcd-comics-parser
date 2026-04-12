package initiator

import (
	"context"
	"time"

	"log/slog"
)

type Indexer interface {
	Rebuild(ctx context.Context) error
}

type Initiator struct {
	index Indexer
	ttl   time.Duration
	log   *slog.Logger
}

func New(index Indexer, ttl string, log *slog.Logger) *Initiator {
	d, err := time.ParseDuration(ttl)
	if err != nil {
		log.Error("invalid ttl", "error", err)
		return nil
	}
	return &Initiator{index: index, ttl: d, log: log}
}

func (it *Initiator) Start(ctx context.Context) {
	t := time.NewTicker(it.ttl)
	defer t.Stop()
	if err := it.index.Rebuild(ctx); err != nil {
		it.log.Error("initial rebuild failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			it.log.Info("initiator stopping")
			return
		case <-t.C:
			if err := it.index.Rebuild(ctx); err != nil {
				it.log.Error("periodic rebuild failed", "error", err)
			}
		}
	}
}
