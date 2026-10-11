package eval

import (
	"errors"
	"fmt"

	"calculator/internal/arith"
	"calculator/internal/parser"
)

// Error is a problem found while evaluating a parsed expression: an unknown
// name, a wrong number of arguments, or a math error such as division by zero.
type Error struct {
	code   string
	msg    string
	offset int
	err    error // the *arith.MathError behind a math error
}

func (e *Error) Error() string {
	return e.msg
}

// Code returns the error code, such as "unknown_function" or
// "division_by_zero".
func (e *Error) Code() string {
	return e.code
}

// Pos returns the 0-based index of the name, operator or function that caused
// the error.
func (e *Error) Pos() int {
	return e.offset
}

// Unwrap returns the *arith.MathError behind a math error, or nil.
func (e *Error) Unwrap() error {
	return e.err
}

func unknownVariable(n *parser.Variable) error {
	return &Error{code: "unknown_variable", msg: fmt.Sprintf("unknown variable '%s'", n.Name), offset: n.Pos}
}

func unknownFunction(n *parser.Call) error {
	return &Error{code: "unknown_function", msg: fmt.Sprintf("unknown function '%s'", n.Name), offset: n.Pos}
}

func wrongArgumentCount(n *parser.Call, want int) error {
	msg := fmt.Sprintf("wrong number of arguments for %s: want %d, got %d", n.Name, want, len(n.Args))
	return &Error{code: "wrong_argument_count", msg: msg, offset: n.Pos}
}

// at attaches pos to an error from arith. It returns nil when err is nil, so a
// result can be passed through as return d, at(pos, err).
func at(pos int, err error) error {
	var mathErr *arith.MathError
	if !errors.As(err, &mathErr) {
		return err // nil, or an unexpected error without a code
	}
	return &Error{code: mathErr.Code(), msg: mathErr.Error(), offset: pos, err: err}
}
