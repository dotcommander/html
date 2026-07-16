#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${repo_dir}/html"
work_dir="${repo_dir}/.work/html-qa/config-smoke"
src_dir="${work_dir}/src"
out_dir="${work_dir}/out"
home_dir="${work_dir}/home"
bad_home_dir="${work_dir}/bad-home"
path_file="$(mktemp "${TMPDIR:-/tmp}/html-qa-config-path.XXXXXX")"
err_file="$(mktemp "${TMPDIR:-/tmp}/html-qa-config-err.XXXXXX")"
trap 'rm -f "$path_file" "$err_file"' EXIT

fail() {
  printf 'qa-config: %s\n' "$*" >&2
  exit 1
}

require_file_contains() {
  local label="$1"
  local file="$2"
  local needle="$3"

  if ! LC_ALL=C grep -Fq "$needle" "$file"; then
    fail "$label is missing expected text: $needle"
  fi
}

[[ -x "$bin" ]] || fail "local binary is missing or not executable: $bin; run just build first"

rm -rf "$work_dir"
mkdir -p "${src_dir}" "${out_dir}" "${home_dir}/.config/html" "${bad_home_dir}/.config/html"

cat >"${home_dir}/.config/html/config.json" <<'EOF_CONFIG'
{
  "max_width": "44rem",
  "default_theme": "dark",
  "default_palette": "blue",
  "toc": true
}
EOF_CONFIG

cat >"${src_dir}/configured.md" <<'EOF_MD'
# Configured Output

This document is rendered through a temporary user config.

## Alpha

First section.

## Beta

Second section.
EOF_MD

HOME="$home_dir" "$bin" --no-open --output "${out_dir}/configured.html" "${src_dir}/configured.md" >"$path_file"
printed="$(tr -d '\r\n' <"$path_file")"
[[ "$printed" == "${out_dir}/configured.html" ]] || fail "configured printed $printed, want ${out_dir}/configured.html"
require_file_contains configured "${out_dir}/configured.html" '<!DOCTYPE html>'
require_file_contains configured "${out_dir}/configured.html" 'HTML_DEFAULT_THEME = "dark"'
require_file_contains configured "${out_dir}/configured.html" 'HTML_DEFAULT_PALETTE = "blue"'
require_file_contains configured "${out_dir}/configured.html" '.markdown-body { max-width: 44rem; }'
require_file_contains configured "${out_dir}/configured.html" '<nav class="toc" aria-label="Table of contents">'
require_file_contains configured "${out_dir}/configured.html" 'href="#alpha"'
require_file_contains configured "${out_dir}/configured.html" 'href="#beta"'

cat >"${bad_home_dir}/.config/html/config.json" <<'EOF_BAD_CONFIG'
{"default_palette":"chartreuse"}
EOF_BAD_CONFIG
if HOME="$bad_home_dir" "$bin" --no-open --output "${out_dir}/bad.html" "${src_dir}/configured.md" >"$path_file" 2>"$err_file"; then
  fail "invalid config unexpectedly succeeded"
fi
require_file_contains invalid-config "$err_file" 'default_palette must be'
[[ ! -f "${out_dir}/bad.html" ]] || fail "invalid config wrote an output file"
cp "$err_file" "${out_dir}/invalid-config.txt"

require_file_contains readme-config "${repo_dir}/README.md" '"default_palette": "blue"'
require_file_contains readme-config "${repo_dir}/README.md" 'palettes are `sepia`'
require_file_contains readme-config "${repo_dir}/README.md" '`blue`, `green`, `rose`, or `catppuccin`'
require_file_contains claude-config "${repo_dir}/CLAUDE.md" '"default_palette": "blue"'
require_file_contains claude-config "${repo_dir}/CLAUDE.md" 'Output-affecting fields (`max_width`, `default_theme`, `default_palette`, `toc`)'

cat >"${work_dir}/index.html" <<'EOF_INDEX'
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>HTML Config Smoke</title>
  <style>
    body { margin: 0; background: #f5f4f1; color: #25221f; font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    main { width: min(calc(100% - 2rem), 72rem); margin: 1.25rem auto 2rem; }
    h1 { margin: 0 0 .25rem; font-size: 1.55rem; line-height: 1.15; }
    p { margin: 0 0 1rem; color: #6d6760; line-height: 1.45; }
    .grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 1rem; }
    section { overflow: hidden; background: #fffefa; border: 1px solid #ded8cc; border-radius: 8px; box-shadow: 0 .9rem 2.4rem rgba(35, 30, 24, .07); }
    header { display: flex; justify-content: space-between; gap: .75rem; align-items: center; padding: .75rem .85rem; background: #f1ece3; border-bottom: 1px solid #ded8cc; }
    h2 { margin: 0; font-size: .95rem; }
    a { color: #8b5b2f; font-weight: 800; text-decoration: none; }
    .proof { margin: 0; padding: .75rem .85rem; color: #6d6760; border-bottom: 1px solid #ded8cc; font-size: .84rem; line-height: 1.4; }
    iframe { display: block; width: 100%; height: 34rem; border: 0; background: white; }
    pre { margin: 0; padding: .85rem; overflow: auto; background: #25221f; color: #fffefa; font-size: .78rem; }
    @media (max-width: 45rem) {
      main { width: 100%; margin: 0; padding: 1rem 0 1.25rem; }
      h1, p { padding-inline: 1rem; }
      section { border-left: 0; border-right: 0; border-radius: 0; }
      iframe { height: 28rem; }
    }
  </style>
</head>
<body>
  <main>
    <h1>HTML Config Smoke</h1>
    <p>Generated from the real ./html binary with a temporary HOME and user config.</p>
    <div class="grid">
      <section>
        <header><h2>Configured Output</h2><a href="out/configured.html">Open</a></header>
        <p class="proof">Config applies max_width=44rem, default_theme=dark, default_palette=blue, and toc=true.</p>
        <iframe title="Configured output" src="out/configured.html" loading="eager" onload="this.dataset.loaded='true'"></iframe>
      </section>
      <section>
        <header><h2>Invalid Config Error</h2><a href="out/invalid-config.txt">Open</a></header>
        <p class="proof">Malformed/unsupported config values fail before rendering and do not create output.</p>
        <pre>
EOF_INDEX

html_escape_err="$(sed 's/&/\&amp;/g; s/</\&lt;/g; s/>/\&gt;/g' "${out_dir}/invalid-config.txt")"
printf '%s\n' "$html_escape_err" >>"${work_dir}/index.html"

cat >>"${work_dir}/index.html" <<'EOF_INDEX'
        </pre>
      </section>
    </div>
  </main>
</body>
</html>
EOF_INDEX

printf 'qa-config: ok (%s)\n' "${work_dir}/index.html"
