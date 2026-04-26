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

	updatepb "github.com/Karambollla/course/proto/update"
	"github.com/Karambollla/course/update/adapters/db"
	"github.com/Karambollla/course/update/adapters/events"
	updategrpc "github.com/Karambollla/course/update/adapters/grpc"
	"github.com/Karambollla/course/update/adapters/words"
	"github.com/Karambollla/course/update/adapters/xkcd"
	"github.com/Karambollla/course/update/config"
	"github.com/Karambollla/course/update/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func CloseOrLog(c io.Closer, log *slog.Logger) {
	if err := c.Close(); err != nil {
		log.Error("failed to close resource", "error", err)
	}
}

func main() {

	// config
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()
	cfg := config.MustLoad(configPath)

	// logger
	log := mustMakeLogger(cfg.LogLevel)

	if err := run(cfg, log); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	log.Info("starting server")
	log.Debug("debug messages are enabled")

	// database adapter
	storage, err := db.New(log, cfg.DBAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to db: %v", err)
	}
	if err := storage.Migrate(); err != nil {
		return fmt.Errorf("failed to migrate db: %v", err)
	}

	// xkcd adapter
	xkcd, err := xkcd.NewClient(cfg.XKCD.URL, cfg.XKCD.Timeout, log)
	if err != nil {
		return fmt.Errorf("failed create XKCD client: %v", err)
	}

	// words adapter
	words, err := words.NewClient(cfg.WordsAddress, log)
	if err != nil {
		return fmt.Errorf("failed create Words client: %v", err)
	}

	// broker adapter
	publisher, err := events.NewPublisher(cfg.BrokerAddress, log)
	if err != nil {
		log.Warn("failed to create nats connection, continuing without broker", "error", err)
		publisher = nil
	}

	// service
	updater, err := core.NewService(log, storage, xkcd, words, cfg.XKCD.Concurrency, publisher)
	if err != nil {
		return fmt.Errorf("failed create Update service: %v", err)
	}

	// grpc server
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	updatepb.RegisterUpdateServer(s, updategrpc.NewServer(updater))
	reflection.Register(s)

	// context for Ctrl-C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if publisher != nil {
		defer CloseOrLog(publisher, log)
	}
	defer CloseOrLog(words, log)

	go func() {
		<-ctx.Done()
		log.Debug("shutting down server")
		s.GracefulStop()
	}()

	if err := s.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}
	return nil
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
		panic("unknown log level: " + logLevel)
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
