package render

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"slices"
	"sync"
)

// renderSchemaVersion is bumped whenever renderer behavior changes in a way the
// embedded asset bytes do not capture (goldmark options, wrapPage markup, report
// component semantics, chroma theme selection). Bumping it invalidates every
// cached page.
const renderSchemaVersion = "58"

// fingerprintOnce memoizes the renderer fingerprint — immutable per process.
var fingerprintOnce = sync.OnceValue(func() string {
	h := sha256.New()
	h.Write([]byte(renderSchemaVersion))
	h.Write([]byte{0})

	// Hash every embedded asset in a stable order so adding or editing any
	// asset (base.css, copy.js, theme.js, future assets) changes the
	// fingerprint automatically — no per-asset list to keep in sync.
	var names []string
	_ = fs.WalkDir(assetsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			names = append(names, path)
		}
		return nil
	})
	slices.Sort(names)
	for _, name := range names {
		b, err := assetsFS.ReadFile(name)
		if err != nil {
			panic(err)
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}

	// Include generated highlight CSS so a chroma theme/version change invalidates.
	h.Write([]byte(highlightCSS("")))

	return hex.EncodeToString(h.Sum(nil))
})

// Fingerprint returns a stable hex digest of everything that affects rendered
// output but is independent of the Markdown source: the schema version, all
// embedded assets, the generated highlight CSS, and the output-affecting render
// options (via cacheTag). When it changes, cached pages are stale even if the
// source mtime did not change.
func Fingerprint(opts Options) string { return fingerprintOnce() + opts.cacheTag() }
