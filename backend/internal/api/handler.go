package api

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"calculator/internal/eval"
)

// Input limits. The endpoint is public, so every input is bounded.
const (
	maxBodyBytes = 4 << 10
	// maxExpressionLength counts bytes. Only ASCII is valid in an expression,
	// so for valid input that is the number of characters.
	maxExpressionLength = 1000
)

type request struct {
	// Pointers tell a missing field from an empty one: a missing expression is
	// an invalid request, an empty one is a syntax error.
	Expression *string `json:"expression"`
	Ans        *string `json:"ans"`
}

type response struct {
	Result string `json:"result"`
}

// evaluateFunc is the signature of eval.Evaluate. Tests replace it to reach
// errors that eval never returns.
type evaluateFunc func(expression string, vars map[string]eval.Value) (eval.Value, error)

type handler struct {
	evaluate evaluateFunc
	log      *slog.Logger
}

// NewHandler returns the API's handler, which serves POST /api/v1/calculate.
// Other methods get 405. Unexpected errors are logged to log and answered
// with 500.
func NewHandler(log *slog.Logger) http.Handler {
	return newHandler(eval.Evaluate, log)
}

func newHandler(evaluate evaluateFunc, log *slog.Logger) http.Handler {
	h := &handler{evaluate: evaluate, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/calculate", h.calculate)
	return mux
}

func (h *handler) calculate(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(w, r)
	if err != nil {
		writeError(w, codeInvalidRequest, err.Error(), nil)
		return
	}
	expression := *req.Expression
	if len(expression) > maxExpressionLength {
		msg := fmt.Sprintf("expression is longer than %d characters", maxExpressionLength)
		writeError(w, codeExpressionTooLong, msg, nil)
		return
	}
	var vars map[string]eval.Value
	if req.Ans != nil {
		// Every ParseValue error is invalid input, including overflow, so its
		// code does not go through the status table.
		ans, err := eval.ParseValue(*req.Ans)
		if err != nil {
			writeError(w, codeInvalidAns, ansMessage(err), nil)
			return
		}
		vars = map[string]eval.Value{"Ans": ans}
	}
	result, err := h.evaluate(expression, vars)
	if err != nil {
		h.writeEvalError(w, expression, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Result: result.String()})
}

// decodeRequest reads the JSON body: exactly one object whose member names
// match exactly. An unknown, differently cased or repeated name is an error,
// because ignoring a misspelled "ans" would silently give a wrong result. A
// returned error's message is meant for the client.
func decodeRequest(w http.ResponseWriter, r *http.Request) (request, error) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return request{}, fmt.Errorf("request body is larger than %d bytes", maxBodyBytes)
	}
	if err != nil {
		return request{}, errors.New("request body could not be read")
	}
	if len(data) == 0 {
		return request{}, errors.New("request body is empty")
	}
	var req request
	if err := json.Unmarshal(data, &req, json.RejectUnknownMembers(true)); err != nil {
		return request{}, decodeError(err)
	}
	if req.Expression == nil {
		return request{}, errors.New("expression is missing")
	}
	return req, nil
}

// decodeError describes a JSON decoding error without Go type names.
func decodeError(err error) error {
	if serr, ok := errors.AsType[*jsontext.SyntacticError](err); ok && serr.Err == jsontext.ErrDuplicateName {
		return fmt.Errorf("duplicate field %q", serr.JSONPointer.LastToken())
	}
	serr, ok := errors.AsType[*json.SemanticError](err)
	switch {
	case !ok:
		return errors.New("request body is not valid JSON")
	case serr.Err == json.ErrUnknownName:
		return fmt.Errorf("unknown field %q", serr.JSONPointer.LastToken())
	case serr.JSONPointer == "":
		return errors.New("request body must be a JSON object")
	}
	return fmt.Errorf("%s must be a string", serr.JSONPointer.LastToken())
}

// ansMessage describes why ans was rejected.
func ansMessage(err error) string {
	var outOfRange coded // overflow or underflow
	switch {
	case errors.Is(err, eval.ErrTooManyDigits):
		return "invalid ans: " + err.Error()
	case errors.As(err, &outOfRange):
		return "invalid ans: outside the exponent range"
	}
	return "invalid ans: not a valid number"
}
