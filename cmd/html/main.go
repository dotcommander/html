package main

import (
	"fmt"
	"os"

	"github.com/dotcommander/html/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "html:", err)
		os.Exit(1)
	}
}
