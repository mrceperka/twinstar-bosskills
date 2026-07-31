// Package repository contains shared read models used by multiple web pages.
package repository

import (
	"context"
	"database/sql"
	"time"
)

type Character struct {
	GUID      uint64
	Class     int
	FirstSeen time.Time
	LastSeen  time.Time
	KillCount int
}

// CharacterByName resolves the latest identity and exact kill count via
// character_name_index, a point lookup refreshed from the character
// aggregate at the end of every sync run (cmd/sync's
// refreshCharacterNameIndex). Avoids grouping the whole realm's character
// table per request just to filter by name in HAVING - that used to take
// ~1.2s/189MB per call and barely benefited from the query cache, since each
// distinct name is a distinct cache key.
func CharacterByName(ctx context.Context, db *sql.DB, realmName, name string) (Character, error) {
	const q = `
		SELECT guid, class, first_seen, last_seen, kills
		FROM character_name_index FINAL
		WHERE realm = ? AND name = ?
		ORDER BY kills DESC
		LIMIT 1
	`
	var out Character
	var class uint8
	var kills uint64
	err := db.QueryRowContext(ctx, q, realmName, name).Scan(&out.GUID, &class, &out.FirstSeen, &out.LastSeen, &kills)
	if err == sql.ErrNoRows {
		return Character{}, nil
	}
	if err != nil {
		return Character{}, err
	}
	out.Class = int(class)
	out.KillCount = int(kills)
	return out, nil
}
