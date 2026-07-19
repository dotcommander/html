package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	htmlpkg "html"
	"strconv"
	"strings"

	"github.com/dotcommander/html/internal/report"
)

func rawPre(src []byte) string {
	if reANSI.Match(src) {
		return renderANSI(src)
	}
	clean := reANSI.ReplaceAll(src, nil)
	return `<pre><code class="language-plaintext">` + htmlpkg.EscapeString(string(clean)) + `</code></pre>`
}

func textView(src []byte) string {
	clean := reANSI.ReplaceAll(src, nil)
	return textOverview(clean) + textPre(src)
}

func textPre(src []byte) string {
	if reANSI.Match(src) {
		return renderANSI(src)
	}
	clean := reANSI.ReplaceAll(src, nil)
	return `<pre class="report-text"><code class="language-plaintext">` + htmlpkg.EscapeString(string(clean)) + `</code></pre>`
}

func textOverview(src []byte) string {
	text := string(src)
	words := len(strings.Fields(text))
	chars := len([]rune(text))
	items := [][2]string{
		{"Lines", strconv.Itoa(lineCount(src))},
		{"Words", strconv.Itoa(words)},
		{"Characters", strconv.Itoa(chars)},
	}
	var b strings.Builder
	b.WriteString(`<dl class="text-overview" aria-label="Text overview">`)
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

func binaryView(src []byte, analysis report.Analysis) string {
	var b strings.Builder
	b.WriteString(binaryOverview(src, analysis))
	b.WriteString(`<pre class="binary-preview" aria-label="Binary byte preview"><code>`)
	for i := 0; i < len(src) && i < 128; i += 16 {
		end := i + 16
		if end > len(src) {
			end = len(src)
		}
		fmt.Fprintf(&b, `%08x  %-47s  |%s|`, i, hexBytes(src[i:end]), asciiBytes(src[i:end]))
		if end < len(src) && end < 128 {
			b.WriteByte('\n')
		}
	}
	if len(src) > 128 {
		b.WriteByte('\n')
		b.WriteString(`... `)
		b.WriteString(strconv.Itoa(len(src) - 128))
		b.WriteString(` more bytes`)
	}
	b.WriteString(`</code></pre>`)
	return b.String()
}

func binaryOverview(src []byte, analysis report.Analysis) string {
	items := [][2]string{
		{"Bytes", strconv.Itoa(len(src))},
		{"Preview", strconv.Itoa(min(len(src), 128)) + " bytes"},
	}
	if analysis.Stats.Lines > 0 {
		items = append(items, [2]string{"Lines", strconv.Itoa(analysis.Stats.Lines)})
	}
	if len(analysis.Reasons) > 0 {
		items = append(items, [2]string{"Reason", strings.Join(analysis.Reasons, ", ")})
	}
	var b strings.Builder
	b.WriteString(`<dl class="binary-overview" aria-label="Binary overview">`)
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

func hexBytes(src []byte) string {
	var b strings.Builder
	for i, by := range src {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, `%02x`, by)
	}
	return b.String()
}

func asciiBytes(src []byte) string {
	var b strings.Builder
	for _, by := range src {
		if by >= 0x20 && by <= 0x7e {
			b.WriteByte(by)
			continue
		}
		b.WriteByte('.')
	}
	return b.String()
}

func jsonView(src []byte, analysis report.Analysis) string {
	var pretty bytes.Buffer
	body := src
	if err := json.Indent(&pretty, stripUTF8BOM(src), "", "  "); err == nil {
		body = pretty.Bytes()
	}
	overview := jsonOverview(analysis.Data)
	if overview == "" {
		return jsonPre(body)
	}
	return overview + jsonPre(body)
}

func jsonPre(src []byte) string {
	clean := reANSI.ReplaceAll(src, nil)
	return `<pre class="json-source"><code class="language-json">` + htmlpkg.EscapeString(string(clean)) + `</code></pre>`
}

func jsonOverview(data any) string {
	switch v := data.(type) {
	case map[string]any:
		keysMap := make(map[string]bool, len(v))
		for k := range v {
			keysMap[k] = true
		}
		keys := sortedStringKeys(keysMap)
		if len(keys) == 0 {
			return `<div class="json-overview" aria-label="JSON overview"><span>empty object</span></div>`
		}
		var b strings.Builder
		b.WriteString(`<dl class="json-overview" aria-label="JSON overview">`)
		for _, key := range keys {
			b.WriteString(`<div><dt>`)
			b.WriteString(htmlpkg.EscapeString(key))
			b.WriteString(`</dt><dd>`)
			b.WriteString(htmlpkg.EscapeString(jsonValueLabel(v[key])))
			b.WriteString(`</dd></div>`)
		}
		b.WriteString(`</dl>`)
		return b.String()
	case []any:
		return fmt.Sprintf(`<div class="json-overview" aria-label="JSON overview"><span><strong>%d</strong> %s</span><span>%s</span></div>`, len(v), plural(len(v), "item", "items"), htmlpkg.EscapeString(jsonValueLabel(v)))
	default:
		if data == nil {
			return ""
		}
		return `<div class="json-overview" aria-label="JSON overview"><span>` + htmlpkg.EscapeString(jsonValueLabel(data)) + `</span></div>`
	}
}

func jsonValueLabel(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number:
		return "number"
	case []any:
		return fmt.Sprintf("array (%d)", len(x))
	case map[string]any:
		return fmt.Sprintf("object (%d)", len(x))
	default:
		return "value"
	}
}
