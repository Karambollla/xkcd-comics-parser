package initiator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeIndex struct{}

func (fi *fakeIndex) Rebuild(ctx context.Context) error {
	return nil
}

type countingIndex struct {
	calls chan struct{}
}

func (c *countingIndex) Rebuild(ctx context.Context) error {
	c.calls <- struct{}{}
	return nil
}

func TestInitiatorNew(t *testing.T) {
	testCases := []struct {
		name    string
		ttl     string
		wantNil bool
		wantTTL time.Duration
	}{
		{
			name:    "valid ttl",
			ttl:     "1h",
			wantTTL: time.Hour,
		},
		{
			name:    "invalid ttl",
			ttl:     "invalid",
			wantNil: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			initiator := New(&fakeIndex{}, tc.ttl, testLogger())

			if tc.wantNil {
				assert.Nil(t, initiator)
				return
			}
			assert.NotNil(t, initiator)
			assert.Equal(t, tc.wantTTL, initiator.ttl)
		})
	}
}

func TestInitiatorStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	index := &countingIndex{calls: make(chan struct{}, 2)}
	initiator := New(index, "10ms", testLogger())
	done := make(chan struct{})

	go func() {
		initiator.Start(ctx)
		close(done)
	}()

	select {
	case <-index.calls:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("initial rebuild was not called")
	}

	select {
	case <-index.calls:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("periodic rebuild was not called")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("initiator did not stop")
	}
}
