package eval

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"calculator/internal/arith"
	"calculator/internal/parser"
)

// evalCodes are the error codes Evaluate can return. The API layer maps each
// one to an HTTP status, so any other code would surface as an internal error.
var evalCodes = map[string]bool{
	"syntax_error":         true,
	"nesting_too_deep":     true,
	"unknown_function":     true,
	"unknown_variable":     true,
	"wrong_argument_count": true,
	"division_by_zero":     true,
	"domain_error":         true,
	"overflow":             true,
	"underflow":            true,
}

// FuzzEvaluate checks invariants that must hold for every expression and Ans:
//   - Evaluate does not panic.
//   - An error has a code from the error catalog and a position in
//     [0, len(expression)].
//   - A result has at most arith.Precision significant digits, and its string
//     parses back to the same value, so it can be sent back as Ans.
//
// An ans that ParseValue rejects leaves Ans undefined. The seeds run as part
// of go test. To search for new failing inputs:
//
//	go test ./internal/eval -fuzz=FuzzEvaluate -fuzztime=1m
func FuzzEvaluate(f *testing.F) {
	for _, seed := range []struct{ expression, ans string }{
		{"2 * (3 + 4) ^ 2", ""},
		{"0.1 + 0.2", ""},
		{"200 + 10% + 10%", ""},
		{"100 + (-10%)", ""},
		{"100 - --10%", ""},
		{"(100 + 10)%", ""},
		{"1/3*3", ""},
		{"sqrt(2) ^ 2", ""},
		{"2 ^ 0.5 ^ -1", ""},
		{"Ans*2+1", "0"},
		{"Ans / 7", "1.5E+40"},
		{"-Ans", "-0.1234567890123456789012345678901234"},
		{"Ans * 10", "9.999999999999999999999999999999999E+6144"},
		{"Ans / 10", "1E-6143"},
		{"Ans", "1E+7000"},
		{"Ans", "0.12345678901234567890123456789012345"},
		// apd fails without an error code on a fractional power of a base
		// with thousands of digits, unless arith splits the power.
		{"1." + strings.Repeat("7", 3000) + " ^ 1.5", ""},
		{"1/0", ""},
		{"0^-1", ""},
		{"sqrt(-1)", ""},
		{"(-8)^(1/3)", ""},
		{"10^1000000", ""},
		{"0.5^-1000000", ""},
		{"1 + " + strings.Repeat("9", 40), ""},
		{"foo(1)", ""},
		{"x + 1/0", ""},
		{"sqrt(1, 2)", ""},
		{"2 +", ""},
		{strings.Repeat("(", parser.MaxDepth+1), ""},
	} {
		f.Add(seed.expression, seed.ans)
	}
	f.Fuzz(func(t *testing.T, expression, ans string) {
		var vars map[string]Value
		if v, err := ParseValue(ans); err == nil {
			vars = map[string]Value{"Ans": v}
		}
		// Long inputs are shortened in messages; the failing input is saved in
		// testdata/fuzz anyway.
		call := fmt.Sprintf("Evaluate(%.60q, Ans=%.40q)", expression, ans)
		got, err := Evaluate(expression, vars)
		if err != nil {
			var posErr positionalError
			if !errors.As(err, &posErr) {
				t.Fatalf("%s error = %v, want an error with a code and a position", call, err)
			}
			if !evalCodes[posErr.Code()] {
				t.Errorf("%s error code = %q", call, posErr.Code())
			}
			if pos := posErr.Pos(); pos < 0 || pos > len(expression) {
				t.Errorf("%s error %q at %d, want a position in [0, %d]", call, err, pos, len(expression))
			}
			return
		}
		if digits := got.decimal().NumDigits(); digits > arith.Precision {
			t.Errorf("%s = %s has %d significant digits", call, got, digits)
		}
		s := got.String()
		back, err := ParseValue(s)
		if err != nil {
			t.Fatalf("%s = %s, but ParseValue(%s): %v", call, s, s, err)
		}
		if back.String() != s {
			t.Errorf("%s = %s, but ParseValue(%s) = %s", call, s, s, back)
		}
	})
}
