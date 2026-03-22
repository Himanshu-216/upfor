package sms

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// TwilioSender sends real SMS via the Twilio REST API.
// Set SMS_PROVIDER=twilio and provide TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN,
// TWILIO_FROM in the environment.
type TwilioSender struct {
	AccountSID string
	AuthToken  string
	From       string
}

func (t TwilioSender) Send(to, message string) error {
	apiURL := fmt.Sprintf(
		"https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json",
		t.AccountSID,
	)

	body := url.Values{}
	body.Set("To", to)
	body.Set("From", t.From)
	body.Set("Body", message)

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(body.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(t.AccountSID, t.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("twilio responded with status %d", resp.StatusCode)
	}
	return nil
}
