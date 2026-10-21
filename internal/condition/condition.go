package condition

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"unicode"
)

// Condition is a supported, side-effect-free Go boolean expression.
type Condition struct {
	root      *node
	atoms     []string
	keys      map[string]int
	trueText  string
	falseText string
}

type node struct {
	op    string
	atom  int
	left  *node
	right *node
}

func Parse(expr ast.Expr, info *types.Info, pkg *types.Package) (*Condition, bool) {
	if info == nil || pkg == nil || !isBool(info.TypeOf(expr)) {
		return nil, false
	}
	c := &Condition{keys: make(map[string]int)}
	c.trueText = booleanConstantText(info, expr.Pos(), "true")
	c.falseText = booleanConstantText(info, expr.Pos(), "false")
	root, ok := c.parseGo(expr, info, pkg, 0)
	if !ok || len(c.atoms) > 8 || size(root) > 63 {
		return nil, false
	}
	c.root = root
	return c, true
}

func booleanConstantText(info *types.Info, pos token.Pos, name string) string {
	fallback := "0 == 0"
	if name == "false" {
		fallback = "0 != 0"
	}
	var innermost *types.Scope
	for _, scope := range info.Scopes {
		if scope.Contains(pos) && (innermost == nil || scope.End()-scope.Pos() < innermost.End()-innermost.Pos()) {
			innermost = scope
		}
	}
	if innermost == nil {
		return fallback
	}
	_, object := innermost.LookupParent(name, pos)
	if object == types.Universe.Lookup(name) {
		return name
	}
	return fallback
}

func isBool(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := types.Unalias(t).Underlying().(*types.Basic)
	return ok && b.Info()&types.IsBoolean != 0
}

func isLocalVar(id *ast.Ident, info *types.Info, pkg *types.Package) bool {
	v, ok := info.Uses[id].(*types.Var)
	return ok && !v.IsField() && v.Pkg() == pkg && v.Parent() != pkg.Scope()
}

func source(expr ast.Expr) string {
	var b bytes.Buffer
	if err := format.Node(&b, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return b.String()
}

func (c *Condition) leaf(key, spelling string) *node {
	if index, ok := c.keys[key]; ok {
		return &node{op: "Var", atom: index}
	}
	index := len(c.atoms)
	c.keys[key] = index
	c.atoms = append(c.atoms, spelling)
	return &node{op: "Var", atom: index}
}

func (c *Condition) parseGo(expr ast.Expr, info *types.Info, pkg *types.Package, depth int) (*node, bool) {
	if depth > 63 || !isBool(info.TypeOf(expr)) {
		return nil, false
	}
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return c.parseGo(e.X, info, pkg, depth+1)
	case *ast.Ident:
		if obj, ok := info.Uses[e].(*types.Const); ok && obj.Parent() == types.Universe && (e.Name == "true" || e.Name == "false") {
			if e.Name == "true" {
				return &node{op: "True"}, true
			}
			return &node{op: "False"}, true
		}
		if isLocalVar(e, info, pkg) {
			return c.leaf("var:"+fmt.Sprintf("%p", info.Uses[e]), e.Name), true
		}
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			x, ok := c.parseGo(e.X, info, pkg, depth+1)
			return &node{op: "Not", left: x}, ok
		}
	case *ast.BinaryExpr:
		switch e.Op {
		case token.LAND, token.LOR:
			left, ok1 := c.parseGo(e.X, info, pkg, depth+1)
			right, ok2 := c.parseGo(e.Y, info, pkg, depth+1)
			if !ok1 || !ok2 {
				return nil, false
			}
			if e.Op == token.LAND {
				return &node{op: "And", left: left, right: right}, true
			}
			return &node{op: "Or", left: left, right: right}, true
		case token.EQL, token.NEQ:
			if !safeComparison(e, info, pkg) {
				return nil, false
			}
			// Keep == and != together as complementary propositions.
			key := source(e.X) + " == " + source(e.Y)
			x := c.leaf("comparison:"+key, key)
			if e.Op == token.NEQ {
				return &node{op: "Not", left: x}, true
			}
			return x, true
		}
	}
	return nil, false
}

func safeComparison(e *ast.BinaryExpr, info *types.Info, pkg *types.Package) bool {
	if !pureComparisonOperand(e.X, info, pkg) || !pureComparisonOperand(e.Y, info, pkg) {
		return false
	}
	if isNil(e.X, info) || isNil(e.Y, info) {
		// Comparing a value with nil cannot invoke a user operation or panic.
		return true
	}
	return scalarComparisonType(info.TypeOf(e.X)) && scalarComparisonType(info.TypeOf(e.Y))
}

func isNil(expr ast.Expr, info *types.Info) bool {
	if parenthesized, ok := expr.(*ast.ParenExpr); ok {
		return isNil(parenthesized.X, info)
	}
	id, ok := expr.(*ast.Ident)
	return ok && info.Uses[id] == types.Universe.Lookup("nil")
}

func scalarComparisonType(t types.Type) bool {
	if t == nil {
		return false
	}
	basic, ok := types.Unalias(t).Underlying().(*types.Basic)
	return ok && basic.Info()&(types.IsBoolean|types.IsInteger|types.IsString) != 0
}

