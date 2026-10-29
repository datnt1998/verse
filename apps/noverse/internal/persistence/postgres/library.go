package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

// LibraryRepository là thư viện repository trên PostgreSQL (phase 5): project
// rows thay thư mục, story rows thay <story>/story.json, project_documents
// kind=idea thay <project>/<dirs.ideas>/*.md. Ngữ nghĩa phải khớp backend file
// (id = slug thư mục, story id = "NN-slug", idea id = tên tài liệu) để URL và
// UI giữ nguyên. Không có thư mục: Dir/ProjectDir/Root luôn rỗng.
//
// File-only capabilities (browse ứng viên, draft cộng tác, assets, validate/
// fix) trả errs.ErrStorageUnsupported thay vì giả dữ liệu rỗng.
type LibraryRepository struct {
	pool *pgxpool.Pool
}

// NewLibraryRepository trả library.Repository backed bởi Postgres.
func NewLibraryRepository(pool *Pool) *LibraryRepository {
	return &LibraryRepository{pool: pool.Pool()}
}

var _ library.Repository = (*LibraryRepository)(nil)

func (r *LibraryRepository) RootPath() string       { return "" }
func (r *LibraryRepository) Config() library.Config { return library.DefaultConfig() }
func errUnsupported() error                         { return errs.ErrStorageUnsupported }

// libActs maps the request-scoped library actor onto the repository actor.
// Nil stays nil (internal paths already authorized before reaching here).
func libActs(actor *library.Actor) *Actor {
	if actor == nil {
		return nil
	}
	return &Actor{Username: actor.Username, GlobalRole: actor.GlobalRole}
}

func (r *LibraryRepository) q(tx pgx.Tx) *gen.Queries { return gen.New(tx) }

func (r *LibraryRepository) tx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return RunInTx(ctx, r.pool, fn)
}

// libConstraintErr maps the library's name-collision contract onto Postgres
// unique violations: every UNIQUE in this surface is a slug/logical-name that
// already exists (project slug, story slug, idea logical name).
func libConstraintErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(constraintErr(err), errs.ErrStorageConflict) {
		return fmt.Errorf("%w: %v", library.ErrExists, sanitizeErr(err))
	}
	return sanitizeErr(err)
}

// memberProject resolves a project slug inside the open transaction and applies
// the actor's membership: admin bypasses, non-member is reported as
// "not found" (never leaks whether another tenant's project exists), and
// write=true requires owner|collab. The role check lives in the same
// transaction as the mutation, so handlers cannot check-then-use.
func (r *LibraryRepository) memberProject(ctx context.Context, q *gen.Queries, actor *Actor, slug string, write bool) (gen.Project, error) {
	var project gen.Project
	p, err := q.GetProjectBySlug(ctx, slug)
	if err != nil {
		if noRows(err) {
			return project, fmt.Errorf("dự án %s: %w", slug, errs.ErrStorageNotFound)
		}
		return project, sanitizeErr(err)
	}
	if actor == nil || actor.isAdmin() {
		return p, nil
	}
	user, err := q.GetUserByUsername(ctx, actor.Username)
	if err != nil {
		if noRows(err) {
			return project, fmt.Errorf("dự án %s: %w", slug, errs.ErrStorageNotFound)
		}
		return project, sanitizeErr(err)
	}
	role, err := q.GetMemberRole(ctx, gen.GetMemberRoleParams{ProjectID: p.ID, UserID: user.ID})
	if err != nil {
		if noRows(err) {
			return project, fmt.Errorf("dự án %s: %w", slug, errs.ErrStorageNotFound)
		}
		return project, sanitizeErr(err)
	}
	if write && role != "owner" && role != "collab" {
		return project, fmt.Errorf("dự án %s: %w", slug, errs.ErrStorageForbidden)
	}
	return p, nil
}

// ── list & resolve ───────────────────────────────────────────────────────────

