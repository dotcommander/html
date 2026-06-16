package render

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	htmlpkg "html"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/dotcommander/html/internal/report"
	"github.com/yuin/goldmark/text"
)

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

func RenderReport(src []byte, opts Options, analysis report.Analysis, plan report.ReportPlan) (string, error) {
	title := reportTitle(src, opts, analysis)
	body, err := renderReportBody(src, opts, analysis, plan)
	if err != nil {
		return "", err
	}
	return wrapPage(title, body, opts), nil
}

func reportTitle(src []byte, opts Options, analysis report.Analysis) string {
	if analysis.Kind != report.KindMarkdown {
		return htmlpkg.EscapeString(opts.FallbackTitle)
	}
	md := mdUnsafe
	if opts.Safe {
		md = mdSafe
	}
	doc := md.Parser().Parse(text.NewReader(src))
	title, _, _ := analyze(doc, src, opts.FallbackTitle)
	return title
}

func renderReportBody(src []byte, opts Options, analysis report.Analysis, plan report.ReportPlan) (string, error) {
	switch plan.Layout {
	case report.LayoutTabbedPage:
		return renderTabs(src, opts, analysis, plan.Components)
	case report.LayoutSlides:
		return renderSlides(src, opts, analysis, plan.Components)
	}
	var b strings.Builder
	for _, c := range plan.Components {
		part, err := renderReportComponent(src, opts, analysis, c)
		if err != nil {
			return "", err
		}
		b.WriteString(part)
	}
	return b.String(), nil
}

func renderTabs(src []byte, opts Options, analysis report.Analysis, components []report.Component) (string, error) {
	var buttons strings.Builder
	var panels strings.Builder
	for i, c := range components {
		tabID := "report-tab-" + strconv.Itoa(i)
		panelID := "report-panel-" + strconv.Itoa(i)
		selected := "false"
		tabindex := ` tabindex="-1"`
		hidden := ` hidden`
		if i == 0 {
			selected = "true"
			tabindex = ` tabindex="0"`
			hidden = ""
		}
		fmt.Fprintf(&buttons, `<button id="%s" type="button" role="tab" aria-selected="%s" aria-controls="%s"%s>%s</button>`, tabID, selected, panelID, tabindex, htmlpkg.EscapeString(c.Title))
		part, err := renderReportComponent(src, opts, analysis, c)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&panels, `<section id="%s" class="report-tab-panel" role="tabpanel" aria-labelledby="%s"%s>%s</section>`, panelID, tabID, hidden, part)
	}
	return `<div class="report-tabs" data-report-tabs><div class="report-tab-list" role="tablist">` + buttons.String() + `</div>` + panels.String() + `</div>`, nil
}

// slideUnit is one rendered slide: a plain-text title (for the aria-label) and
// its HTML body.
type slideUnit struct {
	title string
	html  string
}

// h2SplitRe locates the start of each <h2> heading in rendered article HTML.
// goldmark escapes fenced-code content, so a literal "## " in a code block
// never produces a real <h2> tag — only headings match, which makes splitting
// on this marker safe without a full HTML parse.
var h2SplitRe = regexp.MustCompile(`(?i)<h2(?:\s[^>]*)?>`)

// headingInnerRe captures the inner HTML of the first <h1>/<h2> in a chunk.
var headingInnerRe = regexp.MustCompile(`(?is)<h[12](?:\s[^>]*)?>(.*?)</h[12]>`)

// htmlTagRe strips tags so a heading's inner HTML becomes plain title text.
var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

// headingTitle returns the plain text of the first h1/h2 in chunk, or "".
func headingTitle(chunk string) string {
	m := headingInnerRe.FindStringSubmatch(chunk)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(htmlpkg.UnescapeString(htmlTagRe.ReplaceAllString(m[1], "")))
}

var articleHeadingRe = regexp.MustCompile(`(?i)<h([1-6])(?:\s[^>]*)?>`)

