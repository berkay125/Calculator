package arith

import (
	"testing"

	"github.com/cockroachdb/apd/v3"
)

func TestLookupSqrt(t *testing.T) {
	f, ok := Lookup("sqrt")
	if !ok {
		t.Fatal(`Lookup("sqrt") not found`)
	}
	if f.Arity != 1 {
		t.Errorf("Arity = %d, want 1", f.Arity)
	}
	got, err := f.Call([]*apd.Decimal{mustParse(t, "16")})
	if err != nil {
		t.Fatalf("Call(16): %v", err)
	}
	if s := Format(got); s != "4" {
		t.Errorf("Call(16) = %q, want %q", s, "4")
	}
}

func TestLookupUnknown(t *testing.T) {
	for _, name := range []string{"foo", "SQRT", "Sqrt", ""} {
		if _, ok := Lookup(name); ok {
			t.Errorf("Lookup(%q) found a function, want none", name)
		}
	}
}
