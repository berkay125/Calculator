package eval

import (
	"errors"
	"strings"
	"testing"

	"calculator/internal/arith"
	"calculator/internal/parser"
)

// positionalError is the contract the API layer relies on.
type positionalError interface {
	error
	Code() string
	Pos() int
}

func mustValue(t *testing.T, s string) Value {
	t.Helper()
	v, err := ParseValue(s)
	if err != nil {
		t.Fatalf("ParseValue(%q): %v", s, err)
	}
	return v
}

// ansVars returns a variable table that holds Ans, or nil when ans is empty.
func ansVars(t *testing.T, ans string) map[string]Value {
	t.Helper()
	if ans == "" {
		return nil
	}
	return map[string]Value{"Ans": mustValue(t, ans)}
}

// testName shortens very long inputs for subtest names.
func testName(in string) string {
	if len(in) > 40 {
		return in[:20] + "..." + in[len(in)-10:]
	}
	return in
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		in   string
		ans  string
		want string
	}{
		{"2 * (3 + 4) ^ 2", "", "98"},
		{"0.1 + 0.2", "", "0.3"},
		{"1.50 + 1.50", "", "3"},
		{"10 - 4 - 3", "", "3"},
		{"-2^2", "", "-4"},
		{"2^-1", "", "0.5"},
		{"2^3^2", "", "512"},
		{"(2^3)^2", "", "64"},
		{"0^0", "", "1"},
		{"1/3", "", "0.3333333333333333333333333333333333"},
		{"2/3", "", "0.6666666666666666666666666666666667"},
		{"1/3*3", "", "0.9999999999999999999999999999999999"},
		{"sqrt(16)", "", "4"},
		{"sqrt(2)", "", "1.414213562373095048801688724209698"},
		{"2*sqrt(9)+1", "", "7"},
		{"sqrt(sqrt(16))", "", "2"},
		{"-0", "", "0"},
		{"0 * -1", "", "0"},
		{"--2", "", "2"},
		// The percent table from PLAN.md.
		{"200 + 10%", "", "220"},
		{"200 - 10%", "", "180"},
		{"200 * 10%", "", "20"},
		{"200 / 10%", "", "2000"},
		{"50%", "", "0.5"},
		{"100 + 10%", "", "110"},
		{"100 + (10%)", "", "100.1"},
		{"100 + -10%", "", "90"},
		{"100 - -10%", "", "110"},
		{"100 + (-10%)", "", "99.9"},
		{"100 + (5+5)%", "", "110"},
		{"(100 + 10)%", "", "1.1"},
		{"200 + 10% * 2", "", "200.2"},
		{"200 + 10% + 10%", "", "242"},
		// More percent cases.
		{"100 + --10%", "", "110"},
		{"100 + -(10%)", "", "99.9"},
		{"100 + 10%%", "", "100.1"},
		{"100 + 10%^2", "", "100.01"},
		{"10%%", "", "0.001"},
		{"-200 + 10%", "", "-220"},
		{"(200 + 10%) * 2", "", "440"},
		{"sqrt(100 + 21%)", "", "11"},
		// The final value is rounded even when no operation was applied.
		{"1234567890123456789012345678901234567890", "", "1.234567890123456789012345678901235E+39"},
		{"-(1234567890123456789012345678901234567890)", "", "-1.234567890123456789012345678901235E+39"},
		{"0.1234567890123456789012345678901234567", "", "0.1234567890123456789012345678901235"},
		// Long inputs are not rounded before an operation, and negation is exact.
		{"12345678901234567890123456789012345 - 12345678901234567890123456789012344", "", "1"},
		{"-12345678901234567890123456789012345 + 12345678901234567890123456789012344", "", "-1"},
		// Ans.
		{"Ans*2", "21", "42"},
		{"Ans + 10%", "200", "220"},
		{"-Ans", "1.5E+40", "-1.5E+40"},
		{"Ans^2", "1E+20", "1E+40"},
		{"Ans", "0.1234567890123456789012345678901234567", "0.1234567890123456789012345678901235"},
		{"Ans - Ans", "0.1234567890123456789012345678901234567", "0"},
	}
	for _, tt := range tests {
		t.Run(testName(tt.in), func(t *testing.T) {
			got, err := Evaluate(tt.in, ansVars(t, tt.ans))
			if err != nil {
				t.Fatalf("Evaluate(%q) with Ans=%q: %v", tt.in, tt.ans, err)
			}
			if got.String() != tt.want {
				t.Errorf("Evaluate(%q) with Ans=%q = %s, want %s", tt.in, tt.ans, got, tt.want)
			}
		})
	}
}

