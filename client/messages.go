package client

type SpaceNotificationRequest struct {
	KindID  string `json:"kind_id"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
	ReplyTo string `json:"reply_to"`
}
