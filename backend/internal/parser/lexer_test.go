package parser

import (
	"errors"
	"slices"
	"testing"
)

// tokenize lexes src to the end and returns every token up to and including
// tokEOF, or the tokens before the first error.
func tokenize(src string) ([]token, error) {
	l := &lexer{src: src}
	var toks []token
	for {
		tok, err := l.next()
		if err != nil {
			return toks, err
		}
		toks = append(toks, tok)
		if tok.kind == tokEOF {
			return toks, nil
		}
	}
}

func TestLexer(t *testing.T) {
	tests := []struct {
		in   string
		want []token
	}{
		{"", []token{{tokEOF, "", 0}}},
		{" \t\r\n", []token{{tokEOF, "", 4}}},
		{"2 * (3 + 4) ^ 2", []token{
			{tokNumber, "2", 0}, {tokStar, "*", 2}, {tokLParen, "(", 4}, {tokNumber, "3", 5},
			{tokPlus, "+", 7}, {tokNumber, "4", 9}, {tokRParen, ")", 10}, {tokCaret, "^", 12},
			{tokNumber, "2", 14}, {tokEOF, "", 15},
		}},
		{"+-*/^%(),", []token{
			{tokPlus, "+", 0}, {tokMinus, "-", 1}, {tokStar, "*", 2}, {tokSlash, "/", 3},
			{tokCaret, "^", 4}, {tokPercent, "%", 5}, {tokLParen, "(", 6}, {tokRParen, ")", 7},
			{tokComma, ",", 8}, {tokEOF, "", 9},
		}},
		// Number text is kept exactly as written.
		{"  12.50  ", []token{{tokNumber, "12.50", 2}, {tokEOF, "", 9}}},
		{".5", []token{{tokNumber, ".5", 0}, {tokEOF, "", 2}}},
		{"007", []token{{tokNumber, "007", 0}, {tokEOF, "", 3}}},
		{"-2^-1", []token{
			{tokMinus, "-", 0}, {tokNumber, "2", 1}, {tokCaret, "^", 2}, {tokMinus, "-", 3},
			{tokNumber, "1", 4}, {tokEOF, "", 5},
		}},
		{"200+10%", []token{
			{tokNumber, "200", 0}, {tokPlus, "+", 3}, {tokNumber, "10", 4}, {tokPercent, "%", 6},
			{tokEOF, "", 7},
		}},
		{"sqrt(1, 2)", []token{
			{tokIdent, "sqrt", 0}, {tokLParen, "(", 4}, {tokNumber, "1", 5}, {tokComma, ",", 6},
			{tokNumber, "2", 8}, {tokRParen, ")", 9}, {tokEOF, "", 10},
		}},
		{"Ans/log10", []token{{tokIdent, "Ans", 0}, {tokSlash, "/", 3}, {tokIdent, "log10", 4}, {tokEOF, "", 9}}},
		{"_x1", []token{{tokIdent, "_x1", 0}, {tokEOF, "", 3}}},
		// The lexer accepts adjacent tokens; the parser rejects them.
		{"2x", []token{{tokNumber, "2", 0}, {tokIdent, "x", 1}, {tokEOF, "", 2}}},
		// No scientific notation: "e5" is an identifier.
		{"1e5", []token{{tokNumber, "1", 0}, {tokIdent, "e5", 1}, {tokEOF, "", 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := tokenize(tt.in)
			if err != nil {
				t.Fatalf("tokenize(%q): %v", tt.in, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("tokenize(%q)\n got %v\nwant %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestLexerErrors(t *testing.T) {
	tests := []struct {
		in  string
		pos int
		msg string
	}{
		{"2 $ 3", 2, "unexpected character '$'"},
		{"2 = 3", 2, "unexpected character '='"},
		{"1.2.3", 3, "number has more than one decimal point"},
		{".5.5", 2, "number has more than one decimal point"},
		{"5.", 1, "decimal point must be followed by a digit"},
		{"5.+3", 1, "decimal point must be followed by a digit"},
		{"1..2", 1, "decimal point must be followed by a digit"},
		{"2 + .", 4, "decimal point must be followed by a digit"},
		// Non-ASCII characters are rejected, including letters in identifiers.
		{"2 × 3", 2, "unexpected character '×'"},
		{"√4", 0, "unexpected character '√'"},
		{"café", 3, "unexpected character 'é'"},
		{"1\x00", 1, `unexpected character '\x00'`},
		{"\xff", 0, "unexpected character '�'"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			_, err := tokenize(tt.in)
			var synErr *SyntaxError
			if !errors.As(err, &synErr) {
				t.Fatalf("tokenize(%q) error = %v, want SyntaxError", tt.in, err)
			}
			if synErr.Code() != "syntax_error" || synErr.Pos() != tt.pos || synErr.Error() != tt.msg {
				t.Errorf("tokenize(%q) error = %s %q at %d, want syntax_error %q at %d",
					tt.in, synErr.Code(), synErr.Error(), synErr.Pos(), tt.msg, tt.pos)
			}
		})
	}
}

func TestLexerRepeatsEOF(t *testing.T) {
	l := &lexer{src: " 1 "}
	if _, err := l.next(); err != nil {
		t.Fatalf("next: %v", err)
	}
	want := token{tokEOF, "", 3}
	for range 2 {
		if got, err := l.next(); err != nil || got != want {
			t.Errorf("next() = %v, %v; want %v", got, err, want)
		}
	}
}
