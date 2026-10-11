package arith

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

// Precision is the number of significant digits every operation rounds its
// result to. It matches IEEE 754 decimal128.
const Precision = 34

// The exponent range also matches decimal128. Results outside it are reported
// as overflow or underflow instead of growing without bound.
const (
	maxExponent = 6144
	minExponent = -6143
)

// ctx is shared by all operations. An apd context is safe for concurrent use
// as long as it is not modified. Ties round to even, the IEEE 754 default.
var ctx = &apd.Context{
	Precision:   Precision,
	MaxExponent: maxExponent,
	MinExponent: minExponent,
	Traps:       apd.DefaultTraps,
	Rounding:    apd.RoundHalfEven,
}

// parseCtx keeps every digit of its input (precision 0 disables rounding) but
// still enforces the exponent range. Rounding inputs before an operation would
// make results depend on digits that were already thrown away.
//
// It sets no traps: when a trapped parse fails, apd drops the condition flags,
// and Parse needs them to tell overflow from underflow.
var parseCtx = &apd.Context{
	MaxExponent: maxExponent,
	MinExponent: minExponent,
	Rounding:    apd.RoundHalfEven,
}

// ErrInvalidNumber is returned by Parse for strings that are not finite
// decimal numbers.
var ErrInvalidNumber = errors.New("invalid number")

// Parse converts a decimal string such as "12.5", "-3" or "1.5E+40" into a
// Decimal. All digits are kept; only operation results are rounded. A
// malformed string returns ErrInvalidNumber, and a number outside the exponent
// range returns a MathError of kind Overflow or Underflow.
func Parse(s string) (*apd.Decimal, error) {
	d, cond, err := parseCtx.NewFromString(s)
	if err == nil && cond&(apd.Overflow|apd.Underflow|apd.Subnormal) != 0 {
		return nil, fromCondition(cond, nil)
	}
	if err != nil || d.Form != apd.Finite {
		return nil, fmt.Errorf("%w: %q", ErrInvalidNumber, s)
	}
	return d, nil
}

// Neg returns -x. Negation is exact, so unlike the other operations it does
// not round, and it never returns negative zero.
func Neg(x *apd.Decimal) *apd.Decimal {
	return new(apd.Decimal).Neg(x)
}

// Round rounds x to Precision significant digits, ties to even. Operation
// results are already rounded; Round is for values that skipped every
// operation, such as a long literal. Rounding up can overflow.
func Round(x *apd.Decimal) (*apd.Decimal, error) {
	d := new(apd.Decimal)
	if cond, err := ctx.Round(d, x); err != nil {
		return nil, fromCondition(cond, err)
	}
	return d, nil
}

// Add returns x + y.
func Add(x, y *apd.Decimal) (*apd.Decimal, error) {
	return apply(ctx.Add, x, y)
}

// Sub returns x - y.
func Sub(x, y *apd.Decimal) (*apd.Decimal, error) {
	return apply(ctx.Sub, x, y)
}

// Mul returns x * y.
func Mul(x, y *apd.Decimal) (*apd.Decimal, error) {
	return apply(ctx.Mul, x, y)
}

// Div returns x / y. Any division by zero, including 0/0, is reported as
// DivisionByZero.
func Div(x, y *apd.Decimal) (*apd.Decimal, error) {
	return apply(ctx.Quo, x, y)
}

// Pow returns x raised to the power y. 0^0 is 1 and zero raised to a negative
// power is DivisionByZero. A negative x with a non-integer y is a DomainError.
// Non-integer powers are computed through ln/exp and are approximate.
func Pow(x, y *apd.Decimal) (*apd.Decimal, error) {
	if x.IsZero() {
		switch y.Sign() {
		case 0:
			// apd follows the GDA spec, where 0^0 is undefined.
			return apd.New(1, 0), nil
		case -1:
			// apd returns Infinity here without raising a condition.
			return nil, &MathError{Kind: DivisionByZero}
		}
	}
	d, err := apply(ctx.Pow, x, y)
	var mathErr *MathError
	if errors.As(err, &mathErr) && (mathErr.Kind == Overflow || mathErr.Kind == Underflow) {
		mathErr.Kind = powRangeKind(x, y)
	}
	return d, err
}

var one = apd.New(1, 0)

// powRangeKind tells whether an out-of-range x^y is too large or too small.
// apd cannot be trusted here: for a negative exponent it computes the positive
// power first and only turns overflow into underflow, never the reverse, so
// 0.5^-1E+100 comes back as underflow. |x|^y grows without bound when |x| > 1
// and y > 0, or when |x| < 1 and y < 0.
func powRangeKind(x, y *apd.Decimal) Kind {
	var abs apd.Decimal
	abs.Abs(x)
	if (abs.Cmp(one) > 0) == (y.Sign() > 0) {
		return Overflow
	}
	return Underflow
}

// Sqrt returns the square root of x. A negative x is a DomainError.
func Sqrt(x *apd.Decimal) (*apd.Decimal, error) {
	d := new(apd.Decimal)
	if cond, err := ctx.Sqrt(d, x); err != nil {
		return nil, fromCondition(cond, err)
	}
	return d, nil
}

var hundred = apd.New(100, 0)

// Percent returns x / 100, the value of "x%" on its own or after * and /.
func Percent(x *apd.Decimal) (*apd.Decimal, error) {
	return Div(x, hundred)
}

// PercentOf returns pct percent of base (base * pct / 100). It gives "a + b%"
// its phone-calculator meaning: a + PercentOf(a, b).
func PercentOf(base, pct *apd.Decimal) (*apd.Decimal, error) {
	p, err := Mul(base, pct)
	if err != nil {
		return nil, err
	}
	return Div(p, hundred)
}

// apply runs a binary apd operation into a new Decimal and translates a
// trapped condition into a MathError.
func apply(op func(d, x, y *apd.Decimal) (apd.Condition, error), x, y *apd.Decimal) (*apd.Decimal, error) {
	d := new(apd.Decimal)
	if cond, err := op(d, x, y); err != nil {
		return nil, fromCondition(cond, err)
	}
	return d, nil
}
