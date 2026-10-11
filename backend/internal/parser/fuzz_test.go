package parser

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// FuzzParse checks invariants that must hold for every input:
//   - Parse does not panic.
//   - An error has a parser error code and a position in [0, len(src)].
//   - Every node's position points at the token that created it.
//   - The tree follows the precedence table: wherever the table needs
//     parentheses, the tree holds a Group node.
//   - Parse(infix(tree)) rebuilds the same tree. infix writes Group nodes as
//     parentheses and adds no others, so the shape must come back from the
//     tokens alone.
//
// The seeds run as part of go test. To search for new failing inputs:
//
//	go test ./internal/parser -fuzz=FuzzParse -fuzztime=1m
func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"2 * (3 + 4) ^ 2",
		"1 - 2 + 3 * 4 / 5",
		"-2^2",
		"2^-1",
		"2^3^2",
		"2 ^ -3 ^ 2",
		"--2 * -3",
		"-(2 + 3)",
		"200+10%",
		"100+(10%)",
		"100 - --10%",
		"(5+5)%",
		"10%^2",
		"50%%",
		"Ans + sqrt(9, 10%)",
		"f()",
		"f (1, (2))",
		"12.50 + .5",
		"\t1\r\n+ 2 ",
		"",
		"2 +",
		"(2",
		"2(3)",
		"1e5",
		"1.2.3",
		"5.",
		"2 $ 3",
		"2 × 3",
		strings.Repeat("(", MaxDepth+1) + "1",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		n, err := Parse(src)
		if err != nil {
			checkParseError(t, src, err)
			return
		}
		walk(n, func(n Node) {
			if pos, tok := n.Position(), tokenText(n); pos < 0 || pos >= len(src) || !strings.HasPrefix(src[pos:], tok) {
				t.Errorf("Parse(%q): node %s is at %d, which is not its token %q", src, n, pos, tok)
			}
			if child := misplaced(n); child != nil {
				t.Errorf("Parse(%q): %s in %s needs parentheses by the precedence table", src, child, n)
			}
		})
		text := infix(n)
		again, err := Parse(text)
		if err != nil {
			t.Fatalf("Parse(%q) = %s, but Parse(infix %q): %v", src, n, text, err)
		}
		if again.String() != n.String() {
			t.Errorf("Parse(%q) = %s, but Parse(infix %q) = %s", src, n, text, again)
		}
	})
}

func checkParseError(t *testing.T, src string, err error) {
	t.Helper()
	var posErr positionalError
	if !errors.As(err, &posErr) {
		t.Fatalf("Parse(%q) error = %v, want an error with a code and a position", src, err)
	}
	if code := posErr.Code(); code != "syntax_error" && code != "nesting_too_deep" {
		t.Errorf("Parse(%q) error code = %q", src, code)
	}
	if pos := posErr.Pos(); pos < 0 || pos > len(src) {
		t.Errorf("Parse(%q) error %q at %d, want a position in [0, %d]", src, err, pos, len(src))
	}
}

// walk calls visit for n and for every node below it.
func walk(n Node, visit func(Node)) {
	visit(n)
	switch n := n.(type) {
	case *Call:
		for _, arg := range n.Args {
			walk(arg, visit)
		}
	case *Unary:
		walk(n.X, visit)
	case *Binary:
		walk(n.L, visit)
		walk(n.R, visit)
	case *Percent:
		walk(n.X, visit)
	case *Group:
		walk(n.Inner, visit)
	}
}

// tokenText returns the token that creates n, as written in the input.
func tokenText(n Node) string {
	switch n := n.(type) {
	case *Number:
		return n.Text
	case *Variable:
		return n.Name
	case *Call:
		return n.Name
	case *Unary:
		return n.Op
	case *Binary:
		return n.Op
	case *Percent:
		return "%"
	case *Group:
		return "("
	}
	panic(fmt.Sprintf("unknown node type %T", n))
}

// The precedence table from PLAN.md, written out again so that the fuzz test
// checks the parser against the spec instead of against its own table.
var specPower = map[string]int{"+": 1, "-": 1, "*": 2, "/": 2, "^": 4}

const (
	specUnaryPower   = 3
	specPercentPower = 5
	specAtomPower    = 6 // numbers, variables, calls and groups
)

// power returns how tightly n binds according to the spec.
func power(n Node) int {
	switch n := n.(type) {
	case *Binary:
		return specPower[n.Op]
	case *Unary:
		return specUnaryPower
	case *Percent:
		return specPercentPower
	}
	return specAtomPower
}

// misplaced returns the child of n that could only be there inside
// parentheses, according to the precedence table, or nil. A Group binds like
// an atom, so a child the input wrote in parentheses is never returned.
func misplaced(n Node) Node {
	switch n := n.(type) {
	case *Unary:
		if power(n.X) < specUnaryPower {
			return n.X
		}
	case *Percent:
		if power(n.X) < specPercentPower {
			return n.X
		}
	case *Binary:
		p, rightAssoc := specPower[n.Op], n.Op == "^"
		if l := power(n.L); l < p || l == p && rightAssoc {
			return n.L
		}
		// A minus sign at the start of an operand is always a prefix
		// operator, so 2^-1 needs no parentheses.
		if _, unary := n.R.(*Unary); unary {
			return nil
		}
		if r := power(n.R); r < p || r == p && !rightAssoc {
			return n.R
		}
	}
	return nil
}

// infix prints n as an expression that Parse accepts. Group nodes become
// parentheses and no others are added. No spaces are needed: every operator
// is a single symbol, and two operands are never adjacent.
func infix(n Node) string {
	switch n := n.(type) {
	case *Number:
		return n.Text
	case *Variable:
		return n.Name
	case *Call:
		args := make([]string, len(n.Args))
		for i, arg := range n.Args {
			args[i] = infix(arg)
		}
		return n.Name + "(" + strings.Join(args, ",") + ")"
	case *Unary:
		return n.Op + infix(n.X)
	case *Binary:
		return infix(n.L) + n.Op + infix(n.R)
	case *Percent:
		return infix(n.X) + "%"
	case *Group:
		return "(" + infix(n.Inner) + ")"
	}
	panic(fmt.Sprintf("unknown node type %T", n))
}
