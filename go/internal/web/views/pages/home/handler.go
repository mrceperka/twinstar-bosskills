package home

import (
	"database/sql"
)

// Deps is what the handler factory needs from the server.
type Deps struct {
	DB      *sql.DB
	CSSHash string
	JSHash  string
}
