package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
)

// Actor là danh tính bất biến của request/run đi vào repository — không bao giờ
// đặt trên pool hay process-global. Nil actor = đường nội bộ (engine/import)
// đã được phân quyền trước khi tới đây; actor non-nil bắt buộc membership
// (admin bypass), writes bắt buộc owner|collab, mọi check nằm trong cùng tx
// với mutation nên không có check-then-use ở handler.
type Actor struct {
	Username   string
	GlobalRole string // "admin" | "member"
}

// Can reports whether the actor may write; admin global role bypasses
// membership entirely (matches the project role matrix).
func (a *Actor) isAdmin() bool { return a != nil && a.GlobalRole == "admin" }

// ScopeRef là (project, story) đã resolve thành UUID trong một lần resolve —
// các phương thức domain dùng nó thay vì slug để không tra lại, và composite
// FK bảo đảm storyID thật sự thuộc projectID.
type ScopeRef struct {
	ProjectID pgtype.UUID
	StoryID   pgtype.UUID
	StorySlug string
}

// Narrative là repository miền truyện trên Postgres: story/chapter/version/
// outline/characters/state/events/lease/search. Không đọc file nghiệp vụ —
// đường file nằm ở internal/store file backend.
type Narrative struct {
	pool *pgxpool.Pool
	// fence, when set, runs first in every transaction so a writer that lost
	// its story lease cannot commit typed rows (chapter versions, reviews).
	fence FenceCheck
}

// NewNarrative builds the story-domain repository on the shared pool.
func NewNarrative(p *Pool) *Narrative { return &Narrative{pool: p.Pool()} }

// WithFence returns a copy whose transactions all pass fence first. The
// receiver stays unfenced: the lease itself claims and renews through it.
func (n *Narrative) WithFence(fence FenceCheck) *Narrative {
	fenced := *n
	fenced.fence = fence
	return &fenced
}

func (n *Narrative) q(tx pgx.Tx) *gen.Queries { return gen.New(tx) }
func (n *Narrative) qp() *gen.Queries         { return gen.New(n.pool) }

// tx runs fn in one transaction, behind the fence when one is set.
func (n *Narrative) tx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	if n.fence == nil {
		return RunInTx(ctx, n.pool, fn)
	}
	return RunInTx(ctx, n.pool, func(tx pgx.Tx) error {
		if err := n.fence(ctx, tx); err != nil {
			return err
		}
		return fn(tx)
	})
}

// resolveStory quy (projectSlug, storySlug, actor) thành ScopeRef bên trong
// một transaction đang mở. Actor non-nil: membership phải tồn tại; write=true
// đòi owner|collab. Mọi "không thấy/không có quyền" đều ra ErrStorageNotFound/
// ErrStorageForbidden — không phân biệt dự án khác tồn tại hay không (IDOR).
func (n *Narrative) resolveStory(ctx context.Context, q *gen.Queries, projectSlug, storySlug string, actor *Actor, write bool) (ScopeRef, error) {
	var scope ScopeRef
	project, err := q.GetProjectBySlug(ctx, projectSlug)
	if err != nil {
		if noRows(err) {
			return scope, fmt.Errorf("truyện %s/%s: %w", projectSlug, storySlug, errs.ErrStorageNotFound)
		}
		return scope, sanitizeErr(err)
	}
	if actor != nil && !actor.isAdmin() {
		user, err := q.GetUserByUsername(ctx, actor.Username)
		if err != nil {
			if noRows(err) {
				return scope, fmt.Errorf("truyện %s/%s: %w", projectSlug, storySlug, errs.ErrStorageNotFound)
			}
			return scope, sanitizeErr(err)
		}
		role, err := q.GetMemberRole(ctx, gen.GetMemberRoleParams{ProjectID: project.ID, UserID: user.ID})
		if err != nil {
			if noRows(err) {
				return scope, fmt.Errorf("truyện %s/%s: %w", projectSlug, storySlug, errs.ErrStorageNotFound)
			}
			return scope, sanitizeErr(err)
		}
		if write && role != "owner" && role != "collab" {
			return scope, fmt.Errorf("truyện %s/%s: %w", projectSlug, storySlug, errs.ErrStorageForbidden)
		}
	}
	story, err := q.GetStoryBySlug(ctx, gen.GetStoryBySlugParams{ProjectID: project.ID, Slug: storySlug})
	if err != nil {
		if noRows(err) {
			return scope, fmt.Errorf("truyện %s/%s: %w", projectSlug, storySlug, errs.ErrStorageNotFound)
		}
		return scope, sanitizeErr(err)
	}
	scope = ScopeRef{ProjectID: project.ID, StoryID: story.ID, StorySlug: storySlug}
	return scope, nil
}

// Resolve mở một transaction chỉ để kiểm phạm vi — dùng khi caller cần kiểm
// quyền trước chuỗi thao tác ngoài DB. Các write thật resolve lại trong tx.
func (n *Narrative) Resolve(ctx context.Context, projectSlug, storySlug string, actor *Actor, write bool) (ScopeRef, error) {
	var scope ScopeRef
	err := n.tx(ctx, func(tx pgx.Tx) error {
		var err error
		scope, err = n.resolveStory(ctx, n.q(tx), projectSlug, storySlug, actor, write)
		return err
	})
	return scope, err
}

// authorUser resolves a display author (username, "" or free text) to either a
// users row or a legacy name inside the tx — matching comments' legacy author
// contract and the author attribution CHECK on chapter_versions.
func (n *Narrative) authorUser(ctx context.Context, q *gen.Queries, author string) (pgtype.UUID, pgtype.Text, error) {
	var uid pgtype.UUID
	var legacy pgtype.Text
	if strings.TrimSpace(author) != "" {
		if u, err := q.GetUserByUsername(ctx, author); err == nil {
			uid = u.ID
			return uid, legacy, nil
		} else if !noRows(err) {
			return uid, legacy, sanitizeErr(err)
		}
	}
	name := strings.TrimSpace(author)
	if name == "" {
		name = "khách"
	}
	legacy = pgtype.Text{String: name, Valid: true}
	return uid, legacy, nil
}

