package game

import (
	"math"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"  Pes! ":     "pes",
		"ČERVEN\n":    "cerven",
		"multi-slovo": "multi-slovo",
		"123 abc":     "abc",
		"":            "",
		"loď":         "lod",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCosine(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	c := []float32{0, 1, 0}
	if math.Abs(cosine(a, b)-1) > 1e-6 {
		t.Errorf("identical vectors should be 1")
	}
	if math.Abs(cosine(a, c)) > 1e-6 {
		t.Errorf("orthogonal vectors should be 0")
	}
	if cosine(a, nil) != 0 {
		t.Errorf("nil vector should return 0")
	}
}
