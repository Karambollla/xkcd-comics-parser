package words

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"github.com/Karambollla/course/api/core"
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
	req := &wordspb.WordsRequest{Phrase: phrase}
	resp, err := c.client.Norm(ctx, req)
	if err != nil {
		if status.Code(err) == codes.ResourceExhausted {
			return nil, core.ErrBadArguments
		}
		return nil, err
	}
	return resp.GetWords(), nil
}

func (c Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, nil)
	return err
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
