package postgres_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/pgtest"
)

// libraryFixture is a migrated schema with a PG-backed accounts store and a
// project created through the repository itself (admin actor), plus actors for
// owner/collab/viewer/outsider — the same matrix the handlers see.
type libraryFixture struct {
	repo     *postgres.LibraryRepository
	pool     *postgres.Pool
	owner    *library.Actor
	collab   *library.Actor
	viewer   *library.Actor
	outsider *library.Actor
	admin    *library.Actor
}

const libTestPassword = "mat-khau-12345"

func provisionLibrary(t *testing.T, projectSlug string) libraryFixture {
	t.Helper()
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	ctx := context.Background()
	repo := postgres.NewLibraryRepository(pool)
	accounts := identity.NewFromRepository(identityRepo(pool), library.IdentityOptions())

	for _, u := range []struct {
		name string
		role identity.Role
	}{
		{"lib-admin", identity.RoleAdmin},
		{"lib-owner", identity.RoleMember},
		{"lib-collab", identity.RoleMember},
		{"lib-viewer", identity.RoleMember},
		{"lib-outsider", identity.RoleMember},
	} {
		if _, err := accounts.CreateUser(identity.NewUser{
			Username: u.name, Name: u.name, Role: u.role, Password: libTestPassword,
		}); err != nil {
			t.Fatalf("create %s: %v", u.name, err)
		}
	}

	admin := &library.Actor{Username: "lib-admin", GlobalRole: string(identity.RoleAdmin)}
	// The fixture inserts the project row directly so the owner row can belong
	// to lib-owner; CreateProject's own owner upsert is asserted separately.
	q := gen.New(pool.Pool())
	project, err := q.InsertProject(ctx, gen.InsertProjectParams{
		ID: deterministicUUID("project", projectSlug), Slug: projectSlug,
		Name: "Dự án thử", Description: "mô tả", Notes: "ghi chú", UniverseName: "Vũ trụ thử",
	})
	if err != nil {
		t.Fatalf("anchor project: %v", err)
	}
	for _, m := range []struct{ name, role string }{
		{"lib-owner", "owner"}, {"lib-collab", "collab"}, {"lib-viewer", "view"},
	} {
		usr, err := q.GetUserByUsername(ctx, m.name)
		if err != nil {
			t.Fatalf("read %s: %v", m.name, err)
		}
		if err := q.UpsertMember(ctx, gen.UpsertMemberParams{ProjectID: project.ID, UserID: usr.ID, Role: m.role}); err != nil {
			t.Fatalf("member %s: %v", m.name, err)
		}
	}
	return libraryFixture{
		repo:     repo,
		pool:     pool,
		owner:    &library.Actor{Username: "lib-owner"},
		collab:   &library.Actor{Username: "lib-collab"},
		viewer:   &library.Actor{Username: "lib-viewer"},
		outsider: &library.Actor{Username: "lib-outsider"},
		admin:    admin,
	}
}

