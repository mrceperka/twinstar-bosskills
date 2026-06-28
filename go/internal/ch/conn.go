// Package ch wraps the ClickHouse database/sql driver with sensible pool
// defaults and a couple of helpers used by the migration runner and the
// sync job.
package ch

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

type Options struct {
	// DSN in clickhouse-go v2 form, e.g.
	// "clickhouse://default:@127.0.0.1:9000/bosskills?dial_timeout=10s".
	DSN string
	// MaxOpen caps the connection pool. ClickHouse prefers a few long-lived
	// connections — keep this small.
	MaxOpen int
	// ConnLifetime recycles idle conns. ClickHouse server kills long idles
	// by default; 1h is safe.
	ConnLifetime time.Duration
}

func Open(opts Options) (*sql.DB, error) {
	if opts.DSN == "" {
		return nil, fmt.Errorf("ch.Open: empty DSN")
	}
	db, err := sql.Open("clickhouse", opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("ch.Open: %w", err)
	}
	if opts.MaxOpen <= 0 {
		opts.MaxOpen = 8
	}
	if opts.ConnLifetime <= 0 {
		opts.ConnLifetime = time.Hour
	}
	db.SetMaxOpenConns(opts.MaxOpen)
	db.SetMaxIdleConns(opts.MaxOpen)
	db.SetConnMaxLifetime(opts.ConnLifetime)
	return db, nil
}

// Ping verifies connectivity. ClickHouse's database/sql Ping issues a SELECT 1
// internally.
func Ping(ctx context.Context, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}
