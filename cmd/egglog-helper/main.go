package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"egglog-helper/internal/check"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprintln(stderr, "usage: egglog-helper check [-json] [-egglog path] [Go package patterns...]")
		return 2
	}
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "write JSON diagnostics")
	enginePath := flags.String("egglog", defaultEngine(), "path to egglog executable")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	diagnostics, err := check.Run(context.Background(), flags.Args(), *enginePath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *asJSON {
		if diagnostics == nil {
			diagnostics = []check.Diagnostic{}
		}
		if err := json.NewEncoder(stdout).Encode(diagnostics); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	} else {
		for _, d := range diagnostics {
			fmt.Fprintf(stdout, "%s:%d:%d: condition can be simplified\n  before: %s\n  after:  %s\n", d.File, d.Line, d.Column, d.Before, d.After)
		}
	}
	if len(diagnostics) != 0 {
		return 1
	}
	return 0
}

func defaultEngine() string {
	if path := os.Getenv("EGGLOG_BIN"); path != "" {
		return path
	}
	name := "egglog"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe, err := os.Executable()
	if err != nil {
		return name
	}
	return filepath.Join(filepath.Dir(exe), name)
}
