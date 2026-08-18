package game

import (
	"testing"
	"time"
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

func TestHintRankRange(t *testing.T) {
	cases := []struct {
		best      int64
		low, high int64
	}{
		{0, 40, 100},
		{900, 40, 100},
		{300, 15, 35},
		{4, 3, 10},
	}
	for _, c := range cases {
		low, high := hintRankRange(c.best)
		if low != c.low || high != c.high {
			t.Errorf("hintRankRange(%d) = (%d, %d), want (%d, %d)", c.best, low, high, c.low, c.high)
		}
	}
}

func TestDateAndReset(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Prague")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(nil, nil, loc)

	// 22:30 UTC on 17 Aug is already 18 Aug in Prague (CEST, UTC+2).
	moment := time.Date(2026, 8, 17, 22, 30, 0, 0, time.UTC)
	if got := svc.PuzzleID(ModeDay, moment); got != "2026-08-18" {
		t.Errorf("PuzzleID(day) = %q, want 2026-08-18", got)
	}
	if got := svc.PuzzleID(ModeWeek, moment); got != "2026-W34" {
		t.Errorf("PuzzleID(week) = %q, want 2026-W34", got)
	}

	reset := svc.ResetsAt(ModeDay, moment)
	if want := time.Date(2026, 8, 19, 0, 0, 0, 0, loc); !reset.Equal(want) {
		t.Errorf("ResetsAt(day) = %s, want %s", reset, want)
	}

	// 18 Aug 2026 is a Tuesday, so the weekly puzzle rolls over the following Monday.
	weekReset := svc.ResetsAt(ModeWeek, moment)
	if want := time.Date(2026, 8, 24, 0, 0, 0, 0, loc); !weekReset.Equal(want) {
		t.Errorf("ResetsAt(week) = %s, want %s", weekReset, want)
	}
}

func TestParseMode(t *testing.T) {
	for in, want := range map[string]Mode{"": ModeDay, "day": ModeDay, "week": ModeWeek} {
		got, err := ParseMode(in)
		if err != nil || got != want {
			t.Errorf("ParseMode(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ParseMode("month"); err == nil {
		t.Error("ParseMode(month) should fail")
	}
}
