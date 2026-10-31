package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
)

// LibraryStore hiện thực library.Store trên templates/image_prompts/overlays
// qua code sqlc. Truy vấn chéo mảng chỉ đọc owner_channel_templates (kênh dùng
// template) và backdrop_* (kiểm tham chiếu nền); mọi ghi nằm trong bảng của
// Thư viện.
type LibraryStore struct {
	q  *gen.Queries
	db DBTXer
}

// NewLibraryStore dựng store trên pool/connection (cần Begin được để giữ
// tham chiếu prompt nguyên tử với ghi template/xoá/đổi loại prompt).
func NewLibraryStore(db DBTXer) *LibraryStore {
	return &LibraryStore{q: gen.New(db), db: db}
}

var _ library.Store = (*LibraryStore)(nil)

// ── template ───────────────────────────────────────────────────────────────

// ListTemplates đọc mọi template kèm kênh đang dùng.
func (s *LibraryStore) ListTemplates(ctx context.Context) ([]domain.TemplateSummary, error) {
	rows, err := s.q.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.TemplateSummary, 0, len(rows))
	for _, row := range rows {
		tpl, err := templateFromRow(row.ID, row.Name, row.Recipe, row.Revision, row.PreviewRevision, row.PreviewBuiltAt, row.CreatedAt, row.UpdatedAt)
		if err != nil {
			return nil, err
		}
		channels, err := decodeChannels(row.Channels)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.TemplateSummary{Template: tpl, Channels: channels})
	}
	return out, nil
}

// GetTemplate đọc một template; ErrNotFound khi không có.
func (s *LibraryStore) GetTemplate(ctx context.Context, id string) (*domain.Template, error) {
	uid, err := libUUIDArg(id)
	if err != nil {
		return nil, library.ErrNotFound
	}
	var row struct {
		ID                                   pgtype.UUID
		Name                                 string
		Recipe                               []byte
		Revision                             int64
		PreviewRevision                      pgtype.Int8
		PreviewBuiltAt, CreatedAt, UpdatedAt pgtype.Timestamptz
		Project                              []byte
	}
	err = s.projectDB(ctx).QueryRow(ctx, `SELECT id,name,recipe,revision,preview_revision,preview_built_at,created_at,updated_at,project FROM templates WHERE id=$1`, uid).Scan(&row.ID, &row.Name, &row.Recipe, &row.Revision, &row.PreviewRevision, &row.PreviewBuiltAt, &row.CreatedAt, &row.UpdatedAt, &row.Project)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tpl, err := templateFromRow(row.ID, row.Name, row.Recipe, row.Revision, row.PreviewRevision, row.PreviewBuiltAt, row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return nil, err
	}
	tpl.Project = append(json.RawMessage(nil), row.Project...)
	return &tpl, nil
}

// InsertTemplate ghi template mới (revision do service đặt, thường 1).
func (s *LibraryStore) InsertTemplate(ctx context.Context, t domain.Template) error {
	uid, err := libUUIDArg(t.ID)
	if err != nil {
		return err
	}
	recipe, err := json.Marshal(t.Recipe)
	if err != nil {
		return fmt.Errorf("mã hoá recipe: %w", err)
	}
	_, err = s.projectDB(ctx).Exec(ctx, `INSERT INTO templates(id,name,recipe,revision,project) VALUES($1,$2,$3::jsonb,$4,$5::jsonb)`, uid, t.Name, recipe, t.Revision, t.Project)
	return mapLibraryErr(err)
}

