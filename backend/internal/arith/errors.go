package arith

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

// Kind classifies a MathError. Each kind has a stable error code that clients
// can rely on.
type Kind int

const (
	DivisionByZero Kind = iota + 1
	DomainError
	Overflow
	Underflow
)

var kinds = [...]struct{ code, message string }{
	DivisionByZero: {"division_by_zero", "division by zero"},
	DomainError:    {"domain_error", "result is not a real number"},
	Overflow:       {"overflow", "result is too large"},
	Underflow:      {"underflow", "result is too close to zero"},
}

// Code returns the machine-readable error code of k, such as
// "division_by_zero".
func (k Kind) Code() string {
	return kinds[k].code
}

// MathError reports a result that is mathematically undefined or cannot be
// represented.
type MathError struct {
	Kind Kind
}

func (e *MathError) Error() string {
	return kinds[e.Kind].message
}

// Code returns the error code of e's kind.
func (e *MathError) Code() string {
	return e.Kind.Code()
}

// fromCondition translates the flags of a trapped apd operation into a
// MathError. Errors without a known flag are wrapped and returned as is.
func fromCondition(cond apd.Condition, err error) error {
	switch {
	case cond&(apd.DivisionByZero|apd.DivisionUndefined) != 0:
		return &MathError{Kind: DivisionByZero}
	case cond&(apd.Overflow|apd.SystemOverflow) != 0:
		return &MathError{Kind: Overflow}
	case cond&(apd.Underflow|apd.Subnormal|apd.SystemUnderflow) != 0:
		return &MathError{Kind: Underflow}
	case cond&(apd.InvalidOperation|apd.DivisionImpossible) != 0:
		return &MathError{Kind: DomainError}
	}
	return fmt.Errorf("arith: %w", err)
}
