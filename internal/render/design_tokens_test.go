package render

import (
	"regexp"
	"strings"
	"testing"
)

const (
	tokenValuesStart = "/* design-token-values:start */"
	tokenValuesEnd   = "/* design-token-values:end */"
)

var (
	rawColorPattern              = regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|\b(?:rgb|rgba|hsl|hsla|hwb|lab|lch|oklab|oklch)\s*\(`)
	fontDeclarationPattern       = regexp.MustCompile(`(?i)\b(font-family|font)\s*:\s*([^;}]+)`)
	semanticFontReferencePattern = regexp.MustCompile(`var\(--font-[a-z0-9-]+\)`)
)

func TestEmbeddedCSSUsesDesignTokens(t *testing.T) {
	assets := []struct {
		name string
		css  string
	}{
		{name: "base.css", css: baseCSS()},
		{name: "alerts.css", css: alertCSS()},
		{name: "frame.css", css: frameCSS()},
	}

	for _, asset := range assets {
		t.Run(asset.name, func(t *testing.T) {
			assertTokenizedCSS(t, asset.css)
		})
	}
}

func assertTokenizedCSS(t *testing.T, css string) {
	t.Helper()

	inTokenValues := false
	var consumerCSS strings.Builder
	for lineNumber, line := range strings.Split(css, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case tokenValuesStart:
			if inTokenValues {
				t.Fatalf("line %d: nested design-token value block", lineNumber+1)
			}
			inTokenValues = true
			consumerCSS.WriteByte('\n')
			continue
		case tokenValuesEnd:
			if !inTokenValues {
				t.Fatalf("line %d: design-token value block ends without a start", lineNumber+1)
			}
			inTokenValues = false
			consumerCSS.WriteByte('\n')
			continue
		}

		if inTokenValues {
			consumerCSS.WriteByte('\n')
			continue
		}
		consumerCSS.WriteString(line)
		consumerCSS.WriteByte('\n')
	}

	if inTokenValues {
		t.Error("design-token value block is missing its end marker")
	}
	consumerStyles := consumerCSS.String()
	for _, match := range rawColorPattern.FindAllStringIndex(consumerStyles, -1) {
		lineNumber := strings.Count(consumerStyles[:match[0]], "\n") + 1
		t.Errorf("line %d: raw color must be declared in a design-token value block: %s", lineNumber, consumerStyles[match[0]:match[1]])
	}
	for _, match := range literalFontDeclarations(consumerStyles) {
		lineNumber := strings.Count(consumerStyles[:match.start], "\n") + 1
		t.Errorf("line %d: font stack must use a semantic font token: %s", lineNumber, match.declaration)
	}
}

func TestRawColorPatternSpansLines(t *testing.T) {
	css := ".x { color: rgb\n  (1 2 3); }"
	if !rawColorPattern.MatchString(css) {
		t.Fatal("multiline raw color was not detected")
	}
}

type fontDeclarationMatch struct {
	start       int
	declaration string
}

func literalFontDeclarations(css string) []fontDeclarationMatch {
	var literals []fontDeclarationMatch
	for _, match := range fontDeclarationPattern.FindAllStringSubmatchIndex(css, -1) {
		property := strings.ToLower(css[match[2]:match[3]])
		value := strings.TrimSpace(css[match[4]:match[5]])
		fontReference := semanticFontReferencePattern.FindStringIndex(value)
		valid := value == "inherit"
		if property == "font-family" {
			valid = valid || fontReference != nil && fontReference[0] == 0 && fontReference[1] == len(value)
		} else if fontReference != nil {
			valid = valid || fontReference[1] == len(value)
		}
		if !valid {
			literals = append(literals, fontDeclarationMatch{
				start:       match[0],
				declaration: strings.Join(strings.Fields(css[match[0]:match[1]]), " "),
			})
		}
	}
	return literals
}

func TestLiteralFontDeclarations(t *testing.T) {
	tests := []struct {
		name    string
		css     string
		literal bool
	}{
		{name: "inline literal", css: `.x { font-family: Arial, sans-serif; }`, literal: true},
		{name: "multiline literal", css: ".x {\n  font-family:\n    Arial, sans-serif;\n}", literal: true},
		{name: "semantic family", css: `.x { font-family: var(--font-sans); }`},
		{name: "semantic shorthand", css: `.x { font: 700 1rem var(--font-sans); }`},
		{name: "inherited shorthand", css: `.x { font: inherit; }`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := len(literalFontDeclarations(test.css)) > 0
			if got != test.literal {
				t.Fatalf("literal font declaration = %t, want %t", got, test.literal)
			}
		})
	}
}
