package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/Karambollla/course/api/core"
	"github.com/VictoriaMetrics/metrics"
)

type PingResponse struct {
	Replies map[string]string `json:"replies"`
}

type SearchReply struct {
	Comics []core.Comics `json:"comics"`
	Total  int           `json:"total"`
}

type UpdateStatsResponse struct {
	WordsTotal    int `json:"words_total"`
	WordsUnique   int `json:"words_unique"`
	ComicsFetched int `json:"comics_fetched"`
	ComicsTotal   int `json:"comics_total"`
}
type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type Authenticator interface {
	Login(user, password string) (string, error)
}

const minLim = 10

func NewMetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics.WritePrometheus(w, true)
	}
}

func NewPingHandler(log *slog.Logger, pingers map[string]core.Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		replies := make(map[string]string)
		var wg sync.WaitGroup
		var mu sync.Mutex

		slog.Info("starting pinging...")
		for name, pinger := range pingers {
			slog.Info("pinging service", "name", name)
			wg.Go(func() {
				err := pinger.Ping(r.Context())
				mu.Lock()
				if err != nil {
					slog.Error("failed pinging", "service", name)
					replies[name] = "unavailable"
				} else {
					replies[name] = "ok"
				}
				mu.Unlock()
			})
		}

		wg.Wait()

		resp := PingResponse{Replies: replies}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			slog.Error("fail json conversion", "error", err)
		}
	}
}

func NewLoginHandler(log *slog.Logger, auth Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			log.Error("failed to decode login request", "error", err)
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		token, err := auth.Login(req.Name, req.Password)
		log.Info("token is:", "token", token)
		if err != nil {
			log.Info("failed login attempt", "name", req.Name)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, err = w.Write([]byte(token))
		if err != nil {
			log.Error("failed to write token", "error", err)
			return
		}
		log.Info("successful login", "name", req.Name)
	}
}

func NewUpdateHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Updating")
		err := updater.Update(r.Context())
		if err != nil {
			if errors.Is(err, core.ErrAlreadyExists) {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			log.Error("failed to start update", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		log.Info("Update successful")
		w.WriteHeader(http.StatusOK)
	}
}

func NewUpdateStatsHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Fetching stats")
		stats, err := updater.Stats(r.Context())
		response := UpdateStatsResponse{
			WordsTotal:    stats.WordsTotal,
			ComicsFetched: stats.ComicsFetched,
			WordsUnique:   stats.WordsUnique,
			ComicsTotal:   stats.ComicsTotal,
		}
		if err != nil {
			log.Error("failed fetching stats", "error", err)
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			slog.Error("fail json convertion", "error", err)
		}
	}
}

func NewUpdateStatusHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Getting status")
		status, err := updater.Status(r.Context())
		if err != nil {
			log.Error("failed fetching status", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"status": string(status)}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error("failed to encode response", "error", err)
		}
	}
}

func NewDropHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Dropping")
		err := updater.Drop(r.Context())
		if err != nil {
			log.Error("failed to drop database", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func NewSearchHandler(log *slog.Logger, searcher core.Searcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("starting search...")
		phrase := r.URL.Query().Get("phrase")
		if phrase == "" {
			http.Error(w, "phrase is empty", http.StatusBadRequest)
			return
		}

		limitStr := r.URL.Query().Get("limit")
		limit := minLim
		if limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err != nil || parsedLimit <= 0 {
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}
			limit = parsedLimit
		}

		slog.Info("current limit", "limit", limit)

		var err error
		comics, total, err := searcher.Search(r.Context(), phrase, limit)
		if err != nil {
			http.Error(w, "error searching", http.StatusBadRequest)
			return
		}
		resp := SearchReply{
			Comics: comics,
			Total:  total,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		if err != nil {
			slog.Error("fail json conversion", "error", err)
			return
		}
		slog.Info("search successful", "total", resp.Total)
	}
}

func NewSearchIndexHandler(log *slog.Logger, searcher core.Searcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("starting index search...")
		phrase := r.URL.Query().Get("phrase")
		if phrase == "" {
			http.Error(w, "phrase is empty", http.StatusBadRequest)
			return
		}

		limitStr := r.URL.Query().Get("limit")
		limit := minLim
		if limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err != nil || parsedLimit <= 0 {
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}
			limit = parsedLimit
		}

		slog.Info("current limit", "limit", limit)

		comics, total, err := searcher.SearchIndex(r.Context(), phrase, limit)
		if err != nil {
			http.Error(w, "error searching", http.StatusBadRequest)
			return
		}
		resp := SearchReply{
			Comics: comics,
			Total:  total,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		if err != nil {
			slog.Error("fail json conversion", "error", err)
			return
		}
		slog.Info("index search successful", "total", resp.Total)
	}
}

func NewISearchHandler(log *slog.Logger, searcher core.Searcher) http.HandlerFunc {
	return NewSearchIndexHandler(log, searcher)
}
