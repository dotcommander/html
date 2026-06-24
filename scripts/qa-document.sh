#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${repo_dir}/html"
work_dir="${repo_dir}/.work/html-qa/document-smoke"
src_dir="${work_dir}/src"
out_dir="${work_dir}/out"
path_file="$(mktemp "${TMPDIR:-/tmp}/html-qa-document-path.XXXXXX")"
err_file="$(mktemp "${TMPDIR:-/tmp}/html-qa-document-err.XXXXXX")"
trap 'rm -f "$path_file" "$err_file"' EXIT

fail() {
  printf 'qa-document: %s\n' "$*" >&2
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

require_file_not_contains() {
  local label="$1"
  local file="$2"
  local needle="$3"

  if LC_ALL=C grep -Fq -- "$needle" "$file"; then
    fail "$label contains forbidden text: $needle"
  fi
}

run_doc() {
  local slug="$1"
  local source="$2"
  local output="$3"
  shift 3

  "$bin" --no-open "$@" --output "$output" "$source" >"$path_file"
  local printed
  printed="$(tr -d '\r\n' <"$path_file")"
  [[ "$printed" == "$output" ]] || fail "$slug printed $printed, want $output"
  [[ -f "$output" ]] || fail "$slug did not write output file: $output"
  require_file_contains "$slug" "$output" '<!DOCTYPE html>'
  require_file_contains "$slug" "$output" 'class="theme-controls"'
  require_file_contains "$slug" "$output" 'data-palette-choice="catppuccin"'
}

write_stdout_doc() {
  local slug="$1"
  local output="$2"
  shift 2

  "$bin" --stdout "$@" >"$output"
  require_file_contains "$slug" "$output" '<!DOCTYPE html>'
  require_file_contains "$slug" "$output" 'class="theme-controls"'
  require_file_contains "$slug" "$output" 'data-palette-choice="catppuccin"'
}

expect_error_contains() {
  local slug="$1"
  local needle="$2"
  shift 2

  local err_out="${out_dir}/${slug}.txt"
  if "$@" >"$path_file" 2>"$err_file"; then
    fail "$slug unexpectedly succeeded"
  fi
  require_file_contains "$slug" "$err_file" "$needle"
  cp "$err_file" "$err_out"
}

[[ -x "$bin" ]] || fail "local binary is missing or not executable: $bin; run just build first"

rm -rf "$work_dir"
mkdir -p "$src_dir/media-assets" "$out_dir"

printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\nIDATx\x9cc`\x00\x00\x00\x02\x00\x01\xe2!\xbc3\x00\x00\x00\x00IEND\xaeB`\x82' >"${src_dir}/media-assets/pixel.png"
cat >"${src_dir}/media-assets/badge.svg" <<'EOF_SVG'
<svg xmlns="http://www.w3.org/2000/svg" width="180" height="84" viewBox="0 0 180 84" role="img" aria-label="Document smoke SVG">
  <rect width="180" height="84" rx="10" fill="#fffefa"/>
  <circle cx="44" cy="42" r="22" fill="#2d7a46"/>
  <rect x="78" y="20" width="36" height="44" rx="7" fill="#2563eb"/>
  <path d="M142 18 164 66 120 66Z" fill="#be4b75"/>
</svg>
EOF_SVG

cat >"${src_dir}/document.md" <<'EOF_MARKDOWN'
# Document Mode

Normal Markdown output with tables, tasks, quotes, code, and local images.

| Piece | State |
|---|---|
| Table | Ready |
| Images | Inline |

- [x] Render Markdown
- [ ] Inspect browser capture

> Local images should survive as inlined data URIs.

```go
fmt.Println("document")
```

![Pixel](media-assets/pixel.png)

![Badge](media-assets/badge.svg)
EOF_MARKDOWN

cat >"${src_dir}/unsafe.md" <<'EOF_UNSAFE'
# Safe Mode

<script>alert(1)</script>

Some **bold** text still renders.
EOF_UNSAFE

cat >"${src_dir}/plain.txt" <<'EOF_PLAIN'
# not a heading
line two with <tag> & "quotes"
third line of plain prose
EOF_PLAIN

cat >"${src_dir}/weak-inline.txt" <<'EOF_WEAK'
This input has only inline Markdown cues: **bold**, `code`, and [docs](https://example.test/docs).
EOF_WEAK

cat >"${src_dir}/sample.go" <<'EOF_GO'
package main

import "fmt"

func main() {
	fmt.Println("document mode")
}
EOF_GO

cat >"${src_dir}/build.log" <<'EOF_LOG'
build started
compile package
ok
EOF_LOG

printf '\x89PNG\r\n\x1a\n\x00\xffhtml' >"${src_dir}/raw.bin"

run_doc markdown "${src_dir}/document.md" "${out_dir}/markdown.html"
require_file_contains markdown "${out_dir}/markdown.html" '<h1 id="document-mode">Document Mode</h1>'
require_file_contains markdown "${out_dir}/markdown.html" '<table>'
require_file_contains markdown "${out_dir}/markdown.html" 'type="checkbox"'
require_file_contains markdown "${out_dir}/markdown.html" '<blockquote>'
require_file_contains markdown "${out_dir}/markdown.html" 'class="chroma light"'
require_file_contains markdown "${out_dir}/markdown.html" 'data:image/png;base64,'
require_file_contains markdown "${out_dir}/markdown.html" 'data:image/svg+xml;base64,'

"$bin" --no-open --output - "${src_dir}/document.md" >"${out_dir}/output-dash.html"
require_file_contains output-dash "${out_dir}/output-dash.html" '<!DOCTYPE html>'
require_file_contains output-dash "${out_dir}/output-dash.html" '<h1 id="document-mode">Document Mode</h1>'
require_file_contains output-dash "${out_dir}/output-dash.html" 'data:image/svg+xml;base64,'
require_file_not_contains output-dash "${out_dir}/output-dash.html" '/cache/'

run_doc forced-markdown "${src_dir}/weak-inline.txt" "${out_dir}/forced-markdown.html" --markdown
require_file_contains forced-markdown "${out_dir}/forced-markdown.html" '<strong>bold</strong>'
require_file_contains forced-markdown "${out_dir}/forced-markdown.html" '<code>code</code>'
require_file_contains forced-markdown "${out_dir}/forced-markdown.html" '<a href="https://example.test/docs">docs</a>'

run_doc safe "${src_dir}/unsafe.md" "${out_dir}/safe.html" --safe
require_file_not_contains safe "${out_dir}/safe.html" 'alert(1)'
require_file_contains safe "${out_dir}/safe.html" '<strong>bold</strong>'

run_doc plain "${src_dir}/plain.txt" "${out_dir}/plain.html" --plain --lang text
require_file_contains plain "${out_dir}/plain.html" '<pre><code class="language-plaintext">'
require_file_contains plain "${out_dir}/plain.html" '# not a heading'
require_file_contains plain "${out_dir}/plain.html" '&lt;tag&gt; &amp; &#34;quotes&#34;'
require_file_not_contains plain "${out_dir}/plain.html" '<h1 id='

run_doc code "${src_dir}/sample.go" "${out_dir}/code.html"
require_file_contains code "${out_dir}/code.html" 'class="chroma light"'
require_file_contains code "${out_dir}/code.html" 'document mode'

printf 'package main\n\nfunc main() { println("stdin go") }\n' | write_stdout_doc stdout-plain-go "${out_dir}/stdout-plain-go.html" --plain --lang go --title "Go Stdin"
require_file_contains stdout-plain-go "${out_dir}/stdout-plain-go.html" '<title>Go Stdin</title>'
require_file_contains stdout-plain-go "${out_dir}/stdout-plain-go.html" 'class="chroma light"'
require_file_contains stdout-plain-go "${out_dir}/stdout-plain-go.html" 'stdin go'

printf 'normal\n\033[31mred\033[0m\n\033[1;32mbold green\033[0m\n' | write_stdout_doc ansi "${out_dir}/ansi.html" --title "ANSI Stdin"
require_file_contains ansi "${out_dir}/ansi.html" '<pre><code class="language-ansi">'
require_file_contains ansi "${out_dir}/ansi.html" 'style="color:#aa0000"'
require_file_contains ansi "${out_dir}/ansi.html" 'bold green'

run_doc frame "${src_dir}/build.log" "${out_dir}/frame.html" --frame
require_file_contains frame "${out_dir}/frame.html" 'class="term-frame"'
require_file_contains frame "${out_dir}/frame.html" 'class="term-title">build<'
require_file_contains frame "${out_dir}/frame.html" 'build started'

printf '# Stdin Markdown\n\n- [x] stdin task\n- [ ] browser check\n' | write_stdout_doc stdin-markdown "${out_dir}/stdin-markdown.html" --title "Stdin Markdown"
require_file_contains stdin-markdown "${out_dir}/stdin-markdown.html" 'type="checkbox"'
require_file_contains stdin-markdown "${out_dir}/stdin-markdown.html" '<h1 id="stdin-markdown">Stdin Markdown</h1>'
require_file_contains stdin-markdown "${out_dir}/stdin-markdown.html" '<title>Stdin Markdown</title>'

if "$bin" --no-open --output "${out_dir}/binary-doc.html" "${src_dir}/raw.bin" >"$path_file" 2>"$err_file"; then
  fail "binary document render unexpectedly succeeded"
fi
require_file_contains binary-refusal "$err_file" 'binary'
[[ ! -f "${out_dir}/binary-doc.html" ]] || fail "binary document render wrote an output file"
cp "$err_file" "${out_dir}/binary-refusal.txt"

expect_error_contains plain-markdown-conflict "--plain and --markdown are mutually exclusive" "$bin" --plain --markdown --no-open "${src_dir}/document.md"
expect_error_contains frame-markdown-conflict "--frame and --markdown are mutually exclusive" "$bin" --frame --markdown --no-open "${src_dir}/document.md"
expect_error_contains plain-report-conflict "--plain and report flags are mutually exclusive" "$bin" --plain --mode table --no-open "${src_dir}/document.md"
expect_error_contains markdown-report-conflict "--markdown and report flags are mutually exclusive" "$bin" --markdown --mode table --no-open "${src_dir}/document.md"
expect_error_contains frame-report-conflict "--frame and report flags are mutually exclusive" "$bin" --frame --mode table --no-open "${src_dir}/document.md"
expect_error_contains plan-output-conflict "--plan and --output are mutually exclusive" "$bin" --plan --planner off --output "${out_dir}/plan.html" "${src_dir}/document.md"
expect_error_contains stdout-output-conflict "--stdout and --output are mutually exclusive" "$bin" --stdout --output "${out_dir}/stdout.html" "${src_dir}/document.md"

cat >"${work_dir}/index.html" <<'EOF_INDEX'
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>HTML Document Smoke</title>
  <style>
    body { margin: 0; background: #f5f4f1; color: #25221f; font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    main { width: min(calc(100% - 2rem), 88rem); margin: 1.25rem auto 2rem; }
    h1 { margin: 0 0 .25rem; font-size: 1.55rem; line-height: 1.15; }
    p { margin: 0 0 1rem; color: #6d6760; line-height: 1.45; }
    .contract-panel { margin: 0 0 1rem; padding: .9rem; background: #fffefa; border: 1px solid #ded8cc; border-radius: 8px; box-shadow: 0 .9rem 2.4rem rgba(35, 30, 24, .07); }
    .contract-panel h2 { margin: 0 0 .35rem; font-size: 1rem; }
    .contract-panel p { margin: 0 0 .7rem; font-size: .84rem; }
    .contract-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr)); gap: .45rem; margin: 0; padding: 0; list-style: none; }
    .contract-list a { display: flex; min-height: 2.4rem; align-items: center; padding: .42rem .55rem; color: #25221f; background: #f1ece3; border: 1px solid #ded8cc; border-radius: 6px; font-size: .78rem; line-height: 1.25; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 28rem), 1fr)); gap: 1rem; }
    section { overflow: hidden; background: #fffefa; border: 1px solid #ded8cc; border-radius: 8px; box-shadow: 0 .9rem 2.4rem rgba(35, 30, 24, .07); }
    header { display: flex; justify-content: space-between; gap: .75rem; align-items: center; padding: .7rem .85rem; background: #f1ece3; border-bottom: 1px solid #ded8cc; }
    h2 { margin: 0; font-size: .92rem; }
    a { color: #8b5b2f; font-weight: 700; text-decoration: none; }
    .preview-note { min-height: 3.2rem; margin: 0; padding: .65rem .85rem; color: #6d6760; background: #fffefa; border-bottom: 1px solid #ded8cc; font-size: .82rem; line-height: 1.35; }
    iframe { display: block; width: 100%; height: 24rem; border: 0; background: white; }
    @media (max-width: 45rem) {
      main { width: 100%; margin: 0; }
      h1, p { padding-inline: 1rem; }
      h1 { padding-top: 1rem; }
      .grid { gap: .75rem; }
      section { border-left: 0; border-right: 0; border-radius: 0; }
      iframe { height: 22rem; }
    }
  </style>
</head>
<body>
  <main>
    <h1>HTML Document Smoke</h1>
    <p>Generated by scripts/qa-document.sh from the real ./html binary and ordinary document-mode flags.</p>
    <section class="contract-panel" aria-label="Document error contracts">
      <h2>Document Error Contracts</h2>
      <p>These links are captured from real failed CLI invocations; each file proves the public command rejected an invalid document-mode request before writing an output artifact.</p>
      <ul class="contract-list">
        <li><a data-error-contract href="out/binary-refusal.txt">Binary input refusal</a></li>
        <li><a data-error-contract href="out/plain-markdown-conflict.txt">--plain and --markdown conflict</a></li>
        <li><a data-error-contract href="out/frame-markdown-conflict.txt">--frame and --markdown conflict</a></li>
        <li><a data-error-contract href="out/plain-report-conflict.txt">--plain and report flags conflict</a></li>
        <li><a data-error-contract href="out/markdown-report-conflict.txt">--markdown and report flags conflict</a></li>
        <li><a data-error-contract href="out/frame-report-conflict.txt">--frame and report flags conflict</a></li>
        <li><a data-error-contract href="out/plan-output-conflict.txt">--plan and --output conflict</a></li>
        <li><a data-error-contract href="out/stdout-output-conflict.txt">--stdout and --output conflict</a></li>
      </ul>
    </section>
    <div class="grid">
EOF_INDEX

for file in markdown output-dash forced-markdown safe plain code stdout-plain-go ansi frame stdin-markdown; do
  case "$file" in
    markdown) note="Markdown document with table, tasks, quote, code, PNG, and SVG inlined as data URIs." ;;
    output-dash) note="The public -o - path writes the final document HTML to stdout without leaking a cache path." ;;
    forced-markdown) note="Forced Markdown renders weak inline cues that auto-detection intentionally leaves plain." ;;
    safe) note="Safe mode removes raw script HTML while preserving ordinary Markdown formatting." ;;
    plain) note="Forced plain text preserves literal headings and escapes HTML-like source text." ;;
    code) note="Source file output uses Chroma highlighting from the .go extension." ;;
    stdout-plain-go) note="Plain stdin with an explicit Go language uses Chroma highlighting and the supplied title." ;;
    ansi) note="Piped ANSI stdin keeps terminal color spans in a self-contained HTML document." ;;
    frame) note="Plain log output renders inside the terminal-frame component." ;;
    stdin-markdown) note="Stdin auto-detection recognizes GFM task-list Markdown and applies the provided title." ;;
  esac
  cat >>"${work_dir}/index.html" <<EOF_CARD
      <section><header><h2>${file}</h2><a href="out/${file}.html">Open</a></header><p class="preview-note">${note}</p><iframe title="${file}" src="out/${file}.html" loading="eager" onload="this.dataset.loaded='true'"></iframe></section>
EOF_CARD
done

cat >>"${work_dir}/index.html" <<'EOF_INDEX'
    </div>
  </main>
</body>
</html>
EOF_INDEX

printf 'qa-document: ok (%s)\n' "${work_dir}/index.html"
