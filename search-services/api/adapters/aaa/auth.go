package aaa

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	jwt "github.com/golang-jwt/jwt"
	"github.com/Karambollla/course/api/core"
)

const secretKey = "something secret here" // token sign key
const adminRole = "superuser"             // token subject

// Authentication, Authorization, Accounting
type AAA struct {
	users    map[string]string
	tokenTTL time.Duration
	log      *slog.Logger
}

func New(tokenTTL time.Duration, log *slog.Logger) (AAA, error) {
	const adminUser = "ADMIN_USER"
	const adminPass = "ADMIN_PASSWORD"
	user, ok := os.LookupEnv(adminUser)
	if !ok {
		return AAA{}, fmt.Errorf("could not get admin user from enviroment")
	}
	password, ok := os.LookupEnv(adminPass)
	if !ok {
		return AAA{}, fmt.Errorf("could not get admin password from enviroment")
	}

	return AAA{
		users:    map[string]string{user: password},
		tokenTTL: tokenTTL,
		log:      log,
	}, nil
}

func (a AAA) Login(name, password string) (string, error) {
	if pass, ok := a.users[name]; !ok || pass != password {
		return "", core.ErrInvalidCredentials
	}
	claims := jwt.StandardClaims{
		Subject:   adminRole,
		ExpiresAt: time.Now().Add(a.tokenTTL).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func (a AAA) Verify(tokenString string) error {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, core.ErrInvalidToken
		}

		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			a.log.Error("unsupported HMAC algorithm", "alg", token.Method.Alg())
			return nil, core.ErrInvalidToken
		}

		return []byte(secretKey), nil
	})

	if err != nil {
		a.log.Error("failed to parse token", "error", err)
		return err
	}

	claims, ok := token.Claims.(*jwt.StandardClaims)
	if !ok || !token.Valid {
		a.log.Error("invalid token claims")
		return core.ErrInvalidToken
	}

	if claims.Subject != adminRole {
		a.log.Error("unknown role in token", "role", claims.Subject)
		return core.ErrUnknownRole
	}

	if claims.ExpiresAt < time.Now().Unix() {
		a.log.Error("token expired at", "expiresAt", claims.ExpiresAt)
		return core.ErrTokenExpired
	}

	return nil
}
