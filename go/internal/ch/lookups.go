package ch

import (
	"context"
	"database/sql"
	"strings"
)

// Placeholders returns "?,?,..." with n entries for IN-list bind parameters.
// Returns empty string for n <= 0.
func Placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

// BossNames returns boss_remote_id → boss_name for the requested IDs in one
// realm. Uses any(boss_name) so callers always get the freshest synced name.
// Reads from boss_kill rather than the boss lookup table to avoid drift
// between roster and event rows.
func BossNames(ctx context.Context, db *sql.DB, realmName string, ids []uint32) (map[uint32]string, error) {
	out := map[uint32]string{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, realmName)
	for _, id := range ids {
		args = append(args, id)
	}
	q := "SELECT boss_remote_id, any(boss_name) FROM boss_kill " +
		"WHERE realm = ? AND boss_remote_id IN (" + Placeholders(len(ids)) + ") " +
		"GROUP BY boss_remote_id"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uint32
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// CharacterNames returns guid → most-recent character name for the requested
// guids in one realm, via the `character` MV.
func CharacterNames(ctx context.Context, db *sql.DB, realmName string, guids []uint64) (map[uint64]string, error) {
	out := map[uint64]string{}
	if len(guids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(guids)+1)
	args = append(args, realmName)
	for _, g := range guids {
		args = append(args, g)
	}
	q := "SELECT guid, argMaxMerge(name_state) FROM character " +
		"WHERE realm = ? AND guid IN (" + Placeholders(len(guids)) + ") " +
		"GROUP BY realm, guid"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var g uint64
		var name string
		if err := rows.Scan(&g, &name); err != nil {
			return nil, err
		}
		out[g] = name
	}
	return out, rows.Err()
}
