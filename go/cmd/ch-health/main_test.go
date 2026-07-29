package main

import (
	"strings"
	"testing"
)

// The %d verbs are filled positionally, so a section that declares the wrong
// flags silently puts --hours into a LIMIT clause. Assert the declaration
// matches the SQL for every section.
func TestSectionVerbsMatchFlags(t *testing.T) {
	for _, s := range sections {
		want := 0
		if s.hours {
			want++
		}
		if s.limit {
			want++
		}
		if got := strings.Count(s.sql, "%d"); got != want {
			t.Errorf("section %q: %d %%d verbs, but flags declare %d", s.name, got, want)
		}
		// Sprintf only runs when args exist, so a literal % (e.g. LIKE '%Memory%')
		// is safe in an argument-free section and corrupting in one with args.
		if want > 0 {
			if strings.Count(s.sql, "%") != strings.Count(s.sql, "%d") {
				t.Errorf("section %q takes args but contains a literal %%; escape it as %%%%", s.name)
			}
		}
	}
}

func TestSectionSQLArgOrder(t *testing.T) {
	s := section{sql: "hours=%d limit=%d", hours: true, limit: true}
	if got, want := sectionSQL(s, 24, 5), "hours=24 limit=5"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// A limit-only section must receive limit, not hours.
	s = section{sql: "LIMIT %d", limit: true}
	if got, want := sectionSQL(s, 24, 5), "LIMIT 5"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	s = section{sql: "toIntervalHour(%d)", hours: true}
	if got, want := sectionSQL(s, 24, 5), "toIntervalHour(24)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// No flags: returned verbatim, Sprintf never runs.
	s = section{sql: "LIKE '%Memory%'"}
	if got, want := sectionSQL(s, 24, 5), "LIKE '%Memory%'"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPickSections(t *testing.T) {
	if got := len(mustPick(t, "")); got != len(sections) {
		t.Errorf("empty --only selected %d sections, want all %d", got, len(sections))
	}

	got := mustPick(t, "cache, cache-hits")
	if len(got) != 2 || got[0].name != "cache" || got[1].name != "cache-hits" {
		t.Errorf("unexpected selection: %+v", got)
	}

	if _, err := pickSections("nope"); err == nil {
		t.Error("unknown section: want error, got nil")
	}
	if _, err := pickSections(",,"); err == nil {
		t.Error("only separators: want error, got nil")
	}
}

func mustPick(t *testing.T, only string) []section {
	t.Helper()
	got, err := pickSections(only)
	if err != nil {
		t.Fatalf("pickSections(%q): %v", only, err)
	}
	return got
}
