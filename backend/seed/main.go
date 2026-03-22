package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "upfor.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS checkins (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, activity TEXT NOT NULL,
		address TEXT NOT NULL DEFAULT '',
		lat REAL NOT NULL, lng REAL NOT NULL, updated_at DATETIME NOT NULL
	)`)
	if err == nil {
		db.Exec(`ALTER TABLE checkins ADD COLUMN address TEXT NOT NULL DEFAULT ''`)
	}
	if err != nil {
		log.Fatal(err)
	}

	now := time.Now().UTC()

	users := []struct {
		id, name, activity, address string
		lat, lng                    float64
	}{
		// ~1 km north of Magarpatta (near Hadapsar)
		{"test-user-001", "Rahul", "pickleball", "Hadapsar, Pune", 18.5160, 73.9270},
		// ~1.5 km west of Magarpatta (near Wanowrie)
		{"test-user-002", "Priya", "walking", "Wanowrie, Pune", 18.5080, 73.9150},
	}

	for _, u := range users {
		_, err := db.Exec(`
			INSERT INTO checkins (id, name, activity, address, lat, lng, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				name=excluded.name, activity=excluded.activity, address=excluded.address,
				lat=excluded.lat, lng=excluded.lng, updated_at=excluded.updated_at
		`, u.id, u.name, u.activity, u.address, u.lat, u.lng, now)
		if err != nil {
			log.Printf("insert %s: %v", u.name, err)
		} else {
			fmt.Printf("inserted: %-10s | %-12s | %.4f, %.4f\n", u.name, u.activity, u.lat, u.lng)
		}
	}

	rows, _ := db.Query(`SELECT name, activity, lat, lng FROM checkins`)
	defer rows.Close()
	fmt.Println("\n--- current checkins ---")
	for rows.Next() {
		var name, activity string
		var lat, lng float64
		rows.Scan(&name, &activity, &lat, &lng)
		fmt.Printf("  %-10s | %-12s | %.4f, %.4f\n", name, activity, lat, lng)
	}
}
