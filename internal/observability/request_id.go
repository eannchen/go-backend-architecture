package observability

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const MaxRequestIDLength = 128

// GenerateRequestID returns a cryptographically random request identifier.
func GenerateRequestID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate request ID: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

// IsValidRequestID reports whether an identifier is safe to propagate and log.
func IsValidRequestID(id string) bool {
	if len(id) == 0 || len(id) > MaxRequestIDLength {
		return false
	}
	for _, char := range id {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' || char == '.') {
			return false
		}
	}
	return true
}
