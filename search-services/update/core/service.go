package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Service struct {
	log         *slog.Logger
	db          DB
	xkcd        XKCD
	words       Words
	publisher   EventPublisher
	concurrency int
	mu          sync.Mutex
	status      ServiceStatus
}

const failedComicId = 404
const failedComicURL = "https://xkcd.com/404/"
const failedComicText = "404 not found"

func NewService(
	log *slog.Logger, db DB, xkcd XKCD, words Words, concurrency int, publisher EventPublisher,
) (*Service, error) {
	if concurrency < 1 {
		return nil, fmt.Errorf("wrong concurrency specified: %d", concurrency)
	}
	if publisher == nil {
		return nil, fmt.Errorf("event publisher must not be nil")
	}
	return &Service{
		log:         log,
		db:          db,
		xkcd:        xkcd,
		words:       words,
		publisher:   publisher,
		concurrency: concurrency,
		status:      StatusIdle,
	}, nil
}

func (s *Service) Update(ctx context.Context) error {
	s.status = StatusRunning
	if ok := s.mu.TryLock(); !ok {
		s.log.Error("service already runs update or drop")
		return ErrAlreadyExists
	}

	s.status = StatusRunning
	s.log.Info("Update started")

	defer func() {
		s.status = StatusIdle
		s.mu.Unlock()
	}()

	existing := make(map[int]bool)
	ids, err := s.db.IDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		existing[id] = true
	}

	var missing []int
	lastId, err := s.xkcd.LastID(ctx)
	if err != nil {
		return err
	}

	for id := 1; id <= lastId; id++ {
		if !existing[id] {
			missing = append(missing, id)
		}
	}

	if len(missing) == 0 {
		s.log.Info("no new comics to fetch")
		return nil
	}

	s.log.Info("fetching missing comics", "count", len(missing))
	start := time.Now()
	s.log.Info("start of fetching")
	tasks := make(chan int, len(missing))
	for _, id := range missing {
		tasks <- id
	}
	close(tasks)
	errs := make(chan error, len(missing))
	var wg sync.WaitGroup

	for i := 0; i < s.concurrency; i++ {
		wg.Go(func() {
			for id := range tasks {
				if err := ctx.Err(); err != nil {
					return
				}

				if id == failedComicId {
					words, err := s.words.Norm(ctx, failedComicText)
					if err != nil {
						s.log.Error("goroutine failed norming 404 comic", "id", id, "error", err)
						errs <- err
						continue
					}

					if err := s.db.Add(ctx, Comics{
						ID:    failedComicId,
						URL:   failedComicURL,
						Words: words,
					}); err != nil {
						s.log.Error("goroutine failed adding 404 comic", "id", id, "error", err)
						errs <- err
					}
					continue
				}

				info, err := s.xkcd.Get(ctx, id)
				if err != nil {
					if errors.Is(err, ErrNotFound) {
						continue
					}
					s.log.Error("goroutine failed getting", "id", id, "error", err)
					errs <- err
					continue
				}

				words, err := s.words.Norm(ctx, info.Description)
				if err != nil {
					s.log.Error("goroutine failed norming", "id", id, "error", err)
					errs <- err
					continue
				}

				if err := s.db.Add(ctx, Comics{
					ID:    info.ID,
					URL:   info.URL,
					Words: words,
				}); err != nil {
					s.log.Error("goroutine failed adding", "id", id, "error", err)
					errs <- err
					continue
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	elapsed := time.Since(start)
	s.log.Info("time passed", "time", elapsed.String())
	var firstError error
	for err := range errs {
		if firstError == nil {
			firstError = err
		}
	}
	if firstError == nil {
		if err := s.publisher.PublishUpdated(ctx); err != nil {
			return ErrFailedPublish
		}
		s.log.Info("Update finished successfully")
	}
	return firstError
}

func (s *Service) Stats(ctx context.Context) (ServiceStats, error) {
	stat, err := s.db.Stats(ctx)
	if err != nil {
		s.log.Error("failed parsing stats", "error", err)
		return ServiceStats{}, err
	}

	lastID, err := s.xkcd.LastID(ctx)
	s.log.Info("lastID | service core", "lastid", lastID)
	if err != nil {
		s.log.Error("failed parsing lastID", "error", err)
		return ServiceStats{}, err
	}

	return ServiceStats{
		ComicsTotal: lastID,
		DBStats:     stat,
	}, nil
}

func (s *Service) Status(ctx context.Context) ServiceStatus {
	return s.status
}

func (s *Service) Drop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == StatusRunning {
		return ErrDropUpdate
	}

	err := s.db.Drop(ctx)
	if err != nil {
		return err
	}

	if err := s.publisher.PublishDropped(ctx); err != nil {
		return ErrFailedPublish
	}

	s.log.Info("DB dropped successfully")
	return nil
}
