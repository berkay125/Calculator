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
var parseCtx = ctx.WithPrecision(0)

// ErrInvalidNumber is returned by Parse for strings that are not finite
// decimal numbers within the exponent range.
var ErrInvalidNumber = errors.New("invalid number")

// Parse converts a decimal string such as "12.5", "-3" or "1.5E+40" into a
// Decimal. All digits are kept; only operation results are rounded.
func Parse(s string) (*apd.Decimal, error) {
	d, _, err := parseCtx.NewFromString(s)
	if err != nil || d.Form != apd.Finite {
		return nil, fmt.Errorf("%w: %q", ErrInvalidNumber, s)
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

// apply runs a binary apd operation into a new Decimal and translates a
// trapped condition into a MathError.
func apply(op func(d, x, y *apd.Decimal) (apd.Condition, error), x, y *apd.Decimal) (*apd.Decimal, error) {
	d := new(apd.Decimal)
	if cond, err := op(d, x, y); err != nil {
		return nil, fromCondition(cond, err)
	}
	return d, nil
}