// UpdateTemplate ghi recipe + tên với CAS revision. Tham chiếu prompt của
// recipe được khoá lại trong cùng transaction và kiểm đúng loại, nên một
// xoá/đổi loại prompt đua với lần ghi này bị chặn cả hai phía.
func (s *LibraryStore) UpdateTemplate(ctx context.Context, id string, name *string, recipe domain.Recipe, expectRevision int64, project json.RawMessage) (int64, bool, error) {
	uid, err := libUUIDArg(id)
	if err != nil {
		return 0, false, err
	}
	data, err := json.Marshal(recipe)
	if err != nil {
		return 0, false, fmt.Errorf("mã hoá recipe: %w", err)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	if err := lockSharedRefs(ctx, qtx, recipe); err != nil {
		return 0, false, err
	}
	if err := lockPromptRefs(ctx, qtx, recipe); err != nil {
		return 0, false, err
	}
	revision, err := qtx.UpdateTemplate(ctx, gen.UpdateTemplateParams{
		Name: libTextPtr(name), Recipe: data, ID: uid, ExpectRevision: expectRevision,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, mapLibraryErr(err)
	}
	if len(project) > 0 {
		if _, err := tx.Exec(ctx, `UPDATE templates SET project=$2::jsonb WHERE id=$1`, uid, project); err != nil {
			return 0, false, mapLibraryErr(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, mapLibraryErr(err)
	}
	return revision, true, nil
}

// projectDB joins media.Publish so asset bindings and project visibility commit
// together. Metadata-only callers use the ordinary pool.
func (s *LibraryStore) projectDB(ctx context.Context) gen.DBTX {
	if tx, ok := ctx.Value(mediaTxKey{}).(pgx.Tx); ok {
		return tx
	}
	return s.db
}

func (s *LibraryStore) InitializeTemplateProject(ctx context.Context, id string, project json.RawMessage, expectRevision int64) (bool, error) {
	uid, err := libUUIDArg(id)
	if err != nil {
		return false, err
	}
	tag, err := s.projectDB(ctx).Exec(ctx, `UPDATE templates SET project=$2::jsonb WHERE id=$1 AND project IS NULL AND revision=$3`, uid, project, expectRevision)
	return tag.RowsAffected() == 1, mapLibraryErr(err)
}

// lockSharedRefs khóa advisory các tài nguyên Video nền recipe trỏ tới (cùng
// hình khoá backdrop.ChannelLockKey/VideoLockKey — import chuỗi lạnh để không
// kéo package mảng khác vào store), rồi kiểm tồn tại trong cùng transaction:
// xoá kênh/video nền bên mảng Video nền cũng giữ cùng khoá nên không thể đua
// "kiểm rồi ghi". ErrBackdropRef khi một tham chiếu đã mất.
func lockSharedRefs(ctx context.Context, q *gen.Queries, recipe domain.Recipe) error {
	b := recipe.Backdrop
	if b == nil {
		return nil
	}
	keys := make([]string, 0, len(b.Channels)+len(b.FixedVideoIDs))
	channelIDs := make([]string, 0, len(b.Channels))
	for _, link := range b.Channels {
		if link.ChannelID == "" {
			continue
		}
		channelIDs = append(channelIDs, link.ChannelID)
		keys = append(keys, "backdrop_channel:"+link.ChannelID)
	}
	for _, id := range b.FixedVideoIDs {
		if id == "" {
			continue
		}
		keys = append(keys, "backdrop_video:"+id)
	}
	sort.Strings(keys)
	keys = dedupeStrings(keys)
	for _, key := range keys {
		if err := q.LockSharedRef(ctx, key); err != nil {
			return mapLibraryErr(err)
		}
	}
	// Kiểm tồn tại dưới khoá: kênh, video ghim của kênh, video lẻ.
	if err := checkMissingChannels(ctx, q, channelIDs); err != nil {
		return err
	}
	for _, link := range b.Channels {
		missing, err := q.BackdropVideosExisting(ctx, gen.BackdropVideosExistingParams{ChannelID: link.ChannelID, Ids: link.Pinned})
		if err != nil {
			return mapLibraryErr(err)
		}
		present := make(map[string]bool, len(missing))
		for _, id := range missing {
			present[id] = true
		}
		for _, id := range link.Pinned {
			if !present[id] {
				return fmt.Errorf("%w: video ghim %s của %s", library.ErrBackdropRef, id, link.ChannelID)
			}
		}
	}
	for _, id := range b.FixedVideoIDs {
		ok, err := q.BackdropVideoExists(ctx, id)
		if err != nil {
			return mapLibraryErr(err)
		}
		if !ok {
			return fmt.Errorf("%w: video lẻ %s", library.ErrBackdropRef, id)
		}
	}
	return nil
}

// checkMissingChannels báo ErrBackdropRef khi một kênh nền đã mất.
func checkMissingChannels(ctx context.Context, q *gen.Queries, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	found, err := q.BackdropChannelsExisting(ctx, ids)
	if err != nil {
		return mapLibraryErr(err)
	}
	present := make(map[string]bool, len(found))
	for _, id := range found {
		present[id] = true
	}
	for _, id := range ids {
		if !present[id] {
			return fmt.Errorf("%w: kênh nền %s", library.ErrBackdropRef, id)
		}
	}
	return nil
}

// dedupeStrings bỏ trùng chuỗi đã sắp.
func dedupeStrings(in []string) []string {
	out := in[:0]
	var last string
	for i, v := range in {
		if i == 0 || v != last {
			out = append(out, v)
			last = v
		}
	}
	return out
}

// lockPromptRefs khoá (FOR UPDATE) và kiểm kind của mọi prompt recipe trỏ
// tới; ErrPromptRef khi một prompt đã mất hoặc đổi loại.
func lockPromptRefs(ctx context.Context, q *gen.Queries, recipe domain.Recipe) error {
	refs := []struct {
		id   string
		kind domain.PromptKind
	}{
		{recipe.Scenes.ScenePromptID, domain.PromptScene},
		{recipe.Scenes.CoverPromptID, domain.PromptCover},
	}
	ids := make([]pgtype.UUID, 0, len(refs))
	want := make(map[string]domain.PromptKind, len(refs))
	for _, ref := range refs {
		if ref.id == "" || library.IsBuiltinPromptID(ref.id) {
			continue
		}
		uid, err := libUUIDArg(ref.id)
		if err != nil {
			return fmt.Errorf("%w: %s", library.ErrPromptRef, ref.id)
		}
		ids = append(ids, uid)
		want[ref.id] = ref.kind
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.LockPromptKinds(ctx, ids)
	if err != nil {
		return mapLibraryErr(err)
	}
	if len(rows) != len(ids) {
		return fmt.Errorf("%w: prompt đã bị xoá", library.ErrPromptRef)
	}
	for _, row := range rows {
		if want[libRowUUID(row.ID)] != row.Kind {
			return fmt.Errorf("%w: prompt đổi loại", library.ErrPromptRef)
		}
	}
	return nil
}

// RenameTemplate đổi tên, revision tuỳ chọn.
func (s *LibraryStore) RenameTemplate(ctx context.Context, id, name string, expectRevision *int64) (bool, error) {
	uid, err := libUUIDArg(id)
	if err != nil {
		return false, nil
	}
	var revision int64
	err = s.projectDB(ctx).QueryRow(ctx, `UPDATE templates SET name=$2, updated_at=now(), project=CASE WHEN project IS NULL THEN NULL ELSE jsonb_set(jsonb_set(project,'{name}',to_jsonb($2::text)),'{updatedAt}',to_jsonb((extract(epoch FROM now())*1000)::bigint)) END WHERE id=$1 AND ($3::bigint IS NULL OR revision=$3) RETURNING revision`, uid, name, expectRevision).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, mapLibraryErr(err)
	}
	return true, nil
}

// NameChannelStandardTemplates đổi tên trần plain → name cho Mẫu chuẩn chỉ kênh
// channelID dùng; trả id các template đã đổi.
func (s *LibraryStore) NameChannelStandardTemplates(ctx context.Context, channelID, plain, name string) ([]string, error) {
	uid, err := libUUIDArg(channelID)
	if err != nil {
		return nil, nil
	}
	rows, err := s.projectDB(ctx).Query(ctx, `UPDATE templates t SET name=$3, updated_at=now(), project=CASE WHEN project IS NULL THEN NULL ELSE jsonb_set(jsonb_set(project,'{name}',to_jsonb($3::text)),'{updatedAt}',to_jsonb((extract(epoch FROM now())*1000)::bigint)) END WHERE name=$2 AND EXISTS (SELECT 1 FROM owner_channel_templates c WHERE c.template_id=t.id AND c.owner_channel_id=$1) AND NOT EXISTS (SELECT 1 FROM owner_channel_templates c WHERE c.template_id=t.id AND c.owner_channel_id<>$1) RETURNING t.id`, uid, plain, name)
	if err != nil {
		return nil, mapLibraryErr(err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id pgtype.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, uuidText(id))
	}
	return ids, rows.Err()
}

// DeleteTemplate xoá một template; khoá ngoại RESTRICT là lưới đỡ cuối cùng.
func (s *LibraryStore) DeleteTemplate(ctx context.Context, id string) error {
	uid, err := libUUIDArg(id)
	if err != nil {
		return library.ErrNotFound
	}
	n, err := s.q.DeleteTemplate(ctx, uid)
	if err != nil {
		return mapLibraryErr(err)
	}
	if n == 0 {
		return library.ErrNotFound
	}
	return nil
}

// TemplateChannels đọc kênh đang dùng một template.
func (s *LibraryStore) TemplateChannels(ctx context.Context, templateID string) (domain.ChannelRefList, error) {
	uid, err := libUUIDArg(templateID)
	if err != nil {
		return nil, nil
	}
	rows, err := s.q.TemplateChannels(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make(domain.ChannelRefList, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.ChannelRef{ID: libRowUUID(row.ID), Name: row.Name, Position: int(row.Position)})
	}
	return out, nil
}

// TemplatesUsingPrompt tìm template đang trỏ tới một prompt.
func (s *LibraryStore) TemplatesUsingPrompt(ctx context.Context, promptID string) ([]library.TemplateRef, error) {
	rows, err := s.q.TemplatesUsingPrompt(ctx, promptID)
	if err != nil {
		return nil, err
	}
	out := make([]library.TemplateRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, library.TemplateRef{ID: libRowUUID(row.ID), Name: row.Name})
	}
	return out, nil
}

// ── prompt ảnh ─────────────────────────────────────────────────────────────

// ListPrompts đọc prompt của bạn; q đã gấp dấu ở service (fold_title).
func (s *LibraryStore) ListPrompts(ctx context.Context, kind domain.PromptKind, q string) ([]domain.ImagePrompt, error) {
	rows, err := s.q.ListPrompts(ctx, gen.ListPromptsParams{Kind: kind, Q: q})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ImagePrompt, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.ImagePrompt{
			ID: libRowUUID(row.ID), Name: row.Name, Kind: row.Kind,
			Composition: row.Composition, Prompt: row.Prompt, UpdatedAt: row.UpdatedAt.Time,
		})
	}
	return out, nil
}

// GetPrompt đọc một prompt của bạn; nil khi không có.
func (s *LibraryStore) GetPrompt(ctx context.Context, id string) (*domain.ImagePrompt, error) {
	uid, err := libUUIDArg(id)
	if err != nil {
		return nil, nil
	}
	row, err := s.q.GetPrompt(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &domain.ImagePrompt{
		ID: libRowUUID(row.ID), Name: row.Name, Kind: row.Kind,
		Composition: row.Composition, Prompt: row.Prompt, UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

// PromptExists báo một prompt của bạn còn và đúng loại.
func (s *LibraryStore) PromptExists(ctx context.Context, id string, kind domain.PromptKind) (bool, error) {
	uid, err := libUUIDArg(id)
	if err != nil {
		return false, nil
	}
	return s.q.PromptExists(ctx, gen.PromptExistsParams{ID: uid, Kind: kind})
}

// NameTaken báo tên prompt đã có (không phân biệt hoa thường).
func (s *LibraryStore) NameTaken(ctx context.Context, name, exceptID string) (bool, error) {
	params := gen.PromptNameTakenParams{Name: name}
	if exceptID != "" {
		uid, err := libUUIDArg(exceptID)
		if err != nil {
			return false, err
		}
		params.ExceptID = uid
	}
	return s.q.PromptNameTaken(ctx, params)
}

// InsertPrompt ghi prompt mới.
func (s *LibraryStore) InsertPrompt(ctx context.Context, p domain.ImagePrompt) error {
	uid, err := libUUIDArg(p.ID)
	if err != nil {
		return err
	}
	err = s.q.InsertPrompt(ctx, gen.InsertPromptParams{
		ID: uid, Name: p.Name, Kind: p.Kind,
		Composition: optionalText(p.Composition), Prompt: p.Prompt, UpdatedAt: ts(p.UpdatedAt),
	})
	return mapLibraryErr(err)
}

// UpdatePrompt ghi đè một prompt của bạn. Khi loại prompt đổi, hàng bị khoá
// và kiểm lại template đang dùng trong cùng transaction — trùng với khoá mà
// UpdateTemplate cầm khi ghi recipe.
func (s *LibraryStore) UpdatePrompt(ctx context.Context, p domain.ImagePrompt) error {
	uid, err := libUUIDArg(p.ID)
	if err != nil {
		return library.ErrNotFound
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	locked, err := qtx.LockPrompt(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return library.ErrNotFound
	}
	if err != nil {
		return mapLibraryErr(err)
	}
	if locked.Kind != p.Kind {
		refs, err := qtx.TemplatesUsingPrompt(ctx, p.ID)
		if err != nil {
			return mapLibraryErr(err)
		}
		if len(refs) > 0 {
			return fmt.Errorf("%w: template còn dùng prompt", library.ErrInUse)
		}
	}
	err = qtx.UpdatePrompt(ctx, gen.UpdatePromptParams{
		Name: p.Name, Kind: p.Kind, Composition: optionalText(p.Composition),
		Prompt: p.Prompt, UpdatedAt: ts(p.UpdatedAt), ID: uid,
	})
	if err != nil {
		return mapLibraryErr(err)
	}
	return tx.Commit(ctx)
}

// DeletePrompt xoá một prompt của bạn; hàng bị khoá và kiểm lại template
// đang dùng trong cùng transaction để không xoá giữa hai lần đọc.
func (s *LibraryStore) DeletePrompt(ctx context.Context, id string) error {
	uid, err := libUUIDArg(id)
	if err != nil {
		return library.ErrNotFound
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)
	if _, err := qtx.LockPrompt(ctx, uid); errors.Is(err, pgx.ErrNoRows) {
		return library.ErrNotFound
	} else if err != nil {
		return mapLibraryErr(err)
	}
	refs, err := qtx.TemplatesUsingPrompt(ctx, id)
	if err != nil {
		return mapLibraryErr(err)
	}
	if len(refs) > 0 {
		return fmt.Errorf("%w: template còn dùng prompt", library.ErrInUse)
	}
	if _, err := qtx.DeletePrompt(ctx, uid); err != nil {
		return mapLibraryErr(err)
	}
	return tx.Commit(ctx)
}

// ── lớp phủ ────────────────────────────────────────────────────────────────

// ListOverlays đọc lớp phủ còn trong thư viện, mới nhất trước.
func (s *LibraryStore) ListOverlays(ctx context.Context) ([]domain.Overlay, error) {
	rows, err := s.q.ListOverlays(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Overlay, 0, len(rows))
	for _, row := range rows {
		out = append(out, overlayFromRow(row.ID, row.Sha256, row.Name, row.FilePath, row.Mime, row.Bytes,
			row.DurationSec, row.Width, row.Height, row.Background, row.Builtin, row.CreatedAt))
	}
	return out, nil
}

// OverlayUsage gộp template đang gắn từng lớp phủ (một truy vấn).
func (s *LibraryStore) OverlayUsage(ctx context.Context) (map[string][]library.TemplateRef, error) {
	rows, err := s.q.OverlayUsage(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]library.TemplateRef)
	for _, row := range rows {
		out[row.OverlayID] = append(out[row.OverlayID], library.TemplateRef{ID: libRowUUID(row.ID), Name: row.Name})
	}
	return out, nil
}

// OverlayByID đọc một lớp phủ còn trong thư viện.
func (s *LibraryStore) OverlayByID(ctx context.Context, id string) (*domain.Overlay, error) {
	uid, err := libUUIDArg(id)
	if err != nil {
		return nil, nil
	}
	row, err := s.q.OverlayByID(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	overlay := overlayFromRow(row.ID, row.Sha256, row.Name, row.FilePath, row.Mime, row.Bytes,
		row.DurationSec, row.Width, row.Height, row.Background, row.Builtin, row.CreatedAt)
	return &overlay, nil
}

// OverlayBySHA tìm theo nội dung, kể cả dòng đã bỏ khỏi thư viện.
func (s *LibraryStore) OverlayBySHA(ctx context.Context, sha string) (*domain.Overlay, bool, error) {
	row, err := s.q.OverlayBySHA(ctx, sha)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	overlay := overlayFromRow(row.ID, row.Sha256, row.Name, row.FilePath, row.Mime, row.Bytes,
		row.DurationSec, row.Width, row.Height, row.Background, row.Builtin, row.CreatedAt)
	return &overlay, row.Removed, nil
}

// OverlayExists báo một lớp phủ còn trong thư viện.
func (s *LibraryStore) OverlayExists(ctx context.Context, id string) (bool, error) {
	uid, err := libUUIDArg(id)
	if err != nil {
		return false, nil
	}
	return s.q.OverlayExists(ctx, uid)
}

// InsertOverlay ghi một lớp phủ mới; đụng sha256/builtin là ErrOverlayExists.
func (s *LibraryStore) InsertOverlay(ctx context.Context, o domain.Overlay) error {
	uid, err := libUUIDArg(o.ID)
	if err != nil {
		return err
	}
	params := gen.InsertOverlayParams{
		ID: uid, Sha256: o.SHA256, Name: o.Name, FilePath: o.FilePath, Mime: o.Mime,
		Bytes: o.Bytes, Background: o.Background, CreatedAt: ts(o.CreatedAt),
	}
	if o.DurationSec != nil {
		params.DurationSec = pgtype.Float4{Float32: float32(*o.DurationSec), Valid: true}
	}
	if o.Width != nil {
		params.Width = pgtype.Int4{Int32: int32(*o.Width), Valid: true}
	}
	if o.Height != nil {
		params.Height = pgtype.Int4{Int32: int32(*o.Height), Valid: true}
	}
	if o.Builtin != "" {
		params.Builtin = pgtype.Text{String: o.Builtin, Valid: true}
	}
	return mapLibraryErr(mediaQueries(ctx, s.q).InsertOverlay(ctx, params))
}

// RestoreOverlay đưa một dòng đã bỏ trở lại thư viện (đổi tên theo tệp vừa thêm).
func (s *LibraryStore) RestoreOverlay(ctx context.Context, id, name string) error {
	uid, err := libUUIDArg(id)
	if err != nil {
		return library.ErrNotFound
	}
	return mediaQueries(ctx, s.q).RestoreOverlay(ctx, gen.RestoreOverlayParams{Name: name, ID: uid})
}

// RemoveOverlay đặt removed_at (tombstone), giữ tệp cho template đang dùng.
func (s *LibraryStore) RemoveOverlay(ctx context.Context, id string) error {
	uid, err := libUUIDArg(id)
	if err != nil {
		return library.ErrNotFound
	}
	n, err := s.q.RemoveOverlay(ctx, uid)
	if err != nil {
		return err
	}
	if n == 0 {
		return library.ErrNotFound
	}
	return nil
}

// ExistingBuiltins trả id hiệu ứng có sẵn đã từng có dòng (kể cả đã bỏ).
func (s *LibraryStore) ExistingBuiltins(ctx context.Context) (map[string]bool, error) {
	rows, err := s.q.ExistingBuiltins(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.Valid {
			out[row.String] = true
		}
	}
	return out, nil
}

// ── kiểm tham chiếu nền (chỉ đọc) ──────────────────────────────────────────

// BackdropChannels trả id kênh nền không còn trong backdrop_channels.
func (s *LibraryStore) BackdropChannels(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	found, err := s.q.BackdropChannelsExisting(ctx, ids)
	if err != nil {
		return nil, err
	}
	present := make(map[string]bool, len(found))
	for _, id := range found {
		present[id] = true
	}
	missing := make([]string, 0, len(ids))
	for _, id := range ids {
		if !present[id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

// BackdropVideos trả id video không thuộc kênh nền.
func (s *LibraryStore) BackdropVideos(ctx context.Context, channelID string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	found, err := s.q.BackdropVideosExisting(ctx, gen.BackdropVideosExistingParams{ChannelID: channelID, Ids: ids})
	if err != nil {
		return nil, err
	}
	present := make(map[string]bool, len(found))
	for _, id := range found {
		present[id] = true
	}
	missing := make([]string, 0, len(ids))
	for _, id := range ids {
		if !present[id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

// BackdropVideoExists báo một video lẻ còn trong backdrop_videos.
func (s *LibraryStore) BackdropVideoExists(ctx context.Context, id string) (bool, error) {
	return s.q.BackdropVideoExists(ctx, id)
}

// ── chuyển kiểu ────────────────────────────────────────────────────────────

// templateFromRow gộp các cột của templates thành kiểu miền.
func templateFromRow(id pgtype.UUID, name string, recipe []byte, revision int64, previewRevision pgtype.Int8, previewBuiltAt, createdAt, updatedAt pgtype.Timestamptz) (domain.Template, error) {
	var parsed domain.Recipe
	if len(recipe) > 0 {
		if err := json.Unmarshal(recipe, &parsed); err != nil {
			return domain.Template{}, fmt.Errorf("recipe của template %s hỏng: %w", uuidText(id), err)
		}
	}
	tpl := domain.Template{
		ID: uuidText(id), Name: name, Recipe: parsed, Revision: revision,
		PreviewBuiltAt: libTimePtr(previewBuiltAt), CreatedAt: createdAt.Time, UpdatedAt: updatedAt.Time,
	}
	if previewRevision.Valid {
		value := previewRevision.Int64
		tpl.PreviewRevision = &value
	}
	return tpl, nil
}

// overlayFromRow gộp các cột của overlays thành kiểu miền.
func overlayFromRow(id pgtype.UUID, sha, name, filePath, mime string, bytes int64, duration pgtype.Float4, width, height pgtype.Int4, background, builtin string, createdAt pgtype.Timestamptz) domain.Overlay {
	overlay := domain.Overlay{
		ID: uuidText(id), SHA256: sha, Name: name, FilePath: filePath, Mime: mime,
		Bytes: bytes, Background: background, Builtin: builtin, CreatedAt: createdAt.Time,
	}
	if duration.Valid {
		value := float64(duration.Float32)
		overlay.DurationSec = &value
	}
	if width.Valid {
		value := int(width.Int32)
		overlay.Width = &value
	}
	if height.Valid {
		value := int(height.Int32)
		overlay.Height = &value
	}
	return overlay
}

// decodeChannels đọc cột jsonb kênh dùng của một template.
func decodeChannels(data []byte) (domain.ChannelRefList, error) {
	if len(data) == 0 {
		return domain.ChannelRefList{}, nil
	}
	var raw []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Position int    `json:"position"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("cột kênh dùng của template hỏng: %w", err)
	}
	out := make(domain.ChannelRefList, 0, len(raw))
	for _, item := range raw {
		out = append(out, domain.ChannelRef{ID: item.ID, Name: item.Name, Position: item.Position})
	}
	return out, nil
}

// optionalText đổi chuỗi rỗng thành NULL (cột composition).
func optionalText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// mapLibraryErr dịch lỗi Postgres của Thư viện sang lỗi sentinel nghiệp vụ.
func mapLibraryErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "image_prompts_name_idx":
			return fmt.Errorf("%w: %s", library.ErrNameTaken, pgErr.Detail)
		case "overlays_sha256_key", "overlays_builtin_key":
			return fmt.Errorf("%w: %s", library.ErrOverlayExists, pgErr.Detail)
		}
	}
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
		// Template còn kênh dùng (RESTRICT) — service đã báo tên kênh trước,
		// đây là đua hiếm giữa kiểm và xoá.
		return fmt.Errorf("%w: template còn kênh dùng", library.ErrInUse)
	}
	return err
}

// ── trợ giúp cục bộ ────────────────────────────────────────────────────────

// libUUIDArg đổi chuỗi uuid sang kiểu pgx; chuỗi hỏng là lỗi của caller.
func libUUIDArg(id string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(id); err != nil {
		return pgtype.UUID{}, fmt.Errorf("id không phải uuid: %w", err)
	}
	return u, nil
}

// libRowUUID đổi uuid của hàng sang chuỗi.
func libRowUUID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	value, _ := u.Value()
	return fmt.Sprint(value)
}

// libTextPtr đổi *string sang cột text có thể NULL.
func libTextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// libTimePtr đổi cột timestamptz có thể NULL sang *time.Time.
func libTimePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}
