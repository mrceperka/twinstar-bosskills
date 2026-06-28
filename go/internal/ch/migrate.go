package ch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Migration is a single numbered .sql file.
type Migration struct {
	Version uint32
	Name    string
	SQL     string
}

var migrationFilename = regexp.MustCompile(`^(\d+)_([^.]+)\.sql$`)

// LoadMigrations reads numbered .sql files (e.g. 001_init.sql) from fsys and
// returns them sorted by version.
func LoadMigrations(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	out := make([]Migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationFilename.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parse version in %s: %w", e.Name(), err)
		}
		body, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		out = append(out, Migration{
			Version: uint32(v),
			Name:    m[2],
			SQL:     string(body),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// EnsureMigrationTable creates the bookkeeping table if missing.
func EnsureMigrationTable(ctx context.Context, db *sql.DB) error {
	const ddl = `CREATE TABLE IF NOT EXISTS _migrations (
		version UInt32,
		name    String,
		applied_at DateTime DEFAULT now()
	) ENGINE = MergeTree ORDER BY version`
	_, err := db.ExecContext(ctx, ddl)
	return err
}

// AppliedVersions returns the set of versions already applied.
func AppliedVersions(ctx context.Context, db *sql.DB) (map[uint32]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM _migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uint32]bool{}
	for rows.Next() {
		var v uint32
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

// Apply runs every pending migration. Each statement inside a migration runs
// as its own ExecContext — ClickHouse-go does not support multi-statement
// requests over the native protocol.
func Apply(ctx context.Context, db *sql.DB, migrations []Migration) ([]Migration, error) {
	if err := EnsureMigrationTable(ctx, db); err != nil {
		return nil, fmt.Errorf("ensure _migrations: %w", err)
	}
	applied, err := AppliedVersions(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("read applied: %w", err)
	}
	var ran []Migration
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		stmts := SplitStatements(m.SQL)
		if len(stmts) == 0 {
			return ran, fmt.Errorf("migration %03d_%s: no statements", m.Version, m.Name)
		}
		for i, stmt := range stmts {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return ran, fmt.Errorf("migration %03d_%s stmt %d: %w\n--- SQL ---\n%s",
					m.Version, m.Name, i+1, err, snippet(stmt))
			}
		}
		if _, err := db.ExecContext(ctx,
			"INSERT INTO _migrations (version, name) VALUES (?, ?)",
			m.Version, m.Name); err != nil {
			return ran, fmt.Errorf("record migration %03d: %w", m.Version, err)
		}
		ran = append(ran, m)
	}
	return ran, nil
}

func snippet(s string) string {
	const max = 400
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max] + " …"
	}
	return s
}

// ErrNoMigrations is returned when the migrations directory is empty.
var ErrNoMigrations = errors.New("no migrations found")
