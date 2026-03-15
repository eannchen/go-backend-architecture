#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./scripts/bootstrap-template.sh --module <go-module>

Examples:
  ./scripts/bootstrap-template.sh --module github.com/acme/orders-api

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

# Only [a-z0-9_-] allowed; hyphens preserved so e.g. vocynex-api stays vocynex-api.
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

  # Derive template module from go.mod so bootstrap works for any module path
  # (e.g. "go-backend-architecture" or "github.com/eannchen/go-backend-architecture").
  local template_module
  template_module="$(grep -E '^module ' go.mod | sed -E 's/^module +//' | head -1)"
  if [[ -z "$template_module" ]]; then
    echo "could not read module from go.mod" >&2
    exit 1
  fi

  replace_in_file "go.mod" "module ${template_module}" "module ${module}"

  # Replace imports only in Go source files.
  local go_file
  while IFS= read -r go_file; do
    replace_in_file "$go_file" "\"${template_module}/" "\"${module}/"
  done < <(find . -type f -name '*.go' -not -path './.git/*' -not -path './volumes/*' -not -path './tmp/*')

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
    "docs/openapi.yaml"

  # Badge URLs use the GitHub owner/repo path (e.g. eannchen/go-backend-architecture).
  # Replace before slug tokens so the partial slug match doesn't leave a stale owner.
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

  # Keep local DB URLs aligned with the project slug even if template defaults drift.
  replace_local_db_name "${project_slug}" ".env.example"
  replace_local_db_name "${project_slug}" "Makefile"

  cat <<EOF
Updated template identifiers:
  module path:  ${module}
  service name: ${service_name}
  project slug: ${project_slug}
  api title:    ${api_title}

Next steps:
  1. Review .env.example, docker-compose.yml, and docs/openapi.yaml.
  2. Run make openapi-generate
  3. Run make test
EOF
}

main "$@"
