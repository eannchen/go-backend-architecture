// Program profile-selector reduces the source template from two example server
// profiles to one project shape:
//
//   - public-http keeps the browser/public HTTP API, OpenAPI contract, sessions,
//     OTP/OAuth, rate limiting, and their user persistence.
//   - service-grpc keeps the service-to-service gRPC API, protobuf contract, mTLS,
//     caller identity, and outbound gRPC support.
//
// Run it once on a clean template checkout, before bootstrap-template.sh. Without
// --apply it only prints the planned deletions. With --apply it installs the chosen
// profile's conventional .air.toml, sqlc.yaml, and .env.example; deletes the other
// profile; writes .template-profile; and removes this one-time selector. It never
// connects to a database or changes deployed infrastructure.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const (
	publicHTTPProfile  = "public-http"
	serviceGRPCProfile = "service-grpc"
)

// selectionPlan is the complete file-level transformation for one choice. Profile
// ownership is explicit so adding a capability requires deliberately deciding
// whether it is shared, public-HTTP-only, or service-gRPC-only.
type selectionPlan struct {
	profile      string
	remove       []string
	airSource    string
	sqlcSource   string
	envFragments []string
}

func main() {
	profile := flag.String("profile", "", "profile to keep: public-http or service-grpc")
	apply := flag.Bool("apply", false, "apply the selection; omission prints a dry run")
	flag.Parse()

	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	plan, err := newSelectionPlan(*profile)
	if err != nil {
		fatal(err)
	}
	if err := validateRepository(root, plan); err != nil {
		fatal(err)
	}

	printPlan(plan, *apply)
	if !*apply {
		fmt.Println("\nDry run only. Add --apply to make these changes.")
		return
	}
	if err := applySelection(root, plan); err != nil {
		fatal(err)
	}

	fmt.Printf("\nSelected %s. The selector removed itself so this choice cannot be applied twice.\n", plan.profile)
	fmt.Println("Next run: go mod tidy")
	fmt.Println("Then run: make check test")
}

// newSelectionPlan answers “what remains?” Common removals are inputs needed only
// while selecting. The chosen case then removes every capability owned exclusively
// by the other profile and identifies the chosen configuration fragments.
func newSelectionPlan(profile string) (selectionPlan, error) {
	commonRemovals := []string{
		".github/workflows/profile-selector.yml",
		"config/env",
		"tools/profile-selector",
	}

	switch profile {
	case publicHTTPProfile:
		return selectionPlan{
			profile: profile,
			remove: append(commonRemovals,
				".air.grpcapi.toml",
				".github/workflows/protobuf.yml",
				"buf.gen.yaml",
				"buf.yaml",
				"cmd/grpcapi",
				"contracts/grpc",
				"internal/app/grpcapi",
				"internal/delivery/grpc",
				"internal/infra/config/grpcapi_config.go",
				"internal/infra/config/grpcapi_config_test.go",
				"internal/infra/grpcclient",
				"internal/infra/security",
				"internal/observability/grpcsemconv",
				"internal/security",
				"internal/util/grpcmetadata",
				"make/service-grpc.mk",
				"sqlc.service-grpc.yaml",
			),
			airSource:    ".air.httpapi.toml",
			sqlcSource:   "sqlc.public-http.yaml",
			envFragments: []string{"config/env/common.env.example", "config/env/public-http.env.example"},
		}, nil
	case serviceGRPCProfile:
		return selectionPlan{
			profile: profile,
			remove: append(commonRemovals,
				".air.httpapi.toml",
				".github/workflows/openapi.yml",
				"cmd/httpapi",
				"contracts/http",
				"internal/app/httpapi",
				"internal/delivery/http",
				"internal/domain/user",
				"internal/infra/cache/redis/store/user_store.go",
				"internal/infra/cache/redis/store/user_store_integration_test.go",
				"internal/infra/composed/user",
				"internal/infra/config/httpapi_config.go",
				"internal/infra/config/httpapi_config_test.go",
				"internal/infra/db/postgres/sqlc/gen/models.go",
				"internal/infra/db/postgres/sqlc/gen/user.sql.go",
				"internal/infra/db/postgres/sqlc/schema/auth.sql",
				"internal/infra/db/postgres/sqlc/user.sql",
				"internal/infra/db/postgres/store/user_store.go",
				"internal/infra/db/postgres/store/user_store_integration_test.go",
				"internal/infra/db/postgres/migrations/00001_schema.sql",
				"internal/infra/external/oauth",
				"internal/infra/external/otp",
				"internal/repository/cache/cachetest/user_cache_store.go",
				"internal/repository/cache/user_repository.go",
				"internal/repository/db/dbtest/user_repository.go",
				"internal/repository/db/user_repository.go",
				"internal/repository/external/oauth",
				"internal/repository/external/otp",
				"internal/repository/kvstore/kvstoretest/oauth_state_repository.go",
				"internal/repository/kvstore/kvstoretest/otp_repository.go",
				"internal/repository/kvstore/kvstoretest/session_repository.go",
				"internal/repository/kvstore/kvstoretest/token_bucket_repository.go",
				"internal/repository/kvstore/oauth_state_repository.go",
				"internal/repository/kvstore/otp_repository.go",
				"internal/repository/kvstore/session_repository.go",
				"internal/repository/kvstore/sliding_window_repository.go",
				"internal/repository/kvstore/token_bucket_repository.go",
				"internal/infra/kvstore/redis/store/oauth_state_store.go",
				"internal/infra/kvstore/redis/store/oauth_state_store_integration_test.go",
				"internal/infra/kvstore/redis/store/otp_store.go",
				"internal/infra/kvstore/redis/store/otp_store_integration_test.go",
				"internal/infra/kvstore/redis/store/rate_limit_store_integration_test.go",
				"internal/infra/kvstore/redis/store/session_store.go",
				"internal/infra/kvstore/redis/store/session_store_integration_test.go",
				"internal/infra/kvstore/redis/store/sliding_window_store.go",
				"internal/infra/kvstore/redis/store/token_bucket_store.go",
				"internal/usecase/auth",
				"internal/usecase/globalratelimit",
				"make/public-http.mk",
				"oapi-codegen.yaml",
				"sqlc.public-http.yaml",
			),
			airSource:    ".air.grpcapi.toml",
			sqlcSource:   "sqlc.service-grpc.yaml",
			envFragments: []string{"config/env/common.env.example", "config/env/service-grpc.env.example"},
		}, nil
	default:
		return selectionPlan{}, fmt.Errorf("--profile must be %q or %q", publicHTTPProfile, serviceGRPCProfile)
	}
}

