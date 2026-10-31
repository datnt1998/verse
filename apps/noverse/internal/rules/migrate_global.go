package rules

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/gofrs/flock"
)

const GlobalMigrationKey = "rules.project-only.v1"
const GlobalMigrationAction = "Chuyển quy tắc toàn cục vào từng dự án"

type SystemAuditor interface {
	RecordSystemAudit(action, target, reason string) error
}

// LegacyGlobalFiles reads the retired global rule set strictly: an unreadable
// source must stop the upgrade rather than silently lose a user's rules.
func LegacyGlobalFiles(dir string) (map[string]string, error) {
	out := map[string]string{}
	if dir == "" {
		return out, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || !strings.EqualFold(filepath.Ext(name), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		out[name] = string(data)
	}
	return out, nil
}

// MergeLegacyGlobal preserves the former nearer-wins behavior. The managed
// list is additive, so missing global lines precede the existing project list.
// Other same-name project files already override global and remain untouched.
func MergeLegacyGlobal(name, global, current string, exists bool) string {
	if !exists {
		return global
	}
	if name != ManagedRulesFile {
		return current
	}
	seen := map[string]bool{}
	for _, line := range ParseRuleLines(current) {
		seen[line] = true
	}
	var prefix strings.Builder
	for _, line := range ParseRuleLines(global) {
		if !seen[line] {
			prefix.WriteString("- " + line + "\n")
			seen[line] = true
		}
	}
	return prefix.String() + current
}

// MigrateGlobalFiles runs before serving a file library. A root-local lock and
// done marker make retries safe, including a crash between the audit and marker.
func MigrateGlobalFiles(root, homeRules string, audit SystemAuditor) error {
	if root == "" {
		return nil // no library selected yet
	}
	dir := filepath.Join(root, ".ainovel")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(dir, GlobalMigrationKey+".lock"))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	marker := filepath.Join(dir, GlobalMigrationKey+".done")
	if _, err := os.Stat(marker); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	files, err := LegacyGlobalFiles(homeRules)
	if err != nil {
		return err
	}
	lib, err := library.Open(root)
	if err != nil {
		return err
	}
	scan, err := lib.Scan()
	if err != nil {
		return err
	}
	for _, project := range scan.Projects {
		targets := []string{project.Dir}
		if project.Legacy {
			stories, err := lib.Stories(project.ID)
			if err != nil {
				return err
			}
			targets = nil
			for _, story := range stories {
				targets = append(targets, story.Dir)
			}
		}
		for _, target := range targets {
			for name, text := range files {
				path := filepath.Join(DefaultProjectRulesDir(target), name)
				current, err := os.ReadFile(path)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				merged := MergeLegacyGlobal(name, text, string(current), err == nil)
				if err == nil && merged == string(current) {
					continue
				}
				if err := writeMigrationFile(path, []byte(merged)); err != nil {
					return err
				}
			}
		}
	}
	if audit == nil {
		return errors.New("thiếu kho nhật ký quản trị khi chuyển quy tắc")
	}
	if err := audit.RecordSystemAudit(GlobalMigrationAction, GlobalMigrationKey+":"+root, fmt.Sprintf("Đã chuyển %d tệp quy tắc vào %d dự án; giữ quy tắc riêng của dự án.", len(files), len(scan.Projects))); err != nil {
		return err
	}
	return writeMigrationFile(marker, []byte("done\n"))
}

func writeMigrationFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".rules-migration-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
