package llm

import "testing"

// The rank a player sees is read straight off the score, so the one property
// the ranking must never lose is that no two words share a score.
func TestSpreadIsStrictlyDecreasingInsideBand(t *testing.T) {
	words := make([]string, 1100)
	for i := range words {
		words[i] = string(rune('a' + i%26))
	}

	got := spread(words, 0, 40)
	if len(got) != len(words) {
		t.Fatalf("spread returned %d words, want %d", len(got), len(words))
	}
	for i, w := range got {
		if w.Score <= 0 || w.Score >= 40 {
			t.Fatalf("word %d scored %v, want inside (0, 40)", i, w.Score)
		}
		if i > 0 && w.Score >= got[i-1].Score {
			t.Fatalf("word %d scored %v, not below its predecessor %v", i, w.Score, got[i-1].Score)
		}
	}
}

func TestSpreadEmpty(t *testing.T) {
	if got := spread(nil, 0, 40); got != nil {
		t.Fatalf("spread(nil) = %v, want nil", got)
	}
}

// Chunks of a band are thematic slices of equal closeness, so merging must
// interleave them by relative position rather than concatenating them.
func TestMergeInterleavesByRelativePosition(t *testing.T) {
	got := merge([][]string{
		{"a1", "a2", "a3", "a4"},
		{"b1", "b2"},
	})

	want := []string{"a1", "b1", "a2", "a3", "b2", "a4"}
	if len(got) != len(want) {
		t.Fatalf("merge returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("merge returned %v, want %v", got, want)
		}
	}
}

// Two chunks of the same length put their n-th words at the same relative
// position; the merge has to stay deterministic there.
func TestMergeBreaksTiesByChunkOrder(t *testing.T) {
	got := merge([][]string{{"a1", "a2"}, {"b1", "b2"}})

	want := []string{"a1", "b1", "a2", "b2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("merge returned %v, want %v", got, want)
		}
	}
}

// Bands sit next to each other on one 0-100 scale, so a word at the bottom of
// a band must still outrank the top of the band below it.
func TestBandsDoNotOverlapAfterSpread(t *testing.T) {
	words := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = "w"
		}
		return out
	}

	prevLow := 100.0
	for _, b := range bands {
		got := spread(words(b.count), b.low, b.high)
		if got[0].Score >= prevLow {
			t.Fatalf("band %.0f-%.0f starts at %v, overlapping the band above at %v", b.low, b.high, got[0].Score, prevLow)
		}
		prevLow = got[len(got)-1].Score
	}
}
