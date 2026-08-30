package httpcontext

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

func TestSessionContextRoundTrip(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/me", nil), httptest.NewRecorder())
	want := SessionInfo{UserID: 42, Email: "user@example.com", Method: auth.MethodOTP}

	if _, ok := SessionFromContext(c); ok {
		t.Fatal("expected missing session")
	}
	SetSessionContext(c, want)
	got, ok := SessionFromContext(c)
	if !ok || got != want {
		t.Fatalf("session = %+v, %v; want %+v, true", got, ok, want)
	}
}
