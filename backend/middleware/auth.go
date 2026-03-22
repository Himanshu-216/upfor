package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"
)

type contextKey string

const userIDKey contextKey = "userID"

// Auth validates the Bearer token and injects user_id into the request context.
// Returns 401 if the token is missing, invalid, or expired.
func Auth(db *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			return
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		var userID string
		var expiresAt time.Time
		err := db.QueryRow(
			`SELECT user_id, expires_at FROM sessions WHERE token = ?`, token,
		).Scan(&userID, &expiresAt)
		if err != nil || time.Now().UTC().After(expiresAt) {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

// UserID extracts the authenticated user's ID from the request context.
func UserID(r *http.Request) string {
	v, _ := r.Context().Value(userIDKey).(string)
	return v
}