// ── stories ──────────────────────────────────────────────────────────────────

// StoryInfo là bản ghi truyện mức hiển thị (story.json parity).
type StoryInfo struct {
	Slug         string
	Title        string
	ShortTitle   string
	Synopsis     string
	TitleRenamed bool
	Status       string
	Idea         string
	Tags         []string
	PublishedAt  *time.Time
	CreatedAt    time.Time
	Version      int64
}

// CreateStory tạo truyện trong project đã resolve; idea (nếu có) đi vào
// project_documents kind=idea và liên kết FK, không nhồi cột.
func (n *Narrative) CreateStory(ctx context.Context, projectSlug string, actor *Actor, in StoryInfo) (StoryInfo, error) {
	var out StoryInfo
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		project, err := q.GetProjectBySlug(ctx, projectSlug)
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("dự án %s: %w", projectSlug, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		if actor != nil && !actor.isAdmin() {
			user, err := q.GetUserByUsername(ctx, actor.Username)
			if err != nil {
				if noRows(err) {
					return fmt.Errorf("dự án %s: %w", projectSlug, errs.ErrStorageNotFound)
				}
				return sanitizeErr(err)
			}
			role, err := q.GetMemberRole(ctx, gen.GetMemberRoleParams{ProjectID: project.ID, UserID: user.ID})
			if err != nil {
				if noRows(err) {
					return fmt.Errorf("dự án %s: %w", projectSlug, errs.ErrStorageNotFound)
				}
				return sanitizeErr(err)
			}
			if role != "owner" && role != "collab" {
				return fmt.Errorf("dự án %s: %w", projectSlug, errs.ErrStorageForbidden)
			}
		}
		id, err := randomUUID()
		if err != nil {
			return err
		}
		var ideaDoc pgtype.UUID
		if strings.TrimSpace(in.Idea) != "" {
			docID, err := randomUUID()
			if err != nil {
				return err
			}
			row, err := q.UpsertProjectDocument(ctx, gen.UpsertProjectDocumentParams{
				ProjectID:   project.ID,
				ID:          docID,
				Kind:        "idea",
				LogicalName: in.Slug,
				TextContent: pgtype.Text{String: in.Idea, Valid: true},
			})
			if err != nil {
				return sanitizeErr(err)
			}
			ideaDoc = row
		}
		status := in.Status
		if status == "" {
			status = "idea"
		}
		tags := in.Tags
		if tags == nil {
			tags = []string{}
		}
		row, err := q.InsertStory(ctx, gen.InsertStoryParams{
			ProjectID:      project.ID,
			ID:             id,
			Slug:           in.Slug,
			Title:          in.Title,
			ShortTitle:     in.ShortTitle,
			Synopsis:       in.Synopsis,
			TitleRenamed:   in.TitleRenamed,
			Status:         status,
			Tags:           tags,
			PublishedAt:    pgTimestamptzPtr(in.PublishedAt),
			IdeaDocumentID: ideaDoc,
		})
		if err != nil {
			return constraintErr(err)
		}
		out = storyInfo(q, ctx, row)
		return nil
	})
	return out, err
}

func pgTimestamptzPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return ts(*t)
}

// storyInfo đọc idea text kèm theo (best effort: thiếu document coi như rỗng).
func storyInfo(q *gen.Queries, ctx context.Context, row gen.Story) StoryInfo {
	info := StoryInfo{
		Slug:         row.Slug,
		Title:        row.Title,
		ShortTitle:   row.ShortTitle,
		Synopsis:     row.Synopsis,
		TitleRenamed: row.TitleRenamed,
		Status:       row.Status,
		Tags:         row.Tags,
		PublishedAt:  tsPtr(row.PublishedAt),
		CreatedAt:    row.CreatedAt.Time,
		Version:      row.Version,
	}
	if row.Tags == nil {
		info.Tags = []string{}
	}
	if row.IdeaDocumentID.Valid {
		if idea, err := q.StoryIdeaText(ctx, gen.StoryIdeaTextParams{ProjectID: row.ProjectID, ID: row.IdeaDocumentID}); err == nil {
			info.Idea = idea.String
		}
	}
	return info
}

// ListStories trả mọi truyện của project theo slug (actor phải là member trừ
// admin — đọc cũng đi qua scope).
func (n *Narrative) ListStories(ctx context.Context, projectSlug string, actor *Actor) ([]StoryInfo, error) {
	out := []StoryInfo{}
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		project, err := q.GetProjectBySlug(ctx, projectSlug)
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("dự án %s: %w", projectSlug, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		if actor != nil && !actor.isAdmin() {
			user, err := q.GetUserByUsername(ctx, actor.Username)
			if err != nil {
				if noRows(err) {
					return fmt.Errorf("dự án %s: %w", projectSlug, errs.ErrStorageNotFound)
				}
				return sanitizeErr(err)
			}
			if _, err := q.GetMemberRole(ctx, gen.GetMemberRoleParams{ProjectID: project.ID, UserID: user.ID}); err != nil {
				if noRows(err) {
					return fmt.Errorf("dự án %s: %w", projectSlug, errs.ErrStorageNotFound)
				}
				return sanitizeErr(err)
			}
		}
		rows, err := q.ListStoriesByProject(ctx, project.ID)
		if err != nil {
			return sanitizeErr(err)
		}
		for _, r := range rows {
			out = append(out, storyInfo(q, ctx, r))
		}
		return nil
	})
	return out, err
}

// GetStory đọc một truyện; thiếu → ErrStorageNotFound.
func (n *Narrative) GetStory(ctx context.Context, projectSlug, storySlug string, actor *Actor) (StoryInfo, error) {
	var info StoryInfo
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		scope, err := n.resolveStory(ctx, q, projectSlug, storySlug, actor, false)
		if err != nil {
			return err
		}
		row, err := q.GetStoryByID(ctx, gen.GetStoryByIDParams{ProjectID: scope.ProjectID, ID: scope.StoryID})
		if err != nil {
			return sanitizeErr(err)
		}
		info = storyInfo(q, ctx, row)
		return nil
	})
	return info, err
}

