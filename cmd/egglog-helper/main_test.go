package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func cliFixture(t *testing.T, source string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module sample\n\ngo 1.26\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), []byte("package sample\n"+source), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
}

func requireEngine(t *testing.T) {
	t.Helper()
	if os.Getenv("EGGLOG_BIN") != "" {
		return
	}
	_, current, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(current), "..", "..", ".tools", "bin", "egglog.exe")
	if _, err := os.Stat(path); err != nil {
		t.Skip("build egglog first or set EGGLOG_BIN")
	}
	t.Setenv("EGGLOG_BIN", path)
}

func TestCheckTextAndExitCode(t *testing.T) {
	cliFixture(t, "func f(a,b bool) { if a || (a && b) {} }\n")
	requireEngine(t)
	var out, errOut bytes.Buffer
	code := run([]string{"check", "./..."}, &out, &errOut)
	if code != 1 || errOut.Len() != 0 || !strings.Contains(out.String(), "after:  a") || !strings.Contains(out.String(), "sample.go:2:") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestCheckJSONAndCleanProject(t *testing.T) {
	cliFixture(t, "func f(a bool) { if a || a {} }\n")
	requireEngine(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"check", "-json", "./..."}, &out, &errOut); code != 1 {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
	var diagnostics []struct {
		After string `json:"after"`
	}
	if err := json.Unmarshal(out.Bytes(), &diagnostics); err != nil || len(diagnostics) != 1 || diagnostics[0].After != "a" {
		t.Fatalf("JSON=%q error=%v", out.String(), err)
	}
	cliFixture(t, "func f(a bool) { if a {} }\n")
	out.Reset()
	errOut.Reset()
	if code := run([]string{"check", "./..."}, &out, &errOut); code != 0 || out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("clean code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestInvalidArgumentsAndMissingEngineExitTwo(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"unknown"}, &out, &errOut); code != 2 {
		t.Fatalf("invalid command code=%d", code)
	}
	cliFixture(t, "func f(a bool) { if a || a {} }\n")
	t.Setenv("EGGLOG_BIN", filepath.Join(t.TempDir(), "missing"))
	out.Reset()
	errOut.Reset()
	if code := run([]string{"check", "./..."}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "egglog") {
		t.Fatalf("missing engine code=%d stderr=%q", code, errOut.String())
	}
}

func TestFixtureHonorsConfiguredEgglogPath(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "egglog.exe")
	t.Setenv("EGGLOG_BIN", custom)
	cliFixture(t, "func f(a bool) { if a {} }\n")
	if got := os.Getenv("EGGLOG_BIN"); got != custom {
		t.Fatalf("configured engine path overwritten: %q", got)
	}
}
