package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/api"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
)

const (
	korKronDarkShamanEntry = 71859
)

type darkShamanLFRSignature struct {
	Guild    string
	KillTime string
	Length   uint32
	Wipes    uint32
	Deaths   uint32
	RessUsed uint32
}

func filterDuplicateDarkShamanLFRBossKills(bosskills []api.BossKill, existing map[darkShamanLFRSignature]bool) ([]api.BossKill, int) {
	if len(bosskills) == 0 {
		return bosskills, 0
	}
	out := make([]api.BossKill, 0, len(bosskills))
	seen := make(map[darkShamanLFRSignature]int)
	skipped := 0
	for _, bk := range bosskills {
		sig, ok := makeDarkShamanLFRSignature(bk)
		if !ok {
			out = append(out, bk)
			continue
		}
		if existing[sig] {
			skipped++
			continue
		}
		if idx, ok := seen[sig]; ok {
			if bossKillIDLess(bk.ID, out[idx].ID) {
				out[idx] = bk
			}
			skipped++
			continue
		}
		seen[sig] = len(out)
		out = append(out, bk)
	}
	return out, skipped
}

func existingDarkShamanLFRSignatures(ctx context.Context, db *sql.DB, realmName string, bosskills []api.BossKill) (map[darkShamanLFRSignature]bool, error) {
	out := map[darkShamanLFRSignature]bool{}
	times := make([]time.Time, 0, len(bosskills))
	seenTimes := map[string]bool{}
	for _, bk := range bosskills {
		if bk.Entry != korKronDarkShamanEntry || bk.Mode != wow.DifficultyMoPLFR {
			continue
		}
		t, err := parseAPITime(bk.Time)
		if err != nil {
			continue
		}
		key := canonicalKillTime(t)
		if seenTimes[key] {
			continue
		}
		seenTimes[key] = true
		times = append(times, t)
	}
	if len(times) == 0 {
		return out, nil
	}

	args := make([]any, 0, len(times)+2)
	args = append(args, realmName, wow.DifficultyMoPLFR)
	placeholders := make([]byte, 0, len(times)*3)
	for i, t := range times {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, t)
	}
	q := `SELECT guild, kill_time, length, wipes, deaths, ress_used
		FROM boss_kill
		WHERE realm = ? AND boss_remote_id = 71859 AND mode = ?
		  AND kill_time IN (` + string(placeholders) + `)`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query existing Kor'kron Dark Shaman LFR signatures: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sig darkShamanLFRSignature
		var killTime time.Time
		if err := rows.Scan(&sig.Guild, &killTime, &sig.Length, &sig.Wipes, &sig.Deaths, &sig.RessUsed); err != nil {
			return nil, err
		}
		sig.KillTime = canonicalKillTime(killTime)
		out[sig] = true
	}
	return out, rows.Err()
}

func makeDarkShamanLFRSignature(bk api.BossKill) (darkShamanLFRSignature, bool) {
	if bk.Entry != korKronDarkShamanEntry || bk.Mode != wow.DifficultyMoPLFR {
		return darkShamanLFRSignature{}, false
	}
	t, err := parseAPITime(bk.Time)
	killTime := bk.Time
	if err == nil {
		killTime = canonicalKillTime(t)
	}
	return darkShamanLFRSignature{
		Guild:    bk.Guild,
		KillTime: killTime,
		Length:   uint32(bk.Length),
		Wipes:    uint32(bk.Wipes),
		Deaths:   uint32(bk.Deaths),
		RessUsed: uint32(bk.RessUsed),
	}, true
}

func canonicalKillTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func bossKillIDLess(a, b string) bool {
	ap, an, aok := splitBossKillID(a)
	bp, bn, bok := splitBossKillID(b)
	if aok && bok && ap == bp {
		return an < bn
	}
	return a < b
}

func splitBossKillID(id string) (string, uint64, bool) {
	idx := strings.LastIndexByte(id, '_')
	if idx < 0 || idx == len(id)-1 {
		return "", 0, false
	}
	n, err := strconv.ParseUint(id[idx+1:], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return id[:idx], n, true
}
