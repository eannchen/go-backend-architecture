package observability

import "maps"

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

// Fields is an observability-agnostic key/value hashmap.
type Fields map[string]any

// FromPairs creates attributes from key/value pairs. If pairs length is odd, the last key is ignored.
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

func OptionalFields(fieldSets ...Fields) Fields {
	if len(fieldSets) == 0 {
		return nil
	}
	return fieldSets[0]
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
