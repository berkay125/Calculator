package arith

import "github.com/cockroachdb/apd/v3"

// Results whose adjusted exponent (the exponent in scientific notation) lies
// in this range are written in plain notation, others in scientific notation.
// The upper bound lets every integer of up to Precision digits print in full.
const (
	plainMinExponent = -7
	plainMaxExponent = Precision - 1
)

// Format returns the canonical string form of d: trailing zeros removed,
// negative zero written as "0", plain notation for moderate magnitudes and
// scientific notation such as "1.5E+40" otherwise. Parse accepts every string
// Format produces.
func Format(d *apd.Decimal) string {
	var r apd.Decimal
	r.Reduce(d) // also turns -0 into 0
	adj := int64(r.Exponent) + r.NumDigits() - 1
	if adj >= plainMinExponent && adj <= plainMaxExponent {
		return r.Text('f')
	}
	return r.Text('E')
}
