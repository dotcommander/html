package render

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// localLinkRebaser keeps relative Markdown links valid when a rendered page is
// written outside the source directory, such as into the application cache.
type localLinkRebaser struct{}

var linkBaseDirKey = parser.NewContextKey()

func (localLinkRebaser) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	baseDir, _ := pc.Get(linkBaseDirKey).(string)
	if baseDir == "" {
		return
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := n.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		if destination, ok := rebaseLocalLink(baseDir, string(link.Destination)); ok {
			link.Destination = []byte(destination)
		}
		return ast.WalkContinue, nil
	})
}

func rebaseLocalLink(baseDir, destination string) (string, bool) {
	u, err := url.Parse(destination)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Opaque != "" || u.Path == "" {
		return "", false
	}
	if strings.HasPrefix(destination, "//") || filepath.IsAbs(filepath.FromSlash(u.Path)) {
		return "", false
	}
	abs, err := filepath.Abs(filepath.Join(baseDir, filepath.FromSlash(u.Path)))
	if err != nil {
		return "", false
	}
	path := filepath.ToSlash(abs)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	fileURL := url.URL{
		Scheme:      "file",
		Path:        path,
		RawQuery:    u.RawQuery,
		Fragment:    u.Fragment,
		RawFragment: u.RawFragment,
	}
	return fileURL.String(), true
}
