package middleware

import (
	"net/http"

	"go.uber.org/ratelimit"
)

func Rate(next http.HandlerFunc, rps int) http.HandlerFunc {
	if rps <= 0 {
		rps = 1
	}
	limiter := ratelimit.New(rps)
	return func(w http.ResponseWriter, r *http.Request) {
		limiter.Take()
		next(w, r)
	}
}
