package router

import (
	"database/sql"
	"net/http"

	"upfor/handlers"
	"upfor/middleware"
)

func Setup(h *handlers.Handler, db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	// Public routes (no token required)
	mux.HandleFunc("/api/health", middleware.CORS(h.Health))
	mux.HandleFunc("/api/auth/send-otp", middleware.CORS(h.SendOTP))
	mux.HandleFunc("/api/auth/verify-otp", middleware.CORS(h.VerifyOTP))

	// Protected routes
	auth := func(fn http.HandlerFunc) http.HandlerFunc {
		return middleware.Auth(db, fn)
	}
	mux.HandleFunc("/api/checkin", auth(h.Checkin))
	mux.HandleFunc("/api/nearby", auth(h.Nearby))
	mux.HandleFunc("/api/checkout/", auth(h.Checkout))
	mux.HandleFunc("/api/requests/", auth(h.Requests))
	mux.HandleFunc("/api/chat/", auth(h.Chat))

	return mux
}
