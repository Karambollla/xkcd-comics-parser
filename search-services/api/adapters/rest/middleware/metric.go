package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/felixge/httpsnoop"
)

func WithMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := httpsnoop.CaptureMetrics(next, w, r)
		duration := time.Since(start)

		status := rw.Code
		url := r.URL.Path
		hist := metrics.GetOrCreateHistogram(
			fmt.Sprintf(`http_request_duration_seconds{status=%q,url=%q}`, strconv.Itoa(status), url),
		)
		hist.Update(duration.Seconds())
	})
}
