package search

import (
	"context"
	"log/slog"

	"github.com/Karambollla/course/api/core"
	searchpb "github.com/Karambollla/course/proto/search"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Client struct {
	log    *slog.Logger
	client searchpb.SearchClient
	conn   *grpc.ClientConn
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		client: searchpb.NewSearchClient(conn),
		log:    log,
		conn:   conn,
	}, nil
}

func (c *Client) Search(ctx context.Context, query string, limit int) ([]core.Comics, int, error) {
	req := searchpb.SearchRequest{
		Words: query,
		Limit: int32(limit),
	}
	reply, err := c.client.Search(ctx, &req)
	if err != nil {
		return nil, 0, err
	}

	result := make([]core.Comics, 0, len(reply.Comics))

	for _, pbComic := range reply.Comics {
		if pbComic == nil {
			continue
		}

		result = append(result, core.Comics{
			ID:  int(pbComic.Id),
			URL: pbComic.Url,
		})
	} // might be wrong, 2 different comics structures

	return result, int(reply.Total), nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	return err
}

func (c *Client) SearchIndex(ctx context.Context, query string, limit int) ([]core.Comics, int, error) {
	req := searchpb.SearchRequest{
		Words: query,
		Limit: int32(limit),
	}
	reply, err := c.client.SearchIndex(ctx, &req)
	if err != nil {
		return nil, 0, err
	}

	result := make([]core.Comics, 0, len(reply.Comics))

	for _, pbComic := range reply.Comics {
		if pbComic == nil {
			continue
		}

		result = append(result, core.Comics{
			ID:  int(pbComic.Id),
			URL: pbComic.Url,
		})
	}

	return result, int(reply.Total), nil
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
