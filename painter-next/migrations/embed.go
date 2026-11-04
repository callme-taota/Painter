package migrations

import "embed"

// Files are immutable once released; each migration is verified by SHA-256.
//
//go:embed *.sql
var Files embed.FS
