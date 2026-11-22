package grpcmetadata

import (
	"fmt"
	"strings"
)

// NormalizeKey trims whitespace, lowercases an application-defined gRPC metadata
// key, and validates its characters. Empty input returns ("", nil), allowing
// optional metadata fields to remain disabled. The grpc- prefix is reserved and
// cannot be used for application-defined keys.
func NormalizeKey(key string) (string, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return "", nil
	}
	if strings.HasPrefix(key, "grpc-") {
		return "", fmt.Errorf("metadata key %q uses the reserved grpc- prefix", key)
	}
	for _, character := range key {
		if isKeyCharacter(character) {
			continue
		}
		return "", fmt.Errorf("metadata key %q must contain only letters, digits, hyphens, underscores, or periods", key)
	}
	return key, nil
}

func isKeyCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= '0' && character <= '9' ||
		character == '-' || character == '_' || character == '.'
}
