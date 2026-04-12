package grpc

import (
	"context"

	searchpb "github.com/Karambollla/course/proto/search"
	"github.com/Karambollla/course/search/core"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

func NewServer(svc core.Searcher) *Server {
	return &Server{service: svc}
}

type Server struct {
	searchpb.UnimplementedSearchServer
	service core.Searcher
}

func (s *Server) Rebuild(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, s.service.Rebuild(ctx)
}

func (s *Server) Ping(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *Server) Search(ctx context.Context, req *searchpb.SearchRequest) (*searchpb.SearchReply, error) {
	res, total, err := s.service.Search(ctx, req.Words, int(req.Limit))
	if err != nil {
		return nil, err
	}

	return makeSearchReply(res, total), nil
}

func (s *Server) SearchIndex(ctx context.Context, req *searchpb.SearchRequest) (*searchpb.SearchReply, error) {
	res, total, err := s.service.SearchIndex(ctx, req.Words, int(req.Limit))
	if err != nil {
		return nil, err
	}

	return makeSearchReply(res, total), nil
}

func makeSearchReply(res []core.Comics, total int) *searchpb.SearchReply {
	comics := make([]*searchpb.Comic, 0)
	for _, c := range res {
		comics = append(comics, &searchpb.Comic{
			Id:  int32(c.ID),
			Url: c.URL,
		})
	}

	return &searchpb.SearchReply{
		Comics: comics,
		Total:  int32(total),
	}
}
