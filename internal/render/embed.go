package render

import "embed"

//go:embed assets/base.css assets/copy.js assets/theme.js assets/headings.js
var assetsFS embed.FS

func baseCSS() string    { return mustReadAsset("assets/base.css") }
func copyJS() string     { return mustReadAsset("assets/copy.js") }
func themeJS() string    { return mustReadAsset("assets/theme.js") }
func headingsJS() string { return mustReadAsset("assets/headings.js") }

func mustReadAsset(name string) string {
	b, err := assetsFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(b)
}
