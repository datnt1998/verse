package library

import (
	"context"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
)

// Actor là danh tính của request đi vào repository — immutable, request-scoped.
// Nil = đường nội bộ (scan nền, engine wiring) đã được phân quyền trước; actor
// non-nil bắt buộc membership được kiểm trong repository (admin bypass, đúng
// ma trận vai trò của tài khoản). Backend file không phân biệt actor.
type Actor struct {
	Username   string
	GlobalRole string // "admin" | "member"
}

// Repository là ranh giới hosted của thư viện: mọi CRUD project/story/idea và
// toàn bộ điểm resolve đi qua nó thay vì os.Stat trực tiếp, để backend
// PostgreSQL có thể thay thư mục bằng rows. Backend file giữ nguyên ngữ nghĩa
// thư mục; file-only capabilities (browse ứng viên, draft cộng tác, assets,
// validate/fix) trả ErrStorageUnsupported trên backend hosted — không giả
// kết quả rỗng.
type Repository interface {
	// Identity/config của backend. RootPath()/Config() trả thông tin hiển thị;
	// hosted backend không có thư mục thư viện nên RootPath() = "".
	RootPath() string
	Config() Config

	// Liệt kê & resolve (read scope: member đọc được; actor nil = tất cả).
	Scan(ctx context.Context, actor *Actor) (*ScanResult, error)
	ResolveProject(ctx context.Context, actor *Actor, project string) (ProjectRef, bool)
	Stories(ctx context.Context, actor *Actor, project string) ([]StoryDir, error)
	ResolveStory(ctx context.Context, actor *Actor, project, story string) (Story, bool)

	// CRUD project (write scope: owner|collab cho dữ kiện, admin cho vòng đời
	// project — handler giữ quyền như cũ, repo kiểm lại trong tx).
	CreateProject(ctx context.Context, actor *Actor, slug string, f ProjectFile) (string, error)
	AdoptProject(ctx context.Context, actor *Actor, dirName, name string) (string, error)
	ProjectFile(ctx context.Context, actor *Actor, project string) (*ProjectFile, error)
	UpdateProject(ctx context.Context, actor *Actor, project string, f ProjectFile) error

	// CRUD story + story.json facts (write scope: owner|collab).
	CreateStory(ctx context.Context, actor *Actor, project, name, ideaID string) (Story, bool, error)
	RenameStory(ctx context.Context, actor *Actor, project, story, slug string) (Story, error)
	DeleteStory(ctx context.Context, actor *Actor, project, story string) error
	LoadStoryFile(ctx context.Context, actor *Actor, st Story) (*StoryFile, error)
	WriteStoryFile(ctx context.Context, actor *Actor, st Story, f StoryFile) error

	// CRUD ý tưởng (project_documents kind=idea trên hosted backend).
	ListIdeas(ctx context.Context, actor *Actor, project string) ([]Idea, error)
	GetIdea(ctx context.Context, actor *Actor, project, id string) (*Idea, string, error)
	CreateIdea(ctx context.Context, actor *Actor, project, text, title string) (*Idea, error)

	// File-only capabilities — hosted backend trả unsupported.
	Assets(ctx context.Context, actor *Actor, project string) ([]Asset, error)
	Validate(ctx context.Context, actor *Actor, project string) (*Validation, error)
	Fix(ctx context.Context, actor *Actor, project string, codes []string) (*Validation, error)
	DraftDir(ctx context.Context, actor *Actor, project, id string) (string, error)
	CreateDraft(ctx context.Context, actor *Actor, project, id string) (string, error)
	RemoveDraft(ctx context.Context, actor *Actor, project, id string) error
	CommitDraft(ctx context.Context, actor *Actor, project, id, name string) (Story, bool, error)
	CleanupDrafts(ctx context.Context, actor *Actor, maxAge time.Duration) (int, error)
}

// ── file adapter ─────────────────────────────────────────────────────────────

// fileRepository adapts *Library to Repository: every method ignores actor and
// keeps the exact directory semantics of the existing code paths.
type fileRepository struct{ lib *Library }

// FileRepository wraps an opened Library as a Repository (file backend).
func FileRepository(lib *Library) Repository { return fileRepository{lib: lib} }

func (f fileRepository) RootPath() string { return f.lib.Root }
func (f fileRepository) Config() Config   { return f.lib.Config }

func (f fileRepository) Scan(_ context.Context, _ *Actor) (*ScanResult, error) {
	return f.lib.Scan()
}

func (f fileRepository) ResolveProject(_ context.Context, _ *Actor, project string) (ProjectRef, bool) {
	dir, ok := f.lib.ResolveProject(project)
	return ProjectRef{ID: project, Dir: dir, Legacy: project == LegacyProjectID}, ok
}

func (f fileRepository) Stories(_ context.Context, _ *Actor, project string) ([]StoryDir, error) {
	return f.lib.Stories(project)
}

