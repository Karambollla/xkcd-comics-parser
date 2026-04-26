package events

import (
	"context"
	"log/slog"

	"github.com/nats-io/nats.go"
)

const (
	eventsSubject = "xkcd.db.*"
)

type Rebuilder interface {
	Rebuild(ctx context.Context) error
}

type Subscriber struct {
	nc        *nats.Conn
	sub       *nats.Subscription
	rebuilder Rebuilder
	log       *slog.Logger
}

func NewSubscriber(brokerAddress string, rebuilder Rebuilder, log *slog.Logger) (*Subscriber, error) {
	nc, err := nats.Connect(brokerAddress)
	if err != nil {
		log.Error("couldnt connect to nats", "error", err)
		return nil, err
	}

	sub, err := nc.SubscribeSync(eventsSubject)
	if err != nil {
		nc.Close()
		log.Error("couldnt subscribe to nats", "error", err)
		return nil, err
	}

	return &Subscriber{nc: nc, sub: sub, rebuilder: rebuilder, log: log}, nil
}

func (s *Subscriber) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		msg, err := s.sub.NextMsgWithContext(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.log.Error("cannot get next nats message", "error", err)
			continue
		}

		s.log.Info("received message", "subject", msg.Subject)
		if err := s.rebuilder.Rebuild(ctx); err != nil {
			s.log.Error("rebuild failed", "subject", msg.Subject, "error", err)
		}
	}
}

func (s *Subscriber) Close() error {
	if s.sub != nil {
		if err := s.sub.Unsubscribe(); err != nil {
			s.log.Warn("failed to unsubscribe nats", "error", err)
			return err
		}
	}

	if s.nc != nil {
		if err := s.nc.Drain(); err != nil {
			s.log.Warn("failed to drain nats", "error", err)
			s.nc.Close()
			return err
		}
		s.nc.Close()
	}

	return nil
}
