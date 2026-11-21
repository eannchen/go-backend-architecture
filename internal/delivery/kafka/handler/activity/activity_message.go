package activity

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/usecase/activity"
)

// activityMessage is the JSON contract for activities.recorded.v1.
// Example payload:
// {"event_id":"evt-1","action":"report.downloaded","occurred_at":"2026-11-12T09:00:00Z"}
type activityMessage struct {
	EventID    string    `json:"event_id"`
	Action     string    `json:"action"`
	OccurredAt time.Time `json:"occurred_at"`
}

func decodeActivity(value []byte) (activity.Request, error) {
	var message activityMessage
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	// Fixed diagnostics keep payload fragments out of worker logs.
	if err := decoder.Decode(&message); err != nil {
		return activity.Request{}, errors.New("invalid activity JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return activity.Request{}, errors.New("activity must contain one JSON object")
	}
	if strings.TrimSpace(message.EventID) == "" || strings.TrimSpace(message.Action) == "" || message.OccurredAt.IsZero() {
		return activity.Request{}, errors.New("activity requires event_id, action, and occurred_at")
	}
	return activity.Request{EventID: message.EventID, Action: message.Action, OccurredAt: message.OccurredAt}, nil
}
