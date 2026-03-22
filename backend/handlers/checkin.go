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

// Checkin handles POST /api/checkin
func (h *Handler) Checkin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.CheckinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Activity = strings.TrimSpace(req.Activity)
	if req.Name == "" || req.Activity == "" {
		writeErr(w, "name and activity are required", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	_, err := h.DB.Exec(`
		INSERT INTO checkins (id, name, activity, address, lat, lng, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name       = excluded.name,
			activity   = excluded.activity,
			address    = excluded.address,
			lat        = excluded.lat,
			lng        = excluded.lng,
			updated_at = excluded.updated_at
	`, req.ID, req.Name, req.Activity, req.Address, req.Lat, req.Lng, time.Now().UTC())
	if err != nil {
		log.Println("checkin insert:", err)
		writeErr(w, "db error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"id": req.ID})
}

// Checkout handles DELETE /api/checkout/:id
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/checkout/")
	if id == "" {
		writeErr(w, "id required", http.StatusBadRequest)
		return
	}

	h.DB.Exec(`DELETE FROM checkins WHERE id = ?`, id)
	h.DB.Exec(`DELETE FROM messages WHERE request_id IN (SELECT id FROM requests WHERE from_id = ? OR to_id = ?)`, id, id)
	h.DB.Exec(`DELETE FROM requests WHERE from_id = ? OR to_id = ?`, id, id)

	writeJSON(w, map[string]bool{"ok": true})
}
