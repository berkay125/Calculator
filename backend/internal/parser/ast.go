package parser

import "fmt"

// Node is an expression in the syntax tree: a *Number, *Unary, *Binary or
// *Group. Every node records the position of the token that created it.
// String prints the subtree as a fully parenthesized S-expression, such as
// (* 2 (^ (group (+ 3 4)) 2)).
type Node interface {
	String() string
}

// Number is a number literal, kept exactly as written.
type Number struct {
	Text string
	Pos  int
}

// Unary is a prefix operator applied to X. The only one is "-".
type Unary struct {
	Op  string
	X   Node
	Pos int // position of the operator
}

// Binary is an infix operator: "+", "-", "*", "/" or "^".
type Binary struct {
	Op   string
	L, R Node
	Pos  int // position of the operator
}

// Group is a parenthesized expression. It is a node of its own because
// parentheses change how percent is evaluated: 100 + (10%) is not 100 + 10%.
type Group struct {
	Inner Node
	Pos   int // position of "("
}

func (n *Number) String() string {
	return n.Text
}

func (n *Unary) String() string {
	return fmt.Sprintf("(%s %s)", n.Op, n.X)
}

func (n *Binary) String() string {
	return fmt.Sprintf("(%s %s %s)", n.Op, n.L, n.R)
}

func (n *Group) String() string {
	return fmt.Sprintf("(group %s)", n.Inner)
}
