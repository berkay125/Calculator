package api

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"calculator/internal/eval"
)

// reply is a decoded response. Pointers tell a missing field from a zero one.
type reply struct {
	Result *string `json:"result"`
	Error  *struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Position *int   `json:"position"`
	} `json:"error"`
}

func do(t *testing.T, h http.Handler, method, body string) (*httptest.ResponseRecorder, reply) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, "/api/v1/calculate", strings.NewReader(body)))
	var r reply
	if rec.Code == http.StatusMethodNotAllowed {
		return rec, r
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	// The body must hold exactly the fields of the API contract.
	if err := json.Unmarshal(rec.Body.Bytes(), &r, json.RejectUnknownMembers(true)); err != nil {
		t.Fatalf("decoding response %q: %v", rec.Body.String(), err)
	}
	if (r.Result == nil) == (r.Error == nil) {
		t.Fatalf("response has a result and an error, or neither: %+v", r)
	}
	return rec, r
}

func post(t *testing.T, body string) (*httptest.ResponseRecorder, reply) {
	t.Helper()
	return do(t, NewHandler(slog.New(slog.DiscardHandler)), http.MethodPost, body)
}

// jsonBody returns a request body with the given expression and, when ans is
// not empty, ans.
func jsonBody(t *testing.T, expression, ans string) string {
	t.Helper()
	req := map[string]string{"expression": expression}
	if ans != "" {
		req["ans"] = ans
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// padded returns a body of exactly n bytes: a JSON object followed by spaces.
func padded(t *testing.T, n int) string {
	t.Helper()
	body := `{"expression": "1"}`
	return body + strings.Repeat(" ", n-len(body))
}

func testName(s string) string {
	if len(s) > 40 {
		return s[:30] + "..."
	}
	return s
}

func TestCalculate(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{`{"expression": "2 * (3 + 4) ^ 2"}`, "98"},
		{`{"expression": "1/3"}`, "0.3333333333333333333333333333333333"},
		{`{"expression": "2^200"}`, "1.606938044258990275541962092341163E+60"},
		{`{"expression": "Ans*2", "ans": "21"}`, "42"},
		{`{"expression": "Ans/2", "ans": "1.5E+40"}`, "7.5E+39"},
		// An ans of 34 digits is accepted.
		{`{"expression": "Ans", "ans": "0.1234567890123456789012345678901234"}`, "0.1234567890123456789012345678901234"},
		{`{"ans": "2", "expression": "Ans"}`, "2"},
		// A null ans is the same as none.
		{`{"expression": "1", "ans": null}`, "1"},
		// The limits are inclusive.
		{jsonBody(t, "11"+strings.Repeat("+1", 499), ""), "510"},
		{padded(t, maxBodyBytes), "1"},
	}
	for _, tt := range tests {
		t.Run(testName(tt.body), func(t *testing.T) {
			rec, r := post(t, tt.body)
			if rec.Code != http.StatusOK || r.Result == nil || *r.Result != tt.want {
				t.Errorf("POST %s = %d %s, want 200 %s", tt.body, rec.Code, rec.Body, tt.want)
			}
		})
	}
}

func TestCalculateErrors(t *testing.T) {
	const none = -1 // no position in the response
	tests := []struct {
		body   string
		status int
		code   string
		pos    int
		msg    string
	}{
		// Invalid requests.
		{``, 400, "invalid_request", none, "request body is empty"},
		{`{`, 400, "invalid_request", none, "request body is not valid JSON"},
		{`expression=1`, 400, "invalid_request", none, "request body is not valid JSON"},
		{`[]`, 400, "invalid_request", none, "request body must be a JSON object"},
		{`"1+1"`, 400, "invalid_request", none, "request body must be a JSON object"},
		{`{"expression": 5}`, 400, "invalid_request", none, "expression must be a string"},
		{`{"expression": "1", "ans": 5}`, 400, "invalid_request", none, "ans must be a string"},
		{`{}`, 400, "invalid_request", none, "expression is missing"},
		{`null`, 400, "invalid_request", none, "expression is missing"},
		{`{"expression": null}`, 400, "invalid_request", none, "expression is missing"},
		{`{"ans": "1"}`, 400, "invalid_request", none, "expression is missing"},
		// Names must match exactly: a misspelled ans must not be ignored.
		{`{"expression": "Ans*2", "answer": "21"}`, 400, "invalid_request", none, `unknown field "answer"`},
		{`{"expression": "Ans*2", "ANS": "21"}`, 400, "invalid_request", none, `unknown field "ANS"`},
		{`{"Expression": "1"}`, 400, "invalid_request", none, `unknown field "Expression"`},
		{`{"expression": "1", "ans": "2", "ans": "3"}`, 400, "invalid_request", none, `duplicate field "ans"`},
		{`{"expression": "1", "expression": "2"}`, 400, "invalid_request", none, `duplicate field "expression"`},
		{`{"expression": {"value": "1"}}`, 400, "invalid_request", none, "expression must be a string"},
		// Invalid UTF-8 is not replaced, it is rejected.
		{"{\"expression\": \"1\xff\"}", 400, "invalid_request", none, "request body is not valid JSON"},
		{`{"expression": "1"} {"expression": "2"}`, 400, "invalid_request", none, "request body is not valid JSON"},
		{`{"expression": "1"} x`, 400, "invalid_request", none, "request body is not valid JSON"},
		{padded(t, maxBodyBytes+1), 400, "invalid_request", none, "request body is larger than 4096 bytes"},
		// Expression length.
		{jsonBody(t, "1"+strings.Repeat("+1", 500), ""), 400, "expression_too_long", none, "expression is longer than 1000 characters"},
		// The expression is checked before ans.
		{jsonBody(t, "1"+strings.Repeat("+1", 500), "abc"), 400, "expression_too_long", none, "expression is longer than 1000 characters"},
		// Invalid ans. Out of range is invalid input, not an overflow.
		{`{"expression": "1", "ans": "abc"}`, 400, "invalid_ans", none, "invalid ans: not a valid number"},
		{`{"expression": "1", "ans": ""}`, 400, "invalid_ans", none, "invalid ans: not a valid number"},
		{`{"expression": "1", "ans": " 1"}`, 400, "invalid_ans", none, "invalid ans: not a valid number"},
		{`{"expression": "1", "ans": "NaN"}`, 400, "invalid_ans", none, "invalid ans: not a valid number"},
		{`{"expression": "1", "ans": "1E+7000"}`, 400, "invalid_ans", none, "invalid ans: outside the exponent range"},
		{`{"expression": "1", "ans": "1E-7000"}`, 400, "invalid_ans", none, "invalid ans: outside the exponent range"},
		{`{"expression": "1", "ans": "0.12345678901234567890123456789012345"}`, 400, "invalid_ans", none, "invalid ans: number has more than 34 digits"},
		// Errors from evaluation keep their position, including 0.
		{`{"expression": ""}`, 400, "syntax_error", 0, "empty expression"},
		{`{"expression": "2 +"}`, 400, "syntax_error", 3, "unexpected end of expression"},
		{`{"expression": "2 $ 3"}`, 400, "syntax_error", 2, "unexpected character '$'"},
		{jsonBody(t, strings.Repeat("(", 101), ""), 400, "nesting_too_deep", 100, "parentheses are nested more than 100 levels deep"},
		{`{"expression": "foo(1)"}`, 400, "unknown_function", 0, "unknown function 'foo'"},
		{`{"expression": "2 * Ans"}`, 400, "unknown_variable", 4, "unknown variable 'Ans'"},
		{`{"expression": "sqrt(1, 2)"}`, 400, "wrong_argument_count", 0, "wrong number of arguments for sqrt: want 1, got 2"},
		{`{"expression": "1/0"}`, 422, "division_by_zero", 1, "division by zero"},
		{`{"expression": "sqrt(-1)"}`, 422, "domain_error", 0, "result is not a real number"},
		{`{"expression": "10^1000000"}`, 422, "overflow", 2, "result is too large"},
		{`{"expression": "0.1^1000000"}`, 422, "underflow", 3, "result is too close to zero"},
	}
	for _, tt := range tests {
		t.Run(testName(tt.body), func(t *testing.T) {
			rec, r := post(t, tt.body)
			if rec.Code != tt.status || r.Error == nil {
				t.Fatalf("POST %s = %d %s, want %d %s", testName(tt.body), rec.Code, rec.Body, tt.status, tt.code)
			}
			pos := none
			if r.Error.Position != nil {
				pos = *r.Error.Position
			}
			if r.Error.Code != tt.code || pos != tt.pos || r.Error.Message != tt.msg {
				t.Errorf("POST %s error = %s %q at %d, want %s %q at %d",
					testName(tt.body), r.Error.Code, r.Error.Message, pos, tt.code, tt.msg, tt.pos)
			}
		})
	}
}

func TestUnreadableBody(t *testing.T) {
	h := NewHandler(slog.New(slog.DiscardHandler))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/calculate", iotest.ErrReader(errors.New("connection reset"))))
	var r reply
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || rec.Code != 400 || r.Error == nil ||
		r.Error.Code != "invalid_request" || r.Error.Message != "request body could not be read" {
		t.Errorf("response = %d %s, want 400 invalid_request", rec.Code, rec.Body)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h := NewHandler(slog.New(slog.DiscardHandler))
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec, _ := do(t, h, method, `{"expression": "1"}`)
			if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodPost {
				t.Errorf("%s = %d, Allow %q; want 405, Allow POST", method, rec.Code, rec.Header().Get("Allow"))
			}
		})
	}
}

