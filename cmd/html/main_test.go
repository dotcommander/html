package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCLIStreams(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "help", args: []string{"--help"}, wantStdout: "Usage:"},
		{name: "parser error", args: []string{"--not-a-flag"}, wantCode: 1, wantStderr: "html: flag provided but not defined: -not-a-flag"},
		{name: "runtime error", wantCode: 1, wantStderr: "html: no input:"},
		{name: "version", args: []string{"--version"}, wantStdout: "html devel\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=TestCLIHelperProcess", "--")
			cmd.Args = append(cmd.Args, tt.args...)
			cmd.Env = append(os.Environ(), "HTML_CLI_HELPER=1")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			code := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("run helper: %v", err)
			}
			if code != tt.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, tt.wantCode, stderr.String())
			}
			if tt.wantStdout != "" && !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Fatalf("stdout %q does not contain %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("stderr %q does not contain %q", stderr.String(), tt.wantStderr)
			}
			if strings.Count(stdout.String(), "Usage:") > 1 || strings.Contains(stderr.String(), "Usage:") {
				t.Fatalf("usage written to wrong stream or more than once; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			if tt.wantCode != 0 && strings.Count(strings.TrimSpace(stderr.String()), "\n") != 0 {
				t.Fatalf("error diagnostic was not a single line: %q", stderr.String())
			}
		})
	}
}

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv("HTML_CLI_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"html"}, os.Args[i+1:]...)
			main()
			return
		}
	}
	os.Exit(2)
}
