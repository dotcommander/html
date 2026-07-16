package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_GitHubAlerts(t *testing.T) {
	t.Parallel()

	src := []byte(`# Alerts

> [!NOTE]
> Useful information.

> [!TIP]
> Helpful advice.

> [!IMPORTANT]
> Essential information.

> [!WARNING]
> Urgent attention required.

> [!CAUTION]
> Risky consequences.
`)
	got, err := Render(src, Options{FallbackTitle: "alerts"})
	require.NoError(t, err)

	for _, alertType := range []string{"note", "tip", "important", "warning", "caution"} {
		assert.Contains(t, got, `class="markdown-alert markdown-alert-`+alertType+`"`)
	}
	for _, label := range []string{"Note", "Tip", "Important", "Warning", "Caution"} {
		assert.Contains(t, got, `class="markdown-alert-title">`+label+`</p>`)
	}
	assert.NotContains(t, got, "[!NOTE]")
	assert.Equal(t, 1, strings.Count(got, ".markdown-alert {"), "alert CSS should be embedded once")
}

func TestRender_GitHubAlertMarkerGrammar(t *testing.T) {
	t.Parallel()

	src := []byte("> [!note]  \n> Accepted lowercase marker.\n\n> [!TIP]\t\n> Accepted trailing tab.\n\n> [!NOTE] trailing\n> Ordinary quote.\n")
	got, err := Render(src, Options{FallbackTitle: "grammar"})
	require.NoError(t, err)

	assert.Equal(t, 1, strings.Count(got, `class="markdown-alert markdown-alert-note"`))
	assert.Equal(t, 1, strings.Count(got, `class="markdown-alert markdown-alert-tip"`))
	assert.Contains(t, got, "[!NOTE] trailing")
	assert.Equal(t, 1, strings.Count(got, "<blockquote>"))
}

func TestRender_GitHubAlertSafeMode(t *testing.T) {
	t.Parallel()

	src := []byte("> [!WARNING]\n> **Check** <img src=x onerror=alert(1)> this.\n")
	got, err := Render(src, Options{FallbackTitle: "safe", Safe: true})
	require.NoError(t, err)

	assert.Contains(t, got, `class="markdown-alert markdown-alert-warning"`)
	assert.Contains(t, got, "<strong>Check</strong>")
	assert.NotContains(t, got, "onerror")
}

func TestRender_OrdinaryBlockquoteUnchanged(t *testing.T) {
	t.Parallel()

	src := []byte("> Ordinary quotation.\n")
	got, err := Render(src, Options{FallbackTitle: "quote"})
	require.NoError(t, err)

	assert.Contains(t, got, "<blockquote>\n<p>Ordinary quotation.</p>\n</blockquote>")
	assert.NotContains(t, got, "markdown-alert")
	assert.NotContains(t, got, ".markdown-alert {")
}

func TestRender_InvalidAlertMarkerRemainsBlockquote(t *testing.T) {
	t.Parallel()

	src := []byte("> [!UNKNOWN]\n> Still a quote.\n")
	got, err := Render(src, Options{FallbackTitle: "quote"})
	require.NoError(t, err)

	assert.Contains(t, got, "<blockquote>")
	assert.Contains(t, got, "[!UNKNOWN]")
	assert.NotContains(t, got, "markdown-alert")
}