// codedError is an error with a code but no position.
type codedError string

func (e codedError) Error() string { return "coded error" }
func (e codedError) Code() string  { return string(e) }

// Errors that eval never returns: an error without a code, or with a code
// missing from the status table, is a bug. It is logged and answered with 500.
// A known code without a position is answered without one.
func TestUnexpectedErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
		logged bool
	}{
		{"no code", errors.New("boom"), 500, "internal_error", true},
		{"unknown code", codedError("new_code"), 500, "internal_error", true},
		{"no position", codedError("overflow"), 422, "overflow", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var log bytes.Buffer
			evaluate := func(string, map[string]eval.Value) (eval.Value, error) {
				return eval.Value{}, tt.err
			}
			h := newHandler(evaluate, slog.New(slog.NewTextHandler(&log, nil)))
			rec, r := do(t, h, http.MethodPost, `{"expression": "1 + 1"}`)
			if rec.Code != tt.status || r.Error == nil || r.Error.Code != tt.code || r.Error.Position != nil {
				t.Errorf("response = %d %s, want %d %s without a position", rec.Code, rec.Body, tt.status, tt.code)
			}
			if logged := strings.Contains(log.String(), tt.err.Error()); logged != tt.logged {
				t.Errorf("log = %q, want the error logged: %v", log.String(), tt.logged)
			}
		})
	}
}

// Every code the API can return maps to 400 or 422, and the table holds no
// other codes. A new code in eval breaks this test instead of becoming a 500.
func TestEveryCodeHasAStatus(t *testing.T) {
	codes := append(eval.Codes(), codeInvalidRequest, codeInvalidAns, codeExpressionTooLong)
	for _, code := range codes {
		if status := statusByCode[code]; status != 400 && status != 422 {
			t.Errorf("code %s maps to %d, want 400 or 422", code, status)
		}
	}
	if len(statusByCode) != len(codes) {
		t.Errorf("statusByCode has %d codes, want the %d that can be returned", len(statusByCode), len(codes))
	}
}

// A client that disconnects while the response is written does not break
// the handler.
func TestWriteToClosedClient(t *testing.T) {
	writeJSON(failingWriter{httptest.NewRecorder()}, http.StatusOK, response{Result: "1"})
}

type failingWriter struct{ http.ResponseWriter }

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
