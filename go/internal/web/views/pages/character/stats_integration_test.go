package character

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/api"
	"github.com/mrceperka/twinstar-bosskills/go/internal/ch"
)

func TestStatsInsertClickHouseSmoke(t *testing.T) {
	if os.Getenv("BK_STATS_INSERT_SMOKE") != "1" {
		t.Skip("set BK_STATS_INSERT_SMOKE=1 to run")
	}
	dsn := os.Getenv("BK_CH_DSN")
	if dsn == "" {
		t.Skip("BK_CH_DSN is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := ch.Open(ch.Options{DSN: dsn, MaxOpen: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	characterName := os.Getenv("BK_STATS_SMOKE_CHARACTER")
	if characterName == "" {
		characterName = "Plaguis"
	}

	cli := api.NewClient(os.Getenv("BK_TWINSTAR_API_URL"))
	payload, err := cli.GetCharacterStats(ctx, "Helios", characterName)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	if err := insertStatsRow(ctx, db, buildStatsStorageRow("Helios", characterName, payload, now, "")); err != nil {
		t.Fatal(err)
	}

	row, err := loadStatsRow(ctx, db, "Helios", characterName)
	if err != nil {
		t.Fatal(err)
	}
	if !row.Found {
		t.Fatalf("inserted %s stats row was not found", characterName)
	}
	if row.RawJSON == "" || row.Health == 0 || row.Intellect == 0 {
		t.Fatalf("inserted row is incomplete: health=%d intellect=%d raw=%d bytes", row.Health, row.Intellect, len(row.RawJSON))
	}

	if _, class, _, _, _, err := lookupCharacter(ctx, db, "Helios", characterName); err == nil {
		row.CharacterClass = class
		vm := buildStatsViewModel(ctx, row, nil)
		if row.Mana == 0 && class == 4 && !statsViewHasMetric(vm, "Energy") {
			t.Fatalf("Slim-like rogue stats did not render Energy resource: %#v", vm.Groups[0].Metrics)
		}
	} else {
		t.Fatal(err)
	}
}

func statsViewHasMetric(vm StatsViewModel, label string) bool {
	for _, group := range vm.Groups {
		for _, metric := range group.Metrics {
			if metric.Label == label {
				return true
			}
		}
	}
	return false
}

func TestSpecSummaryClickHouseSmoke(t *testing.T) {
	if os.Getenv("BK_STATS_INSERT_SMOKE") != "1" {
		t.Skip("set BK_STATS_INSERT_SMOKE=1 to run")
	}
	dsn := os.Getenv("BK_CH_DSN")
	if dsn == "" {
		t.Skip("BK_CH_DSN is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := ch.Open(ch.Options{DSN: dsn, MaxOpen: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	guid, _, _, _, _, err := lookupCharacter(ctx, db, "Helios", "Plaguis")
	if err != nil {
		t.Fatal(err)
	}
	if guid == 0 {
		t.Skip("Plaguis not found in local ClickHouse")
	}

	rows, err := loadSpecSummary(ctx, db, "Helios", guid, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("Plaguis spec summary is empty")
	}
	if !rows[0].IsMostPlayed || rows[0].KillCount == 0 || rows[0].Spec == 0 {
		t.Fatalf("first summary row is not highlighted/countable: %#v", rows[0])
	}
}
