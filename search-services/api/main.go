package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/Karambollla/course/api/adapters/aaa"
	"github.com/Karambollla/course/api/adapters/rest"
	"github.com/Karambollla/course/api/adapters/rest/middleware"
	"github.com/Karambollla/course/api/adapters/search"
	"github.com/Karambollla/course/api/adapters/update"
	"github.com/Karambollla/course/api/adapters/words"
	"github.com/Karambollla/course/api/config"
	"github.com/Karambollla/course/api/core"
)

func CloseOrLog(c io.Closer, log *slog.Logger) {
	if err := c.Close(); err != nil {
		log.Error("failed to close resource", "error", err)
	}
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()

	cfg := config.MustLoad(configPath)

	log := mustMakeLogger(cfg.LogLevel)

	log.Info("starting server")
	log.Debug("debug messages are enabled")

	updateClient, err := update.NewClient(cfg.UpdateAddress, log)
	if err != nil {
		log.Error("cannot init update adapter", "error", err)
		os.Exit(1)
	}

	wordsClient, err := words.NewClient(cfg.WordsAddress, log)
	if err != nil {
		log.Error("cannot init words adapter", "error", err)
		os.Exit(1)
	}

	searchClient, err := search.NewClient(cfg.SearchAddress, log)
	if err != nil {
		log.Error("cannot init search adapter", "error", err)
		os.Exit(1)
	}

	AAA, err := aaa.New(cfg.TokenTTL, log)
	if err != nil {
		log.Error("cannot init AAA adapter", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	pingers := map[string]core.Pinger{
		"words":  wordsClient,
		"update": updateClient,
		"search": searchClient,
	}

	mux.Handle("GET /", rest.NewSearchPageHandler(log, searchClient))
	mux.Handle("GET /static/", rest.NewStaticHandler())
	mux.Handle("GET /admin", rest.NewAdminPageHandler(log, AAA, updateClient))
	mux.Handle("POST /admin/login", rest.NewAdminLoginHandler(log, AAA))
	mux.Handle("POST /admin/logout", rest.NewAdminLogoutHandler())
	mux.Handle("POST /admin/update", middleware.Auth(rest.NewAdminUpdateHandler(log, updateClient), AAA))
	mux.Handle("POST /admin/drop", middleware.Auth(rest.NewAdminDropHandler(log, updateClient), AAA))

	mux.Handle("GET /metrics", rest.NewMetricsHandler())
	mux.Handle("POST /api/login", rest.NewLoginHandler(log, AAA))

	searchHandler := middleware.Concurrency(rest.NewSearchHandler(log, searchClient), cfg.SearchConcurrency)
	mux.Handle("GET /api/search", searchHandler)

	isearchHandler := middleware.Rate(rest.NewSearchIndexHandler(log, searchClient), cfg.SearchRate)
	mux.Handle("GET /api/isearch", isearchHandler)

	mux.Handle("GET /api/ping", rest.NewPingHandler(log, pingers))

	updateHandler := middleware.Auth(rest.NewUpdateHandler(log, updateClient), AAA)
	mux.Handle("POST /api/db/update", updateHandler)

	dropHandler := middleware.Auth(rest.NewDropHandler(log, updateClient), AAA)
	mux.Handle("DELETE /api/db", dropHandler)

	mux.Handle("GET /api/db/stats", rest.NewUpdateStatsHandler(log, updateClient))
	mux.Handle("GET /api/db/status", rest.NewUpdateStatusHandler(log, updateClient))

	serverWithMetrics := middleware.WithMetrics(mux)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	defer CloseOrLog(updateClient, log)
	defer CloseOrLog(searchClient, log)
	defer CloseOrLog(wordsClient, log)

	server := http.Server{
		Addr:        cfg.HTTPConfig.Address,
		ReadTimeout: cfg.HTTPConfig.Timeout,
		Handler:     serverWithMetrics,
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}

	go func() {
		<-ctx.Done()
		log.Debug("shutting down server")
		if err := server.Shutdown(context.Background()); err != nil {
			log.Error("erroneous shutdown", "error", err)
		}
	}()

	log.Info("Running HTTP server", "address", cfg.HTTPConfig.Address)
	if err := server.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("server closed unexpectedly", "error", err)
			return
		}
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
		panic("unknown log level: " + logLevel)
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level, AddSource: true})
	return slog.New(handler)
}
