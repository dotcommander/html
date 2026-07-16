//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cmd := exec.CommandContext(ctx, "go", "test", "./internal/render", "-run", "^TestGitHubMarkdownConformance$", "-count=1", "-v")
	cmd.Env = append(os.Environ(), "HTML_GITHUB_CONFORMANCE_LIVE=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "qa-github-markdown: %v\n", err)
		os.Exit(1)
	}
}