func (f fileRepository) ResolveStory(_ context.Context, _ *Actor, project, story string) (Story, bool) {
	return f.lib.ResolveStory(project, story)
}

func (f fileRepository) CreateProject(_ context.Context, _ *Actor, slug string, pf ProjectFile) (string, error) {
	return f.lib.CreateProject(slug, pf)
}

func (f fileRepository) AdoptProject(_ context.Context, _ *Actor, dirName, name string) (string, error) {
	return f.lib.AdoptProject(dirName, name)
}

func (f fileRepository) ProjectFile(_ context.Context, _ *Actor, project string) (*ProjectFile, error) {
	if project == LegacyProjectID {
		return nil, nil
	}
	dir, ok := f.lib.ResolveProject(project)
	if !ok {
		return nil, ErrNotFound
	}
	return LoadProjectFile(dir)
}

func (f fileRepository) UpdateProject(_ context.Context, _ *Actor, project string, pf ProjectFile) error {
	dir, ok := f.lib.ResolveProject(project)
	if !ok || project == LegacyProjectID {
		return ErrNotFound
	}
	return WriteProjectFile(dir, pf)
}

func (f fileRepository) CreateStory(_ context.Context, _ *Actor, project, name, ideaID string) (Story, bool, error) {
	return f.lib.CreateStory(project, name, ideaID)
}

func (f fileRepository) RenameStory(_ context.Context, _ *Actor, project, story, slug string) (Story, error) {
	return f.lib.RenameStory(project, story, slug)
}

func (f fileRepository) DeleteStory(_ context.Context, _ *Actor, _, _ string) error {
	// File-mode deletion is the gateway's directory saga (removeIdleStory);
	// hosted backends implement this row-side.
	return ErrStorageUnsupported
}

func (f fileRepository) LoadStoryFile(_ context.Context, _ *Actor, st Story) (*StoryFile, error) {
	return LoadStoryFile(st.Dir)
}

func (f fileRepository) WriteStoryFile(_ context.Context, _ *Actor, st Story, sf StoryFile) error {
	return WriteStoryFile(st.Dir, sf)
}

func (f fileRepository) ListIdeas(_ context.Context, _ *Actor, project string) ([]Idea, error) {
	return f.lib.ListIdeas(project)
}

func (f fileRepository) GetIdea(_ context.Context, _ *Actor, project, id string) (*Idea, string, error) {
	return f.lib.GetIdea(project, id)
}

func (f fileRepository) CreateIdea(_ context.Context, _ *Actor, project, text, title string) (*Idea, error) {
	return f.lib.CreateIdea(project, text, title)
}

func (f fileRepository) Assets(_ context.Context, _ *Actor, project string) ([]Asset, error) {
	return f.lib.Assets(project)
}

func (f fileRepository) Validate(_ context.Context, _ *Actor, project string) (*Validation, error) {
	return f.lib.Validate(project)
}

func (f fileRepository) Fix(_ context.Context, _ *Actor, project string, codes []string) (*Validation, error) {
	return f.lib.Fix(project, codes)
}

func (f fileRepository) DraftDir(_ context.Context, _ *Actor, project, id string) (string, error) {
	return f.lib.DraftDir(project, id)
}

func (f fileRepository) CreateDraft(_ context.Context, _ *Actor, project, id string) (string, error) {
	return f.lib.CreateDraft(project, id)
}

func (f fileRepository) RemoveDraft(_ context.Context, _ *Actor, project, id string) error {
	return f.lib.RemoveDraft(project, id)
}

func (f fileRepository) CommitDraft(_ context.Context, _ *Actor, project, id, name string) (Story, bool, error) {
	return f.lib.CommitDraft(project, id, name)
}

func (f fileRepository) CleanupDrafts(_ context.Context, _ *Actor, maxAge time.Duration) (int, error) {
	return f.lib.CleanupDrafts(maxAge)
}

// ── helpers shared with hosted backends ──────────────────────────────────────

// ErrStorageUnsupported is returned by capabilities that only exist on the
// file backend (local browse, drafts, assets, validate/fix). It aliases the
// shared sentinel so error chains match errs.ErrStorageUnsupported.
var ErrStorageUnsupported = errs.ErrStorageUnsupported

// SplitNumbered splits a "NN-slug" name; returns (-1, name) when unnumbered —
// the same rule file backend uses to derive story ordering.
func SplitNumbered(name string) (int, string) { return splitNumbered(name) }

// FirstLineTitle extracts the title the file backend derives from an idea's
// first markdown line (hosted backends reuse it for excerpt parity).
func FirstLineTitle(text string) string { return firstLineTitle(text) }

// ExcerptOf builds the idea excerpt the same way as the file backend.
func ExcerptOf(text string, max int) string { return excerptOf(text, max) }

// ValidSegment reports whether a single path/id segment is clean (rejects
// empty, ".", "..", separators, hidden names) — same boundary the file
// backend applies to story directory names.
func ValidSegment(name string) bool { return cleanSegment(name) }
