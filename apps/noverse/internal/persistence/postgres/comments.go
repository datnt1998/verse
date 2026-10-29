package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/noverse/internal/comments"
	"github.com/datnt1998/noverse/internal/errs"
	"github.com/datnt1998/noverse/internal/persistence/postgres/gen"
)

// pgComments là comments.Repository trên bảng comments/comment_replies trong
// phạm vi (project, story) đã resolve. Mọi mutation chạy một transaction:
// khoá hàng chương → load → migrate nghiệp vụ (hàm exported của package
// comments, cùng luật với file backend) → fn → persist diff với CAS trên row
// version. Danh sách được thay nguyên tử — bản ghi bị fn bỏ ra khỏi danh sách
// chuyển soft-delete chứ không biến mất khỏi lịch sử.
type pgComments struct {
	pool  *pgxpool.Pool
	scope ScopeRef
}

var _ comments.Repository = (*pgComments)(nil)

func commentFromRow(r gen.Comment) comments.Comment {
	c := comments.Comment{
		ID:         r.ID,
		Quote:      r.OriginalQuote,
		Chips:      r.Chips,
		Text:       r.Body,
		CreatedAt:  r.CreatedAt.Time.UTC(),
		Status:     r.Status,
		SentAt:     tsPtr(r.SentAt),
		FixedAt:    tsPtr(r.FixedAt),
		ResolvedAt: tsPtr(r.ResolvedAt),
		Unfixed:    r.Unfixed,
		Moved:      r.Moved,
		Relinked:   r.Relinked,
		Whole:      r.Whole,
		NewQuote:   r.NewQuote,
	}
	if r.Chips == nil {
		c.Chips = []string{}
	}
	if r.OriginalParagraph.Valid {
		p := int(r.OriginalParagraph.Int32)
		c.Paragraph = &p
	}
	if r.LegacyAuthorName.Valid {
		c.Author = r.LegacyAuthorName.String
	} else if r.AuthorUserID.Valid {
		// username lookup happens once per list load; see loadList.
		c.Author = "__user:" + uuidString(r.AuthorUserID)
	}
	if r.FixedBy.Valid {
		c.FixedBy = r.FixedBy.String
	}
	if len(r.Anchor) > 0 {
		var a comments.Anchor
		if err := json.Unmarshal(r.Anchor, &a); err == nil {
			c.Anchor = &a
		}
	}
	if len(r.Candidates) > 0 {
		var cands []comments.Candidate
		if err := json.Unmarshal(r.Candidates, &cands); err == nil {
			c.Candidates = cands
		}
	}
	return c
}

// uuidString keeps the historical call sites readable; formatting lives in
// UUIDString so there is one implementation.
func uuidString(u pgtype.UUID) string { return UUIDString(u) }

// loadList đọc danh sách góp ý của chương + resolve tên người dùng cho các
// row author_user_id (batched — không N+1).
func (c *pgComments) loadList(ctx context.Context, q *gen.Queries, chapter int, forUpdate bool) ([]comments.Comment, map[string]int64, error) {
	var rows []gen.Comment
	var err error
	if forUpdate {
		rows, err = q.LockCommentsByChapter(ctx, gen.LockCommentsByChapterParams{ProjectID: c.scope.ProjectID, StoryID: c.scope.StoryID, Number: int32(chapter)})
	} else {
		rows, err = q.ListCommentsByChapter(ctx, gen.ListCommentsByChapterParams{ProjectID: c.scope.ProjectID, StoryID: c.scope.StoryID, Number: int32(chapter)})
	}
	if err != nil {
		return nil, nil, sanitizeErr(err)
	}
	// user id → username, one query per distinct author id is bounded by the
	// comment count; keep it simple and correct.
	userNames := map[pgtype.UUID]string{}
	versions := map[string]int64{}
	list := make([]comments.Comment, 0, len(rows))
	for _, r := range rows {
		versions[r.ID] = r.Version
		cm := commentFromRow(r)
		if r.AuthorUserID.Valid {
			if name, ok := userNames[r.AuthorUserID]; ok {
				cm.Author = name
			} else {
				if u, err := q.GetUserByID(ctx, r.AuthorUserID); err == nil {
					userNames[r.AuthorUserID] = u.Username
					cm.Author = u.Username
				}
			}
		}
		list = append(list, cm)
	}
	return list, versions, nil
}

