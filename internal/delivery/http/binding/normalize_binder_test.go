package binding

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestNormalizeBinder_TrimsStringFields(t *testing.T) {
	e := echo.New()
	e.Binder = NewNormalizeBinder(e.Binder)

	body := []byte(`{"email":"  foo@bar.com  ","code":"  123456  "}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var dest struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.Bind(&dest); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if dest.Email != "foo@bar.com" {
		t.Errorf("email: got %q", dest.Email)
	}
	if dest.Code != "123456" {
		t.Errorf("code: got %q", dest.Code)
	}
}

func TestNormalizeBinder_RespectsTrimFalseTag(t *testing.T) {
	e := echo.New()
	e.Binder = NewNormalizeBinder(e.Binder)

	body := []byte(`{"trimmed":"  a  ","raw":"  b  "}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var dest struct {
		Trimmed string `json:"trimmed"`
		Raw     string `json:"raw" trim:"false"`
	}
	if err := c.Bind(&dest); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if dest.Trimmed != "a" {
		t.Errorf("trimmed: got %q", dest.Trimmed)
	}
	if dest.Raw != "  b  " {
		t.Errorf("raw (no trim): got %q", dest.Raw)
	}
}

func TestNormalizeBinder_TrimsNestedStruct(t *testing.T) {
	e := echo.New()
	e.Binder = NewNormalizeBinder(e.Binder)

	body := []byte(`{"inner":{"name":"  nested  "}}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var dest struct {
		Inner struct {
			Name string `json:"name"`
		} `json:"inner"`
	}
	if err := c.Bind(&dest); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if dest.Inner.Name != "nested" {
		t.Errorf("inner.name: got %q", dest.Inner.Name)
	}
}

func TestNormalizeBinder_TrimsPointerToStruct(t *testing.T) {
	e := echo.New()
	e.Binder = NewNormalizeBinder(e.Binder)

	body := []byte(`{"ptr":{"value":"  x  "}}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var dest struct {
		Ptr *struct {
			Value string `json:"value"`
		} `json:"ptr"`
	}
	if err := c.Bind(&dest); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if dest.Ptr == nil {
		t.Fatal("ptr is nil")
	}
	if dest.Ptr.Value != "x" {
		t.Errorf("ptr.value: got %q", dest.Ptr.Value)
	}
}

func TestNormalizeBinder_TrimsStringSlice(t *testing.T) {
	e := echo.New()
	e.Binder = NewNormalizeBinder(e.Binder)

	body := []byte(`{"items":["  a  ","  b  "]}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var dest struct {
		Items []string `json:"items"`
	}
	if err := c.Bind(&dest); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if len(dest.Items) != 2 {
		t.Fatalf("items len: got %d", len(dest.Items))
	}
	if dest.Items[0] != "a" || dest.Items[1] != "b" {
		t.Errorf("items: got %q", dest.Items)
	}
}

func TestNormalizeBinder_NilInnerUsesDefaultBinder(t *testing.T) {
	e := echo.New()
	e.Binder = NewNormalizeBinder(nil)

	body := []byte(`{"x":"  y  "}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var dest struct {
		X string `json:"x"`
	}
	if err := c.Bind(&dest); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if dest.X != "y" {
		t.Errorf("x: got %q", dest.X)
	}
}

func TestNormalizeBinder_CaseLowerTag(t *testing.T) {
	e := echo.New()
	e.Binder = NewNormalizeBinder(e.Binder)

	body := []byte(`{"email":"  Foo@Bar.COM  ","other":"  As-Is  "}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var dest struct {
		Email string `json:"email" case:"lower"`
		Other string `json:"other"`
	}
	if err := c.Bind(&dest); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if dest.Email != "foo@bar.com" {
		t.Errorf("email: got %q", dest.Email)
	}
	if dest.Other != "As-Is" {
		t.Errorf("other: got %q", dest.Other)
	}
}

func TestNormalizeBinder_CaseUpperTag(t *testing.T) {
	e := echo.New()
	e.Binder = NewNormalizeBinder(e.Binder)

	body := []byte(`{"code":"  abc123  ","other":"  As-Is  "}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var dest struct {
		Code  string `json:"code" case:"upper"`
		Other string `json:"other"`
	}
	if err := c.Bind(&dest); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if dest.Code != "ABC123" {
		t.Errorf("code: got %q", dest.Code)
	}
	if dest.Other != "As-Is" {
		t.Errorf("other: got %q", dest.Other)
	}
}
