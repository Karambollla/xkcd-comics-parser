package events

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/Karambollla/course/update/core"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeNATS struct {
	subject string
	payload []byte
	publish error
	flush   error
	closed  bool
	flushed bool
}

func (f *fakeNATS) Publish(subject string, data []byte) error {
	f.subject = subject
	f.payload = data
	return f.publish
}

func (f *fakeNATS) FlushTimeout(time.Duration) error {
	f.flushed = true
	return f.flush
}

func (f *fakeNATS) Close() {
	f.closed = true
}

func TestPublish(t *testing.T) {
	testCases := []struct {
		name       string
		publish    func(context.Context, *Publisher) error
		subject    string
		payload    []byte
		publishErr error
		flushErr   error
	}{
		{
			name:    "publishes updated event",
			publish: func(ctx context.Context, p *Publisher) error { return p.PublishUpdated(ctx) },
			subject: updatedSubject,
			payload: []byte("XKCD DB has been updated"),
		},
		{
			name:    "publishes dropped event",
			publish: func(ctx context.Context, p *Publisher) error { return p.PublishDropped(ctx) },
			subject: droppedSubject,
			payload: []byte("XKCD DB has been dropped"),
		},
		{
			name:       "returns publish error",
			publish:    func(ctx context.Context, p *Publisher) error { return p.PublishUpdated(ctx) },
			subject:    updatedSubject,
			payload:    []byte("XKCD DB has been updated"),
			publishErr: errors.New("publish failed"),
		},
		{
			name:     "returns flush error",
			publish:  func(ctx context.Context, p *Publisher) error { return p.PublishUpdated(ctx) },
			subject:  updatedSubject,
			payload:  []byte("XKCD DB has been updated"),
			flushErr: errors.New("flush failed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nc := &fakeNATS{publish: tc.publishErr, flush: tc.flushErr}
			p := &Publisher{nc: nc, log: testLogger()}

			err := tc.publish(context.Background(), p)

			if tc.publishErr != nil {
				require.ErrorIs(t, err, tc.publishErr)
				require.False(t, nc.flushed)
				return
			}
			if tc.flushErr != nil {
				require.ErrorIs(t, err, tc.flushErr)
				require.True(t, nc.flushed)
				return
			}
			require.NoError(t, err)
			require.True(t, nc.flushed)
			require.Equal(t, tc.subject, nc.subject)
			require.Equal(t, tc.payload, nc.payload)
		})
	}
}

func TestPublishCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	nc := &fakeNATS{}
	p := &Publisher{nc: nc, log: testLogger()}

	err := p.PublishUpdated(ctx)

	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, nc.subject)
}

func TestPublishWithoutConnection(t *testing.T) {
	var p *Publisher

	err := p.PublishUpdated(context.Background())

	require.ErrorIs(t, err, core.ErrFailedPublish)
}

func TestClose(t *testing.T) {
	nc := &fakeNATS{}
	p := &Publisher{nc: nc, log: testLogger()}

	err := p.Close()

	require.NoError(t, err)
	require.True(t, nc.closed)
}

func TestCloseWithoutConnection(t *testing.T) {
	var p *Publisher

	err := p.Close()

	require.NoError(t, err)
}
