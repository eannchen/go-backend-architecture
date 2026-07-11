package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeEnvFile_MergesOnlyUnsetValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("# comment\nTESTUTIL_EXISTS=from-file\nTESTUTIL_ADDED='from file'\ninvalid line\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("TESTUTIL_EXISTS", "from-shell")
	unsetForTest(t, "TESTUTIL_ADDED")
	mergeEnvFile(t, path)

	if got := os.Getenv("TESTUTIL_EXISTS"); got != "from-shell" {
		t.Fatalf("existing value = %q, want shell value", got)
	}
	if got := os.Getenv("TESTUTIL_ADDED"); got != "from file" {
		t.Fatalf("loaded value = %q, want value from .env", got)
	}
}

func unsetForTest(t *testing.T, key string) {
	t.Helper()

	previous, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, previous)
			return
		}
		_ = os.Unsetenv(key)
	})
}
