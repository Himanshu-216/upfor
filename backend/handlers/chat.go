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

// Chat routes /api/chat/* by sub-path.
func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	sub := strings.TrimPrefix(r.URL.Path, "/api/chat/")
	switch {
	case r.Method == http.MethodPost && sub == "send":
		h.sendMessage(w, r)
	case r.Method == http.MethodGet && sub == "messages":
		h.getMessages(w, r)
	default:
		writeErr(w, "not found", http.StatusNotFound)
	}
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	var body models.SendMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, "invalid json", http.StatusBadRequest)
		return
	}
	body.Body = strings.TrimSpace(body.Body)
	if body.RequestID == "" || body.SenderID == "" || body.Body == "" {
		writeErr(w, "request_id, sender_id, and body are required", http.StatusBadRequest)
		return
	}

	// Only allow messaging on an accepted connection
	var status string
	if err := h.DB.QueryRow(`SELECT status FROM requests WHERE id = ?`, body.RequestID).Scan(&status); err != nil || status != "accepted" {
		writeErr(w, "no accepted connection found", http.StatusForbidden)
		return
	}

	id := uuid.New().String()
	_, err := h.DB.Exec(`
		INSERT INTO messages (id, request_id, sender_id, sender_name, body, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, body.RequestID, body.SenderID, body.SenderName, body.Body, time.Now().UTC())
	if err != nil {
		log.Println("send message:", err)
		writeErr(w, "db error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"id": id})
}

func (h *Handler) getMessages(w http.ResponseWriter, r *http.Request) {
	reqID := r.URL.Query().Get("request_id")
	if reqID == "" {
		writeErr(w, "request_id required", http.StatusBadRequest)
		return
	}

	rows, err := h.DB.Query(`
		SELECT id, request_id, sender_id, sender_name, body, created_at
		FROM messages
		WHERE request_id = ?
		ORDER BY created_at ASC
	`, reqID)
	if err != nil {
		writeErr(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []models.MessageRecord{}
	for rows.Next() {
		var m models.MessageRecord
		var createdAt time.Time
		if err := rows.Scan(&m.ID, &m.RequestID, &m.SenderID, &m.SenderName, &m.Body, &createdAt); err != nil {
			continue
		}
		m.CreatedAt = createdAt.Format("15:04")
		results = append(results, m)
	}

	writeJSON(w, results)
}
