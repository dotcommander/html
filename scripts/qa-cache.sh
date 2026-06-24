#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${repo_dir}/html"
work_dir="${repo_dir}/.work/html-qa/cache-smoke"
src_dir="${work_dir}/src"
cache_dir="${work_dir}/cache"
path_file="$(mktemp "${TMPDIR:-/tmp}/html-qa-cache-path.XXXXXX")"
trap 'rm -f "$path_file"' EXIT

fail() {
  printf 'qa-cache: %s\n' "$*" >&2
  exit 1
}

require_file_contains() {
  local label="$1"
  local file="$2"
  local needle="$3"

  if ! LC_ALL=C grep -Fq -- "$needle" "$file"; then
    fail "$label is missing expected text: $needle"
  fi
}

require_cache_path() {
  local label="$1"
  local path="$2"

  [[ "$path" == "${cache_dir}/"* ]] || fail "$label printed path outside isolated cache: $path"
  [[ "$path" == *.html ]] || fail "$label printed non-html cache path: $path"
  [[ -f "$path" ]] || fail "$label did not write cache file: $path"
  require_file_contains "$label" "$path" '<!DOCTYPE html>'
  require_file_contains "$label" "$path" 'class="theme-controls"'
  require_file_contains "$label" "$path" 'data-palette-choice="catppuccin"'
}

mtime() {
  stat -f '%m' "$1"
}

rel_cache_path() {
  local path="$1"
  printf '%s\n' "${path#${work_dir}/}"
}

[[ -x "$bin" ]] || fail "local binary is missing or not executable: $bin; run just build first"

rm -rf "$work_dir"
mkdir -p "$src_dir/media-assets" "$cache_dir"

printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\nIDATx\x9cc`\x00\x00\x00\x02\x00\x01\xe2!\xbc3\x00\x00\x00\x00IEND\xaeB`\x82' >"${src_dir}/media-assets/pixel.png"
cat >"${src_dir}/cache.md" <<'EOF_MD'
# Cache Smoke

Default output writes a self-contained HTML document into the cache.

## Details

| Path | State |
|---|---|
| file cache | ready |
| force rebuild | checked |

```go
fmt.Println("cache smoke")
```

![Pixel](media-assets/pixel.png)
EOF_MD

HTML_CACHE_DIR="$cache_dir" "$bin" --no-open "${src_dir}/cache.md" >"$path_file"
file_cache_path="$(tr -d '\r\n' <"$path_file")"
require_cache_path file-cache "$file_cache_path"
require_file_contains file-cache "$file_cache_path" '<h1 id="cache-smoke">Cache Smoke</h1>'
require_file_contains file-cache "$file_cache_path" 'data:image/png;base64,'
first_mtime="$(mtime "$file_cache_path")"

HTML_CACHE_DIR="$cache_dir" "$bin" --no-open "${src_dir}/cache.md" >"$path_file"
fresh_cache_path="$(tr -d '\r\n' <"$path_file")"
[[ "$fresh_cache_path" == "$file_cache_path" ]] || fail "fresh cache printed $fresh_cache_path, want $file_cache_path"
fresh_mtime="$(mtime "$fresh_cache_path")"
[[ "$fresh_mtime" == "$first_mtime" ]] || fail "fresh cache unexpectedly rewrote $fresh_cache_path"

sleep 1
HTML_CACHE_DIR="$cache_dir" "$bin" --no-open --force "${src_dir}/cache.md" >"$path_file"
forced_cache_path="$(tr -d '\r\n' <"$path_file")"
[[ "$forced_cache_path" == "$file_cache_path" ]] || fail "force cache printed $forced_cache_path, want $file_cache_path"
forced_mtime="$(mtime "$forced_cache_path")"
(( forced_mtime > fresh_mtime )) || fail "force cache did not rewrite $forced_cache_path"

