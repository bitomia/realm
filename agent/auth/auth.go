package auth

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type contextKey string

const usernameContextKey contextKey = "username"

var (
	secretKey string
	enabled   bool = false
)

func Initialize() {
	secretKey = os.Getenv("JWT_SECRET")
	if secretKey == "" {
		slog.Warn("Running without authentication")
		enabled = false
	} else {
		slog.Info("Authentication initialized")
		enabled = true
	}
}

func WithAuth(handler http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			handler.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		bearer := strings.TrimSpace(strings.Replace(authHeader, "Bearer", "", 1))
		claims, err := ValidateBearer(bearer, secretKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), usernameContextKey, claims.Username)
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}
