package main

import (
	"fmt"
	"log"
	"net/http"

	"upfor/config"
	"upfor/db"
	"upfor/handlers"
	"upfor/router"
	"upfor/sms"
)

func main() {
	cfg := config.Load()

	database := db.Open(cfg.DBPath)
	defer database.Close()
	db.Migrate(database)

	var smsSender sms.Sender
	if cfg.SMSProvider == "twilio" {
		smsSender = sms.TwilioSender{
			AccountSID: cfg.TwilioAccountSID,
			AuthToken:  cfg.TwilioAuthToken,
			From:       cfg.TwilioFrom,
		}
	} else {
		smsSender = sms.ConsoleSender{}
	}

	h := &handlers.Handler{
		DB:         database,
		SMS:        smsSender,
		CheckinTTL: cfg.CheckinTTL,
	}

	fmt.Printf("UpFor backend running on http://localhost:%s\n", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, router.Setup(h, database)))
}
