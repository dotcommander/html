package render

import (
	"fmt"
	htmlpkg "html"
	"regexp"
	"strconv"
	"strings"
)

func logView(src []byte) string {
	lines := strings.Split(string(reANSI.ReplaceAll(src, nil)), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return rawPre(src)
	}
	counts := logSeverityCounts{}
	for _, line := range lines {
		counts.add(logSeverity(line))
	}
	var b strings.Builder
	b.WriteString(logOverview(counts))
	b.WriteString(`<ol class="log-lines">`)
	for _, line := range lines {
		text := htmlpkg.EscapeString(line)
		severity := logSeverity(line)
		if severity == "" {
			b.WriteString(`<li class="log-line"><span class="log-message">`)
			b.WriteString(text)
			b.WriteString(`</span></li>`)
			continue
		}
		fmt.Fprintf(&b, `<li class="log-line log-%s"><span class="log-level">%s</span><span class="log-message">%s</span></li>`, severity, strings.ToUpper(severity), text)
	}
	b.WriteString(`</ol>`)
	return b.String()
}

type logSeverityCounts struct {
	Lines int
	Debug int
	Info  int
	Warn  int
	Error int
	Fatal int
}

func (c *logSeverityCounts) add(severity string) {
	c.Lines++
	switch severity {
	case "debug":
		c.Debug++
	case "info":
		c.Info++
	case "warn":
		c.Warn++
	case "error":
		c.Error++
	case "fatal":
		c.Fatal++
	}
}

func logOverview(counts logSeverityCounts) string {
	items := [][2]string{{"Lines", strconv.Itoa(counts.Lines)}}
	for _, item := range []struct {
		label string
		count int
	}{
		{"Errors", counts.Error},
		{"Warnings", counts.Warn},
		{"Info", counts.Info},
		{"Debug", counts.Debug},
		{"Fatal", counts.Fatal},
	} {
		if item.count > 0 {
			items = append(items, [2]string{item.label, strconv.Itoa(item.count)})
		}
	}
	var b strings.Builder
	b.WriteString(`<dl class="log-overview" aria-label="Log overview">`)
	for _, item := range items {
		b.WriteString(`<div><dt>`)
		b.WriteString(htmlpkg.EscapeString(item[0]))
		b.WriteString(`</dt><dd>`)
		b.WriteString(htmlpkg.EscapeString(item[1]))
		b.WriteString(`</dd></div>`)
	}
	b.WriteString(`</dl>`)
	return b.String()
}

func logSeverity(line string) string {
	upper := strings.ToUpper(line)
	switch {
	case strings.Contains(upper, "FATAL") || strings.Contains(upper, "PANIC:"):
		return "fatal"
	case strings.Contains(upper, "ERROR") || strings.HasPrefix(upper, "FAIL") || strings.Contains(upper, "--- FAIL:"):
		return "error"
	case strings.Contains(upper, "WARN"):
		return "warn"
	case strings.Contains(upper, "INFO") || strings.HasPrefix(upper, "PASS") || strings.HasPrefix(upper, "OK\t"):
		return "info"
	case strings.Contains(upper, "DEBUG"):
		return "debug"
	default:
		if status, ok := accessLogStatus(line); ok {
			switch {
			case status >= 500:
				return "error"
			case status >= 400:
				return "warn"
			case status >= 200:
				return "info"
			}
		}
		return ""
	}
}

var accessLogStatusRe = regexp.MustCompile(`"\s+([1-5][0-9]{2})\b`)

func accessLogStatus(line string) (int, bool) {
	m := accessLogStatusRe.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	status, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return status, true
}

type transcriptTurn struct {
	Speaker string
	Text    []string
}

func transcriptView(src []byte) string {
	turns := transcriptTurns(src)
	if len(turns) == 0 {
		return rawPre(src)
	}
	speakers := map[string]bool{}
	for _, turn := range turns {
		speakers[turn.Speaker] = true
	}
	var b strings.Builder
	b.WriteString(transcriptOverview(len(turns), len(speakers)))
	b.WriteString(`<ol class="transcript-turns">`)
	for _, turn := range turns {
		b.WriteString(`<li class="transcript-turn"><span class="transcript-speaker">`)
		b.WriteString(htmlpkg.EscapeString(turn.Speaker))
		b.WriteString(`</span><div class="transcript-text">`)
		for _, text := range turn.Text {
			b.WriteString(`<p>`)
			b.WriteString(htmlpkg.EscapeString(text))
			b.WriteString(`</p>`)
		}
		b.WriteString(`</div></li>`)
	}
	b.WriteString(`</ol>`)
	return b.String()
}

func transcriptOverview(turns, speakers int) string {
	items := [][2]string{
		{"Turns", strconv.Itoa(turns)},
		{"Speakers", strconv.Itoa(speakers)},
	}
	var b strings.Builder
	b.WriteString(`<dl class="transcript-overview" aria-label="Transcript overview">`)
	for _, item := range items {
		b.WriteString(`<div><dt>`)
		b.WriteString(htmlpkg.EscapeString(item[0]))
		b.WriteString(`</dt><dd>`)
		b.WriteString(htmlpkg.EscapeString(item[1]))
		b.WriteString(`</dd></div>`)
	}
	b.WriteString(`</dl>`)
	return b.String()
}

func transcriptTurns(src []byte) []transcriptTurn {
	lines := strings.Split(string(reANSI.ReplaceAll(src, nil)), "\n")
	turns := make([]transcriptTurn, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		speaker, text, ok := transcriptLine(line)
		if ok {
			turns = append(turns, transcriptTurn{Speaker: speaker, Text: []string{text}})
			continue
		}
		if len(turns) > 0 {
			turns[len(turns)-1].Text = append(turns[len(turns)-1].Text, line)
		}
	}
	return turns
}

func transcriptLine(line string) (speaker, text string, ok bool) {
	speaker, text, ok = strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	speaker = strings.TrimSpace(speaker)
	text = strings.TrimSpace(text)
	if speaker == "" || text == "" || len(speaker) > 48 || strings.ContainsAny(speaker, "{}[]=/\\") {
		return "", "", false
	}
	return speaker, text, true
}

func escapeTableText(text string) string {
	return htmlpkg.EscapeString(cleanTableText(text))
}

func cleanTableText(text string) string {
	return string(reANSI.ReplaceAll([]byte(text), nil))
}