// UpdateStoryCAS ghi story.json fields với optimistic lock trên row version;
// expected là version đã đọc — stale → ErrStorageConflict. idea="" giữ nguyên
// document cũ; idea non-empty thay document.
func (n *Narrative) UpdateStoryCAS(ctx context.Context, projectSlug, storySlug string, actor *Actor, in StoryInfo, expected int64) (StoryInfo, error) {
	var out StoryInfo
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		scope, err := n.resolveStory(ctx, q, projectSlug, storySlug, actor, true)
		if err != nil {
			return err
		}
		row, err := q.LockStoryByID(ctx, gen.LockStoryByIDParams{ProjectID: scope.ProjectID, ID: scope.StoryID})
		if err != nil {
			if noRows(err) {
				return errs.ErrStorageNotFound
			}
			return sanitizeErr(err)
		}
		ideaDoc := row.IdeaDocumentID
		if strings.TrimSpace(in.Idea) != "" {
			docID := ideaDoc
			if !docID.Valid {
				if docID, err = randomUUID(); err != nil {
					return err
				}
			}
			if _, err := q.UpsertProjectDocument(ctx, gen.UpsertProjectDocumentParams{
				ProjectID:   scope.ProjectID,
				ID:          docID,
				Kind:        "idea",
				LogicalName: storySlug,
				TextContent: pgtype.Text{String: in.Idea, Valid: true},
			}); err != nil {
				return sanitizeErr(err)
			}
			ideaDoc = docID
		}
		slug := in.Slug
		if slug == "" {
			slug = row.Slug
		}
		status := in.Status
		if status == "" {
			status = row.Status
		}
		updated, err := q.UpdateStoryCAS(ctx, gen.UpdateStoryCASParams{
			ProjectID:      scope.ProjectID,
			ID:             scope.StoryID,
			Slug:           slug,
			Title:          in.Title,
			ShortTitle:     in.ShortTitle,
			Synopsis:       in.Synopsis,
			TitleRenamed:   in.TitleRenamed,
			Status:         status,
			Tags:           in.Tags,
			PublishedAt:    pgTimestamptzPtr(in.PublishedAt),
			IdeaDocumentID: ideaDoc,
			Version:        expected,
		})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("cập nhật truyện %s: %w", storySlug, errs.ErrStorageConflict)
			}
			return constraintErr(err)
		}
		out = storyInfo(q, ctx, updated)
		return nil
	})
	return out, err
}

// RenameStory đổi slug giữ nguyên UUID và mọi trường khác (địa chỉ ngoài đổi,
// identity/blob keys/quyền giữ); trùng slug → conflict từ UNIQUE, stale
// version → ErrStorageConflict.
func (n *Narrative) RenameStory(ctx context.Context, projectSlug, storySlug, newSlug string, actor *Actor, expected int64) error {
	return n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		scope, err := n.resolveStory(ctx, q, projectSlug, storySlug, actor, true)
		if err != nil {
			return err
		}
		row, err := q.LockStoryByID(ctx, gen.LockStoryByIDParams{ProjectID: scope.ProjectID, ID: scope.StoryID})
		if err != nil {
			if noRows(err) {
				return errs.ErrStorageNotFound
			}
			return sanitizeErr(err)
		}
		_, err = q.UpdateStoryCAS(ctx, gen.UpdateStoryCASParams{
			ProjectID:      scope.ProjectID,
			ID:             scope.StoryID,
			Slug:           newSlug,
			Title:          row.Title,
			ShortTitle:     row.ShortTitle,
			Synopsis:       row.Synopsis,
			TitleRenamed:   row.TitleRenamed,
			Status:         row.Status,
			Tags:           row.Tags,
			PublishedAt:    row.PublishedAt,
			IdeaDocumentID: row.IdeaDocumentID,
			Version:        expected,
		})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("đổi tên truyện %s: %w", storySlug, errs.ErrStorageConflict)
			}
			return constraintErr(err)
		}
		return nil
	})
}

// ── chapters / drafts / immutable versions ───────────────────────────────────

// ChapterInfo là hàng chương mức liệt kê: số, tiêu đề, revision hiện tại,
// volume (nếu có) và cờ có bản nháp.
type ChapterInfo struct {
	Number          int
	Title           string
	Draft           bool
	CurrentRevision *int
	Volume          *int
}

// ListChapters trả mọi chương của truyện theo số (không tải thân chương —
// card-level read, không copy full story).
func (n *Narrative) ListChapters(ctx context.Context, projectSlug, storySlug string, actor *Actor) ([]ChapterInfo, error) {
	out := []ChapterInfo{}
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		scope, err := n.resolveStory(ctx, q, projectSlug, storySlug, actor, false)
		if err != nil {
			return err
		}
		rows, err := q.ListChapters(ctx, gen.ListChaptersParams{ProjectID: scope.ProjectID, StoryID: scope.StoryID})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, r := range rows {
			info := ChapterInfo{Number: int(r.Number), Title: r.Title, Draft: r.DraftText != ""}
			if r.CurrentRevision.Valid {
				v := int(r.CurrentRevision.Int32)
				info.CurrentRevision = &v
			}
			if r.VolumeNumber.Valid {
				v := int(r.VolumeNumber.Int32)
				info.Volume = &v
			}
			out = append(out, info)
		}
		return nil
	})
	return out, err
}

// ChapterRef là tham chiếu chương đã resolve theo scope: id UUID (để các
// bảng domain khác như audio trỏ tới), revision hiện tại và volume.
type ChapterRef struct {
	ID              pgtype.UUID
	Number          int
	Title           string
	CurrentRevision int
	Volume          int
	Committed       bool
}

