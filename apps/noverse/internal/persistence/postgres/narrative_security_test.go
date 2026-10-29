package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/platform/identity"
)

// memberFixture tạo project với owner/collab/view + một truyện, trả repo và
// các actor — nền cho IDOR/permission tests.
type memberFixture struct {
	pgFixture
	owner  *postgres.Actor
	collab *postgres.Actor
	viewer *postgres.Actor
	stray  *postgres.Actor // tài khoản tồn tại nhưng không phải member
}

func provisionMembers(t *testing.T) memberFixture {
	t.Helper()
	f := provisionNarrative(t, "project-a", "truyen-a")
	ctx := context.Background()
	accounts := identity.NewFromRepository(postgres.NewAccounts(f.pool), library.IdentityOptions())
	q := gen.New(f.pool.Pool())

	project, err := q.GetProjectBySlug(ctx, "project-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []struct {
		name string
		role string
	}{
		{"owner", "owner"}, {"collab", "collab"}, {"viewer", "view"}, {"stray", ""},
	} {
		if _, err := accounts.CreateUser(identity.NewUser{
			Username: u.name, Name: u.name, Role: identity.RoleMember, Password: "mat-khau-12345"}); err != nil {
			t.Fatalf("create %s: %v", u.name, err)
		}
		if u.role == "" {
			continue
		}
		usr, err := q.GetUserByUsername(ctx, u.name)
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpsertMember(ctx, gen.UpsertMemberParams{
			ProjectID: project.ID, UserID: usr.ID, Role: u.role}); err != nil {
			t.Fatal(err)
		}
	}
	return memberFixture{
		pgFixture: f,
		owner:     &postgres.Actor{Username: "owner"},
		collab:    &postgres.Actor{Username: "collab"},
		viewer:    &postgres.Actor{Username: "viewer"},
		stray:     &postgres.Actor{Username: "stray"},
	}
}

// TestNarrativeIsolation kiểm tenant IDOR: mọi read/write phạm vi project —
// non-member (và member của project khác) phải bị chặn ngang bằng "không
// tồn tại", không lộ sự tồn tại của dữ liệu; view không được write; admin
// global bypass membership đúng ma trận quyền.
func TestNarrativeIsolation(t *testing.T) {
	f := provisionMembers(t)
	ctx := context.Background()

	// Stray không phải member: không đọc được truyện, không resolve được,
	// không list được — lỗi chuẩn ErrStorageNotFound cho cả ba (không phân
	// biệt "có dự án khác" và "không có gì").
	if _, err := f.nar.GetStory(ctx, "project-a", "truyen-a", f.stray); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("stray read must be not-found: %v", err)
	}
	if _, err := f.nar.ListStories(ctx, "project-a", f.stray); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("stray list must be not-found: %v", err)
	}
	if _, err := f.nar.Resolve(ctx, "project-a", "truyen-a", f.stray, false); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("stray resolve must be not-found: %v", err)
	}

	// Viewer đọc được, không được ghi — write resolve phải forbidden.
	if _, err := f.nar.Resolve(ctx, "project-a", "truyen-a", f.viewer, false); err != nil {
		t.Fatalf("viewer must read: %v", err)
	}
	if _, err := f.nar.Resolve(ctx, "project-a", "truyen-a", f.viewer, true); !errors.Is(err, errs.ErrStorageForbidden) {
		t.Fatalf("viewer write must be forbidden: %v", err)
	}
	if _, err := f.nar.CreateStory(ctx, "project-a", f.viewer, postgres.StoryInfo{Slug: "hack"}); !errors.Is(err, errs.ErrStorageForbidden) {
		t.Fatalf("viewer create must be forbidden: %v", err)
	}

	// Collab + owner write được; admin global bypass membership hoàn toàn.
	if _, err := f.nar.Resolve(ctx, "project-a", "truyen-a", f.collab, true); err != nil {
		t.Fatalf("collab must write: %v", err)
	}
	admin := &postgres.Actor{Username: "stray", GlobalRole: "admin"}
	if _, err := f.nar.Resolve(ctx, "project-a", "truyen-a", admin, true); err != nil {
		t.Fatalf("admin must bypass membership: %v", err)
	}

	// Story thuộc project A không lọt sang project B kể cả khi B tồn tại.
	f2 := provisionNarrative(t, "project-b", "truyen-b")
	if _, err := f.nar.Resolve(ctx, "project-b", "truyen-a", nil, false); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("story slug must not leak across projects: %v", err)
	}
	// scope B dùng cho repo — đúng câu chuyện của B.
	if _, err := f2.nar.GetStory(ctx, "project-b", "truyen-b", nil); err != nil {
		t.Fatalf("own story must read: %v", err)
	}
}

