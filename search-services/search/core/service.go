package core

import (
	"context"
	"log/slog"
)

type SearchService struct {
	db    DBReader
	words Words
	log   *slog.Logger
	index Indexer
}

func (s *SearchService) Search(ctx context.Context, query string, limit int) ([]Comics, int, error) {
	s.log.Info("starting search")
	words, err := s.words.Norm(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	s.log.Info("limit", "value", limit)
	comics, err := s.db.IDs(ctx, words, limit)
	if err != nil {
		return nil, 0, err
	}
	if len(comics) > limit {
		comics = comics[:limit]
	}
	return comics, len(comics), nil
}

func (s *SearchService) Rebuild(ctx context.Context) error {
	s.log.Info("starting rebuild")
	return s.index.Rebuild(ctx)
}

func (s *SearchService) SearchIndex(ctx context.Context, query string, limit int) ([]Comics, int, error) {
	words, err := s.words.Norm(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	return s.index.Search(ctx, words, limit)
}

func NewService(log *slog.Logger, db DBReader, words Words, index Indexer) (*SearchService, error) {
	return &SearchService{db: db, words: words, log: log, index: index}, nil
}
