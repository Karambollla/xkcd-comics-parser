package middleware

import (
	"log/slog"
	"net/http"
	"strings"
)

type TokenVerifier interface {
	Verify(token string) error
}

func TokenFromRequest(r *http.Request) string {
	query := r.Header.Get("Authorization")
	if query != "" {
		token := strings.TrimPrefix(query, "Token ")
		token = strings.TrimSpace(token)
		if token != "" {
			return token
		}
	}

	cookie, err := r.Cookie("token")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func Auth(next http.HandlerFunc, verifier TokenVerifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := TokenFromRequest(r)
		slog.Info("auth token extracted", "present", token != "")
		if token == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}

		if err := verifier.Verify(token); err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
