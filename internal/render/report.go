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
	b.WriteString(`</div>`)
	return b.String(), nil
}

func renderReportComponent(src []byte, opts Options, analysis report.Analysis, c report.Component) (string, error) {
	title := htmlpkg.EscapeString(c.Title)
	switch c.Type {
	case report.ComponentArticle:
		mdOpts := opts
		mdOpts.ReportTag = ""
		mdOpts.Plain = false
		doc, err := Render(src, mdOpts)
		if err != nil {
			return "", err
		}
		return extractArticle(doc), nil
	case report.ComponentPreformatted:
		return `<section class="report-section"><h2>` + title + `</h2>` + rawPre(src) + `</section>`, nil
	case report.ComponentCodeBlock:
		return `<section class="report-section"><h2>` + title + `</h2>` + codeBlock(src, opts) + `</section>`, nil
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
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, stripUTF8BOM(src), "", "  "); err == nil {
			return `<section class="report-section"><h2>` + title + `</h2>` + rawPre(pretty.Bytes()) + `</section>`, nil
		}
		return `<section class="report-section"><h2>` + title + `</h2>` + rawPre(src) + `</section>`, nil
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
	fmt.Fprintf(&b, `<div class="report-table-wrap"><input class="report-filter" type="search" placeholder="Filter rows" aria-label="Filter rows"><p class="report-filter-status" aria-live="polite">%s</p><table class="report-table" data-report-table><thead><tr>`, rowStatusText(len(rows)))
	for _, label := range labels {
		b.WriteString(`<th scope="col"><button type="button">`)
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
	labels := headerLabels(headers)
	var b strings.Builder
	b.WriteString(`<div class="record-cards">`)
	for i, row := range rows {
		fmt.Fprintf(&b, `<article class="record-card"><h3>Record %d</h3><dl>`, i+1)
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
	case report.KindCSVRecords, report.KindTSVRecords:
		comma := ','
		if analysis.Kind == report.KindTSVRecords {
			comma = '\t'
		}
		if records, ok := analysis.Data.([][]string); ok {
			return delimitedRows(records)
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
	if len(records) <= 1 {
		return nil, nil
	}
	headers := records[0]
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
	b.WriteString(`<pre class="diff-view"><code>`)
	combinedPrefixCols := 0
	inHunk := false
	for _, line := range lines {
		clean := string(reANSI.ReplaceAll([]byte(line), nil))
		class := "ctx"
		switch {
		case isDiffBoundaryLine(clean):
			class = "file"
			combinedPrefixCols = 0
			inHunk = false
		case strings.HasPrefix(clean, "@@"):
			class = "hunk"
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
		fmt.Fprintf(&b, `<span class="%s">%s</span>`+"\n", class, htmlpkg.EscapeString(clean))
	}
	b.WriteString(`</code></pre>`)
	return b.String()
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
