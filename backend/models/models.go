package models

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
	ToName       string `json:"to_name"`
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

// ── Auth ──────────────────────────────────────────────────────────────────────

type SendOTPRequest struct {
	Phone string `json:"phone"`
}

type VerifyOTPRequest struct {
	Phone string `json:"phone"`
	OTP   string `json:"otp"`
}

type AuthResponse struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}