// ChapterRefByNumber resolves one chapter row inside an already-resolved
// scope. A chapter that has never existed yields Committed=false with a zero
// ID — callers that need the row (script/audio writes) must ensure it exists
// via a committed version first.
func (n *Narrative) ChapterRefByNumber(ctx context.Context, scope ScopeRef, chapter int) (ChapterRef, error) {
	row, err := n.qp().GetChapterByNumber(ctx, gen.GetChapterByNumberParams{
		ProjectID: scope.ProjectID, StoryID: scope.StoryID, Number: int32(chapter)})
	if err != nil {
		if noRows(err) {
			return ChapterRef{Number: chapter}, nil
		}
		return ChapterRef{}, sanitizeErr(err)
	}
	ref := ChapterRef{ID: row.ID, Number: int(row.Number), Title: row.Title}
	if row.CurrentRevision.Valid {
		ref.CurrentRevision = int(row.CurrentRevision.Int32)
		ref.Committed = true
	}
	if row.VolumeID.Valid {
		// volumes.number is the tap-NN the audio layer uses.
		if vol, verr := n.qp().GetVolumeByID(ctx, gen.GetVolumeByIDParams{ProjectID: scope.ProjectID, ID: row.VolumeID}); verr == nil {
			ref.Volume = int(vol.Number)
		}
	}
	return ref, nil
}

// SaveDraft ghi bản nháp đang viết của chương (tạo hàng chương nếu chưa có).
func (n *Narrative) SaveDraft(ctx context.Context, scope ScopeRef, chapter int, content string) error {
	return n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		row, err := n.ensureChapter(ctx, q, scope, chapter)
		if err != nil {
			return err
		}
		n, err := q.SaveChapterDraft(ctx, gen.SaveChapterDraftParams{
			ProjectID: scope.ProjectID,
			ID:        row.ID,
			DraftText: content,
		})
		if err != nil {
			return sanitizeErr(err)
		}
		if n == 0 {
			return fmt.Errorf("lưu nháp chương %d: %w", chapter, errs.ErrStorageConflict)
		}
		return nil
	})
}

// SaveDraftCAS ghi bản nháp chỉ khi row version khớp expected — đường
// "hai phiên sửa cùng chương": stale submit → ErrStorageConflict, không ghi đè.
func (n *Narrative) SaveDraftCAS(ctx context.Context, scope ScopeRef, chapter int, content string, expected int64) error {
	return n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		row, err := n.ensureChapter(ctx, q, scope, chapter)
		if err != nil {
			return err
		}
		tag, err := q.SaveChapterDraftCAS(ctx, gen.SaveChapterDraftCASParams{
			ProjectID: scope.ProjectID,
			ID:        row.ID,
			Version:   expected,
			DraftText: content,
		})
		if err != nil {
			return sanitizeErr(err)
		}
		if tag == 0 {
			return fmt.Errorf("lưu nháp chương %d: %w", chapter, errs.ErrStorageConflict)
		}
		return nil
	})
}

// ChapterDraftVersion trả row version hiện tại để client CAS lần ghi sau.
func (n *Narrative) ChapterDraftVersion(ctx context.Context, scope ScopeRef, chapter int) (int64, error) {
	var v int64
	err := n.tx(ctx, func(tx pgx.Tx) error {
		row, err := n.q(tx).GetChapterByNumber(ctx, gen.GetChapterByNumberParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID, Number: int32(chapter)})
		if err != nil {
			if noRows(err) {
				return errs.ErrStorageNotFound
			}
			return sanitizeErr(err)
		}
		v = row.Version
		return nil
	})
	return v, err
}

// LoadDraft đọc bản nháp; không có → "" (parity file LoadDraft).
func (n *Narrative) LoadDraft(ctx context.Context, scope ScopeRef, chapter int) (string, error) {
	var text string
	err := n.tx(ctx, func(tx pgx.Tx) error {
		row, err := n.q(tx).GetChapterByNumber(ctx, gen.GetChapterByNumberParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID, Number: int32(chapter)})
		if err != nil {
			if noRows(err) {
				text = ""
				return nil
			}
			return sanitizeErr(err)
		}
		text = row.DraftText
		return nil
	})
	return text, err
}

// ChapterText đọc chính văn hiển thị cho neo góp ý: bản nháp đang viết nếu có,
// không thì nội dung revision được nghiệm thu gần nhất.
func (n *Narrative) ChapterText(ctx context.Context, scope ScopeRef, chapter int) (string, error) {
	var text string
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		row, err := q.GetChapterByNumber(ctx, gen.GetChapterByNumberParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID, Number: int32(chapter)})
		if err != nil {
			if noRows(err) {
				text = ""
				return nil
			}
			return sanitizeErr(err)
		}
		if strings.TrimSpace(row.DraftText) != "" {
			text = row.DraftText
			return nil
		}
		v, err := q.LatestChapterVersion(ctx, gen.LatestChapterVersionParams{
			ProjectID: scope.ProjectID, ChapterID: row.ID})
		if err == nil {
			text = v.Content
			return nil
		}
		if !noRows(err) {
			return sanitizeErr(err)
		}
		text = ""
		return nil
	})
	return text, err
}

func (n *Narrative) ensureChapter(ctx context.Context, q *gen.Queries, scope ScopeRef, chapter int) (gen.Chapter, error) {
	id, err := randomUUID()
	if err != nil {
		return gen.Chapter{}, err
	}
	row, err := q.EnsureChapter(ctx, gen.EnsureChapterParams{
		ProjectID: scope.ProjectID,
		StoryID:   scope.StoryID,
		ID:        id,
		Number:    int32(chapter),
	})
	if err != nil {
		if noRows(err) {
			return gen.Chapter{}, fmt.Errorf("chương %d đã bị xoá; không thể dùng lại số chương: %w", chapter, errs.ErrStorageConflict)
		}
		return gen.Chapter{}, sanitizeErr(err)
	}
	return gen.Chapter{
		ProjectID: row.ProjectID, StoryID: row.StoryID, ID: row.ID,
		VolumeID: row.VolumeID, Number: row.Number, Title: row.Title,
		DraftText: row.DraftText, CurrentRevision: row.CurrentRevision,
		DeletedAt: row.DeletedAt, CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt, Version: row.Version,
	}, nil
}