// persistList writes the post-fn list inside the tx: upsert survivors with
// CAS on the row version read earlier (comment versions are 1 + per-update
// bumps), insert new ids, soft-delete missing ids. A stale concurrent write
// surfaces ErrStorageConflict through the CAS guard, never a silent overwrite.
func (c *pgComments) persistList(ctx context.Context, q *gen.Queries, chapter int, chapterID pgtype.UUID, before map[string]int64, list []comments.Comment) error {
	seen := map[string]bool{}
	for _, cm := range list {
		seen[cm.ID] = true
		params, err := c.rowForComment(ctx, q, chapterID, cm, before[cm.ID])
		if err != nil {
			return err
		}
		if _, ok := before[cm.ID]; ok {
			affected, err := q.UpdateCommentCAS(ctx, params.update())
			if err != nil {
				return sanitizeErr(err)
			}
			if affected == 0 {
				return errs.ErrStorageConflict
			}
			continue
		}
		if err := q.InsertComment(ctx, params.insert()); err != nil {
			return constraintErr(err)
		}
	}
	for id := range before {
		if seen[id] {
			continue
		}
		if _, err := q.SoftDeleteComment(ctx, gen.SoftDeleteCommentParams{ProjectID: c.scope.ProjectID, ID: id}); err != nil {
			return sanitizeErr(err)
		}
	}
	return nil
}

// pgCommentRow bundles the column values shared by insert/update params.
type pgCommentRow struct {
	ProjectID         pgtype.UUID
	StoryID           pgtype.UUID
	ID                string
	Version           int64
	ChapterID         pgtype.UUID
	AuthorUserID      pgtype.UUID
	LegacyAuthorName  pgtype.Text
	Body              string
	OriginalParagraph pgtype.Int4
	OriginalQuote     string
	Chips             []string
	Anchor            []byte
	AnchorRevision    pgtype.Int4
	Candidates        []byte
	Status            string
	FixedBy           pgtype.Text
	SentAt            pgtype.Timestamptz
	FixedAt           pgtype.Timestamptz
	ResolvedAt        pgtype.Timestamptz
	Unfixed           bool
	Moved             bool
	Relinked          bool
	Whole             bool
	NewQuote          string
}

func (r pgCommentRow) insert() gen.InsertCommentParams {
	return gen.InsertCommentParams{
		ProjectID: r.ProjectID, StoryID: r.StoryID, ID: r.ID, ChapterID: r.ChapterID,
		AuthorUserID: r.AuthorUserID, LegacyAuthorName: r.LegacyAuthorName,
		Body: r.Body, OriginalParagraph: r.OriginalParagraph, OriginalQuote: r.OriginalQuote,
		Chips: r.Chips, Anchor: r.Anchor, AnchorRevision: r.AnchorRevision,
		Candidates: r.Candidates, Status: r.Status, FixedBy: r.FixedBy,
		SentAt: r.SentAt, FixedAt: r.FixedAt, ResolvedAt: r.ResolvedAt,
		Unfixed: r.Unfixed, Moved: r.Moved, Relinked: r.Relinked,
		Whole: r.Whole, NewQuote: r.NewQuote,
	}
}

func (r pgCommentRow) update() gen.UpdateCommentCASParams {
	return gen.UpdateCommentCASParams{
		ProjectID: r.ProjectID, ID: r.ID, Version: r.Version,
		AuthorUserID: r.AuthorUserID, LegacyAuthorName: r.LegacyAuthorName,
		Body: r.Body, OriginalParagraph: r.OriginalParagraph, OriginalQuote: r.OriginalQuote,
		Chips: r.Chips, Anchor: r.Anchor, AnchorRevision: r.AnchorRevision,
		Candidates: r.Candidates, Status: r.Status, FixedBy: r.FixedBy,
		SentAt: r.SentAt, FixedAt: r.FixedAt, ResolvedAt: r.ResolvedAt,
		Unfixed: r.Unfixed, Moved: r.Moved, Relinked: r.Relinked,
		Whole: r.Whole, NewQuote: r.NewQuote,
	}
}

