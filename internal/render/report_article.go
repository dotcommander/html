package render

import (
	"bytes"
	htmlpkg "html"
	"strconv"
	"strings"
)

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
	if images := len(articleImageRe.FindAllStringIndex(articleHTML, -1)); images > 0 {
		items = append(items, [2]string{"Images", strconv.Itoa(images)})
	}
	if tables := len(articleTableRe.FindAllStringIndex(articleHTML, -1)); tables > 0 {
		items = append(items, [2]string{"Tables", strconv.Itoa(tables)})
	}
	if codeBlocks := len(articleCodeBlockRe.FindAllStringIndex(articleHTML, -1)); codeBlocks > 0 {
		items = append(items, [2]string{"Code blocks", strconv.Itoa(codeBlocks)})
	}
	if tasks := len(articleTaskRe.FindAllStringIndex(articleHTML, -1)); tasks > 0 {
		items = append(items, [2]string{"Tasks", strconv.Itoa(tasks)})
	}
	if quotes := len(articleBlockquoteRe.FindAllStringIndex(articleHTML, -1)); quotes > 0 {
		items = append(items, [2]string{"Quotes", strconv.Itoa(quotes)})
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
