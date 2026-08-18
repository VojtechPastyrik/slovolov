package game

import "testing"

func TestSummariseEmpty(t *testing.T) {
	got := summarise(nil)
	if got.Players != 0 || got.Median != 0 {
		t.Fatalf("empty distribution gave players=%d median=%d, want zeroes", got.Players, got.Median)
	}
	if len(got.Buckets) != len(statBuckets) {
		t.Fatalf("got %d buckets, want %d even with no players", len(got.Buckets), len(statBuckets))
	}
}

func TestSummariseCountsAndMedian(t *testing.T) {
	// 1+2+4+3 = 10 players; the 5th and 6th both sit at 12 guesses.
	got := summarise(map[int64]int64{3: 1, 8: 2, 12: 4, 30: 3})

	if got.Players != 10 {
		t.Fatalf("players = %d, want 10", got.Players)
	}
	if got.Median != 12 {
		t.Fatalf("median = %d, want 12", got.Median)
	}
}

// The median is the number that has to survive a joker reporting a solve in
// one guess and another sitting on a thousand.
func TestSummariseMedianResistsOutliers(t *testing.T) {
	clean := map[int64]int64{14: 5, 15: 5}
	dirty := map[int64]int64{1: 1, 14: 5, 15: 5, 4000: 1}

	if a, b := summarise(clean).Median, summarise(dirty).Median; a != b {
		t.Fatalf("median moved from %d to %d when outliers were added", a, b)
	}
}

func TestSummariseBucketsByRange(t *testing.T) {
	got := summarise(map[int64]int64{1: 2, 5: 1, 6: 1, 100: 3, 101: 1})

	want := map[string]int64{"1–5": 3, "6–10": 1, "11–20": 0, "21–50": 0, "51–100": 3, "100+": 1}
	for _, b := range got.Buckets {
		if b.Players != want[b.Label] {
			t.Errorf("bucket %q has %d players, want %d", b.Label, b.Players, want[b.Label])
		}
	}
}

// The rank a player sees is read off the score, so two guesses the model rated
// identically must still land on different ranks.
func TestJitterSeparatesEqualScores(t *testing.T) {
	a, b := jitter("pekařka", 40), jitter("pizza", 40)
	if a == b {
		t.Fatalf("two words with score 40 both jittered to %v", a)
	}
}

func TestJitterIsStable(t *testing.T) {
	if first, second := jitter("pizza", 40), jitter("pizza", 40); first != second {
		t.Fatalf("same word jittered to %v then %v", first, second)
	}
}

// The offset must be small enough that it never reorders words the model
// actually placed apart — the model reports one decimal place.
func TestJitterNeverReordersRealDifferences(t *testing.T) {
	for _, word := range []string{"pekař", "pizza", "doprava", "chleba", "voda"} {
		if got := jitter(word, 40); got < 40 || got >= 40.1 {
			t.Fatalf("jitter(%q, 40) = %v, want inside [40, 40.1)", word, got)
		}
	}
}
