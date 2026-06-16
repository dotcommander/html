package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/dotcommander/html/internal/actions"
	"github.com/dotcommander/html/internal/config"
	"github.com/spf13/cobra"
)

func Execute() error { return newRootCmd().Execute() }

func newRootCmd() *cobra.Command {
	var noOpen, force, safe, plain, markdown bool
	var title, lang, output string
	cmd := &cobra.Command{
		Use:   "html [file]",
		Short: "Render Markdown or piped text to a clean HTML page and open it in the browser",
		Long: "Render a Markdown file — or data piped on stdin (e.g. `tree -d | html`) — to a\n" +
			"self-contained HTML page and open it in the browser. Piped input is auto-\n" +
			"detected as Markdown or plain text; use --plain/--markdown to force a mode.",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true, // don't dump usage text on a runtime error
		SilenceErrors: true, // cmd/html/main.go prints the error
		RunE: func(cmd *cobra.Command, args []string) error {
			if plain && markdown {
				return fmt.Errorf("--plain and --markdown are mutually exclusive")
			}
			// Optional user preferences; a missing file yields a zero Config and
			// reproduces current behavior. A malformed file errors here.
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			opts := actions.Options{
				NoOpen:   noOpen,
				Force:    force,
				Safe:     safe,
				Plain:    plain,
				Markdown: markdown,
				Title:    title,
				Lang:     lang,
				OpenCmd:  cfg.OpenCommand,
				MaxWidth: cfg.MaxWidth,
				Theme:    cfg.DefaultTheme,
				TOC:      cfg.TOC,
				Output:   output,
			}
			switch {
			case len(args) == 1:
				opts.File = args[0]
			case isPiped(cmd.InOrStdin()):
				opts.Stdin = cmd.InOrStdin()
			default:
				return fmt.Errorf("no input: provide a file path or pipe data (e.g. `tree -d | html`)")
			}

			path, err := actions.Run(opts)
			if err != nil {
				return err
			}

			switch opts.Output {
			case "":
				if path != "" {
					fmt.Fprintln(cmd.OutOrStdout(), path)
				}
			case "-":
				html, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("read cache: %w", err)
				}
				_, err = cmd.OutOrStdout().Write(html)
				if err != nil {
					return fmt.Errorf("write stdout: %w", err)
				}
			default:
				html, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("read cache: %w", err)
				}
				if err := os.WriteFile(opts.Output, html, 0o644); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), opts.Output)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&noOpen, "no-open", "n", false, "render only; print the cache path without opening the browser")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "rebuild even if the cached HTML is fresh")
	cmd.Flags().BoolVar(&safe, "safe", false, "disable raw HTML passthrough (safe for untrusted Markdown)")
	cmd.Flags().BoolVarP(&plain, "plain", "p", false, "render input as preformatted plain text, not Markdown")
	cmd.Flags().BoolVarP(&markdown, "markdown", "m", false, "render input as Markdown (overrides stdin auto-detection)")
	cmd.Flags().StringVarP(&title, "title", "t", "stdin", "page title for piped input")
	cmd.Flags().StringVarP(&lang, "lang", "l", "", "syntax-highlight language for plain mode (e.g. go, json; \"text\" = no highlighting)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "write rendered HTML to a path (\"-\" for stdout) instead of caching and opening")
	return cmd
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
