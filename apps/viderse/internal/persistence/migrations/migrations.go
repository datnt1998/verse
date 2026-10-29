// Package migrations embeds the *.sql goose migrations applied by
// platform/pgmigrate through the viderse_migrate role. Migrations are
// expand-only; Down sections are throwaway-only and never a production
// rollback mechanism.
package migrations

import "embed"

// FS contains the *.sql goose migrations, embedded at build time.
//
//go:embed *.sql
var FS embed.FS
