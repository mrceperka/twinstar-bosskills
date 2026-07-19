package ch

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"
)

// TestDuplicateKillAggregationRegression demonstrates the failure mode that
// motivated the character aggregate's switch from sumState(1) to
// uniqExactState(remote_id). Materialized views process both inserted rows even
// when a ReplacingMergeTree later collapses their shared sorting key.
func TestDuplicateKillAggregationRegression(t *testing.T) {
	dsn := os.Getenv("BK_CH_DSN")
	if dsn == "" {
		t.Skip("BK_CH_DSN not set; ClickHouse integration test")
	}

	db, err := Open(Options{DSN: dsn, MaxOpen: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	suffix := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(fmt.Sprintf("%d", time.Now().UnixNano()), "")
	source := "test_dedup_source_" + suffix
	oldTarget := "test_dedup_sum_" + suffix
	newTarget := "test_dedup_uniq_" + suffix
	oldMV := "test_dedup_sum_mv_" + suffix
	newMV := "test_dedup_uniq_mv_" + suffix

	objects := []string{oldMV, newMV, source, oldTarget, newTarget}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, name := range objects {
			_, _ = db.ExecContext(cleanupCtx, "DROP TABLE IF EXISTS "+name)
		}
	})

	execAll(t, ctx, db,
		"CREATE TABLE "+source+" (remote_id String, version UInt64) ENGINE = ReplacingMergeTree(version) ORDER BY remote_id",
		"CREATE TABLE "+oldTarget+" (kills AggregateFunction(sum, UInt64)) ENGINE = AggregatingMergeTree ORDER BY tuple()",
		"CREATE TABLE "+newTarget+" (kills AggregateFunction(uniqExact, String)) ENGINE = AggregatingMergeTree ORDER BY tuple()",
		"CREATE MATERIALIZED VIEW "+oldMV+" TO "+oldTarget+" AS SELECT sumState(toUInt64(1)) AS kills FROM "+source,
		"CREATE MATERIALIZED VIEW "+newMV+" TO "+newTarget+" AS SELECT uniqExactState(remote_id) AS kills FROM "+source,
		"INSERT INTO "+source+" (remote_id, version) VALUES ('same-kill', 1)",
		"INSERT INTO "+source+" (remote_id, version) VALUES ('same-kill', 2)",
	)

	var oldCount, newCount uint64
	if err := db.QueryRowContext(ctx, "SELECT sumMerge(kills) FROM "+oldTarget).Scan(&oldCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT uniqExactMerge(kills) FROM "+newTarget).Scan(&newCount); err != nil {
		t.Fatal(err)
	}
	t.Logf("duplicate kill counts: former sumState implementation=%d, current uniqExactState implementation=%d", oldCount, newCount)
	if oldCount != 2 {
		t.Fatalf("former sumState pattern returned %d, want 2 to demonstrate duplicate counting", oldCount)
	}
	if newCount != 1 {
		t.Fatalf("uniqExactState pattern returned %d, want 1", newCount)
	}
}

func execAll(t *testing.T, ctx context.Context, db *sql.DB, statements ...string) {
	t.Helper()
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("execute %q: %v", statement, err)
		}
	}
}
