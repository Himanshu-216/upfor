package sms

// Sender is the interface every SMS provider must implement.
type Sender interface {
	Send(to, message string) error
}