// ChapterAuthor là tác giả được quy cho một revision nghiệm thu.
type ChapterAuthor struct {
	Kind      string // "user" | "agent" | "import"
	Username  string // Kind=="user": resolve qua users.username
	AgentName string // Kind=="agent": tên agent (writer/editor/…)
}

// AcceptRecord nghiệm thu chương: chuẩn hoá nội dung, hash canonical, append
// chapter_versions bất biến (revision kế tiếp dưới khoá hàng chương) rồi trỏ
// current_revision — một transaction, không pointer-không-version.
// Idempotent theo (digest, origin, facts, style): gọi lại cùng input trả
// revision hiện có, không append trùng.
func (n *Narrative) AcceptRecord(ctx context.Context, scope ScopeRef, chapter int, origin domain.ChapterOrigin, content string, facts domain.ChapterFacts, style domain.StyleDelta, author ChapterAuthor) (*domain.ChapterRecord, error) {
	var record *domain.ChapterRecord
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		row, err := n.ensureChapter(ctx, q, scope, chapter)
		if err != nil {
			return err
		}
		locked, err := q.LockChapterByID(ctx, gen.LockChapterByIDParams{ProjectID: scope.ProjectID, ID: row.ID})
		if err != nil {
			return sanitizeErr(err)
		}
		normalized := domain.NormalizeChapterContent(content)
		digest := domain.ChapterContentSHA256(content)
		latest, err := q.LatestChapterVersion(ctx, gen.LatestChapterVersionParams{ProjectID: scope.ProjectID, ChapterID: row.ID})
		if err != nil && !noRows(err) {
			return sanitizeErr(err)
		}
		if err == nil {
			var oldFacts domain.ChapterFacts
			var oldStyle domain.StyleDelta
			_ = json.Unmarshal(latest.Facts, &oldFacts)
			_ = json.Unmarshal(latest.StyleDelta, &oldStyle)
			facts.ForeshadowUpdates = domain.RestoreOwnPlants(oldFacts.ForeshadowUpdates, facts.ForeshadowUpdates)
			if latest.Sha256 == digest && latest.Origin == string(origin) &&
				reflect.DeepEqual(oldFacts, facts) && reflect.DeepEqual(oldStyle, style) {
				rec := chapterVersionRecord(int(locked.Number), latest)
				record = &rec
				return nil
			}
		}
		factsJSON, err := json.Marshal(facts)
		if err != nil {
			return err
		}
		styleJSON, err := json.Marshal(style)
		if err != nil {
			return err
		}
		revision := 1
		if err == nil {
			revision = int(latest.RevisionNo) + 1
		}
		var authorKind, agentName string
		var authorID pgtype.UUID
		switch author.Kind {
		case "user":
			u, err := q.GetUserByUsername(ctx, author.Username)
			if err != nil {
				if noRows(err) {
					return fmt.Errorf("tác giả %s: %w", author.Username, errs.ErrStorageNotFound)
				}
				return sanitizeErr(err)
			}
			authorKind, authorID = "user", u.ID
		case "agent":
			if strings.TrimSpace(author.AgentName) == "" {
				return fmt.Errorf("tác giả agent thiếu tên: %w", errs.ErrConfig)
			}
			authorKind, agentName = "agent", author.AgentName
		case "import":
			authorKind = "import"
		default:
			return fmt.Errorf("loại tác giả %q: %w", author.Kind, errs.ErrConfig)
		}
		ver, err := q.InsertChapterVersion(ctx, gen.InsertChapterVersionParams{
			ProjectID:    scope.ProjectID,
			ChapterID:    row.ID,
			RevisionNo:   int32(revision),
			Content:      normalized,
			Sha256:       digest,
			Origin:       string(origin),
			AuthorKind:   authorKind,
			AuthorUserID: authorID,
			AgentName:    pgtype.Text{String: agentName, Valid: agentName != ""},
			Facts:        factsJSON,
			StyleDelta:   styleJSON,
			LegacyBuild:  pgtype.Text{String: version.Build(), Valid: true},
			AcceptedAt:   ts(time.Now()),
		})
		if err != nil {
			return constraintErr(err)
		}
		if err := q.SetChapterCurrentRevision(ctx, gen.SetChapterCurrentRevisionParams{
			ProjectID: scope.ProjectID,
			ID:        row.ID,
			Revision:  pgtype.Int4{Int32: ver.RevisionNo, Valid: true},
		}); err != nil {
			return sanitizeErr(err)
		}
		// A rewrite supersedes what the reviews measured: invalidate this
		// chapter's reviews (and later ones) and their kept decisions in the
		// same transaction, so no reader sees a verdict about stale text.
		if revision > 1 {
			if err := q.InvalidateReviewsFromChapter(ctx, gen.InvalidateReviewsFromChapterParams{
				ProjectID: scope.ProjectID, StoryID: scope.StoryID, Number: int32(chapter)}); err != nil {
				return sanitizeErr(err)
			}
			if err := q.DeleteReviewDecisionsOfInvalidated(ctx, gen.DeleteReviewDecisionsOfInvalidatedParams{
				ProjectID: scope.ProjectID, StoryID: scope.StoryID}); err != nil {
				return sanitizeErr(err)
			}
		}
		rec := chapterVersionRecord(chapter, ver)
		record = &rec
		return nil
	})
	return record, err
}

func chapterVersionRecord(chapter int, v gen.ChapterVersion) domain.ChapterRecord {
	rec := domain.ChapterRecord{
		Version:       domain.ChapterRecordVersion,
		Chapter:       chapter,
		Revision:      int(v.RevisionNo),
		Build:         v.LegacyBuild.String,
		Origin:        domain.ChapterOrigin(v.Origin),
		Content:       v.Content,
		ContentSHA256: v.Sha256,
		AcceptedAt:    v.AcceptedAt.Time,
	}
	_ = json.Unmarshal(v.Facts, &rec.Facts)
	_ = json.Unmarshal(v.StyleDelta, &rec.StyleDelta)
	return rec
}

