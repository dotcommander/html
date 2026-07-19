package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// interspersedArgs allows flags before or after the optional file argument,
// preserves --, and expands grouped boolean shorthands before flag.Parse.
func interspersedArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, 1)
	parsing := true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if parsing && arg == "--" {
			parsing = false
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !parsing || !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 && !strings.Contains(arg, "=") {
			expanded := make([]string, 0, len(arg)-1)
			allBool := true
			for _, short := range arg[1:] {
				f := fs.Lookup(string(short))
				if f == nil {
					allBool = false
					break
				}
				boolean, ok := f.Value.(interface{ IsBoolFlag() bool })
				if !ok || !boolean.IsBoolFlag() {
					allBool = false
					break
				}
				expanded = append(expanded, "-"+string(short))
			}
			if allBool {
				flags = append(flags, expanded...)
				continue
			}
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
		f := fs.Lookup(name)
		if f == nil || strings.Contains(arg, "=") {
			continue
		}
		if boolean, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		} else if f != nil {
			return nil, fmt.Errorf("flag needs an argument: -%s", name)
		}
	}
	return append(append(flags, "--"), positionals...), nil
}

func boolFlag(fs *flag.FlagSet, target *bool, name, shorthand, usage string) {
	fs.BoolVar(target, name, false, usage)
	fs.BoolVar(target, shorthand, false, usage)
}

func stringFlag(fs *flag.FlagSet, target *string, name, shorthand, value, usage string) {
	fs.StringVar(target, name, value, usage)
	fs.StringVar(target, shorthand, value, usage)
}

func changedFlags(fs *flag.FlagSet) map[string]bool {
	changed := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { changed[f.Name] = true })
	return changed
}

func printUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(w, "Render Markdown or piped text to a clean HTML page and open it in the browser")
	fmt.Fprintln(w, "\nUsage:\n  html [flags] [file]\n\nFlags:")
	fs.VisitAll(func(f *flag.Flag) {
		switch f.Name {
		}
		prefix := "--"
		if len(f.Name) == 1 {
			prefix = "-"
		}
		fmt.Fprintf(w, "  %s%s\n    \t%s (default %s)\n", prefix, f.Name, f.Usage, f.DefValue)
	})
}
