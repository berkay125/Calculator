package parser

import (
	"fmt"
	"unicode/utf8"
)

// tokenKind identifies what a token is.
type tokenKind int

const (
	tokEOF     tokenKind = iota
	tokNumber            // 12, 3.5, .5
	tokIdent             // sqrt, Ans
	tokPlus              // +
	tokMinus             // -
	tokStar              // *
	tokSlash             // /
	tokCaret             // ^
	tokPercent           // %
	tokLParen            // (
	tokRParen            // )
	tokComma             // ,
)

// symbols maps each single-character token to its kind.
var symbols = map[byte]tokenKind{
	'+': tokPlus,
	'-': tokMinus,
	'*': tokStar,
	'/': tokSlash,
	'^': tokCaret,
	'%': tokPercent,
	'(': tokLParen,
	')': tokRParen,
	',': tokComma,
}

// token is one lexical unit of an expression.
type token struct {
	kind tokenKind
	text string // the token as written in the input; empty for tokEOF
	pos  int    // index of the token's first character in the input
}

// lexer splits an expression into tokens on demand: the parser pulls one token
// at a time and stops at the first error, whether lexical or syntactic.
//
// Only ASCII is accepted. The lexer stops at the first non-ASCII character, so
// every position it reports is a byte index, a rune index and a JavaScript
// string index at the same time.
type lexer struct {
	src string
	pos int // index of the next unread byte
}

// next returns the next token, skipping whitespace. At the end of the input it
// returns a tokEOF token positioned at len(src), on every call.
func (l *lexer) next() (token, error) {
	l.skip(isSpace)
	start := l.pos
	if start == len(l.src) {
		return token{kind: tokEOF, pos: start}, nil
	}
	c := l.src[start]
	switch {
	case isDigit(c) || c == '.':
		return l.number()
	case isIdentStart(c):
		l.skip(isIdentPart)
		return token{kind: tokIdent, text: l.src[start:l.pos], pos: start}, nil
	}
	if kind, ok := symbols[c]; ok {
		l.pos++
		return token{kind: kind, text: l.src[start:l.pos], pos: start}, nil
	}
	r, _ := utf8.DecodeRuneInString(l.src[start:])
	return token{}, &SyntaxError{Msg: fmt.Sprintf("unexpected character %q", r), Offset: start}
}

// number scans a literal of the form digits ["." digits] or "." digits.
// Scientific notation is not part of the grammar: in "1e5" the number ends
// after "1" and "e5" is lexed as an identifier, which the parser rejects.
func (l *lexer) number() (token, error) {
	start := l.pos
	l.skip(isDigit)
	if l.peek() == '.' {
		dot := l.pos
		l.pos++
		if l.skip(isDigit) == 0 {
			return token{}, &SyntaxError{Msg: "decimal point must be followed by a digit", Offset: dot}
		}
	}
	if l.peek() == '.' {
		return token{}, &SyntaxError{Msg: "number has more than one decimal point", Offset: l.pos}
	}
	return token{kind: tokNumber, text: l.src[start:l.pos], pos: start}, nil
}

// peek returns the next unread byte, or 0 at the end of the input.
func (l *lexer) peek() byte {
	if l.pos < len(l.src) {
		return l.src[l.pos]
	}
	return 0
}

// skip advances past the bytes that satisfy ok and reports how many it passed.
func (l *lexer) skip(ok func(byte) bool) int {
	start := l.pos
	for l.pos < len(l.src) && ok(l.src[l.pos]) {
		l.pos++
	}
	return l.pos - start
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

func isIdentStart(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_'
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}
