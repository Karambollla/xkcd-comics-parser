package core

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeDB struct {
	ids     []int
	idsErr  error
	stats   DBStats
	statErr error
	dropErr error
	addErr  error
	added   []Comics
}

func (f *fakeDB) Add(_ context.Context, comic Comics) error {
	f.added = append(f.added, comic)
	return f.addErr
}

func (f *fakeDB) Stats(context.Context) (DBStats, error) {
	return f.stats, f.statErr
}

func (f *fakeDB) Drop(context.Context) error {
	return f.dropErr
}

func (f *fakeDB) IDs(context.Context) ([]int, error) {
	return f.ids, f.idsErr
}

type fakeXKCD struct {
	lastID    int
	lastIDErr error
	comics    map[int]XKCDInfo
	getErr    map[int]error
}

func (f fakeXKCD) Get(_ context.Context, id int) (XKCDInfo, error) {
	if err := f.getErr[id]; err != nil {
		return XKCDInfo{}, err
	}
	return f.comics[id], nil
}

func (f fakeXKCD) LastID(context.Context) (int, error) {
	return f.lastID, f.lastIDErr
}

type fakeWords struct {
	err error
}

func (f fakeWords) Norm(_ context.Context, phrase string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []string{phrase}, nil
}

type fakePublisher struct {
	updateErr error
	dropErr   error
	updated   bool
	dropped   bool
}

type blockingDB struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingDB) Add(context.Context, Comics) error {
	return nil
}

func (b *blockingDB) Stats(context.Context) (DBStats, error) {
	return DBStats{}, nil
}

func (b *blockingDB) Drop(context.Context) error {
	return nil
}

func (b *blockingDB) IDs(context.Context) ([]int, error) {
	b.once.Do(func() {
		close(b.started)
	})
	<-b.release
	return nil, nil
}

func (f *fakePublisher) PublishUpdated(context.Context) error {
	f.updated = true
	return f.updateErr
}

func (f *fakePublisher) PublishDropped(context.Context) error {
	f.dropped = true
	return f.dropErr
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestService(t *testing.T, db DB, xkcd XKCD, words Words, publisher EventPublisher) *Service {
	t.Helper()

	service, err := NewService(testLogger(), db, xkcd, words, 1, publisher)
	require.NoError(t, err)
	return service
}

func TestNewService(t *testing.T) {
	testCases := []struct {
		name        string
		concurrency int
		publisher   EventPublisher
		wantErr     bool
	}{
		{
			name:        "returns service",
			concurrency: 1,
			publisher:   &fakePublisher{},
		},
		{
			name:        "rejects bad concurrency",
			concurrency: 0,
			publisher:   &fakePublisher{},
			wantErr:     true,
		},
		{
			name:        "rejects nil publisher",
			concurrency: 1,
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service, err := NewService(testLogger(), &fakeDB{}, fakeXKCD{}, fakeWords{}, tc.concurrency, tc.publisher)

			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, service)
				return
			}
			require.NoError(t, err)
			require.Equal(t, StatusIdle, service.Status(context.Background()))
		})
	}
}

