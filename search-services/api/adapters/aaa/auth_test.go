package aaa

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Karambollla/course/api/core"
	jwt "github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/require"
)

func newTestAAA(ttl time.Duration) AAA {
	return AAA{
		users:    map[string]string{"admin": "password"},
		tokenTTL: ttl,
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func makeToken(t *testing.T, subject string, expiresAt time.Time, secret string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
		Subject:   subject,
		ExpiresAt: expiresAt.Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

func TestNew(t *testing.T) {
	t.Setenv("ADMIN_USER", "admin")
	t.Setenv("ADMIN_PASSWORD", "password")

	auth, err := New(time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))

	require.NoError(t, err)
	require.Equal(t, time.Minute, auth.tokenTTL)
	require.Equal(t, "password", auth.users["admin"])
}

func TestNewMissingEnv(t *testing.T) {
	testCases := []struct {
		name     string
		user     string
		password string
	}{
		{
			name:     "missing user",
			password: "password",
		},
		{
			name: "missing password",
			user: "admin",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.user != "" {
				t.Setenv("ADMIN_USER", tc.user)
			}
			if tc.password != "" {
				t.Setenv("ADMIN_PASSWORD", tc.password)
			}

			_, err := New(time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))

			require.Error(t, err)
		})
	}
}

func TestLogin(t *testing.T) {
	testCases := []struct {
		name        string
		user        string
		password    string
		expectedErr error
	}{
		{
			name:     "returns token",
			user:     "admin",
			password: "password",
		},
		{
			name:        "returns error for wrong password",
			user:        "admin",
			password:    "bad",
			expectedErr: core.ErrInvalidCredentials,
		},
		{
			name:        "returns error for unknown user",
			user:        "unknown",
			password:    "password",
			expectedErr: core.ErrInvalidCredentials,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			auth := newTestAAA(time.Minute)

			token, err := auth.Login(tc.user, tc.password)

			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				require.Empty(t, token)
				return
			}
			require.NoError(t, err)
			require.NotEmpty(t, token)
			require.NoError(t, auth.Verify(token))
		})
	}
}

func TestVerify(t *testing.T) {
	testCases := []struct {
		name        string
		token       func(t *testing.T) string
		wantErr     bool
		expectedErr error
	}{
		{
			name: "accepts valid token",
			token: func(t *testing.T) string {
				return makeToken(t, adminRole, time.Now().Add(time.Minute), secretKey)
			},
		},
		{
			name: "rejects invalid token",
			token: func(t *testing.T) string {
				return "bad-token"
			},
			wantErr: true,
		},
		{
			name: "rejects token with wrong role",
			token: func(t *testing.T) string {
				return makeToken(t, "user", time.Now().Add(time.Minute), secretKey)
			},
			expectedErr: core.ErrUnknownRole,
		},
		{
			name: "rejects expired token",
			token: func(t *testing.T) string {
				return makeToken(t, adminRole, time.Now().Add(-time.Minute), secretKey)
			},
			wantErr: true,
		},
		{
			name: "rejects token with wrong secret",
			token: func(t *testing.T) string {
				return makeToken(t, adminRole, time.Now().Add(time.Minute), "wrong secret")
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			auth := newTestAAA(time.Minute)

			err := auth.Verify(tc.token(t))

			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				return
			}
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
