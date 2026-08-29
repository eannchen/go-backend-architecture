package binding

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/labstack/echo/v5"
)

type nestedValue struct {
	Value string
}

func TestNormalizeStrings(t *testing.T) {
	tests := []struct {
		name   string
		target any
		want   any
	}{
		{
			name: "strings and tags",
			target: &struct {
				Trimmed string
				Raw     string `trim:"false"`
				Lower   string `case:"lower"`
				Upper   string `case:"upper"`
			}{Trimmed: "  value  ", Raw: "  raw  ", Lower: "  Foo  ", Upper: "  bar  "},
			want: &struct {
				Trimmed string
				Raw     string `trim:"false"`
				Lower   string `case:"lower"`
				Upper   string `case:"upper"`
			}{Trimmed: "value", Raw: "  raw  ", Lower: "foo", Upper: "BAR"},
		},
		{
			name: "nested struct and pointer",
			target: &struct {
				Nested  nestedValue
				Pointer *nestedValue
			}{Nested: nestedValue{Value: "  nested  "}, Pointer: &nestedValue{Value: "  pointer  "}},
			want: &struct {
				Nested  nestedValue
				Pointer *nestedValue
			}{Nested: nestedValue{Value: "nested"}, Pointer: &nestedValue{Value: "pointer"}},
		},
		{
			name: "string struct and pointer slices",
			target: &struct {
				Strings  []string `case:"lower"`
				Raw      []string `trim:"false"`
				Structs  []nestedValue
				Pointers []*nestedValue
			}{
				Strings:  []string{"  ONE  ", "  Two  "},
				Raw:      []string{"  unchanged  "},
				Structs:  []nestedValue{{Value: "  struct  "}},
				Pointers: []*nestedValue{{Value: "  pointer  "}, nil},
			},
			want: &struct {
				Strings  []string `case:"lower"`
				Raw      []string `trim:"false"`
				Structs  []nestedValue
				Pointers []*nestedValue
			}{
				Strings:  []string{"one", "two"},
				Raw:      []string{"  unchanged  "},
				Structs:  []nestedValue{{Value: "struct"}},
				Pointers: []*nestedValue{{Value: "pointer"}, nil},
			},
		},
		{name: "nil target", target: nil, want: nil},
		{name: "non-pointer target", target: struct{ Value string }{Value: "  unchanged  "}, want: struct{ Value string }{Value: "  unchanged  "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizeStrings(tt.target)
			if !reflect.DeepEqual(tt.target, tt.want) {
				t.Fatalf("normalized value = %#v, want %#v", tt.target, tt.want)
			}
		})
	}
}

func TestNormalizeBinder_UsesDefaultBinderThenNormalizes(t *testing.T) {
	e := echo.New()
	e.Binder = NewNormalizeBinder(nil)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"email":"  Foo@Bar.COM  "}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req, httptest.NewRecorder())
	var target struct {
		Email string `json:"email" case:"lower"`
	}

	if err := c.Bind(&target); err != nil {
		t.Fatalf("bind request: %v", err)
	}
	if target.Email != "foo@bar.com" {
		t.Fatalf("email = %q, want normalized email", target.Email)
	}
}

func TestNormalizeBinder_DoesNotNormalizeAfterBindingFailure(t *testing.T) {
	wantErr := errors.New("bind failed")
	binder := NewNormalizeBinder(binderFunc(func(*echo.Context, any) error { return wantErr }))
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodPost, "/", nil), httptest.NewRecorder())
	target := struct{ Value string }{Value: "  unchanged  "}

	err := binder.Bind(c, &target)

	if !errors.Is(err, wantErr) {
		t.Fatalf("bind error = %v, want %v", err, wantErr)
	}
	if target.Value != "  unchanged  " {
		t.Fatalf("value = %q, want unchanged value", target.Value)
	}
}

type binderFunc func(*echo.Context, any) error

func (f binderFunc) Bind(c *echo.Context, target any) error {
	return f(c, target)
}
