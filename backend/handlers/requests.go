package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"upfor/models"
)

// Requests routes /api/requests/* by sub-path.
func (h *Handler) Requests(w http.ResponseWriter, r *http.Request) {
	sub := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	switch {
	case r.Method == http.MethodPost && sub == "send":
		h.sendRequest(w, r)
	case r.Method == http.MethodGet && sub == "incoming":
		h.incomingRequests(w, r)
	case r.Method == http.MethodGet && sub == "sent":
		h.sentRequests(w, r)
	case r.Method == http.MethodPost && sub == "respond":
		h.respondRequest(w, r)
	default:
		writeErr(w, "not found", http.StatusNotFound)
	}
}

func (h *Handler) sendRequest(w http.ResponseWriter, r *http.Request) {
	var body models.SendRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.FromID == "" || body.ToID == "" {
		writeErr(w, "from_id and to_id required", http.StatusBadRequest)
		return
	}
	if body.FromID == body.ToID {
		writeErr(w, "cannot request yourself", http.StatusBadRequest)
		return
	}

	// Return existing request if one already exists between these two users
	var existingID, existingStatus string
	err := h.DB.QueryRow(`
		SELECT id, status FROM requests
		WHERE (from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)
		ORDER BY created_at DESC LIMIT 1
	`, body.FromID, body.ToID, body.ToID, body.FromID).Scan(&existingID, &existingStatus)
	if err == nil {
		writeJSON(w, map[string]string{"id": existingID, "status": existingStatus})
		return
	}

	id := uuid.New().String()
	_, err = h.DB.Exec(`
		INSERT INTO requests (id, from_id, from_name, from_activity, to_id, status, created_at)
		VALUES (?, ?, ?, ?, ?, 'pending', ?)
	`, id, body.FromID, body.FromName, body.FromActivity, body.ToID, time.Now().UTC())
	if err != nil {
		log.Println("send request:", err)
		writeErr(w, "db error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"id": id, "status": "pending"})
}

func (h *Handler) incomingRequests(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeErr(w, "user_id required", http.StatusBadRequest)
		return
	}

	rows, err := h.DB.Query(`
		SELECT id, from_id, from_name, from_activity, to_id, status, created_at
		FROM requests
		WHERE to_id = ? AND status = 'pending'
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		writeErr(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []models.RequestRecord{}
	for rows.Next() {
		var rec models.RequestRecord
		var createdAt time.Time
		if err := rows.Scan(&rec.ID, &rec.FromID, &rec.FromName, &rec.FromActivity,
			&rec.ToID, &rec.Status, &createdAt); err != nil {
			continue
		}
		rec.CreatedAt = createdAt.Format("15:04")
		results = append(results, rec)
	}

	writeJSON(w, results)
}

func (h *Handler) sentRequests(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeErr(w, "user_id required", http.StatusBadRequest)
		return
	}

	rows, err := h.DB.Query(`
		SELECT r.id, r.from_id, r.from_name, r.from_activity, r.to_id,
		       COALESCE(c.name, ''), r.status, r.created_at
		FROM requests r
		LEFT JOIN checkins c ON c.id = r.to_id
		WHERE r.from_id = ?
		ORDER BY r.created_at DESC
	`, userID)
	if err != nil {
		writeErr(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []models.RequestRecord{}
	for rows.Next() {
		var rec models.RequestRecord
		var createdAt time.Time
		if err := rows.Scan(&rec.ID, &rec.FromID, &rec.FromName, &rec.FromActivity,
			&rec.ToID, &rec.ToName, &rec.Status, &createdAt); err != nil {
			continue
		}
		rec.CreatedAt = createdAt.Format("15:04")
		results = append(results, rec)
	}

	writeJSON(w, results)
}

func (h *Handler) respondRequest(w http.ResponseWriter, r *http.Request) {
	var body models.RespondRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.RequestID == "" {
		writeErr(w, "request_id required", http.StatusBadRequest)
		return
	}
	if body.Status != "accepted" && body.Status != "declined" {
		writeErr(w, "status must be accepted or declined", http.StatusBadRequest)
		return
	}

	res, err := h.DB.Exec(`
		UPDATE requests SET status = ? WHERE id = ? AND status = 'pending'
	`, body.Status, body.RequestID)
	if err != nil {
		writeErr(w, "db error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, map[string]any{"ok": n > 0, "status": body.Status})
}
