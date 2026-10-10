package kafka

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
)

// notificationMessage is the JSON contract for notifications.requested.v1.
// Kafka keys affect partition routing; the JSON recipient does not select a
// partition by itself. A publisher may choose a key or leave the message keyless.
// Example payload:
// {"request_id":"req-1","recipient":"user-1","content":"Your report is ready"}
type notificationMessage struct {
	RequestID string `json:"request_id"`
	Recipient string `json:"recipient"`
	Content   string `json:"content"`
}

func decodeNotification(value []byte) (notification.Request, error) {
	var message notificationMessage
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	// Use a fixed diagnostic: JSON decoder errors can include sensitive payload
	// fragments. The worker reports the topic/partition/offset instead of the data.
	if err := decoder.Decode(&message); err != nil {
		return notification.Request{}, errors.New("invalid notification JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return notification.Request{}, errors.New("notification must contain one JSON object")
	}
	if strings.TrimSpace(message.RequestID) == "" || strings.TrimSpace(message.Recipient) == "" || strings.TrimSpace(message.Content) == "" {
		return notification.Request{}, errors.New("notification requires request_id, recipient, and content")
	}
	return notification.Request{RequestID: message.RequestID, Recipient: message.Recipient, Content: message.Content}, nil
}
