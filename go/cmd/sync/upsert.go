package main

import (
	"context"
	"database/sql"
	"fmt"

	"twinstar-bosskills/internal/api"
	"twinstar-bosskills/internal/wow"
)

// upsertRaidsAndBosses writes raid + boss lookup rows for the given realm.
// ClickHouse's ReplacingMergeTree handles dedup on background merge; we just
// INSERT and let CH pick the highest `version` on read.
func upsertRaidsAndBosses(ctx context.Context, db *sql.DB, realmName string, raids []api.Raid) error {
	if len(raids) == 0 {
		return nil
	}

	// raids
	{
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin raid: %w", err)
		}
		stmt, err := tx.PrepareContext(ctx, "INSERT INTO raid (realm, name, position)")
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("prepare raid: %w", err)
		}
		for i, r := range raids {
			position := wow.RaidPosition(r.Map)
			if position == 0 {
				position = i + 1
			}
			if _, err := stmt.ExecContext(ctx, realmName, r.Map, uint16(position)); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("insert raid %s: %w", r.Map, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit raid: %w", err)
		}
	}

	// bosses
	{
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin boss: %w", err)
		}
		stmt, err := tx.PrepareContext(ctx, "INSERT INTO boss (realm, raid_name, remote_id, name, position)")
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("prepare boss: %w", err)
		}
		for _, raid := range raids {
			for i, b := range raid.Bosses {
				remoteID := uint32(b.Entry)
				position := wow.BossPosition(remoteID)
				if position == 0 {
					position = i + 1
				}
				if _, err := stmt.ExecContext(ctx, realmName, raid.Map, remoteID, b.Name, uint16(position)); err != nil {
					_ = tx.Rollback()
					return fmt.Errorf("insert boss %s/%d: %w", b.Name, b.Entry, err)
				}
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit boss: %w", err)
		}
	}
	return nil
}

// existingRemoteIDs returns the set of boss_kill.remote_id values already
// present in CH for the given realm + candidate list. Used to avoid feeding
// duplicate rows to the materialized views.
func existingRemoteIDs(ctx context.Context, db *sql.DB, realmName string, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	// Build placeholders.
	args := make([]any, 0, len(ids)+1)
	args = append(args, realmName)
	placeholders := make([]byte, 0, len(ids)*3)
	for i, id := range ids {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, id)
	}
	q := "SELECT remote_id FROM boss_kill WHERE realm = ? AND remote_id IN (" + string(placeholders) + ")"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query existing remote_ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
