package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"upfor/models"
)

// SendOTP handles POST /api/auth/send-otp
func (h *Handler) SendOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.SendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if req.Phone == "" {
		writeErr(w, "phone required", http.StatusBadRequest)
		return
	}

	// Rate-limit: one OTP per 60 seconds
	var lastSent time.Time
	err := h.DB.QueryRow(`SELECT created_at FROM otps WHERE phone = ?`, req.Phone).Scan(&lastSent)
	if err == nil && time.Since(lastSent) < 60*time.Second {
		remaining := int(60 - time.Since(lastSent).Seconds())
		writeErr(w, fmt.Sprintf("wait %d seconds before requesting a new OTP", remaining), http.StatusTooManyRequests)
		return
	}

	code := generateOTP()
	expiresAt := time.Now().UTC().Add(10 * time.Minute)

	_, err = h.DB.Exec(`
		INSERT INTO otps (phone, code, expires_at, attempts, created_at)
		VALUES (?, ?, ?, 0, ?)
		ON CONFLICT(phone) DO UPDATE SET
			code       = excluded.code,
			expires_at = excluded.expires_at,
			attempts   = 0,
			created_at = excluded.created_at
	`, req.Phone, code, expiresAt, time.Now().UTC())
	if err != nil {
		log.Println("store otp:", err)
		writeErr(w, "db error", http.StatusInternalServerError)
		return
	}

	msg := fmt.Sprintf("Your UpFor OTP is: %s. Valid for 10 minutes.", code)
	if err := h.SMS.Send(req.Phone, msg); err != nil {
		log.Println("sms send:", err)
		writeErr(w, "failed to send OTP", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"message": "OTP sent"})
}

// VerifyOTP handles POST /api/auth/verify-otp
func (h *Handler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.VerifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	req.OTP = strings.TrimSpace(req.OTP)
	if req.Phone == "" || req.OTP == "" {
		writeErr(w, "phone and otp required", http.StatusBadRequest)
		return
	}

	var storedCode string
	var expiresAt time.Time
	var attempts int
	err := h.DB.QueryRow(
		`SELECT code, expires_at, attempts FROM otps WHERE phone = ?`, req.Phone,
	).Scan(&storedCode, &expiresAt, &attempts)
	if err != nil {
		writeErr(w, "no OTP found — request one first", http.StatusBadRequest)
		return
	}

	if time.Now().UTC().After(expiresAt) {
		h.DB.Exec(`DELETE FROM otps WHERE phone = ?`, req.Phone)
		writeErr(w, "OTP expired — request a new one", http.StatusBadRequest)
		return
	}

	if attempts >= 5 {
		h.DB.Exec(`DELETE FROM otps WHERE phone = ?`, req.Phone)
		writeErr(w, "too many failed attempts — request a new OTP", http.StatusBadRequest)
		return
	}

	if req.OTP != storedCode {
		h.DB.Exec(`UPDATE otps SET attempts = attempts + 1 WHERE phone = ?`, req.Phone)
		writeErr(w, "incorrect OTP", http.StatusUnauthorized)
		return
	}

	// Valid — clean up the OTP
	h.DB.Exec(`DELETE FROM otps WHERE phone = ?`, req.Phone)

	// Get or create user
	var userID string
	err = h.DB.QueryRow(`SELECT id FROM users WHERE phone = ?`, req.Phone).Scan(&userID)
	if err != nil {
		userID = uuid.New().String()
		if _, err = h.DB.Exec(
			`INSERT INTO users (id, phone, created_at) VALUES (?, ?, ?)`,
			userID, req.Phone, time.Now().UTC(),
		); err != nil {
			log.Println("create user:", err)
			writeErr(w, "db error", http.StatusInternalServerError)
			return
		}
	}

	// Issue session token (7-day expiry)
	token := generateToken()
	_, err = h.DB.Exec(`
		INSERT INTO sessions (token, user_id, phone, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, token, userID, req.Phone, time.Now().UTC(), time.Now().UTC().Add(7*24*time.Hour))
	if err != nil {
		log.Println("create session:", err)
		writeErr(w, "db error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, models.AuthResponse{Token: token, UserID: userID})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func generateOTP() string {
	return "111111"
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
