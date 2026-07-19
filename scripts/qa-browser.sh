#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
capture_dir="${repo_dir}/tools/chromedp-capture"
out_dir="${repo_dir}/.work/html-qa/browser"
# BSD mktemp requires the X run at the end of the template. Keeping a suffix
# after it creates a literal reusable filename and makes the second QA run fail.
metric_file="$(mktemp "${TMPDIR:-/tmp}/html-qa-browser.XXXXXX")"
trap 'rm -f "$metric_file"' EXIT

fail() {
  printf 'qa-browser: %s\n' "$*" >&2
  exit 1
}

capture() {
  local name="$1"
  local url="$2"
  local width="$3"
  local height="$4"
  shift 4

  local png="${out_dir}/${name}-${width}x${height}.png"
  (cd "$capture_dir" && go run . --url "$url" --out "$png" --width "$width" --height "$height" "$@") >"$metric_file"
  local metrics
  metrics="$(cat "$metric_file")"
  printf '%s\n' "$metrics" >"${png%.png}.json"
  require_metric "$name" "$metrics" '.console_errors == 0'
  require_metric "$name" "$metrics" '.scroll_width <= .client_width'
  [[ -s "$png" ]] || fail "$name screenshot is empty: $png"
}

capture_design_contract_matrix() {
  local widths=(320 375 414 768)
  local cases=(
    "markdown-components|${matrix_markdown_components_url}"
    "table-tabs|${matrix_csv_tabs_url}"
    "slides|${matrix_markdown_slides_url}"
    "media|${media_url}"
  )
  local case_entry name url width metrics_file state_expr
  local capture_args=()

  for case_entry in "${cases[@]}"; do
    name="${case_entry%%|*}"
    url="${case_entry#*|}"

    case "$name" in
      markdown-components)
        capture_args=(--click-copy --focus-selector .copy-btn)
        state_expr='.copy_status == "Copied" and .focus_visible == true and .focus_selector == ".copy-btn"'
        ;;
      table-tabs)
        capture_args=(--click-tab Records --focus-selector '#report-tab-1')
        state_expr='.selected_tab == "Records" and (.visible_tab_panel | contains("Records")) and .focus_visible == true and .focus_selector == "#report-tab-1"'
        ;;
      slides)
        capture_args=(--click-slide-next --focus-selector '[data-slide-next]')
        state_expr='.initial_slide_prev_disabled == true and .initial_slide_prev_style == "opacity=0.55;cursor=not-allowed" and .slide_status == "2 / 3" and (.current_slide | contains("Slide 2 of 3")) and .focus_visible == true and .focus_selector == "[data-slide-next]"'
        ;;
      media)
        capture_args=(--focus-selector '#theme-toggle')
        state_expr='.focus_visible == true and .focus_selector == "#theme-toggle"'
        ;;
    esac
    for width in "${widths[@]}"; do
      capture "design-contract-${name}" "$url" "$width" 900 "${capture_args[@]}"
      metrics_file="${out_dir}/design-contract-${name}-${width}x900.json"
      require_json_file "design-contract-${name}-${width}" "$metrics_file" \
        '.scroll_width <= .client_width and .body_width <= .client_width and .clickables_checked > 0 and (.wrapped_clickables | length) == 0 and .contrast_candidates > 0 and .contrast_checked == .contrast_candidates and (.contrast_skips | length) == 0 and (.contrast_failures | length) == 0'
      require_json_file "design-contract-${name}-${width}-state" "$metrics_file" "$state_expr"
    done
  done
}

require_metric() {
  local name="$1"
  local metrics="$2"
  local expr="$3"

  if ! jq -e "$expr" <<<"$metrics" >/dev/null; then
    fail "$name metric failed: $expr; metrics=$metrics"
  fi
}

require_json_file() {
  local name="$1"
  local file="$2"
  local expr="$3"

  if ! jq -e "$expr" "$file" >/dev/null; then
    fail "$name metric failed: $expr; file=$file"
  fi
}

count_html() {
  find "$1" -type f -name '*.html' | wc -l | tr -d ' '
}

count_detection_kind() {
  local kind="$1"
  LC_ALL=C tr '<' '\n' <"${repo_dir}/.work/html-qa/detection/index.html" | LC_ALL=C grep -c "^tr data-kind=\"${kind}\"" | tr -d ' '
}

rm -rf "${repo_dir}/.work/html-qa"
mkdir -p "$out_dir"
(cd "$repo_dir" && just qa-cli >/dev/null)
(cd "$repo_dir" && just qa-document >/dev/null)
(cd "$repo_dir" && just qa-config >/dev/null)
(cd "$repo_dir" && just qa-cache >/dev/null)
(cd "$repo_dir" && just qa-detection >/dev/null)
(cd "$repo_dir" && just qa-theme-gallery >/dev/null)
(cd "$repo_dir" && just qa-kind-matrix >/dev/null)
(cd "$repo_dir" && just qa-dashboard >/dev/null)

expected_roots=$'browser\ncache-smoke\ncli-smoke\nconfig-smoke\ndetection\ndocument-smoke\nindex.html\nkind-matrix\ntheme-gallery'
actual_roots="$(find "${repo_dir}/.work/html-qa" -mindepth 1 -maxdepth 1 -exec basename {} \; | LC_ALL=C sort)"
if [[ "$actual_roots" != "$expected_roots" ]]; then
  fail "unexpected top-level QA artifacts; got: ${actual_roots//$'\n'/, }"
fi

matrix_direct_capture_slugs=(
  markdown
  media
  markdown-components
  markdown-slides
  markdown-article-override
  markdown-csv-precedence
  markdown-json-precedence
  markdown-unknown-structure
  markdown-unknown-task-list
  json-records
  bom-json-records
  json-record-cards
  ndjson-records
  single-jsonl-record
  single-jsonlines-record
  jsonl-record-cards
  json-object
  json-scalar-array
  json-scalar-file
  empty-json-array
  json-empty-object-array
  bad-json-source-code
  bad-jsonl-source-code
  csv-records
  bom-csv-records
  bad-csv-source-code
  csv-header-only
  timestamped-csv-records
  csv-tabs
  csv-cards-override
  csv-code-override
  tsv-records
  table-records
  psql-table-records
  diff
  plain-diff-headers
  plain-diff-multi-file
  combined-diff
  git-binary-patch
  git-mode-only-patch
  git-copy-only-patch
  diff-override
  source-code
  yaml-source-code
  shell-content-source
  go-source-precedence
  go-source-fence-string
  go-source-csv-precedence
  tree-listing
  ascii-tree-listing
  tree-summary-listing
  posix-path-listing
  absolute-path-listing
  spaced-path-listing
  windows-path-listing
  url-list-plain
  fractions-plain
  http-request-paths-plain
  ordinary-ok-plain
  config-keys-plain
  dash-divider-plain
  tree-override
  log
  go-test-log
  single-severity-log
  single-go-test-log
  plain-log-override
  transcript
  generic-speaker-transcript
  mixed
  mixed-single-override
  single-comma-plain
  plain
  yaml-plain
  binary
)
expected_matrix_slugs="$(printf '%s\n' "${matrix_direct_capture_slugs[@]}" | LC_ALL=C sort)"
actual_matrix_slugs="$(find "${repo_dir}/.work/html-qa/kind-matrix" -maxdepth 1 -type f -name '*.html' ! -name index.html -exec basename {} .html \; | LC_ALL=C sort)"
if [[ "$actual_matrix_slugs" != "$expected_matrix_slugs" ]]; then
  diff_output="$(diff -u <(printf '%s\n' "$expected_matrix_slugs") <(printf '%s\n' "$actual_matrix_slugs") || true)"
  fail "kind matrix direct browser coverage is stale; update matrix_direct_capture_slugs and direct captures: ${diff_output//$'\n'/; }"
fi

theme_case_slugs=(
  light-sepia
  light-blue
  light-green
  light-rose
  light-catppuccin
  dark-sepia
  dark-blue
  dark-green
  dark-rose
  dark-catppuccin
)
expected_theme_case_slugs="$(printf '%s\n' "${theme_case_slugs[@]}" | LC_ALL=C sort)"
actual_theme_case_slugs="$(find "${repo_dir}/.work/html-qa/theme-gallery" -maxdepth 1 -type f -name '*.html' ! -name index.html -exec basename {} .html \; | LC_ALL=C sort)"
if [[ "$actual_theme_case_slugs" != "$expected_theme_case_slugs" ]]; then
  diff_output="$(diff -u <(printf '%s\n' "$expected_theme_case_slugs") <(printf '%s\n' "$actual_theme_case_slugs") || true)"
  fail "theme case browser coverage is stale; update theme_case_slugs, theme_accents, and direct captures: ${diff_output//$'\n'/; }"
fi

theme_component_direct_capture_slugs=(
  article-media
  table
  cards
  tabs
  slides
  json
  diff
  tree
  log
  plain-code
)
expected_theme_component_slugs="$(printf '%s\n' "${theme_component_direct_capture_slugs[@]}" | LC_ALL=C sort)"
actual_theme_component_slugs="$(find "${repo_dir}/.work/html-qa/theme-gallery/pages/light-sepia" -maxdepth 1 -type f -name '*.html' -exec basename {} .html \; | LC_ALL=C sort)"
if [[ "$actual_theme_component_slugs" != "$expected_theme_component_slugs" ]]; then
  diff_output="$(diff -u <(printf '%s\n' "$expected_theme_component_slugs") <(printf '%s\n' "$actual_theme_component_slugs") || true)"
  fail "theme component direct browser coverage is stale; update theme_component_direct_capture_slugs and direct captures: ${diff_output//$'\n'/; }"
