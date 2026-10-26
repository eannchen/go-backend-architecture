package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewSelectionPlanOwnsOneTransport checks a selected profile owns only its chosen transport paths.
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

// TestCombineFilesPreservesFragmentOrder checks generated contract fragments retain their intended order.
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

// TestApplySelectionGeneratesOutputsAndRemovesOwnedPaths checks selection writes chosen outputs and removes paths owned by the other transport.
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

// TestSelectedProfilesBuild applies each manifest to a disposable source copy.
// Building the result catches imports left pointing at deleted capabilities.
func TestSelectedProfilesBuild(t *testing.T) {
	source, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	command.Dir = source
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{publicHTTPProfile, serviceGRPCProfile, kafkaWorkerProfile, kafkaShareWorkerProfile} {
		t.Run(profile, func(t *testing.T) {
			root := t.TempDir()
			for _, relative := range strings.Split(string(output), "\x00") {
				if relative == "" {
					continue
				}
				info, err := os.Lstat(filepath.Join(source, relative))
				if err != nil {
					t.Fatal(err)
				}
				if !info.Mode().IsRegular() {
					continue
				}
				content, err := os.ReadFile(filepath.Join(source, relative))
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, root, relative, string(content))
			}
			plan, err := newSelectionPlan(profile)
			if err != nil {
				t.Fatal(err)
			}
			for _, relative := range append(append([]string{plan.airSource, plan.sqlcSource}, plan.envFragments...), plan.remove...) {
				if _, err := os.Stat(filepath.Join(root, relative)); err != nil {
					t.Fatalf("stale manifest %s: %v", relative, err)
				}
			}
			if err := applySelection(root, plan); err != nil {
				t.Fatal(err)
			}
			build := exec.CommandContext(t.Context(), "go", "build", "./...")
			build.Dir = root
			if output, err := build.CombinedOutput(); err != nil {
				t.Fatalf("selected build: %v\n%s", err, output)
			}
			target := exec.CommandContext(t.Context(), "make", "-n", "run")
			target.Dir = root
			// make test can export its profile flags through .env's export directive.
			// A selected project must be checked as a fresh invocation, not a child profile.
			target.Env = append(os.Environ(), "PUBLIC_HTTP_PROFILE=", "SERVICE_GRPC_PROFILE=", "KAFKA_WORKER_PROFILE=", "KAFKA_SHARE_WORKER_PROFILE=", "MAKEFLAGS=", "MFLAGS=")
			output, err := target.CombinedOutput()
			if err != nil {
				t.Fatalf("run target: %v\n%s", err, output)
			}
			if strings.Contains(profile, "kafka") && !strings.Contains(string(output), "go run ./cmd/kafka") {
				t.Fatalf("wrong Kafka run target: %s", output)
			}
		})
	}
}