func articleView(src []byte, opts Options) (string, error) {
	mdOpts := opts
	mdOpts.ReportTag = ""
	mdOpts.Plain = false
	doc, err := Render(src, mdOpts)
	if err != nil {
		return "", err
	}
	article := extractArticle(doc)
	return articleOverview(src, article) + article, nil
}

func articleOverview(src []byte, articleHTML string) string {
	headings := 0
	sections := 0
	for _, m := range articleHeadingRe.FindAllStringSubmatch(articleHTML, -1) {
		headings++
		if len(m) > 1 && m[1] == "2" {
			sections++
		}
	}
	items := [][2]string{
		{"Lines", strconv.Itoa(lineCount(src))},
		{"Headings", strconv.Itoa(headings)},
	}
	if sections > 0 {
		items = append(items, [2]string{"Sections", strconv.Itoa(sections)})
	}
	var b strings.Builder
	b.WriteString(`<dl class="article-overview" aria-label="Article overview">`)
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

func lineCount(src []byte) int {
	if len(src) == 0 {
		return 0
	}
	n := bytes.Count(src, []byte("\n"))
	if !bytes.HasSuffix(src, []byte("\n")) {
		n++
	}
	return n
}

// splitArticleByH2 breaks rendered article HTML into one slide per <h2> section
// — the "h2 sections become slides" intent. Content before the first <h2> (the
// h1 title and any intro) becomes the opening slide; an article with no <h2>
// stays a single slide titled fallback.
func splitArticleByH2(articleHTML, fallback string) []slideUnit {
	loc := h2SplitRe.FindAllStringIndex(articleHTML, -1)
	if len(loc) == 0 {
		return []slideUnit{{title: fallback, html: articleHTML}}
	}
	titleOr := func(chunk string) string {
		if t := headingTitle(chunk); t != "" {
			return t
		}
		return fallback
	}
	var units []slideUnit
	if intro := articleHTML[:loc[0][0]]; strings.TrimSpace(intro) != "" {
		units = append(units, slideUnit{title: titleOr(intro), html: intro})
	}
	for i, m := range loc {
		end := len(articleHTML)
		if i+1 < len(loc) {
			end = loc[i+1][0]
		}
		chunk := articleHTML[m[0]:end]
		units = append(units, slideUnit{title: titleOr(chunk), html: chunk})
	}
	return units
}

func renderSlides(src []byte, opts Options, analysis report.Analysis, components []report.Component) (string, error) {
	var units []slideUnit
	for _, c := range components {
		part, err := renderReportComponent(src, opts, analysis, c)
		if err != nil {
			return "", err
		}
		if c.Type == report.ComponentArticle {
			units = append(units, splitArticleByH2(part, c.Title)...)
			continue
		}
		units = append(units, slideUnit{title: c.Title, html: part})
	}

	var b strings.Builder
	b.WriteString(`<div class="report-slides" data-report-slides>`)
	total := len(units)
	for i, u := range units {
		fmt.Fprintf(&b, `<section class="report-slide" aria-label="Slide %d of %d: %s"><div class="report-slide-count">%d / %d</div>%s</section>`, i+1, total, htmlpkg.EscapeString(u.title), i+1, total, u.html)
	}
	if total > 1 {
		fmt.Fprintf(&b, `<div class="report-slide-controls" aria-label="Slide controls"><button type="button" data-slide-prev>Previous</button><span data-slide-status>1 / %d</span><button type="button" data-slide-next>Next</button></div>`, total)
	}
	b.WriteString(`</div>`)
	return b.String(), nil
}

func renderReportComponent(src []byte, opts Options, analysis report.Analysis, c report.Component) (string, error) {
	title := htmlpkg.EscapeString(c.Title)
	switch c.Type {
	case report.ComponentArticle:
		article, err := articleView(src, opts)
		if err != nil {
			return "", err
		}
		return article, nil
	case report.ComponentPreformatted:
		if analysis.Kind == report.KindLog || strings.EqualFold(c.Title, "Log") {
			return `<section class="report-section"><h2>` + title + `</h2>` + logView(src) + `</section>`, nil
		}
		if analysis.Kind == report.KindTranscript || strings.EqualFold(c.Title, "Transcript") {
			return `<section class="report-section"><h2>` + title + `</h2>` + transcriptView(src) + `</section>`, nil
		}
		return `<section class="report-section"><h2>` + title + `</h2>` + textView(src) + `</section>`, nil
	case report.ComponentCodeBlock:
		return `<section class="report-section"><h2>` + title + `</h2>` + codeView(src, opts, analysis) + `</section>`, nil
	case report.ComponentDataTable:
		return `<section class="report-section"><h2>` + title + `</h2>` + dataTable(src, analysis) + `</section>`, nil
	case report.ComponentRecordCards:
		return `<section class="report-section"><h2>` + title + `</h2>` + recordCards(src, analysis) + `</section>`, nil
	case report.ComponentDiffView:
		return `<section class="report-section"><h2>` + title + `</h2>` + diffView(src) + `</section>`, nil
	case report.ComponentFileTree:
		return `<section class="report-section"><h2>` + title + `</h2>` + fileTree(src) + `</section>`, nil
	case report.ComponentSummary:
		return summary(analysis), nil
	case report.ComponentRawJSON:
		return `<section class="report-section"><h2>` + title + `</h2>` + jsonView(src, analysis) + `</section>`, nil
	default:
		return `<section class="report-section"><h2>` + title + `</h2>` + rawPre(src) + `</section>`, nil
	}
}

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

func jsonView(src []byte, analysis report.Analysis) string {
	var pretty bytes.Buffer
	body := src
	if err := json.Indent(&pretty, stripUTF8BOM(src), "", "  "); err == nil {
		body = pretty.Bytes()
	}
	overview := jsonOverview(analysis.Data)
	if overview == "" {
		return rawPre(body)
	}
	return overview + rawPre(body)
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

func logView(src []byte) string {
	lines := strings.Split(string(reANSI.ReplaceAll(src, nil)), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return rawPre(src)
	}
	var b strings.Builder
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
		return ""
	}
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
	var b strings.Builder
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

func codeBlock(src []byte, opts Options) string {
	if reANSI.Match(src) {
		return renderANSI(src)
	}
	if lexer := pickLexer(opts.Lang, opts.SourceName, src); lexer != nil {
		if html, err := highlightCode(string(src), lexer); err == nil {
			return html
		}
	}
	return rawPre(src)
}

func codeView(src []byte, opts Options, analysis report.Analysis) string {
	return codeOverview(src, opts, analysis) + codeBlock(src, opts)
}

func codeOverview(src []byte, opts Options, analysis report.Analysis) string {
	items := make([][2]string, 0, 3)
	if lang, ok := analysis.Data.(string); ok && strings.TrimSpace(lang) != "" {
		items = append(items, [2]string{"Language", lang})
	} else if lexer := pickLexer(opts.Lang, opts.SourceName, src); lexer != nil {
		items = append(items, [2]string{"Language", lexer.Config().Name})
	} else {
		items = append(items, [2]string{"Language", "Plain text"})
	}
	if opts.SourceName != "" {
		items = append(items, [2]string{"Source", opts.SourceName})
	}
	if analysis.Stats.Lines > 0 {
		items = append(items, [2]string{"Lines", strconv.Itoa(analysis.Stats.Lines)})
	}
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<dl class="code-overview" aria-label="Code overview">`)
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

func summary(a report.Analysis) string {
	items := []string{
		"Kind: " + string(a.Kind),
		fmt.Sprintf("Confidence: %.2f", a.Confidence),
		fmt.Sprintf("Bytes: %d", a.Stats.Bytes),
		fmt.Sprintf("Lines: %d", a.Stats.Lines),
	}
	if a.Stats.Records > 0 {
		items = append(items, fmt.Sprintf("Records: %d", a.Stats.Records))
	}
	if a.Stats.Fields > 0 {
		items = append(items, fmt.Sprintf("Fields: %d", a.Stats.Fields))
	}
	if a.Stats.Files > 0 {
		items = append(items, fmt.Sprintf("Files: %d", a.Stats.Files))
	}
	var b strings.Builder
	b.WriteString(`<section class="report-summary" aria-label="Summary"><dl>`)
	for _, item := range items {
		k, v, _ := strings.Cut(item, ": ")
		b.WriteString(`<div><dt>`)
		b.WriteString(htmlpkg.EscapeString(k))
		b.WriteString(`</dt><dd>`)
		b.WriteString(htmlpkg.EscapeString(v))
		b.WriteString(`</dd></div>`)
	}
	b.WriteString(`</dl>`)
	if len(a.Reasons) > 0 {
		b.WriteString(`<p>`)
		b.WriteString(htmlpkg.EscapeString(strings.Join(a.Reasons, "; ")))
		b.WriteString(`</p>`)
	}
	b.WriteString(`</section>`)
	return b.String()
}

func dataTable(src []byte, analysis report.Analysis) string {
	headers, rows := tableRows(src, analysis)
	if len(headers) == 0 {
		return rawPre(src)
	}
	labels := headerLabels(headers)
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="report-table-wrap"><input class="report-filter" type="search" placeholder="Filter rows" aria-label="Filter rows"><div class="report-mobile-sort"><select aria-label="Sort rows"><option value="">Sort rows</option>`)
	for i, label := range labels {
		fmt.Fprintf(&b, `<option value="%d:ascending">%s ↑</option><option value="%d:descending">%s ↓</option>`, i, escapeTableText(label), i, escapeTableText(label))
	}
	fmt.Fprintf(&b, `</select></div><p class="report-filter-status" aria-live="polite">%s</p><table class="report-table" data-report-table><thead><tr>`, rowStatusText(len(rows)))
	for _, label := range labels {
		fmt.Fprintf(&b, `<th scope="col"><button type="button" data-sort-label="%s" aria-label="Sort by %s ascending">`, escapeTableText(label), escapeTableText(label))
		b.WriteString(escapeTableText(label))
		b.WriteString(`</button></th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range rows {
		b.WriteString(`<tr>`)
		for i := range headers {
			label := ""
			if i < len(labels) {
				label = labels[i]
			}
			fmt.Fprintf(&b, `<td data-label="%s">`, escapeTableText(label))
			if i < len(row) {
				b.WriteString(escapeTableText(row[i]))
			}
			b.WriteString(`</td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func rowStatusText(n int) string {
	if n == 1 {
		return "1 row"
	}
	return fmt.Sprintf("%d rows", n)
}

func recordCards(src []byte, analysis report.Analysis) string {
	headers, rows := tableRows(src, analysis)
	if len(headers) == 0 {
		return rawPre(src)
	}
	if len(rows) == 0 {
		return `<p class="record-empty" aria-live="polite">No records</p>`
	}
	labels := headerLabels(headers)
	var b strings.Builder
	b.WriteString(`<div class="record-cards">`)
	for i, row := range rows {
		fmt.Fprintf(&b, `<article class="record-card"><h3>%s</h3><dl>`, recordCardTitle(i+1, labels, row))
		for j, label := range labels {
			if j >= len(row) || strings.TrimSpace(cleanTableText(row[j])) == "" {
				continue
			}
			b.WriteString(`<div><dt>`)
			b.WriteString(escapeTableText(label))
			b.WriteString(`</dt><dd>`)
			b.WriteString(escapeTableText(row[j]))
			b.WriteString(`</dd></div>`)
		}
		b.WriteString(`</dl></article>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func recordCardTitle(n int, labels, row []string) string {
	fallback := fmt.Sprintf("Record %d", n)
	for _, preferred := range []string{"name", "title", "id", "key"} {
		for i, label := range labels {
			if !strings.EqualFold(strings.TrimSpace(label), preferred) || i >= len(row) {
				continue
			}
			value := strings.TrimSpace(cleanTableText(row[i]))
			if value == "" {
				continue
			}
			return escapeTableText(fallback + ": " + value)
		}
	}
	return escapeTableText(fallback)
}

func tableRows(src []byte, analysis report.Analysis) ([]string, [][]string) {
	src = stripUTF8BOM(src)
	switch analysis.Kind {
	case report.KindJSONRecords:
		if records, ok := analysis.Data.([]any); ok {
			if headers, rows := jsonRecordRows(records); len(headers) > 0 {
				return headers, rows
			}
		}
		var records []any
		dec := json.NewDecoder(bytes.NewReader(src))
		dec.UseNumber()
		if err := dec.Decode(&records); err == nil {
			return jsonRecordRows(records)
		}
	case report.KindCSVRecords, report.KindTSVRecords, report.KindTableRecords:
		if records, ok := analysis.Data.([][]string); ok {
			return delimitedRows(records)
		}
		if analysis.Kind == report.KindTableRecords {
			return nil, nil
		}
		comma := ','
		if analysis.Kind == report.KindTSVRecords {
			comma = '\t'
		}
		text := trimOuterBlankLines(string(stripUTF8BOM(src)))
		if strings.HasPrefix(strings.TrimLeft(text, " \t\r\n"), string(utf8BOM)) {
			return nil, nil
		}
		r := csv.NewReader(strings.NewReader(text))
		r.Comma = comma
		r.FieldsPerRecord = -1
		records, err := r.ReadAll()
		if err == nil {
			return delimitedRows(records)
		}
	}
	return nil, nil
}

func jsonRecordRows(records []any) ([]string, [][]string) {
	seen := map[string]bool{}
	objects := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		obj, ok := rec.(map[string]any)
		if !ok {
			return nil, nil
		}
		objects = append(objects, obj)
		for k := range obj {
			seen[k] = true
		}
	}
	headers := sortedStringKeys(seen)
	if len(headers) == 0 {
		return nil, nil
	}
	rows := make([][]string, 0, len(objects))
	for _, rec := range objects {
		row := make([]string, len(headers))
		for i, h := range headers {
			if v, ok := rec[h]; ok {
				row[i] = stringify(v)
			}
		}
		rows = append(rows, row)
	}
	return headers, rows
}

func delimitedRows(records [][]string) ([]string, [][]string) {
	if len(records) == 0 {
		return nil, nil
	}
	headers := records[0]
	if len(headers) == 0 {
		return nil, nil
	}
	rows := make([][]string, 0, len(records)-1)
	for _, rec := range records[1:] {
		row := make([]string, len(headers))
		for i := range headers {
			if i < len(rec) {
				row[i] = rec[i]
			}
		}
		rows = append(rows, row)
	}
	return headers, rows
}

func trimOuterBlankLines(text string) string {
	lines := strings.SplitAfter(text, "\n")
	start := 0
	for start < len(lines) && strings.Trim(lines[start], " \t\r\n") == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.Trim(lines[end-1], " \t\r\n") == "" {
		end--
	}
	return strings.Join(lines[start:end], "")
}

func headerLabels(headers []string) []string {
	labels := make([]string, len(headers))
	counts := map[string]int{}
	used := map[string]bool{}
	for i, header := range headers {
		base := headerLabel(header, i)
		counts[base]++
		label := base
		if counts[base] > 1 {
			label = fmt.Sprintf("%s %d", base, counts[base])
		}
		for used[label] {
			counts[base]++
			label = fmt.Sprintf("%s %d", base, counts[base])
		}
		labels[i] = label
		used[label] = true
	}
	return labels
}

func headerLabel(header string, index int) string {
	header = cleanTableText(header)
	if strings.TrimSpace(header) == "" {
		return fmt.Sprintf("Column %d", index+1)
	}
	return header
}

func stripUTF8BOM(src []byte) []byte {
	return bytes.TrimPrefix(src, utf8BOM)
}

func stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case json.Number:
		return x.String()
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(x)
		}
		return string(b)
	}
}

func diffView(src []byte) string {
	lines := strings.Split(string(src), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var b strings.Builder
	stats := diffStats{}
	var code strings.Builder
	code.WriteString(`<pre class="diff-view"><code>`)
	combinedPrefixCols := 0
	inHunk := false
	for _, line := range lines {
		clean := string(reANSI.ReplaceAll([]byte(line), nil))
		class := "ctx"
		switch {
		case isDiffBoundaryLine(clean):
			class = "file"
			stats.files++
			combinedPrefixCols = 0
			inHunk = false
		case strings.HasPrefix(clean, "@@"):
			class = "hunk"
			stats.hunks++
			combinedPrefixCols = combinedDiffPrefixCols(clean)
			inHunk = true
		case inHunk && combinedPrefixCols > 1:
			class = combinedDiffLineClass(clean, combinedPrefixCols)
		case inHunk && strings.HasPrefix(clean, "+"):
			class = "add"
		case inHunk && strings.HasPrefix(clean, "-"):
			class = "del"
		case strings.HasPrefix(clean, "+") && !strings.HasPrefix(clean, "+++"):
			class = "add"
		case strings.HasPrefix(clean, "-") && !strings.HasPrefix(clean, "---"):
			class = "del"
		case isDiffMetadataLine(clean):
			class = "file"
			combinedPrefixCols = 0
		}
		switch class {
		case "add":
			stats.additions++
		case "del":
			stats.deletions++
		}
		fmt.Fprintf(&code, `<span class="%s">%s</span>`+"\n", class, htmlpkg.EscapeString(clean))
	}
	code.WriteString(`</code></pre>`)
	b.WriteString(diffSummary(stats))
	b.WriteString(code.String())
	return b.String()
}

type diffStats struct {
	files     int
	hunks     int
	additions int
	deletions int
}

func diffSummary(stats diffStats) string {
	var b strings.Builder
	b.WriteString(`<div class="diff-summary" aria-label="Diff summary">`)
	fmt.Fprintf(&b, `<span><strong>%d</strong> %s</span>`, stats.files, plural(stats.files, "file", "files"))
	fmt.Fprintf(&b, `<span><strong>%d</strong> %s</span>`, stats.hunks, plural(stats.hunks, "hunk", "hunks"))
	fmt.Fprintf(&b, `<span class="diff-added"><strong>+%d</strong> %s</span>`, stats.additions, plural(stats.additions, "addition", "additions"))
	fmt.Fprintf(&b, `<span class="diff-removed"><strong>-%d</strong> %s</span>`, stats.deletions, plural(stats.deletions, "deletion", "deletions"))
	b.WriteString(`</div>`)
	return b.String()
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func isDiffMetadataLine(line string) bool {
	for _, prefix := range diffMetadataPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func isDiffBoundaryLine(line string) bool {
	return strings.HasPrefix(line, "diff --git") ||
		strings.HasPrefix(line, "diff --cc") ||
		strings.HasPrefix(line, "diff --combined")
}

var diffMetadataPrefixes = []string{
	"diff --git",
	"diff --cc",
	"diff --combined",
	"---",
	"+++",
	"index ",
	"old mode ",
	"new mode ",
	"new file mode ",
	"deleted file mode ",
	"rename from ",
	"rename to ",
	"copy from ",
	"copy to ",
	"similarity index ",
	"dissimilarity index ",
	"GIT binary patch",
	"literal ",
	"delta ",
	"Binary files ",
	`\ No newline at end of file`,
}

func combinedDiffPrefixCols(line string) int {
	if !strings.HasPrefix(line, "@@@") {
		return 0
	}
	count := 0
	for count < len(line) && line[count] == '@' {
		count++
	}
	return count - 1
}

func combinedDiffLineClass(line string, prefixCols int) string {
	if len(line) < prefixCols {
		return "ctx"
	}
	prefix := line[:prefixCols]
	for _, r := range prefix {
		if r != ' ' && r != '+' && r != '-' {
			return "ctx"
		}
	}
	if strings.Contains(prefix, "+") {
		return "add"
	}
	if strings.Contains(prefix, "-") {
		return "del"
	}
	return "ctx"
}

func fileTree(src []byte) string {
	lines := nonEmptyTreeLines(string(src))
	if len(lines) == 0 {
		return rawPre(src)
	}
	var b strings.Builder
	b.WriteString(`<ul class="file-tree">`)
	for _, line := range lines {
		name, depth := treeLine(line)
		fmt.Fprintf(&b, `<li style="--depth:%d"><span>%s</span></li>`, depth, htmlpkg.EscapeString(name))
	}
	b.WriteString(`</ul>`)
	return b.String()
}

func nonEmptyTreeLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		clean := string(reANSI.ReplaceAll([]byte(line), nil))
		if strings.TrimSpace(clean) != "" && !isTreeRootLine(clean) && !isTreeSummaryLine(clean) {
			lines = append(lines, clean)
		}
	}
	return lines
}

func isTreeRootLine(line string) bool {
	switch strings.TrimSpace(line) {
	case ".", "./", `.\`:
		return true
	default:
		return false
	}
}

func isTreeSummaryLine(line string) bool {
	fields := strings.Fields(strings.ReplaceAll(strings.TrimSpace(line), ",", ""))
	if len(fields) == 2 {
		return isUnsignedInt(fields[0]) && isTreeDirectoryWord(fields[1])
	}
	return len(fields) == 4 &&
		isUnsignedInt(fields[0]) &&
		isTreeDirectoryWord(fields[1]) &&
		isUnsignedInt(fields[2]) &&
		isTreeFileWord(fields[3])
}

func isUnsignedInt(text string) bool {
	if text == "" {
		return false
	}
	_, err := strconv.Atoi(text)
	return err == nil
}

func isTreeDirectoryWord(text string) bool {
	return text == "directory" || text == "directories"
}

func isTreeFileWord(text string) bool {
	return text == "file" || text == "files"
}

func treeLine(line string) (string, int) {
	if name, depth, ok := asciiTreeLine(line); ok {
		return name, depth
	}
	name := strings.TrimSpace(strings.TrimLeft(line, "├└│─ "))
	if name == "" {
		name = line
	}
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, `.\`)
	return name, pathDepth(line)
}

func asciiTreeLine(line string) (string, int, bool) {
	markers := []string{"|-- ", "`-- ", "+-- "}
	for _, marker := range markers {
		if i := strings.Index(line, marker); i >= 0 {
			name := strings.TrimSpace(line[i+len(marker):])
			if name == "" {
				name = line
			}
			return name, strings.Count(line[:i], "|   ") + strings.Count(line[:i], "    "), true
		}
	}
	return "", 0, false
}

func pathDepth(line string) int {
	if strings.ContainsAny(line, "├└│") {
		return unicodeTreeDepth(line)
	}
	clean := strings.TrimSpace(line)
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "../")
	clean = strings.TrimPrefix(clean, "~/")
	clean = strings.TrimPrefix(clean, `.\`)
	clean = strings.TrimPrefix(clean, `..\`)
	clean = strings.ReplaceAll(clean, `\`, "/")
	clean = trimWindowsVolumeAnchor(clean)
	clean = strings.TrimLeft(clean, "/")
	return strings.Count(path.Clean(clean), "/")
}

func unicodeTreeDepth(line string) int {
	marker := strings.IndexAny(line, "├└")
	if marker < 0 {
		marker = len(line)
	}
	prefix := line[:marker]
	return strings.Count(prefix, "│") + strings.Count(prefix, "    ")
}

func trimWindowsVolumeAnchor(path string) string {
	if len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0]) {
		return path[2:]
	}
	return path
}

func isASCIILetter(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}

func extractArticle(doc string) string {
	start := strings.Index(doc, `<article class="markdown-body">`)
	if start < 0 {
		return doc
	}
	start += len(`<article class="markdown-body">`)
	end := strings.LastIndex(doc, `</article>`)
	if end <= start {
		return doc
	}
	return doc[start:end]
}

func sortedStringKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
