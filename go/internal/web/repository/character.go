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

// CharacterByName resolves the latest identity and exact kill count from the
// derived ClickHouse character aggregate.
func CharacterByName(ctx context.Context, db *sql.DB, realmName, name string) (Character, error) {
	const q = `
		SELECT guid,
		       argMaxMerge(class_state) AS class,
		       minMerge(first_seen_state) AS first_seen,
		       maxMerge(last_seen_state) AS last_seen,
		       uniqExactMerge(kill_count_state) AS kills
		FROM character
		WHERE realm = ?
		GROUP BY realm, guid
		HAVING argMaxMerge(name_state) = ?
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
