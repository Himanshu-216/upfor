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

// ── Structs ───────────────────────────────────────────────────────────────────

type CheckinRequest struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Activity string  `json:"activity"`
	Address  string  `json:"address"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
}

type UserResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Activity  string  `json:"activity"`
	Address   string  `json:"address"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Distance  float64 `json:"distance"`
	UpdatedAt string  `json:"updated_at"`
}

type SendRequestBody struct {
	FromID       string `json:"from_id"`
	FromName     string `json:"from_name"`
	FromActivity string `json:"from_activity"`
	ToID         string `json:"to_id"`
}

type RespondRequestBody struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"` // "accepted" | "declined"
}

type RequestRecord struct {
	ID           string `json:"id"`
	FromID       string `json:"from_id"`
	FromName     string `json:"from_name"`
	FromActivity string `json:"from_activity"`
	ToID         string `json:"to_id"`
	ToName       string `json:"to_name"` // resolved via JOIN with checkins
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

type MessageRecord struct {
	ID         string `json:"id"`
	RequestID  string `json:"request_id"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
}

type SendMessageBody struct {
	RequestID  string `json:"request_id"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Body       string `json:"body"`
}

// ── DB ────────────────────────────────────────────────────────────────────────

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
			address     TEXT NOT NULL DEFAULT '',
			lat         REAL NOT NULL,
			lng         REAL NOT NULL,
			updated_at  DATETIME NOT NULL
		)
	`)
	if err != nil {
		log.Fatal("create checkins table:", err)
	}
	// Migrate: add address column if upgrading from an older DB
	db.Exec(`ALTER TABLE checkins ADD COLUMN address TEXT NOT NULL DEFAULT ''`)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS requests (
			id            TEXT PRIMARY KEY,
			from_id       TEXT NOT NULL,
			from_name     TEXT NOT NULL,
			from_activity TEXT NOT NULL,
			to_id         TEXT NOT NULL,
			status        TEXT NOT NULL DEFAULT 'pending',
			created_at    DATETIME NOT NULL
		)
	`)
	if err != nil {
		log.Fatal("create requests table:", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id          TEXT PRIMARY KEY,
			request_id  TEXT NOT NULL,
			sender_id   TEXT NOT NULL,
			sender_name TEXT NOT NULL,
			body        TEXT NOT NULL,
			created_at  DATETIME NOT NULL
		)
	`)
	if err != nil {
		log.Fatal("create messages table:", err)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func cors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func purgeStale() {
	cutoff := time.Now().UTC().Add(-2 * time.Hour)
	db.Exec(`DELETE FROM checkins WHERE updated_at < ?`, cutoff)
	db.Exec(`
		DELETE FROM requests WHERE
			from_id NOT IN (SELECT id FROM checkins) OR
			to_id   NOT IN (SELECT id FROM checkins)
	`)
	db.Exec(`DELETE FROM messages WHERE request_id NOT IN (SELECT id FROM requests)`)
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// POST /api/checkin
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
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": req.ID})
}

// GET /api/nearby — activity="" returns everyone in radius
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

	purgeStale()

	rows, err := db.Query(`
		SELECT id, name, activity, address, lat, lng, updated_at
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
			id, name, act, addr string
			ulat, ulng          float64
			updatedAt           time.Time
		)
		if err := rows.Scan(&id, &name, &act, &addr, &ulat, &ulng, &updatedAt); err != nil {
			continue
		}
		d := haversine(lat, lng, ulat, ulng)
		if d <= radius {
			results = append(results, UserResponse{
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// DELETE /api/checkout/:id
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
	db.Exec(`DELETE FROM messages WHERE request_id IN (SELECT id FROM requests WHERE from_id = ? OR to_id = ?)`, id, id)
	db.Exec(`DELETE FROM requests WHERE from_id = ? OR to_id = ?`, id, id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// /api/requests/* — route by sub-path
func requestsHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == http.MethodOptions {
		return
	}

	sub := strings.TrimPrefix(r.URL.Path, "/api/requests/")

	switch {
	case r.Method == http.MethodPost && sub == "send":
		sendRequestHandler(w, r)
	case r.Method == http.MethodGet && sub == "incoming":
		incomingRequestsHandler(w, r)
	case r.Method == http.MethodGet && sub == "sent":
		sentRequestsHandler(w, r)
	case r.Method == http.MethodPost && sub == "respond":
		respondRequestHandler(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// POST /api/requests/send
func sendRequestHandler(w http.ResponseWriter, r *http.Request) {
	var body SendRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.FromID == "" || body.ToID == "" {
		http.Error(w, "from_id and to_id required", http.StatusBadRequest)
		return
	}
	if body.FromID == body.ToID {
		http.Error(w, "cannot request yourself", http.StatusBadRequest)
		return
	}

	var existingID, existingStatus string
	err := db.QueryRow(`
		SELECT id, status FROM requests
		WHERE (from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)
		ORDER BY created_at DESC LIMIT 1
	`, body.FromID, body.ToID, body.ToID, body.FromID).Scan(&existingID, &existingStatus)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": existingID, "status": existingStatus})
		return
	}

	id := uuid.New().String()
	_, err = db.Exec(`
		INSERT INTO requests (id, from_id, from_name, from_activity, to_id, status, created_at)
		VALUES (?, ?, ?, ?, ?, 'pending', ?)
	`, id, body.FromID, body.FromName, body.FromActivity, body.ToID, time.Now().UTC())
	if err != nil {
		log.Println("send request:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "pending"})
}

// GET /api/requests/incoming?user_id=
func incomingRequestsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(`
		SELECT id, from_id, from_name, from_activity, to_id, status, created_at
		FROM requests
		WHERE to_id = ? AND status = 'pending'
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []RequestRecord{}
	for rows.Next() {
		var rec RequestRecord
		var createdAt time.Time
		if err := rows.Scan(&rec.ID, &rec.FromID, &rec.FromName, &rec.FromActivity, &rec.ToID, &rec.Status, &createdAt); err != nil {
			continue
		}
		rec.CreatedAt = createdAt.Format("15:04")
		results = append(results, rec)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// GET /api/requests/sent?user_id= — JOINs checkins to resolve recipient name
func sentRequestsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(`
		SELECT r.id, r.from_id, r.from_name, r.from_activity, r.to_id,
		       COALESCE(c.name, ''), r.status, r.created_at
		FROM requests r
		LEFT JOIN checkins c ON c.id = r.to_id
		WHERE r.from_id = ?
		ORDER BY r.created_at DESC
	`, userID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []RequestRecord{}
	for rows.Next() {
		var rec RequestRecord
		var createdAt time.Time
		if err := rows.Scan(&rec.ID, &rec.FromID, &rec.FromName, &rec.FromActivity,
			&rec.ToID, &rec.ToName, &rec.Status, &createdAt); err != nil {
			continue
		}
		rec.CreatedAt = createdAt.Format("15:04")
		results = append(results, rec)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// POST /api/requests/respond
func respondRequestHandler(w http.ResponseWriter, r *http.Request) {
	var body RespondRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.RequestID == "" {
		http.Error(w, "request_id required", http.StatusBadRequest)
		return
	}
	if body.Status != "accepted" && body.Status != "declined" {
		http.Error(w, "status must be accepted or declined", http.StatusBadRequest)
		return
	}

	res, err := db.Exec(`
		UPDATE requests SET status = ? WHERE id = ? AND status = 'pending'
	`, body.Status, body.RequestID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": n > 0, "status": body.Status})
}

// /api/chat/* — route by sub-path
func chatHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == http.MethodOptions {
		return
	}

	sub := strings.TrimPrefix(r.URL.Path, "/api/chat/")

	switch {
	case r.Method == http.MethodPost && sub == "send":
		sendMessageHandler(w, r)
	case r.Method == http.MethodGet && sub == "messages":
		getMessagesHandler(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// POST /api/chat/send
func sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	var body SendMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body.Body = strings.TrimSpace(body.Body)
	if body.RequestID == "" || body.SenderID == "" || body.Body == "" {
		http.Error(w, "request_id, sender_id, and body are required", http.StatusBadRequest)
		return
	}

	// Only allow messaging in an accepted connection
	var status string
	if err := db.QueryRow(`SELECT status FROM requests WHERE id = ?`, body.RequestID).Scan(&status); err != nil || status != "accepted" {
		http.Error(w, "no accepted connection found", http.StatusForbidden)
		return
	}

	id := uuid.New().String()
	_, err := db.Exec(`
		INSERT INTO messages (id, request_id, sender_id, sender_name, body, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, body.RequestID, body.SenderID, body.SenderName, body.Body, time.Now().UTC())
	if err != nil {
		log.Println("send message:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// GET /api/chat/messages?request_id=X
func getMessagesHandler(w http.ResponseWriter, r *http.Request) {
	reqID := r.URL.Query().Get("request_id")
	if reqID == "" {
		http.Error(w, "request_id required", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(`
		SELECT id, request_id, sender_id, sender_name, body, created_at
		FROM messages
		WHERE request_id = ?
		ORDER BY created_at ASC
	`, reqID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []MessageRecord{}
	for rows.Next() {
		var m MessageRecord
		var createdAt time.Time
		if err := rows.Scan(&m.ID, &m.RequestID, &m.SenderID, &m.SenderName, &m.Body, &createdAt); err != nil {
			continue
		}
		m.CreatedAt = createdAt.Format("15:04")
		results = append(results, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	initDB()
	defer db.Close()

	http.HandleFunc("/api/checkin", checkinHandler)
	http.HandleFunc("/api/nearby", nearbyHandler)
	http.HandleFunc("/api/checkout/", checkoutHandler)
	http.HandleFunc("/api/requests/", requestsHandler)
	http.HandleFunc("/api/chat/", chatHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("UpFor backend running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
