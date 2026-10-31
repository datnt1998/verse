package rules

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
)

func TestGlobalMigrationCopiesEveryProjectAndIsIdempotent(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(home, "tone.md"), "Giọng văn cũ")
	write(filepath.Join(home, ManagedRulesFile), "- Luật chung\n- Trùng\n")
	for _, project := range []string{"one", "two"} {
		write(filepath.Join(root, project, library.ProjectFileName), `{"name":"Dự án"}`)
	}
	write(filepath.Join(root, "one", ".ainovel", "rules", ManagedRulesFile), "- Trùng\n- Luật riêng\n")
	write(filepath.Join(root, "one", ".ainovel", "rules", "tone.md"), "Giọng dự án")
	accounts := identity.NewStore(filepath.Join(t.TempDir(), "accounts.json"), library.IdentityOptions())
	if err := MigrateGlobalFiles(root, home, accounts); err != nil {
		t.Fatal(err)
	}
	for _, project := range []string{"one", "two"} {
		data, err := os.ReadFile(filepath.Join(root, project, ".ainovel", "rules", ManagedRulesFile))
		if err != nil || strings.Count(string(data), "Luật chung") != 1 || strings.Count(string(data), "Trùng") != 1 {
			t.Fatalf("project %s: %q %v", project, data, err)
		}
	}
	data, _ := os.ReadFile(filepath.Join(root, "one", ".ainovel", "rules", "tone.md"))
	if string(data) != "Giọng dự án" {
		t.Fatalf("overwrote project rule: %q", data)
	}
	data, _ = os.ReadFile(filepath.Join(root, "two", ".ainovel", "rules", "tone.md"))
	if string(data) != "Giọng văn cũ" {
		t.Fatalf("missing global copy: %q", data)
	}
	write(filepath.Join(home, "new.md"), "Must not be imported after done")
	if err := MigrateGlobalFiles(root, home, accounts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "two", ".ainovel", "rules", "new.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("done marker ignored: %v", err)
	}
	page, err := accounts.Audit(identity.AuditQuery{})
	if err != nil || len(page.Items) != 1 || page.Items[0].Action != GlobalMigrationAction {
		t.Fatalf("audit: %+v %v", page, err)
	}
	// Retry after a crash between audit and marker must not duplicate either.
	if err := os.Remove(filepath.Join(root, ".ainovel", GlobalMigrationKey+".done")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(home, "new.md")); err != nil {
		t.Fatal(err)
	}
	if err := MigrateGlobalFiles(root, home, accounts); err != nil {
		t.Fatal(err)
	}
	page, _ = accounts.Audit(identity.AuditQuery{})
	if len(page.Items) != 1 {
		t.Fatalf("duplicated migration audit: %+v", page)
	}
}

func TestGlobalMigrationStopsOnUnreadableSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "file-not-directory")
	if err := os.WriteFile(source, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MigrateGlobalFiles(root, source, nil); err == nil {
		t.Fatal("migration silently ignored unreadable source")
	}
	if _, err := os.Stat(filepath.Join(root, ".ainovel", GlobalMigrationKey+".done")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marked failed migration done: %v", err)
	}
}