func TestEvaluateErrors(t *testing.T) {
	const (
		// 34 nines: the largest value in range.
		largest = "9.999999999999999999999999999999999E+6144"
		// 40 nines: in range, but rounding to 34 digits overflows.
		tooLong = "9.999999999999999999999999999999999999999E+6144"
	)
	tests := []struct {
		in   string
		ans  string
		code string
		pos  int
		msg  string
	}{
		{"foo(1)", "", "unknown_function", 0, "unknown function 'foo'"},
		{"2 + foo(1)", "", "unknown_function", 4, "unknown function 'foo'"},
		{"Sqrt(4)", "", "unknown_function", 0, "unknown function 'Sqrt'"},
		{"x", "", "unknown_variable", 0, "unknown variable 'x'"},
		{"Ans", "", "unknown_variable", 0, "unknown variable 'Ans'"},
		{"ans", "1", "unknown_variable", 0, "unknown variable 'ans'"},
		{"sqrt(1, 2)", "", "wrong_argument_count", 0, "wrong number of arguments for sqrt: want 1, got 2"},
		{"sqrt()", "", "wrong_argument_count", 0, "wrong number of arguments for sqrt: want 1, got 0"},
		{"1 + sqrt(sqrt(1, 2))", "", "wrong_argument_count", 9, "wrong number of arguments for sqrt: want 1, got 2"},
		{"1/0", "", "division_by_zero", 1, "division by zero"},
		{"1/(2-2)", "", "division_by_zero", 1, "division by zero"},
		{"0/0", "", "division_by_zero", 1, "division by zero"},
		{"0^-1", "", "division_by_zero", 1, "division by zero"},
		{"2 * 3/0", "", "division_by_zero", 5, "division by zero"},
		{"sqrt(-1)", "", "domain_error", 0, "result is not a real number"},
		{"1 + sqrt(-1)", "", "domain_error", 4, "result is not a real number"},
		{"(-8)^(1/3)", "", "domain_error", 4, "result is not a real number"},
		{"10^1000000", "", "overflow", 2, "result is too large"},
		{"0.1^1000000", "", "underflow", 3, "result is too close to zero"},
		// Names are checked before anything is computed, in input order.
		{"x + y", "", "unknown_variable", 0, "unknown variable 'x'"},
		{"1/0 + x", "", "unknown_variable", 6, "unknown variable 'x'"},
		{"1/0 + foo()", "", "unknown_function", 6, "unknown function 'foo'"},
		{"sqrt(1/0, 2)", "", "wrong_argument_count", 0, "wrong number of arguments for sqrt: want 1, got 2"},
		// Operands are evaluated left to right, and errors inside them keep
		// their own position.
		{"1/0 + 1/0", "", "division_by_zero", 1, "division by zero"},
		{"2^(1/0)", "", "division_by_zero", 4, "division by zero"},
		{"-(1/0)", "", "division_by_zero", 3, "division by zero"},
		{"(1/0)%", "", "division_by_zero", 2, "division by zero"},
		{"1 + (1/0)%", "", "division_by_zero", 6, "division by zero"},
		{"sqrt(1/0)", "", "division_by_zero", 6, "division by zero"},
		{"sqrt(x)", "", "unknown_variable", 5, "unknown variable 'x'"},
		// Percent errors point at the "%" when b percent of a fails, and at the
		// operator when a ± (b percent of a) fails.
		{"Ans + Ans%", "1E+6000", "overflow", 9, "result is too large"},
		{"Ans + 1%", largest, "overflow", 4, "result is too large"},
		{"Ans%", "1E-6143", "underflow", 3, "result is too close to zero"},
		// Rounding the final value can overflow. The error points at the root
		// of the expression.
		{"Ans", tooLong, "overflow", 0, "result is too large"},
		{" -(Ans)", tooLong, "overflow", 1, "result is too large"},
		{strings.Repeat("9", 6145), "", "overflow", 0, "result is too large"},
		// Literals outside the exponent range.
		{"1" + strings.Repeat("0", 6145), "", "overflow", 0, "result is too large"},
		{"2 + 0." + strings.Repeat("0", 6150) + "1", "", "underflow", 4, "result is too close to zero"},
		// Parser errors pass through.
		{"2 +", "", "syntax_error", 3, "unexpected end of expression"},
		{strings.Repeat("(", parser.MaxDepth+1), "", "nesting_too_deep", parser.MaxDepth, "parentheses are nested more than 100 levels deep"},
	}
	for _, tt := range tests {
		t.Run(testName(tt.in), func(t *testing.T) {
			got, err := Evaluate(tt.in, ansVars(t, tt.ans))
			var posErr positionalError
			if !errors.As(err, &posErr) {
				t.Fatalf("Evaluate(%.40q) = %v, %v; want a positional error", tt.in, got, err)
			}
			if posErr.Code() != tt.code || posErr.Pos() != tt.pos || posErr.Error() != tt.msg {
				t.Errorf("Evaluate(%.40q) error = %s %q at %d, want %s %q at %d",
					tt.in, posErr.Code(), posErr.Error(), posErr.Pos(), tt.code, tt.msg, tt.pos)
			}
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	_, err := Evaluate("1/0", nil)
	var mathErr *arith.MathError
	if !errors.As(err, &mathErr) || mathErr.Kind != arith.DivisionByZero {
		t.Errorf("Evaluate(1/0) error = %v, want it to wrap a division_by_zero MathError", err)
	}
	_, err = Evaluate("x", nil)
	if errors.As(err, &mathErr) {
		t.Errorf("Evaluate(x) error = %v wraps a MathError, want none", err)
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"98", "98"},
		{"-0.5", "-0.5"},
		{"1.50", "1.5"},
		{"1.5E+40", "1.5E+40"},
		// Every digit is kept.
		{"0.12345678901234567890123456789012345", "0.12345678901234567890123456789012345"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := mustValue(t, tt.in).String(); got != tt.want {
				t.Errorf("ParseValue(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
	for _, in := range []string{"", " 1", "abc", "1/2", "NaN", "Infinity", "1E+7000"} {
		t.Run(in, func(t *testing.T) {
			if v, err := ParseValue(in); err == nil {
				t.Errorf("ParseValue(%q) = %s, want an error", in, v)
			}
		})
	}
}

func TestResultRoundTrip(t *testing.T) {
	for _, in := range []string{"1/3", "-1.5", "2^200", "1/2^200", "10^20/7"} {
		t.Run(in, func(t *testing.T) {
			v, err := Evaluate(in, nil)
			if err != nil {
				t.Fatalf("Evaluate(%q): %v", in, err)
			}
			if got := mustValue(t, v.String()).String(); got != v.String() {
				t.Errorf("ParseValue(%s).String() = %s", v, got)
			}
		})
	}
}

func TestZeroValue(t *testing.T) {
	if got := (Value{}).String(); got != "0" {
		t.Errorf("Value{}.String() = %s, want 0", got)
	}
	got, err := Evaluate("Ans + 1", map[string]Value{"Ans": {}})
	if err != nil || got.String() != "1" {
		t.Errorf("Evaluate(Ans + 1) with a zero Ans = %v, %v; want 1", got, err)
	}
}

type fakeNode struct{}

func (fakeNode) Position() int  { return 0 }
func (fakeNode) String() string { return "fake" }

// Trees that the parser never produces fail with an error that has no code,
// which the API layer reports as an internal error.
func TestHandBuiltTrees(t *testing.T) {
	e := &evaluator{}
	if _, err := e.eval(fakeNode{}); err == nil || !strings.Contains(err.Error(), "unexpected node") {
		t.Errorf("eval(fakeNode) error = %v, want unexpected node", err)
	}
	if _, err := e.eval(&parser.Number{Text: "abc"}); !errors.Is(err, arith.ErrInvalidNumber) {
		t.Errorf("eval(Number abc) error = %v, want ErrInvalidNumber", err)
	}
}