// LoadRecord đọc revision hiện hành (bản ghi nghiệm thu) của chương; chưa
// nghiệm thu → (nil, nil) — parity file ChapterRecords.Load.
func (n *Narrative) LoadRecord(ctx context.Context, scope ScopeRef, chapter int) (*domain.ChapterRecord, error) {
	var rec *domain.ChapterRecord
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		row, err := q.GetChapterByNumber(ctx, gen.GetChapterByNumberParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID, Number: int32(chapter)})
		if err != nil {
			if noRows(err) {
				return nil
			}
			return sanitizeErr(err)
		}
		v, err := q.LatestChapterVersion(ctx, gen.LatestChapterVersionParams{ProjectID: scope.ProjectID, ChapterID: row.ID})
		if err != nil {
			if noRows(err) {
				return nil
			}
			return sanitizeErr(err)
		}
		r := chapterVersionRecord(chapter, v)
		rec = &r
		return nil
	})
	return rec, err
}

// VersionHistory liệt kê các revision đã nghiệm thu mới → cũ (keyset:
// revision_no < before) — lịch sử chỉ đọc, không bao giờ mutate.
func (n *Narrative) VersionHistory(ctx context.Context, scope ScopeRef, chapter, before, limit int) ([]domain.ChapterRecord, error) {
	out := []domain.ChapterRecord{}
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		row, err := q.GetChapterByNumber(ctx, gen.GetChapterByNumberParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID, Number: int32(chapter)})
		if err != nil {
			if noRows(err) {
				return nil
			}
			return sanitizeErr(err)
		}
		if limit <= 0 {
			limit = 50
		}
		if before <= 0 {
			before = math.MaxInt32
		}
		rows, err := q.ListChapterVersions(ctx, gen.ListChapterVersionsParams{
			ProjectID:  scope.ProjectID,
			ChapterID:  row.ID,
			RevisionNo: int32(before),
			Limit:      int32(limit),
		})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, v := range rows {
			out = append(out, chapterVersionRecord(chapter, v))
		}
		return nil
	})
	return out, err
}

// ── outlines ─────────────────────────────────────────────────────────────────

func (n *Narrative) loadOutlinePayload(ctx context.Context, q *gen.Queries, scope ScopeRef, kind string, dst any) (bool, error) {
	row, err := q.GetOutline(ctx, gen.GetOutlineParams{ProjectID: scope.ProjectID, StoryID: scope.StoryID, Kind: kind})
	if err != nil {
		if noRows(err) {
			return false, nil
		}
		return false, sanitizeErr(err)
	}
	if dst == nil {
		return true, nil
	}
	if err := json.Unmarshal(row.Payload, dst); err != nil {
		return false, fmt.Errorf("đọc outline %s: %w", kind, err)
	}
	return true, nil
}

func (n *Narrative) upsertOutline(ctx context.Context, q *gen.Queries, scope ScopeRef, kind string, payload []byte, text *string) error {
	var tc pgtype.Text
	if text != nil {
		tc = pgtype.Text{String: *text, Valid: true}
	}
	if err := q.UpsertOutline(ctx, gen.UpsertOutlineParams{
		ProjectID:   scope.ProjectID,
		StoryID:     scope.StoryID,
		Kind:        kind,
		Payload:     payload,
		TextContent: tc,
	}); err != nil {
		return sanitizeErr(err)
	}
	return nil
}

// SaveFlatOutline/L ưu/đọc outline phẳng (kind=flat) — entries dạng JSON array.
func (n *Narrative) SaveFlatOutline(ctx context.Context, scope ScopeRef, entries []domain.OutlineEntry) error {
	payload, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return n.tx(ctx, func(tx pgx.Tx) error {
		return n.upsertOutline(ctx, n.q(tx), scope, "flat", payload, nil)
	})
}

func (n *Narrative) LoadFlatOutline(ctx context.Context, scope ScopeRef) ([]domain.OutlineEntry, error) {
	var entries []domain.OutlineEntry
	err := n.tx(ctx, func(tx pgx.Tx) error {
		found, err := n.loadOutlinePayload(ctx, n.q(tx), scope, "flat", &entries)
		if err != nil {
			return err
		}
		if !found {
			entries = nil
		}
		return nil
	})
	return entries, err
}

// SaveLayeredOutline ghi cấu trúc tầng: outlines kind=layered giữ payload đầy
// đủ và mỗi VolumeOutline cũng đồng bộ sang bảng volumes (số → title+outline)
// để composite FK chapter→volume hoạt động.
func (n *Narrative) SaveLayeredOutline(ctx context.Context, scope ScopeRef, volumes []domain.VolumeOutline) error {
	payload, err := json.Marshal(volumes)
	if err != nil {
		return err
	}
	return n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		if err := n.upsertOutline(ctx, q, scope, "layered", payload, nil); err != nil {
			return err
		}
		if err := q.DeleteVolumesForStory(ctx, gen.DeleteVolumesForStoryParams{ProjectID: scope.ProjectID, StoryID: scope.StoryID}); err != nil {
			return sanitizeErr(err)
		}
		for _, v := range volumes {
			vid, err := randomUUID()
			if err != nil {
				return err
			}
			outlineJSON, err := json.Marshal(v)
			if err != nil {
				return err
			}
			if err := q.UpsertVolume(ctx, gen.UpsertVolumeParams{
				ProjectID: scope.ProjectID,
				StoryID:   scope.StoryID,
				ID:        vid,
				Number:    int32(v.Index),
				Title:     v.Title,
				Outline:   outlineJSON,
			}); err != nil {
				return sanitizeErr(err)
			}
		}
		return nil
	})
}

