package handlers

import (
	"database/sql"
	"time"

	"upfor/sms"
)

// Handler holds shared dependencies injected at startup.
type Handler struct {
	DB         *sql.DB
	SMS        sms.Sender
	CheckinTTL time.Duration
}