// TestLibraryRepositoryProjectScope proves the actor scope on the hosted
// library: members read, only owner|collab write, non-members and non-existent
// projects are indistinguishable (not-found), and only admins create projects.
func TestLibraryRepositoryProjectScope(t *testing.T) {
	f := provisionLibrary(t, "du-an-a")
	ctx := context.Background()

	scan, err := f.repo.Scan(ctx, f.owner)
	if err != nil {
		t.Fatalf("owner scan: %v", err)
	}
	if len(scan.Projects) != 1 || scan.Projects[0].ID != "du-an-a" || scan.Projects[0].Dir != "" {
		t.Fatalf("owner scan must list the hosted project with no dir: %+v", scan.Projects)
	}
	if len(scan.Candidates) != 0 || len(scan.Legacy) != 0 {
		t.Fatalf("hosted scan has no candidates/legacy: %+v", scan)
	}
	if scan, err := f.repo.Scan(ctx, f.outsider); err != nil || len(scan.Projects) != 0 {
		t.Fatalf("outsider must see no projects: %+v err=%v", scan, err)
	}
	if scan, err := f.repo.Scan(ctx, f.admin); err != nil || len(scan.Projects) != 1 {
		t.Fatalf("admin must list every project: %+v err=%v", scan, err)
	}

	if ref, ok := f.repo.ResolveProject(ctx, f.owner, "du-an-a"); !ok || ref.ID != "du-an-a" || ref.Dir != "" {
		t.Fatalf("owner resolve: %+v ok=%v", ref, ok)
	}
	if _, ok := f.repo.ResolveProject(ctx, f.outsider, "du-an-a"); ok {
		t.Fatal("outsider must not resolve another tenant's project")
	}
	if _, ok := f.repo.ResolveProject(ctx, f.owner, "khong-co"); ok {
		t.Fatal("missing project must not resolve")
	}

	pf, err := f.repo.ProjectFile(ctx, f.viewer, "du-an-a")
	if err != nil {
		t.Fatalf("viewer must read project file: %v", err)
	}
	if pf.Name != "Dự án thử" || pf.UniverseName != "Vũ trụ thử" || pf.CreatedAt == nil {
		t.Fatalf("project file fields: %+v", pf)
	}
	if _, err := f.repo.ProjectFile(ctx, f.outsider, "du-an-a"); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("outsider project file must be not-found: %v", err)
	}
	if _, err := f.repo.ProjectFile(ctx, f.owner, "khong-co"); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("missing project file must be not-found: %v", err)
	}

	if err := f.repo.UpdateProject(ctx, f.viewer, "du-an-a", library.ProjectFile{Name: "hack"}); !errors.Is(err, errs.ErrStorageForbidden) {
		t.Fatalf("viewer update must be forbidden: %v", err)
	}
	if err := f.repo.UpdateProject(ctx, f.outsider, "du-an-a", library.ProjectFile{Name: "hack"}); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("outsider update must be not-found: %v", err)
	}
	if err := f.repo.UpdateProject(ctx, f.collab, "du-an-a", library.ProjectFile{
		Name: "Tên mới", Description: "mô tả mới", Notes: "ghi chú mới", UniverseName: "",
	}); err != nil {
		t.Fatalf("collab update: %v", err)
	}
	if pf, err := f.repo.ProjectFile(ctx, f.owner, "du-an-a"); err != nil || pf.Name != "Tên mới" || pf.UniverseName != "" {
		t.Fatalf("update must persist: %+v err=%v", pf, err)
	}

	if _, err := f.repo.CreateProject(ctx, f.owner, "du-an-b", library.ProjectFile{Name: "B"}); !errors.Is(err, errs.ErrStorageForbidden) {
		t.Fatalf("non-admin create must be forbidden: %v", err)
	}
	if _, err := f.repo.CreateProject(ctx, f.admin, "du-an-a", library.ProjectFile{Name: "trùng"}); !errors.Is(err, library.ErrExists) {
		t.Fatalf("duplicate slug must be ErrExists: %v", err)
	}
	if _, err := f.repo.CreateProject(ctx, f.admin, "Bad Slug", library.ProjectFile{}); !errors.Is(err, library.ErrInvalidName) {
		t.Fatalf("invalid slug must be ErrInvalidName: %v", err)
	}
	// Creating as an admin makes that admin the project's owner row (the
	// membership the repository then checks for every later operation).
	if _, err := f.repo.CreateProject(ctx, f.admin, "du-an-admin", library.ProjectFile{Name: "Admin"}); err != nil {
		t.Fatalf("admin create: %v", err)
	}
	q := gen.New(f.pool.Pool())
	created, err := q.GetProjectBySlug(ctx, "du-an-admin")
	if err != nil {
		t.Fatalf("read created project: %v", err)
	}
	adminUser, err := q.GetUserByUsername(ctx, "lib-admin")
	if err != nil {
		t.Fatalf("read admin user: %v", err)
	}
	role, err := q.GetMemberRole(ctx, gen.GetMemberRoleParams{ProjectID: created.ID, UserID: adminUser.ID})
	if err != nil || role != "owner" {
		t.Fatalf("creator must own the new project: role=%q err=%v", role, err)
	}

	// File-only capabilities stay honest on the hosted backend.
	if _, err := f.repo.Assets(ctx, f.owner, "du-an-a"); !errors.Is(err, errs.ErrStorageUnsupported) {
		t.Fatalf("assets must be unsupported: %v", err)
	}
	if _, err := f.repo.Validate(ctx, f.owner, "du-an-a"); !errors.Is(err, errs.ErrStorageUnsupported) {
		t.Fatalf("validate must be unsupported: %v", err)
	}
	if _, err := f.repo.AdoptProject(ctx, f.admin, "cand", "x"); !errors.Is(err, errs.ErrStorageUnsupported) {
		t.Fatalf("adopt must be unsupported: %v", err)
	}
}

