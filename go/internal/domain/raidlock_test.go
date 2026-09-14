package domain

import (
	"testing"
	"time"
)

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

func TestRaidLock_ResetBoundary(t *testing.T) {
	cases := []struct {
		name      string
		at        string
		wantStart string
	}{
		// Thursday afternoon -> previous Wednesday 06:00 UTC same week
		{"Thursday after reset", "2026-06-25T15:00:00Z", "2026-06-24T06:00:00Z"},
		// Wednesday 05:59 UTC -> previous week's Wednesday 06:00
		{"Just before reset", "2026-06-24T05:59:59Z", "2026-06-17T06:00:00Z"},
		// Wednesday 06:00 UTC sharp -> same Wednesday
		{"Exactly at reset", "2026-06-24T06:00:00Z", "2026-06-24T06:00:00Z"},
		// Wednesday 06:00:01 UTC -> same Wednesday
		{"One second after reset", "2026-06-24T06:00:01Z", "2026-06-24T06:00:00Z"},
		// Tuesday morning -> previous Wednesday
		{"Tuesday morning", "2026-06-23T08:00:00Z", "2026-06-17T06:00:00Z"},
		// Sunday -> previous Wednesday
		{"Sunday", "2026-06-21T12:00:00Z", "2026-06-17T06:00:00Z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			at := mustParse(t, c.at)
			want := mustParse(t, c.wantStart)
			got := RaidLock(at, 0)
			if !got.Start.Equal(want) {
				t.Errorf("Start: got %s, want %s", got.Start, want)
			}
			if !got.End.Equal(want.AddDate(0, 0, 7)) {
				t.Errorf("End: got %s, want %s", got.End, want.AddDate(0, 0, 7))
			}
		})
	}
}

func TestRaidLock_Shift(t *testing.T) {
	at := mustParse(t, "2026-06-25T15:00:00Z")
	cur := RaidLock(at, 0)
	prev := RaidLock(at, 1)
	twoBack := RaidLock(at, 2)
	if !prev.Start.Equal(cur.Start.AddDate(0, 0, -7)) {
		t.Errorf("shift=1: got start %s, want %s", prev.Start, cur.Start.AddDate(0, 0, -7))
	}
	if !twoBack.Start.Equal(cur.Start.AddDate(0, 0, -14)) {
		t.Errorf("shift=2: got start %s, want %s", twoBack.Start, cur.Start.AddDate(0, 0, -14))
	}
}

func TestRaidLock_TimezoneInputNormalized(t *testing.T) {
	// Input in a non-UTC location should still produce UTC-anchored boundaries.
	loc, _ := time.LoadLocation("America/Los_Angeles")
	// 2026-06-25 08:00 PDT == 2026-06-25 15:00 UTC
	at := time.Date(2026, 6, 25, 8, 0, 0, 0, loc)
	got := RaidLock(at, 0)
	want := mustParse(t, "2026-06-24T06:00:00Z")
	if !got.Start.Equal(want) {
		t.Errorf("got %s, want %s", got.Start, want)
	}
}