func (c *pgComments) rowForComment(ctx context.Context, q *gen.Queries, chapterID pgtype.UUID, cm comments.Comment, expected int64) (*pgCommentRow, error) {
	row := &pgCommentRow{
		ProjectID:     c.scope.ProjectID,
		StoryID:       c.scope.StoryID,
		ID:            cm.ID,
		Version:       expected,
		ChapterID:     chapterID,
		Body:          cm.Text,
		OriginalQuote: cm.Quote,
		Chips:         cm.Chips,
		Status:        cm.Status,
		SentAt:        pgTimestamptzPtr(cm.SentAt),
		FixedAt:       pgTimestamptzPtr(cm.FixedAt),
		ResolvedAt:    pgTimestamptzPtr(cm.ResolvedAt),
		Unfixed:       cm.Unfixed,
		Moved:         cm.Moved,
		Relinked:      cm.Relinked,
		Whole:         cm.Whole,
		NewQuote:      cm.NewQuote,
	}
	if cm.Chips == nil {
		row.Chips = []string{}
	}
	if cm.Paragraph != nil {
		row.OriginalParagraph = pgtype.Int4{Int32: int32(*cm.Paragraph), Valid: true}
	}
	uid, legacy, err := narrativeAuthor(ctx, q, cm.Author)
	if err != nil {
		return nil, err
	}
	row.AuthorUserID = uid
	row.LegacyAuthorName = legacy
	if cm.FixedBy != "" {
		row.FixedBy = pgtype.Text{String: cm.FixedBy, Valid: true}
	}
	if cm.Anchor != nil {
		data, err := json.Marshal(cm.Anchor)
		if err != nil {
			return nil, err
		}
		row.Anchor = data
	}
	if len(cm.Candidates) > 0 {
		data, err := json.Marshal(cm.Candidates)
		if err != nil {
			return nil, err
		}
		row.Candidates = data
	} else {
		row.Candidates = []byte("[]")
	}
	return row, nil
}

// narrativeAuthor resolves a comments.Author string to (user id | legacy name)
// exactly like Narrative.authorUser but usable without a *Narrative instance.
func narrativeAuthor(ctx context.Context, q *gen.Queries, author string) (pgtype.UUID, pgtype.Text, error) {
	var uid pgtype.UUID
	if author != "" {
		if u, err := q.GetUserByUsername(ctx, author); err == nil {
			return u.ID, pgtype.Text{}, nil
		} else if !noRows(err) {
			return uid, pgtype.Text{}, sanitizeErr(err)
		}
	}
	name := author
	if name == "" {
		name = "khách"
	}
	return uid, pgtype.Text{String: name, Valid: true}, nil
}

// Transact khoá danh sách chương trong một tx, migrate trên chính văn hiện
// tại (đọc lười khi có góp ý neo được), chạy fn rồi persist khi changed hoặc
// migrate đã sửa dữ liệu — nguyên tử, không có bản ghi lệch.
func (c *pgComments) Transact(chapter int, fn func([]comments.Comment) ([]comments.Comment, bool, error)) error {
	return RunInTx(context.Background(), c.pool, func(tx pgx.Tx) error {
		q := gen.New(tx)
		chRow, err := q.GetChapterByNumber(context.Background(), gen.GetChapterByNumberParams{
			ProjectID: c.scope.ProjectID, StoryID: c.scope.StoryID, Number: int32(chapter)})
		if err != nil && !noRows(err) {
			return sanitizeErr(err)
		}
		list, before, err := c.loadList(context.Background(), q, chapter, true)
		if err != nil {
			return err
		}
		var paras []string
		needText := false
		for _, cm := range list {
			if comments.AnchorEligible(cm) && cm.Status != comments.StatusDone {
				needText = true
				break
			}
		}
		if needText {
			if text, err := c.chapterTextTx(context.Background(), q, chapter, chRow); err == nil && text != "" {
				paras = comments.NormalizedParagraphs(text)
			}
		}
		migrated, changedByMigrate := comments.MigrateStored(list, paras, time.Now().UTC())
		out, changed, err := fn(migrated)
		if err != nil {
			return err
		}
		if !changed && !changedByMigrate {
			return nil
		}
		var chapterID pgtype.UUID
		if chRow.ID.Valid {
			chapterID = chRow.ID
		} else {
			// no chapter row yet → create it so the composite FK holds
			// (file mode stores a comment file without a chapter file).
			created, err := c.ensureChapterTx(context.Background(), q, chapter)
			if err != nil {
				return err
			}
			chapterID = created
		}
		return c.persistList(context.Background(), q, chapter, chapterID, before, out)
	})
}

