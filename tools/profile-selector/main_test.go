package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSelectionPlanOwnsOneTransport(t *testing.T) {
	publicPlan, err := newSelectionPlan(publicHTTPProfile)
	if err != nil {
		t.Fatalf("newSelectionPlan() error = %v", err)
	}
	grpcPlan, err := newSelectionPlan(serviceGRPCProfile)
	if err != nil {
		t.Fatalf("newSelectionPlan() error = %v", err)
	}

	if !contains(publicPlan.remove, "cmd/grpcapi") || contains(publicPlan.remove, "cmd/httpapi") {
		t.Fatalf("public HTTP removal manifest = %#v", publicPlan.remove)
	}
	if !contains(grpcPlan.remove, "cmd/httpapi") || contains(grpcPlan.remove, "cmd/grpcapi") {
		t.Fatalf("service gRPC removal manifest = %#v", grpcPlan.remove)
	}
}

func TestCombineFilesPreservesFragmentOrder(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "common", "COMMON=true\n")
	writeTestFile(t, root, "profile", "PROFILE=true\n")

	content, err := combineFiles(root, []string{"common", "profile"})
	if err != nil {
		t.Fatalf("combineFiles() error = %v", err)
	}
	if got := string(content); !strings.Contains(got, "COMMON=true\n\nPROFILE=true") {
		t.Fatalf("combineFiles() = %q", got)
	}
}

func TestApplySelectionGeneratesOutputsAndRemovesOwnedPaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "common.env", "COMMON=true\n")
	writeTestFile(t, root, "profile.env", "PROFILE=true\n")
	writeTestFile(t, root, ".air.profile.toml", "air")
	writeTestFile(t, root, "sqlc.profile.yaml", "sqlc")
	writeTestFile(t, root, "unused/file", "remove")

	plan := selectionPlan{
		profile:      publicHTTPProfile,
		remove:       []string{"unused"},
		airSource:    ".air.profile.toml",
		sqlcSource:   "sqlc.profile.yaml",
		envFragments: []string{"common.env", "profile.env"},
	}
	if err := applySelection(root, plan); err != nil {
		t.Fatalf("applySelection() error = %v", err)
	}

	for _, path := range []string{".env.example", ".air.toml", "sqlc.yaml", ".template-profile"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Errorf("generated path %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "unused")); !os.IsNotExist(err) {
		t.Fatalf("unused path still exists: %v", err)
	}
}

func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