func TestUpdate(t *testing.T) {
	testCases := []struct {
		name        string
		db          *fakeDB
		xkcd        fakeXKCD
		words       fakeWords
		publisher   *fakePublisher
		expectedErr error
		expectedAdd int
		published   bool
	}{
		{
			name:        "adds missing comics and publishes",
			db:          &fakeDB{ids: []int{1}},
			xkcd:        fakeXKCD{lastID: 3, comics: map[int]XKCDInfo{2: {ID: 2, URL: "url-2", Description: "desc-2"}, 3: {ID: 3, URL: "url-3", Description: "desc-3"}}},
			words:       fakeWords{},
			publisher:   &fakePublisher{},
			expectedAdd: 2,
			published:   true,
		},
		{
			name:      "skips not found comics and publishes",
			db:        &fakeDB{},
			xkcd:      fakeXKCD{lastID: 1, comics: map[int]XKCDInfo{}, getErr: map[int]error{1: ErrNotFound}},
			words:     fakeWords{},
			publisher: &fakePublisher{},
			published: true,
		},
		{
			name:        "returns ids error",
			db:          &fakeDB{idsErr: errors.New("ids failed")},
			xkcd:        fakeXKCD{},
			words:       fakeWords{},
			publisher:   &fakePublisher{},
			expectedErr: errors.New("ids failed"),
		},
		{
			name:        "returns norm error",
			db:          &fakeDB{},
			xkcd:        fakeXKCD{lastID: 1, comics: map[int]XKCDInfo{1: {ID: 1, URL: "url", Description: "desc"}}},
			words:       fakeWords{err: errors.New("norm failed")},
			publisher:   &fakePublisher{},
			expectedErr: errors.New("norm failed"),
		},
		{
			name:        "returns failed publish",
			db:          &fakeDB{},
			xkcd:        fakeXKCD{lastID: 1, comics: map[int]XKCDInfo{1: {ID: 1, URL: "url", Description: "desc"}}},
			words:       fakeWords{},
			publisher:   &fakePublisher{updateErr: errors.New("publish failed")},
			expectedErr: ErrFailedPublish,
			expectedAdd: 1,
			published:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := newTestService(t, tc.db, tc.xkcd, tc.words, tc.publisher)

			err := service.Update(context.Background())

			if tc.expectedErr != nil {
				require.Error(t, err)
				if errors.Is(tc.expectedErr, ErrFailedPublish) {
					require.ErrorIs(t, err, tc.expectedErr)
				}
			} else {
				require.NoError(t, err)
			}
			require.Len(t, tc.db.added, tc.expectedAdd)
			require.Equal(t, tc.published, tc.publisher.updated)
			require.Equal(t, StatusIdle, service.Status(context.Background()))
		})
	}
}

func TestUpdateReturnsAlreadyExistsWhenUpdateIsRunning(t *testing.T) {
	db := &blockingDB{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	service := newTestService(t, db, fakeXKCD{}, fakeWords{}, &fakePublisher{})
	firstDone := make(chan error, 1)

	go func() {
		firstDone <- service.Update(context.Background())
	}()

	select {
	case <-db.started:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("first update did not start")
	}

	err := service.Update(context.Background())

	require.ErrorIs(t, err, ErrAlreadyExists)

	close(db.release)
	require.NoError(t, <-firstDone)
	require.Equal(t, StatusIdle, service.Status(context.Background()))
}

func TestStats(t *testing.T) {
	dbStats := DBStats{WordsTotal: 10, WordsUnique: 7, ComicsFetched: 3}
	service := newTestService(t, &fakeDB{stats: dbStats}, fakeXKCD{lastID: 5}, fakeWords{}, &fakePublisher{})

	stats, err := service.Stats(context.Background())

	require.NoError(t, err)
	require.Equal(t, dbStats, stats.DBStats)
	require.Equal(t, 5, stats.ComicsTotal)
}

func TestStatsError(t *testing.T) {
	testCases := []struct {
		name string
		db   *fakeDB
		xkcd fakeXKCD
	}{
		{
			name: "returns db stats error",
			db:   &fakeDB{statErr: errors.New("stats failed")},
			xkcd: fakeXKCD{lastID: 5},
		},
		{
			name: "returns last id error",
			db:   &fakeDB{},
			xkcd: fakeXKCD{lastIDErr: errors.New("last id failed")},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := newTestService(t, tc.db, tc.xkcd, fakeWords{}, &fakePublisher{})

			_, err := service.Stats(context.Background())

			require.Error(t, err)
		})
	}
}

func TestDrop(t *testing.T) {
	testCases := []struct {
		name        string
		db          *fakeDB
		publisher   *fakePublisher
		expectedErr error
		published   bool
	}{
		{
			name:      "drops db and publishes",
			db:        &fakeDB{},
			publisher: &fakePublisher{},
			published: true,
		},
		{
			name:        "returns drop error",
			db:          &fakeDB{dropErr: errors.New("drop failed")},
			publisher:   &fakePublisher{},
			expectedErr: errors.New("drop failed"),
		},
		{
			name:        "returns failed publish",
			db:          &fakeDB{},
			publisher:   &fakePublisher{dropErr: errors.New("publish failed")},
			expectedErr: ErrFailedPublish,
			published:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := newTestService(t, tc.db, fakeXKCD{}, fakeWords{}, tc.publisher)

			err := service.Drop(context.Background())

			if tc.expectedErr != nil {
				require.Error(t, err)
				if errors.Is(tc.expectedErr, ErrFailedPublish) {
					require.ErrorIs(t, err, tc.expectedErr)
				}
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.published, tc.publisher.dropped)
		})
	}
}
