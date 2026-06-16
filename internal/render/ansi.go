package render

import (
	"fmt"
	htmlpkg "html"
	"strconv"
	"strings"
)

// ansi16 is the standard 16-color ANSI palette (xterm-style), indexed 0–15:
// 0–7 are the normal colors (SGR 30–37 / 40–47), 8–15 the bright ones (90–97 /
// 100–107).
var ansi16 = [16]string{
	"#000000", "#aa0000", "#00aa00", "#aa5500", "#0000aa", "#aa00aa", "#00aaaa", "#aaaaaa",
	"#555555", "#ff5555", "#55ff55", "#ffff55", "#5555ff", "#ff55ff", "#55ffff", "#ffffff",
}

// ansiStyle is the active SGR state while scanning ANSI input.
type ansiStyle struct {
	fg, bg                  string
	bold, italic, underline bool
}

func (s ansiStyle) empty() bool {
	return s.fg == "" && s.bg == "" && !s.bold && !s.italic && !s.underline
}

func (s ansiStyle) css() string {
	var p []string
	if s.fg != "" {
		p = append(p, "color:"+s.fg)
	}
	if s.bg != "" {
		p = append(p, "background-color:"+s.bg)
	}
	if s.bold {
		p = append(p, "font-weight:bold")
	}
	if s.italic {
		p = append(p, "font-style:italic")
	}
	if s.underline {
		p = append(p, "text-decoration:underline")
	}
	return strings.Join(p, ";")
}

// renderANSI converts ANSI/SGR-colored text into an HTML <pre><code> with inline-
// styled <span>s, so the colors of piped terminal output (git diff --color,
// tree -C, ls --color) survive into the page. Only SGR (ESC[…m) sequences are
// interpreted; any other escape sequence (cursor moves, screen clears) is
// dropped. Text between sequences is HTML-escaped as whole UTF-8 runs.
func renderANSI(src []byte) string {
	var b strings.Builder
	b.WriteString(`<pre><code class="language-ansi">`)
	var cur ansiStyle
	spanOpen := false
	flushSpan := func() {
		if spanOpen {
			b.WriteString("</span>")
			spanOpen = false
		}
	}
	writeText := func(text []byte) {
		if len(text) == 0 {
			return
		}
		if !spanOpen && !cur.empty() {
			b.WriteString(`<span style="` + cur.css() + `">`)
			spanOpen = true
		}
		b.WriteString(htmlpkg.EscapeString(string(text)))
	}
	i, runStart := 0, 0
	for i < len(src) {
		if src[i] == 0x1b && i+1 < len(src) && src[i+1] == '[' {
			writeText(src[runStart:i])
			j := i + 2
			for j < len(src) && src[j] >= 0x30 && src[j] <= 0x3f { // parameter bytes
				j++
			}
			for j < len(src) && src[j] >= 0x20 && src[j] <= 0x2f { // intermediate bytes
				j++
			}
			if j >= len(src) { // truncated escape: drop the tail
				i = len(src)
				runStart = i
				break
			}
			if src[j] == 'm' { // SGR — the only sequence we interpret
				flushSpan()
				cur = applySGR(cur, string(src[i+2:j]))
			}
			i = j + 1
			runStart = i
			continue
		}
		i++
	}
	writeText(src[runStart:i])
	flushSpan()
	b.WriteString("</code></pre>\n")
	return b.String()
}

// applySGR folds one SGR parameter list (the bytes between ESC[ and m) into the
// running style. An empty list means reset.
func applySGR(s ansiStyle, params string) ansiStyle {
	if params == "" {
		return ansiStyle{}
	}
	codes := strings.Split(params, ";")
	for k := 0; k < len(codes); k++ {
		n, err := strconv.Atoi(codes[k])
		if err != nil {
			continue
		}
		switch {
		case n == 0:
			s = ansiStyle{}
		case n == 1:
			s.bold = true
		case n == 3:
			s.italic = true
		case n == 4:
			s.underline = true
		case n == 22:
			s.bold = false
		case n == 23:
			s.italic = false
		case n == 24:
			s.underline = false
		case n >= 30 && n <= 37:
			s.fg = ansi16[n-30]
		case n == 39:
			s.fg = ""
		case n >= 40 && n <= 47:
			s.bg = ansi16[n-40]
		case n == 49:
			s.bg = ""
		case n >= 90 && n <= 97:
			s.fg = ansi16[n-90+8]
		case n >= 100 && n <= 107:
			s.bg = ansi16[n-100+8]
		case n == 38 || n == 48:
			color, used := parseExtColor(codes[k:])
			if color != "" {
				if n == 38 {
					s.fg = color
				} else {
					s.bg = color
				}
			}
			k += used
		}
	}
	return s
}

// parseExtColor parses an extended-color tail beginning at codes[0] (which is
// "38" or "48"): "5;n" (256-color) or "2;r;g;b" (truecolor). It returns the hex
// color and how many EXTRA params it consumed beyond codes[0].
func parseExtColor(codes []string) (hex string, consumed int) {
	if len(codes) < 2 {
		return "", 0
	}
	switch codes[1] {
	case "5":
		if len(codes) < 3 {
			return "", 1
		}
		n, err := strconv.Atoi(codes[2])
		if err != nil {
			return "", 2
		}
		return color256(n), 2
	case "2":
		if len(codes) < 5 {
			return "", len(codes) - 1
		}
		r, _ := strconv.Atoi(codes[2])
		g, _ := strconv.Atoi(codes[3])
		bl, _ := strconv.Atoi(codes[4])
		return fmt.Sprintf("#%02x%02x%02x", clamp8(r), clamp8(g), clamp8(bl)), 4
	}
	return "", 1
}

func clamp8(n int) int {
	switch {
	case n < 0:
		return 0
	case n > 255:
		return 255
	default:
		return n
	}
}

// color256 maps an xterm 256-color index to a hex string: 0–15 palette, 16–231
// the 6×6×6 cube, 232–255 the grayscale ramp.
func color256(n int) string {
	switch {
	case n < 0 || n > 255:
		return ""
	case n < 16:
		return ansi16[n]
	case n < 232:
		n -= 16
		level := func(v int) int {
			if v == 0 {
				return 0
			}
			return v*40 + 55
		}
		return fmt.Sprintf("#%02x%02x%02x", level(n/36), level((n/6)%6), level(n%6))
	default:
		v := (n-232)*10 + 8
		return fmt.Sprintf("#%02x%02x%02x", v, v, v)
	}
}
