#!/usr/bin/env bash

# Purpose
# -------
# Turn a freshly created copy of this template into a project with the user's own
# identity. For example, it changes the template Go module, service name, database
# name, Compose resource names, contract package paths, and visible API title to
# values derived from `--module github.com/acme/orders-api`.
#
# When to run it
# --------------
# Run the profile selector first if the project should contain only public HTTP or
# only service gRPC. Commit that selection, then run this bootstrap once. Bootstrap
# also works on the unselected two-profile template, but it does not choose or
# remove capabilities itself.
#
# What it does not do
# -------------------
# It does not rename the checkout directory, configure Git remotes, install tools,
# migrate a database, or run code generators. Those operations remain explicit so
# the script cannot unexpectedly modify the developer's machine or external state.
#
# Procedure
# ---------
# 1. Validate `--module` and derive the service name and local project slug.
# 2. Replace the old Go module in go.mod, Go imports, and protobuf go_package values.
# 3. Replace template identity in .env.example, Compose, contracts, and README.
# 4. Print the resulting identity, selected profile, and verification commands.

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./tools/bootstrap-template.sh --module <go-module>

Examples:
  ./tools/bootstrap-template.sh --module github.com/acme/orders-api

Options:
  --module        Required. New Go module path.
  --help          Show this help.
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

# Convert the module's final segment into a stable local resource name used by
# Compose containers, the database, and other project-scoped identifiers.
to_project_slug() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9-]+/_/g; s/^_+//; s/_+$//'
}

replace_in_file() {
  local file="$1"
  local old="$2"
  local new="$3"

  if [[ -z "$file" || ! -f "$file" ]]; then
    return 0
  fi

  if [[ -z "$old" || "$old" == "$new" ]]; then
    return 0
  fi

  # Perl's \Q...\E treats module paths and other input as literal text instead
  # of accidentally interpreting punctuation as a regular expression.
  OLD="$old" NEW="$new" perl -0pi -e 's/\Q$ENV{OLD}\E/$ENV{NEW}/g' "$file"
}

replace_in_files() {
  local old="$1"
  local new="$2"
  shift 2

  local file
  for file in "$@"; do
    replace_in_file "$file" "$old" "$new"
  done
}

replace_slug_tokens() {
  local slug="$1"
  shift
  replace_in_files "go-backend-architecture" "$slug" "$@"
  replace_in_files "go_backend_architecture" "$slug" "$@"
  replace_in_files "go-backend-architecture-local" "${slug}-local" "$@"
}

replace_local_db_name() {
  local db_name="$1"
  local file="$2"
  if [[ -z "$db_name" || -z "$file" || ! -f "$file" ]]; then
    return 0
  fi
  DB_NAME="$db_name" perl -0pi -e 's#(localhost:5432/)[^?]+(\?sslmode=disable)#$1$ENV{DB_NAME}$2#g' "$file"
}

main() {
  require_cmd perl

  # Phase 1: accept one required module path and reject ambiguous input early.
  local module=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --module)
        if [[ $# -lt 2 || "${2:-}" == --* ]]; then
          echo "--module requires a value" >&2
          usage >&2
          exit 1
        fi
        module="$2"
        shift 2
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      *)
        echo "unknown argument: $1" >&2
        usage >&2
        exit 1
        ;;
    esac
  done

  if [[ -z "$module" ]]; then
    echo "--module is required" >&2
    usage >&2
    exit 1
  fi

  local service_name="${module##*/}"

  local project_slug
  project_slug="$(to_project_slug "$service_name")"

  if [[ -z "$project_slug" ]]; then
    echo "derived project slug is empty; check --module value" >&2
    exit 1
  fi

  local api_title="${service_name} API"

  # Phase 2: derive the old module from go.mod instead of hard-coding it, then
  # update go.mod and every Go import that begins with that exact module path.
  local template_module
  template_module="$(grep -E '^module ' go.mod | sed -E 's/^module +//' | head -1)"
  if [[ -z "$template_module" ]]; then
    echo "could not read module from go.mod" >&2
    exit 1
  fi

  replace_in_file "go.mod" "module ${template_module}" "module ${module}"

  # Limit import rewriting to Go files and ignore generated/runtime directories.
  local go_file
  while IFS= read -r go_file; do
    replace_in_file "$go_file" "\"${template_module}/" "\"${module}/"
  done < <(find . -type f -name '*.go' -not -path './.git/*' -not -path './volumes/*' -not -path './tmp/*')

  # Phase 3: update project-facing names. Missing profile-owned files are ignored,
  # so the same bootstrap works after either HTTP or gRPC profile selection.
  replace_in_files \
    "SERVICE_NAME=go-backend-architecture" \
    "SERVICE_NAME=${service_name}" \
    ".env.example"

  replace_in_files \
    "# Go Backend Architecture" \
    "# ${service_name}" \
    "README.md"

  replace_in_files \
    "Go Backend Architecture Template API" \
    "${api_title}" \
    "contracts/http/openapi.yaml"

  # Protobuf go_package values are module-qualified and must match rewritten imports.
  if [[ -d "contracts/grpc" ]]; then
    local proto_file
    while IFS= read -r proto_file; do
      replace_in_file "$proto_file" "${template_module}/" "${module}/"
    done < <(find contracts/grpc -type f -name '*.proto')
  fi

  # Badge URLs use the full GitHub owner/repository path. Replace that first so a
  # later project-slug replacement cannot leave the old owner behind.
  local template_github_path="${template_module#github.com/}"
  local new_github_path="${module#github.com/}"
  if [[ "$template_github_path" != "$template_module" && "$new_github_path" != "$module" ]]; then
    replace_in_file "README.md" "$template_github_path" "$new_github_path"
  fi

  replace_slug_tokens "${project_slug}" \
    ".env.example" \
    "docker-compose.yml" \
    "README.md" \
    "Makefile"

  # Normalize local DB URLs separately because their database segment may already
  # differ from the literal template slug.
  replace_local_db_name "${project_slug}" ".env.example"
  replace_local_db_name "${project_slug}" "Makefile"

  # Phase 4: report which source shape was bootstrapped. The marker exists only
  # after the one-time profile selector has reduced the template to one transport.
  local profile="both"
  if [[ -f ".template-profile" ]]; then
    profile="$(tr -d '[:space:]' < .template-profile)"
  fi

  cat <<EOF
Updated template identifiers:
  module path:  ${module}
  service name: ${service_name}
  project slug: ${project_slug}
  api title:    ${api_title}
  profile:      ${profile}

Next steps:
  1. Review .env.example, docker-compose.yml, and the selected contract.
  2. Run go mod tidy.
  3. Run make check test.
EOF
}

main "$@"