// Scan liệt kê project rows actor đọc được. Candidates/Legacy rỗng: trên PG
// không có thư mục chưa nhận và không có dự án ẩn danh.
func (r *LibraryRepository) Scan(ctx context.Context, actor *library.Actor) (*library.ScanResult, error) {
	out := &library.ScanResult{Projects: []library.ProjectRef{}, Candidates: []string{}, Legacy: []string{}}
	err := r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		act := libActs(actor)
		rows, err := q.ListProjects(ctx)
		if err != nil {
			return sanitizeErr(err)
		}
		if act == nil || act.isAdmin() {
			for _, p := range rows {
				out.Projects = append(out.Projects, library.ProjectRef{ID: p.Slug})
			}
			return nil
		}
		user, err := q.GetUserByUsername(ctx, act.Username)
		if err != nil {
			if noRows(err) {
				return nil
			}
			return sanitizeErr(err)
		}
		members, err := q.ListMembershipsByUser(ctx, user.ID)
		if err != nil {
			return sanitizeErr(err)
		}
		allowed := make(map[string]bool, len(members))
		for _, m := range members {
			allowed[m.Slug] = true
		}
		for _, p := range rows {
			if allowed[p.Slug] {
				out.Projects = append(out.Projects, library.ProjectRef{ID: p.Slug})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ResolveProject reports whether actor may read the project. The boolean
// contract has no error channel; storage failures are reported as "not found"
// so a caller never renders a half-resolved project.
func (r *LibraryRepository) ResolveProject(ctx context.Context, actor *library.Actor, project string) (library.ProjectRef, bool) {
	var ref library.ProjectRef
	err := r.tx(ctx, func(tx pgx.Tx) error {
		p, err := r.memberProject(ctx, r.q(tx), libActs(actor), project, false)
		if err != nil {
			return err
		}
		ref = library.ProjectRef{ID: p.Slug}
		return nil
	})
	if err != nil {
		return library.ProjectRef{}, false
	}
	return ref, true
}

// Stories lists the project's stories by slug; UpdatedAt is the row's
// updated_at (hosted equivalent of the directory mtime) and Dir stays empty.
func (r *LibraryRepository) Stories(ctx context.Context, actor *library.Actor, project string) ([]library.StoryDir, error) {
	out := []library.StoryDir{}
	err := r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, false)
		if err != nil {
			return err
		}
		rows, err := q.ListStoriesByProject(ctx, p.ID)
		if err != nil {
			return sanitizeErr(err)
		}
		for _, s := range rows {
			out = append(out, library.StoryDir{Project: project, Name: s.Slug, UpdatedAt: tsPtr(s.UpdatedAt)})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ResolveStory resolves (project, story) for a readable actor. No directory is
// attached: hosted stories have no filesystem mirror, and engine paths fail
// loudly instead of reading a tree.
func (r *LibraryRepository) ResolveStory(ctx context.Context, actor *library.Actor, project, story string) (library.Story, bool) {
	var st library.Story
	err := r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, false)
		if err != nil {
			return err
		}
		if row, err := q.GetStoryBySlug(ctx, gen.GetStoryBySlugParams{ProjectID: p.ID, Slug: story}); err != nil {
			if noRows(err) {
				return fmt.Errorf("truyện %s/%s: %w", project, story, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		} else {
			st = library.Story{Project: project, Story: story, ID: project + "/" + story, UpdatedAt: tsPtr(row.UpdatedAt)}
		}
		return nil
	})
	if err != nil {
		return library.Story{}, false
	}
	return st, true
}

// ── project CRUD ─────────────────────────────────────────────────────────────

// CreateProject inserts the project row and makes the actor its owner in the
// same transaction (the owner row is what the membership checks read). Only an
// admin may create a project; the handler already enforces this and the
// repository re-checks so an internal caller cannot bypass it.
func (r *LibraryRepository) CreateProject(ctx context.Context, actor *library.Actor, slug string, f library.ProjectFile) (string, error) {
	if !library.ValidDirName(slug) {
		return "", fmt.Errorf("%w: %q", library.ErrInvalidName, slug)
	}
	act := libActs(actor)
	if act != nil && !act.isAdmin() {
		return "", fmt.Errorf("tạo dự án: %w", errs.ErrStorageForbidden)
	}
	err := r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		id, err := randomUUID()
		if err != nil {
			return err
		}
		if _, err := q.InsertProject(ctx, gen.InsertProjectParams{
			ID:           id,
			Slug:         slug,
			Name:         f.Name,
			Description:  f.Description,
			Notes:        f.Notes,
			UniverseName: f.UniverseName,
		}); err != nil {
			return libConstraintErr(err)
		}
		if act == nil {
			return nil
		}
		user, err := q.GetUserByUsername(ctx, act.Username)
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("tạo dự án %s: %w", slug, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		return sanitizeErr(q.UpsertMember(ctx, gen.UpsertMemberParams{ProjectID: id, UserID: user.ID, Role: "owner"}))
	})
	if err != nil {
		return "", err
	}
	return slug, nil
}

// AdoptProject is a local-browse capability: there are no unadopted folders on
// a hosted library.
func (r *LibraryRepository) AdoptProject(context.Context, *library.Actor, string, string) (string, error) {
	return "", errUnsupported()
}

// ProjectFile reads the project.json-equivalent fields for a readable actor;
// a missing project is reported as storage-not-found (mapped to 404).
func (r *LibraryRepository) ProjectFile(ctx context.Context, actor *library.Actor, project string) (*library.ProjectFile, error) {
	var out *library.ProjectFile
	err := r.tx(ctx, func(tx pgx.Tx) error {
		p, err := r.memberProject(ctx, r.q(tx), libActs(actor), project, false)
		if err != nil {
			return err
		}
		out = &library.ProjectFile{
			Name:         p.Name,
			Description:  p.Description,
			Notes:        p.Notes,
			CreatedAt:    tsPtr(p.CreatedAt),
			UniverseName: p.UniverseName,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateProject writes the project.json fields with owner|collab write scope.
func (r *LibraryRepository) UpdateProject(ctx context.Context, actor *library.Actor, project string, f library.ProjectFile) error {
	return r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, true)
		if err != nil {
			return err
		}
		n, err := q.UpdateProjectFile(ctx, gen.UpdateProjectFileParams{
			ID:           p.ID,
			Name:         f.Name,
			Description:  f.Description,
			Notes:        f.Notes,
			UniverseName: f.UniverseName,
		})
		if err != nil {
			return sanitizeErr(err)
		}
		if n == 0 {
			return fmt.Errorf("dự án %s: %w", project, errs.ErrStorageNotFound)
		}
		return nil
	})
}

// ── story CRUD ───────────────────────────────────────────────────────────────

// storySlug derives the same "NN-<slug>" identity the file backend uses, so
// existing URLs and bookmarks stay valid on the hosted path.
func storySlug(name string, next int) (slug string, temp bool) {
	name = strings.TrimSpace(name)
	base := "truyen-moi"
	if name != "" {
		base = library.Slugify(name, library.DefaultConfig().SlugWords)
	}
	if utf8.RuneCountInString(base) > 96 {
		base = strings.TrimRight(string([]rune(base)[:96]), "-")
	}
	return fmt.Sprintf("%02d-%s", next, base), base == "truyen-moi"
}

// nextStoryNumber is the maximum "NN-" prefix + 1 (0 when unnumbered).
func nextStoryNumber(rows []gen.Story) int {
	max := 0
	for _, s := range rows {
		if n, _ := library.SplitNumbered(s.Slug); n > max {
			max = n
		}
	}
	return max + 1
}

// CreateStory creates "NN-<slug>" under the project row lock, so concurrent
// creates cannot both take the same number. Duplicate non-temporary names are
// rejected (ErrExists) exactly like the directory backend.
func (r *LibraryRepository) CreateStory(ctx context.Context, actor *library.Actor, project, name, ideaID string) (library.Story, bool, error) {
	var st library.Story
	temp := false
	err := r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, true)
		if err != nil {
			return err
		}
		if _, err := q.LockProjectRow(ctx, p.ID); err != nil {
			if noRows(err) {
				return fmt.Errorf("dự án %s: %w", project, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		rows, err := q.ListStoriesByProject(ctx, p.ID)
		if err != nil {
			return sanitizeErr(err)
		}
		slug, tempName := storySlug(name, nextStoryNumber(rows))
		if !tempName {
			_, base := library.SplitNumbered(slug)
			for _, s := range rows {
				if _, existing := library.SplitNumbered(s.Slug); existing == base {
					return fmt.Errorf("đã có truyện «%s» cùng tên trong dự án: %w", s.Slug, library.ErrExists)
				}
			}
		}
		var ideaDoc pgtype.UUID
		if strings.TrimSpace(ideaID) != "" {
			doc, err := q.GetProjectDocumentByName(ctx, gen.GetProjectDocumentByNameParams{
				ProjectID: p.ID, Kind: "idea", LogicalName: ideaID,
			})
			if err != nil {
				if noRows(err) {
					return fmt.Errorf("ý tưởng %s: %w", ideaID, library.ErrIdeaNotFound)
				}
				return sanitizeErr(err)
			}
			ideaDoc = doc.ID
		}
		id, err := randomUUID()
		if err != nil {
			return err
		}
		if _, err := q.InsertStory(ctx, gen.InsertStoryParams{
			ProjectID:      p.ID,
			ID:             id,
			Slug:           slug,
			Title:          strings.TrimSpace(name),
			Status:         "idea",
			Tags:           []string{},
			IdeaDocumentID: ideaDoc,
		}); err != nil {
			return libConstraintErr(err)
		}
		st = library.Story{Project: project, Story: slug, ID: project + "/" + slug}
		temp = tempName
		return nil
	})
	if err != nil {
		return library.Story{}, false, err
	}
	return st, temp, nil
}

// RenameStory changes only the slug, preserving the "NN-" number (unknown
// number → next free one), the story UUID and every other field. The new slug
// obeys the same ValidDirName boundary as the directory backend.
func (r *LibraryRepository) RenameStory(ctx context.Context, actor *library.Actor, project, story, slug string) (library.Story, error) {
	if !library.ValidDirName(slug) {
		return library.Story{}, fmt.Errorf("%w: %q", library.ErrInvalidName, slug)
	}
	var out library.Story
	err := r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, true)
		if err != nil {
			return err
		}
		row, err := q.GetStoryBySlug(ctx, gen.GetStoryBySlugParams{ProjectID: p.ID, Slug: story})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("truyện %s/%s: %w", project, story, library.ErrNotFound)
			}
			return sanitizeErr(err)
		}
		locked, err := q.LockStoryByID(ctx, gen.LockStoryByIDParams{ProjectID: p.ID, ID: row.ID})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("truyện %s/%s: %w", project, story, library.ErrNotFound)
			}
			return sanitizeErr(err)
		}
		number, _ := library.SplitNumbered(story)
		if number <= 0 {
			rows, err := q.ListStoriesByProject(ctx, p.ID)
			if err != nil {
				return sanitizeErr(err)
			}
			number = nextStoryNumber(rows)
		}
		newSlug := fmt.Sprintf("%02d-%s", number, slug)
		if newSlug == story {
			out = library.Story{Project: project, Story: newSlug, ID: project + "/" + newSlug}
			return nil
		}
		if _, err := q.GetStoryBySlug(ctx, gen.GetStoryBySlugParams{ProjectID: p.ID, Slug: newSlug}); err == nil {
			return fmt.Errorf("đổi tên truyện: %w", library.ErrExists)
		} else if !noRows(err) {
			return sanitizeErr(err)
		}
		if _, err := q.UpdateStoryCAS(ctx, gen.UpdateStoryCASParams{
			ProjectID:      p.ID,
			ID:             locked.ID,
			Slug:           newSlug,
			Title:          locked.Title,
			ShortTitle:     locked.ShortTitle,
			Synopsis:       locked.Synopsis,
			TitleRenamed:   locked.TitleRenamed,
			Status:         locked.Status,
			Tags:           locked.Tags,
			PublishedAt:    locked.PublishedAt,
			IdeaDocumentID: locked.IdeaDocumentID,
			Version:        locked.Version,
		}); err != nil {
			if noRows(err) {
				return fmt.Errorf("đổi tên truyện %s: %w", story, errs.ErrStorageConflict)
			}
			return libConstraintErr(err)
		}
		out = library.Story{Project: project, Story: newSlug, ID: project + "/" + newSlug}
		return nil
	})
	if err != nil {
		return library.Story{}, err
	}
	return out, nil
}

