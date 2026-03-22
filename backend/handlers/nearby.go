package handlers

import (
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"upfor/db"
	"upfor/geo"
	"upfor/models"
)

// Nearby handles GET /api/nearby
// Passing activity="" returns everyone in the radius regardless of activity.
func (h *Handler) Nearby(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	activity := strings.TrimSpace(q.Get("activity"))
	lat, _ := strconv.ParseFloat(q.Get("lat"), 64)
	lng, _ := strconv.ParseFloat(q.Get("lng"), 64)
	radius, _ := strconv.ParseFloat(q.Get("radius"), 64)
	excludeID := q.Get("exclude_id")

	if radius <= 0 {
		radius = 5
	}

	db.PurgeStale(h.DB, h.CheckinTTL)

	rows, err := h.DB.Query(`
		SELECT id, name, activity, address, lat, lng, updated_at
		FROM checkins
		WHERE id != ? AND LOWER(activity) LIKE LOWER(?)
	`, excludeID, "%"+activity+"%")
	if err != nil {
		log.Println("nearby query:", err)
		writeErr(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []models.UserResponse{}
	for rows.Next() {
		var (
			id, name, act, addr string
			ulat, ulng          float64
			updatedAt           time.Time
		)
		if err := rows.Scan(&id, &name, &act, &addr, &ulat, &ulng, &updatedAt); err != nil {
			continue
		}
		d := geo.Haversine(lat, lng, ulat, ulng)
		if d <= radius {
			results = append(results, models.UserResponse{
				ID:        id,
				Name:      name,
				Activity:  act,
				Address:   addr,
				Lat:       ulat,
				Lng:       ulng,
				Distance:  math.Round(d*100) / 100,
				UpdatedAt: updatedAt.Format("15:04"),
			})
		}
	}

	writeJSON(w, results)
}
