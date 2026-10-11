package parser

import (
	"fmt"
	"strings"
)

// Node is an expression in the syntax tree: a *Number, *Variable, *Call,
// *Unary, *Binary, *Percent or *Group. Every node records the position of the
// token that created it. String prints the subtree as a fully parenthesized
// S-expression, such as (* 2 (^ (group (+ 3 4)) 2)).
type Node interface {
	String() string
}

// Number is a number literal, kept exactly as written.
type Number struct {
	Text string
	Pos  int
}

// Variable is a name that is not followed by "(", such as Ans. The evaluator
// resolves it.
type Variable struct {
	Name string
	Pos  int
}

// Call is a function call such as sqrt(9). The parser accepts any name; the
// evaluator looks it up and checks the number of arguments.
type Call struct {
	Name string
	Args []Node
	Pos  int // position of the name
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

// Percent is a postfix "%" applied to X. Its value depends on where it appears
// (200 + 10% is 220, 200 * 10% is 20), so the evaluator applies the percent
// rule, not the parser.
type Percent struct {
	X   Node
	Pos int // position of "%"
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

func (n *Variable) String() string {
	return n.Name
}

// String prints the call as (call name args...). The "call" tag keeps a
// function named group apart from a Group node.
func (n *Call) String() string {
	parts := []string{"call", n.Name}
	for _, arg := range n.Args {
		parts = append(parts, arg.String())
	}
	return "(" + strings.Join(parts, " ") + ")"
}

func (n *Unary) String() string {
	return fmt.Sprintf("(%s %s)", n.Op, n.X)
}

func (n *Binary) String() string {
	return fmt.Sprintf("(%s %s %s)", n.Op, n.L, n.R)
}

func (n *Percent) String() string {
	return fmt.Sprintf("(%% %s)", n.X)
}

func (n *Group) String() string {
	return fmt.Sprintf("(group %s)", n.Inner)
}
