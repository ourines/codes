package httpserver

// FeishuEvent represents a Feishu webhook event (schema 2.0).
type FeishuEvent struct {
	Schema  string            `json:"schema"`
	Header  FeishuHeader      `json:"header"`
	Event   FeishuEventDetail `json:"event"`
	// For URL verification
	Challenge string `json:"challenge,omitempty"`
	Type      string `json:"type,omitempty"`
	Token     string `json:"token,omitempty"`
}

type FeishuHeader struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	AppID     string `json:"app_id"`
}

type FeishuEventDetail struct {
	Message FeishuMessage `json:"message"`
	Sender  FeishuSender  `json:"sender"`
}

type FeishuMessage struct {
	MessageID   string `json:"message_id"`
	ChatID      string `json:"chat_id"`
	ChatType    string `json:"chat_type"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"` // JSON string: {"text":"..."}
}

type FeishuSender struct {
	SenderID struct {
		OpenID string `json:"open_id"`
	} `json:"sender_id"`
}

type FeishuTextContent struct {
	Text string `json:"text"`
}

type FeishuChallengeResponse struct {
	Challenge string `json:"challenge"`
}