// TestLibraryRepositoryStoryNumbering proves "NN-slug" numbering under the
// project row lock (concurrent creates never collide), the duplicate-name
// contract, rename preserving the number and soft delete hiding the story.
func TestLibraryRepositoryStoryNumbering(t *testing.T) {
	f := provisionLibrary(t, "du-an-a")
	ctx := context.Background()

	st, temp, err := f.repo.CreateStory(ctx, f.owner, "du-an-a", "Truyện kể dài", "")
	if err != nil {
		t.Fatalf("create story: %v", err)
	}
	if st.Story != "01-truyen-ke-dai" || st.ID != "du-an-a/01-truyen-ke-dai" || st.Dir != "" || temp {
		t.Fatalf("numbered story identity: %+v temp=%v", st, temp)
	}
	untitled, temp, err := f.repo.CreateStory(ctx, f.owner, "du-an-a", "", "")
	if err != nil || !temp || untitled.Story != "02-truyen-moi" {
		t.Fatalf("untitled story must be 02-truyen-moi/temp: %+v temp=%v err=%v", untitled, temp, err)
	}
	if _, _, err := f.repo.CreateStory(ctx, f.owner, "du-an-a", "Truyện kể dài", ""); !errors.Is(err, library.ErrExists) {
		t.Fatalf("duplicate name must be ErrExists: %v", err)
	}
	if _, _, err := f.repo.CreateStory(ctx, f.viewer, "du-an-a", "Của người xem", ""); !errors.Is(err, errs.ErrStorageForbidden) {
		t.Fatalf("viewer create must be forbidden: %v", err)
	}
	if _, _, err := f.repo.CreateStory(ctx, f.outsider, "du-an-a", "Của người lạ", ""); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("outsider create must be not-found: %v", err)
	}

	dirs, err := f.repo.Stories(ctx, f.viewer, "du-an-a")
	if err != nil {
		t.Fatalf("viewer stories: %v", err)
	}
	if len(dirs) != 2 || dirs[0].Name != "01-truyen-ke-dai" || dirs[1].Name != "02-truyen-moi" || dirs[0].Dir != "" || dirs[0].UpdatedAt == nil {
		t.Fatalf("story list: %+v", dirs)
	}
	if dirs, err := f.repo.Stories(ctx, f.outsider, "du-an-a"); !errors.Is(err, errs.ErrStorageNotFound) || len(dirs) != 0 {
		t.Fatalf("outsider story list must be not-found: %+v err=%v", dirs, err)
	}

	if st, ok := f.repo.ResolveStory(ctx, f.viewer, "du-an-a", "01-truyen-ke-dai"); !ok || st.ID != "du-an-a/01-truyen-ke-dai" {
		t.Fatalf("viewer resolve: %+v ok=%v", st, ok)
	}
	if _, ok := f.repo.ResolveStory(ctx, f.outsider, "du-an-a", "01-truyen-ke-dai"); ok {
		t.Fatal("outsider must not resolve another tenant's story")
	}
	if _, ok := f.repo.ResolveStory(ctx, f.owner, "du-an-a", "99-khong-co"); ok {
		t.Fatal("missing story must not resolve")
	}

	if _, err := f.repo.RenameStory(ctx, f.viewer, "du-an-a", "02-truyen-moi", "ten-moi"); !errors.Is(err, errs.ErrStorageForbidden) {
		t.Fatalf("viewer rename must be forbidden: %v", err)
	}
	renamed, err := f.repo.RenameStory(ctx, f.owner, "du-an-a", "02-truyen-moi", "ten-moi")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Story != "02-ten-moi" || renamed.ID != "du-an-a/02-ten-moi" {
		t.Fatalf("rename must keep the number: %+v", renamed)
	}
	if _, ok := f.repo.ResolveStory(ctx, f.owner, "du-an-a", "02-truyen-moi"); ok {
		t.Fatal("old slug must stop resolving after rename")
	}
	// A duplicate "NN-" can only be crafted by hand (the file backend test does
	// the same with two 02- directories): renaming onto it must be ErrExists.
	q := gen.New(f.pool.Pool())
	project, err := q.GetProjectBySlug(ctx, "du-an-a")
	if err != nil {
		t.Fatalf("read project: %v", err)
	}
	if _, err := q.InsertStory(ctx, gen.InsertStoryParams{
		ProjectID: project.ID, ID: deterministicUUID("story", "dup"),
		Slug: "01-khac", Title: "x", Status: "idea", Tags: []string{},
	}); err != nil {
		t.Fatalf("insert duplicate-number story: %v", err)
	}
	if _, err := f.repo.RenameStory(ctx, f.owner, "du-an-a", "01-truyen-ke-dai", "khac"); !errors.Is(err, library.ErrExists) {
		t.Fatalf("rename onto an existing NN-slug must be ErrExists: %v", err)
	}

	if err := f.repo.DeleteStory(ctx, f.viewer, "du-an-a", "02-ten-moi"); !errors.Is(err, errs.ErrStorageForbidden) {
		t.Fatalf("viewer delete must be forbidden: %v", err)
	}
	if err := f.repo.DeleteStory(ctx, f.owner, "du-an-a", "02-ten-moi"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := f.repo.ResolveStory(ctx, f.owner, "du-an-a", "02-ten-moi"); ok {
		t.Fatal("soft-deleted story must stop resolving")
	}
	if err := f.repo.DeleteStory(ctx, f.owner, "du-an-a", "02-ten-moi"); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("second delete must be not-found: %v", err)
	}
}

