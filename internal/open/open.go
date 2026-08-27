// Package open launches a file or URL in the operating system's default
// application. It resolves the launcher on PATH and returns a clear error when
// none is found, rather than spawning a PATH-shadowed or missing binary.
package open

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Open launches path, fire-and-forget. When command is non-empty it is resolved
// on PATH and used as the launcher (the configured open_command); otherwise the
// OS default handler is used. The path is passed as a discrete argv element
// (never shell-interpolated), so spaces and shell metacharacters are safe.
func Open(path, command string) error {
	if command != "" {
		return launch(command, path)
	}
	switch runtime.GOOS {
	case "windows":
		// `cmd /c start "" <path>` — the empty "" is the (required) window title.
		//nolint:gosec,noctx // Fixed launcher; the detached browser must outlive this call.
		_, err := start(exec.Command("cmd", "/c", "start", "", path))
		return err
	case "darwin":
		return launch("open", path)
	default:
		// Linux/BSD: prefer xdg-open, fall back to a stray `open` if present.
		if bin, err := exec.LookPath("xdg-open"); err == nil {
			//nolint:gosec,noctx // Resolved launcher; the detached browser must outlive this call.
			_, err := start(exec.Command(bin, path))
			return err
		}
		return launch("open", path)
	}
}

// launch resolves opener on PATH to an absolute binary and starts it with path.
func launch(opener, path string) error {
	bin, err := exec.LookPath(opener)
	if err != nil {
		return fmt.Errorf("open: no %q launcher found on PATH: %w", opener, err)
	}
	//nolint:gosec,noctx // Resolved launcher; the detached browser must outlive this call.
	_, err = start(exec.Command(bin, path))
	return err
}

// start launches cmd and reaps it asynchronously. The returned channel closes
// after Wait releases the child process resources.
func start(cmd *exec.Cmd) (<-chan struct{}, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	return done, nil
}
