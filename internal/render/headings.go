package render

import (
	htmlpkg "html"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// shouldRenderTOC decides whether to emit a table of contents: an explicit
// override wins; otherwise the TOC appears once a document has tocMinEntries or
// more h2/h3 headings.
func shouldRenderTOC(override *bool, n int) bool {
	if override != nil {
		return *override
	}
	return n >= tocMinEntries
}

// heading is one h2/h3 entry collected for the table of contents.
type heading struct {
	level int
	text  string // trimmed plain text, not yet HTML-escaped
	id    string // goldmark auto-generated heading id
}

// analyze walks an already-parsed document node and returns the page title
// (first level-1 heading text, HTML-escaped and trimmed, or the escaped
// fallback), whether that title came from a real heading, and the h2/h3
// headings (in document order) used to build the table of contents. ids are
// read straight from the parsed AST, so TOC links match the ids goldmark
// renders into the body.
func analyze(node ast.Node, src []byte, fallback string) (title string, fromHeading bool, headings []heading) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		switch {
		case h.Level == 1 && title == "":
			title = headingText(h, src)
		case h.Level == 2, h.Level == 3:
			headings = append(headings, heading{
				level: h.Level,
				text:  headingText(h, src),
				id:    headingID(h),
			})
		}
		// Heading children are inline nodes already consumed by headingText.
		return ast.WalkSkipChildren, nil
	})

	if title == "" {
		return htmlpkg.EscapeString(fallback), false, headings
	}
	return htmlpkg.EscapeString(title), true, headings
}

// headingText returns the concatenated plain text of a heading's inline
// descendants (Text and String nodes), trimmed. Emphasis/code/link markup is
// flattened to its text; raw inline HTML is dropped.
func headingText(n ast.Node, src []byte) string {
	var sb strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := c.(type) {
		case *ast.Text:
			sb.Write(t.Segment.Value(src))
		case *ast.String:
			sb.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(sb.String())
}

// headingID returns the goldmark auto-generated id attribute of a heading, or
// "" if none was assigned.
func headingID(n ast.Node) string {
	v, ok := n.AttributeString("id")
	if !ok {
		return ""
	}
	switch s := v.(type) {
	case []byte:
		return string(s)
	case string:
		return s
	}
	return ""
}

// tocMinEntries is the h2/h3 count below which no table of contents is rendered
// — short documents stay uncluttered.
const tocMinEntries = 4

// buildTOC renders a compact navigation list of the given headings as same-page
// anchor links, or "" when there are none. Whether a TOC should appear at all is
// decided by shouldRenderTOC; this only formats it. Heading text and ids are
// HTML-escaped.
func buildTOC(headings []heading) string {
	if len(headings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav class="toc" aria-label="Table of contents">` + "\n<ul>\n")
	for _, h := range headings {
		b.WriteString(`<li class="toc-h`)
		b.WriteString(strconv.Itoa(h.level))
		b.WriteString(`"><a href="#`)
		b.WriteString(htmlpkg.EscapeString(h.id))
		b.WriteString(`">`)
		b.WriteString(htmlpkg.EscapeString(h.text))
		b.WriteString("</a></li>\n")
	}
	b.WriteString("</ul>\n</nav>\n")
	return b.String()
}

// insertAfterFirstH1 inserts toc immediately after the first </h1> in content
// (skipping a trailing newline), or prepends it when no h1 is present.
func insertAfterFirstH1(content, toc string) string {
	const closeTag = "</h1>"
	i := strings.Index(content, closeTag)
	if i < 0 {
		return toc + content
	}
	i += len(closeTag)
	if i < len(content) && content[i] == '\n' {
		i++
	}
	return content[:i] + toc + content[i:]
}
