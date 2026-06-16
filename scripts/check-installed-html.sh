#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
expected_bin="${HOME}/go/bin/html"
path_bin="$(command -v html || true)"

fail() {
  printf 'installed-check: %s\n' "$*" >&2
  exit 1
}

require_contains() {
  local label="$1"
  local haystack="$2"
  local needle="$3"

  if [[ "$haystack" != *"$needle"* ]]; then
    fail "$label is missing expected text: $needle"
  fi
}

require_file_contains() {
  local label="$1"
  local file="$2"
  local needle="$3"

  if ! LC_ALL=C grep -Fq "$needle" "$file"; then
    fail "$label is missing expected text: $needle"
  fi
}

[[ -n "$path_bin" ]] || fail "html is not on PATH"
[[ "$path_bin" == "$expected_bin" ]] || fail "PATH html is $path_bin, want $expected_bin"
[[ -x "$path_bin" ]] || fail "$path_bin is not executable"

source_help="$(cd "$repo_dir" && go run ./cmd/html --help)"
installed_help="$("$path_bin" --help)"

expected_help=(
  "Render a Markdown file — or data piped on stdin"
  "Usage:"
  "html [file] [flags]"
  "--plain"
  "--markdown"
  "--lang string"
  "--title string"
  "--safe"
  "--no-open"
)

for text in "${expected_help[@]}"; do
  require_contains "source help" "$source_help" "$text"
  require_contains "PATH html help" "$installed_help" "$text"
done

module_info="$(go version -m "$path_bin")"
require_contains "PATH html module info" "$module_info" $'path\tgithub.com/dotcommander/html/cmd/html'

smoke_path="$(printf '| a | b |\n|---|---|\n| 1 | 2 |\n' | "$path_bin" -n)"
[[ -n "$smoke_path" ]] || fail "stdin smoke did not print a cache path"
[[ -f "$smoke_path" ]] || fail "stdin smoke cache path does not exist: $smoke_path"

require_file_contains "stdin smoke HTML" "$smoke_path" "<table>"
require_file_contains "stdin smoke HTML" "$smoke_path" "theme-toggle"

printf 'installed-check: ok (%s)\n' "$path_bin"
