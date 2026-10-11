package parser

import "fmt"

// SyntaxError reports input that is not a valid expression, together with the
// position of the offending character.
type SyntaxError struct {
	Msg    string
	Offset int // 0-based index of the offending character in the input
}

func (e *SyntaxError) Error() string {
	return e.Msg
}

// Code returns the error code "syntax_error".
func (e *SyntaxError) Code() string {
	return "syntax_error"
}

// Pos returns the 0-based index of the offending character in the input.
func (e *SyntaxError) Pos() int {
	return e.Offset
}

// NestingError reports parentheses nested deeper than MaxDepth.
type NestingError struct {
	Offset int // 0-based index of the first "(" beyond the limit
}

func (e *NestingError) Error() string {
	return fmt.Sprintf("parentheses are nested more than %d levels deep", MaxDepth)
}

// Code returns the error code "nesting_too_deep".
func (e *NestingError) Code() string {
	return "nesting_too_deep"
}

// Pos returns the 0-based index of the first "(" beyond the limit.
func (e *NestingError) Pos() int {
	return e.Offset
}
