package testutil

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SkipUnlessEnv skips when any required key is unset after loading a nearby .env file.
func SkipUnlessEnv(t testing.TB, keys ...string) {
	t.Helper()
	loadEnvFromDotEnv(t)

	missing := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		t.Skipf("integration: set %s (optional: .env at repo root)", strings.Join(missing, ", "))
	}
}

func loadEnvFromDotEnv(t testing.TB) {
	t.Helper()

	path, err := findDotEnvPath()
	if err != nil {
		return
	}
	mergeEnvFile(t, path)
}

func findDotEnvPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf(".env not found from working directory upwards")
}

func mergeEnvFile(t testing.TB, path string) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open .env: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		value = strings.Trim(strings.TrimSpace(value), `"'`)
		t.Setenv(key, value)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan .env: %v", err)
	}
}
