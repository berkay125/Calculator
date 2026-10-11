package arith

import "testing"

func TestFormat(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"0", "0"},
		{"-0", "0"},
		{"0.000", "0"},
		{"123.456", "123.456"},
		{"1.500", "1.5"},
		{"100", "100"},
		{"-42", "-42"},
		// Plain notation up to an adjusted exponent of Precision-1 ...
		{"1E+33", "1000000000000000000000000000000000"},
		{"-9.5E+33", "-9500000000000000000000000000000000"},
		// ... scientific above it.
		{"1E+34", "1E+34"},
		{"1.5E+40", "1.5E+40"},
		{"-1.5E+40", "-1.5E+40"},
		// Plain notation down to an adjusted exponent of -7 ...
		{"1E-7", "0.0000001"},
		{"1.25E-7", "0.000000125"},
		// ... scientific below it.
		{"1E-8", "1E-8"},
		{"-1.5E-10", "-1.5E-10"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := Format(mustParse(t, tt.in)); got != tt.want {
				t.Errorf("Format(%s) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatDoesNotModifyInput(t *testing.T) {
	d := mustParse(t, "1.500")
	Format(d)
	if got := d.Text('f'); got != "1.500" {
		t.Errorf("input changed to %s", got)
	}
}

// The frontend sends a previous result back as "ans", so Parse must accept
// everything Format produces and return the same value.
func TestFormatRoundTrip(t *testing.T) {
	for _, in := range []string{"0", "-42", "0.3333333333333333333333333333333333", "1E+34", "-1.5E-10", "1E+6144", "1E-6143"} {
		t.Run(in, func(t *testing.T) {
			d := mustParse(t, in)
			back := mustParse(t, Format(d))
			if back.Cmp(d) != 0 {
				t.Errorf("Parse(Format(%s)) = %s", in, back)
			}
		})
	}
}
