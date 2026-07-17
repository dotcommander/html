# html

> Render Markdown or piped text to a clean, self-contained HTML page and open it in the browser.

`html` turns a Markdown file — or anything you pipe to it (`tree -d | html`, `git diff --color | html`, `cat main.go | html`) — into a single offline HTML document with GitHub-style rendering, syntax highlighting, and copy buttons, then opens it in your browser. No external CSS, JS, or fonts: the output is one file you can email, commit, or open on a plane. Non-Markdown input (logs, source, JSON, command output) renders as faithful, syntax-highlighted preformatted text instead of being mangled by the Markdown parser. Results are cached and only re-rendered when the source changes.

## Install

```bash
# Immutable release (recommended)
go install github.com/dotcommander/html/cmd/html@v0.2.1

# Latest published release
go install github.com/dotcommander/html/cmd/html@latest
```

Requires Go 1.26.3 or newer on macOS, Linux, or Windows. The directory selected
by `GOBIN` (or the first `GOPATH` entry plus `/bin`) must be on `PATH`.

## Quick start

```bash
html README.md            # render + open in your browser (prints the cache path)
html -n README.md         # render only, don't open (-n / --no-open)
tree -d | html            # pipe any command output — auto-detected
```

## What it does

```bash
html README.md            # Markdown file → GitHub-style page, opened in the browser
tree -d | html            # pipe stdin: auto-detected as Markdown or plain text
git diff --color | html   # ANSI colors preserved as styled spans — diffs stay colored
git diff --color | html --frame   # wrap it in a faux terminal window — a share-ready "screenshot"
cat main.go | html        # plain code is auto syntax-highlighted (language detected)
html data.json            # files highlight by extension (.go / .json / .py / …)
```

Generated CSS and JavaScript are always embedded. In trusted Markdown mode,
supported local images up to 10 MiB each are embedded until the document reaches
the 32 MiB image budget; remote, missing, unsupported, oversized, and over-budget
images remain external references. Repeated references count toward the budget.
For trusted Markdown files, the CLI reports each distinct non-embedded image on
stderr with a stable reason code, without changing the rendered document or
cache path written to stdout.
Plain text and stdin have no local-image base directory.

## Common flags

| Flag | Effect |
| --- | --- |
| `-n`, `--no-open` | render only; print the cache path without opening the browser |
| `-f`, `--force` | rebuild even if the cached HTML is fresh |
| `-o`, `--output <path>` | write the HTML to a stable path (`-` = stdout) |
| `-p`, `--plain` | force preformatted plain text (skip Markdown parsing) |
| `-m`, `--markdown` | force Markdown (override stdin auto-detection) |
| `-t`, `--title <text>` | page title for piped input (default `stdin`) |
| `-l`, `--lang <lang>` | syntax-highlight language for plain mode (`go`, `json`; `text` = none) |
| `--frame` | wrap plain/ANSI output in a terminal-window frame, implies `--plain` (share-ready "screenshot") |
| `--safe` | disable raw-HTML passthrough — use for untrusted Markdown |
| `--version` | print the release version (`html devel` for local builds) |

Run `html --help` for the full list, including the report-mode flags (`--mode`, `--layout`).

## Markdown vs. plain text

Piped input is auto-classified. A high-confidence structural signal — a fenced code block, a GFM table, or a setext heading — makes it Markdown; otherwise it stays plain text, so scripts, diffs, JSON, YAML, and logs are rendered faithfully rather than mangled. Normal document rendering refuses binary input (a NUL byte, or >10% non-text bytes); report mode can render a safe hex/ascii binary preview. Force the document mode with `-m` / `-p`.

Files are decided by extension: `.md` / `.markdown` → Markdown, everything else → plain.

GitHub-style alerts render as themed callouts in trusted and safe Markdown:

```markdown
> [!WARNING]
> Back up the destination before replacing it.
```

The supported alert types are `NOTE`, `TIP`, `IMPORTANT`, `WARNING`, and
`CAUTION`.

## Output & caching

Rendered pages are cached under `~/.config/html/cache/` and reused until the source changes. Use `-f` to force a re-render, or `-o <path>` to write the HTML somewhere stable to share or attach.

For trusted cached file renders, relative Markdown links are resolved against the
source document and emitted as absolute `file:` URLs. Explicit `-o`/stdout
output and `--safe` mode preserve links as written.

Cache validity includes the source bytes, not only modification times. Cache
directories are private to the current user. Stable outputs are written
atomically, and `--output` refuses to overwrite the input through the same path,
a symlink, or a hardlink.

## Reports and optional planning

Normal document rendering is the default. Report output is opt-in through
`--plan`, `--mode`, `--layout`, or `--planner`. The deterministic planner is the
default and performs no network request.

The optional LLM planner requires an explicit `--planner auto` or `--planner llm`
plus both `--llm-url` and `--llm-model`. The URL must be HTTP(S). A request may
send analysis metadata and up to 8 KiB of input to that endpoint:

```bash
html data.json --plan --planner llm \
  --llm-url https://example.invalid/v1/chat/completions \
  --llm-model example-model --llm-timeout 10s
```

For two-column record data with one category column and one finite numeric
column, `--mode chart` renders a bounded, accessible horizontal bar chart and
keeps the full data table alongside it. Inputs with more than 1,000 rows or 24
visible categories get an inline chart diagnostic while the summary and table
remain available.

In automatic report mode, an H2 or H3 section whose complete body is an ordered
list becomes a timeline. The planner preserves all other Markdown as article
content and validates that the components still own the original source bytes.

## Browser QA

Run the generated browser QA suite with:

```bash
just qa-browser
```

The suite regenerates `.work/html-qa/`, uses the source-owned chromedp helper at
`tools/chromedp-capture`, captures desktop/mobile PNGs, checks console errors and
overflow, and writes JSON metrics under `.work/html-qa/browser/`.

To compare supported Markdown fixtures with GitHub's rendering API, run the
opt-in network check while authenticated with `gh`:

```bash
just qa-github-markdown
```

## Configuration (optional)

`~/.config/html/config.json` — every field is optional; a missing file means default behavior. This is valid JSON containing every supported key:

```json
{
  "open_command": "firefox",
  "max_width": "60rem",
  "default_theme": "dark",
  "default_palette": "blue",
  "default_code_theme": "dracula",
  "toc": true
}
```

`open_command` defaults to the platform launcher. `max_width` accepts a CSS
length. Theme values are `light`, `dark`, or `auto`; palettes are `sepia`,
`blue`, `green`, `rose`, or `catppuccin`;
`default_code_theme` is a Chroma style.
Omit `toc` to retain automatic table-of-contents selection.

## Safety

Raw HTML in Markdown is passed through by default for trusted local files. For
untrusted input, pass `--safe`. Safe mode strips raw HTML, performs no local
image reads or image fingerprinting, and renders Markdown images as escaped,
non-fetching placeholders with no `src`. Ordinary links remain clickable.
