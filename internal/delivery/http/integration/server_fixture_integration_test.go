//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	httpdelivery "github.com/eannchen/go-backend-architecture/internal/delivery/http"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/binding"
	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// serverFixture keeps transport construction identical across feature tests.
// Feature fixtures still own their handler, usecase, and repository wiring.
type serverFixture struct {
	server http.Handler
}

func newServerFixture(
	t *testing.T,
	validatorRegistrars []httpdelivery.ValidationRegistrar,
	routeRegistrars ...httpdelivery.RouteRegistrar,
) *serverFixture {
	t.Helper()

	server, err := httpdelivery.NewServer(
		httpdelivery.ServerConfig{},
		logger.NoopLogger{},
		binding.NewNormalizeBinder(nil),
		validatorRegistrars,
		nil,
		nil,
		routeRegistrars...,
	)
	if err != nil {
		t.Fatalf("build HTTP integration server: %v", err)
	}
	return &serverFixture{server: server}
}

func (f *serverFixture) sendJSON(
	t *testing.T,
	method string,
	path string,
	body any,
	cookie *http.Cookie,
	wantStatus int,
) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody []byte
	if body != nil {
		var err error
		requestBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("encode %s %s request: %v", method, path, err)
		}
	}

	request := httptest.NewRequest(method, path, bytes.NewReader(requestBody))
	if body != nil {
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	f.server.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, response.Code, wantStatus, response.Body.String())
	}
	return response
}
