package logger

import "maps"

type Severity uint8

const (
	SeverityDebug Severity = iota
	SeverityInfo
	SeverityWarn
	SeverityError
)

// Fields is a logger-agnostic key/value hashmap.
type Fields map[string]any

// FromPairs creates fields from key/value pairs. If pairs length is odd, the last key is ignored.
func FromPairs(pairs ...any) Fields {
	out := make(Fields, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok || key == "" {
			continue
		}
		out[key] = pairs[i+1]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func OptionalFields(optionalFields ...Fields) Fields {
	if len(optionalFields) == 0 {
		return nil
	}
	return optionalFields[0]
}

func CloneFields(fields Fields) Fields {
	if len(fields) == 0 {
		return nil
	}
	cloned := make(Fields, len(fields))
	maps.Copy(cloned, fields)
	return cloned
}

func MergeFields(fieldSets ...Fields) Fields {
	total := 0
	for _, fields := range fieldSets {
		total += len(fields)
	}
	if total == 0 {
		return nil
	}
	out := make(Fields, total)
	for _, fields := range fieldSets {
		maps.Copy(out, fields)
	}
	return out
}
