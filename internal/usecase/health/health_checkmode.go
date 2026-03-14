package health

import "fmt"

type CheckMode string

const (
	CheckModeLive  CheckMode = "live"
	CheckModeReady CheckMode = "ready"
)

func (m CheckMode) IsValid() bool {
	switch m {
	case CheckModeLive, CheckModeReady:
		return true
	default:
		return false
	}
}

func ParseCheckMode(raw string) (CheckMode, error) {
	if raw == "" {
		return CheckModeReady, nil
	}

	mode := CheckMode(raw)
	if !mode.IsValid() {
		return "", fmt.Errorf("invalid check mode %q, allowed: live, ready", raw)
	}
	return mode, nil
}
