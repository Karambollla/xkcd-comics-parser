package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	searchpb "github.com/Karambollla/course/proto/search"
	"github.com/Karambollla/course/search/adapters/db"
	"github.com/Karambollla/course/search/adapters/events"
	searchgrpc "github.com/Karambollla/course/search/adapters/grpc"
	"github.com/Karambollla/course/search/adapters/index"
	"github.com/Karambollla/course/search/adapters/initiator"
	"github.com/Karambollla/course/search/adapters/words"
	"github.com/Karambollla/course/search/config"
	"github.com/Karambollla/course/search/core"
)

func CloseOrLog(c io.Closer) {
	if err := c.Close(); err != nil {
		slog.Error("error closing", "error", err)
		return
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	lis, err := net.Listen("tcp", cfg.SearchAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	// words adapter
	words, err := words.NewClient(cfg.WordsAddress, log)
	if err != nil {
		return fmt.Errorf("failed to create words client: %w", err)
	}

	// db adapter for service
	db, err := db.New(log, cfg.DBAddress)
	if err != nil {
		return fmt.Errorf("failed to create db connector: %w", err)
	}

	// index adapter
	idx, err := index.NewIndex(cfg.DBAddress, log)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	initiator := initiator.New(idx, cfg.TTL, log)

	// service
	svc, err := core.NewService(log, db, words, idx)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// events subscriber
	subscriber, err := events.NewSubscriber(cfg.BrokerAddress, svc, log)
	if err != nil {
		log.Warn("failed to create events subscriber, continuing without broker", "error", err)
	}

	// gRPC server
	s := grpc.NewServer()
	searchpb.RegisterSearchServer(s, searchgrpc.NewServer(svc))
	reflection.Register(s)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	defer CloseOrLog(words)
	defer CloseOrLog(db)
	defer func() { _ = idx.Close() }()
	if subscriber != nil {
		defer CloseOrLog(subscriber)
	}
	go initiator.Start(ctx)
	if subscriber != nil {
		go subscriber.Start(ctx)
	}
	go func() {
		<-ctx.Done()
		log.Debug("shutting down gRPC server")
		s.GracefulStop()
	}()

	if err := s.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}

func main() {

	var addr string
	flag.StringVar(&addr, "addr", "config.yaml", "grpc listen address")
	flag.Parse()

	cfg := config.MustLoad(addr)
	log := mustMakeLogger(cfg.LogLevel)
	log.Info("starting search gRPC server", "address", cfg.SearchAddress)

	if err := run(cfg, log); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func mustMakeLogger(logLevel string) *slog.Logger {
	var level slog.Level
	switch logLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