// TestLibraryRepositoryConcurrentNumbering: five parallel creates take five
// distinct numbers — the project row lock serialises the max+1 read with the
// insert (a plain read-then-insert would hand out duplicates).
func TestLibraryRepositoryConcurrentNumbering(t *testing.T) {
	f := provisionLibrary(t, "du-an-a")
	ctx := context.Background()

	var wg sync.WaitGroup
	errsCh := make(chan error, 5)
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := f.repo.CreateStory(ctx, f.owner, "du-an-a", "", ""); err != nil {
				errsCh <- err
			}
		}()
	}
	wg.Wait()
	close(errsCh)
	for err := range errsCh {
		t.Fatalf("concurrent create failed: %v", err)
	}

	dirs, err := f.repo.Stories(ctx, f.owner, "du-an-a")
	if err != nil {
		t.Fatalf("stories: %v", err)
	}
	if len(dirs) != 5 {
		t.Fatalf("5 creates must yield 5 stories: %+v", dirs)
	}
	seen := map[string]bool{}
	for _, d := range dirs {
		n, _ := library.SplitNumbered(d.Name)
		if n <= 0 || seen[d.Name] {
			t.Fatalf("duplicate/unnumbered slug %q in %+v", d.Name, dirs)
		}
		seen[d.Name] = true
	}
}

