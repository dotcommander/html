package render

import (
	"strings"
	"testing"
)

func TestRenderANSI(t *testing.T) {
	t.Parallel()
	in := []byte("plain \x1b[31mred\x1b[0m \x1b[1;32mbold green\x1b[0m │end\n")
	out := renderANSI(in)
	if !strings.HasPrefix(out, `<pre><code class="language-ansi">`) {
		t.Errorf("expected ansi pre/code wrapper, got:\n%s", out)
	}
	if !strings.Contains(out, "color:#aa0000") {
		t.Errorf("expected red foreground span, got:\n%s", out)
	}
	if !strings.Contains(out, "color:#00aa00") || !strings.Contains(out, "font-weight:bold") {
		t.Errorf("expected bold green span, got:\n%s", out)
	}
	if !strings.Contains(out, "red") || !strings.Contains(out, "bold green") {
		t.Errorf("expected text preserved")
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("raw escape leaked into output")
	}
	if !strings.Contains(out, "│end") {
		t.Errorf("expected multibyte char preserved intact")
	}
}

func TestRenderANSI_TrueColorAnd256(t *testing.T) {
	t.Parallel()
	out := renderANSI([]byte("\x1b[38;2;255;128;0morange\x1b[0m\x1b[38;5;9mbright\x1b[0m\n"))
	if !strings.Contains(out, "color:#ff8000") {
		t.Errorf("expected truecolor span, got:\n%s", out)
	}
	if !strings.Contains(out, "color:#ff5555") {
		t.Errorf("expected 256-color index 9 -> bright red, got:\n%s", out)
	}
}
