#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
gobin="$(go env GOBIN)"
if [[ -z "$gobin" ]]; then
  gopath="$(go env GOPATH)"
  gobin="${gopath%%:*}/bin"
fi
expected_bin="${gobin}/html"
path_bin="$(command -v html || true)"
cache_dir="$(mktemp -d "${TMPDIR:-/tmp}/html-installed-cache.XXXXXX")"
trap 'rm -rf "$cache_dir"' EXIT

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

[[ "$source_help" == "$installed_help" ]] || fail "source and installed help contracts differ"
require_contains "source help" "$source_help" "Usage:"
require_contains "source help" "$source_help" "--version"

source_version="$(cd "$repo_dir" && go run ./cmd/html --version)"
installed_version="$("$path_bin" --version)"
[[ "$source_version" == "$installed_version" ]] || fail "source and installed versions differ"
[[ "$installed_version" == "html devel" ]] || fail "local installed version is $installed_version, want html devel"

module_info="$(go version -m "$path_bin")"
require_contains "PATH html module info" "$module_info" $'path\tgithub.com/dotcommander/html/cmd/html'

smoke_path="$(printf '| a | b |\n|---|---|\n| 1 | 2 |\n' | HTML_CACHE_DIR="$cache_dir" "$path_bin" -n)"
[[ -n "$smoke_path" ]] || fail "stdin smoke did not print a cache path"
[[ -f "$smoke_path" ]] || fail "stdin smoke cache path does not exist: $smoke_path"

require_file_contains "stdin smoke HTML" "$smoke_path" "<table>"
require_file_contains "stdin smoke HTML" "$smoke_path" "theme-toggle"

printf 'installed-check: ok (%s)\n' "$path_bin"
