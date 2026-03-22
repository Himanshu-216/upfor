package main

import (
	"fmt"
	"log"
	"time"

	"upfor/db"
)

func main() {
	database := db.Open("../upfor.db")
	defer database.Close()
	db.Migrate(database)

	now := time.Now().UTC()

	users := []struct {
		id, name, activity, address string
		lat, lng                    float64
	}{
		{"test-user-001", "Rahul", "pickleball", "Hadapsar, Pune", 18.5160, 73.9270},
		{"test-user-002", "Priya", "walking", "Wanowrie, Pune", 18.5080, 73.9150},
	}

	for _, u := range users {
		_, err := database.Exec(`
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

	rows, _ := database.Query(`SELECT name, activity, lat, lng FROM checkins`)
	defer rows.Close()
	fmt.Println("\n--- current checkins ---")
	for rows.Next() {
		var name, activity string
		var lat, lng float64
		rows.Scan(&name, &activity, &lat, &lng)
		fmt.Printf("  %-10s | %-12s | %.4f, %.4f\n", name, activity, lat, lng)
	}
}