func pureComparisonOperand(expr ast.Expr, info *types.Info, pkg *types.Package) bool {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return pureComparisonOperand(e.X, info, pkg)
	case *ast.Ident:
		if isLocalVar(e, info, pkg) {
			return true
		}
		if obj, ok := info.Uses[e].(*types.Const); ok {
			return obj.Parent() == types.Universe || obj.Parent() == pkg.Scope()
		}
		return e.Name == "nil" && info.Uses[e] == types.Universe.Lookup("nil")
	case *ast.BasicLit:
		return true
	}
	return false
}

func size(n *node) int {
	if n == nil {
		return 0
	}
	return 1 + size(n.left) + size(n.right)
}

func (c *Condition) Input() string { return encode(c.root) }

func encode(n *node) string {
	switch n.op {
	case "Var":
		return fmt.Sprintf("(Var %q)", fmt.Sprintf("v%d", n.atom))
	case "True", "False":
		return "(" + n.op + ")"
	case "Not":
		return "(Not " + encode(n.left) + ")"
	default:
		return "(" + n.op + " " + encode(n.left) + " " + encode(n.right) + ")"
	}
}

func (c *Condition) Verify(extracted string) (string, bool) {
	suggestion, changed, _ := c.Analyze(extracted)
	return suggestion, changed
}

// Analyze validates an engine extraction before offering a shorter Go expression.
func (c *Condition) Analyze(extracted string) (string, bool, error) {
	parts, err := tokenize(extracted)
	if err != nil {
		return "", false, err
	}
	position := 0
	n, err := parseNode(parts, &position, len(c.atoms), 0)
	if err != nil {
		return "", false, err
	}
	if position != len(parts) {
		return "", false, fmt.Errorf("extra tokens in extraction")
	}
	for assignment := 0; assignment < 1<<len(c.atoms); assignment++ {
		if evaluate(c.root, assignment) != evaluate(n, assignment) {
			return "", false, fmt.Errorf("extraction changes the condition's truth value")
		}
	}
	if size(n) >= size(c.root) {
		return "", false, nil
	}
	result := c.render(n, 0)
	if _, err := parser.ParseExpr(result); err != nil {
		return "", false, fmt.Errorf("invalid Go suggestion: %w", err)
	}
	return result, true, nil
}

func evaluate(n *node, bits int) bool {
	switch n.op {
	case "True":
		return true
	case "False":
		return false
	case "Var":
		return bits&(1<<n.atom) != 0
	case "Not":
		return !evaluate(n.left, bits)
	case "And":
		return evaluate(n.left, bits) && evaluate(n.right, bits)
	default:
		return evaluate(n.left, bits) || evaluate(n.right, bits)
	}
}

func precedence(n *node) int {
	switch n.op {
	case "Or":
		return 1
	case "And":
		return 2
	case "Not":
		return 3
	default:
		return 4
	}
}

func (c *Condition) render(n *node, parent int) string {
	var s string
	switch n.op {
	case "Var":
		s = c.atoms[n.atom]
	case "True":
		s = c.trueText
	case "False":
		s = c.falseText
	case "Not":
		if n.left.op == "Var" && strings.ContainsAny(c.atoms[n.left.atom], " =!<>") {
			s = "!(" + c.render(n.left, 0) + ")"
		} else {
			s = "!" + c.render(n.left, precedence(n))
		}
	case "And":
		s = c.render(n.left, precedence(n)) + " && " + c.render(n.right, precedence(n))
	case "Or":
		s = c.render(n.left, precedence(n)) + " || " + c.render(n.right, precedence(n))
	}
	if precedence(n) < parent {
		return "(" + s + ")"
	}
	return s
}

func tokenize(s string) ([]string, error) {
	var out []string
	for i := 0; i < len(s); {
		switch {
		case unicode.IsSpace(rune(s[i])):
			i++
		case s[i] == '(' || s[i] == ')':
			out = append(out, s[i:i+1])
			i++
		case s[i] == '"':
			start := i
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' {
					i++
				}
				i++
			}
			if i >= len(s) {
				return nil, fmt.Errorf("unterminated string")
			}
			i++
			out = append(out, s[start:i])
		default:
			start := i
			for i < len(s) && s[i] != '(' && s[i] != ')' && !unicode.IsSpace(rune(s[i])) {
				i++
			}
			out = append(out, s[start:i])
		}
	}
	return out, nil
}

func parseNode(parts []string, position *int, atoms, depth int) (*node, error) {
	if depth > 63 || *position+2 >= len(parts) || parts[*position] != "(" {
		return nil, fmt.Errorf("invalid expression")
	}
	*position++
	op := parts[*position]
	*position++
	n := &node{op: op}
	switch op {
	case "True", "False":
	case "Var":
		if *position >= len(parts) {
			return nil, fmt.Errorf("missing atom")
		}
		name, err := strconv.Unquote(parts[*position])
		if err != nil || !strings.HasPrefix(name, "v") {
			return nil, fmt.Errorf("invalid atom")
		}
		index, err := strconv.Atoi(name[1:])
		if err != nil || index < 0 || index >= atoms {
			return nil, fmt.Errorf("unknown atom")
		}
		n.atom = index
		*position++
	case "Not":
		var err error
		n.left, err = parseNode(parts, position, atoms, depth+1)
		if err != nil {
			return nil, err
		}
	case "And", "Or":
		var err error
		n.left, err = parseNode(parts, position, atoms, depth+1)
		if err != nil {
			return nil, err
		}
		n.right, err = parseNode(parts, position, atoms, depth+1)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported operator %q", op)
	}
	if *position >= len(parts) || parts[*position] != ")" {
		return nil, fmt.Errorf("missing close parenthesis")
	}
	*position++
	return n, nil
}
