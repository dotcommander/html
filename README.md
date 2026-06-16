# html

> Render Markdown or piped text to a clean, self-contained HTML page and open it in the browser.

`html` turns a Markdown file — or anything you pipe to it (`tree -d | html`, `git diff --color | html`, `cat main.go | html`) — into a single offline HTML document with GitHub-style rendering, syntax highlighting, and copy buttons, then opens it in your browser. No external CSS, JS, or fonts: the output is one file you can email, commit, or open on a plane. Non-Markdown input (logs, source, JSON, command output) renders as faithful, syntax-highlighted preformatted text instead of being mangled by the Markdown parser. Results are cached and only re-rendered when the source changes.

## Install

```bash
# from the repo root
go build -o html ./cmd/html/ && ln -sf "$(pwd)/html" ~/go/bin/html
```

Requires Go 1.26+. `~/go/bin` must be on your `PATH`.

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

Every output is a single self-contained `.html` file — open it offline, no network, no assets.

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

Run `html --help` for the full list, including the report-mode flags (`--mode`, `--layout`).

## Markdown vs. plain text

Piped input is auto-classified. A high-confidence structural signal — a fenced code block, a GFM table, or a setext heading — makes it Markdown; otherwise it stays plain text, so scripts, diffs, JSON, YAML, and logs are rendered faithfully rather than mangled. Binary input (a NUL byte, or >10% non-text bytes) is refused. Force the mode with `-m` / `-p`.

Files are decided by extension: `.md` / `.markdown` → Markdown, everything else → plain.

## Output & caching

Rendered pages are cached under `~/.config/html/cache/` and reused until the source changes. Use `-f` to force a re-render, or `-o <path>` to write the HTML somewhere stable to share or attach.

## Configuration (optional)

`~/.config/html/config.json` — every field optional; a missing file means default behavior:

```json
{
  "open_command": "firefox",   // launcher; "" = OS default (open / xdg-open / start)
  "max_width": "60rem",        // reader column width (any CSS length)
  "default_theme": "dark",     // "light" | "dark" | "auto" ("" = follow system)
  "toc": true                  // force a table of contents (omit = automatic for 4+ headings)
}
```

## Safety

Raw HTML in Markdown is passed through by default — these are your own local files. For untrusted input, pass `--safe` to strip raw-HTML passthrough.