fi

cli_direct_capture_slugs=(
  article
  cards
  table
  tabs
  code
  diff
  tree
  log
  slides
  media
  binary
  stdout-table
)
expected_cli_slugs="$(printf '%s\n' "${cli_direct_capture_slugs[@]}" | LC_ALL=C sort)"
actual_cli_slugs="$(find "${repo_dir}/.work/html-qa/cli-smoke/out" -maxdepth 1 -type f -name '*.html' -exec basename {} .html \; | LC_ALL=C sort)"
if [[ "$actual_cli_slugs" != "$expected_cli_slugs" ]]; then
  diff_output="$(diff -u <(printf '%s\n' "$expected_cli_slugs") <(printf '%s\n' "$actual_cli_slugs") || true)"
  fail "CLI report direct browser coverage is stale; update cli_direct_capture_slugs and direct captures: ${diff_output//$'\n'/; }"
fi

document_direct_capture_slugs=(
  markdown
  output-dash
  forced-markdown
  safe
  plain
  plain-csv-table
  plain-column-table
  plain-skill-leaderboards
  code
  stdout-plain-table
  stdout-plain-go
  ansi
  frame
  stdin-markdown
)
expected_document_slugs="$(printf '%s\n' "${document_direct_capture_slugs[@]}" | LC_ALL=C sort)"
actual_document_slugs="$(find "${repo_dir}/.work/html-qa/document-smoke/out" -maxdepth 1 -type f -name '*.html' -exec basename {} .html \; | LC_ALL=C sort)"
if [[ "$actual_document_slugs" != "$expected_document_slugs" ]]; then
  diff_output="$(diff -u <(printf '%s\n' "$expected_document_slugs") <(printf '%s\n' "$actual_document_slugs") || true)"
  fail "document direct browser coverage is stale; update document_direct_capture_slugs and direct captures: ${diff_output//$'\n'/; }"
fi

expected_config_artifacts=$'index.html\nout/configured.html\nout/invalid-config.txt'
actual_config_artifacts="$(cd "${repo_dir}/.work/html-qa/config-smoke" && find . -type f \( -name '*.html' -o -name '*.txt' \) ! -path './src/*' -print | sed 's#^\./##' | LC_ALL=C sort)"
if [[ "$actual_config_artifacts" != "$expected_config_artifacts" ]]; then
  diff_output="$(diff -u <(printf '%s\n' "$expected_config_artifacts") <(printf '%s\n' "$actual_config_artifacts") || true)"
  fail "config smoke artifacts changed; update config captures and dashboard expectations: ${diff_output//$'\n'/; }"
fi

cache_html_count="$(find "${repo_dir}/.work/html-qa/cache-smoke/cache" -maxdepth 1 -type f -name '*.html' | wc -l | tr -d ' ')"
cache_fingerprint_count="$(find "${repo_dir}/.work/html-qa/cache-smoke/cache" -maxdepth 1 -type f -name '*.fp' | wc -l | tr -d ' ')"
[[ "$cache_html_count" == "2" ]] || fail "cache smoke expected 2 cached HTML pages, got $cache_html_count"
[[ "$cache_fingerprint_count" == "2" ]] || fail "cache smoke expected 2 cache fingerprints, got $cache_fingerprint_count"
[[ -f "${repo_dir}/.work/html-qa/cache-smoke/index.html" ]] || fail "cache smoke index.html was not generated"

actual_detection_artifacts="$(cd "${repo_dir}/.work/html-qa/detection" && find . -type f -print | sed 's#^\./##' | LC_ALL=C sort)"
if [[ "$actual_detection_artifacts" != "index.html" ]]; then
  fail "detection smoke artifacts changed; got: ${actual_detection_artifacts//$'\n'/, }"
fi

detection_rows_expected="$(LC_ALL=C tr '<' '\n' <"${repo_dir}/.work/html-qa/detection/index.html" | LC_ALL=C grep -c '^tr data-kind="' | tr -d ' ')"
detection_markdown_expected="$(count_detection_kind markdown)"
detection_plain_expected="$(count_detection_kind plain)"
detection_binary_expected="$(count_detection_kind binary)"
[[ "$detection_rows_expected" -gt 0 ]] || fail "detection matrix has no rows"
[[ $((detection_markdown_expected + detection_plain_expected + detection_binary_expected)) -eq "$detection_rows_expected" ]] || fail "detection kind counts do not sum to rows"

dashboard_kind_matrix_count="$(count_html "${repo_dir}/.work/html-qa/kind-matrix")"
dashboard_cli_count="$(count_html "${repo_dir}/.work/html-qa/cli-smoke")"
dashboard_document_count="$(count_html "${repo_dir}/.work/html-qa/document-smoke")"
dashboard_config_count="$(count_html "${repo_dir}/.work/html-qa/config-smoke")"
dashboard_cache_count="$(count_html "${repo_dir}/.work/html-qa/cache-smoke")"
dashboard_detection_count="$(count_html "${repo_dir}/.work/html-qa/detection")"
dashboard_theme_count="$(count_html "${repo_dir}/.work/html-qa/theme-gallery")"
dashboard_suite_count=$((dashboard_kind_matrix_count + dashboard_cli_count + dashboard_document_count + dashboard_config_count + dashboard_cache_count + dashboard_detection_count + dashboard_theme_count))
dashboard_expr='.title == "HTML QA Dashboard" and .iframe_count == 7 and .iframe_loaded_frames == 7 and .dashboard_cards == 7 and .dashboard_suite_html == '"$dashboard_suite_count"' and .dashboard_card_counts["Theme Gallery"] == '"$dashboard_theme_count"' and .dashboard_card_counts["Report Kind Matrix"] == '"$dashboard_kind_matrix_count"' and .dashboard_card_counts["CLI Report Smoke"] == '"$dashboard_cli_count"' and .dashboard_card_counts["Document Smoke"] == '"$dashboard_document_count"' and .dashboard_card_counts["Config Smoke"] == '"$dashboard_config_count"' and .dashboard_card_counts["Cache Smoke"] == '"$dashboard_cache_count"' and .dashboard_card_counts["Detection Matrix"] == '"$dashboard_detection_count"' and (.dashboard_detection_proof | contains("'"$detection_rows_expected"' detector cases")) and (.dashboard_detection_proof | contains("'"$detection_binary_expected"' binary, '"$detection_plain_expected"' plain, '"$detection_markdown_expected"' Markdown"))'
detection_expr='.title == "HTML Detection Matrix" and .detection_rows == '"$detection_rows_expected"' and .detection_kind_counts.markdown == '"$detection_markdown_expected"' and .detection_kind_counts.plain == '"$detection_plain_expected"' and .detection_kind_counts.binary == '"$detection_binary_expected"
theme_gallery_expr='.title == "HTML Theme Gallery" and .iframe_count == 10 and .iframe_loaded_frames == 10 and (.iframe_theme_palettes | sort) == ["dark/blue","dark/catppuccin","dark/green","dark/rose","dark/sepia","light/blue","light/catppuccin","light/green","light/rose","light/sepia"]'