printf '# Stdin Cache\n\n- [x] content hash path\n- [ ] browser capture\n' | HTML_CACHE_DIR="$cache_dir" "$bin" --no-open >"$path_file"
stdin_cache_path="$(tr -d '\r\n' <"$path_file")"
require_cache_path stdin-cache "$stdin_cache_path"
require_file_contains stdin-cache "$stdin_cache_path" '<h1 id="stdin-cache">Stdin Cache</h1>'
require_file_contains stdin-cache "$stdin_cache_path" 'type="checkbox"'

file_cache_rel="$(rel_cache_path "$file_cache_path")"
stdin_cache_rel="$(rel_cache_path "$stdin_cache_path")"

cat >"${work_dir}/index.html" <<EOF_INDEX
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>HTML Cache Smoke</title>
  <style>
    body { margin: 0; background: #f5f4f1; color: #25221f; font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    main { width: min(calc(100% - 2rem), 88rem); margin: 1.25rem auto 2rem; }
    h1 { margin: 0 0 .25rem; font-size: 1.55rem; line-height: 1.15; }
    p { margin: 0 0 1rem; color: #6d6760; line-height: 1.45; }
    .proof { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr)); gap: .65rem; margin: 0 0 1rem; }
    .proof div { min-width: 0; padding: .75rem; background: #fffefa; border: 1px solid #ded8cc; border-radius: 8px; box-shadow: 0 .7rem 2rem rgba(35, 30, 24, .06); }
    .proof dt, .proof dd { margin: 0; }
    .proof dt { color: #6d6760; font-size: .75rem; font-weight: 800; text-transform: uppercase; }
    .proof dd { margin-top: .2rem; overflow-wrap: anywhere; font-size: .86rem; font-weight: 700; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 28rem), 1fr)); gap: 1rem; }
    section { overflow: hidden; background: #fffefa; border: 1px solid #ded8cc; border-radius: 8px; box-shadow: 0 .9rem 2.4rem rgba(35, 30, 24, .07); }
    header { display: flex; justify-content: space-between; gap: .75rem; align-items: center; padding: .7rem .85rem; background: #f1ece3; border-bottom: 1px solid #ded8cc; }
    h2 { margin: 0; font-size: .92rem; }
    a { color: #8b5b2f; font-weight: 700; text-decoration: none; }
    iframe { display: block; width: 100%; height: 34rem; border: 0; background: white; }
    @media (max-width: 45rem) {
      main { width: 100%; margin: 0; }
      h1, p { padding-inline: 1rem; }
      h1 { padding-top: 1rem; }
      .proof { padding-inline: 1rem; }
      .grid { gap: .75rem; }
      section { border-left: 0; border-right: 0; border-radius: 0; }
      iframe { height: 30rem; }
    }
  </style>
</head>
<body>
  <main data-cache-contract data-cache-reused="true" data-cache-forced="true">
    <h1>HTML Cache Smoke</h1>
    <p>Generated by scripts/qa-cache.sh from the real ./html binary with an isolated HTML_CACHE_DIR.</p>
    <dl class="proof">
      <div><dt>File cache</dt><dd>${file_cache_rel}</dd></div>
      <div><dt>Fresh reuse</dt><dd>same path, unchanged mtime</dd></div>
      <div><dt>Force rebuild</dt><dd>same path, newer mtime</dd></div>
      <div><dt>Stdin cache</dt><dd>${stdin_cache_rel}</dd></div>
    </dl>
    <div class="grid">
      <section><header><h2>File Cache</h2><a href="${file_cache_rel}">Open</a></header><iframe title="File cache" src="${file_cache_rel}" loading="eager" onload="this.dataset.loaded='true'"></iframe></section>
      <section><header><h2>Stdin Cache</h2><a href="${stdin_cache_rel}">Open</a></header><iframe title="Stdin cache" src="${stdin_cache_rel}" loading="eager" onload="this.dataset.loaded='true'"></iframe></section>
    </div>
  </main>
</body>
</html>
EOF_INDEX

printf 'qa-cache: ok (%s)\n' "${work_dir}/index.html"
