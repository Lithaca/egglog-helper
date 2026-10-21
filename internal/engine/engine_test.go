package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func engineBinary(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("EGGLOG_BIN"); path != "" {
		return path
	}
	_, current, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(current), "..", "..", ".tools", "bin", "egglog.exe")
	if _, err := os.Stat(path); err != nil {
		t.Skip("build egglog first or set EGGLOG_BIN")
	}
	return path
}

func TestRealEgglogSimplifiesBooleanExpressions(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	rules, err := os.ReadFile(filepath.Join(filepath.Dir(current), "..", "..", "rules", "bool.egg"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, input, want string }{
		{"absorption", `(Or (Var "v0") (And (Var "v0") (Var "v1")))`, `(Var "v0")`},
		{"complement", `(Or (And (Var "v0") (Var "v1")) (And (Var "v0") (Not (Var "v1"))))`, `(Var "v0")`},
		{"contradiction", `(And (Var "v0") (Not (Var "v0")))`, `(False)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Simplify(context.Background(), engineBinary(t), string(rules), tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMissingEngineIsAnError(t *testing.T) {
	_, err := Simplify(context.Background(), filepath.Join(t.TempDir(), "missing-egglog"), "", `(True)`)
	if err == nil || !strings.Contains(err.Error(), "egglog") {
		t.Fatalf("missing egglog error = %v", err)
	}
}

func TestCancelledRunIsAnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Simplify(ctx, engineBinary(t), "", `(True)`)
	if err == nil {
		t.Fatal("cancelled execution was accepted")
	}
}
