package main

import (
	"strings"
	"testing"
)

func TestSourcePurgeExplicitDryRun(t *testing.T) {
	for _, args := range [][]string{nil, {"id"}, {"id", "--dry-run"}, {"id", "--dry-run=1"}, {"id", "--dry-run="}, {"id", "--dry-run=true", "extra"}, {"--dry-run=true"}} {
		if _, _, err := parseSourcePurgeArgs(args); err == nil {
			t.Errorf("accepted missing/implicit/invalid dry-run: %q", args)
		}
	}
	for _, dry := range []string{"true", "false"} {
		id, got, err := parseSourcePurgeArgs([]string{"migration-id", "--dry-run=" + dry})
		if err != nil || id != "migration-id" || got != (dry == "true") {
			t.Errorf("explicit dry-run rejected: %s %s %t %v", dry, id, got, err)
		}
	}
}

func TestSourcePurgeRequiresMaintenanceDSN(t *testing.T) {
	t.Setenv("MIGRATION_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "postgres://application-role/not-used")
	if err := runStorage([]string{"migration", "purge-source", "migration-id", "--dry-run=true"}); err == nil || !strings.Contains(err.Error(), "MIGRATION_DATABASE_URL") {
		t.Fatalf("application DSN fallback permitted: %v", err)
	}
}
