package config

import (
	"os"
	"time"
)

type Config struct {
	Port       string
	DBPath     string
	CheckinTTL time.Duration

	// SMS — set SMS_PROVIDER=twilio to use real SMS, default prints to console
	SMSProvider     string // "console" | "twilio"
	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFrom       string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "upfor.db"
	}
	smsProvider := os.Getenv("SMS_PROVIDER")
	if smsProvider == "" {
		smsProvider = "console"
	}
	return Config{
		Port:            port,
		DBPath:          dbPath,
		CheckinTTL:      2 * time.Hour,
		SMSProvider:     smsProvider,
		TwilioAccountSID: os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:  os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioFrom:       os.Getenv("TWILIO_FROM"),
	}
}
