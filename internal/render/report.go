package render

import (
	"fmt"
	htmlpkg "html"
	"regexp"
	"strconv"
	"strings"

	"github.com/dotcommander/html/internal/report"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

func RenderReport(src []byte, opts Options, analysis report.Analysis, plan report.ReportPlan) (string, error) {
	if err := report.ValidateComponentSources(src, plan.Components); err != nil {
		// Semantic plans are cacheable, so their byte ranges can become stale.
		// Preserve the established article renderer instead of rendering the
		// wrong bytes or surfacing a partial report.
		plan.Layout = report.LayoutSinglePage
		plan.Components = []report.Component{{Type: report.ComponentArticle, Source: "input", Title: "Article", Options: map[string]string{}}}
	}
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
	if refs := semanticTimelineLists(plan.Components); len(refs) > 0 {
		opts.semanticLists = refs
		return articleView(src, opts)
	}
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
		title := htmlpkg.EscapeString(c.Title)
		fmt.Fprintf(&buttons, `<button id="%s" type="button" role="tab" aria-selected="%s" aria-controls="%s"%s title="%s"><span>%s</span></button>`, tabID, selected, panelID, tabindex, title, title)
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
var articleImageRe = regexp.MustCompile(`(?i)<img(?:\s|>)`)
var articleTableRe = regexp.MustCompile(`(?i)<table(?:\s|>)`)
var articleCodeBlockRe = regexp.MustCompile(`(?i)<pre(?:\s|>)`)
var articleTaskRe = regexp.MustCompile(`(?i)<input\b[^>]*\btype="checkbox"`)
var articleBlockquoteRe = regexp.MustCompile(`(?i)<blockquote(?:\s|>)`)

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
		fmt.Fprintf(&b, `<nav class="report-slide-controls" aria-label="Slide controls"><button type="button" data-slide-prev aria-label="Previous slide" title="Previous slide"><span aria-hidden="true">‹</span></button><span data-slide-status>1 / %d</span><button type="button" data-slide-next aria-label="Next slide" title="Next slide"><span aria-hidden="true">›</span></button></nav>`, total)
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
	case report.ComponentTimeline:
		return "", fmt.Errorf("timeline component requires full-document semantic rendering")
	case report.ComponentPreformatted:
		if analysis.Kind == report.KindBinary {
			return `<section class="report-section"><h2>` + title + `</h2>` + binaryView(src, analysis) + `</section>`, nil
		}
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
	case report.ComponentChart:
		return `<section class="report-section"><h2>` + title + `</h2>` + chartView(src, analysis, c.Options) + `</section>`, nil
	case report.ComponentRecordCards:
		return `<section class="report-section"><h2>` + title + `</h2>` + recordCards(src, analysis) + `</section>`, nil
	case report.ComponentReview:
		return `<section class="report-section"><h2>` + title + `</h2>` + reviewCards(src, analysis) + `</section>`, nil
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

func semanticTimelineLists(components []report.Component) []report.SourceRef {
	var refs []report.SourceRef
	for _, component := range components {
		if component.Timeline != nil {
			refs = append(refs, component.Timeline.List)
		}
	}
	return refs
}

type semanticListTransformer struct {
	refs []report.SourceRef
}

func (transformer semanticListTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	wanted := make(map[[2]int]struct{}, len(transformer.refs))
	for _, ref := range transformer.refs {
		wanted[[2]int{ref.Start, ref.End}] = struct{}{}
	}
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		list, ok := node.(*ast.List)
		if !ok || !list.IsOrdered() {
			return ast.WalkContinue, nil
		}
		start, end, ok := report.SourceRangeForNode(list, src)
		if !ok {
			return ast.WalkContinue, nil
		}
		if _, ok := wanted[[2]int{start, end}]; !ok {
			return ast.WalkContinue, nil
		}
		list.SetAttributeString("class", []byte("report-timeline-list"))
		for item := list.FirstChild(); item != nil; item = item.NextSibling() {
			item.SetAttributeString("class", []byte("report-timeline-item"))
		}
		return ast.WalkSkipChildren, nil
	})
}
