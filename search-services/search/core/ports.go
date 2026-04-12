package core

import "context"

type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]Comics, int, error)
	SearchIndex(ctx context.Context, query string, limit int) ([]Comics, int, error)
	Rebuild(ctx context.Context) error
}

type Words interface {
	Norm(ctx context.Context, phrase string) ([]string, error)
}

type DBReader interface {
	IDs(ctx context.Context, words []string, limit int) ([]Comics, error)
	AllComics(ctx context.Context) ([]Comics, error)
	GetComicsByIDs(ctx context.Context, ids []int) ([]Comics, error)
}

type Indexer interface {
	Rebuild(ctx context.Context) error
	Search(ctx context.Context, tokens []string, limit int) ([]Comics, int, error)
}
