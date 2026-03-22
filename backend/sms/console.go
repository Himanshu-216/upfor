package sms

import "fmt"

// ConsoleSender prints the OTP to stdout — use in development so you don't
// need real SMS credentials.
type ConsoleSender struct{}

func (ConsoleSender) Send(to, message string) error {
	fmt.Printf("\n📱 [SMS → %s] %s\n\n", to, message)
	return nil
}
