package logger

// Field is a logger-agnostic key/value pair.
type Field struct {
	Key   string
	Value any
}

// FieldOf creates one structured field.
func FieldOf(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// Fields creates fields from key/value pairs.
//
// Example:
//
//	Fields("address", ":8080", "component", "http_server")
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
