package words

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	wordspb "github.com/Karambollla/course/proto/words"
)

type Client struct {
	log    *slog.Logger
	client wordspb.WordsClient
	conn   *grpc.ClientConn
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		client: wordspb.NewWordsClient(conn),
		log:    log,
		conn:   conn,
	}, nil
}

func (c Client) Norm(ctx context.Context, phrase string) ([]string, error) {
	res, err := c.client.Norm(ctx, &wordspb.WordsRequest{Phrase: phrase})
	if err != nil {
		c.log.Error("failed to norm words", "error", err)
		return nil, err
	}
	return res.Words, nil
}

func (c Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	if err != nil {
		c.log.Error("words service ping failed", "error", err)
		return errors.New("words service is unavailable")
	}
	return nil
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
