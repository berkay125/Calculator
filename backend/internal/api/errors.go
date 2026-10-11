package api

import (
	"encoding/json/v2"
	"errors"
	"net/http"
)

// The codes of the errors the handler finds itself.
const (
	codeInvalidRequest    = "invalid_request"
	codeInvalidAns        = "invalid_ans"
	codeExpressionTooLong = "expression_too_long"
)

// statusByCode maps every error code to its HTTP status: 400 for invalid
// input, 422 for undefined math. A code missing here is answered with 500.
var statusByCode = map[string]int{
	codeInvalidRequest:     http.StatusBadRequest,
	codeInvalidAns:         http.StatusBadRequest,
	codeExpressionTooLong:  http.StatusBadRequest,
	"nesting_too_deep":     http.StatusBadRequest,
	"syntax_error":         http.StatusBadRequest,
	"unknown_function":     http.StatusBadRequest,
	"unknown_variable":     http.StatusBadRequest,
	"wrong_argument_count": http.StatusBadRequest,
	"division_by_zero":     http.StatusUnprocessableEntity,
	"domain_error":         http.StatusUnprocessableEntity,
	"overflow":             http.StatusUnprocessableEntity,
	"underflow":            http.StatusUnprocessableEntity,
}

// coded is an error with an error code. The handler knows errors only through
// this interface and positional, not through their concrete types.
type coded interface {
	error
	Code() string
}

// positional is an error that also knows where in the expression it is.
type positional interface {
	Pos() int
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Position is a pointer so that 0 is written and a missing position is
	// left out.
	Position *int `json:"position,omitempty"`
}

// writeError writes an error with a code from statusByCode.
func writeError(w http.ResponseWriter, code, message string, position *int) {
	body := errorBody{Code: code, Message: message, Position: position}
	writeJSON(w, statusByCode[code], errorResponse{body})
}

// writeEvalError answers an error from evaluating expression. An error without
// a known code is a bug: it is logged, and the client gets 500.
func (h *handler) writeEvalError(w http.ResponseWriter, expression string, err error) {
	var c coded
	if errors.As(err, &c) {
		if _, ok := statusByCode[c.Code()]; ok {
			var position *int
			if p, ok := c.(positional); ok {
				pos := p.Pos()
				position = &pos
			}
			writeError(w, c.Code(), c.Error(), position)
			return
		}
	}
	h.log.Error("evaluate failed", "expression", expression, "error", err)
	body := errorBody{Code: "internal_error", Message: "internal server error"}
	writeJSON(w, http.StatusInternalServerError, errorResponse{body})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// An error here means the client has gone away; there is no one to tell.
	_ = json.MarshalWrite(w, v)
}
