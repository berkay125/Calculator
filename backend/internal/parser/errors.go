package parser

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
