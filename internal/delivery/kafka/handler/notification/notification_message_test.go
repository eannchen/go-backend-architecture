package notification

import (
	"strings"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
)

// TestDecodeNotification protects the wire payload shape and transport-to-usecase
// mapping. Raw JSON examples can later be validated against the AsyncAPI schema.
func TestDecodeNotification(t *testing.T) {
	valid := `{"request_id":"req-1","recipient":"user-1","content":"Ready"}`
	request, err := decodeNotification([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	want := notification.Request{RequestID: "req-1", Recipient: "user-1", Content: "Ready"}
	if request != want {
		t.Fatal("decoded request differs from the wire contract")
	}
	for _, tc := range []struct{ name, payload string }{
		{"malformed", `{`},
		{"null", `null`},
		{"array", `[]`},
		{"missing fields", `{}`},
		{"wrong type", `{"request_id":1}`},
		{"unknown fields", `{"request_id":"req-1","recipient":"user-1","content":"Ready","private":"secret-value"}`},
		{"multiple objects", `{"request_id":"req-1","recipient":"user-1","content":"Ready"}{"request_id":"req-1","recipient":"user-1","content":"Ready"}`},
		{"blank recipient", `{"request_id":"req-1","recipient":" ","content":"Ready"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeNotification([]byte(tc.payload))
			if err == nil {
				t.Fatal("invalid payload accepted")
			}
			if strings.Contains(err.Error(), "secret-value") {
				t.Fatal("payload leaked in diagnostics")
			}
		})
	}
}
