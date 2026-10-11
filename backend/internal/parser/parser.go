package parser

import "fmt"

// MaxDepth is the deepest parenthesis nesting that Parse accepts.
const MaxDepth = 100

// The precedence table. An operator with a higher power binds tighter.
//
//	power  operator  associativity
//	1      + -       left
//	2      * /       left
//	3      unary -   prefix
//	4      ^         right
var binaryOps = map[tokenKind]struct {
	power      int
	rightAssoc bool
}{
	tokPlus:  {power: 1},
	tokMinus: {power: 1},
	tokStar:  {power: 2},
	tokSlash: {power: 2},
	tokCaret: {power: 4, rightAssoc: true},
}

// unaryPower is the binding power of unary minus. It sits between * and ^, so
// -2^2 is -(2^2) while -2*3 is (-2)*3.
const unaryPower = 3

// Parse turns an expression into a syntax tree. Malformed input returns a
// *SyntaxError, and parentheses nested deeper than MaxDepth return a
// *NestingError. Both report the position of the offending character.
func Parse(src string) (Node, error) {
	p := &parser{lex: lexer{src: src}}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.tok.kind == tokEOF {
		return nil, &SyntaxError{Msg: "empty expression", Offset: p.tok.pos}
	}
	n, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	if p.tok.kind != tokEOF {
		return nil, p.unexpected()
	}
	return n, nil
}

type parser struct {
	lex   lexer
	tok   token // the current token, not yet consumed
	depth int   // number of open parentheses around the current token
}

// advance replaces the current token with the next one from the lexer.
func (p *parser) advance() error {
	tok, err := p.lex.next()
	if err != nil {
		return err
	}
	p.tok = tok
	return nil
}

// expr parses an expression whose binary operators, outside parentheses, all
// have at least minPower. A looser operator ends the expression and is left
// for the caller.
func (p *parser) expr(minPower int) (Node, error) {
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		op, ok := binaryOps[p.tok.kind]
		if !ok || op.power < minPower {
			return left, nil
		}
		opTok := p.tok
		if err := p.advance(); err != nil {
			return nil, err
		}
		// The right operand of a left-associative operator only takes tighter
		// operators, so 1-2-3 is (1-2)-3. A right-associative operator also
		// takes its own level, so 2^3^2 is 2^(3^2).
		next := op.power + 1
		if op.rightAssoc {
			next = op.power
		}
		right, err := p.expr(next)
		if err != nil {
			return nil, err
		}
		left = &Binary{Op: opTok.text, L: left, R: right, Pos: opTok.pos}
	}
}

// unary parses an operand with any number of leading minus signs. The operand
// of a minus sign only takes operators that bind tighter than unaryPower.
func (p *parser) unary() (Node, error) {
	if p.tok.kind != tokMinus {
		return p.primary()
	}
	opTok := p.tok
	if err := p.advance(); err != nil {
		return nil, err
	}
	x, err := p.expr(unaryPower)
	if err != nil {
		return nil, err
	}
	return &Unary{Op: opTok.text, X: x, Pos: opTok.pos}, nil
}

// primary parses a number or a parenthesized expression.
func (p *parser) primary() (Node, error) {
	switch p.tok.kind {
	case tokNumber:
		n := &Number{Text: p.tok.text, Pos: p.tok.pos}
		if err := p.advance(); err != nil {
			return nil, err
		}
		return n, nil
	case tokLParen:
		return p.group()
	}
	return nil, p.unexpected()
}

// group parses "(" expression ")" into a Group node.
func (p *parser) group() (Node, error) {
	open := p.tok
	if p.depth == MaxDepth {
		return nil, &NestingError{Offset: open.pos}
	}
	p.depth++
	if err := p.advance(); err != nil {
		return nil, err
	}
	inner, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	if p.tok.kind != tokRParen {
		if p.tok.kind == tokEOF {
			return nil, &SyntaxError{Msg: "unclosed parenthesis", Offset: open.pos}
		}
		return nil, p.unexpected()
	}
	p.depth--
	if err := p.advance(); err != nil {
		return nil, err
	}
	return &Group{Inner: inner, Pos: open.pos}, nil
}

// unexpected reports the current token as a syntax error.
func (p *parser) unexpected() error {
	if p.tok.kind == tokEOF {
		return &SyntaxError{Msg: "unexpected end of expression", Offset: p.tok.pos}
	}
	return &SyntaxError{Msg: fmt.Sprintf("unexpected '%s'", p.tok.text), Offset: p.tok.pos}
}
