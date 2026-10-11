package eval

import (
	"github.com/cockroachdb/apd/v3"

	"calculator/internal/arith"
)

// Value is an exact decimal number: the result of Evaluate or the value of a
// variable such as Ans. The zero Value is 0.
type Value struct {
	d *apd.Decimal
}

// ParseValue parses a decimal string, such as an earlier result, and keeps
// every digit. Plain and scientific notation are both accepted ("0.5",
// "1.5E+40"), so every string that String returns parses back to the same
// value.
func ParseValue(s string) (Value, error) {
	d, err := arith.Parse(s)
	if err != nil {
		return Value{}, err
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
