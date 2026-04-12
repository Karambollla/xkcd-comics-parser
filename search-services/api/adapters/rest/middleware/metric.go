package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

type responseWriter struct {
	http.ResponseWriter
	code int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.code = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func WithMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, code: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rw, r)
		duration := time.Since(start)

		status := strconv.Itoa(rw.code)
		url := r.URL.Path
		hist := metrics.GetOrCreateHistogram(
			fmt.Sprintf(`http_request_duration_seconds{status=%q,url=%q}`, status, url),
		)
		hist.Update(duration.Seconds())
	})
}
