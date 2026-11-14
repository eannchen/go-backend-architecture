package activity

import (
	"strings"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/usecase/activity"
)

// TestDecodeActivity protects the wire payload shape and transport-to-usecase
// mapping. Raw JSON examples can later be validated against the AsyncAPI schema.
func TestDecodeActivity(t *testing.T) {
	valid := `{"event_id":"evt-1","action":"report.downloaded","occurred_at":"2026-11-12T09:00:00Z"}`
	request, err := decodeActivity([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	want := activity.Request{EventID: "evt-1", Action: "report.downloaded", OccurredAt: time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)}
	if request != want {
		t.Fatal("decoded request differs from the wire contract")
	}
	for _, tc := range []struct{ name, payload string }{
		{"malformed", `{`},
		{"null", `null`},
		{"array", `[]`},
		{"missing fields", `{}`},
		{"wrong type", `{"event_id":1}`},
		{"unknown fields", `{"event_id":"evt-1","action":"report.downloaded","occurred_at":"2026-11-12T09:00:00Z","private":"secret-value"}`},
		{"multiple objects", `{"event_id":"evt-1","action":"report.downloaded","occurred_at":"2026-11-12T09:00:00Z"}{"event_id":"evt-1","action":"report.downloaded","occurred_at":"2026-11-12T09:00:00Z"}`},
		{"bad timestamp", `{"event_id":"evt-1","action":"download","occurred_at":"yesterday"}`},
		{"blank action", `{"event_id":"evt-1","action":" ","occurred_at":"2026-11-12T09:00:00Z"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeActivity([]byte(tc.payload))
			if err == nil {
				t.Fatal("invalid payload accepted")
			}
			if strings.Contains(err.Error(), "secret-value") {
				t.Fatal("payload leaked in diagnostics")
			}
		})
	}
}
