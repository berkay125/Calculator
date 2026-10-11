package arith

import (
	"errors"
	"testing"

	"github.com/cockroachdb/apd/v3"
)

func TestKindCodesAndMessages(t *testing.T) {
	tests := []struct {
		kind    Kind
		code    string
		message string
	}{
		{DivisionByZero, "division_by_zero", "division by zero"},
		{DomainError, "domain_error", "result is not a real number"},
		{Overflow, "overflow", "result is too large"},
		{Underflow, "underflow", "result is too close to zero"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := &MathError{Kind: tt.kind}
			if got := err.Code(); got != tt.code {
				t.Errorf("Code() = %q, want %q", got, tt.code)
			}
			if got := err.Error(); got != tt.message {
				t.Errorf("Error() = %q, want %q", got, tt.message)
			}
		})
	}
}

func TestFromCondition(t *testing.T) {
	tests := []struct {
		name string
		cond apd.Condition
		want Kind
	}{
		{"division by zero", apd.DivisionByZero, DivisionByZero},
		{"division undefined", apd.DivisionUndefined | apd.InvalidOperation, DivisionByZero},
		{"overflow", apd.Overflow | apd.Inexact | apd.Rounded, Overflow},
		{"system overflow", apd.SystemOverflow, Overflow},
		{"underflow", apd.Underflow | apd.Subnormal | apd.Inexact, Underflow},
		{"subnormal", apd.Subnormal, Underflow},
		{"system underflow", apd.SystemUnderflow, Underflow},
		{"invalid operation", apd.InvalidOperation, DomainError},
		{"division impossible", apd.DivisionImpossible, DomainError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fromCondition(tt.cond, errors.New("trapped"))
			var mathErr *MathError
			if !errors.As(err, &mathErr) || mathErr.Kind != tt.want {
				t.Errorf("fromCondition(%v) = %v, want MathError with kind %s", tt.cond, err, tt.want.Code())
			}
		})
	}
}

func TestFromConditionUnknown(t *testing.T) {
	cause := errors.New("exponent out of range")
	err := fromCondition(apd.Inexact, cause)
	var mathErr *MathError
	if errors.As(err, &mathErr) {
		t.Fatalf("got MathError %v, want a plain error", err)
	}
	if !errors.Is(err, cause) {
		t.Errorf("got %v, want it to wrap %v", err, cause)
	}
}