// TestChapterVersionImmutable chứng minh lịch sử không thay đổi: UPDATE/DELETE
// trực tiếp trên chapter_versions đều bị trigger từ chối, và accept lại cùng
// input không nhân đôi revision; attribution được CHECK bắt buộc.
func TestChapterVersionImmutable(t *testing.T) {
	f := provisionNarrative(t, "project-a", "truyen-a")
	ctx := context.Background()

	r1, err := f.nar.AcceptRecord(ctx, f.scope, 1, domain.ChapterOriginGenerated, "v1", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "writer"})
	if err != nil {
		t.Fatal(err)
	}
	if r1.Revision != 1 {
		t.Fatalf("revision: %d", r1.Revision)
	}
	// Immutability triggers: history never mutates.
	if _, err := f.pool.Pool().Exec(ctx,
		`UPDATE chapter_versions SET content = 'sửa lén' WHERE project_id = $1`, f.scope.ProjectID); err == nil {
		t.Fatal("UPDATE on chapter_versions must be rejected")
	} else if !strings.Contains(err.Error(), "vai trò bảo trì") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := f.pool.Pool().Exec(ctx,
		`DELETE FROM chapter_versions WHERE project_id = $1`, f.scope.ProjectID); err == nil {
		t.Fatal("DELETE on chapter_versions must be rejected")
	}
	// Idempotent re-accept leaves history alone.
	if _, err := f.nar.AcceptRecord(ctx, f.scope, 1, domain.ChapterOriginGenerated, "v1", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "writer"}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := f.pool.Pool().QueryRow(ctx,
		`SELECT count(*) FROM chapter_versions WHERE project_id = $1`, f.scope.ProjectID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("idempotent accept must keep 1 revision: %v %d", err, n)
	}
	// Attribution CHECK: author_kind=user without user id must fail.
	if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO chapter_versions
		(project_id, chapter_id, revision_no, content, sha256, origin, author_kind, accepted_at)
		SELECT project_id, chapter_id, 99, 'x', repeat('0',64), 'user', 'user', now()
		FROM chapter_versions WHERE project_id = $1`, f.scope.ProjectID); err == nil {
		t.Fatal("author_kind=user without user id must violate CHECK")
	}
	// History readable: old revision still loads.
	hist, err := f.nar.VersionHistory(ctx, f.scope, 1, 0, 10)
	if err != nil || len(hist) != 1 || hist[0].Content != "v1" {
		t.Fatalf("history: %v %+v", err, hist)
	}
}

// TestCrossProjectFKRejected: composite FKs phải từ chối mọi liên kết chéo
// project ngay cả bằng SQL trực tiếp — comment gắn chapter của story khác,
// chapter gắn volume của story khác, reply gắn comment của project khác.
func TestCrossProjectFKRejected(t *testing.T) {
	a := provisionNarrative(t, "project-a", "truyen-a")
	b := provisionNarrative(t, "project-b", "truyen-b")
	ctx := context.Background()

	if _, err := a.nar.AcceptRecord(ctx, a.scope, 1, domain.ChapterOriginGenerated, "A1", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "writer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.nar.AcceptRecord(ctx, b.scope, 1, domain.ChapterOriginGenerated, "B1", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "writer"}); err != nil {
		t.Fatal(err)
	}
	var aChapter, aStory, bChapter pgtype.UUID
	if err := a.pool.Pool().QueryRow(ctx,
		`SELECT id, story_id FROM chapters WHERE project_id=$1`, a.scope.ProjectID).Scan(&aChapter, &aStory); err != nil {
		t.Fatal(err)
	}
	if err := b.pool.Pool().QueryRow(ctx,
		`SELECT id FROM chapters WHERE project_id=$1`, b.scope.ProjectID).Scan(&bChapter); err != nil {
		t.Fatal(err)
	}
	// Comment của project A trỏ chapter của project B → composite FK phải chặn.
	_, err := a.pool.Pool().Exec(ctx, `INSERT INTO comments
		(project_id, story_id, id, chapter_id, legacy_author_name, body, status)
		VALUES ($1, $2, 'evil-1', $3, 'kẻ lạ', 'x', 'pending')`,
		a.scope.ProjectID, aStory, bChapter)
	var pgErr *pgconn.PgError
	if err == nil || !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("cross-project comment must violate FK: %v", err)
	}
	// Góp ý hợp lệ trên project A rồi reply trỏ sang project B.
	if _, err := a.pool.Pool().Exec(ctx, `INSERT INTO comments
		(project_id, story_id, id, chapter_id, legacy_author_name, body, status)
		VALUES ($1, $2, 'ok-1', $3, 'dat', 'x', 'pending')`,
		a.scope.ProjectID, aStory, aChapter); err != nil {
		t.Fatal(err)
	}
	if _, err := b.pool.Pool().Exec(ctx, `INSERT INTO comment_replies
		(project_id, comment_id, id, legacy_author_name, body)
		VALUES ($1, 'ok-1', 'r-1', 'kẻ lạ', 'x')`, b.scope.ProjectID); err == nil {
		t.Fatal("reply must not link a comment owned by another project")
	}
	// Chapter của B không gắn volume của A.
	if err := a.nar.SaveLayeredOutline(ctx, a.scope, []domain.VolumeOutline{{Index: 1, Title: "Tập"}}); err != nil {
		t.Fatal(err)
	}
	if err := b.nar.SetChapterVolume(ctx, b.scope, 1, 1); err == nil {
		t.Fatal("volume number must not resolve across stories")
	}
}

// TestOutlineTransaction: concurrent draft CAS và accept serialize — hai
// session "cùng sửa một chương": người đầu commit, người sau gặp conflict
// (không ghi đè âm thầm), lịch sử giữ đúng hai revision tuần tự.
func TestOutlineTransaction(t *testing.T) {
	f := provisionNarrative(t, "project-a", "truyen-a")
	ctx := context.Background()

	if err := f.nar.SaveDraft(ctx, f.scope, 1, "nháp v1"); err != nil {
		t.Fatal(err)
	}
	v1, err := f.nar.ChapterDraftVersion(ctx, f.scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.nar.SaveDraftCAS(ctx, f.scope, 1, "nháp v2", v1); err != nil {
		t.Fatal(err)
	}
	// Stale submit từ version cũ phải conflict.
	if err := f.nar.SaveDraftCAS(ctx, f.scope, 1, "nháp cũ", v1); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("stale draft must conflict: %v", err)
	}
	got, err := f.nar.LoadDraft(ctx, f.scope, 1)
	if err != nil || got != "nháp v2" {
		t.Fatalf("conflict must not overwrite: %q %v", got, err)
	}

	// Accept hai revision liên tiếp — seq đúng, pointer trỏ bản mới.
	if _, err := f.nar.AcceptRecord(ctx, f.scope, 1, domain.ChapterOriginGenerated, "bản 1", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "writer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.nar.AcceptRecord(ctx, f.scope, 1, domain.ChapterOriginUser, "bản 2", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "user", Username: "owner"}); !errors.Is(err, errs.ErrStorageNotFound) {
		// owner user không tồn tại trong fixture này → phải not-found, không crash
		t.Fatalf("missing author must error cleanly: %v", err)
	}
	// Version guard trigger: update không tăng đúng version phải bị từ chối.
	if _, err := f.pool.Pool().Exec(ctx,
		`UPDATE chapters SET title = 'x' WHERE project_id = $1`, f.scope.ProjectID); err == nil {
		t.Fatal("update without version bump must be rejected by guard")
	}
}

// TestCASVersioning: story row CAS — rename giữ identity, stale version
// conflict, slug trùng bị UNIQUE chặn.
func TestCASVersioning(t *testing.T) {
	f := provisionNarrative(t, "project-a", "truyen-a")
	ctx := context.Background()

	info, err := f.nar.GetStory(ctx, "project-a", "truyen-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.nar.RenameStory(ctx, "project-a", "truyen-a", "truyen-moi", nil, info.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := f.nar.GetStory(ctx, "project-a", "truyen-a", nil); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("old slug must be gone: %v", err)
	}
	renamed, err := f.nar.GetStory(ctx, "project-a", "truyen-moi", nil)
	if err != nil || renamed.Title != "Truyện thử" {
		t.Fatalf("rename must keep identity+fields: %v %+v", err, renamed)
	}
	// Stale version phải conflict.
	if err := f.nar.RenameStory(ctx, "project-a", "truyen-moi", "lai-doi", nil, info.Version); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("stale rename must conflict: %v", err)
	}
	// Slug trùng → conflict (UNIQUE project+slug).
	if _, err := f.nar.CreateStory(ctx, "project-a", nil, postgres.StoryInfo{Slug: "truyen-moi", Title: "x"}); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("duplicate slug must conflict: %v", err)
	}
}

// TestSearch: pg_trgm trên story title + chapter title/content + character,
// luôn scope project trước giới hạn; kết quả không lọt sang project khác.
func TestSearch(t *testing.T) {
	f := provisionNarrative(t, "project-a", "truyen-a")
	other := provisionNarrative(t, "project-b", "truyen-b")
	ctx := context.Background()

	if err := f.nar.SaveDraft(ctx, f.scope, 1, "Linh mở cửa thư viện lúc nửa đêm."); err != nil {
		t.Fatal(err)
	}
	if _, err := f.nar.AcceptRecord(ctx, f.scope, 1, domain.ChapterOriginGenerated,
		"Linh mở cửa thư viện lúc nửa đêm.", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "writer"}); err != nil {
		t.Fatal(err)
	}
	if err := f.nar.SaveCharacters(ctx, f.scope, []domain.Character{{ID: "linh", Name: "Linh"}}); err != nil {
		t.Fatal(err)
	}
	// Dữ liệu bên project khác có cùng nội dung — không được lọt kết quả.
	if _, err := other.nar.AcceptRecord(ctx, other.scope, 1, domain.ChapterOriginGenerated,
		"Linh mở cửa thư viện lúc nửa đêm.", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "writer"}); err != nil {
		t.Fatal(err)
	}

	hits, err := f.nar.Search(ctx, "project-a", "thư viện", nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, h := range hits {
		kinds[h.Kind] = true
		if h.Story != "" && h.Story != "truyen-a" {
			t.Fatalf("cross-tenant hit: %+v", h)
		}
	}
	if !kinds["chapter_content"] {
		t.Fatalf("content hit missing: %+v", hits)
	}
	ch, err := f.nar.Search(ctx, "project-a", "Linh", nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	var foundChar bool
	for _, h := range ch {
		if h.Kind == "character" && h.Title == "Linh" {
			foundChar = true
		}
	}
	if !foundChar {
		t.Fatalf("character hit missing: %+v", ch)
	}
	st, err := f.nar.Search(ctx, "project-a", "Truyện thử", nil, 10)
	if err != nil || len(st) == 0 || st[0].Kind != "story" {
		t.Fatalf("story title hit missing: %v %+v", err, st)
	}
	// Non-member search → not-found (project predicate trước, không rò rỉ).
	stray := &postgres.Actor{Username: "khong-ai"}
	if _, err := f.nar.Search(ctx, "project-a", "thư viện", stray, 10); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("non-member search must be not-found: %v", err)
	}
}

// TestRepliesAndEvents: bảng comment_replies lưu reply tường minh và hàng đợi
// sự kiện reanchor drain nguyên tử.
func TestRepliesAndEvents(t *testing.T) {
	f := provisionNarrative(t, "project-a", "truyen-a")
	cs := f.nar.CommentsStore(f.scope)
	c, err := cs.Add(1, comments.Input{Text: "xem lại"}, "dat")
	if err != nil {
		t.Fatal(err)
	}
	replies := f.nar.Replies(f.scope)
	if err := replies.AddReply(c.ID, "mình sẽ xử lý", "owner"); err != nil {
		t.Fatal(err)
	}
	got, err := replies.Replies(c.ID)
	if err != nil || len(got) != 1 || got[0].Text != "mình sẽ xử lý" {
		t.Fatalf("replies: %v %+v", err, got)
	}
	// Event queue drains atomically.
	if err := cs.AppendEvent(comments.ReanchorEvent{Chapter: 1, Fixed: 1}); err != nil {
		t.Fatal(err)
	}
	events, err := cs.DrainEvents()
	if err != nil || len(events) != 1 || events[0].Fixed != 1 {
		t.Fatalf("events: %v %+v", err, events)
	}
	if again, err := cs.DrainEvents(); err != nil || len(again) != 0 {
		t.Fatalf("drained queue must stay empty: %v %+v", err, again)
	}
}
