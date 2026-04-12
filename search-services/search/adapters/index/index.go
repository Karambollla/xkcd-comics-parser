package index

import (
	"context"
	"log/slog"
	"sort"
	"sync"

	"github.com/Karambollla/course/search/adapters/db"
	"github.com/Karambollla/course/search/core"
)

type Index struct {
	Tokens map[string][]int
	mu     sync.RWMutex
	db     core.DBReader
	log    *slog.Logger
}

func NewIndex(dsn string, log *slog.Logger) (*Index, error) {
	dbConn, err := db.New(log, dsn)
	if err != nil {
		return nil, err
	}
	return &Index{
		Tokens: make(map[string][]int),
		mu:     sync.RWMutex{},
		db:     dbConn,
		log:    log,
	}, nil
}

func (i *Index) Rebuild(ctx context.Context) error {
	comics, err := i.db.AllComics(ctx)
	if err != nil {
		return err
	}
	tmp := make(map[string][]int)
	for _, c := range comics {
		seen := make(map[string]struct{})
		for _, w := range c.Words {
			if _, ok := seen[w]; ok {
				continue
			}
			tmp[w] = append(tmp[w], c.ID)
			seen[w] = struct{}{}
		}
	}
	for k := range tmp {
		sort.Ints(tmp[k])
	}
	i.mu.Lock()
	i.Tokens = tmp
	i.mu.Unlock()
	return nil
}

func (i *Index) Search(ctx context.Context, tokens []string, limit int) ([]core.Comics, int, error) {
	if len(tokens) == 0 {
		return nil, 0, nil
	}
	counts := make(map[int]int)
	var total int
	i.mu.RLock()
	for _, t := range tokens {
		if postings, ok := i.Tokens[t]; ok {
			for _, id := range postings {
				counts[id]++
			}
		}
	}
	total = len(counts)
	i.mu.RUnlock()

	if total == 0 {
		return nil, 0, nil
	}

	type item struct {
		id    int
		score int
	}
	items := make([]item, 0, total)
	for id, sc := range counts {
		items = append(items, item{id: id, score: sc})
	}
	sort.Slice(items, func(a, b int) bool {
		if items[a].score == items[b].score {
			return items[a].id < items[b].id
		}
		return items[a].score > items[b].score
	})
	ids := make([]int, 0, limit)
	for _, it := range items[:limit] {
		ids = append(ids, it.id)
	}
	// ignore score rn
	comics, err := i.db.GetComicsByIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	return comics, total, nil
}

func (i *Index) Close() error {
	if c, ok := i.db.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}
