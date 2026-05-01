package events

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeRebuilder struct {
	err   error
	calls chan struct{}
}

func (f *fakeRebuilder) Rebuild(context.Context) error {
	f.calls <- struct{}{}
	return f.err
}

type fakeSubscription struct {
	msgs           chan *nats.Msg
	err            error
	unsubscribeErr error
	unsubscribed   bool
}

func (f *fakeSubscription) NextMsgWithContext(ctx context.Context) (*nats.Msg, error) {
	if f.err != nil {
		return nil, f.err
	}

	select {
	case msg := <-f.msgs:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *fakeSubscription) Unsubscribe() error {
	f.unsubscribed = true
	return f.unsubscribeErr
}

type fakeConn struct {
	drainErr error
	drained  bool
	closed   bool
}

func (f *fakeConn) Drain() error {
	f.drained = true
	return f.drainErr
}

func (f *fakeConn) Close() {
	f.closed = true
}

func TestStart(t *testing.T) {
	testCases := []struct {
		name       string
		rebuildErr error
	}{
		{
			name: "rebuilds on message",
		},
		{
			name:       "continues when rebuild returns error",
			rebuildErr: errors.New("rebuild failed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sub := &fakeSubscription{msgs: make(chan *nats.Msg, 1)}
			rebuilder := &fakeRebuilder{err: tc.rebuildErr, calls: make(chan struct{}, 1)}
			subscriber := &Subscriber{sub: sub, rebuilder: rebuilder, log: testLogger()}
			done := make(chan struct{})

			go func() {
				subscriber.Start(ctx)
				close(done)
			}()

			sub.msgs <- &nats.Msg{Subject: "xkcd.db.updated"}

			select {
			case <-rebuilder.calls:
			case <-time.After(100 * time.Millisecond):
				t.Fatal("rebuild was not called")
			}

			cancel()
			select {
			case <-done:
			case <-rebuilder.calls:
				t.Fatal("subscriber rebuilt after cancellation")
			case <-time.After(100 * time.Millisecond):
				t.Fatal("subscriber did not stop")
			}
		})
	}
}

func TestStartWithNilContext(t *testing.T) {
	sub := &fakeSubscription{msgs: make(chan *nats.Msg, 1)}
	rebuilder := &fakeRebuilder{calls: make(chan struct{}, 1)}
	subscriber := &Subscriber{sub: sub, rebuilder: rebuilder, log: testLogger()}

	go subscriber.Start(context.Background())
	sub.msgs <- &nats.Msg{Subject: "xkcd.db.updated"}

	select {
	case <-rebuilder.calls:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("rebuild was not called")
	}
}

func TestClose(t *testing.T) {
	testCases := []struct {
		name        string
		unsubErr    error
		drainErr    error
		expectedErr error
	}{
		{
			name: "closes subscriber",
		},
		{
			name:        "returns unsubscribe error",
			unsubErr:    errors.New("unsubscribe failed"),
			expectedErr: errors.New("unsubscribe failed"),
		},
		{
			name:        "returns drain error",
			drainErr:    errors.New("drain failed"),
			expectedErr: errors.New("drain failed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sub := &fakeSubscription{unsubscribeErr: tc.unsubErr}
			nc := &fakeConn{drainErr: tc.drainErr}
			subscriber := &Subscriber{sub: sub, nc: nc, log: testLogger()}

			err := subscriber.Close()

			if tc.expectedErr != nil {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.True(t, sub.unsubscribed)
			require.True(t, nc.drained)
			require.True(t, nc.closed)
		})
	}
}
