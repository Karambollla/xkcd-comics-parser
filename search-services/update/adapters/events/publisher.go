package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/Karambollla/course/update/core"
)

const (
	updatedSubject = "xkcd.db.updated"
	droppedSubject = "xkcd.db.dropped"
	flushTimeout   = 2 * time.Second
)

type natsConn interface {
	Publish(subject string, data []byte) error
	FlushTimeout(timeout time.Duration) error
	Close()
}

type Publisher struct {
	nc  natsConn
	log *slog.Logger
}

func NewPublisher(brokerAddress string, log *slog.Logger) (*Publisher, error) {
	nc, err := nats.Connect(brokerAddress)
	if err != nil {
		return nil, err
	}
	return &Publisher{nc: nc, log: log}, nil
}

func (p *Publisher) PublishUpdated(ctx context.Context) error {
	return p.publish(ctx, updatedSubject, []byte("XKCD DB has been updated"))
}

func (p *Publisher) PublishDropped(ctx context.Context) error {
	return p.publish(ctx, droppedSubject, []byte("XKCD DB has been dropped"))
}

func (p *Publisher) publish(ctx context.Context, subject string, payload []byte) error {
	if p == nil || p.nc == nil {
		return core.ErrFailedPublish
	}

	for {
		select {
		default:
			p.log.Info("sending message to subscribers", "subject", subject)
			err := p.nc.Publish(subject, payload)
			if err != nil {
				p.log.Error("could not publish message", "error", err)
				return err
			}
			if err := p.nc.FlushTimeout(flushTimeout); err != nil {
				p.log.Error("could not flush messages", "error", err)
				return err
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *Publisher) Close() error {
	if p != nil && p.nc != nil {
		p.nc.Close()
		p.log.Info("nats closed")
		return nil
	}
	return nil
}
