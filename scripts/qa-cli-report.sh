#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${repo_dir}/html"
work_dir="${repo_dir}/.work/html-qa/cli-smoke"
src_dir="${work_dir}/src"
out_dir="${work_dir}/out"
path_file="$(mktemp "${TMPDIR:-/tmp}/html-qa-cli-path.XXXXXX")"
trap 'rm -f "$path_file"' EXIT

fail() {
  printf 'qa-cli-report: %s\n' "$*" >&2
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

run_case() {
  local slug="$1"
  local source="$2"
  local output="$3"
  shift 3

  "$bin" --no-open --planner off "$@" --output "$output" "$source" >"$path_file"
  local printed
  printed="$(tr -d '\r\n' <"$path_file")"
  [[ "$printed" == "$output" ]] || fail "$slug printed $printed, want $output"
  [[ -f "$output" ]] || fail "$slug did not write output file: $output"
  require_file_contains "$slug" "$output" '<!DOCTYPE html>'
  require_file_contains "$slug" "$output" 'class="theme-controls"'
  require_file_contains "$slug" "$output" 'data-palette-choice="catppuccin"'
}

[[ -x "$bin" ]] || fail "local binary is missing or not executable: $bin; run just build first"

rm -rf "$work_dir"
mkdir -p "$src_dir/media-assets" "$out_dir"

cat >"${src_dir}/article.md" <<'EOF_ARTICLE'
# Report

## Section

Body text for the article renderer.
EOF_ARTICLE

cat >"${src_dir}/records.csv" <<'EOF_CSV'
name,score,status
alpha,10,ready
beta,2,review
EOF_CSV

cat >"${src_dir}/change.patch" <<'EOF_DIFF'
diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1 +1 @@
-old
+new
EOF_DIFF

cat >"${src_dir}/tree.txt" <<'EOF_TREE'
.
├── cmd
│   └── html
└── internal
EOF_TREE

cat >"${src_dir}/run.log" <<'EOF_LOG'
2026-06-16 12:00:00 ERROR stop
2026-06-16 12:00:01 INFO ok
EOF_LOG

cat >"${src_dir}/deck.md" <<'EOF_DECK'
# Deck

Intro slide.

## First

The first section becomes a slide.

## Second

The second section becomes another slide.
EOF_DECK

printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\nIDATx\x9cc`\x00\x00\x00\x02\x00\x01\xe2!\xbc3\x00\x00\x00\x00IEND\xaeB`\x82' >"${src_dir}/media-assets/pixel.png"
printf '\x89PNG\r\n\x1a\n\x00\xffhtml' >"${src_dir}/logo.bin"
cat >"${src_dir}/media-assets/badge.svg" <<'EOF_SVG'
<svg xmlns="http://www.w3.org/2000/svg" width="120" height="56" viewBox="0 0 120 56" role="img" aria-label="CLI smoke SVG">
  <rect width="120" height="56" rx="8" fill="#fffefa"/>
  <circle cx="30" cy="28" r="14" fill="#2d7a46"/>
  <rect x="52" y="14" width="24" height="28" rx="5" fill="#2563eb"/>
  <path d="M96 13 110 43 82 43Z" fill="#be4b75"/>
</svg>
EOF_SVG
cat >"${src_dir}/media.md" <<'EOF_MEDIA'
# Media

![Pixel](media-assets/pixel.png)

![Badge](media-assets/badge.svg)
EOF_MEDIA

run_case article "${src_dir}/article.md" "${out_dir}/article.html" --mode article
require_file_contains article "${out_dir}/article.html" 'class="article-overview"'
require_file_contains article "${out_dir}/article.html" '<dt>Sections</dt><dd>1</dd>'

run_case cards "${src_dir}/records.csv" "${out_dir}/cards.html" --mode cards
require_file_contains cards "${out_dir}/cards.html" 'class="record-cards"'
require_file_contains cards "${out_dir}/cards.html" '<dt>Cards</dt><dd>2</dd>'
require_file_contains cards "${out_dir}/cards.html" '<dt>name</dt><dd>alpha</dd>'

run_case table "${src_dir}/records.csv" "${out_dir}/table.html" --mode table
require_file_contains table "${out_dir}/table.html" 'class="report-table"'
require_file_contains table "${out_dir}/table.html" 'class="report-filter"'
require_file_contains table "${out_dir}/table.html" 'class="report-mobile-sort"'
require_file_contains table "${out_dir}/table.html" 'data-report-table'

run_case tabs "${src_dir}/records.csv" "${out_dir}/tabs.html" --layout tabs
require_file_contains tabs "${out_dir}/tabs.html" 'class="report-tabs"'
require_file_contains tabs "${out_dir}/tabs.html" 'role="tablist"'

run_case code "${src_dir}/records.csv" "${out_dir}/code.html" --mode code
require_file_contains code "${out_dir}/code.html" 'class="code-overview"'
require_file_contains code "${out_dir}/code.html" '<dt>Language</dt><dd>CSV</dd>'
require_file_contains code "${out_dir}/code.html" 'class="chroma light"'

run_case diff "${src_dir}/change.patch" "${out_dir}/diff.html" --mode diff
require_file_contains diff "${out_dir}/diff.html" 'class="diff-view"'
require_file_contains diff "${out_dir}/diff.html" 'class="add">+new'

run_case tree "${src_dir}/tree.txt" "${out_dir}/tree.html" --mode tree
require_file_contains tree "${out_dir}/tree.html" 'class="file-tree"'
require_file_contains tree "${out_dir}/tree.html" '<dt>Entries</dt><dd>3</dd>'

run_case log "${src_dir}/run.log" "${out_dir}/log.html" --mode log
require_file_contains log "${out_dir}/log.html" 'class="log-lines"'
require_file_contains log "${out_dir}/log.html" 'class="log-line log-error"'

run_case slides "${src_dir}/deck.md" "${out_dir}/slides.html" --layout slides
require_file_contains slides "${out_dir}/slides.html" 'class="report-slides"'
require_file_contains slides "${out_dir}/slides.html" 'data-slide-next'

run_case media "${src_dir}/media.md" "${out_dir}/media.html" --mode article
require_file_contains media "${out_dir}/media.html" 'data:image/png;base64,'
require_file_contains media "${out_dir}/media.html" 'data:image/svg+xml;base64,'
require_file_contains media "${out_dir}/media.html" '<dt>Images</dt><dd>2</dd>'

run_case binary "${src_dir}/logo.bin" "${out_dir}/binary.html" --mode auto
require_file_contains binary "${out_dir}/binary.html" '<dt>Kind</dt><dd>binary</dd>'
require_file_contains binary "${out_dir}/binary.html" 'class="binary-preview"'
require_file_contains binary "${out_dir}/binary.html" '00000000  89 50 4e 47 0d 0a 1a 0a 00 ff 68 74 6d 6c'

stdout_html="$("$bin" --stdout --planner off --mode table "${src_dir}/records.csv")"
[[ "$stdout_html" == "<!DOCTYPE html>"* ]] || fail "--stdout did not print an HTML document"
[[ "$stdout_html" == *'class="report-table"'* ]] || fail "--stdout report output is missing report-table"
printf '%s' "$stdout_html" >"${out_dir}/stdout-table.html"
require_file_contains stdout-table "${out_dir}/stdout-table.html" 'class="theme-controls"'
require_file_contains stdout-table "${out_dir}/stdout-table.html" 'class="report-table"'
require_file_contains stdout-table "${out_dir}/stdout-table.html" 'class="report-filter"'

plan_json="$("$bin" --plan --planner off --layout slides "${src_dir}/deck.md")"
[[ "$plan_json" == *'"layout": "slides-page"'* ]] || fail "--plan did not report slides layout"
[[ "$plan_json" == *'"type": "article"'* ]] || fail "--plan did not include article component"
printf '%s\n' "$plan_json" >"${out_dir}/plan.json"
require_file_contains plan-json "${out_dir}/plan.json" '"planner": {'
require_file_contains plan-json "${out_dir}/plan.json" '"name": "deterministic"'
plan_json_escaped="$(sed 's/&/\&amp;/g; s/</\&lt;/g; s/>/\&gt;/g' "${out_dir}/plan.json")"

cat >"${work_dir}/index.html" <<EOF_INDEX
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>HTML CLI Report Smoke</title>
  <style>
    body { margin: 0; background: #f5f4f1; color: #25221f; font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    main { width: min(calc(100% - 2rem), 88rem); margin: 1.25rem auto 2rem; }
    h1 { margin: 0 0 .25rem; font-size: 1.55rem; line-height: 1.15; }
    p { margin: 0 0 1rem; color: #6d6760; }
    .contract-panel { margin: 0 0 1rem; background: #fffefa; border: 1px solid #ded8cc; border-radius: 8px; box-shadow: 0 .9rem 2.4rem rgba(35, 30, 24, .07); overflow: hidden; }
    .contract-panel header { display: flex; justify-content: space-between; gap: .75rem; align-items: center; padding: .7rem .85rem; background: #f1ece3; border-bottom: 1px solid #ded8cc; }
    .contract-panel p { margin: 0; padding: .7rem .85rem; color: #6d6760; font-size: .84rem; line-height: 1.4; border-bottom: 1px solid #ded8cc; }
    .contract-panel pre { margin: 0; padding: .85rem; overflow: auto; background: #25221f; color: #fffefa; font-size: .76rem; line-height: 1.45; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 28rem), 1fr)); gap: 1rem; }
    section { overflow: hidden; background: #fffefa; border: 1px solid #ded8cc; border-radius: 8px; box-shadow: 0 .9rem 2.4rem rgba(35, 30, 24, .07); }
    header { display: flex; justify-content: space-between; gap: .75rem; align-items: center; padding: .7rem .85rem; background: #f1ece3; border-bottom: 1px solid #ded8cc; }
    h2 { margin: 0; font-size: .92rem; }
    a { color: #8b5b2f; font-weight: 700; text-decoration: none; }
    iframe { display: block; width: 100%; height: 36rem; border: 0; background: white; }
    @media (max-width: 45rem) {
      main { width: 100%; margin: 0; }
      h1, p { padding-inline: 1rem; }
      h1 { padding-top: 1rem; }
      .grid { gap: .75rem; }
      section { border-left: 0; border-right: 0; border-radius: 0; }
      iframe { height: 32rem; }
    }
  </style>
</head>
<body>
  <main>
    <h1>HTML CLI Report Smoke</h1>
    <p>Generated by scripts/qa-cli-report.sh from the real ./html binary and public flags.</p>
    <section class="contract-panel" data-plan-contract data-plan-layout="slides-page" data-plan-component="article" aria-label="Report plan contract">
      <header><h2>Report Plan JSON</h2><a href="out/plan.json">Open</a></header>
      <p>Captured from <code>html --plan --planner off --layout slides</code>; proves plan output is JSON-only, deterministic, and reports the requested slides layout.</p>
      <pre><code>${plan_json_escaped}</code></pre>
    </section>
    <div class="grid">
EOF_INDEX

for file in article cards table tabs code diff tree log slides media binary stdout-table; do
  cat >>"${work_dir}/index.html" <<EOF_CARD
      <section><header><h2>${file}</h2><a href="out/${file}.html">Open</a></header><iframe title="${file}" src="out/${file}.html" loading="eager" onload="this.dataset.loaded='true'"></iframe></section>
EOF_CARD
done

cat >>"${work_dir}/index.html" <<'EOF_INDEX'
    </div>
  </main>
</body>
</html>
EOF_INDEX

printf 'qa-cli-report: ok (%s)\n' "${work_dir}/index.html"
