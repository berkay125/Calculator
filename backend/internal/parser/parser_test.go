package parser

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// positionalError is the contract the API layer relies on: every parser error
// has an error code and the position of the offending character.
type positionalError interface {
	error
	Code() string
	Pos() int
}

func mustParse(t *testing.T, src string) Node {
	t.Helper()
	n, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return n
}

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"2", "2"},
		{"12.50", "12.50"},
		{".5", ".5"},
		// Left associativity.
		{"1 + 2 + 3", "(+ (+ 1 2) 3)"},
		{"1 - 2 - 3", "(- (- 1 2) 3)"},
		{"1 - 2 + 3", "(+ (- 1 2) 3)"},
		{"8 / 4 / 2", "(/ (/ 8 4) 2)"},
		{"2 * 3 / 4", "(/ (* 2 3) 4)"},
		// Right associativity.
		{"2 ^ 3 ^ 2", "(^ 2 (^ 3 2))"},
		// Precedence between levels.
		{"1 + 2 * 3", "(+ 1 (* 2 3))"},
		{"1 * 2 + 3", "(+ (* 1 2) 3)"},
		{"2 * 3 ^ 2", "(* 2 (^ 3 2))"},
		{"2 ^ 3 * 2", "(* (^ 2 3) 2)"},
		{"1 + 2 * 3 - 4 / 5 ^ 6", "(- (+ 1 (* 2 3)) (/ 4 (^ 5 6)))"},
		// Unary minus binds looser than ^ and tighter than * and +.
		{"-2 ^ 2", "(- (^ 2 2))"},
		{"-2 * 3", "(* (- 2) 3)"},
		{"-2 + 3", "(+ (- 2) 3)"},
		{"2 * -3", "(* 2 (- 3))"},
		{"2 - -3", "(- 2 (- 3))"},
		{"2 ^ -1", "(^ 2 (- 1))"},
		{"2 ^ -1 * 3", "(* (^ 2 (- 1)) 3)"},
		{"2 ^ -3 ^ 2", "(^ 2 (- (^ 3 2)))"},
		{"--2", "(- (- 2))"},
		// Parentheses are kept as group nodes.
		{"2 * (3 + 4) ^ 2", "(* 2 (^ (group (+ 3 4)) 2))"},
		{"(2)", "(group 2)"},
		{"((2))", "(group (group 2))"},
		{"-(2 + 3)", "(- (group (+ 2 3)))"},
		{"(1 - 2) - 3", "(- (group (- 1 2)) 3)"},
		{"1 - (2 - 3)", "(- 1 (group (- 2 3)))"},
		// Percent is a postfix operator that binds tightest. The percent rule
		// is applied by the evaluator; the parser only builds the node.
		{"50%", "(% 50)"},
		{"200+10%", "(+ 200 (% 10))"},
		{"100+(10%)", "(+ 100 (group (% 10)))"},
		{"100+-10%", "(+ 100 (- (% 10)))"},
		{"100 - --10%", "(- 100 (- (- (% 10))))"},
		{"(5+5)%", "(% (group (+ 5 5)))"},
		{"100+(5+5)%", "(+ 100 (% (group (+ 5 5))))"},
		{"200+10%*2", "(+ 200 (* (% 10) 2))"},
		{"200+10%+10%", "(+ (+ 200 (% 10)) (% 10))"},
		{"2^10%", "(^ 2 (% 10))"},
		{"10%^2", "(^ (% 10) 2)"},
		{"50%%", "(% (% 50))"},
		// Variables.
		{"Ans", "Ans"},
		{"Ans*2", "(* Ans 2)"},
		{"-Ans%", "(- (% Ans))"},
		{"x", "x"},
		// Function calls. The parser does not know function names.
		{"sqrt(9)", "(call sqrt 9)"},
		{"foo(1, 2)", "(call foo 1 2)"},
		{"f()", "(call f)"},
		{"sqrt (9)", "(call sqrt 9)"},
		{"group(9)", "(call group 9)"},
		{"sqrt(sqrt(16))", "(call sqrt (call sqrt 16))"},
		{"2*sqrt(1+3)^2", "(* 2 (^ (call sqrt (+ 1 3)) 2))"},
		{"-sqrt(4)", "(- (call sqrt 4))"},
		{"sqrt(4)%", "(% (call sqrt 4))"},
		{"f((1), -2, 3%)", "(call f (group 1) (- 2) (% 3))"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := mustParse(t, tt.in).String(); got != tt.want {
				t.Errorf("Parse(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

// withPositions prints n like String, with every node tagged by its Position.
func withPositions(t *testing.T, n Node) string {
	t.Helper()
	switch n := n.(type) {
	case *Number:
		return fmt.Sprintf("%s@%d", n.Text, n.Position())
	case *Variable:
		return fmt.Sprintf("%s@%d", n.Name, n.Position())
	case *Call:
		s := fmt.Sprintf("(call %s@%d", n.Name, n.Position())
		for _, arg := range n.Args {
			s += " " + withPositions(t, arg)
		}
		return s + ")"
	case *Percent:
		return fmt.Sprintf("(%%@%d %s)", n.Position(), withPositions(t, n.X))
	case *Unary:
		return fmt.Sprintf("(%s@%d %s)", n.Op, n.Position(), withPositions(t, n.X))
	case *Binary:
		return fmt.Sprintf("(%s@%d %s %s)", n.Op, n.Position(), withPositions(t, n.L), withPositions(t, n.R))
	case *Group:
		return fmt.Sprintf("(group@%d %s)", n.Position(), withPositions(t, n.Inner))
	}
	t.Fatalf("unknown node type %T", n)
	return ""
}

func TestParsePositions(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"2 * (3 + 4) ^ 2", "(*@2 2@0 (^@12 (group@4 (+@7 3@5 4@9)) 2@14))"},
		{" -2 ^ -1", "(-@1 (^@4 2@2 (-@6 1@7)))"},
		{"12.5-.5", "(-@4 12.5@0 .5@5)"},
		{"Ans + sqrt(9, 10%)", "(+@4 Ans@0 (call sqrt@6 9@11 (%@16 10@14)))"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := withPositions(t, mustParse(t, tt.in)); got != tt.want {
				t.Errorf("Parse(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		in  string
		pos int
		msg string
	}{
		{"", 0, "empty expression"},
		{"   ", 3, "empty expression"},
		{"2 +", 3, "unexpected end of expression"},
		{"2 ^", 3, "unexpected end of expression"},
		{"-", 1, "unexpected end of expression"},
		{"(2 +", 4, "unexpected end of expression"},
		{"(2", 0, "unclosed parenthesis"},
		{"(1 + (2 * 3)", 0, "unclosed parenthesis"},
		{"((1)", 0, "unclosed parenthesis"},
		{"2)", 1, "unexpected ')'"},
		{"()", 1, "unexpected ')'"},
		{"2 3", 2, "unexpected '3'"},
		{"(2 3)", 3, "unexpected '3'"},
		{"* 2", 0, "unexpected '*'"},
		{"2 + * 3", 4, "unexpected '*'"},
		{"+2", 0, "unexpected '+'"},
		{"%", 0, "unexpected '%'"},
		{"2 + %", 4, "unexpected '%'"},
		{"1, 2", 1, "unexpected ','"},
		{"(1, 2)", 2, "unexpected ','"},
		// Function calls.
		{"sqrt(", 5, "unexpected end of expression"},
		{"sqrt(9", 4, "unclosed parenthesis"},
		{"sqrt(1,", 7, "unexpected end of expression"},
		{"f(1,)", 4, "unexpected ')'"},
		{"f(,1)", 2, "unexpected ','"},
		{"f(1 2)", 4, "unexpected '2'"},
		{"sqrt 9", 5, "unexpected '9'"},
		// Implicit multiplication and scientific notation are not supported.
		{"2(3)", 1, "unexpected '('"},
		{"(2)(3)", 3, "unexpected '('"},
		{"2x", 1, "unexpected 'x'"},
		{"2 sqrt(4)", 2, "unexpected 'sqrt'"},
		{"Ans Ans", 4, "unexpected 'Ans'"},
		{"1e5", 1, "unexpected 'e5'"},
		// Lexer errors pass through. Tokens are read on demand, so the leftmost
		// error wins whether it is lexical or syntactic.
		{"2 $ 3", 2, "unexpected character '$'"},
		{"1 + 2.3.4", 7, "number has more than one decimal point"},
		{"-$", 1, "unexpected character '$'"},
		{"($", 1, "unexpected character '$'"},
		{"(1)$", 3, "unexpected character '$'"},
		{"1%$", 2, "unexpected character '$'"},
		{"Ans$", 3, "unexpected character '$'"},
		{"f(1,$", 4, "unexpected character '$'"},
		{") $", 0, "unexpected ')'"},
		{"$ )", 0, "unexpected character '$'"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			n, err := Parse(tt.in)
			var synErr *SyntaxError
			if !errors.As(err, &synErr) {
				t.Fatalf("Parse(%q) = %v, %v; want SyntaxError", tt.in, n, err)
			}
			if synErr.Pos() != tt.pos || synErr.Error() != tt.msg {
				t.Errorf("Parse(%q) error = %q at %d, want %q at %d", tt.in, synErr.Error(), synErr.Pos(), tt.msg, tt.pos)
			}
		})
	}
}

func TestParseNesting(t *testing.T) {
	nested := func(depth int) string {
		return strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth)
	}
	nestedCalls := func(depth int) string {
		return strings.Repeat("sqrt(", depth) + "1" + strings.Repeat(")", depth)
	}

	for _, in := range []string{
		nested(MaxDepth),
		nestedCalls(MaxDepth),
		strings.Repeat("(sqrt(", MaxDepth/2) + "1" + strings.Repeat("))", MaxDepth/2),
		// Depth counts open parentheses only, so siblings do not add up.
		strings.Repeat(nested(MaxDepth)+"+", 3) + "1",
		"f(" + nested(MaxDepth-1) + ", " + nested(MaxDepth-1) + ")",
	} {
		if _, err := Parse(in); err != nil {
			t.Errorf("Parse(%.20q...): %v", in, err)
		}
	}

	tests := []struct {
		name string
		in   string
		pos  int
	}{
		{"one level too deep", nested(MaxDepth + 1), MaxDepth},
		{"far too deep", nested(10 * MaxDepth), MaxDepth},
		{"inside a sibling group", nested(1) + "+" + nested(MaxDepth+1), len("(1)+") + MaxDepth},
		// Function call parentheses count too; the error points at the "(".
		{"nested calls", nestedCalls(MaxDepth + 1), MaxDepth*len("sqrt(") + len("sqrt")},
		{"call inside groups", strings.Repeat("(", MaxDepth) + "f(1)" + strings.Repeat(")", MaxDepth), MaxDepth + len("f")},
		// The limit is hit before the lexer reaches the invalid character.
		{"before a lexer error", strings.Repeat("(", MaxDepth+1) + "$", MaxDepth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.in)
			var nestErr *NestingError
			if !errors.As(err, &nestErr) || nestErr.Pos() != tt.pos {
				t.Errorf("Parse error = %v, want NestingError at %d", err, tt.pos)
			}
		})
	}
}

func TestErrorCodes(t *testing.T) {
	tests := []struct {
		in   string
		code string
		msg  string
	}{
		{"2 +", "syntax_error", "unexpected end of expression"},
		{strings.Repeat("(", MaxDepth+1), "nesting_too_deep", "parentheses are nested more than 100 levels deep"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			_, err := Parse(tt.in)
			var posErr positionalError
			if !errors.As(err, &posErr) {
				t.Fatalf("Parse(%q) error = %v, want a positional error", tt.in, err)
			}
			if posErr.Code() != tt.code || posErr.Error() != tt.msg {
				t.Errorf("Parse(%q) error = %s %q, want %s %q", tt.in, posErr.Code(), posErr.Error(), tt.code, tt.msg)
			}
		})
	}
}
