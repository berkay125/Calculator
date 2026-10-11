package arith

import (
	"errors"
	"testing"

	"github.com/cockroachdb/apd/v3"
)

type binaryOp func(x, y *apd.Decimal) (*apd.Decimal, error)

func mustParse(t *testing.T, s string) *apd.Decimal {
	t.Helper()
	d, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return d
}

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"12.5", "12.5"},
		{"-3", "-3"},
		{".5", "0.5"},
		{"1.5E+40", "1.5E+40"},
		{"1.5e-10", "1.5E-10"},
		// Input is never rounded, even beyond Precision digits.
		{"0.12345678901234567890123456789012345", "0.12345678901234567890123456789012345"},
		{"123456789012345678901234567890123456789", "1.23456789012345678901234567890123456789E+38"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := Format(mustParse(t, tt.in)); got != tt.want {
				t.Errorf("Format(Parse(%q)) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	for _, in := range []string{"", " 1", "abc", "1.2.3", "1e", "NaN", "sNaN", "Infinity", "-inf", "1E+7000", "1E-7000"} {
		t.Run(in, func(t *testing.T) {
			d, err := Parse(in)
			if !errors.Is(err, ErrInvalidNumber) {
				t.Errorf("Parse(%q) = %v, %v; want ErrInvalidNumber", in, d, err)
			}
		})
	}
}

func TestOperations(t *testing.T) {
	tests := []struct {
		name string
		op   binaryOp
		x, y string
		want string
	}{
		{"add is exact for decimals", Add, "0.1", "0.2", "0.3"},
		{"add strips trailing zeros", Add, "1.50", "1.50", "3"},
		{"add rounds beyond precision", Add, "1E+34", "1", "1E+34"},
		{"add rounds ties to even (down)", Add, "1E+34", "5", "1E+34"},
		{"add rounds ties to even (up)", Add, "1E+34", "15", "1.000000000000000000000000000000002E+34"},
		{"result rounding applies to long inputs", Add, "0.12345678901234567890123456789012345", "0", "0.1234567890123456789012345678901234"},
		{"long inputs are not rounded before the operation", Sub, "12345678901234567890123456789012345", "12345678901234567890123456789012344", "1"},
		{"sub to negative", Sub, "2", "5", "-3"},
		{"sub to zero", Sub, "0.3", "0.3", "0"},
		{"mul", Mul, "1.5", "-4", "-6"},
		{"mul by zero gives positive zero", Mul, "-1", "0", "0"},
		{"mul keeps 34-digit integers exact", Mul, "99999999999999999", "99999999999999999", "9999999999999999800000000000000001"},
		{"div exact", Div, "1", "4", "0.25"},
		{"div rounds to precision", Div, "1", "3", "0.3333333333333333333333333333333333"},
		{"div rounds the last digit", Div, "2", "3", "0.6666666666666666666666666666666667"},
		{"div negative", Div, "-7", "2", "-3.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.op(mustParse(t, tt.x), mustParse(t, tt.y))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s := Format(got); s != tt.want {
				t.Errorf("got %q, want %q", s, tt.want)
			}
		})
	}
}

func TestOperationErrors(t *testing.T) {
	tests := []struct {
		name string
		op   binaryOp
		x, y string
		want Kind
	}{
		{"divide by zero", Div, "1", "0", DivisionByZero},
		{"divide negative by zero", Div, "-1", "0", DivisionByZero},
		{"zero divided by zero", Div, "0", "0", DivisionByZero},
		{"overflow", Mul, "9E+6144", "10", Overflow},
		{"overflow on add", Add, "9.999999999999999999999999999999999E+6144", "1E+6131", Overflow},
		{"underflow", Div, "1E-6143", "10", Underflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.op(mustParse(t, tt.x), mustParse(t, tt.y))
			var mathErr *MathError
			if !errors.As(err, &mathErr) {
				t.Fatalf("got %v, %v; want MathError with kind %s", got, err, tt.want.Code())
			}
			if mathErr.Kind != tt.want {
				t.Errorf("got kind %s, want %s", mathErr.Code(), tt.want.Code())
			}
		})
	}
}
