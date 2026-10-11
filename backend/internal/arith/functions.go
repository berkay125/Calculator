package arith

import "github.com/cockroachdb/apd/v3"

// Func is a function that expressions can call by name, such as sqrt.
type Func struct {
	// Arity is the exact number of arguments the function takes.
	Arity int
	// Call computes the result. Callers must pass exactly Arity arguments.
	Call func(args []*apd.Decimal) (*apd.Decimal, error)
}

// functions holds every callable function under the name used in expressions.
// Names are case-sensitive. Adding a function only needs an entry here; the
// parser accepts any name and the evaluator looks it up.
var functions = map[string]Func{
	"sqrt": {Arity: 1, Call: func(args []*apd.Decimal) (*apd.Decimal, error) {
		return Sqrt(args[0])
	}},
}

// Lookup returns the function registered under name.
func Lookup(name string) (Func, bool) {
	f, ok := functions[name]
	return f, ok
}
