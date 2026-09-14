package ch

import (
	"reflect"
	"testing"
)

func TestSplitStatements_Basic(t *testing.T) {
	in := "CREATE TABLE a (x UInt8) ENGINE=Memory;\nINSERT INTO a VALUES (1);"
	want := []string{
		"CREATE TABLE a (x UInt8) ENGINE=Memory",
		"INSERT INTO a VALUES (1)",
	}
	got := SplitStatements(in)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestSplitStatements_IgnoresSemiInsideString(t *testing.T) {
	in := "INSERT INTO t VALUES ('a;b'); SELECT 1;"
	got := SplitStatements(in)
	if len(got) != 2 {
		t.Fatalf("want 2 statements, got %d: %#v", len(got), got)
	}
	if got[0] != "INSERT INTO t VALUES ('a;b')" {
		t.Errorf("stmt 0: %q", got[0])
	}
}

func TestSplitStatements_IgnoresComments(t *testing.T) {
	in := `
-- header comment;
CREATE TABLE t (
    x UInt8 -- inline; not a terminator
) ENGINE=Memory;
/* block; with;
   semis */
SELECT 1;
`
	got := SplitStatements(in)
	if len(got) != 2 {
		t.Fatalf("want 2 statements, got %d: %#v", len(got), got)
	}
}

func TestSplitStatements_DropsEmpty(t *testing.T) {
	in := ";;\n; SELECT 1; ;\n;"
	got := SplitStatements(in)
	if len(got) != 1 || got[0] != "SELECT 1" {
		t.Fatalf("got %#v", got)
	}
}
