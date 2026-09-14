// Package ch wraps the ClickHouse database/sql driver with sensible pool
// defaults and a couple of helpers used by the migration runner and the
// sync job.
package ch

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

type Options struct {
	// DSN in clickhouse-go v2 form, e.g.
	// "clickhouse://default:@127.0.0.1:9000/bosskills?dial_timeout=10s".
	DSN string
	// MaxOpen caps the connection pool. ClickHouse prefers a few long-lived
	// connections - keep this small.
	MaxOpen int
	// ConnLifetime recycles idle conns. ClickHouse server kills long idles
	// by default; 1h is safe.
	ConnLifetime time.Duration
	// QueryCache turns on ClickHouse's server-side result cache for every
	// query on this pool. Only the read-only web server should set it - the
	// sync job's existing-kill preflight SELECT must see its own writes.
	QueryCache bool
}

// queryCacheDSN returns dsn with the query cache settings applied. Long TTL is
// safe because the data only changes when cmd/sync runs, and sync flushes the
// cache when it finishes.
func queryCacheDSN(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("use_query_cache", "1")
	q.Set("query_cache_ttl", "3600")
	// Cheap queries would just evict the expensive aggregations we care about.
	q.Set("query_cache_min_query_duration", "100")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func Open(opts Options) (*sql.DB, error) {
	if opts.DSN == "" {
		return nil, fmt.Errorf("ch.Open: empty DSN")
	}
	dsn := opts.DSN
	if opts.QueryCache {
		var err error
		dsn, err = queryCacheDSN(dsn)
		if err != nil {
			return nil, fmt.Errorf("ch.Open: query cache dsn: %w", err)
		}
	}
	db, err := sql.Open("clickhouse", dsn)
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
