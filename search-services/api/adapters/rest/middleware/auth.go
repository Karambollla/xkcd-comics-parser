package middleware

import (
	"log/slog"
	"net/http"
	"strings"
)

type TokenVerifier interface {
	Verify(token string) error
}

func Auth(next http.HandlerFunc, verifier TokenVerifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.Header.Get("Authorization")
		slog.Info("header authorization is", "authorization", query)
		if query == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(query, "Token ")
		token = strings.TrimSpace(token)
		if token == "" {
			http.Error(w, "empty token", http.StatusUnauthorized)
			return
		}

		if err := verifier.Verify(token); err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
