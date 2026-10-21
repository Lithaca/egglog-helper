package check

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"os/exec"
	"sort"
	"strings"
	"time"

	"egglog-helper/internal/condition"
	"egglog-helper/internal/engine"
	"egglog-helper/rules"
	"golang.org/x/tools/go/packages"
)

type Diagnostic struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Before string `json:"before"`
	After  string `json:"after"`
}

func Run(ctx context.Context, patterns []string, enginePath string) ([]Diagnostic, error) {
	resolvedEngine, err := exec.LookPath(enginePath)
	if err != nil {
		return nil, fmt.Errorf("find egglog executable %q: %w", enginePath, err)
	}
	enginePath = resolvedEngine
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	loaded, err := packages.Load(&packages.Config{Context: ctx, Mode: packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load Go packages: %w", err)
	}
	if len(loaded) == 0 {
		return nil, fmt.Errorf("no Go packages matched %s", strings.Join(patterns, ", "))
	}
	var diagnostics []Diagnostic
	cache := make(map[string]string)
	for _, pkg := range loaded {
		if len(pkg.Errors) != 0 {
			return nil, fmt.Errorf("load Go package %s: %v", pkg.PkgPath, pkg.Errors[0])
		}
		if pkg.TypesInfo == nil || pkg.Types == nil || pkg.Fset == nil {
			return nil, fmt.Errorf("missing type information for Go package %s", pkg.PkgPath)
		}
		for _, file := range pkg.Syntax {
			if ast.IsGenerated(file) {
				continue
			}
			var scanErr error
			ast.Inspect(file, func(n ast.Node) bool {
				if scanErr != nil || n == nil {
					return false
				}
				var expr ast.Expr
				switch stmt := n.(type) {
				case *ast.IfStmt:
					expr = stmt.Cond
				case *ast.ForStmt:
					expr = stmt.Cond
				}
				if expr == nil {
					return true
				}
				c, ok := condition.Parse(expr, pkg.TypesInfo, pkg.Types)
				if !ok {
					return true
				}
				input := c.Input()
				extracted, hit := cache[input]
				if !hit {
					runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
					var err error
					extracted, err = engine.Simplify(runCtx, enginePath, rules.Boolean, input)
					cancel()
					if err != nil {
						scanErr = err
						return false
					}
					cache[input] = extracted
				}
				after, changed, err := c.Analyze(extracted)
				if err != nil {
					scanErr = fmt.Errorf("egglog returned an invalid expression: %w", err)
					return false
				}
				if !changed {
					return true
				}
				var b bytes.Buffer
				if err := format.Node(&b, pkg.Fset, expr); err != nil {
					scanErr = err
					return false
				}
				position := pkg.Fset.Position(expr.Pos())
				diagnostics = append(diagnostics, Diagnostic{File: position.Filename, Line: position.Line, Column: position.Column, Before: b.String(), After: after})
				return true
			})
			if scanErr != nil {
				return nil, fmt.Errorf("analyze %s: %w", pkg.PkgPath, scanErr)
			}
		}
	}
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].File != diagnostics[j].File {
			return diagnostics[i].File < diagnostics[j].File
		}
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		return diagnostics[i].Column < diagnostics[j].Column
	})
	return diagnostics, nil
}
