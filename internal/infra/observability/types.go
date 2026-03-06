package observability

// Field is an observability-agnostic key/value attribute.
type Field struct {
	Key   string
	Value any
}

// FieldOf creates one key/value attribute.
func FieldOf(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// Fields creates attributes from key/value pairs.
//
// Example:
//
//	Fields("http.method", "GET", "http.route", "/healthz")
//
// If pairs length is odd, the last dangling key is ignored.
func Fields(pairs ...any) []Field {
	out := make([]Field, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok || key == "" {
			continue
		}
		out = append(out, FieldOf(key, pairs[i+1]))
	}
	return out
}

type Severity uint8

const (
	SeverityDebug Severity = iota
	SeverityInfo
	SeverityWarn
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityDebug:
		return "debug"
	case SeverityWarn:
		return "warn"
	case SeverityError:
		return "error"
	default:
		return "info"
	}
}
