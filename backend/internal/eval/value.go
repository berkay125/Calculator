package eval

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"

	"calculator/internal/arith"
)

// Value is an exact decimal number: the result of Evaluate or the value of a
// variable such as Ans. The zero Value is 0.
type Value struct {
	d *apd.Decimal
}

// ErrTooManyDigits is returned by ParseValue for a number with more than
// arith.Precision digits.
var ErrTooManyDigits = fmt.Errorf("number has more than %d digits", arith.Precision)

// ParseValue parses an earlier result, such as the value of Ans, and keeps
// every digit. Plain and scientific notation are both accepted ("0.5",
// "1.5E+40"), so every string that String returns parses back to the same
// value.
//
// No result has more than arith.Precision digits, so a longer number returns
// ErrTooManyDigits instead of making every operation on it work with all its
// digits. Digits are counted on the parsed number: leading zeros do not count,
// trailing zeros do ("1.50" has three).
func ParseValue(s string) (Value, error) {
	d, err := arith.Parse(s)
	if err != nil {
		return Value{}, err
	}
	if d.NumDigits() > arith.Precision {
		return Value{}, ErrTooManyDigits
	}
	return Value{d}, nil
}

// String returns v in canonical form, as arith.Format writes it: "98",
// "0.5", "1.5E+40".
func (v Value) String() string {
	return arith.Format(v.decimal())
}

func (v Value) decimal() *apd.Decimal {
	if v.d == nil {
		return new(apd.Decimal)
	}
	return v.d
}
