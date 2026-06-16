package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderSchemaVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "53", renderSchemaVersion)
}

func TestFingerprintIncludesTitleAndSourceName(t *testing.T) {
	t.Parallel()

	base := Options{Plain: true, FallbackTitle: "first", SourceName: "input.txt"}

	assert.NotEqual(t, Fingerprint(base), Fingerprint(Options{Plain: true, FallbackTitle: "second", SourceName: "input.txt"}))
	assert.NotEqual(t, Fingerprint(base), Fingerprint(Options{Plain: true, FallbackTitle: "first", SourceName: "input.go"}))
}

func TestFingerprintOptionTagsDoNotCollide(t *testing.T) {
	t.Parallel()

	left := Options{Plain: true, FallbackTitle: "a+source=b"}
	right := Options{Plain: true, FallbackTitle: "a", SourceName: "b"}

	assert.NotEqual(t, Fingerprint(left), Fingerprint(right))
}
