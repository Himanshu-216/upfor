package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var db *sql.DB

type CheckinRequest struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Activity string  `json:"activity"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
}

type UserResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Activity  string  `json:"activity"`
	Distance  float64 `json:"distance"`
	UpdatedAt string  `json:"updated_at"`
}

// haversine returns the great-circle distance in km between two lat/lng points.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "upfor.db")
	if err != nil {
		log.Fatal("open db:", err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS checkins (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			activity    TEXT NOT NULL,
			lat         REAL NOT NULL,
			lng         REAL NOT NULL,
			updated_at  DATETIME NOT NULL
		)
	`)
	if err != nil {
		log.Fatal("create table:", err)
	}
}

func cors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// POST /api/checkin  — register or refresh a user's active status
func checkinHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CheckinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Activity = strings.TrimSpace(req.Activity)
	if req.Name == "" || req.Activity == "" {
		http.Error(w, "name and activity are required", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	_, err := db.Exec(`
		INSERT INTO checkins (id, name, activity, lat, lng, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name       = excluded.name,
			activity   = excluded.activity,
			lat        = excluded.lat,
			lng        = excluded.lng,
			updated_at = excluded.updated_at
	`, req.ID, req.Name, req.Activity, req.Lat, req.Lng, time.Now().UTC())
	if err != nil {
		log.Println("checkin insert:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": req.ID})
}

// GET /api/nearby?activity=&lat=&lng=&radius=&exclude_id=
func nearbyHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

	// Purge entries not updated in the last 2 hours
	db.Exec(`DELETE FROM checkins WHERE updated_at < ?`, time.Now().UTC().Add(-2*time.Hour))

	rows, err := db.Query(`
		SELECT id, name, activity, lat, lng, updated_at
		FROM checkins
		WHERE id != ? AND LOWER(activity) LIKE LOWER(?)
	`, excludeID, "%"+activity+"%")
	if err != nil {
		log.Println("nearby query:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []UserResponse{}
	for rows.Next() {
		var (
			id, name, act string
			ulat, ulng    float64
			updatedAt     time.Time
		)
		if err := rows.Scan(&id, &name, &act, &ulat, &ulng, &updatedAt); err != nil {
			continue
		}
		d := haversine(lat, lng, ulat, ulng)
		if d <= radius {
			results = append(results, UserResponse{
				ID:        id,
				Name:      name,
				Activity:  act,
				Distance:  math.Round(d*100) / 100,
				UpdatedAt: updatedAt.Format("15:04"),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// DELETE /api/checkout/{id}  — user goes offline
func checkoutHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/checkout/")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	db.Exec(`DELETE FROM checkins WHERE id = ?`, id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func main() {
	initDB()
	defer db.Close()

	http.HandleFunc("/api/checkin", checkinHandler)
	http.HandleFunc("/api/nearby", nearbyHandler)
	http.HandleFunc("/api/checkout/", checkoutHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("UpFor backend running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
