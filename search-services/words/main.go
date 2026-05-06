package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net"
	"os"
	"strconv"

	"github.com/ilyakaznacheev/cleanenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	wordspb "github.com/Karambollla/course/proto/words"
	"github.com/Karambollla/course/words/words"
)

const maxPhraseLen = 20000

type server struct {
	wordspb.UnimplementedWordsServer
	log *slog.Logger
}

func (s *server) Ping(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	s.log.InfoContext(ctx, "pong")
	return &emptypb.Empty{}, nil
}

func (s *server) Norm(_ context.Context, in *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
	if len(in.GetPhrase()) > maxPhraseLen {
		return nil, status.Error(
			codes.ResourceExhausted,
			"phrase is large than "+strconv.Itoa(maxPhraseLen),
		)
	}
	return &wordspb.WordsReply{
		Words: words.Norm(in.GetPhrase()),
	}, nil
}

type Config struct {
	Address string `yaml:"words_address" env:"WORDS_ADDRESS" env-default:"80"`
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()

	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic(err)
	}
	slog.Info("starting listening")

	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.Info("creating a server")
	s := grpc.NewServer()
	wordspb.RegisterWordsServer(s, &server{log: log})
	reflection.Register(s)

	if err := s.Serve(listener); err != nil {
		log.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}
