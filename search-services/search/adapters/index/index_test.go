package index

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/Karambollla/course/search/core"
)

type fakeDB struct {
	allComicsErr error
	getErr       error
	closed       bool
	allComics    []core.Comics
	requestedIDs []int
}

func (f *fakeDB) IDs(context.Context, []string, int) ([]core.Comics, error) {
	return nil, nil
}

func (f *fakeDB) AllComics(context.Context) ([]core.Comics, error) {
	return f.allComics, f.allComicsErr
}

func (f *fakeDB) GetComicsByIDs(_ context.Context, ids []int) ([]core.Comics, error) {
	f.requestedIDs = ids
	if f.getErr != nil {
		return nil, f.getErr
	}

	byID := make(map[int]core.Comics)
	for _, c := range f.allComics {
		byID[c.ID] = c
	}

	comics := make([]core.Comics, 0, len(ids))
	for _, id := range ids {
		comics = append(comics, byID[id])
	}
	return comics, nil
}

func (f *fakeDB) Close() error {
	f.closed = true
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestIndex(db core.DBReader) *Index {
	return &Index{
		Tokens: make(map[string][]int),
		db:     db,
		log:    testLogger(),
	}
}

func TestRebuild(t *testing.T) {
	testCases := []struct {
		name           string
		db             *fakeDB
		expectedTokens map[string][]int
		expectedErr    error
	}{
		{
			name: "builds token index",
			db: &fakeDB{allComics: []core.Comics{
				{
					ID:    2,
					Words: []string{"go", "db", "go"},
				},
				{
					ID:    1,
					Words: []string{"go", "api"},
				},
			}},
			expectedTokens: map[string][]int{
				"go":  {1, 2},
				"db":  {2},
				"api": {1},
			},
		},
		{
			name:        "returns db error",
			db:          &fakeDB{allComicsErr: errors.New("all comics failed")},
			expectedErr: errors.New("all comics failed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			index := newTestIndex(tc.db)

			err := index.Rebuild(context.Background())

			if tc.expectedErr != nil {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedTokens, index.Tokens)
		})
	}
}

func TestSearch(t *testing.T) {
	testCases := []struct {
		name           string
		tokens         map[string][]int
		queryTokens    []string
		limit          int
		db             *fakeDB
		expectedIDs    []int
		expectedTotal  int
		expectedComics int
		expectedErr    error
	}{
		{
			name:        "returns ranked comics",
			tokens:      map[string][]int{"go": {1, 2}, "api": {1, 3}},
			queryTokens: []string{"go", "api"},
			limit:       2,
			db: &fakeDB{allComics: []core.Comics{
				{
					ID:  1,
					URL: "url-1",
				},
				{
					ID:  2,
					URL: "url-2",
				},
				{
					ID:  3,
					URL: "url-3",
				},
			}},
			expectedIDs:    []int{1, 2},
			expectedTotal:  3,
			expectedComics: 2,
		},
		{
			name:           "limit can be bigger than total",
			tokens:         map[string][]int{"go": {1}},
			queryTokens:    []string{"go"},
			limit:          10,
			db:             &fakeDB{allComics: []core.Comics{{ID: 1, URL: "url-1"}}},
			expectedIDs:    []int{1},
			expectedTotal:  1,
			expectedComics: 1,
		},
		{
			name:          "returns empty result for empty tokens",
			queryTokens:   nil,
			limit:         10,
			db:            &fakeDB{},
			expectedTotal: 0,
		},
		{
			name:          "returns empty result when nothing found",
			tokens:        map[string][]int{"go": {1}},
			queryTokens:   []string{"missing"},
			limit:         10,
			db:            &fakeDB{},
			expectedTotal: 0,
		},
		{
			name:          "returns db error",
			tokens:        map[string][]int{"go": {1}},
			queryTokens:   []string{"go"},
			limit:         10,
			db:            &fakeDB{getErr: errors.New("get failed")},
			expectedIDs:   []int{1},
			expectedTotal: 1,
			expectedErr:   errors.New("get failed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.tokens == nil {
				tc.tokens = make(map[string][]int)
			}
			index := &Index{Tokens: tc.tokens, db: tc.db, log: testLogger()}

			comics, total, err := index.Search(context.Background(), tc.queryTokens, tc.limit)

			if tc.expectedErr != nil {
				require.Error(t, err)
				require.Equal(t, tc.expectedIDs, tc.db.requestedIDs)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedTotal, total)
			require.Equal(t, tc.expectedIDs, tc.db.requestedIDs)
			require.Len(t, comics, tc.expectedComics)
		})
	}
}

func TestClose(t *testing.T) {
	db := &fakeDB{}
	index := newTestIndex(db)

	err := index.Close()

	require.NoError(t, err)
	require.True(t, db.closed)
}

func TestCloseWithoutCloser(t *testing.T) {
	index := newTestIndex(noCloseDB{})

	err := index.Close()

	require.NoError(t, err)
}

type noCloseDB struct{}

func (noCloseDB) IDs(context.Context, []string, int) ([]core.Comics, error) {
	return nil, nil
}

func (noCloseDB) AllComics(context.Context) ([]core.Comics, error) {
	return nil, nil
}

func (noCloseDB) GetComicsByIDs(context.Context, []int) ([]core.Comics, error) {
	return nil, nil
}
