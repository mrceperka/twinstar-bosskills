package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/api"
	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
)

// syncOptions controls one realm's sync run.
type syncOptions struct {
	Realm       string
	StartsAt    *time.Time
	EndsAt      *time.Time
	BosskillIDs []string
	BossIDs     []uint32
	Page        *int // 0-indexed; if set, only this page is fetched
	PageSize    *int
	BatchSize   int // CH insert batch size
	Concurrency int
}

// syncRealm pulls everything the upstream API knows about a realm into CH.
func syncRealm(ctx context.Context, log *slog.Logger, db *sql.DB, cli *api.Client, opts syncOptions) error {
	if !realm.IsKnown(opts.Realm) {
		return fmt.Errorf("unknown realm %q", opts.Realm)
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 200
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	expansion := realm.Expansion(opts.Realm)

	log = log.With("realm", opts.Realm)
	log.Info("fetching raids")
	raids, err := cli.GetRaids(ctx, opts.Realm, expansion)
	if err != nil {
		return fmt.Errorf("getRaids: %w", err)
	}

	log.Info("upserting raid/boss lookups", "raids", len(raids))
	if err := upsertRaidsAndBosses(ctx, db, opts.Realm, raids); err != nil {
		return fmt.Errorf("upsertRaidsAndBosses: %w", err)
	}

	wantBossIDs := map[uint32]bool{}
	for _, id := range opts.BossIDs {
		wantBossIDs[id] = true
	}

	var inserted, skipped, failed int
	for _, raid := range raids {
		for _, boss := range raid.Bosses {
			bossID := uint32(boss.Entry)
			if len(wantBossIDs) > 0 && !wantBossIDs[bossID] {
				continue
			}
			n, sk, fl, err := syncBoss(ctx, log, db, cli, opts, raid.Map, boss)
			if err != nil {
				log.Error("syncBoss", "boss", boss.Name, "err", err)
			}
			inserted += n
			skipped += sk
			failed += fl
		}
	}
	log.Info("realm done", "inserted", inserted, "skipped", skipped, "failed", failed)
	return nil
}

func syncBoss(
	ctx context.Context,
	log *slog.Logger,
	db *sql.DB,
	cli *api.Client,
	opts syncOptions,
	raidName string,
	boss api.Boss,
) (inserted, skipped, failed int, err error) {
	log = log.With("boss", boss.Name, "boss_entry", boss.Entry)

	q := api.Query{
		Realm: opts.Realm,
		Filters: []api.Filter{
			{Column: "entry", Operator: api.OpEquals, Value: boss.Entry},
		},
	}
	if opts.StartsAt != nil {
		q.Filters = append(q.Filters, api.Filter{
			Column: "time", Operator: api.OpGTE, Value: opts.StartsAt.UTC().Format(time.RFC3339),
		})
	}
	if opts.EndsAt != nil {
		q.Filters = append(q.Filters, api.Filter{
			Column: "time", Operator: api.OpLTE, Value: opts.EndsAt.UTC().Format(time.RFC3339),
		})
	}
	if len(opts.BosskillIDs) > 0 {
		q.Filters = append(q.Filters, api.Filter{
			Column: "id", Operator: api.OpIn, Value: opts.BosskillIDs,
		})
	}

	// page/pageSize collapse to "fetch only this page"; otherwise list all.
	var bosskills []api.BossKill
	if opts.Page != nil || opts.PageSize != nil {
		if opts.Page != nil {
			q.Page = *opts.Page
		}
		if opts.PageSize != nil {
			q.PageSize = *opts.PageSize
		}
		res, err := cli.GetLatestBossKills(ctx, q)
		if err != nil {
			return 0, 0, 0, err
		}
		bosskills = res.Data
	} else {
		bs, err := cli.ListAllLatestBossKills(ctx, q, opts.Concurrency)
		if err != nil {
			return 0, 0, 0, err
		}
		bosskills = bs
	}
	if len(bosskills) == 0 {
		return 0, 0, 0, nil
	}
	log.Info("fetched kills", "count", len(bosskills))

	ids := make([]string, len(bosskills))
	for i, bk := range bosskills {
		ids[i] = bk.ID
	}
	existing, err := existingRemoteIDs(ctx, db, opts.Realm, ids)
	if err != nil {
		return 0, 0, 0, err
	}

	rows := make([]row, 0, opts.BatchSize)
	flush := func() error {
		if len(rows) == 0 {
			return nil
		}
		if err := insertBossKills(ctx, db, rows); err != nil {
			return err
		}
		inserted += len(rows)
		rows = rows[:0]
		return nil
	}

	for _, bk := range bosskills {
		if existing[bk.ID] {
			skipped++
			continue
		}
		detail, derr := cli.GetBossKillDetail(ctx, opts.Realm, bk.ID)
		if derr != nil {
			log.Warn("detail fetch failed", "id", bk.ID, "err", derr)
			failed++
			continue
		}
		// Discard non-LFR kills with no loot. Upstream sometimes records two
		// rows for the same fight on bosses like Kor'kron Dark Shaman — the
		// duplicate has no loot. Loot-bearing modes (everything except LFR=7)
		// should always produce at least one loot row in a legitimate kill.
		if bk.Mode != 7 && (detail == nil || len(detail.Loot) == 0) {
			log.Info("skip no-loot non-LFR kill", "id", bk.ID, "boss", boss.Name, "mode", bk.Mode)
			skipped++
			continue
		}
		r, rerr := buildRow(opts.Realm, raidName, bk, detail)
		if rerr != nil {
			log.Warn("build row failed", "id", bk.ID, "err", rerr)
			failed++
			continue
		}
		// Use the boss name from the upstream raid roster — we already applied
		// the sveltekit rename rules to it in GetRaids.
		r.BossName = boss.Name
		rows = append(rows, r)
		if len(rows) >= opts.BatchSize {
			if err := flush(); err != nil {
				return inserted, skipped, failed, err
			}
		}
	}
	if err := flush(); err != nil {
		return inserted, skipped, failed, err
	}
	return inserted, skipped, failed, nil
}

// insertBossKills batches one INSERT covering len(rows) tuples.
//
// The Nested columns are passed as parallel slices in the column list — this
// is the wire format ClickHouse expects.
func insertBossKills(ctx context.Context, db *sql.DB, rows []row) error {
	if len(rows) == 0 {
		return nil
	}
	const insertSQL = `INSERT INTO boss_kill (
		remote_id, realm, raid_name, boss_remote_id, boss_name, mode, guild, kill_time,
		length, wipes, deaths, ress_used,
		players.guid, players.talent_spec, players.avg_item_lvl,
		players.dmg_done, players.healing_done, players.overhealing_done, players.absorb_done,
		players.dmg_taken, players.dmg_absorbed, players.healing_taken,
		players.dispels, players.interrupts,
		players.name, players.race, players.class, players.gender, players.level,
		deaths_detail.remote_id, deaths_detail.guid, deaths_detail.time,
		loot.remote_id, loot.item_id, loot.count,
		timeline.time, timeline.encounter_damage, timeline.encounter_heal,
		timeline.raid_damage, timeline.raid_heal
	)`

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare: %w", err)
	}
	for _, r := range rows {
		_, err := stmt.ExecContext(ctx,
			r.RemoteID, r.Realm, r.RaidName, r.BossRemoteID, r.BossName, r.Mode, r.Guild, r.KillTime,
			r.Length, r.Wipes, r.Deaths, r.RessUsed,
			r.PlayersGUID, r.PlayersTalentSpec, r.PlayersAvgItemLvl,
			r.PlayersDmgDone, r.PlayersHealingDone, r.PlayersOverhealingDone, r.PlayersAbsorbDone,
			r.PlayersDmgTaken, r.PlayersDmgAbsorbed, r.PlayersHealingTaken,
			r.PlayersDispels, r.PlayersInterrupts,
			r.PlayersName, r.PlayersRace, r.PlayersClass, r.PlayersGender, r.PlayersLevel,
			r.DeathsRemoteID, r.DeathsGUID, r.DeathsTime,
			r.LootRemoteID, r.LootItemID, r.LootCount,
			r.TimelineTime, r.TimelineEncounterDamage, r.TimelineEncounterHeal,
			r.TimelineRaidDamage, r.TimelineRaidHeal,
		)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("append batch row %s: %w", r.RemoteID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ErrNoData signals a realm had no kills in the requested window. Useful for
// CLI exit code semantics.
var ErrNoData = errors.New("no data")
