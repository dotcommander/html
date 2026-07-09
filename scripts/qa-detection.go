//go:build ignore

package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotcommander/html/internal/render"
)

type detectCase struct {
	Name     string
	Category string
	Input    []byte
	Want     render.Kind
	Reason   string
}

type detectResult struct {
	Case detectCase
	Got  render.Kind
}

func main() {
	root, err := repoRoot()
	check(err)
	outDir := filepath.Join(root, ".work/html-qa/detection")
	check(os.RemoveAll(outDir))
	check(os.MkdirAll(outDir, 0o755))

	var results []detectResult
	for _, c := range detectCases() {
		got := render.Detect(c.Input)
		require(got == c.Want, "%s detected as %s, want %s", c.Name, kindName(got), kindName(c.Want))
		results = append(results, detectResult{Case: c, Got: got})
	}

	indexPath := filepath.Join(outDir, "index.html")
	check(os.WriteFile(indexPath, []byte(renderDetectionIndex(results)), 0o644))
	fmt.Println(indexPath)
}

func detectCases() []detectCase {
	lateHeading := strings.Repeat("plain line\n", 260) + "# Late Heading\n\nbody\n"
	return []detectCase{
		{Name: "NUL Byte", Category: "Binary", Input: []byte("abc\x00def"), Want: render.KindBinary, Reason: "A NUL byte is always binary."},
		{Name: "PNG Header", Category: "Binary", Input: []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0xff}, Want: render.KindBinary, Reason: "NUL byte refuses binary input."},
		{Name: "Control Blob", Category: "Binary", Input: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 'a'}, Want: render.KindBinary, Reason: "Too many non-text control bytes."},
		{Name: "Single Bell Text", Category: "Plain", Input: []byte("hello\a world with mostly text\n"), Want: render.KindPlain, Reason: "One stray control byte does not make otherwise textual input binary."},
		{Name: "Fenced Markdown", Category: "Markdown", Input: []byte("# Title\n\n```go\nfmt.Println(\"html\")\n```\n"), Want: render.KindMarkdown, Reason: "Fence is a strong Markdown signal."},
		{Name: "CRLF ATX Heading", Category: "Markdown", Input: []byte("# Title\r\n\r\nbody text\r\n"), Want: render.KindMarkdown, Reason: "Windows line endings still preserve the ATX heading signal."},
		{Name: "Tilde Fence", Category: "Markdown", Input: []byte("~~~\ncode block\n~~~\n"), Want: render.KindMarkdown, Reason: "Tilde fence is a strong Markdown signal."},
		{Name: "Indented Fence", Category: "Markdown", Input: []byte("   ```sh\necho hi\n```\n"), Want: render.KindMarkdown, Reason: "Up to three spaces before a fence are valid Markdown."},
		{Name: "GFM Table", Category: "Markdown", Input: []byte("| a | b |\n|---|---|\n| 1 | 2 |\n"), Want: render.KindMarkdown, Reason: "Delimiter row under a pipe header."},
		{Name: "GFM Table No Edge Pipes", Category: "Markdown", Input: []byte("a | b\n--- | ---\n1 | 2\n"), Want: render.KindMarkdown, Reason: "GFM tables do not require leading or trailing pipes."},
		{Name: "Task List", Category: "Markdown", Input: []byte("- [x] ship renderer\n- [ ] verify screenshots\n"), Want: render.KindMarkdown, Reason: "GFM task markers are specific enough."},
		{Name: "Multi-line Blockquote", Category: "Markdown", Input: []byte("> Keep generated pages self-contained.\n> Verify browser output.\n"), Want: render.KindMarkdown, Reason: "Adjacent blockquote lines are a high-confidence Markdown block."},
		{Name: "ATX Heading", Category: "Markdown", Input: []byte("# Title\n\nbody text\n"), Want: render.KindMarkdown, Reason: "Heading followed by blank line."},
		{Name: "Setext Heading", Category: "Markdown", Input: []byte("Title\n=====\n\nbody text\n"), Want: render.KindMarkdown, Reason: "Equals underline is unambiguous."},
		{Name: "Setext Dash Heading", Category: "Markdown", Input: []byte("Title\n-----\n\nbody text\n"), Want: render.KindMarkdown, Reason: "Dash underline with blank separation is Markdown."},
		{Name: "Tree Output", Category: "Plain", Input: []byte(".\n├── cmd\n│   └── html\n└── internal\n"), Want: render.KindPlain, Reason: "Command output must stay literal."},
		{Name: "LS Output", Category: "Plain", Input: []byte("total 8\ndrwxr-xr-x  4 u g 128 .\n-rw-r--r--  1 u g 10 a.txt\n"), Want: render.KindPlain, Reason: "Directory listings must stay literal."},
		{Name: "Git Diff", Category: "Plain", Input: []byte("diff --git a/x b/x\n@@ -1,2 +1,2 @@\n-old line\n+new line\n"), Want: render.KindPlain, Reason: "Diff markers are not Markdown."},
		{Name: "Git Log", Category: "Plain", Input: []byte("commit abc123\nAuthor: A <a@b.c>\n\n    message body\n"), Want: render.KindPlain, Reason: "Indented commit bodies are command output, not Markdown."},
		{Name: "JSON", Category: "Plain", Input: []byte("{\n  \"a\": 1,\n  \"b\": [2, 3]\n}\n"), Want: render.KindPlain, Reason: "Structured data renders as plain unless report mode is requested."},
		{Name: "YAML", Category: "Plain", Input: []byte("name: html\nitems:\n  - render\n  - verify\n"), Want: render.KindPlain, Reason: "YAML bullets do not imply Markdown."},
		{Name: "YAML Separator", Category: "Plain", Input: []byte("---\nname: html\nitems: [render, verify]\n"), Want: render.KindPlain, Reason: "YAML document separators are not Markdown dividers."},
		{Name: "TOML Array", Category: "Plain", Input: []byte("name = \"html\"\nitems = [\"render\", \"verify\"]\n"), Want: render.KindPlain, Reason: "Config arrays are not Markdown links or lists."},
		{Name: "CSV", Category: "Plain", Input: []byte("a,b,c\n1,2,3\n4,5,6\n"), Want: render.KindPlain, Reason: "CSV renders as plain unless report mode is requested."},
		{Name: "Aligned Columns", Category: "Plain", Input: []byte("NAME      PID   CPU\napi       123   4.5\nworker    456   0.1\n"), Want: render.KindPlain, Reason: "Columnar command output is plain document input; rendering may upgrade it to an HTML table."},
		{Name: "Pipe Prose Without Delimiter", Category: "Plain", Input: []byte("name | status\nalpha | ready\nbeta | review\n"), Want: render.KindPlain, Reason: "Pipe-separated text without a delimiter row is not a Markdown table."},
		{Name: "Go Source", Category: "Plain", Input: []byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hi\") }\n"), Want: render.KindPlain, Reason: "Source code stays plain in ordinary document mode."},
		{Name: "Python Dunder Comment", Category: "Plain", Input: []byte("# module\nclass A:\n    def __init__(self):\n        pass\n"), Want: render.KindPlain, Reason: "Hash comments must not become headings."},
		{Name: "Shell Backticks", Category: "Plain", Input: []byte("#!/bin/bash\n# build it\nout=`date`\necho $out\n"), Want: render.KindPlain, Reason: "Inline Markdown-like cues are weak."},
		{Name: "JS Bracket Call", Category: "Plain", Input: []byte("// handlers\nhandlers[name](req)\n"), Want: render.KindPlain, Reason: "Bracket-call syntax must not become a Markdown link."},
		{Name: "HTML Document", Category: "Plain", Input: []byte("<!doctype html>\n<html><body>hi</body></html>\n"), Want: render.KindPlain, Reason: "Raw HTML document is shown as text unless forced."},
		{Name: "Link And Emphasis Only", Category: "Plain", Input: []byte("see [docs](http://x) and **bold** text here\n"), Want: render.KindPlain, Reason: "Inline cues alone are intentionally insufficient."},
		{Name: "Greater Than Comparator", Category: "Plain", Input: []byte("value > threshold\ncount > 10\n"), Want: render.KindPlain, Reason: "Comparator-looking text is not a Markdown blockquote."},
		{Name: "Dash Divider", Category: "Plain", Input: []byte("Results\n--------------------\nrow 1\nrow 2\n"), Want: render.KindPlain, Reason: "Command-output divider stays plain without blank separation."},
		{Name: "MySQL Box Table", Category: "Plain", Input: []byte("+----+----+\n| a  | b  |\n+----+----+\n"), Want: render.KindPlain, Reason: "Box tables are plain in document mode."},
		{Name: "Changelog Bullets", Category: "Plain", Input: []byte("- fixed a bug\n- added a feature\n- removed cruft\n"), Want: render.KindPlain, Reason: "Bullets without task markers are not decisive."},
		{Name: "Log Hash Line", Category: "Plain", Input: []byte("# starting server on :8080\nlistening\n"), Want: render.KindPlain, Reason: "Hash-prefixed logs must not become headings."},
		{Name: "ANSI Colored", Category: "Plain", Input: []byte("\x1b[01;34mdir\x1b[0m\nfile\n"), Want: render.KindPlain, Reason: "ANSI escape is text and later preserved by plain rendering."},
		{Name: "Late Markdown Beyond Scan", Category: "Plain", Input: []byte(lateHeading), Want: render.KindPlain, Reason: "Detection is intentionally bounded to the first scan window."},
		{Name: "Empty Input", Category: "Plain", Input: []byte(""), Want: render.KindPlain, Reason: "Empty input has no Markdown signal."},
	}
}

