package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateToken returns a cryptographically random hex string of the given byte length.
// Used by OAuth state and session token generation.
func GenerateToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}
