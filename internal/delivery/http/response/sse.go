package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
)

// ErrStreamingUnsupported indicates that the response writer cannot flush SSE frames.
var ErrStreamingUnsupported = apperr.New(apperr.CodeInternal, "streaming is not supported")

// SSEStream writes Server-Sent Events. A single goroutine must own each stream.
type SSEStream struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// StartSSE opens an SSE response and returns its event writer.
func (r *responder) StartSSE(c *echo.Context) (*SSEStream, error) {
	w := c.Response()
	if !supportsFlushing(w) {
		return nil, ErrStreamingUnsupported
	}
	flusher := w.(http.Flusher)

	// Streaming must outlive the normal server WriteTimeout; client cancellation
	// remains controlled by the request context.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})

	h := w.Header()
	h.Set(echo.HeaderContentType, "text/event-stream")
	h.Set(echo.HeaderCacheControl, "no-cache")
	h.Set(echo.HeaderConnection, "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	return &SSEStream{w: w, flusher: flusher}, nil
}

func supportsFlushing(w http.ResponseWriter) bool {
	for {
		unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			_, ok := w.(http.Flusher)
			return ok
		}

		next := unwrapper.Unwrap()
		if next == w {
			return false
		}
		w = next
	}
}

// Event writes and flushes one named SSE event.
func (s *SSEStream) Event(event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sse payload: %w", err)
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// Comment writes and flushes one SSE keep-alive comment.
func (s *SSEStream) Comment(text string) error {
	if _, err := fmt.Fprintf(s.w, ": %s\n\n", text); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
