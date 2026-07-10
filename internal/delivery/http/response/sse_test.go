package response

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestStartSSEWritesEventAndCommentFrames(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	stream, err := NewResponder(nil).StartSSE(c)
	if err != nil {
		t.Fatalf("StartSSE() error = %v", err)
	}
	if err := stream.Event("update", map[string]string{"status": "ready"}); err != nil {
		t.Fatalf("Event() error = %v", err)
	}
	if err := stream.Comment("keep-alive"); err != nil {
		t.Fatalf("Comment() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(echo.HeaderContentType); got != "text/event-stream" {
		t.Fatalf("content type = %q, want text/event-stream", got)
	}
	if got := rec.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Fatalf("X-Accel-Buffering = %q, want no", got)
	}
	want := "event: update\ndata: {\"status\":\"ready\"}\n\n: keep-alive\n\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("frames = %q, want %q", got, want)
	}
}

func TestStartSSERejectsNonFlushingWriter(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	c := e.NewContext(req, &nonFlushingWriter{header: make(http.Header)})

	stream, err := NewResponder(nil).StartSSE(c)
	if stream != nil {
		t.Fatal("expected no stream")
	}
	if err != ErrStreamingUnsupported {
		t.Fatalf("error = %v, want %v", err, ErrStreamingUnsupported)
	}
}

type nonFlushingWriter struct {
	header http.Header
	body   bytes.Buffer
}

func (w *nonFlushingWriter) Header() http.Header {
	return w.header
}

func (w *nonFlushingWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

func (w *nonFlushingWriter) WriteHeader(int) {}