func (n *Narrative) LoadLayeredOutline(ctx context.Context, scope ScopeRef) ([]domain.VolumeOutline, error) {
	var volumes []domain.VolumeOutline
	err := n.tx(ctx, func(tx pgx.Tx) error {
		found, err := n.loadOutlinePayload(ctx, n.q(tx), scope, "layered", &volumes)
		if err != nil {
			return err
		}
		if !found {
			volumes = nil
		}
		return nil
	})
	return volumes, err
}

// ClearLayeredOutline xoá cả payload tầng và hàng volumes mirror.
func (n *Narrative) ClearLayeredOutline(ctx context.Context, scope ScopeRef) error {
	return n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		if err := q.DeleteOutline(ctx, gen.DeleteOutlineParams{ProjectID: scope.ProjectID, StoryID: scope.StoryID, Kind: "layered"}); err != nil {
			return sanitizeErr(err)
		}
		return sanitizeErr(q.DeleteVolumesForStory(ctx, gen.DeleteVolumesForStoryParams{ProjectID: scope.ProjectID, StoryID: scope.StoryID}))
	})
}

// SetChapterVolume gán chương vào một volume đã tồn tại (composite FK từ chối
// volume thuộc story/project khác — kiểm chéo ở test).
func (n *Narrative) SetChapterVolume(ctx context.Context, scope ScopeRef, chapter, volume int) error {
	return n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		vol, err := q.GetVolumeByNumber(ctx, gen.GetVolumeByNumberParams{ProjectID: scope.ProjectID, StoryID: scope.StoryID, Number: int32(volume)})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("tập %d: %w", volume, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		tag, err := q.SetChapterVolume(ctx, gen.SetChapterVolumeParams{
			ProjectID: scope.ProjectID,
			StoryID:   scope.StoryID,
			Number:    int32(chapter),
			VolumeID:  vol.ID,
		})
		if err != nil {
			return sanitizeErr(err)
		}
		if tag == 0 {
			return fmt.Errorf("chương %d: %w", chapter, errs.ErrStorageNotFound)
		}
		return nil
	})
}

// SavePremise/LoadPremise — premise lưu text thuần trong outlines.premise.
func (n *Narrative) SavePremise(ctx context.Context, scope ScopeRef, content string) error {
	return n.tx(ctx, func(tx pgx.Tx) error {
		return n.upsertOutline(ctx, n.q(tx), scope, "premise", []byte(`{}`), &content)
	})
}

func (n *Narrative) LoadPremise(ctx context.Context, scope ScopeRef) (string, error) {
	var text string
	err := n.tx(ctx, func(tx pgx.Tx) error {
		row, err := n.q(tx).GetOutline(ctx, gen.GetOutlineParams{ProjectID: scope.ProjectID, StoryID: scope.StoryID, Kind: "premise"})
		if err != nil {
			if noRows(err) {
				return nil
			}
			return sanitizeErr(err)
		}
		text = row.TextContent.String
		return nil
	})
	return text, err
}

// SaveCompass/LoadCompass và SaveRetention/LoadRetention — các mảng
// kind=compass/retention của outlines.
func (n *Narrative) SaveCompass(ctx context.Context, scope ScopeRef, compass domain.StoryCompass) error {
	payload, err := json.Marshal(compass)
	if err != nil {
		return err
	}
	return n.tx(ctx, func(tx pgx.Tx) error {
		return n.upsertOutline(ctx, n.q(tx), scope, "compass", payload, nil)
	})
}

func (n *Narrative) LoadCompass(ctx context.Context, scope ScopeRef) (*domain.StoryCompass, error) {
	var out *domain.StoryCompass
	err := n.tx(ctx, func(tx pgx.Tx) error {
		var c domain.StoryCompass
		found, err := n.loadOutlinePayload(ctx, n.q(tx), scope, "compass", &c)
		if err != nil {
			return err
		}
		if found {
			out = &c
		}
		return nil
	})
	return out, err
}

func (n *Narrative) SaveRetention(ctx context.Context, scope ScopeRef, fields domain.StoryRetention) error {
	payload, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return n.tx(ctx, func(tx pgx.Tx) error {
		return n.upsertOutline(ctx, n.q(tx), scope, "retention", payload, nil)
	})
}

func (n *Narrative) LoadRetention(ctx context.Context, scope ScopeRef) (domain.StoryRetention, error) {
	var out domain.StoryRetention
	err := n.tx(ctx, func(tx pgx.Tx) error {
		_, err := n.loadOutlinePayload(ctx, n.q(tx), scope, "retention", &out)
		return err
	})
	return out, err
}

// ── characters ───────────────────────────────────────────────────────────────

// SaveCharacters thay toàn bộ bộ nhân vật trong một transaction — parity
// file CharacterStore.Save (ghi đè characters.json). legacy_id = Character.ID
// nếu khai, không thì name.
func (n *Narrative) SaveCharacters(ctx context.Context, scope ScopeRef, chars []domain.Character) error {
	return n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		if err := q.DeleteCharactersForStory(ctx, gen.DeleteCharactersForStoryParams{ProjectID: scope.ProjectID, StoryID: scope.StoryID}); err != nil {
			return sanitizeErr(err)
		}
		for _, c := range chars {
			id, err := randomUUID()
			if err != nil {
				return err
			}
			payload, err := json.Marshal(c)
			if err != nil {
				return err
			}
			legacy := strings.TrimSpace(c.ID)
			if legacy == "" {
				legacy = c.Name
			}
			if err := q.InsertCharacter(ctx, gen.InsertCharacterParams{
				ProjectID: scope.ProjectID,
				StoryID:   scope.StoryID,
				ID:        id,
				LegacyID:  legacy,
				Name:      c.Name,
				Payload:   payload,
			}); err != nil {
				return sanitizeErr(err)
			}
		}
		return nil
	})
}

