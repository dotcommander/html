package render

import (
	"fmt"
	htmlpkg "html"
	"html/template"
	"strings"

	"github.com/dotcommander/html/internal/report"
)

// TemplateContractVersion changes when the author-facing context or execution
// contract changes. It is part of every page's cache freshness fingerprint.
const TemplateContractVersion = "1"

const maxTemplateOutput = 64 << 20

// documentFragment is renderer-owned HTML before page presentation. Only
// ordinary Markdown has a separate TOC; report navigation stays inside body.
type documentFragment struct {
	escapedTitle string
	body         string
	toc          string
}

func (f documentFragment) defaultBody() string {
	if f.toc != "" {
		return insertAfterFirstH1(f.body, f.toc)
	}
	return f.body
}

// templateContext marks only renderer-produced fragments as trusted HTML.
// Titles and JSON values remain ordinary, contextually escaped template data.
type templateContext struct {
	Title    string
	Content  template.HTML
	TOC      template.HTML
	Data     any
	Head     template.HTML
	Controls template.HTML
	Scripts  template.HTML
}

func isReadingTemplate(selector string) bool {
	return selector == "reader" || selector == "notebook"
}

// ValidateTemplateMode is also used before cache lookup so incompatible modes
// cannot bypass validation via a previously rendered page.
func ValidateTemplateMode(opts Options, reportMode bool) error {
	if isReadingTemplate(opts.Template) && (reportMode || opts.Plain || opts.Frame) {
		return fmt.Errorf("template %q requires ordinary Markdown; plain, framed, and report modes are incompatible", opts.Template)
	}
	return nil
}

func assemblePage(f documentFragment, src []byte, opts Options) (string, error) {
	head := pageHead(f.escapedTitle, f.body, opts)
	scripts := pageScripts()
	if opts.Template == "" || opts.Template == "default" {
		body := f.defaultBody()
		if opts.Frame {
			body = terminalFrame(f.escapedTitle, body)
		}
		// Keep default page bytes stable, including its historical whitespace.
		return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>%s</head>
<body>
  %s
  <article class="markdown-body">
%s
  </article>
  %s
</body>
</html>
`, head, pageControls, body, scripts), nil
	}
	source := opts.TemplateSource
	if isReadingTemplate(opts.Template) {
		source = mustReadAsset("assets/" + opts.Template + ".html.tmpl")
	}
	body := f.body
	if opts.Frame {
		body = terminalFrame(f.escapedTitle, body)
	}
	data, _ := report.DecodeJSON(src)
	context := templateContext{
		Title: htmlpkg.UnescapeString(f.escapedTitle), Content: template.HTML(body),
		TOC: template.HTML(f.toc), Data: data, Head: template.HTML(head),
		Controls: template.HTML(pageControls), Scripts: template.HTML(scripts),
	}
	return executePageTemplate(source, context, maxTemplateOutput)
}

// executePageTemplate buffers the entire result before the caller can publish
// it. Templates have only standard Go template functions; no I/O or trust casts.
func executePageTemplate(source string, context templateContext, limit int) (string, error) {
	t, err := template.New("page").Option("missingkey=error").Funcs(template.FuncMap{"index": strictTemplateIndex}).Parse(source)
	if err != nil {
		return "", fmt.Errorf("parse page template: %w", err)
	}
	w := templateOutput{limit: limit}
	if err := t.Execute(&w, context); err != nil {
		return "", fmt.Errorf("execute page template: %w", err)
	}
	return w.String(), nil
}

type templateOutput struct {
	buffer strings.Builder
	limit  int
}

func (w *templateOutput) String() string { return w.buffer.String() }

func (w *templateOutput) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.buffer.Len() {
		return 0, fmt.Errorf("custom-template output exceeds %d-byte limit (64 MiB maximum)", w.limit)
	}
	return w.buffer.Write(p)
}