// TestLibraryRepositoryStoryFileAndIdeas proves story.json parity on rows
// (title/status/tags/idea link + synopsis preservation) and the idea document
// contract (id naming, title/excerpt derivation, usedBy links, actor scope).
func TestLibraryRepositoryStoryFileAndIdeas(t *testing.T) {
	f := provisionLibrary(t, "du-an-a")
	ctx := context.Background()

	idea, err := f.repo.CreateIdea(ctx, f.owner, "du-an-a", "Một cô gái tìm lại ký ức.\nDòng thứ hai.", "")
	if err != nil {
		t.Fatalf("create idea: %v", err)
	}
	if idea.Title != "Một cô gái tìm lại ký ức." || idea.Excerpt != "Dòng thứ hai." || idea.UsedBy == nil || len(idea.UsedBy) != 0 {
		t.Fatalf("idea fields: %+v", idea)
	}
	if idea.CreatedAt == nil || idea.UpdatedAt == nil {
		t.Fatalf("idea timestamps: %+v", idea)
	}
	if _, err := f.repo.CreateIdea(ctx, f.viewer, "du-an-a", "x", ""); !errors.Is(err, errs.ErrStorageForbidden) {
		t.Fatalf("viewer create idea must be forbidden: %v", err)
	}

	got, text, err := f.repo.GetIdea(ctx, f.viewer, "du-an-a", idea.ID)
	if err != nil {
		t.Fatalf("viewer get idea: %v", err)
	}
	if got.ID != idea.ID || !strings.Contains(text, "# Một cô gái tìm lại ký ức.") {
		t.Fatalf("idea body: %q id=%q", text, got.ID)
	}
	if _, _, err := f.repo.GetIdea(ctx, f.outsider, "du-an-a", idea.ID); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("outsider idea must be not-found: %v", err)
	}
	if _, _, err := f.repo.GetIdea(ctx, f.owner, "du-an-a", "khong-co"); !errors.Is(err, library.ErrIdeaNotFound) {
		t.Fatalf("missing idea must be ErrIdeaNotFound: %v", err)
	}

	st, _, err := f.repo.CreateStory(ctx, f.owner, "du-an-a", "Truyện từ ý tưởng", idea.ID)
	if err != nil {
		t.Fatalf("create story from idea: %v", err)
	}
	sf, err := f.repo.LoadStoryFile(ctx, f.viewer, st)
	if err != nil {
		t.Fatalf("load story file: %v", err)
	}
	if sf.Title != "Truyện từ ý tưởng" || sf.Status != "idea" || sf.Idea != idea.ID || sf.Tags == nil || sf.CreatedAt == nil {
		t.Fatalf("story file from idea: %+v", sf)
	}
	list, err := f.repo.ListIdeas(ctx, f.owner, "du-an-a")
	if err != nil || len(list) != 1 {
		t.Fatalf("list ideas: %+v err=%v", list, err)
	}
	if want := "du-an-a/" + st.Story; len(list[0].UsedBy) != 1 || list[0].UsedBy[0] != want {
		t.Fatalf("idea usedBy must point at the story: %+v want %q", list[0].UsedBy, want)
	}

	if _, _, err := f.repo.CreateStory(ctx, f.owner, "du-an-a", "Ý tưởng ma", "khong-co"); !errors.Is(err, library.ErrIdeaNotFound) {
		t.Fatalf("story with missing idea must be ErrIdeaNotFound: %v", err)
	}

	if sf, err := f.repo.LoadStoryFile(ctx, f.outsider, st); !errors.Is(err, errs.ErrStorageNotFound) || sf != nil {
		t.Fatalf("outsider story file must be not-found: %+v err=%v", sf, err)
	}
	if err := f.repo.WriteStoryFile(ctx, f.viewer, st, library.StoryFile{Title: "hack"}); !errors.Is(err, errs.ErrStorageForbidden) {
		t.Fatalf("viewer write must be forbidden: %v", err)
	}

	written := library.StoryFile{Title: "Tên người dùng", ShortTitle: "Ngắn", Status: "writing", Tags: []string{"a"}, Idea: ""}
	if err := f.repo.WriteStoryFile(ctx, f.collab, st, written); err != nil {
		t.Fatalf("collab write: %v", err)
	}
	sf, err = f.repo.LoadStoryFile(ctx, f.owner, st)
	if err != nil {
		t.Fatalf("reload story file: %v", err)
	}
	if sf.Title != "Tên người dùng" || sf.ShortTitle != "Ngắn" || sf.Status != "writing" || len(sf.Tags) != 1 || sf.Tags[0] != "a" || sf.Idea != "" {
		t.Fatalf("story file after write: %+v", sf)
	}
	// The engine-owned synopsis column survives a story.json rewrite.
	var synopsis string
	if err := f.pool.Pool().QueryRow(ctx,
		`SELECT synopsis FROM stories WHERE project_id = (SELECT id FROM projects WHERE slug = 'du-an-a') AND slug = $1`,
		st.Story).Scan(&synopsis); err != nil {
		t.Fatalf("read synopsis: %v", err)
	}
	if synopsis != "" {
		t.Fatalf("synopsis column must not be invented: %q", synopsis)
	}
}
