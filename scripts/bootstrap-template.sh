#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./scripts/bootstrap-template.sh --module <go-module> [--service-name <name>] [--project-slug <slug>] [--api-title <title>]

Examples:
  ./scripts/bootstrap-template.sh --module github.com/acme/orders-api
  ./scripts/bootstrap-template.sh --module github.com/acme/orders-api --service-name orders-api --project-slug orders_api --api-title "Orders API"

Options:
  --module        Required. New Go module path.
  --service-name  Optional. Replaces SERVICE_NAME and the root README title. Default: basename of module path.
  --project-slug  Optional. Replaces local stack/database/container naming. Default: service name lowercased with non-alphanumerics replaced by underscores.
  --api-title     Optional. Replaces the OpenAPI title. Default: "<service-name> API".
  --help          Show this help.
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

to_project_slug() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/_/g; s/^_+//; s/_+$//'
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
  local service_name=""
  local project_slug=""
  local api_title=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --module)
        module="${2:-}"
        shift 2
        ;;
      --service-name)
        service_name="${2:-}"
        shift 2
        ;;
      --project-slug)
        project_slug="${2:-}"
        shift 2
        ;;
      --api-title)
        api_title="${2:-}"
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

  if [[ -z "$service_name" ]]; then
    service_name="${module##*/}"
  fi

  if [[ -z "$project_slug" ]]; then
    project_slug="$(to_project_slug "$service_name")"
  fi

  if [[ -z "$project_slug" ]]; then
    echo "derived project slug is empty; pass --project-slug explicitly" >&2
    exit 1
  fi

  if [[ -z "$api_title" ]]; then
    api_title="${service_name} API"
  fi

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
    "# go-backend-architecture" \
    "# ${service_name}" \
    "README.md"

  replace_in_files \
    "Go Backend Architecture Template API" \
    "${api_title}" \
    "docs/openapi.yaml"

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
  3. Run go test ./...
EOF
}

main "$@"
