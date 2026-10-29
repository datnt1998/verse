package library

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestLibraryLayoutContract pins the on-disk library layout the file backend
// must keep honoring while the Postgres backend is added:
//   - <root>/.ainovel/library.json with dirs overrides (a hardcoded "kich_ban"
//     or "output" anywhere must fail this test),
//   - the anonymous legacy project "_" aggregating root-level output/ dirs,
//   - candidate dirs (kich_ban/ without project.json) adoptable via AdoptProject,
//   - renamed project roots where the directory slug stays the identity,
//   - rules symlinks: confined links are flagged rules-symlink (fixable),
//     links escaping the project are not.

func copyContractTree(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("..", "store", "testdata", "storage-contract", name)
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(to, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture %s: %v", name, err)
	}
	return dst
}

func TestLibraryLayoutContract(t *testing.T) {
	t.Run("default_layout_classification", func(t *testing.T) {
		root := copyContractTree(t, "library")
		lib, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		scan, err := lib.Scan()
		if err != nil {
			t.Fatal(err)
		}
		if len(scan.Projects) != 2 || scan.Projects[0].ID != "du-an" || scan.Projects[1].ID != LegacyProjectID || !scan.Projects[1].Legacy {
			t.Fatalf("project + anonymous legacy must classify: %+v", scan.Projects)
		}
		if len(scan.Candidates) != 1 || scan.Candidates[0] != "cu-dir" {
			t.Fatalf("candidate (kich_ban without project.json): %v", scan.Candidates)
		}
		if len(scan.Legacy) != 1 || scan.Legacy[0] != "truyen-cu" {
			t.Fatalf("legacy root story dirs: %v", scan.Legacy)
		}
		// stories resolve under both normal and anonymous projects
		st, ok := lib.ResolveStory("du-an", "01-dem-trang")
		if !ok || st.ID != "du-an/01-dem-trang" || st.Dir != filepath.Join(root, "du-an", "kich_ban", "01-dem-trang") {
			t.Fatalf("story resolve: %+v ok=%v", st, ok)
		}
		st, ok = lib.ResolveStory(LegacyProjectID, "truyen-cu")
		if !ok || st.ProjectDir != root {
			t.Fatalf("anonymous project story must resolve at root: %+v ok=%v", st, ok)
		}
		if _, ok := lib.ResolveProject("cu-dir"); !ok {
			t.Fatal("candidate dir resolves structurally but is not a project until adopted")
		}
	})

	t.Run("dirs_overrides_honored", func(t *testing.T) {
		root := copyContractTree(t, "library-custom")
		lib, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		if lib.Config.Dirs.Stories != "truyen" {
			t.Fatalf("dirs.stories override must load: %+v", lib.Config.Dirs)
		}
		// a hardcoded kich_ban lookup must fail here: story lives under truyen/
		st, ok := lib.ResolveStory("du-an", "01-mua-he")
		if !ok || st.Dir != filepath.Join(root, "du-an", "truyen", "01-mua-he") {
			t.Fatalf("custom stories dir must resolve: %+v ok=%v", st, ok)
		}
		stories, err := lib.Stories("du-an")
		if err != nil || len(stories) != 1 || stories[0].Name != "01-mua-he" {
			t.Fatalf("Stories must list under dirs.stories: %v %+v", err, stories)
		}
		scan, err := lib.Scan()
		if err != nil {
			t.Fatal(err)
		}
		if len(scan.Candidates) != 0 {
			t.Fatalf("kich_ban-looking dir under custom config must not be a candidate: %v", scan.Candidates)
		}
	})

	t.Run("renamed_root_keeps_slug_identity", func(t *testing.T) {
		root := copyContractTree(t, "library-renamed")
		lib, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		scan, err := lib.Scan()
		if err != nil {
			t.Fatal(err)
		}
		if len(scan.Projects) != 1 || scan.Projects[0].ID != "du-an-cu" {
			t.Fatalf("directory slug is the identity, not the display name: %+v", scan.Projects)
		}
		pf, err := LoadProjectFile(filepath.Join(root, "du-an-cu"))
		if err != nil || pf.Name != "Dự án Đã Đổi Tên" {
			t.Fatalf("renamed project keeps directory id: %v %+v", err, pf)
		}
		sf, err := LoadStoryFile(filepath.Join(root, "du-an-cu", "kich_ban", "01-ten-moi"))
		if err != nil || sf == nil || !sf.TitleRenamed || sf.Title != "Tên Mới Của Truyện" {
			t.Fatalf("titleRenamed flag must round-trip: %v %+v", err, sf)
		}
	})

	t.Run("adopted_candidate_becomes_project", func(t *testing.T) {
		root := copyContractTree(t, "library")
		lib, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		dir, err := lib.AdoptProject("cu-dir", "Cũ Đã Nhận")
		if err != nil {
			t.Fatal(err)
		}
		if dir != filepath.Join(root, "cu-dir") {
			t.Fatalf("adopt returns project dir: %s", dir)
		}
		scan, err := lib.Scan()
		if err != nil {
			t.Fatal(err)
		}
		if len(scan.Candidates) != 0 {
			t.Fatalf("adopted dir leaves candidate list: %v", scan.Candidates)
		}
		found := false
		for _, p := range scan.Projects {
			if p.ID == "cu-dir" {
				found = true
			}
		}
		if !found {
			t.Fatalf("adopted dir must scan as project: %+v", scan.Projects)
		}
		st, ok := lib.ResolveStory("cu-dir", "02-tiem-nang")
		if !ok || st.Dir != filepath.Join(root, "cu-dir", "kich_ban", "02-tiem-nang") {
			t.Fatalf("adopted project story must resolve: %+v ok=%v", st, ok)
		}
	})

	t.Run("source_symlink_confined_vs_escape", func(t *testing.T) {
		root := copyContractTree(t, "library")
		pdir := filepath.Join(root, "du-an")
		storyRules := filepath.Join(pdir, "kich_ban", "01-dem-trang", ".ainovel", "rules")
		if err := os.MkdirAll(storyRules, 0o755); err != nil {
			t.Fatal(err)
		}
		// confined: symlink into the project-level shared rules
		confined := filepath.Join(storyRules, "confined-link.md")
		if err := os.Symlink(filepath.Join("..", "..", "..", "..", ".ainovel", "rules", "giong-ke.md"), confined); err != nil {
			t.Fatal(err)
		}
		// escape: symlink resolving outside the project
		outside := filepath.Join(root, "outside.txt")
		if err := os.WriteFile(outside, []byte("ngoài dự án\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		escape := filepath.Join(storyRules, "escape-link.md")
		if err := os.Symlink(filepath.Join("..", "..", "..", "..", "..", "outside.txt"), escape); err != nil {
			t.Fatal(err)
		}
		lib, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		v, err := lib.Validate("du-an")
		if err != nil {
			t.Fatal(err)
		}
		var confinedIssue, escapeIssue *ValidationIssue
		for i := range v.Issues {
			switch v.Issues[i].Path {
			case "kich_ban/01-dem-trang/.ainovel/rules/confined-link.md":
				confinedIssue = &v.Issues[i]
			case "kich_ban/01-dem-trang/.ainovel/rules/escape-link.md":
				escapeIssue = &v.Issues[i]
			}
		}
		if confinedIssue == nil || confinedIssue.Code != issueRulesSymlink || !confinedIssue.Fixable {
			t.Fatalf("confined rules symlink must be flagged fixable: %+v", confinedIssue)
		}
		if escapeIssue != nil && escapeIssue.Code == issueRulesSymlink {
			t.Fatalf("escaping symlink must not be treated as shared-rules link: %+v", escapeIssue)
		}
	})

	t.Run("project_json_absent_vs_corrupt", func(t *testing.T) {
		root := copyContractTree(t, "library")
		// absent project.json → the dir is not a project (ErrNotFound for write)
		pf, err := LoadProjectFile(filepath.Join(root, "cu-dir"))
		if err != nil || pf != nil {
			t.Fatalf("absent project.json must be (nil,nil): %v %+v", err, pf)
		}
		bad := filepath.Join(root, "hong")
		if err := os.MkdirAll(bad, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bad, "project.json"), []byte(`{"name":`), 0o644); err != nil {
			t.Fatal(err)
		}
		if pf, err := LoadProjectFile(bad); err == nil || errors.Is(err, os.ErrNotExist) {
			t.Fatalf("corrupt project.json must error, not look absent: %v %+v", err, pf)
		}
	})
}
