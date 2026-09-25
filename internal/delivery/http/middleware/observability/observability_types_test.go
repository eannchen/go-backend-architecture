package observabilitymw

import "testing"

// TestRequestInfoUsesBoundedMethodAndOmitsUnknownRoute checks method and route labels stay bounded when no named route matches.
func TestRequestInfoUsesBoundedMethodAndOmitsUnknownRoute(t *testing.T) {
	method, original := normalizeRequestMethod("CUSTOM")
	request := requestInfo{
		requestMethod:         method,
		requestMethodOriginal: original,
		urlPath:               "/unmatched/42",
		urlScheme:             "http",
	}

	fields := request.spanStartFields()
	if fields[keyHTTPRequestMethod] != "_OTHER" || fields[keyHTTPRequestMethodOriginal] != "CUSTOM" {
		t.Fatalf("method fields = %#v", fields)
	}
	if _, exists := fields[keyHTTPRoute]; exists {
		t.Fatalf("unmatched request has an invented route: %#v", fields)
	}
	if got := request.spanName(); got != "HTTP" {
		t.Fatalf("span name = %q, want HTTP", got)
	}
}
