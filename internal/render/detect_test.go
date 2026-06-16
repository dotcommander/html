package render

import "testing"

func TestDetect(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want Kind
	}{
		// Binary — refused.
		{"nul byte", "abc\x00def", KindBinary},
		{"png header", "\x89PNG\r\n\x1a\n\x00\x00\x00", KindBinary},
		{"control blob", string([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 'a'}), KindBinary},
		// Plain — command output, data, code, config: must never be Markdown.
		{"tree output", ".\n├── cmd\n│   └── html\n└── internal\n", KindPlain},
		{"ls -la", "total 8\ndrwxr-xr-x  4 u g 128 .\n-rw-r--r--  1 u g 10 a.txt\n", KindPlain},
		{"git diff", "diff --git a/x b/x\n@@ -1,2 +1,2 @@\n-old line\n+new line\n", KindPlain},
		{"git log", "commit abc123\nAuthor: A <a@b.c>\n\n    message body\n", KindPlain},
		{"json", "{\n  \"a\": 1,\n  \"b\": [2, 3]\n}\n", KindPlain},
		{"yaml", "name: x\nitems:\n  - a\n  - b\n", KindPlain},
		{"csv", "a,b,c\n1,2,3\n4,5,6\n", KindPlain},
		{"go source", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hi\") }\n", KindPlain},
		{"python dunder+comment", "# module\nclass A:\n    def __init__(self):\n        pass\n", KindPlain},
		{"shell backtick+comment", "#!/bin/bash\n# build it\nout=`date`\necho $out\n", KindPlain},
		{"js bracket-call+comment", "// handlers\nhandlers[name](req)\n", KindPlain},
		{"prose paragraph", "This is a plain paragraph of text with no markup at all.\n", KindPlain},
		{"html doc", "<!doctype html>\n<html><body>hi</body></html>\n", KindPlain},
		{"ansi colored", "\x1b[01;34mdir\x1b[0m\nfile\n", KindPlain},
		{"empty", "", KindPlain},
		{"dash divider", "Results\n--------------------\nrow 1\nrow 2\n", KindPlain},
		{"mysql box table", "+----+----+\n| a  | b  |\n+----+----+\n", KindPlain},
		{"yaml doc separator", "---\nname: x\nitems: [a, b]\n", KindPlain},
		// Robustness guards: ambiguous-but-weak cues stay plain.
		{"changelog bullets only", "- fixed a bug\n- added a feature\n- removed cruft\n", KindPlain},
		{"log hash line", "# starting server on :8080\nlistening\n", KindPlain},
		{"lone heading + prose", "# Title\n\nsome ordinary prose paragraph here\n", KindPlain},
		{"link and emphasis only", "see [docs](http://x) and **bold** text here\n", KindPlain},
		// Markdown — high-confidence structural signals.
		{"readme with fence", "# Title\n\nText.\n\n```go\nfmt.Println()\n```\n", KindMarkdown},
		{"tilde fence", "~~~\ncode block\n~~~\n", KindMarkdown},
		{"gfm table", "| a | b |\n|---|---|\n| 1 | 2 |\n", KindMarkdown},
		{"setext equals", "Title\n=====\n\nbody text\n", KindMarkdown},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Detect([]byte(tc.in)); got != tc.want {
				t.Errorf("Detect(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
