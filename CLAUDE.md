# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`html` is a Go CLI that converts a Markdown file — or data piped on stdin (e.g. `tree -d | html`) — into a single self-contained, offline HTML document with GitHub-style rendering, syntax highlighting, and copy buttons, then opens it in the browser. Non-Markdown input (command output, logs, source, JSON) renders as faithful preformatted text instead of being mangled by the Markdown parser. Output is cached and only re-rendered when the source changes.

## Build / Install / Run

```bash
go build ./cmd/html/                  # build
go build -o html ./cmd/html/ && ln -sf "$(pwd)/html" ~/go/bin/html   # install to PATH

html README.md            # render + open in browser, prints cache path
html -n README.md         # --no-open: render only, don't launch browser
html -f README.md         # --force: re-render even if cache is fresh
html --safe untrusted.md   # disable raw-HTML passthrough (safe mode)

tree -d | html            # pipe stdin: auto-detected as Markdown or plain text
git diff | html -n        # plain (preformatted), auto-detected — no mangling
cat README.md | html      # Markdown, auto-detected (fenced code / table / setext)
ls -la | html -p          # --plain/-p: force preformatted plain text
echo '# hi' | html -m -t Log   # --markdown/-m force Markdown; --title/-t page title
html access.log           # non-.md file → plain; .md/.markdown → Markdown

cat main.go | html        # plain code is auto syntax-highlighted (chroma detect)
html data.json            # files highlight by extension (.go/.json/.py/…)
git diff --color | html   # ANSI colors preserved as styled spans
git diff --color | html --frame   # --frame: wrap plain/ANSI output in a faux terminal-window "screenshot"
cat x.go | html -l go     # --lang/-l forces a highlight language; -l text = raw
```

Module path: `github.com/dotcommander/html` · Go 1.26.x.

## Test / Lint

```bash
go test ./...                                   # all tests
go test -run TestRender_Smoke ./internal/render/   # single test
go vet ./...
```

No `.golangci.yml` or Makefile exists. All tests use `t.Parallel()` (top-level and subtests), `t.TempDir()`, and `t.Cleanup()` — never `defer`. No `testing.Short()` gates, no build tags, no `testdata/` golden files.

**Test gotcha:** tests write to the *real* `~/.config/html/cache/`, not a temp cache dir. Each test uses a unique temp source file so its `sha256` cache key is distinct, and registers a `t.Cleanup` to delete its cache entry. A crashed test can leave a stale cache file that makes a later `Fresh()` check skip a render — clear `~/.config/html/cache/` if tests behave inconsistently.

## Architecture

Linear pipeline, one cobra command, no config. A Markdown file flows:

```
cmd/html/main.go            entrypoint (<20 lines), prints errors with "html:" prefix
  └─ internal/cli/root.go   cobra wiring, parses --no-open/--force, always prints cache path to stdout
       └─ internal/actions/run.go   orchestration (stat → cache check → render → open)
            ├─ internal/cache/cache.go    cache key, freshness, atomic write
            └─ internal/render/
                 ├─ render.go   goldmark GFM pipeline + h1 title extraction
                 ├─ page.go     chroma CSS generation + HTML5 wrapper
                 └─ embed.go    //go:embed assets/base.css + assets/copy.js
```

**`actions.Run`** is the orchestrator: `os.Stat` validates the source; `cache.Fresh` compares mtimes; if stale or `--force`, it reads the source through `io.LimitReader(f, 32<<20)` (32 MiB cap — never `io.ReadAll` unbounded), calls `render.Render`, and `cache.Write`. Then `exec.Command("open", path).Start()` (fire-and-forget) unless `--no-open`. The fallback title is the source basename without extension.

**`render.Render(src, fallbackTitle)`** uses a package-level `goldmark` singleton (`render.go`) built once at init. Then `wrapPage` inlines everything — `baseCSS()` + generated chroma CSS into one `<style>`, `copyJS()` into one `<script>` — producing a zero-external-resource document.

### Load-bearing design decisions (and where they live)

