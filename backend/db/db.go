package db

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

func Open(path string) *sql.DB {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal("open db:", err)
	}
	return d
}

func Migrate(d *sql.DB) {
	must(d, `
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
	// Non-fatal migration: adds address column to existing DBs
	d.Exec(`ALTER TABLE checkins ADD COLUMN address TEXT NOT NULL DEFAULT ''`)

	must(d, `
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

	must(d, `
		CREATE TABLE IF NOT EXISTS messages (
			id          TEXT PRIMARY KEY,
			request_id  TEXT NOT NULL,
			sender_id   TEXT NOT NULL,
			sender_name TEXT NOT NULL,
			body        TEXT NOT NULL,
			created_at  DATETIME NOT NULL
		)
	`)

	must(d, `
		CREATE TABLE IF NOT EXISTS users (
			id         TEXT PRIMARY KEY,
			phone      TEXT UNIQUE NOT NULL,
			created_at DATETIME NOT NULL
		)
	`)

	must(d, `
		CREATE TABLE IF NOT EXISTS otps (
			phone      TEXT PRIMARY KEY,
			code       TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			attempts   INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL
		)
	`)

	must(d, `
		CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL,
			phone      TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			expires_at DATETIME NOT NULL
		)
	`)
}

func PurgeStale(d *sql.DB, ttl time.Duration) {
	cutoff := time.Now().UTC().Add(-ttl)
	d.Exec(`DELETE FROM checkins WHERE updated_at < ?`, cutoff)
	d.Exec(`
		DELETE FROM requests WHERE
			from_id NOT IN (SELECT id FROM checkins) OR
			to_id   NOT IN (SELECT id FROM checkins)
	`)
	d.Exec(`DELETE FROM messages WHERE request_id NOT IN (SELECT id FROM requests)`)
}

func must(d *sql.DB, query string) {
	if _, err := d.Exec(query); err != nil {
		log.Fatal("migrate:", err)
	}
}
