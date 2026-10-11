package arith

import (
	"errors"
	"strings"
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
		// The edges of the exponent range.
		{"9.999999999999999999999999999999999E+6144", "9.999999999999999999999999999999999E+6144"},
		{"1E-6143", "1E-6143"},
		{"0E-7000", "0"},
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
	for _, in := range []string{"", " 1", "abc", "1.2.3", "1e", "NaN", "sNaN", "Infinity", "-inf"} {
		t.Run(in, func(t *testing.T) {
			d, err := Parse(in)
			if !errors.Is(err, ErrInvalidNumber) {
				t.Errorf("Parse(%q) = %v, %v; want ErrInvalidNumber", in, d, err)
			}
		})
	}
}

func TestParseOutOfRange(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Kind
	}{
		{"large exponent", "1E+7000", Overflow},
		{"negative with large exponent", "-1E+7000", Overflow},
		{"just above the range", "1E+6145", Overflow},
		{"long integer", "1" + strings.Repeat("0", 6145), Overflow},
		{"small exponent", "1E-7000", Underflow},
		{"just below the range", "1E-6144", Underflow},
		{"long fraction", "0." + strings.Repeat("0", 6150) + "1", Underflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Parse(tt.in)
			var mathErr *MathError
			if !errors.As(err, &mathErr) || mathErr.Kind != tt.want {
				t.Errorf("Parse = %v, %v; want MathError with kind %s", d, err, tt.want.Code())
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
		{"pow integer exponent", Pow, "2", "10", "1024"},
		{"pow negative exponent", Pow, "2", "-1", "0.5"},
		{"pow zero exponent", Pow, "5", "0", "1"},
		{"pow zero to the zero", Pow, "0", "0", "1"},
		{"pow zero base", Pow, "0", "2", "0"},
		{"pow negative base, odd exponent", Pow, "-2", "3", "-8"},
		{"pow negative base, odd exponent with fraction digits", Pow, "-2", "3.0", "-8"},
		{"pow negative base, even exponent", Pow, "-2", "2", "4"},
		{"pow fractional exponent, exact result", Pow, "4", "0.5", "2"},
		{"pow fractional exponent above one", Pow, "9", "1.5", "27"},
		{"pow fractional exponent, irrational result", Pow, "2", "0.5", "1.414213562373095048801688724209698"},
		{"pow huge exponent of one", Pow, "1", "1E+100", "1"},
		{"pow huge even exponent of minus one", Pow, "-1", "1E+100", "1"},
		{"percent of", PercentOf, "200", "10", "20"},
		{"percent of with fraction", PercentOf, "50", "12.5", "6.25"},
		{"negative percent of", PercentOf, "200", "-10", "-20"},
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
		{"pow zero to a negative power", Pow, "0", "-1", DivisionByZero},
		{"pow negative base, fractional exponent", Pow, "-8", "0.5", DomainError},
		{"pow negative base, rounded one third", Pow, "-8", "0.3333333333333333333333333333333333", DomainError},
		{"pow overflow past the exponent range", Pow, "10", "7000", Overflow},
		{"pow overflow in intermediate steps", Pow, "10", "1000000", Overflow},
		{"pow overflow with a huge exponent", Pow, "2", "1E+100", Overflow},
		{"pow underflow with a negative exponent", Pow, "10", "-1000000", Underflow},
		{"pow underflow with a small base", Pow, "0.5", "1E+100", Underflow},
		{"pow underflow with a large base and a huge negative exponent", Pow, "2", "-1E+100", Underflow},
		{"pow overflow with a small base and a huge negative exponent", Pow, "0.5", "-1E+100", Overflow},
		{"pow overflow with a small base and a negative exponent", Pow, "0.1", "-1000000", Overflow},
		{"pow overflow with a negative small base", Pow, "-0.5", "-1E+100", Overflow},
		{"pow overflow past the range with a negative exponent", Pow, "0.1", "-7000", Overflow},
		{"pow underflow past the range with a negative exponent", Pow, "10", "-7000", Underflow},
		{"percent of overflow", PercentOf, "1E+6144", "1E+6144", Overflow},
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

func TestUnaryOperations(t *testing.T) {
	tests := []struct {
		name string
		op   func(x *apd.Decimal) (*apd.Decimal, error)
		x    string
		want string
	}{
		{"sqrt exact", Sqrt, "16", "4"},
		{"sqrt fraction", Sqrt, "0.25", "0.5"},
		{"sqrt irrational", Sqrt, "2", "1.414213562373095048801688724209698"},
		{"sqrt zero", Sqrt, "0", "0"},
		{"sqrt negative zero", Sqrt, "-0", "0"},
		{"percent", Percent, "10", "0.1"},
		{"percent with fraction", Percent, "12.5", "0.125"},
		{"negative percent", Percent, "-5", "-0.05"},
		{"round keeps short values", Round, "12.5", "12.5"},
		{"round ties to even (down)", Round, "0.12345678901234567890123456789012345", "0.1234567890123456789012345678901234"},
		{"round ties to even (up)", Round, "0.12345678901234567890123456789012355", "0.1234567890123456789012345678901236"},
		{"round long integer", Round, "1234567890123456789012345678901234567890", "1.234567890123456789012345678901235E+39"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.op(mustParse(t, tt.x))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s := Format(got); s != tt.want {
				t.Errorf("got %q, want %q", s, tt.want)
			}
		})
	}
}

func TestUnaryOperationErrors(t *testing.T) {
	tests := []struct {
		name string
		op   func(x *apd.Decimal) (*apd.Decimal, error)
		x    string
		want Kind
	}{
		{"sqrt of negative", Sqrt, "-1", DomainError},
		{"round up past the exponent range", Round, "9.999999999999999999999999999999999999999E+6144", Overflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.op(mustParse(t, tt.x))
			var mathErr *MathError
			if !errors.As(err, &mathErr) || mathErr.Kind != tt.want {
				t.Errorf("got %v, %v; want MathError with kind %s", got, err, tt.want.Code())
			}
		})
	}
}

func TestNeg(t *testing.T) {
	tests := []struct {
		x, want string
	}{
		{"5", "-5"},
		{"-2.5", "2.5"},
		// Negation is exact: it does not round to Precision digits.
		{"123456789012345678901234567890123456789", "-1.23456789012345678901234567890123456789E+38"},
	}
	for _, tt := range tests {
		t.Run(tt.x, func(t *testing.T) {
			if got := Format(Neg(mustParse(t, tt.x))); got != tt.want {
				t.Errorf("Neg(%s) = %s, want %s", tt.x, got, tt.want)
			}
		})
	}
	for _, x := range []string{"0", "-0"} {
		if got := Neg(mustParse(t, x)); got.Negative {
			t.Errorf("Neg(%s) = %s, want positive zero", x, got)
		}
	}
}
