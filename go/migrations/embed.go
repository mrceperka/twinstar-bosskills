// Package migrations embeds the .sql files so the migrate binary is fully
// self-contained.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