// TransactAt migrate KHÔNG giải mốc neo (đường ghi chương tự xử lý) — persist
// chỉ khi fn báo changed.
func (c *pgComments) TransactAt(chapter int, _ []string, fn func([]comments.Comment) ([]comments.Comment, bool, error)) error {
	return RunInTx(context.Background(), c.pool, func(tx pgx.Tx) error {
		q := gen.New(tx)
		chRow, err := q.GetChapterByNumber(context.Background(), gen.GetChapterByNumberParams{
			ProjectID: c.scope.ProjectID, StoryID: c.scope.StoryID, Number: int32(chapter)})
		if err != nil && !noRows(err) {
			return sanitizeErr(err)
		}
		list, before, err := c.loadList(context.Background(), q, chapter, true)
		if err != nil {
			return err
		}
		migrated, _ := comments.MigrateStored(list, nil, time.Now().UTC())
		out, changed, err := fn(migrated)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		var chapterID pgtype.UUID
		if chRow.ID.Valid {
			chapterID = chRow.ID
		} else {
			created, err := c.ensureChapterTx(context.Background(), q, chapter)
			if err != nil {
				return err
			}
			chapterID = created
		}
		return c.persistList(context.Background(), q, chapter, chapterID, before, out)
	})
}

// chapterTextTx: bản nháp đang viết nếu có, không thì revision mới nhất —
// trên chapter row đã khoá/đọc trong tx.
func (c *pgComments) chapterTextTx(ctx context.Context, q *gen.Queries, chapter int, chRow gen.Chapter) (string, error) {
	if chRow.ID.Valid {
		if chRow.DraftText != "" {
			return chRow.DraftText, nil
		}
		v, err := q.LatestChapterVersion(ctx, gen.LatestChapterVersionParams{
			ProjectID: c.scope.ProjectID, ChapterID: chRow.ID})
		if err == nil {
			return v.Content, nil
		}
		if !noRows(err) {
			return "", sanitizeErr(err)
		}
	}
	return "", nil
}

// ChapterText đọc chính văn để giải mốc neo (đường đọc/tạo/gắn lại).
func (c *pgComments) ChapterText(chapter int) (string, error) {
	var text string
	err := RunInTx(context.Background(), c.pool, func(tx pgx.Tx) error {
		q := gen.New(tx)
		row, err := q.GetChapterByNumber(context.Background(), gen.GetChapterByNumberParams{
			ProjectID: c.scope.ProjectID, StoryID: c.scope.StoryID, Number: int32(chapter)})
		if err != nil && !noRows(err) {
			return sanitizeErr(err)
		}
		var e error
		text, e = c.chapterTextTx(context.Background(), q, chapter, row)
		return e
	})
	return text, err
}

func (c *pgComments) Chapters() ([]int, error) {
	var out []int
	err := RunInTx(context.Background(), c.pool, func(tx pgx.Tx) error {
		rows, err := gen.New(tx).CommentChapters(context.Background(), gen.CommentChaptersParams{
			ProjectID: c.scope.ProjectID, StoryID: c.scope.StoryID})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, n := range rows {
			out = append(out, int(n))
		}
		return nil
	})
	return out, err
}

func (c *pgComments) ReadRaw(chapter int) ([]comments.Comment, error) {
	var list []comments.Comment
	err := RunInTx(context.Background(), c.pool, func(tx pgx.Tx) error {
		var err error
		list, _, err = c.loadList(context.Background(), gen.New(tx), chapter, false)
		return err
	})
	return list, err
}

