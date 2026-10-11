package eval

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"

	"calculator/internal/arith"
	"calculator/internal/parser"
)

// Evaluate parses expression and computes its value. vars holds the values of
// variables such as Ans; names are case-sensitive. The result is rounded to
// arith.Precision significant digits, even when no operation was applied (a
// long literal, Ans, -x).
//
// Every error has a code and the position of the input that caused it:
//   - Malformed input returns the parser's errors.
//   - Unknown names and wrong argument counts are checked before anything is
//     computed, so an invalid expression is reported as such even when
//     evaluating it would also divide by zero.
//   - Math errors point at the operator or function that failed.
func Evaluate(expression string, vars map[string]Value) (Value, error) {
	root, err := parser.Parse(expression)
	if err != nil {
		return Value{}, err
	}
	e := &evaluator{vars: vars}
	if err := e.check(root); err != nil {
		return Value{}, err
	}
	d, err := e.eval(root)
	if err != nil {
		return Value{}, err
	}
	if d, err = arith.Round(d); err != nil {
		return Value{}, at(root.Position(), err)
	}
	return Value{d}, nil
}

type evaluator struct {
	vars map[string]Value
}

// check returns the first unknown name or wrong argument count in n, in input
// order.
func (e *evaluator) check(n parser.Node) error {
	switch n := n.(type) {
	case *parser.Variable:
		if _, ok := e.vars[n.Name]; !ok {
			return unknownVariable(n)
		}
	case *parser.Call:
		f, ok := arith.Lookup(n.Name)
		if !ok {
			return unknownFunction(n)
		}
		if len(n.Args) != f.Arity {
			return wrongArgumentCount(n, f.Arity)
		}
		for _, arg := range n.Args {
			if err := e.check(arg); err != nil {
				return err
			}
		}
	case *parser.Unary:
		return e.check(n.X)
	case *parser.Binary:
		if err := e.check(n.L); err != nil {
			return err
		}
		return e.check(n.R)
	case *parser.Percent:
		return e.check(n.X)
	case *parser.Group:
		return e.check(n.Inner)
	}
	return nil
}

// binaryOps maps each binary operator the parser produces to its operation.
var binaryOps = map[string]func(x, y *apd.Decimal) (*apd.Decimal, error){
	"+": arith.Add,
	"-": arith.Sub,
	"*": arith.Mul,
	"/": arith.Div,
	"^": arith.Pow,
}

// eval computes the value of n, which must have passed check. Operands are
// evaluated left to right, and the first error is returned.
func (e *evaluator) eval(n parser.Node) (*apd.Decimal, error) {
	switch n := n.(type) {
	case *parser.Number:
		d, err := arith.Parse(n.Text)
		return d, at(n.Pos, err)
	case *parser.Variable:
		return e.vars[n.Name].decimal(), nil
	case *parser.Call:
		return e.call(n)
	case *parser.Unary:
		// The only prefix operator is minus.
		x, err := e.eval(n.X)
		if err != nil {
			return nil, err
		}
		return arith.Neg(x), nil
	case *parser.Binary:
		return e.binary(n)
	case *parser.Percent:
		x, err := e.eval(n.X)
		if err != nil {
			return nil, err
		}
		d, err := arith.Percent(x)
		return d, at(n.Pos, err)
	case *parser.Group:
		return e.eval(n.Inner)
	}
	return nil, fmt.Errorf("eval: unexpected node %T", n)
}

func (e *evaluator) call(n *parser.Call) (*apd.Decimal, error) {
	f, _ := arith.Lookup(n.Name) // check has verified the name and the arity
	args := make([]*apd.Decimal, len(n.Args))
	for i, arg := range n.Args {
		d, err := e.eval(arg)
		if err != nil {
			return nil, err
		}
		args[i] = d
	}
	d, err := f.Call(args)
	return d, at(n.Pos, err)
}

// binary applies a binary operator. For a + b% and a - b% it applies the
// percent rule: b% means b percent of a, so 200 + 10% is 220.
func (e *evaluator) binary(n *parser.Binary) (*apd.Decimal, error) {
	l, err := e.eval(n.L)
	if err != nil {
		return nil, err
	}
	var r *apd.Decimal
	if pct, negated, ok := percentOperand(n.R); ok && (n.Op == "+" || n.Op == "-") {
		r, err = e.percentOf(l, pct, negated)
	} else {
		r, err = e.eval(n.R)
	}
	if err != nil {
		return nil, err
	}
	d, err := binaryOps[n.Op](l, r)
	return d, at(n.Pos, err)
}

// percentOperand reports whether n is a Percent node behind any number of
// unary minus signs, and whether an odd number of them negates it. A Group is
// not looked into, which is how parentheses block the percent rule.
func percentOperand(n parser.Node) (pct *parser.Percent, negated, ok bool) {
	for {
		switch x := n.(type) {
		case *parser.Percent:
			return x, negated, true
		case *parser.Unary:
			negated = !negated
			n = x.X
		default:
			return nil, false, false
		}
	}
}

// percentOf returns b percent of a, where pct is b% and negated says whether
// minus signs in front of it flip its sign. An error points at the "%".
func (e *evaluator) percentOf(a *apd.Decimal, pct *parser.Percent, negated bool) (*apd.Decimal, error) {
	b, err := e.eval(pct.X)
	if err != nil {
		return nil, err
	}
	if negated {
		b = arith.Neg(b)
	}
	d, err := arith.PercentOf(a, b)
	return d, at(pct.Pos, err)
}
