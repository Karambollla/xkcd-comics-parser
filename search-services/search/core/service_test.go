package core

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeDB struct {
	comics []Comics
	err    error
}

type fakeWords struct{}

type fakeIndex struct {
	comics []Comics
	total  int
	err    error
}

func (f *fakeDB) IDs(_ context.Context, comics []string, limit int) ([]Comics, error) {
	return f.comics, f.err
}
func (f *fakeDB) AllComics(_ context.Context) ([]Comics, error) {
	return f.comics, f.err
}
func (f *fakeDB) GetComicsByIDs(_ context.Context, ids []int) ([]Comics, error) {
	return f.comics, f.err
}

func (f *fakeWords) Norm(ctx context.Context, str string) ([]string, error) {
	if str == "error" {
		return nil, errors.New("norm failed")
	}
	return []string{"go", "test"}, nil
}

func (f *fakeIndex) Search(_ context.Context, tokens []string, limit int) ([]Comics, int, error) {
	return f.comics, f.total, f.err
}
func (f *fakeIndex) Rebuild(_ context.Context) error {
	return f.err
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
func TestSearch(t *testing.T) {
	testCases := []struct {
		name           string
		expectedComics []Comics
		expectedTotal  int
		dbErr          error
		wantErr        bool
		query          string
		limit          int
	}{
		{
			name:           "returns comics",
			expectedComics: []Comics{{ID: 1}, {ID: 2}},
			expectedTotal:  2,
			query:          "test",
			limit:          10,
		},
		{
			name:    "returns norm error",
			query:   "error",
			limit:   10,
			wantErr: true,
		},
		{
			name:    "returns db error",
			dbErr:   errors.New("db failed"),
			query:   "test",
			limit:   10,
			wantErr: true,
		},
		{
			name:           "respects limit",
			expectedComics: []Comics{{ID: 1}},
			expectedTotal:  1,
			query:          "test",
			limit:          1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dbComics := []Comics{{ID: 1}, {ID: 2}}
			service := &SearchService{
				db:    &fakeDB{comics: dbComics, err: tc.dbErr},
				words: &fakeWords{},
				log:   testLogger(),
			}
			comics, total, err := service.Search(context.Background(), tc.query, tc.limit)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedComics, comics)
			require.Equal(t, tc.expectedTotal, total)
		})
	}
}

func TestRebuild(t *testing.T) {
	testCases := []struct {
		name     string
		indexErr error
		wantErr  bool
	}{
		{
			name:     "rebuilds successfully",
			indexErr: nil,
			wantErr:  false,
		},
		{
			name:     "returns index error",
			indexErr: errors.New("index failed"),
			wantErr:  true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := &SearchService{
				index: &fakeIndex{err: tc.indexErr},
				log:   testLogger(),
			}
			err := service.Rebuild(context.Background())

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestSearchIndex(t *testing.T) {
	testCases := []struct {
		name           string
		expectedComics []Comics
		expectedTotal  int
		indexErr       error
		wantErr        bool
		query          string
		limit          int
	}{
		{
			name:           "returns comics",
			expectedComics: []Comics{{ID: 1}, {ID: 2}},
			expectedTotal:  2,
			query:          "test",
			limit:          10,
		},
		{
			name:    "returns norm error",
			query:   "error",
			limit:   10,
			wantErr: true,
		},
		{
			name:     "returns index error",
			indexErr: errors.New("index failed"),
			query:    "test",
			limit:    10,
			wantErr:  true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			indexComics := []Comics{{ID: 1}, {ID: 2}}
			service := &SearchService{
				words: &fakeWords{},
				index: &fakeIndex{comics: indexComics, total: len(indexComics), err: tc.indexErr},
				log:   testLogger(),
			}
			comics, total, err := service.SearchIndex(context.Background(), tc.query, tc.limit)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedComics, comics)
			require.Equal(t, tc.expectedTotal, total)
		})
	}
}