dashboard_url="file://${repo_dir}/.work/html-qa/index.html"
matrix_url="file://${repo_dir}/.work/html-qa/kind-matrix/index.html"
cli_url="file://${repo_dir}/.work/html-qa/cli-smoke/index.html"
document_url="file://${repo_dir}/.work/html-qa/document-smoke/index.html"
config_url="file://${repo_dir}/.work/html-qa/config-smoke/index.html"
configured_url="file://${repo_dir}/.work/html-qa/config-smoke/out/configured.html"
cache_url="file://${repo_dir}/.work/html-qa/cache-smoke/index.html"
cache_file_path=""
for candidate in "${repo_dir}"/.work/html-qa/cache-smoke/cache/*.html; do
  if LC_ALL=C grep -Fq '<h1 id="cache-smoke">Cache Smoke</h1>' "$candidate"; then
    cache_file_path="$candidate"
    break
  fi
done
[[ -n "$cache_file_path" ]] || fail "cache-smoke file cache HTML was not generated"
cache_file_url="file://${cache_file_path}"
detection_url="file://${repo_dir}/.work/html-qa/detection/index.html"
theme_url="file://${repo_dir}/.work/html-qa/theme-gallery/index.html"
media_url="file://${repo_dir}/.work/html-qa/kind-matrix/media.html"
matrix_markdown_url="file://${repo_dir}/.work/html-qa/kind-matrix/markdown.html"
matrix_markdown_components_url="file://${repo_dir}/.work/html-qa/kind-matrix/markdown-components.html"
matrix_markdown_article_override_url="file://${repo_dir}/.work/html-qa/kind-matrix/markdown-article-override.html"
matrix_markdown_csv_precedence_url="file://${repo_dir}/.work/html-qa/kind-matrix/markdown-csv-precedence.html"
matrix_markdown_json_precedence_url="file://${repo_dir}/.work/html-qa/kind-matrix/markdown-json-precedence.html"
matrix_markdown_unknown_structure_url="file://${repo_dir}/.work/html-qa/kind-matrix/markdown-unknown-structure.html"
matrix_markdown_unknown_task_list_url="file://${repo_dir}/.work/html-qa/kind-matrix/markdown-unknown-task-list.html"
matrix_json_records_url="file://${repo_dir}/.work/html-qa/kind-matrix/json-records.html"
matrix_json_record_cards_url="file://${repo_dir}/.work/html-qa/kind-matrix/json-record-cards.html"
matrix_ndjson_records_url="file://${repo_dir}/.work/html-qa/kind-matrix/ndjson-records.html"
matrix_single_jsonl_record_url="file://${repo_dir}/.work/html-qa/kind-matrix/single-jsonl-record.html"
matrix_single_jsonlines_record_url="file://${repo_dir}/.work/html-qa/kind-matrix/single-jsonlines-record.html"
matrix_jsonl_record_cards_url="file://${repo_dir}/.work/html-qa/kind-matrix/jsonl-record-cards.html"
matrix_csv_records_url="file://${repo_dir}/.work/html-qa/kind-matrix/csv-records.html"
matrix_bom_csv_records_url="file://${repo_dir}/.work/html-qa/kind-matrix/bom-csv-records.html"
matrix_bad_csv_source_code_url="file://${repo_dir}/.work/html-qa/kind-matrix/bad-csv-source-code.html"
matrix_transcript_url="file://${repo_dir}/.work/html-qa/kind-matrix/transcript.html"
matrix_mixed_url="file://${repo_dir}/.work/html-qa/kind-matrix/mixed.html"
matrix_plain_url="file://${repo_dir}/.work/html-qa/kind-matrix/plain.html"
matrix_plain_log_url="file://${repo_dir}/.work/html-qa/kind-matrix/plain-log-override.html"
matrix_bom_json_url="file://${repo_dir}/.work/html-qa/kind-matrix/bom-json-records.html"
matrix_json_object_url="file://${repo_dir}/.work/html-qa/kind-matrix/json-object.html"
matrix_json_scalar_array_url="file://${repo_dir}/.work/html-qa/kind-matrix/json-scalar-array.html"
matrix_json_scalar_file_url="file://${repo_dir}/.work/html-qa/kind-matrix/json-scalar-file.html"
matrix_empty_json_array_url="file://${repo_dir}/.work/html-qa/kind-matrix/empty-json-array.html"
matrix_json_empty_object_array_url="file://${repo_dir}/.work/html-qa/kind-matrix/json-empty-object-array.html"
matrix_bad_json_source_code_url="file://${repo_dir}/.work/html-qa/kind-matrix/bad-json-source-code.html"
matrix_bad_jsonl_source_code_url="file://${repo_dir}/.work/html-qa/kind-matrix/bad-jsonl-source-code.html"
matrix_csv_header_only_url="file://${repo_dir}/.work/html-qa/kind-matrix/csv-header-only.html"
matrix_timestamped_csv_records_url="file://${repo_dir}/.work/html-qa/kind-matrix/timestamped-csv-records.html"
matrix_csv_tabs_url="file://${repo_dir}/.work/html-qa/kind-matrix/csv-tabs.html"
matrix_csv_cards_override_url="file://${repo_dir}/.work/html-qa/kind-matrix/csv-cards-override.html"
matrix_csv_code_override_url="file://${repo_dir}/.work/html-qa/kind-matrix/csv-code-override.html"
matrix_tsv_records_url="file://${repo_dir}/.work/html-qa/kind-matrix/tsv-records.html"
matrix_table_records_url="file://${repo_dir}/.work/html-qa/kind-matrix/table-records.html"
matrix_psql_table_url="file://${repo_dir}/.work/html-qa/kind-matrix/psql-table-records.html"
matrix_diff_url="file://${repo_dir}/.work/html-qa/kind-matrix/diff.html"
matrix_plain_diff_headers_url="file://${repo_dir}/.work/html-qa/kind-matrix/plain-diff-headers.html"
matrix_plain_diff_multi_file_url="file://${repo_dir}/.work/html-qa/kind-matrix/plain-diff-multi-file.html"
matrix_combined_diff_url="file://${repo_dir}/.work/html-qa/kind-matrix/combined-diff.html"
matrix_git_binary_patch_url="file://${repo_dir}/.work/html-qa/kind-matrix/git-binary-patch.html"
matrix_git_mode_only_patch_url="file://${repo_dir}/.work/html-qa/kind-matrix/git-mode-only-patch.html"
matrix_git_copy_only_patch_url="file://${repo_dir}/.work/html-qa/kind-matrix/git-copy-only-patch.html"
matrix_diff_override_url="file://${repo_dir}/.work/html-qa/kind-matrix/diff-override.html"
matrix_source_code_url="file://${repo_dir}/.work/html-qa/kind-matrix/source-code.html"
matrix_yaml_source_code_url="file://${repo_dir}/.work/html-qa/kind-matrix/yaml-source-code.html"
matrix_shell_content_source_url="file://${repo_dir}/.work/html-qa/kind-matrix/shell-content-source.html"
matrix_go_source_precedence_url="file://${repo_dir}/.work/html-qa/kind-matrix/go-source-precedence.html"
matrix_go_source_fence_string_url="file://${repo_dir}/.work/html-qa/kind-matrix/go-source-fence-string.html"
matrix_go_source_csv_precedence_url="file://${repo_dir}/.work/html-qa/kind-matrix/go-source-csv-precedence.html"
matrix_tree_listing_url="file://${repo_dir}/.work/html-qa/kind-matrix/tree-listing.html"
matrix_ascii_tree_listing_url="file://${repo_dir}/.work/html-qa/kind-matrix/ascii-tree-listing.html"
matrix_tree_summary_listing_url="file://${repo_dir}/.work/html-qa/kind-matrix/tree-summary-listing.html"
matrix_posix_path_listing_url="file://${repo_dir}/.work/html-qa/kind-matrix/posix-path-listing.html"
matrix_absolute_path_listing_url="file://${repo_dir}/.work/html-qa/kind-matrix/absolute-path-listing.html"
matrix_spaced_path_listing_url="file://${repo_dir}/.work/html-qa/kind-matrix/spaced-path-listing.html"
matrix_windows_path_url="file://${repo_dir}/.work/html-qa/kind-matrix/windows-path-listing.html"
matrix_tree_override_url="file://${repo_dir}/.work/html-qa/kind-matrix/tree-override.html"
matrix_url_list_plain_url="file://${repo_dir}/.work/html-qa/kind-matrix/url-list-plain.html"
matrix_fractions_plain_url="file://${repo_dir}/.work/html-qa/kind-matrix/fractions-plain.html"
matrix_http_request_paths_plain_url="file://${repo_dir}/.work/html-qa/kind-matrix/http-request-paths-plain.html"
matrix_ordinary_ok_plain_url="file://${repo_dir}/.work/html-qa/kind-matrix/ordinary-ok-plain.html"
matrix_config_keys_plain_url="file://${repo_dir}/.work/html-qa/kind-matrix/config-keys-plain.html"
matrix_dash_divider_plain_url="file://${repo_dir}/.work/html-qa/kind-matrix/dash-divider-plain.html"
matrix_log_url="file://${repo_dir}/.work/html-qa/kind-matrix/log.html"
matrix_go_test_log_url="file://${repo_dir}/.work/html-qa/kind-matrix/go-test-log.html"
matrix_single_severity_log_url="file://${repo_dir}/.work/html-qa/kind-matrix/single-severity-log.html"
matrix_single_go_test_log_url="file://${repo_dir}/.work/html-qa/kind-matrix/single-go-test-log.html"
matrix_generic_speaker_transcript_url="file://${repo_dir}/.work/html-qa/kind-matrix/generic-speaker-transcript.html"
matrix_markdown_slides_url="file://${repo_dir}/.work/html-qa/kind-matrix/markdown-slides.html"
matrix_mixed_single_override_url="file://${repo_dir}/.work/html-qa/kind-matrix/mixed-single-override.html"
matrix_single_comma_plain_url="file://${repo_dir}/.work/html-qa/kind-matrix/single-comma-plain.html"
matrix_yaml_plain_url="file://${repo_dir}/.work/html-qa/kind-matrix/yaml-plain.html"
matrix_binary_url="file://${repo_dir}/.work/html-qa/kind-matrix/binary.html"
document_markdown_url="file://${repo_dir}/.work/html-qa/document-smoke/out/markdown.html"
document_output_dash_url="file://${repo_dir}/.work/html-qa/document-smoke/out/output-dash.html"
document_forced_markdown_url="file://${repo_dir}/.work/html-qa/document-smoke/out/forced-markdown.html"
document_safe_url="file://${repo_dir}/.work/html-qa/document-smoke/out/safe.html"
document_plain_url="file://${repo_dir}/.work/html-qa/document-smoke/out/plain.html"
document_plain_csv_table_url="file://${repo_dir}/.work/html-qa/document-smoke/out/plain-csv-table.html"
document_plain_column_table_url="file://${repo_dir}/.work/html-qa/document-smoke/out/plain-column-table.html"
document_plain_skill_leaderboards_url="file://${repo_dir}/.work/html-qa/document-smoke/out/plain-skill-leaderboards.html"
document_code_url="file://${repo_dir}/.work/html-qa/document-smoke/out/code.html"
document_stdout_plain_table_url="file://${repo_dir}/.work/html-qa/document-smoke/out/stdout-plain-table.html"
document_stdout_go_url="file://${repo_dir}/.work/html-qa/document-smoke/out/stdout-plain-go.html"
document_frame_url="file://${repo_dir}/.work/html-qa/document-smoke/out/frame.html"
document_ansi_url="file://${repo_dir}/.work/html-qa/document-smoke/out/ansi.html"
document_stdin_markdown_url="file://${repo_dir}/.work/html-qa/document-smoke/out/stdin-markdown.html"
article_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/article.html"
cards_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/cards.html"
code_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/code.html"
diff_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/diff.html"
tree_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/tree.html"
log_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/log.html"
cli_media_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/media.html"
binary_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/binary.html"
slides_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/slides.html"
tabs_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/tabs.html"
table_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/table.html"
stdout_table_url="file://${repo_dir}/.work/html-qa/cli-smoke/out/stdout-table.html"

capture_design_contract_matrix

capture dashboard "$dashboard_url" 1440 900
require_json_file dashboard "${out_dir}/dashboard-1440x900.json" "$dashboard_expr"
capture dashboard "$dashboard_url" 390 900
require_json_file dashboard "${out_dir}/dashboard-390x900.json" "$dashboard_expr"

capture matrix "$matrix_url" 1440 900
require_json_file matrix "${out_dir}/matrix-1440x900.json" '.iframe_count == 76 and .iframe_loaded_frames == 76 and .coverage_missing == 0 and .image_count >= 8 and .loaded_images >= 8 and .data_uri_images >= 4 and .svg_images >= 4 and .raster_images >= 4 and .media_preview_images >= 8 and .media_preview_source_images >= 4 and .media_preview_rendered_images >= 4'
capture matrix "$matrix_url" 390 900
require_json_file matrix "${out_dir}/matrix-390x900.json" '.iframe_count == 76 and .iframe_loaded_frames == 76 and .coverage_missing == 0 and .image_count >= 8 and .loaded_images >= 8 and .data_uri_images >= 4 and .svg_images >= 4 and .raster_images >= 4 and .media_preview_images >= 8 and .media_preview_source_images >= 4 and .media_preview_rendered_images >= 4'

capture cli "$cli_url" 1440 900
require_json_file cli "${out_dir}/cli-1440x900.json" '.iframe_count == 12 and .iframe_loaded_frames == 12 and .plan_contracts == 1 and .plan_contract_layout == "slides-page" and .plan_contract_component == "article"'
capture cli "$cli_url" 390 900
require_json_file cli "${out_dir}/cli-390x900.json" '.iframe_count == 12 and .iframe_loaded_frames == 12 and .plan_contracts == 1 and .plan_contract_layout == "slides-page" and .plan_contract_component == "article"'

capture cli-article "$article_url" 390 900
require_json_file cli-article "${out_dir}/cli-article-390x900.json" '.title == "Report" and .theme_controls_one_row == true and .component_counts.article_overview == 1 and .component_counts.markdown_body == 1 and .heading_anchors >= 2'

capture cli-cards "$cards_url" 390 900
require_json_file cli-cards "${out_dir}/cli-cards-390x900.json" '.title == "records" and .theme_controls_one_row == true and .component_counts.record_cards == 1 and .record_card_items == 2 and .component_counts.markdown_body == 1'

capture cli-code "$code_url" 390 900
require_json_file cli-code "${out_dir}/cli-code-390x900.json" '.title == "records" and .theme_controls_one_row == true and .component_counts.code_overview == 1 and .code_overview_items == 4 and .component_counts.chroma == 1 and .chroma_line_items == 6 and .copy_buttons >= 1'

capture cli-diff "$diff_url" 390 900
require_json_file cli-diff "${out_dir}/cli-diff-390x900.json" '.title == "change" and .theme_controls_one_row == true and .component_counts.diff_view == 1 and .component_counts.markdown_body == 1'

capture cli-tree "$tree_url" 390 900
require_json_file cli-tree "${out_dir}/cli-tree-390x900.json" '.title == "tree" and .theme_controls_one_row == true and .component_counts.file_tree == 1 and .component_counts.markdown_body == 1'

capture cli-log "$log_url" 390 900
require_json_file cli-log "${out_dir}/cli-log-390x900.json" '.title == "run" and .theme_controls_one_row == true and .component_counts.log_lines == 1'

capture cli-media "$cli_media_url" 390 900
require_json_file cli-media "${out_dir}/cli-media-390x900.json" '.title == "Media" and .theme_controls_one_row == true and .component_counts.article_overview == 1 and .component_counts.markdown_body == 1 and .heading_anchors >= 1 and .image_count == 2 and .loaded_images == 2 and .data_uri_images == 2 and .svg_images == 1 and .raster_images == 1'

capture cli-stdout-table "$stdout_table_url" 390 900
require_json_file cli-stdout-table "${out_dir}/cli-stdout-table-390x900.json" '.title == "records" and .theme_controls_one_row == true and .component_counts.report_table == 1 and .filter_status == "2 rows" and .report_data_rows == 2 and .visible_report_rows == 2'

capture document "$document_url" 1440 900
require_json_file document "${out_dir}/document-1440x900.json" '.iframe_count == 14 and .iframe_loaded_frames == 14 and .error_contracts == 8'
capture document "$document_url" 390 900
require_json_file document "${out_dir}/document-390x900.json" '.iframe_count == 14 and .iframe_loaded_frames == 14 and .error_contracts == 8'

capture document-markdown "$document_markdown_url" 390 900 --palette green
require_json_file document-markdown "${out_dir}/document-markdown-390x900.json" '.theme_controls_one_row == true and .palette == "green" and .accent == "#2d7a46" and .copy_buttons >= 1 and .heading_anchors >= 1 and .task_checkboxes == 2 and .blockquotes == 1 and .component_counts.markdown_alert == 1 and .image_count == 2 and .loaded_images == 2 and .data_uri_images == 2 and .svg_images == 1 and .raster_images == 1'

capture document-output-dash "$document_output_dash_url" 390 900
require_json_file document-output-dash "${out_dir}/document-output-dash-390x900.json" '.theme_controls_one_row == true and .title == "Document Mode" and .copy_buttons >= 1 and .heading_anchors >= 1 and .image_count == 2 and .loaded_images == 2 and .data_uri_images == 2'

capture document-forced-markdown "$document_forced_markdown_url" 390 900
require_json_file document-forced-markdown "${out_dir}/document-forced-markdown-390x900.json" '.theme_controls_one_row == true and .title == "weak-inline" and .component_counts.markdown_body == 1'

capture document-safe "$document_safe_url" 390 900
require_json_file document-safe "${out_dir}/document-safe-390x900.json" '.theme_controls_one_row == true and .title == "Safe Mode" and .component_counts.markdown_body == 1 and .alert_text_present != true'

capture document-plain "$document_plain_url" 390 900
require_json_file document-plain "${out_dir}/document-plain-390x900.json" '.theme_controls_one_row == true and .title == "plain" and .plaintext_blocks == 1 and (.heading_anchors // 0) == 0 and .copy_buttons >= 1'

capture document-plain-csv-table "$document_plain_csv_table_url" 390 900
require_json_file document-plain-csv-table "${out_dir}/document-plain-csv-table-390x900.json" '.theme_controls_one_row == true and .title == "records" and .component_counts.plain_data_table == 1 and .component_counts.markdown_table == 1 and (.plaintext_blocks // 0) == 0 and (.copy_buttons // 0) == 0'

capture document-plain-column-table "$document_plain_column_table_url" 390 900
require_json_file document-plain-column-table "${out_dir}/document-plain-column-table-390x900.json" '.theme_controls_one_row == true and .title == "columns" and .component_counts.plain_data_table == 1 and .component_counts.markdown_table == 1 and (.plaintext_blocks // 0) == 0'

capture document-plain-skill-leaderboards "$document_plain_skill_leaderboards_url" 390 900
require_json_file document-plain-skill-leaderboards "${out_dir}/document-plain-skill-leaderboards-390x900.json" '.theme_controls_one_row == true and .title == "skill-leaderboards" and .component_counts.plain_table_section == 3 and .component_counts.plain_table_meta == 3 and .component_counts.plain_data_table == 3 and .component_counts.markdown_table == 3 and (.component_counts.chroma // 0) == 0 and .scroll_width <= .client_width'

capture document-code "$document_code_url" 390 900
require_json_file document-code "${out_dir}/document-code-390x900.json" '.theme_controls_one_row == true and .title == "sample" and .component_counts.chroma == 1 and .chroma_line_items == 14 and .copy_buttons >= 1'

capture document-stdout-plain-table "$document_stdout_plain_table_url" 390 900
require_json_file document-stdout-plain-table "${out_dir}/document-stdout-plain-table-390x900.json" '.theme_controls_one_row == true and .title == "CSV Stdin" and .component_counts.plain_data_table == 1 and .component_counts.markdown_table == 1 and (.plaintext_blocks // 0) == 0'

capture document-stdout-go "$document_stdout_go_url" 390 900
require_json_file document-stdout-go "${out_dir}/document-stdout-go-390x900.json" '.theme_controls_one_row == true and .title == "Go Stdin" and .component_counts.chroma == 1 and .copy_buttons >= 1'

capture document-frame "$document_frame_url" 390 900
require_json_file document-frame "${out_dir}/document-frame-390x900.json" '.theme_controls_one_row == true and .title == "build" and .component_counts.term_frame == 1 and .term_frame_bars == 1 and .term_frame_body_blocks == 1'

capture document-ansi "$document_ansi_url" 390 900
require_json_file document-ansi "${out_dir}/document-ansi-390x900.json" '.theme_controls_one_row == true and .title == "ANSI Stdin" and .ansi_code_blocks == 1 and .ansi_styled_spans == 2 and .ansi_lines == 4 and .copy_buttons >= 1'

capture document-stdin-markdown "$document_stdin_markdown_url" 390 900
require_json_file document-stdin-markdown "${out_dir}/document-stdin-markdown-390x900.json" '.theme_controls_one_row == true and .title == "Stdin Markdown" and .component_counts.markdown_body == 1 and .task_checkboxes == 2 and .heading_anchors >= 1'

capture config "$config_url" 390 900
require_json_file config "${out_dir}/config-390x900.json" '.title == "HTML Config Smoke" and .iframe_count == 1 and .iframe_loaded_frames == 1'

capture configured "$configured_url" 390 900
require_json_file configured "${out_dir}/configured-390x900.json" '.title == "Configured Output" and .theme == "dark" and .palette == "blue" and .accent == "#6da8ff" and .toc_count == 1 and .heading_anchors >= 3 and .markdown_max_width == "704px" and .theme_controls_one_row == true'

capture cache "$cache_url" 1440 900
require_json_file cache "${out_dir}/cache-1440x900.json" '.title == "HTML Cache Smoke" and .iframe_count == 2 and .iframe_loaded_frames == 2 and .cache_contracts == 1 and .cache_reused == "true" and .cache_forced == "true"'
capture cache "$cache_url" 390 900
require_json_file cache "${out_dir}/cache-390x900.json" '.title == "HTML Cache Smoke" and .iframe_count == 2 and .iframe_loaded_frames == 2 and .cache_contracts == 1 and .cache_reused == "true" and .cache_forced == "true"'

capture cache-file "$cache_file_url" 390 900
require_json_file cache-file "${out_dir}/cache-file-390x900.json" '.title == "Cache Smoke" and .theme_controls_one_row == true and .component_counts.markdown_body == 1 and .component_counts.chroma == 1 and .copy_buttons >= 1 and .heading_anchors >= 2 and .image_count == 1 and .loaded_images == 1 and .data_uri_images == 1 and .raster_images == 1'

capture detection "$detection_url" 1440 900
require_json_file detection "${out_dir}/detection-1440x900.json" "$detection_expr"
capture detection "$detection_url" 390 900
require_json_file detection "${out_dir}/detection-390x900.json" "$detection_expr"

capture theme-gallery "$theme_url" 1440 900
require_json_file theme-gallery "${out_dir}/theme-gallery-1440x900.json" "$theme_gallery_expr"
capture theme-gallery "$theme_url" 390 900
require_json_file theme-gallery "${out_dir}/theme-gallery-390x900.json" "$theme_gallery_expr"

theme_accent() {
  case "$1" in
    light-sepia) printf '#9f5b2d' ;;
    light-blue) printf '#2563eb' ;;
    light-green) printf '#2d7a46' ;;
    light-rose) printf '#be4b75' ;;
    light-catppuccin) printf '#8839ef' ;;
    dark-sepia) printf '#e0a05f' ;;
    dark-blue) printf '#6da8ff' ;;
    dark-green) printf '#79c48a' ;;
    dark-rose) printf '#f08ab2' ;;
    dark-catppuccin) printf '#cba6f7' ;;
    *) fail "unknown theme palette: $1" ;;
  esac
}
for theme in light dark; do
  for palette in sepia blue green rose catppuccin; do
    key="${theme}-${palette}"
    capture "theme-${key}" "file://${repo_dir}/.work/html-qa/theme-gallery/${key}.html" 390 900
    accent="$(theme_accent "$key")"
    require_json_file "theme-${key}" "${out_dir}/theme-${key}-390x900.json" ".theme == \"${theme}\" and .palette == \"${palette}\" and .accent == \"${accent}\" and .theme_controls_one_row == true and .iframe_count == 10 and .iframe_loaded_frames == 10"
  done
done

capture theme-light-media "file://${repo_dir}/.work/html-qa/theme-gallery/pages/light-sepia/article-media.html" 390 900
require_json_file theme-light-media "${out_dir}/theme-light-media-390x900.json" '.theme == "light" and .palette == "sepia" and .theme_controls_one_row == true and .image_count == 2 and .loaded_images == 2 and .data_uri_images == 2'

capture theme-dark-media "file://${repo_dir}/.work/html-qa/theme-gallery/pages/dark-catppuccin/article-media.html" 390 900
require_json_file theme-dark-media "${out_dir}/theme-dark-media-390x900.json" '.theme == "dark" and .palette == "catppuccin" and .theme_controls_one_row == true and .image_count == 2 and .loaded_images == 2 and .data_uri_images == 2'

capture theme-component-table "file://${repo_dir}/.work/html-qa/theme-gallery/pages/light-sepia/table.html" 390 900
require_json_file theme-component-table "${out_dir}/theme-component-table-390x900.json" '.theme == "light" and .palette == "sepia" and .theme_controls_one_row == true and .component_counts.report_table == 1 and .filter_status == "3 rows" and .report_data_rows == 3 and .visible_report_rows == 3'

capture theme-component-cards "file://${repo_dir}/.work/html-qa/theme-gallery/pages/light-sepia/cards.html" 390 900
require_json_file theme-component-cards "${out_dir}/theme-component-cards-390x900.json" '.theme == "light" and .palette == "sepia" and .theme_controls_one_row == true and .component_counts.record_cards == 1 and .record_card_items == 3'

capture theme-component-tabs "file://${repo_dir}/.work/html-qa/theme-gallery/pages/light-sepia/tabs.html" 390 900
require_json_file theme-component-tabs "${out_dir}/theme-component-tabs-390x900.json" '.theme == "light" and .palette == "sepia" and .theme_controls_one_row == true and .component_counts.report_tabs == 1 and .component_counts.report_table == 1 and .report_tab_buttons == 2 and .report_data_rows == 2 and .visible_report_rows == 2'

capture theme-component-slides "file://${repo_dir}/.work/html-qa/theme-gallery/pages/light-sepia/slides.html" 390 900
require_json_file theme-component-slides "${out_dir}/theme-component-slides-390x900.json" '.theme == "light" and .palette == "sepia" and .theme_controls_one_row == true and .component_counts.report_slides == 1 and .report_slide_items == 3 and .slide_status == "1 / 3"'

capture theme-component-json "file://${repo_dir}/.work/html-qa/theme-gallery/pages/light-sepia/json.html" 390 900
require_json_file theme-component-json "${out_dir}/theme-component-json-390x900.json" '.theme == "light" and .palette == "sepia" and .theme_controls_one_row == true and .component_counts.json_source == 1 and .component_counts.json_overview == 1 and .json_overview_items == 3 and .component_counts.report_summary == 1'

capture theme-component-diff "file://${repo_dir}/.work/html-qa/theme-gallery/pages/light-sepia/diff.html" 390 900
require_json_file theme-component-diff "${out_dir}/theme-component-diff-390x900.json" '.theme == "light" and .palette == "sepia" and .theme_controls_one_row == true and .component_counts.diff_view == 1 and .diff_rendered_lines == 7 and .diff_added_lines == 1 and .diff_removed_lines == 1 and .component_counts.report_summary == 1'

capture theme-component-tree "file://${repo_dir}/.work/html-qa/theme-gallery/pages/light-sepia/tree.html" 390 900
require_json_file theme-component-tree "${out_dir}/theme-component-tree-390x900.json" '.theme == "light" and .palette == "sepia" and .theme_controls_one_row == true and .component_counts.file_tree == 1 and .file_tree_items == 6 and .component_counts.report_summary == 1'

capture theme-component-log "file://${repo_dir}/.work/html-qa/theme-gallery/pages/light-sepia/log.html" 390 900
require_json_file theme-component-log "${out_dir}/theme-component-log-390x900.json" '.theme == "light" and .palette == "sepia" and .theme_controls_one_row == true and .component_counts.log_lines == 1 and .log_line_items == 3 and .log_severity_items == 3'

capture theme-component-code "file://${repo_dir}/.work/html-qa/theme-gallery/pages/light-sepia/plain-code.html" 390 900
require_json_file theme-component-code "${out_dir}/theme-component-code-390x900.json" '.theme == "light" and .palette == "sepia" and .theme_controls_one_row == true and .component_counts.chroma == 1 and .chroma_line_items == 14 and .copy_buttons >= 1'

capture theme-dark-component-table "file://${repo_dir}/.work/html-qa/theme-gallery/pages/dark-catppuccin/table.html" 390 900
require_json_file theme-dark-component-table "${out_dir}/theme-dark-component-table-390x900.json" '.theme == "dark" and .palette == "catppuccin" and .theme_controls_one_row == true and .component_counts.report_table == 1 and .report_data_rows == 3 and .visible_report_rows == 3'

capture theme-dark-component-diff "file://${repo_dir}/.work/html-qa/theme-gallery/pages/dark-catppuccin/diff.html" 390 900
require_json_file theme-dark-component-diff "${out_dir}/theme-dark-component-diff-390x900.json" '.theme == "dark" and .palette == "catppuccin" and .theme_controls_one_row == true and .component_counts.diff_view == 1 and .diff_rendered_lines == 7 and .diff_added_lines == 1 and .diff_removed_lines == 1'

capture media "$media_url" 390 900
require_json_file media "${out_dir}/media-390x900.json" '.theme_controls_one_row == true and .image_count == 2 and .loaded_images == 2 and .data_uri_images == 2 and .svg_images == 1 and .raster_images == 1'

capture media-catppuccin "$media_url" 390 900 --palette catppuccin
require_json_file media-catppuccin "${out_dir}/media-catppuccin-390x900.json" '.theme_controls_one_row == true and .palette == "catppuccin" and .accent == "#8839ef"'

capture media-dark "$media_url" 390 900 --click-theme-toggle
require_json_file media-dark "${out_dir}/media-dark-390x900.json" '.theme_controls_one_row == true and .theme == "dark" and .accent == "#e0a05f"'

capture media-dark-catppuccin "$media_url" 390 900 --click-theme-toggle --palette catppuccin
require_json_file media-dark-catppuccin "${out_dir}/media-dark-catppuccin-390x900.json" '.theme_controls_one_row == true and .theme == "dark" and .palette == "catppuccin" and .accent == "#cba6f7"'

capture matrix-markdown "$matrix_markdown_url" 390 900
require_json_file matrix-markdown "${out_dir}/matrix-markdown-390x900.json" '.title == "Release Notes" and .theme_controls_one_row == true and .component_counts.article_overview == 1 and .component_counts.markdown_body == 1 and .heading_anchors >= 3'

capture matrix-markdown-components "$matrix_markdown_components_url" 390 900
require_json_file matrix-markdown-components "${out_dir}/matrix-markdown-components-390x900.json" '.title == "Markdown Components" and .theme_controls_one_row == true and .component_counts.article_overview == 1 and .component_counts.markdown_body == 1 and .component_counts.markdown_table == 1 and .task_checkboxes == 2 and .blockquotes == 1 and .component_counts.chroma == 1 and .chroma_line_items == 2'

capture matrix-markdown-article-override "$matrix_markdown_article_override_url" 390 900
require_json_file matrix-markdown-article-override "${out_dir}/matrix-markdown-article-override-390x900.json" '.title == "Forced Article" and .theme_controls_one_row == true and .component_counts.article_overview == 1 and .component_counts.markdown_body == 1 and (.component_counts.report_summary // 0) == 0'

capture matrix-markdown-csv-precedence "$matrix_markdown_csv_precedence_url" 390 900
require_json_file matrix-markdown-csv-precedence "${out_dir}/matrix-markdown-csv-precedence-390x900.json" '.theme_controls_one_row == true and .component_counts.article_overview == 1 and .component_counts.markdown_body == 1 and (.component_counts.report_table // 0) == 0 and (.component_counts.record_cards // 0) == 0 and (.component_counts.json_source // 0) == 0'

capture matrix-markdown-json-precedence "$matrix_markdown_json_precedence_url" 390 900
require_json_file matrix-markdown-json-precedence "${out_dir}/matrix-markdown-json-precedence-390x900.json" '.theme_controls_one_row == true and .component_counts.article_overview == 1 and .component_counts.markdown_body == 1 and (.component_counts.report_table // 0) == 0 and (.component_counts.record_cards // 0) == 0 and (.component_counts.json_source // 0) == 0'

capture matrix-markdown-unknown-structure "$matrix_markdown_unknown_structure_url" 390 900
require_json_file matrix-markdown-unknown-structure "${out_dir}/matrix-markdown-unknown-structure-390x900.json" '.title == "Title" and .theme_controls_one_row == true and .component_counts.article_overview == 1 and .component_counts.markdown_body == 1 and .component_counts.chroma == 1 and .chroma_line_items == 2 and (.component_counts.report_summary // 0) == 0'

capture matrix-markdown-unknown-task-list "$matrix_markdown_unknown_task_list_url" 390 900
require_json_file matrix-markdown-unknown-task-list "${out_dir}/matrix-markdown-unknown-task-list-390x900.json" '.title == "Markdown Unknown Task List" and .theme_controls_one_row == true and .component_counts.article_overview == 1 and .component_counts.markdown_body == 1 and .task_checkboxes == 2 and (.component_counts.report_summary // 0) == 0'

capture matrix-json-records "$matrix_json_records_url" 390 900
require_json_file matrix-json-records "${out_dir}/matrix-json-records-390x900.json" '.title == "JSON Records" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and .filter_status == "2 rows" and .report_data_rows == 2 and .visible_report_rows == 2'

capture matrix-json-record-cards "$matrix_json_record_cards_url" 390 900
require_json_file matrix-json-record-cards "${out_dir}/matrix-json-record-cards-390x900.json" '.title == "JSON Record Cards" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.record_cards == 1 and .record_card_items == 3'

capture matrix-ndjson-records "$matrix_ndjson_records_url" 390 900
require_json_file matrix-ndjson-records "${out_dir}/matrix-ndjson-records-390x900.json" '.title == "NDJSON Records" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and .filter_status == "2 rows"'

capture matrix-single-jsonl-record "$matrix_single_jsonl_record_url" 390 900
require_json_file matrix-single-jsonl-record "${out_dir}/matrix-single-jsonl-record-390x900.json" '.title == "Single JSONL Record" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and .filter_status == "1 row" and .report_data_rows == 1 and .visible_report_rows == 1'

capture matrix-single-jsonlines-record "$matrix_single_jsonlines_record_url" 390 900
require_json_file matrix-single-jsonlines-record "${out_dir}/matrix-single-jsonlines-record-390x900.json" '.title == "Single JSONLines Record" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and .filter_status == "1 row" and .report_data_rows == 1 and .visible_report_rows == 1'

capture matrix-jsonl-record-cards "$matrix_jsonl_record_cards_url" 390 900
require_json_file matrix-jsonl-record-cards "${out_dir}/matrix-jsonl-record-cards-390x900.json" '.title == "JSONL Record Cards" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.record_cards == 1 and .record_card_items == 2 and (.component_counts.report_table // 0) == 0'

capture matrix-csv-records "$matrix_csv_records_url" 390 900
require_json_file matrix-csv-records "${out_dir}/matrix-csv-records-390x900.json" '.title == "CSV Records" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and .filter_status == "2 rows" and .report_data_rows == 2 and .visible_report_rows == 2'

capture matrix-bom-csv-records "$matrix_bom_csv_records_url" 390 900
require_json_file matrix-bom-csv-records "${out_dir}/matrix-bom-csv-records-390x900.json" '.title == "BOM CSV Records" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and .filter_status == "1 row" and .report_data_rows == 1 and .visible_report_rows == 1'

capture matrix-bad-csv-source-code "$matrix_bad_csv_source_code_url" 390 900
require_json_file matrix-bad-csv-source-code "${out_dir}/matrix-bad-csv-source-code-390x900.json" '.title == "Malformed CSV Source Code" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.code_overview == 1 and .component_counts.chroma == 1 and .chroma_line_items == 6 and (.component_counts.report_table // 0) == 0'

capture matrix-transcript "$matrix_transcript_url" 390 900
require_json_file matrix-transcript "${out_dir}/matrix-transcript-390x900.json" '.title == "Transcript" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .transcript_turns == 3 and .transcript_speakers == 3'

capture matrix-mixed "$matrix_mixed_url" 390 900
require_json_file matrix-mixed "${out_dir}/matrix-mixed-390x900.json" '.title == "Mixed Input" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .report_text_blocks == 1 and .text_overviews == 1 and .report_text_lines == 8'

capture matrix-plain "$matrix_plain_url" 390 900
require_json_file matrix-plain "${out_dir}/matrix-plain-390x900.json" '.title == "Plain Text" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .report_text_blocks == 1 and .text_overviews == 1 and .report_text_lines == 3'

capture matrix-plain-log "$matrix_plain_log_url" 390 900
require_json_file matrix-plain-log "${out_dir}/matrix-plain-log-390x900.json" '.title == "Plain Log Override" and .theme_controls_one_row == true and .component_counts.log_lines == 1 and (.text_overviews // 0) == 0 and (.report_text_blocks // 0) == 0'

capture matrix-bom-json "$matrix_bom_json_url" 390 900
require_json_file matrix-bom-json "${out_dir}/matrix-bom-json-390x900.json" '.title == "BOM JSON Records" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and .filter_status == "1 row"'

capture matrix-json-object "$matrix_json_object_url" 390 900
require_json_file matrix-json-object "${out_dir}/matrix-json-object-390x900.json" '.title == "JSON Object" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.json_source == 1 and .component_counts.json_overview == 1 and .json_overview_items == 3'

capture matrix-json-scalar-array "$matrix_json_scalar_array_url" 390 900
require_json_file matrix-json-scalar-array "${out_dir}/matrix-json-scalar-array-390x900.json" '.title == "JSON Scalar Array" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.json_source == 1 and .component_counts.json_overview == 1 and .json_overview_items == 2'

capture matrix-json-scalar-file "$matrix_json_scalar_file_url" 390 900
require_json_file matrix-json-scalar-file "${out_dir}/matrix-json-scalar-file-390x900.json" '.title == "JSON Scalar File" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.json_source == 1 and .component_counts.json_overview == 1 and .json_overview_items == 1 and (.component_counts.report_table // 0) == 0'

capture matrix-empty-json-array "$matrix_empty_json_array_url" 390 900
require_json_file matrix-empty-json-array "${out_dir}/matrix-empty-json-array-390x900.json" '.title == "Empty JSON Array" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.json_source == 1 and .component_counts.json_overview == 1 and .json_overview_items == 2 and (.component_counts.report_table // 0) == 0 and (.component_counts.record_cards // 0) == 0'

capture matrix-json-empty-object-array "$matrix_json_empty_object_array_url" 390 900
require_json_file matrix-json-empty-object-array "${out_dir}/matrix-json-empty-object-array-390x900.json" '.title == "JSON Empty Object Array" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.json_source == 1 and .component_counts.json_overview == 1 and .json_overview_items == 2 and (.component_counts.report_table // 0) == 0 and (.component_counts.record_cards // 0) == 0'

capture matrix-bad-json-source-code "$matrix_bad_json_source_code_url" 390 900
require_json_file matrix-bad-json-source-code "${out_dir}/matrix-bad-json-source-code-390x900.json" '.title == "Malformed JSON Source Code" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.code_overview == 1 and .component_counts.chroma == 1 and .chroma_line_items == 2 and (.component_counts.report_table // 0) == 0 and (.component_counts.json_source // 0) == 0'

capture matrix-bad-jsonl-source-code "$matrix_bad_jsonl_source_code_url" 390 900
require_json_file matrix-bad-jsonl-source-code "${out_dir}/matrix-bad-jsonl-source-code-390x900.json" '.title == "Malformed JSONL Source Code" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.code_overview == 1 and .component_counts.chroma == 1 and .chroma_line_items == 4 and (.component_counts.report_table // 0) == 0 and (.component_counts.json_source // 0) == 0'

capture matrix-csv-header-only "$matrix_csv_header_only_url" 390 900
require_json_file matrix-csv-header-only "${out_dir}/matrix-csv-header-only-390x900.json" '.title == "CSV Header Only" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and .filter_status == "No rows" and .empty_row_visible == true and .empty_row_text == "No rows"'

capture matrix-timestamped-csv-records "$matrix_timestamped_csv_records_url" 390 900
require_json_file matrix-timestamped-csv-records "${out_dir}/matrix-timestamped-csv-records-390x900.json" '.title == "Timestamped CSV Records" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and (.component_counts.log_lines // 0) == 0 and .filter_status == "2 rows" and .report_data_rows == 2 and .visible_report_rows == 2'

capture matrix-csv-tabs "$matrix_csv_tabs_url" 390 900
require_json_file matrix-csv-tabs "${out_dir}/matrix-csv-tabs-390x900.json" '.title == "CSV Tabs" and .theme_controls_one_row == true and .component_counts.report_tabs == 1 and .component_counts.report_table == 1 and .report_tab_buttons == 2 and .report_data_rows == 2 and .visible_report_rows == 2'

capture matrix-csv-cards-override "$matrix_csv_cards_override_url" 390 900
require_json_file matrix-csv-cards-override "${out_dir}/matrix-csv-cards-override-390x900.json" '.title == "CSV Cards Override" and .theme_controls_one_row == true and .component_counts.record_cards == 1 and .record_card_items == 2 and (.component_counts.report_summary // 0) == 0'

capture matrix-csv-code-override "$matrix_csv_code_override_url" 390 900
require_json_file matrix-csv-code-override "${out_dir}/matrix-csv-code-override-390x900.json" '.title == "CSV Code Override" and .theme_controls_one_row == true and .component_counts.code_overview == 1 and .code_overview_items == 4 and .component_counts.chroma == 1 and .chroma_line_items == 6 and (.component_counts.report_table // 0) == 0'

capture matrix-tsv-records "$matrix_tsv_records_url" 390 900
require_json_file matrix-tsv-records "${out_dir}/matrix-tsv-records-390x900.json" '.title == "TSV Records" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and .filter_status == "2 rows"'

capture matrix-table-records "$matrix_table_records_url" 390 900
require_json_file matrix-table-records "${out_dir}/matrix-table-records-390x900.json" '.title == "ASCII Table Records" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and .filter_status == "2 rows"'

capture matrix-psql-table "$matrix_psql_table_url" 390 900
require_json_file matrix-psql-table "${out_dir}/matrix-psql-table-390x900.json" '.title == "PSQL Table Records" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.report_table == 1 and .filter_status == "2 rows"'

capture matrix-diff "$matrix_diff_url" 390 900
require_json_file matrix-diff "${out_dir}/matrix-diff-390x900.json" '.title == "Unified Diff" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.diff_view == 1 and .diff_rendered_lines == 7 and .diff_added_lines == 1 and .diff_removed_lines == 1'

capture matrix-plain-diff-headers "$matrix_plain_diff_headers_url" 390 900
require_json_file matrix-plain-diff-headers "${out_dir}/matrix-plain-diff-headers-390x900.json" '.title == "Plain Diff Headers" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.diff_view == 1 and .diff_rendered_lines == 5 and .diff_added_lines == 1 and .diff_removed_lines == 1'

capture matrix-plain-diff-multi-file "$matrix_plain_diff_multi_file_url" 390 900
require_json_file matrix-plain-diff-multi-file "${out_dir}/matrix-plain-diff-multi-file-390x900.json" '.title == "Plain Diff Multi File" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.diff_view == 1 and .diff_rendered_lines == 10 and .diff_added_lines == 2 and .diff_removed_lines == 2'

capture matrix-combined-diff "$matrix_combined_diff_url" 390 900
require_json_file matrix-combined-diff "${out_dir}/matrix-combined-diff-390x900.json" '.title == "Combined Diff" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.diff_view == 1 and .diff_rendered_lines == 8 and .diff_added_lines == 1 and .diff_removed_lines == 2'

capture matrix-git-binary-patch "$matrix_git_binary_patch_url" 390 900
require_json_file matrix-git-binary-patch "${out_dir}/matrix-git-binary-patch-390x900.json" '.title == "Git Binary Patch" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.diff_view == 1 and .diff_rendered_lines == 6 and (.diff_added_lines // 0) == 0 and (.diff_removed_lines // 0) == 0'

capture matrix-git-mode-only-patch "$matrix_git_mode_only_patch_url" 390 900
require_json_file matrix-git-mode-only-patch "${out_dir}/matrix-git-mode-only-patch-390x900.json" '.title == "Git Mode-only Patch" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.diff_view == 1 and .diff_rendered_lines == 3 and (.diff_added_lines // 0) == 0 and (.diff_removed_lines // 0) == 0'

capture matrix-git-copy-only-patch "$matrix_git_copy_only_patch_url" 390 900
require_json_file matrix-git-copy-only-patch "${out_dir}/matrix-git-copy-only-patch-390x900.json" '.title == "Git Copy-only Patch" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.diff_view == 1 and .diff_rendered_lines == 3 and (.diff_added_lines // 0) == 0 and (.diff_removed_lines // 0) == 0'

capture matrix-diff-override "$matrix_diff_override_url" 390 900
require_json_file matrix-diff-override "${out_dir}/matrix-diff-override-390x900.json" '.title == "Diff Override" and .theme_controls_one_row == true and .component_counts.diff_view == 1 and .diff_rendered_lines == 6 and .diff_added_lines == 1 and .diff_removed_lines == 1 and (.component_counts.report_summary // 0) == 0'

capture matrix-source-code "$matrix_source_code_url" 390 900
require_json_file matrix-source-code "${out_dir}/matrix-source-code-390x900.json" '.title == "Source Code" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.code_overview == 1 and .code_overview_items == 4 and .component_counts.chroma == 1 and .chroma_line_items == 14 and .copy_buttons >= 1'

capture matrix-yaml-source-code "$matrix_yaml_source_code_url" 390 900
require_json_file matrix-yaml-source-code "${out_dir}/matrix-yaml-source-code-390x900.json" '.title == "YAML Source Code" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.code_overview == 1 and .code_overview_items == 4 and .component_counts.chroma == 1 and .chroma_line_items == 12 and .copy_buttons >= 1'

capture matrix-shell-content-source "$matrix_shell_content_source_url" 390 900
require_json_file matrix-shell-content-source "${out_dir}/matrix-shell-content-source-390x900.json" '.title == "Shell Content Source" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.code_overview == 1 and .code_overview_items == 4 and .component_counts.chroma == 1 and .chroma_line_items == 6 and .copy_buttons >= 1'

capture matrix-go-source-precedence "$matrix_go_source_precedence_url" 390 900
require_json_file matrix-go-source-precedence "${out_dir}/matrix-go-source-precedence-390x900.json" '.title == "Go Source Precedence" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.code_overview == 1 and .code_overview_items == 4 and .component_counts.chroma == 1 and .chroma_line_items == 2 and .copy_buttons >= 1'

capture matrix-go-source-fence-string "$matrix_go_source_fence_string_url" 390 900
require_json_file matrix-go-source-fence-string "${out_dir}/matrix-go-source-fence-string-390x900.json" '.title == "Go Source Fence String" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.code_overview == 1 and .code_overview_items == 4 and .component_counts.chroma == 1 and .copy_buttons >= 1 and (.component_counts.markdown_body // 0) == 1 and (.component_counts.report_table // 0) == 0 and (.component_counts.json_source // 0) == 0'

capture matrix-go-source-csv-precedence "$matrix_go_source_csv_precedence_url" 390 900
require_json_file matrix-go-source-csv-precedence "${out_dir}/matrix-go-source-csv-precedence-390x900.json" '.title == "Go Source CSV Precedence" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.code_overview == 1 and .code_overview_items == 4 and .component_counts.chroma == 1 and .copy_buttons >= 1 and (.component_counts.report_table // 0) == 0 and (.component_counts.record_cards // 0) == 0 and (.component_counts.json_source // 0) == 0'

capture matrix-tree-listing "$matrix_tree_listing_url" 390 900
require_json_file matrix-tree-listing "${out_dir}/matrix-tree-listing-390x900.json" '.title == "Tree Listing" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.file_tree == 1 and .file_tree_items == 6'

capture matrix-ascii-tree-listing "$matrix_ascii_tree_listing_url" 390 900
require_json_file matrix-ascii-tree-listing "${out_dir}/matrix-ascii-tree-listing-390x900.json" '.title == "ASCII Tree Listing" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.file_tree == 1 and .file_tree_items == 3'

capture matrix-tree-summary-listing "$matrix_tree_summary_listing_url" 390 900
require_json_file matrix-tree-summary-listing "${out_dir}/matrix-tree-summary-listing-390x900.json" '.title == "Tree Summary Listing" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.file_tree == 1 and .file_tree_items == 3'

capture matrix-posix-path-listing "$matrix_posix_path_listing_url" 390 900
require_json_file matrix-posix-path-listing "${out_dir}/matrix-posix-path-listing-390x900.json" '.title == "POSIX Path Listing" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.file_tree == 1 and .file_tree_items == 3 and (.component_counts.report_table // 0) == 0'

capture matrix-absolute-path-listing "$matrix_absolute_path_listing_url" 390 900
require_json_file matrix-absolute-path-listing "${out_dir}/matrix-absolute-path-listing-390x900.json" '.title == "Absolute Path Listing" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.file_tree == 1 and .file_tree_items == 3 and (.component_counts.report_table // 0) == 0'

capture matrix-spaced-path-listing "$matrix_spaced_path_listing_url" 390 900
require_json_file matrix-spaced-path-listing "${out_dir}/matrix-spaced-path-listing-390x900.json" '.title == "Spaced Path Listing" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.file_tree == 1 and .file_tree_items == 3 and (.component_counts.report_table // 0) == 0'

capture matrix-windows-path "$matrix_windows_path_url" 390 900
require_json_file matrix-windows-path "${out_dir}/matrix-windows-path-390x900.json" '.title == "Windows Path Listing" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.file_tree == 1 and .file_tree_items == 3'

capture matrix-tree-override "$matrix_tree_override_url" 390 900
require_json_file matrix-tree-override "${out_dir}/matrix-tree-override-390x900.json" '.title == "Tree Override" and .theme_controls_one_row == true and .component_counts.file_tree == 1 and .file_tree_items == 3 and (.component_counts.report_summary // 0) == 0'

capture matrix-url-list-plain "$matrix_url_list_plain_url" 390 900
require_json_file matrix-url-list-plain "${out_dir}/matrix-url-list-plain-390x900.json" '.title == "URL List Plain" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .report_text_blocks == 1 and .text_overviews == 1 and .report_text_lines == 4'

capture matrix-fractions-plain "$matrix_fractions_plain_url" 390 900
require_json_file matrix-fractions-plain "${out_dir}/matrix-fractions-plain-390x900.json" '.title == "Fractions Plain" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .report_text_blocks == 1 and .text_overviews == 1 and .report_text_lines == 4 and (.component_counts.file_tree // 0) == 0'

capture matrix-http-request-paths-plain "$matrix_http_request_paths_plain_url" 390 900
require_json_file matrix-http-request-paths-plain "${out_dir}/matrix-http-request-paths-plain-390x900.json" '.title == "HTTP Request Paths Plain" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .report_text_blocks == 1 and .text_overviews == 1 and .report_text_lines == 4 and (.component_counts.file_tree // 0) == 0'

capture matrix-ordinary-ok-plain "$matrix_ordinary_ok_plain_url" 390 900
require_json_file matrix-ordinary-ok-plain "${out_dir}/matrix-ordinary-ok-plain-390x900.json" '.title == "Ordinary OK Plain" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .report_text_blocks == 1 and .text_overviews == 1 and .report_text_lines == 3 and (.component_counts.log_lines // 0) == 0'

capture matrix-config-keys-plain "$matrix_config_keys_plain_url" 390 900
require_json_file matrix-config-keys-plain "${out_dir}/matrix-config-keys-plain-390x900.json" '.title == "Config Keys Plain" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .report_text_blocks == 1 and .text_overviews == 1 and .report_text_lines == 4 and (.transcript_turns // 0) == 0'

capture matrix-dash-divider-plain "$matrix_dash_divider_plain_url" 390 900
require_json_file matrix-dash-divider-plain "${out_dir}/matrix-dash-divider-plain-390x900.json" '.title == "Dash Divider Plain" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .report_text_blocks == 1 and .text_overviews == 1 and .report_text_lines == 5 and (.component_counts.article_overview // 0) == 0'

capture matrix-log "$matrix_log_url" 390 900
require_json_file matrix-log "${out_dir}/matrix-log-390x900.json" '.title == "HTTP Access Log" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.log_lines == 1 and .log_line_items == 3 and .log_severity_items == 3'

capture matrix-go-test-log "$matrix_go_test_log_url" 390 900
require_json_file matrix-go-test-log "${out_dir}/matrix-go-test-log-390x900.json" '.title == "Go Test Log" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.log_lines == 1 and .log_line_items == 2 and .log_severity_items == 2'

capture matrix-single-severity-log "$matrix_single_severity_log_url" 390 900
require_json_file matrix-single-severity-log "${out_dir}/matrix-single-severity-log-390x900.json" '.title == "Single Severity Log" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.log_lines == 1 and .log_line_items == 1 and .log_severity_items == 1'

capture matrix-single-go-test-log "$matrix_single_go_test_log_url" 390 900
require_json_file matrix-single-go-test-log "${out_dir}/matrix-single-go-test-log-390x900.json" '.title == "Single Go Test Log" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.log_lines == 1 and .log_line_items == 1 and .log_severity_items == 1'

capture matrix-generic-speaker-transcript "$matrix_generic_speaker_transcript_url" 390 900
require_json_file matrix-generic-speaker-transcript "${out_dir}/matrix-generic-speaker-transcript-390x900.json" '.title == "Generic Speaker Transcript" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .transcript_turns == 3 and .transcript_speakers == 3'

capture matrix-markdown-slides "$matrix_markdown_slides_url" 390 900
require_json_file matrix-markdown-slides "${out_dir}/matrix-markdown-slides-390x900.json" '.title == "Deck" and .theme_controls_one_row == true and .component_counts.report_slides == 1 and .report_slide_items == 3 and .slide_status == "1 / 3"'

capture matrix-mixed-single-override "$matrix_mixed_single_override_url" 390 900
require_json_file matrix-mixed-single-override "${out_dir}/matrix-mixed-single-override-390x900.json" '.title == "Mixed Single Layout Override" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .report_text_blocks == 1 and .text_overviews == 1'

capture matrix-single-comma-plain "$matrix_single_comma_plain_url" 390 900
require_json_file matrix-single-comma-plain "${out_dir}/matrix-single-comma-plain-390x900.json" '.title == "Single Comma Plain" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .report_text_blocks == 1 and .text_overviews == 1 and .report_text_lines == 2 and (.component_counts.report_table // 0) == 0'

capture matrix-yaml-plain "$matrix_yaml_plain_url" 390 900
require_json_file matrix-yaml-plain "${out_dir}/matrix-yaml-plain-390x900.json" '.title == "YAML-like Plain Text" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .report_text_blocks == 1 and .text_overviews == 1 and .report_text_lines == 7'

capture matrix-binary "$matrix_binary_url" 390 900
require_json_file matrix-binary "${out_dir}/matrix-binary-390x900.json" '.title == "Binary Preview" and .theme_controls_one_row == true and .component_counts.report_summary == 1 and .component_counts.binary_preview == 1 and .binary_preview_lines == 1'

capture binary "$binary_url" 390 900
require_json_file binary "${out_dir}/binary-390x900.json" '.title == "logo" and .theme_controls_one_row == true and .component_counts.binary_preview == 1 and .binary_preview_lines == 1'

capture slides-next "$slides_url" 390 900 --click-slide-next
require_json_file slides-next "${out_dir}/slides-next-390x900.json" '.slide_status == "2 / 3" and .report_slide_items == 3 and (.current_slide | contains("Slide 2 of 3: First"))'

capture tabs-records "$tabs_url" 390 900 --click-tab Records
require_json_file tabs-records "${out_dir}/tabs-records-390x900.json" '.selected_tab == "Records" and .report_tab_buttons == 2 and .report_data_rows == 2 and .visible_report_rows == 2 and (.visible_tab_panel | contains("Records")) and (.visible_tab_panel | contains("Sort rows")) and (.filter_status == "2 rows") and (.first_row | contains("alpha"))'

capture table-filter "$table_url" 390 900 --filter beta
require_json_file table-filter "${out_dir}/table-filter-390x900.json" '.filter_status == "1 row" and .report_data_rows == 2 and .visible_report_rows == 1 and (.first_row | contains("beta")) and (.empty_row_visible != true)'

capture table-filter-empty "$table_url" 390 900 --filter zzz
require_json_file table-filter-empty "${out_dir}/table-filter-empty-390x900.json" '.filter_status == "No rows match" and .report_data_rows == 2 and (.visible_report_rows // 0) == 0 and .empty_row_visible == true and .empty_row_text == "No rows match" and .first_row == "No rows match"'

capture table-sort-score "$table_url" 1440 900 --sort-header score
require_json_file table-sort-score "${out_dir}/table-sort-score-1440x900.json" '.filter_status == "2 rows" and .report_data_rows == 2 and .visible_report_rows == 2 and .sort_state == "ascending" and .mobile_sort == "1:ascending" and (.sort_label | contains("descending")) and (.first_row | contains("beta"))'

capture table-mobile-sort "$table_url" 390 900 --mobile-sort "1:descending"
require_json_file table-mobile-sort "${out_dir}/table-mobile-sort-390x900.json" '.filter_status == "2 rows" and .report_data_rows == 2 and .visible_report_rows == 2 and .mobile_sort == "1:descending" and .sort_state == "descending" and (.sort_label | contains("ascending")) and (.first_row | contains("alpha"))'

printf 'qa-browser: ok (%s)\n' "$out_dir"
