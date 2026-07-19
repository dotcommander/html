package cli

import (
	"io"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
)

func versionString() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || localBuildVersion(info.Main.Version) {
		return "html devel"
	}
	return formatVersion(info.Main.Version)
}

var pseudoVersionRE = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+-(0\.)?[0-9]{14}-[0-9a-f]+(\+.*)?$`)

func localBuildVersion(version string) bool {
	return version == "" || version == "(devel)" || strings.Contains(version, "+dirty") || pseudoVersionRE.MatchString(version)
}

func formatVersion(version string) string {
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return "html " + version
}

// isPiped reports whether r carries piped/redirected data rather than an
// interactive terminal. Only an *os.File (e.g. os.Stdin) can be a TTY; any other
// reader (such as a test buffer) is treated as piped.
func isPiped(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return true
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}