- **Input: file or stdin** — `actions.Run` branches on `Options.Stdin`: a file is validated with `os.Stat`, mode-selected by extension (`.md`/`.markdown` → Markdown, every other extension → plain), and served from the mtime cache fast-path when fresh; piped stdin is always read first (bounded by the same 32 MiB `io.LimitReader` cap), content-type **auto-detected** (`render.Detect`), and cached by `sha256(content)` (`cache.PathForContent`/`FreshContent`/`WriteContent`) — there is no mtime, so the content hash *is* the key. `--plain`/`--markdown` override detection; `--title`/`-t` sets the stdin page title (default `"stdin"`). Empty stdin and a non-piped invocation with no file argument both error.
- **Robust input-type detection** — `render.Detect` (`detect.go`) classifies a bounded scan window (64 KiB / 256 lines) as binary, Markdown, or plain. **Binary input is refused** (a NUL byte, or >10% non-text bytes) so a piped image/executable never renders as garbage — no flag overrides this. **Markdown requires a high-confidence structural signal** (a fenced code block, a GFM table, a GFM task list, a setext heading, or an ATX heading followed by a blank line/EOF); weak inline cues (a `# comment` line, `__dunder__`, backticks, `arr[i](x)` reading as a link) are intentionally *not* decisive, so scripts, diffs, JSON/YAML, and logs stay plain. Trade-off: prose with only inline Markdown cues renders plain unless you pass `-m`.
- **Plain render mode** — `render.Render` with `Options.Plain` set bypasses goldmark entirely (`plain.go`) and picks the most faithful body, all reusing `wrapPage` (theme toggle, width override, copy button): (1) ANSI-colored input → `renderANSI` (`ansi.go`) converts SGR sequences to inline-styled `<span>`s so `git diff --color`/`tree -C` keep their colors; (2) otherwise a chroma lexer is chosen via `pickLexer` (explicit `Options.Lang`, then `lexers.Match` on `Options.SourceName` for files, then bounded `lexers.Analyse` of the content for stdin) and the source is syntax-highlighted with the same class-based formatter as Markdown code blocks — so the existing `highlightCSS` styles it, no new CSS; (3) otherwise raw HTML-escaped `<pre><code class="language-plaintext">`. `Lang` of `text`/`none`/`plain` forces raw. The mode + `Lang` fold into the fingerprint via `cacheTag` (`+plain`, `+lang=`); because highlighting/ANSI changed renderer behavior the asset bytes don't capture, `renderSchemaVersion` was bumped to `2`.
- **Terminal-window frame** — `--frame` (opt-in, plain-path only) wraps the plain/ANSI body in faux terminal chrome (title bar + traffic-light dots) via `terminalFrame` (`page.go`), injecting `assets/frame.css` into `wrapPage` *only when* `render.Options.Frame` is set. It **implies plain rendering** — `actions.RunWithResult` forces `Plain=true`, and the CLI rejects `--frame` with `--markdown` or any report flag. It is output-affecting, so it folds into the fingerprint as `+frame`. The Markdown path stays byte-identical because the frame markup is gated on `opts.Frame`, which is only ever true on the plain path; no `renderSchemaVersion` bump was needed — the new `frame.css` asset already busts the fingerprint once.
- **Atomic cache writes** — `cache.Write` writes to a temp file in the cache dir, then `os.Rename`s into place; a concurrent reader never sees a partial file. Cache dir is `~/.config/html/cache/` (deliberately *not* `os.UserCacheDir()`).
- **Cache key = `sha256(EvalSymlinks(Abs(path)))`** in `cache.PathFor` — symlinks and `../`-relative spellings of the same file collapse to one key. Falls back to `Abs` if `EvalSymlinks` errors. Stdin sources are keyed by `sha256(content)` (`cache.PathForContent`) instead — identical piped output reuses one entry.
- **Freshness = `cacheMtime >= srcMtime`** in `cache.Fresh`; a missing cache file returns `false, nil` (not an error).
- **Class-based syntax highlighting** — chroma is configured with `WithClasses(true)`, not inline styles, so themes are switchable via CSS. `page.go` generates CSS for both `github` (light) and `github-dark` themes; the dark CSS is wrapped in `@media (prefers-color-scheme: dark)` and the whole thing is memoized with `sync.OnceValue` (generated once per process).
- **GFM enabled** via `extension.GFM` — bundles Tables, Strikethrough, TaskList, and Linkify together; to toggle one you must decompose GFM into its constituent extensions.
- **Raw HTML passthrough** — enabled by default for local trusted input, but can be disabled with `--safe` (which builds goldmark without `goldmarkhtml.WithUnsafe()`, see comment at `render.go`). Do not point this tool at untrusted Markdown unless `--safe`.
- **Image inlining + self-containment boundary** — `imageInliner` (`images.go`) is a goldmark AST transformer (`render.go:39`) that rewrites local `![](./img)` destinations to base64 `data:` URIs, so a *file*-rendered document carries its images inline (≤10 MiB each; remote/`data:`/unknown-type/missing/oversize refs are left untouched — inlining never fails a render). "Self-contained / zero-external-resource" scopes to **render-time resources** — CSS, JS, and these images load with zero network requests; it deliberately does **not** rewrite hyperlinks: a relative `[](./page)` stays as authored `href="./page"` (a navigation target, not a loaded resource — dead-on-click when opened from the cache dir, since a linked page can't be embedded). Two boundaries follow: stdin Markdown has no base directory (`Options.SourceDir == ""`), so its local image refs stay external (inlining is skipped, `run.go`); and a referenced image's bytes/size/presence feed `ImageDependencyFingerprint` so editing or adding the file invalidates the cache.
- **Title extraction** re-parses the source AST to find the first `<h1>`, collecting only `*ast.Text` children (inline HTML/emphasis inside the heading is dropped). This is a *second* parse — `md.Convert` already parsed once; the first AST is discarded.

### Gotchas

- **Assets are compiled in** via `//go:embed` (`embed.go`). Editing `assets/base.css` or `assets/copy.js` requires a rebuild to take effect — no runtime override.
- **Cross-platform launcher** — selected by `runtime.GOOS` (`open` on macOS, `start` on Windows, `xdg-open` with `open` fallback elsewhere); `open_command` config can override this selection.
- **`goldmark-highlighting/v2` is pinned to a pseudo-version** (`v2.0.0-2023...`) — that commit is the only published version; treat it as frozen.
- **Chroma theme names are hardcoded strings** (`styles.Get("github")` / `"github-dark"` in `page.go`). A `nil` from a renamed/missing style would panic in `WriteCSS`; revalidate these names when bumping chroma.
- **Editing any embedded asset invalidates the cache.** `render.Fingerprint()` (`fingerprint.go`) hashes a schema version + every file in `assets/` + the generated highlight CSS; `cache.Fresh` compares it via a `<hash>.fp` sidecar, so changing `base.css`/`copy.js`/`theme.js`/`headings.js`/`frame.css` forces a re-render even when the source mtime is unchanged. Bump `renderSchemaVersion` when changing renderer logic (e.g. `wrapPage` markup) that the asset bytes don't capture.

### Optional config file

`~/.config/html/config.json` (loaded by `internal/config`). **Missing file = current behavior** (zero `Config`, no error); a malformed file fails the command with a clear `html: config: …` error. Loaded at the CLI boundary (`internal/cli/root.go`) and threaded through `actions.Options` → `render.Options` / `open.Open`. All fields optional:

```json
{
  "open_command": "firefox",   // launcher command; "" = OS default (open/xdg-open/start)
  "max_width": "60rem",        // reader column CSS max-width (CSS length: 48rem, 800px, 90%…)
  "default_theme": "dark",     // "light" | "dark" | "auto" ("" = auto/system)
  "toc": true                  // override the automatic 4+-heading TOC; omit = automatic
}
```

Output-affecting fields (`max_width`, `default_theme`, `toc`) are folded into the cache fingerprint via `render.Options.cacheTag`, so a config change re-renders. `open_command` does not affect output and is intentionally excluded from the tag. Per the workspace rule (Go reads config, does not contain config), any new behavioral knob belongs here, not hardcoded.
