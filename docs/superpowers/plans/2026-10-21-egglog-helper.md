# egglog-helper Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Build a safe Go CLI that suggests simpler equivalent boolean conditions using the official egglog engine.

**Architecture:** `go/packages` loads and type-checks packages. A Go expression adapter admits only side-effect-free supported ASTs, generates an egglog program using internal atom IDs, parses the extraction, checks truth-table equivalence, and reports positions and replacement text. The CLI reports suggestions and never edits the target project.

**Tech Stack:** Go 1.26, `golang.org/x/tools/go/packages`, pinned egglog CLI, PowerShell build script.

**Spec:** `docs/superpowers/specs/2026-10-21-egglog-helper-design.md`

## Global Constraints

- Scan `if` and conditional `for` in Go package patterns.
- Limit each expression to 8 independent atoms and a bounded AST size; fail closed on unsafe nodes.
- Preserve Go's short-circuit and type semantics through AST/type screening.
- Every suggestion must be independently truth-table checked and strictly smaller.
- Text and JSON output; codes 0 (none), 1 (suggestion), 2 (error).
- Engine missing, failed, malformed, or timed out must be a visible error.

## Review Focus

- Shadowed `true` or `false`: treat according to resolved object, not spelling.
- Repeated call, field, index, pointer and channel operations: skip entire condition.
- Build tags and generated files: only scan compiled, human-written files.
- Named boolean variables and comparison atoms: preserve valid Go output and type checks.
- Missing engine or malformed extraction: return error, not an empty suggestion list.

---

### Task 1: Expression adapter and verifier

**Files:** `internal/condition/condition_test.go`, `internal/condition/condition.go`, `go.mod`.

**Interfaces:** `Parse(ast.Expr, *types.Info, *types.Package) (*Condition, bool)` returns a supported condition and safety decision. `Condition.Input()` returns the engine expression and atom map. `Condition.Analyze(string) (string, bool)` parses an extracted expression, confirms atom membership and truth-table equivalence, and returns a printable Go suggestion only when smaller.

- [x] Write table tests for boolean laws, unsupported side effects, shadowing, named bools and comparison atoms. Name the behavior each catches.
- [x] Run `go test ./internal/condition`, observe feature-missing failure.
- [x] Implement AST screening, internal tree parser/printer, equivalence and cost checks.
- [x] Run `go test ./internal/condition`; expect pass.

### Task 2: Real egglog process bridge

**Files:** `internal/engine/engine_test.go`, `internal/engine/engine.go`, `rules/bool.egg`, `scripts/build.ps1`.

**Interfaces:** `Simplify(ctx context.Context, executable string, ruleSource string, expression string) (string, error)` runs egglog on a temporary file and returns the extracted expression, enforcing output validation and timeout through `ctx`.

- [x] Write integration tests for absorption/complement rules and missing-engine/timeout failures.
- [x] Run `go test ./internal/engine`, observe feature-missing failure.
- [x] Implement the bridge and actual egglog rules; pin the engine version in the build script.
- [x] Run `go test ./internal/engine`; expect pass with a real engine.

### Task 3: Package scanner and CLI

**Files:** `internal/check/check_test.go`, `internal/check/check.go`, `cmd/egglog-helper/main_test.go`, `cmd/egglog-helper/main.go`.

**Interfaces:** `check.Run(ctx, patterns, enginePath) ([]Diagnostic, error)` loads packages and reports safe improvements. CLI `check [-json] [patterns...]` formats text or JSON and returns the specified exit code.

- [x] Write end-to-end fixture tests with reducible and unsafe `if`/`for`, generated and build-tagged files, text/JSON output and error exit codes.
- [x] Run targeted Go tests, observe feature-missing failure.
- [x] Implement package loading, diagnostic generation, output and exit codes.
- [x] Run `go test ./...`; expect pass.

### Task 4: Usable local package

**Files:** `README.md`, `examples/sample/go.mod`, `examples/sample/main.go`, `.gitignore`, `scripts/build.ps1`.

- [x] Run tool against example module before completing docs; record missing instructions/packaging behavior.
- [x] Finish reproducible Windows build, version pin, PATH-independent example invocation and Chinese README.
- [x] Run `go test ./...`, `go vet ./...`, build script and example `check ./...` and `check -json ./...`; expect documented behavior.
- [x] Review limitations and final diff; fix important findings and report verification.