// validateRepository makes application all-or-nothing for predictable repository
// state: it confirms this is the template root, selection has not already happened,
// every expected source exists, destination files will not be overwritten, and Git
// is clean enough to recover or review the resulting deletion as one change.
func validateRepository(root string, plan selectionPlan) error {
	moduleFile, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !bytes.HasPrefix(moduleFile, []byte("module ")) {
		return errors.New("run the selector from the template repository root")
	}
	if _, err := os.Stat(filepath.Join(root, ".template-profile")); err == nil {
		return errors.New("a profile has already been selected")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect .template-profile: %w", err)
	}
	for _, destination := range []string{".air.toml", "sqlc.yaml"} {
		if _, err := os.Stat(filepath.Join(root, destination)); err == nil {
			return fmt.Errorf("refusing to replace existing %s", destination)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect %s: %w", destination, err)
		}
	}

	required := append([]string{plan.airSource, plan.sqlcSource}, plan.envFragments...)
	required = append(required, plan.remove...)
	for _, path := range required {
		if _, err := os.Lstat(filepath.Join(root, path)); err != nil {
			return fmt.Errorf("profile manifest is stale: required path %q: %w", path, err)
		}
	}

	command := exec.Command("git", "status", "--porcelain", "--untracked-files=normal")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("check Git working tree: %w", err)
	}
	if len(bytes.TrimSpace(output)) != 0 {
		return errors.New("Git working tree must be clean before profile selection")
	}
	return nil
}

// printPlan is the default user experience. A command without --apply reaches this
// point and exits, allowing the destructive scope to be reviewed first.
func printPlan(plan selectionPlan, apply bool) {
	mode := "Dry run"
	if apply {
		mode = "Applying"
	}
	formatted := append([]string(nil), plan.remove...)
	slices.Sort(formatted)
	fmt.Printf("%s profile %q:\n", mode, plan.profile)
	for _, path := range formatted {
		fmt.Printf("  remove %s\n", path)
	}
	fmt.Printf("  generate .env.example from %s\n", strings.Join(plan.envFragments, " + "))
	fmt.Printf("  rename %s to .air.toml\n", plan.airSource)
	fmt.Printf("  rename %s to sqlc.yaml\n", plan.sqlcSource)
	fmt.Println("  write .template-profile")
}

// applySelection first creates the files the selected project needs, then removes
// the other profile. The plan is already in memory, so deleting the selector itself
// at the end does not interrupt the running executable.
func applySelection(root string, plan selectionPlan) error {
	environment, err := combineFiles(root, plan.envFragments)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, ".env.example"), environment, 0o644); err != nil {
		return fmt.Errorf("write .env.example: %w", err)
	}
	if err := os.Rename(filepath.Join(root, plan.airSource), filepath.Join(root, ".air.toml")); err != nil {
		return fmt.Errorf("install Air configuration: %w", err)
	}
	if err := os.Rename(filepath.Join(root, plan.sqlcSource), filepath.Join(root, "sqlc.yaml")); err != nil {
		return fmt.Errorf("install sqlc configuration: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".template-profile"), []byte(plan.profile+"\n"), 0o644); err != nil {
		return fmt.Errorf("write profile marker: %w", err)
	}
	for _, path := range plan.remove {
		if err := removeOwnedPath(root, path); err != nil {
			return err
		}
	}
	return nil
}

// combineFiles turns the temporary shared/profile fragments into the project's one
// .env.example. The fragments are later removed so the selected project has only
// one environment example to maintain.
func combineFiles(root string, paths []string) ([]byte, error) {
	var result bytes.Buffer
	result.WriteString("# Generated by the template profile selector.\n\n")
	for index, path := range paths {
		content, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return nil, fmt.Errorf("read environment fragment %s: %w", path, err)
		}
		if index > 0 {
			result.WriteByte('\n')
		}
		result.Write(bytes.TrimRight(content, "\n"))
		result.WriteByte('\n')
	}
	return result.Bytes(), nil
}

// removeOwnedPath refuses absolute paths and parent traversal. Even if the manifest
// is edited incorrectly, deletion cannot escape the repository root.
func removeOwnedPath(root, relative string) error {
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe removal path %q", relative)
	}
	if err := os.RemoveAll(filepath.Join(root, clean)); err != nil {
		return fmt.Errorf("remove %s: %w", relative, err)
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "profile selector:", err)
	os.Exit(1)
}