// DeleteStory soft-deletes the story row; no directory or user file is touched.
func (r *LibraryRepository) DeleteStory(ctx context.Context, actor *library.Actor, project, story string) error {
	return r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, true)
		if err != nil {
			return err
		}
		row, err := q.GetStoryBySlug(ctx, gen.GetStoryBySlugParams{ProjectID: p.ID, Slug: story})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("truyện %s/%s: %w", project, story, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		n, err := q.SoftDeleteStory(ctx, gen.SoftDeleteStoryParams{ProjectID: p.ID, ID: row.ID})
		if err != nil {
			return sanitizeErr(err)
		}
		if n == 0 {
			return fmt.Errorf("truyện %s/%s: %w", project, story, errs.ErrStorageNotFound)
		}
		return nil
	})
}

// storyFileOf maps a story row onto the story.json contract. Idea is the idea
// document's logical name (the file backend stores the idea id there).
func storyFileOf(q *gen.Queries, ctx context.Context, row gen.Story) *library.StoryFile {
	f := &library.StoryFile{
		Title:        row.Title,
		ShortTitle:   row.ShortTitle,
		TitleRenamed: row.TitleRenamed,
		Status:       row.Status,
		Tags:         row.Tags,
		PublishedAt:  tsPtr(row.PublishedAt),
		CreatedAt:    tsPtr(row.CreatedAt),
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	if row.IdeaDocumentID.Valid {
		if doc, err := q.GetProjectDocumentByID(ctx, gen.GetProjectDocumentByIDParams{ProjectID: row.ProjectID, ID: row.IdeaDocumentID}); err == nil {
			f.Idea = doc.LogicalName
		}
	}
	return f
}

// LoadStoryFile reads the story.json-equivalent row for a readable actor.
func (r *LibraryRepository) LoadStoryFile(ctx context.Context, actor *library.Actor, st library.Story) (*library.StoryFile, error) {
	var out *library.StoryFile
	err := r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), st.Project, false)
		if err != nil {
			return err
		}
		row, err := q.GetStoryBySlug(ctx, gen.GetStoryBySlugParams{ProjectID: p.ID, Slug: st.Story})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("truyện %s/%s: %w", st.Project, st.Story, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		out = storyFileOf(q, ctx, row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// WriteStoryFile writes the story.json fields under the row lock. Idea is the
// idea document's logical name: "" clears the link (the file backend also
// rewrites the whole story.json), a name that does not exist is
// ErrIdeaNotFound. The synopsis column keeps its value (story.json has no such
// field, so the request body cannot carry one).
func (r *LibraryRepository) WriteStoryFile(ctx context.Context, actor *library.Actor, st library.Story, f library.StoryFile) error {
	return r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), st.Project, true)
		if err != nil {
			return err
		}
		row, err := q.GetStoryBySlug(ctx, gen.GetStoryBySlugParams{ProjectID: p.ID, Slug: st.Story})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("truyện %s/%s: %w", st.Project, st.Story, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		locked, err := q.LockStoryByID(ctx, gen.LockStoryByIDParams{ProjectID: p.ID, ID: row.ID})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("truyện %s/%s: %w", st.Project, st.Story, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		ideaDoc := pgtype.UUID{}
		if name := strings.TrimSpace(f.Idea); name != "" {
			doc, err := q.GetProjectDocumentByName(ctx, gen.GetProjectDocumentByNameParams{ProjectID: p.ID, Kind: "idea", LogicalName: name})
			if err != nil {
				if noRows(err) {
					return fmt.Errorf("ý tưởng %s: %w", name, library.ErrIdeaNotFound)
				}
				return sanitizeErr(err)
			}
			ideaDoc = doc.ID
		}
		status := f.Status
		if status == "" {
			status = "idea"
		}
		tags := f.Tags
		if tags == nil {
			tags = []string{}
		}
		_, err = q.UpdateStoryCAS(ctx, gen.UpdateStoryCASParams{
			ProjectID:      p.ID,
			ID:             locked.ID,
			Slug:           st.Story,
			Title:          f.Title,
			ShortTitle:     f.ShortTitle,
			Synopsis:       locked.Synopsis,
			TitleRenamed:   f.TitleRenamed,
			Status:         status,
			Tags:           tags,
			PublishedAt:    pgTimestamptzPtr(f.PublishedAt),
			IdeaDocumentID: ideaDoc,
			Version:        locked.Version,
		})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("ghi truyện %s: %w", st.Story, errs.ErrStorageConflict)
			}
			return libConstraintErr(err)
		}
		return nil
	})
}

