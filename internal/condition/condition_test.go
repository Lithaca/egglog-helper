package condition

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

func conditionFromSource(t *testing.T, source string) (*Condition, bool) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "sample.go", "package sample\n"+source, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}, Scopes: map[ast.Node]*types.Scope{}}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("sample", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	var expr ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		if stmt, ok := n.(*ast.IfStmt); ok && expr == nil {
			expr = stmt.Cond
		}
		return true
	})
	if expr == nil {
		t.Fatal("fixture lacks if condition")
	}
	return Parse(expr, info, pkg)
}

func TestVerifyUsefulBooleanSimplifications(t *testing.T) {
	tests := []struct {
		name, source, extracted, want string
	}{
		{"absorption", "func f(a,b bool) { if a || (a && b) {} }", `(Var "v0")`, "a"},
		{"contradiction", "func f(a bool) { if a && !a {} }", "(False)", "false"},
		{"redundant constant", "func f(a bool) { if a && true {} }", `(Var "v0")`, "a"},
		{"repeated comparison", "func f(x int) { if x == 0 || x == 0 {} }", `(Var "v0")`, "x == 0"},
		{"shadowed true", "func f(true bool) { if true || true {} }", `(Var "v0")`, "true"},
		{"named boolean", "type Flag bool; func f(a Flag) { if a || a {} }", `(Var "v0")`, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, ok := conditionFromSource(t, tt.source)
			if !ok {
				t.Fatal("safe condition rejected")
			}
			got, ok := c.Verify(tt.extracted)
			if !ok || got != tt.want {
				t.Fatalf("Verify(%q) = %q, %v; want %q", tt.extracted, got, ok, tt.want)
			}
		})
	}
}

func TestRejectUnsafeConditions(t *testing.T) {
	tests := []string{
		"func f(a bool) bool { return a }; func g() { if f(true) || true {} }",
		"type P struct { ok bool }; func f(p *P) { if p != nil && p.ok {} }",
		"func f(a []bool) { if a[0] || true {} }",
		"func f(ch chan bool) { if <-ch || true {} }",
		"func f(a, b any) { if a == b || true {} }",
	}
	for _, source := range tests {
		if _, ok := conditionFromSource(t, source); ok {
			t.Fatalf("unsafe condition accepted: %s", source)
		}
	}
}

func TestRejectInvalidExtraction(t *testing.T) {
	c, ok := conditionFromSource(t, "func f(a,b bool) { if a || b {} }")
	if !ok {
		t.Fatal("safe condition rejected")
	}
	for _, extracted := range []string{`(Var "v2")`, `(Var "v0")`, `(And (Var "v0")`, "garbage"} {
		if got, ok := c.Verify(extracted); ok {
			t.Fatalf("invalid extraction %q accepted as %q", extracted, got)
		}
	}
}

func TestShadowedBooleanConstantsKeepTheirMeaning(t *testing.T) {
	for _, tt := range []struct{ source, extracted, want string }{
		{"func f(false, a bool) { if a && !a {} }", "(False)", "0 != 0"},
		{"func f(true, a bool) { if a || !a {} }", "(True)", "0 == 0"},
	} {
		c, ok := conditionFromSource(t, tt.source)
		if !ok {
			t.Fatal("safe condition rejected")
		}
		got, ok := c.Verify(tt.extracted)
		if !ok || got != tt.want {
			t.Fatalf("Verify(%q) = %q, %v; want %q", tt.extracted, got, ok, tt.want)
		}
	}
}

func TestAnalyzeDistinguishesNoImprovementFromBadExtraction(t *testing.T) {
	c, ok := conditionFromSource(t, "func f(a,b bool) { if a || b {} }")
	if !ok {
		t.Fatal("safe condition rejected")
	}
	if _, changed, err := c.Analyze(c.Input()); err != nil || changed {
		t.Fatalf("unchanged extraction: changed=%v err=%v", changed, err)
	}
	for _, extracted := range []string{"garbage", `(Var "v2")`, "(False)"} {
		if _, _, err := c.Analyze(extracted); err == nil {
			t.Fatalf("bad extraction %q did not return an error", extracted)
		}
	}
}
