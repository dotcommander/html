package render

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

const alertTypeAttribute = "html-alert-type"

var alertLabels = map[string]string{
	"note":      "Note",
	"tip":       "Tip",
	"important": "Important",
	"warning":   "Warning",
	"caution":   "Caution",
}

// githubAlerts implements GitHub's blockquote-based alert syntax without raw
// HTML nodes. Its renderer therefore behaves identically in safe mode.
var githubAlerts goldmark.Extender = alertExtension{}

type alertExtension struct{}

func (alertExtension) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(alertTransformer{}, 200),
	))
	md.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(alertRenderer{}, 500),
	))
}

type alertTransformer struct{}

func (alertTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node.Kind() != ast.KindBlockquote {
			return ast.WalkContinue, nil
		}
		blockquote := node.(*ast.Blockquote)
		paragraph, ok := blockquote.FirstChild().(*ast.Paragraph)
		if !ok {
			return ast.WalkContinue, nil
		}
		if paragraph.Lines().Len() == 0 {
			return ast.WalkContinue, nil
		}
		markerSegment := paragraph.Lines().At(0)
		markerLine := bytes.TrimRight(markerSegment.Value(source), " \t\r\n")
		alertType, ok := parseAlertMarker(markerLine)
		if !ok {
			return ast.WalkContinue, nil
		}

		blockquote.SetAttributeString(alertTypeAttribute, alertType)
		removeAlertMarkerLine(paragraph)
		if !paragraph.HasChildren() {
			blockquote.RemoveChild(blockquote, paragraph)
		}
		return ast.WalkContinue, nil
	})
}

func parseAlertMarker(value []byte) (string, bool) {
	if len(value) < len("[!TIP]") || value[0] != '[' {
		return "", false
	}
	end := bytes.IndexByte(value, ']')
	if end != len(value)-1 || end < 3 || value[1] != '!' {
		return "", false
	}
	alertType := strings.ToLower(string(value[2:end]))
	if _, ok := alertLabels[alertType]; !ok {
		return "", false
	}
	return alertType, true
}

func removeAlertMarkerLine(paragraph *ast.Paragraph) {
	for child := paragraph.FirstChild(); child != nil; {
		next := child.NextSibling()
		endsLine := false
		if textNode, ok := child.(*ast.Text); ok {
			endsLine = textNode.SoftLineBreak() || textNode.HardLineBreak()
		}
		paragraph.RemoveChild(paragraph, child)
		if endsLine {
			return
		}
		child = next
	}
}

type alertRenderer struct{}

func (alertRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindBlockquote, renderAlertBlockquote)
}

func renderAlertBlockquote(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	alertType, ok := alertTypeOf(node)
	if !ok {
		if entering {
			_, _ = w.WriteString("<blockquote>\n")
		} else {
			_, _ = w.WriteString("</blockquote>\n")
		}
		return ast.WalkContinue, nil
	}
	if entering {
		_, _ = w.WriteString(`<div class="markdown-alert markdown-alert-` + alertType + `">`)
		_, _ = w.WriteString(`<p class="markdown-alert-title">` + alertLabels[alertType] + "</p>\n")
	} else {
		_, _ = w.WriteString("</div>\n")
	}
	return ast.WalkContinue, nil
}

func alertTypeOf(node ast.Node) (string, bool) {
	value, ok := node.AttributeString(alertTypeAttribute)
	if !ok {
		return "", false
	}
	alertType, ok := value.(string)
	return alertType, ok
}