// LoadCharacters đọc toàn bộ nhân vật theo tên (thứ tự ổn định).
func (n *Narrative) LoadCharacters(ctx context.Context, scope ScopeRef) ([]domain.Character, error) {
	out := []domain.Character{}
	err := n.tx(ctx, func(tx pgx.Tx) error {
		rows, err := n.q(tx).ListCharacters(ctx, gen.ListCharactersParams{ProjectID: scope.ProjectID, StoryID: scope.StoryID})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, r := range rows {
			var c domain.Character
			if err := json.Unmarshal(r.Payload, &c); err != nil {
				return fmt.Errorf("đọc nhân vật %s: %w", r.Name, err)
			}
			out = append(out, c)
		}
		return nil
	})
	return out, err
}

// ── search ───────────────────────────────────────────────────────────────────

// SearchHit là một kết quả tìm kiếm trong phạm vi project.
type SearchHit struct {
	Kind     string // "story" | "chapter_title" | "chapter_content" | "character"
	Story    string // story slug (rỗng với kind=story: slug nằm trong Ref)
	Ref      string // story slug | chapter "NN" | character legacy id
	Title    string
	Snippet  string
	Revision *int
	Score    float64
}

// Search tìm theo trigram + substring, mọi truy vấn bắt scope project (và
// deleted flags) trước LIMIT. Không hứa bỏ dấu: ILIKE/% giữ nguyên dấu.
func (n *Narrative) Search(ctx context.Context, projectSlug, query string, actor *Actor, limit int) ([]SearchHit, error) {
	out := []SearchHit{}
	if limit <= 0 {
		limit = 20
	}
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		project, err := q.GetProjectBySlug(ctx, projectSlug)
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("dự án %s: %w", projectSlug, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		if actor != nil && !actor.isAdmin() {
			user, err := q.GetUserByUsername(ctx, actor.Username)
			if err != nil {
				if noRows(err) {
					return fmt.Errorf("dự án %s: %w", projectSlug, errs.ErrStorageNotFound)
				}
				return sanitizeErr(err)
			}
			if _, err := q.GetMemberRole(ctx, gen.GetMemberRoleParams{ProjectID: project.ID, UserID: user.ID}); err != nil {
				if noRows(err) {
					return fmt.Errorf("dự án %s: %w", projectSlug, errs.ErrStorageNotFound)
				}
				return sanitizeErr(err)
			}
		}
		stories, err := q.SearchStoryTitles(ctx, gen.SearchStoryTitlesParams{ProjectID: project.ID, Similarity: query, Limit: int32(limit)})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, r := range stories {
			out = append(out, SearchHit{Kind: "story", Ref: r.Slug, Title: r.Title, Score: float64(r.Score)})
		}
		// Per-story scoped chapter/character queries: project predicate stays on
		// every row before LIMIT (per query WHERE clauses).
		storyRows, err := q.ListStoriesByProject(ctx, project.ID)
		if err != nil {
			return sanitizeErr(err)
		}
		for _, s := range storyRows {
			titles, err := q.SearchChapterTitles(ctx, gen.SearchChapterTitlesParams{ProjectID: project.ID, StoryID: s.ID, Similarity: query, Limit: int32(limit)})
			if err != nil {
				return sanitizeErr(err)
			}
			for _, r := range titles {
				out = append(out, SearchHit{Kind: "chapter_title", Story: s.Slug, Ref: fmt.Sprintf("%02d", r.Chapter), Title: r.Title, Score: float64(r.Score)})
			}
			vers, err := q.SearchChapterVersions(ctx, gen.SearchChapterVersionsParams{ProjectID: project.ID, StoryID: s.ID, Similarity: query, Limit: int32(limit)})
			if err != nil {
				return sanitizeErr(err)
			}
			for _, r := range vers {
				rev := int(r.RevisionNo)
				out = append(out, SearchHit{Kind: "chapter_content", Story: s.Slug, Ref: fmt.Sprintf("%02d", r.Chapter), Snippet: r.Snippet, Revision: &rev, Score: float64(r.Score)})
			}
			chars, err := q.SearchCharacters(ctx, gen.SearchCharactersParams{ProjectID: project.ID, StoryID: s.ID, Similarity: query, Limit: int32(limit)})
			if err != nil {
				return sanitizeErr(err)
			}
			for _, r := range chars {
				out = append(out, SearchHit{Kind: "character", Story: s.Slug, Ref: r.LegacyID, Title: r.Name, Score: float64(r.Score)})
			}
		}
		return nil
	})
	return out, err
}

// ── story lease ──────────────────────────────────────────────────────────────

// ReleaseLease trả lease về trống khi còn đúng holder.
func (n *Narrative) ReleaseLease(ctx context.Context, scope ScopeRef, holder pgtype.UUID) error {
	return n.tx(ctx, func(tx pgx.Tx) error {
		_, err := n.q(tx).ReleaseStoryLease(ctx, gen.ReleaseStoryLeaseParams{
			ProjectID: scope.ProjectID,
			StoryID:   scope.StoryID,
			HolderID:  holder,
		})
		return sanitizeErr(err)
	})
}

// Pool exposes the shared pool for the sibling repositories (comments,
// reviews, story_state helpers) that bind the same scope.
func (n *Narrative) Pool() *pgxpool.Pool { return n.pool }

// Comments binds a comments.Repository to an already-resolved scope.
func (n *Narrative) Comments(scope ScopeRef) comments.Repository {
	return &pgComments{pool: n.pool, scope: scope}
}

// RepliesStore là bề mặt comment_replies — bảng sẵn sàng cho API reply
// tường minh; facade comments chưa mở UI reply nên nó chỉ phục vụ
// import/endpoint tương lai, không suy ra nghiệp vụ mới.
type RepliesStore interface {
	AddReply(commentID, body, author string) error
	Replies(commentID string) ([]comments.Comment, error)
}

// Replies binds the reply store to an already-resolved scope.
func (n *Narrative) Replies(scope ScopeRef) RepliesStore {
	return &pgComments{pool: n.pool, scope: scope}
}

// CommentsStore builds the full comments.Store facade on this scope — the
// business rules (validate/lifecycle/anchors) are identical to file mode.
func (n *Narrative) CommentsStore(scope ScopeRef) *comments.Store {
	return comments.NewFromRepository(n.Comments(scope))
}
