package cache

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
)

const publicationVersion = "html-sha256-v1:"

func publicationFingerprint(html []byte, fingerprint string) string {
	return fmt.Sprintf("%s%x:%s", publicationVersion, sha256.Sum256(html), fingerprint)
}

// Each file is atomically replaced, but the pair is not. Binding the sidecar to
// the page bytes prevents an interleaved template render from becoming a false
// cache hit. Missing/legacy/malformed metadata fails closed without reading HTML.
func matchesPublication(path, fingerprint string) (bool, error) {
	metadata, err := os.ReadFile(fpPath(path))
	if err != nil {
		return false, nil
	}
	rest, ok := strings.CutPrefix(string(metadata), publicationVersion)
	if !ok {
		return false, nil
	}
	digest, gotFingerprint, ok := strings.Cut(rest, ":")
	if !ok || len(digest) != sha256.Size*2 || gotFingerprint != fingerprint {
		return false, nil
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cache: read html: %w", err)
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return false, fmt.Errorf("cache: hash html: %w", err)
	}
	return digest == fmt.Sprintf("%x", hash.Sum(nil)), nil
}
