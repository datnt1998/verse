package main

import (
	"strings"
	"testing"
)

func TestStorageGCRequiresMaintenanceDSN(t *testing.T) {
	t.Setenv("STORE_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "postgres://noverse_app:secret@127.0.0.1:1/noverse?sslmode=disable")
	t.Setenv("MIGRATION_DATABASE_URL", "")
	t.Setenv("STORAGE_ENV", "dev")
	err := runStorageGC([]string{"--dry-run"})
	if err == nil || !strings.Contains(err.Error(), "MIGRATION_DATABASE_URL") {
		t.Fatalf("GC must require a maintenance DSN before connecting: %v", err)
	}
}