// WithEvents chạy fn trên hàng đợi sự kiện chờ phát — lưu ở story_state
// kind=comment_events (typed singleton, mutable bằng CAS nội bộ upsert).
// DrainEvents trả rỗng sau khi xoá — parity file _events.json.
func (c *pgComments) WithEvents(fn func([]comments.ReanchorEvent) []comments.ReanchorEvent) error {
	return RunInTx(context.Background(), c.pool, func(tx pgx.Tx) error {
		q := gen.New(tx)
		events := []comments.ReanchorEvent{}
		row, err := q.GetStoryState(context.Background(), gen.GetStoryStateParams{
			ProjectID: c.scope.ProjectID, StoryID: c.scope.StoryID, Kind: "comment_events"})
		if err == nil {
			_ = json.Unmarshal(row.Payload, &events)
		} else if !noRows(err) {
			return sanitizeErr(err)
		}
		events = fn(events)
		payload, err := json.Marshal(events)
		if err != nil {
			return err
		}
		return sanitizeErr(q.UpsertStoryState(context.Background(), gen.UpsertStoryStateParams{
			ProjectID:     c.scope.ProjectID,
			StoryID:       c.scope.StoryID,
			Kind:          "comment_events",
			Payload:       payload,
			SchemaVersion: 1,
		}))
	})
}

// AddReply lưu một reply tường minh (bảng comment_replies — dành cho API tương
// lai; facade hiện chưa mở thao tác reply).
func (c *pgComments) AddReply(commentID, body, author string) error {
	return RunInTx(context.Background(), c.pool, func(tx pgx.Tx) error {
		q := gen.New(tx)
		uid, legacy, err := narrativeAuthor(context.Background(), q, author)
		if err != nil {
			return err
		}
		return constraintErr(q.InsertCommentReply(context.Background(), gen.InsertCommentReplyParams{
			ProjectID:        c.scope.ProjectID,
			CommentID:        commentID,
			ID:               randomTextID(),
			AuthorUserID:     uid,
			LegacyAuthorName: legacy,
			Body:             body,
		}))
	})
}

// Replies liệt kê reply của một góp ý theo thứ tự tạo.
func (c *pgComments) Replies(commentID string) ([]comments.Comment, error) {
	var out []comments.Comment
	err := RunInTx(context.Background(), c.pool, func(tx pgx.Tx) error {
		rows, err := gen.New(tx).ListCommentReplies(context.Background(), gen.ListCommentRepliesParams{
			ProjectID: c.scope.ProjectID, CommentID: commentID})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, r := range rows {
			c := comments.Comment{ID: r.ID, Text: r.Body, CreatedAt: r.CreatedAt.Time.UTC(), Status: comments.StatusDone}
			if r.LegacyAuthorName.Valid {
				c.Author = r.LegacyAuthorName.String
			}
			out = append(out, c)
		}
		return nil
	})
	return out, err
}

func randomTextID() string {
	id, err := randomUUID()
	if err != nil {
		panic(err)
	}
	return uuidString(id)
}

// ensureChapterTx creates the chapter row the comments FK needs, inside the
// open transaction — mirrors Narrative.ensureChapter.
func (c *pgComments) ensureChapterTx(ctx context.Context, q *gen.Queries, chapter int) (pgtype.UUID, error) {
	id, err := randomUUID()
	if err != nil {
		return pgtype.UUID{}, err
	}
	row, err := q.EnsureChapter(ctx, gen.EnsureChapterParams{
		ProjectID: c.scope.ProjectID,
		StoryID:   c.scope.StoryID,
		ID:        id,
		Number:    int32(chapter),
	})
	if err != nil {
		if noRows(err) {
			return pgtype.UUID{}, fmt.Errorf("chương %d đã bị xoá; không thể dùng lại số chương: %w", chapter, errs.ErrStorageConflict)
		}
		return pgtype.UUID{}, sanitizeErr(err)
	}
	return row.ID, nil
}