func renderDetectionIndex(results []detectResult) string {
	counts := map[render.Kind]int{}
	var rows strings.Builder
	for _, r := range results {
		counts[r.Got]++
		fmt.Fprintf(&rows, `<tr data-kind="%s"><td><strong>%s</strong><span>%s</span></td><td>%s</td><td>%s</td><td><code>%s</code></td><td>%s</td></tr>`,
			html.EscapeString(kindName(r.Got)),
			html.EscapeString(r.Case.Name),
			html.EscapeString(r.Case.Category),
			html.EscapeString(kindName(r.Case.Want)),
			html.EscapeString(kindName(r.Got)),
			html.EscapeString(sample(r.Case.Input)),
			html.EscapeString(r.Case.Reason),
		)
	}
	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>HTML Detection Matrix</title>
  <style>
    :root { --bg:#f5f4f1; --paper:#fffefa; --text:#25221f; --muted:#6d6760; --border:#ded8cc; --accent:#8b5b2f; --panel:#f1ece3; }
    * { box-sizing: border-box; }
    body { margin:0; color:var(--text); background:var(--bg); font-family:ui-sans-serif,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif; }
    main { width:min(calc(100% - 2rem),80rem); margin:1.25rem auto 2rem; }
    h1 { margin:0 0 .3rem; font-size:1.55rem; line-height:1.15; }
    p { margin:0; color:var(--muted); line-height:1.45; }
    .summary { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,12rem),1fr)); gap:.65rem; margin:1rem 0; }
    .summary div { padding:.75rem; background:var(--paper); border:1px solid var(--border); border-radius:8px; box-shadow:0 .7rem 2rem rgba(35,30,24,.06); }
    .summary dt,.summary dd { margin:0; }
    .summary dt { color:var(--muted); font-size:.75rem; font-weight:800; text-transform:uppercase; }
    .summary dd { margin-top:.2rem; font-size:1.45rem; font-weight:850; }
    .table-wrap { overflow:auto; background:var(--paper); border:1px solid var(--border); border-radius:8px; box-shadow:0 .9rem 2.4rem rgba(35,30,24,.07); }
    table { width:100%; border-collapse:collapse; min-width:58rem; }
    th,td { padding:.65rem .75rem; border-bottom:1px solid var(--border); text-align:left; vertical-align:top; font-size:.86rem; }
    th { position:sticky; top:0; background:var(--panel); color:var(--muted); font-size:.74rem; text-transform:uppercase; letter-spacing:.02em; }
    tr:last-child td { border-bottom:0; }
    td:first-child { width:13rem; }
    td:first-child strong,td:first-child span { display:block; }
    td:first-child span { margin-top:.12rem; color:var(--muted); font-size:.75rem; }
    code { display:block; max-width:28rem; white-space:pre-wrap; word-break:break-word; font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace; font-size:.78rem; color:#4f453c; }
    [data-kind="markdown"] td:nth-child(3),[data-kind="plain"] td:nth-child(3),[data-kind="binary"] td:nth-child(3) { font-weight:850; color:#2d7a46; }
    @media (max-width:45rem) {
      main { width:100%; margin:0; padding:1rem 0 1.25rem; }
      h1,p,.summary { padding-inline:1rem; }
      .summary { grid-template-columns:repeat(3,minmax(0,1fr)); gap:.45rem; }
      .summary div { padding:.6rem; }
      .summary dd { font-size:1.15rem; }
      .table-wrap { border-left:0; border-right:0; border-radius:0; }
    }
  </style>
</head>
<body>
  <main>
    <h1>HTML Detection Matrix</h1>
    <p>Bounded detector QA for ordinary document generation: binary is refused, strong Markdown structures render as Markdown, and ambiguous code/config/log text stays plain.</p>
    <dl class="summary">
      <div><dt>Cases</dt><dd>` + fmt.Sprint(len(results)) + `</dd></div>
      <div><dt>Markdown</dt><dd>` + fmt.Sprint(counts[render.KindMarkdown]) + `</dd></div>
      <div><dt>Plain</dt><dd>` + fmt.Sprint(counts[render.KindPlain]) + `</dd></div>
      <div><dt>Binary</dt><dd>` + fmt.Sprint(counts[render.KindBinary]) + `</dd></div>
    </dl>
    <div class="table-wrap">
      <table>
        <thead><tr><th>Case</th><th>Want</th><th>Got</th><th>Input Sample</th><th>Reason</th></tr></thead>
        <tbody>` + rows.String() + `</tbody>
      </table>
    </div>
  </main>
</body>
</html>
`
}

func sample(src []byte) string {
	const limit = 160
	var b strings.Builder
	for _, c := range src {
		switch {
		case c == 0x1b:
			b.WriteString("ESC")
		case c == '\n' || c == '\r' || c == '\t':
			b.WriteByte(c)
		case c < 0x20 || c == 0x7f:
			fmt.Fprintf(&b, "\\x%02x", c)
		default:
			b.WriteByte(c)
		}
	}
	s := b.String()
	s = strings.TrimSpace(s)
	if len(s) > limit {
		s = s[:limit] + "..."
	}
	return s
}

func kindName(k render.Kind) string {
	switch k {
	case render.KindMarkdown:
		return "markdown"
	case render.KindBinary:
		return "binary"
	default:
		return "plain"
	}
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above working directory")
		}
		dir = parent
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func require(ok bool, format string, args ...any) {
	if !ok {
		panic(fmt.Sprintf(format, args...))
	}
}
