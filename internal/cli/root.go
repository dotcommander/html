package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/dotcommander/html/internal/actions"
	"github.com/dotcommander/html/internal/config"
)

func Execute() error { return newRootCmd().Execute() }

type command struct {
	args   []string
	in     io.Reader
	out    io.Writer
	errOut io.Writer
	ctx    context.Context
}

func newRootCmd() *command {
	return &command{args: os.Args[1:], in: os.Stdin, out: os.Stdout, errOut: os.Stderr, ctx: context.Background()}
}

func (cmd *command) SetArgs(args []string) { cmd.args = args }
func (cmd *command) SetIn(in io.Reader)    { cmd.in = in }
func (cmd *command) SetOut(out io.Writer)  { cmd.out = out }
func (cmd *command) SetErr(out io.Writer)  { cmd.errOut = out }

// Execute adapts CLI input into an actions.Options call and renders its result:
// parse flags, validate their combinations, layer user config, select the
// input source, run, and print.
func (cmd *command) Execute() error {
	fs, v := newRootFlagSet()
	fs.Usage = func() { printUsage(cmd.out, fs) }
	normalized, err := interspersedArgs(fs, cmd.args)
	if err != nil {
		return err
	}
	if err := fs.Parse(normalized); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	args := fs.Args()
	if len(args) > 1 {
		return fmt.Errorf("accepts at most 1 arg(s), received %d", len(args))
	}
	changed := changedFlags(fs)
	if v.version {
		fmt.Fprintln(cmd.out, versionString())
		return nil
	}
	if err := v.validate(changed); err != nil {
		return err
	}
	// Optional user preferences; a missing file yields a zero Config and
	// reproduces current behavior. A malformed file errors here.
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	opts := v.actionsOptions(cfg, changed)
	opts.Context = cmd.ctx
	if err := cmd.selectInput(&opts, args); err != nil {
		return err
	}
	res, err := actions.RunWithResult(opts)
	cmd.printResult(res)
	return err
}

// selectInput binds the command's input source: a file argument when present,
// otherwise piped stdin.
func (cmd *command) selectInput(opts *actions.Options, args []string) error {
	switch {
	case len(args) == 1:
		opts.File = args[0]
	case isPiped(cmd.in):
		opts.Stdin = cmd.in
	default:
		return fmt.Errorf("no input: provide a file path or pipe data (e.g. `tree -d | html`)")
	}
	return nil
}

// printResult writes image diagnostics to stderr and the stdout content or
// document path to stdout, mirroring the render result even when
// actions.RunWithResult also returned an error.
func (cmd *command) printResult(res actions.Result) {
	for _, diagnostic := range res.Diagnostics {
		fmt.Fprintf(cmd.errOut, "html: warning: [%s] image %q was not embedded\n", diagnostic.Code, diagnostic.Destination)
	}
	if res.Stdout != "" {
		fmt.Fprint(cmd.out, res.Stdout)
	} else if res.Path != "" {
		fmt.Fprintln(cmd.out, res.Path)
	}
}
