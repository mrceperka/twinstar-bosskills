package metric

import "testing"

func TestDPS(t *testing.T) {
	cases := []struct {
		name   string
		dmg    int64
		length int64
		want   int64
	}{
		{"round", 3_000_000, 60_000, 50_000},
		{"zero length", 100, 0, 0},
		{"negative length", 100, -1, 0},
		{"one-second fight", 1000, 1000, 1000},
	}
	for _, c := range cases {
		if got := DPS(c.dmg, c.length); got != c.want {
			t.Errorf("%s: DPS(%d,%d) = %d, want %d", c.name, c.dmg, c.length, got, c.want)
		}
	}
}

func TestHPS(t *testing.T) {
	cases := []struct {
		name   string
		heal   int64
		absorb int64
		length int64
		want   int64
	}{
		{"heal + absorb", 900_000, 300_000, 60_000, 20_000},
		{"zero length returns zero", 100, 0, 0, 0},
	}
	for _, c := range cases {
		if got := HPS(c.heal, c.absorb, c.length); got != c.want {
			t.Errorf("%s: HPS(%d,%d,%d) = %d, want %d", c.name, c.heal, c.absorb, c.length, got, c.want)
		}
	}
}

func TestSQLUInt64(t *testing.T) {
	if got := SQLUInt64(DmgDoneArrayJoin); got != "toUInt64(players.dmg_done * 1000 / greatest(length, 1))" {
		t.Errorf("SQLUInt64(DmgDoneArrayJoin) = %q", got)
	}
	if got := SQLUInt64(HealAbsorbIndexed); got != "toUInt64((players.healing_done[idx] + players.absorb_done[idx]) * 1000 / greatest(length, 1))" {
		t.Errorf("SQLUInt64(HealAbsorbIndexed) = %q", got)
	}
}

func TestSQLFloat64(t *testing.T) {
	if got := SQLFloat64(DmgDoneArrayJoin); got != "toFloat64(players.dmg_done) * 1000 / greatest(length, 1)" {
		t.Errorf("SQLFloat64(DmgDoneArrayJoin) = %q", got)
	}
}
