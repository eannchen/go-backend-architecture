package observability

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