// ── ideas (project_documents kind=idea) ──────────────────────────────────────

func validIdeaID(id string) bool {
	return id != "" && !strings.HasPrefix(id, ".") && !strings.ContainsAny(id, `/\`)
}

// ideaFromDocument mirrors the file backend's <id>.md reading: the title is
// the first non-empty markdown line, the excerpt is the rest, and createdAt is
// parsed from the "YYYYMMDD-HHmm" name prefix.
func ideaFromDocument(doc gen.ProjectDocument) library.Idea {
	text := doc.TextContent.String
	idea := library.Idea{ID: doc.LogicalName}
	if len(doc.LogicalName) >= 13 {
		prefix := doc.LogicalName[:13]
		if t, err := time.ParseInLocation("20060102-1504", prefix, time.UTC); err == nil {
			idea.CreatedAt = &t
		}
	}
	title := library.FirstLineTitle(text)
	if title != "" {
		_, rest, _ := strings.Cut(text, "\n")
		idea.Title = title
		idea.Excerpt = library.ExcerptOf(strings.TrimSpace(rest), 200)
	} else {
		idea.Title = doc.LogicalName
		idea.Excerpt = library.ExcerptOf(text, 200)
	}
	idea.UpdatedAt = tsPtr(doc.UpdatedAt)
	idea.UsedBy = []string{}
	return idea
}

// usedIdeas maps idea logical name → ids ("project/story") of stories that
// link the idea document.
func usedIdeas(q *gen.Queries, ctx context.Context, projectID pgtype.UUID, project string) (map[string][]string, error) {
	rows, err := q.ListStoryIdeaRefs(ctx, projectID)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	used := make(map[string][]string, len(rows))
	for _, row := range rows {
		used[row.IdeaName] = append(used[row.IdeaName], project+"/"+row.Slug)
	}
	for name := range used {
		sort.Strings(used[name])
	}
	return used, nil
}

// ListIdeas lists the project's idea documents by id with usedBy links.
func (r *LibraryRepository) ListIdeas(ctx context.Context, actor *library.Actor, project string) ([]library.Idea, error) {
	out := []library.Idea{}
	err := r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, false)
		if err != nil {
			return err
		}
		rows, err := q.ListProjectDocumentsByKind(ctx, gen.ListProjectDocumentsByKindParams{ProjectID: p.ID, Kind: "idea"})
		if err != nil {
			return sanitizeErr(err)
		}
		used, err := usedIdeas(q, ctx, p.ID, project)
		if err != nil {
			return err
		}
		for _, doc := range rows {
			idea := ideaFromDocument(doc)
			if ids := used[idea.ID]; ids != nil {
				idea.UsedBy = ids
			}
			out = append(out, idea)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetIdea returns one idea and its markdown body.
func (r *LibraryRepository) GetIdea(ctx context.Context, actor *library.Actor, project, id string) (*library.Idea, string, error) {
	if !validIdeaID(id) {
		return nil, "", library.ErrIdeaNotFound
	}
	var idea *library.Idea
	var text string
	err := r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, false)
		if err != nil {
			return err
		}
		doc, err := q.GetProjectDocumentByName(ctx, gen.GetProjectDocumentByNameParams{ProjectID: p.ID, Kind: "idea", LogicalName: id})
		if err != nil {
			if noRows(err) {
				return library.ErrIdeaNotFound
			}
			return sanitizeErr(err)
		}
		got := ideaFromDocument(doc)
		used, err := usedIdeas(q, ctx, p.ID, project)
		if err != nil {
			return err
		}
		if ids := used[id]; ids != nil {
			got.UsedBy = ids
		}
		idea, text = &got, doc.TextContent.String
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return idea, text, nil
}

// CreateIdea stores a markdown idea document named "<YYYYMMDD-HHmm>-<slug>",
// matching the file backend's naming, dedupe suffix and title/excerpt rules.
func (r *LibraryRepository) CreateIdea(ctx context.Context, actor *library.Actor, project, text, title string) (*library.Idea, error) {
	title = strings.TrimSpace(title)
	derived := title == ""
	if derived {
		title = library.FirstLineTitle(text)
	}
	content := text
	switch {
	case strings.HasPrefix(strings.TrimSpace(text), "#") && !derived:
		_, rest, _ := strings.Cut(text, "\n")
		content = "# " + title + "\n" + rest
	case !strings.HasPrefix(strings.TrimSpace(text), "#"):
		content = "# " + title + "\n\n" + text
	}
	var out *library.Idea
	err := r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, true)
		if err != nil {
			return err
		}
		// The project lock serialises the same-second name pick with the
		// matching insert, so two creates cannot both claim "-2".
		if _, err := q.LockProjectRow(ctx, p.ID); err != nil {
			if noRows(err) {
				return fmt.Errorf("dự án %s: %w", project, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		existing, err := q.ListProjectDocumentsByKind(ctx, gen.ListProjectDocumentsByKindParams{ProjectID: p.ID, Kind: "idea"})
		if err != nil {
			return sanitizeErr(err)
		}
		taken := make(map[string]bool, len(existing))
		for _, doc := range existing {
			taken[doc.LogicalName] = true
		}
		base := time.Now().UTC().Format("20060102-1504") + "-" + library.Slugify(title, 6)
		id := base
		for i := 2; taken[id]; i++ {
			id = fmt.Sprintf("%s-%d", base, i)
		}
		docID, err := randomUUID()
		if err != nil {
			return err
		}
		if _, err := q.UpsertProjectDocument(ctx, gen.UpsertProjectDocumentParams{
			ProjectID:   p.ID,
			ID:          docID,
			Kind:        "idea",
			LogicalName: id,
			TextContent: pgtype.Text{String: content, Valid: true},
		}); err != nil {
			return libConstraintErr(err)
		}
		doc, err := q.GetProjectDocumentByName(ctx, gen.GetProjectDocumentByNameParams{ProjectID: p.ID, Kind: "idea", LogicalName: id})
		if err != nil {
			return sanitizeErr(err)
		}
		idea := ideaFromDocument(doc)
		if derived {
			_, rest, _ := strings.Cut(text, "\n")
			idea.Excerpt = library.ExcerptOf(strings.TrimSpace(rest), 200)
		}
		out = &idea
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ── file-only capabilities ───────────────────────────────────────────────────

func (r *LibraryRepository) Assets(context.Context, *library.Actor, string) ([]library.Asset, error) {
	return nil, errUnsupported()
}

func (r *LibraryRepository) Validate(context.Context, *library.Actor, string) (*library.Validation, error) {
	return nil, errUnsupported()
}

func (r *LibraryRepository) Fix(context.Context, *library.Actor, string, []string) (*library.Validation, error) {
	return nil, errUnsupported()
}

func (r *LibraryRepository) DraftDir(context.Context, *library.Actor, string, string) (string, error) {
	return "", errUnsupported()
}

func (r *LibraryRepository) CreateDraft(context.Context, *library.Actor, string, string) (string, error) {
	return "", errUnsupported()
}

func (r *LibraryRepository) RemoveDraft(context.Context, *library.Actor, string, string) error {
	return errUnsupported()
}

func (r *LibraryRepository) CommitDraft(context.Context, *library.Actor, string, string, string) (library.Story, bool, error) {
	return library.Story{}, false, errUnsupported()
}

func (r *LibraryRepository) CleanupDrafts(context.Context, *library.Actor, time.Duration) (int, error) {
	return 0, errUnsupported()
}
