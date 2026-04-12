package update

import (
	"context"
	"log/slog"

	"github.com/Karambollla/course/api/core"
	updatepb "github.com/Karambollla/course/proto/update"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Client struct {
	log    *slog.Logger
	client updatepb.UpdateClient
	conn   *grpc.ClientConn
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		client: updatepb.NewUpdateClient(conn),
		log:    log,
		conn:   conn,
	}, nil
}

func (c Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	return err
}

func (c Client) Status(ctx context.Context) (core.UpdateStatus, error) {
	resp, err := c.client.Status(ctx, &emptypb.Empty{})
	if err != nil {
		return "", err
	}
	if resp.Status == updatepb.Status_STATUS_RUNNING {
		return core.StatusUpdateRunning, nil
	}
	return core.StatusUpdateIdle, nil
}

func (c Client) Stats(ctx context.Context) (core.UpdateStats, error) {
	stats, err := c.client.Stats(ctx, &emptypb.Empty{})
	if err != nil {
		return core.UpdateStats{}, nil
	}
	return core.UpdateStats{
		WordsTotal:    int(stats.WordsTotal),
		WordsUnique:   int(stats.WordsUnique),
		ComicsFetched: int(stats.ComicsFetched),
		ComicsTotal:   int(stats.ComicsTotal),
	}, err

}

func (c Client) Update(ctx context.Context) error {
	_, err := c.client.Update(ctx, &emptypb.Empty{})
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return core.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (c Client) Drop(ctx context.Context) error {
	_, err := c.client.Drop(ctx, &emptypb.Empty{})
	return err
}

func (c *Client) Close() error {
	err := c.conn.Close()
	return err
}
