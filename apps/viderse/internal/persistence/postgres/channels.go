package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
)

// ChannelStore là kho của Kênh của bạn trên owner_channels /
// owner_channel_sources. Mọi ghi nhiều dòng đi qua một transaction; đọc bảng
// rival_channels/templates/rival_videos chỉ để dựng bối cảnh (đối thủ tham
// chiếu, template mặc định) — không ghi bảng của mảng khác.
type ChannelStore struct {
	q  *gen.Queries
	db DBTXer
}

// NewChannelStore dựng store trên pool/connection (cần Begin được).
func NewChannelStore(db DBTXer) *ChannelStore {
	return &ChannelStore{q: gen.New(db), db: db}
}

var _ channels.Store = (*ChannelStore)(nil)

// withTx mở một transaction và đưa bộ truy vấn đã gắn tx.
func (s *ChannelStore) withTx(ctx context.Context, fn func(q *gen.Queries) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ── chuyển kiểu ────────────────────────────────────────────────────────────

// uuidArg đổi chuỗi uuid sang kiểu pgx; chuỗi hỏng là lỗi của caller, không
// được lặng lẽ thành uuid rỗng.
func uuidArg(id string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(id); err != nil {
		return pgtype.UUID{}, fmt.Errorf("id không phải uuid: %w", err)
	}
	return u, nil
}

func textArg(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func tsArg(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func strPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func tsPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

// channelFromRow chuyển một dòng owner_channels sang kiểu domain; branding
// JSON hỏng đọc như rỗng (không chặn cả danh sách vì một dòng cũ).
func ownerChannelFromRow(r gen.OwnerChannel) domain.Channel {
	var branding domain.Branding
	if len(r.Branding) > 0 {
		_ = json.Unmarshal(r.Branding, &branding)
	}
	tags := r.AlwaysTags
	if tags == nil {
		tags = []string{}
	}
	id, _ := r.ID.Value()
	var instructions domain.AgentInstructions
	if len(r.AgentInstructions) > 0 {
		_ = json.Unmarshal(r.AgentInstructions, &instructions)
	}
	return domain.Channel{
		ID:                 fmt.Sprint(id),
		Name:               r.Name,
		YoutubeURL:         strPtr(r.YoutubeUrl),
		Language:           r.Language,
		StyleNote:          strPtr(r.StyleNote),
		DescriptionFooter:  strPtr(r.DescriptionFooter),
		AlwaysTags:         tags,
		DefaultTemplateID:  rowUUID(r.DefaultTemplateID),
		AutoFetch:          r.AutoFetch,
		AutoApprove:        r.AutoApprove,
		PerDay:             int(r.PerDay),
		AgentInstructions:  domain.NormalizeAgentInstructions(instructions),
		Branding:           branding,
		AvatarPath:         strPtr(r.AvatarPath),
		BannerPath:         strPtr(r.BannerPath),
		IdentityState:      r.IdentityState,
		IdentityPart:       strPtr(r.IdentityPart),
		IdentityTaskID:     strPtr(r.IdentityTaskID),
		IdentityErrorCode:  strPtr(r.IdentityErrorCode),
		IdentityError:      strPtr(r.IdentityError),
		IdentityFailedStep: strPtr(r.IdentityFailedStep),
		IdentityRetryAt:    tsPtr(r.IdentityRetryAt),
		IdentityStartedAt:  tsPtr(r.IdentityStartedAt),
		CreatedAt:          r.CreatedAt.Time,
		UpdatedAt:          r.UpdatedAt.Time,
	}
}

// agentInstructionsJSON marshal chỉ dẫn kênh; nil → {} để cột NOT NULL đọc ra
// object rỗng.
func agentInstructionsJSON(a domain.AgentInstructions) []byte {
	if a == nil {
		a = domain.NewAgentInstructions()
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

// agentInstructionValue đặt giá trị chỉ dẫn; rỗng → nil để SQL rơi nhánh xoá.
func agentInstructionValue(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func rowUUID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	v, _ := u.Value()
	return fmt.Sprint(v)
}

// ── đọc ────────────────────────────────────────────────────────────────────

// ListChannels đọc mọi kênh kèm đối thủ (thứ tự liên kết, dòng đầu là tham
// chiếu) và template mặc định trong ba truy vấn.
func (s *ChannelStore) ListChannels(ctx context.Context) ([]channels.ChannelView, error) {
	rows, err := s.q.ListOwnerChannels(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]channels.ChannelView, 0, len(rows))
	ids := make([]pgtype.UUID, 0, len(rows))
	for _, r := range rows {
		views = append(views, channels.ChannelView{Channel: ownerChannelFromRow(r)})
		ids = append(ids, r.ID)
	}
	if len(ids) == 0 {
		return views, nil
	}
	if err := s.attachLinks(ctx, s.q, views, ids); err != nil {
		return nil, err
	}
	return views, nil
}

// GetChannel đọc một kênh kèm đối thủ và template; found=false khi không có.
func (s *ChannelStore) GetChannel(ctx context.Context, id string) (channels.ChannelView, bool, error) {
	uid, err := uuidArg(id)
	if err != nil {
		return channels.ChannelView{}, false, nil // id lạ = không tồn tại, không phải 500
	}
	row, err := s.q.GetOwnerChannel(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return channels.ChannelView{}, false, nil
	}
	if err != nil {
		return channels.ChannelView{}, false, err
	}
	views := []channels.ChannelView{{Channel: ownerChannelFromRow(row)}}
	if err := s.attachLinks(ctx, s.q, views, []pgtype.UUID{row.ID}); err != nil {
		return channels.ChannelView{}, false, err
	}
	return views[0], true, nil
}

// attachLinks điền đối thủ và template mặc định cho các view đã đọc (một
// truy vấn mỗi loại cho cả lô).
func (s *ChannelStore) attachLinks(ctx context.Context, q *gen.Queries, views []channels.ChannelView, ids []pgtype.UUID) error {
	sources, err := q.ListChannelSources(ctx, ids)
	if err != nil {
		return err
	}
	defaults, err := q.ListChannelDefaults(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[string]*channels.ChannelView, len(views))
	for i := range views {
		views[i].Sources = []domain.ChannelSource{}
		byID[views[i].ID] = &views[i]
	}
	// Đối thủ tham chiếu = link đầu tiên của MỖI kênh (thứ tự linked_at); seen
	// theo id kênh, không theo cặp — PK cặp vẫn duy nhất.
	seen := map[string]bool{}
	for _, r := range sources {
		v := byID[rowUUID(r.OwnerChannelID)]
		if v == nil {
			continue
		}
		v.Sources = append(v.Sources, domain.ChannelSource{
			RivalID:        r.RivalID,
			Title:          r.Title,
			Handle:         strPtr(r.Handle),
			LinkedAt:       r.LinkedAt.Time,
			Reference:      !seen[v.ID],
			MinDurationSec: int4Ptr(r.MinDurationSec),
		})
		seen[v.ID] = true
	}
	for _, r := range defaults {
		v := byID[rowUUID(r.OwnerChannelID)]
		if v == nil {
			continue
		}
		v.DefaultTemplate = &domain.ChannelTemplate{ID: rowUUID(r.TemplateID), Name: r.Name}
	}
	return nil
}

// ChannelExists báo kênh có tồn tại.
func (s *ChannelStore) ChannelExists(ctx context.Context, id string) (bool, error) {
	uid, err := uuidArg(id)
	if err != nil {
		return false, nil
	}
	return s.q.ChannelExists(ctx, uid)
}

// RivalExists báo rival_id có phải đối thủ đang theo dõi.
func (s *ChannelStore) RivalExists(ctx context.Context, rivalID string) (bool, error) {
	return s.q.RivalExistsForChannels(ctx, rivalID)
}

// TemplateExists báo template id có trong thư viện.
func (s *ChannelStore) TemplateExists(ctx context.Context, templateID string) (bool, error) {
	uid, err := uuidArg(templateID)
	if err != nil {
		return false, nil
	}
	return s.q.TemplateExistsForChannels(ctx, uid)
}

// ── tạo / sửa / xoá ────────────────────────────────────────────────────────

// CreateChannel ghi kênh + link đối thủ trong một transaction. Rival nào
// không còn trong rival_channels làm cả tx hỏng (không tạo kênh nửa vời).
func (s *ChannelStore) CreateChannel(ctx context.Context, in channels.CreateInput) (string, error) {
	uid, err := uuidArg(in.ID)
	if err != nil {
		return "", err
	}
	err = s.withTx(ctx, func(q *gen.Queries) error {
		if len(in.CompetitorIDs) > 0 {
			if err := q.LockChannelForCreate(ctx, in.CompetitorIDs); err != nil {
				return err
			}
			for _, rivalID := range in.CompetitorIDs {
				ok, err := q.RivalExistsForChannels(ctx, rivalID)
				if err != nil {
					return err
				}
				if !ok {
					return &channels.Error{Code: channels.CodeNotFound,
						Message:  fmt.Sprintf("Đối thủ %s chưa được theo dõi.", rivalID),
						Recovery: "Thêm đối thủ ở trang Đối thủ trước."}
				}
			}
		}
		// Template mặc định đã tồn tại (client chọn hoặc Mẫu chuẩn vừa tạo
		// trước transaction); ghi thẳng cùng dòng kênh để không có kênh nào
		// tồn tại mà thiếu default.
		tu, err := uuidArg(in.DefaultTemplateID)
		if err != nil {
			return err
		}
		if err := q.InsertOwnerChannel(ctx, gen.InsertOwnerChannelParams{
			ID:                uid,
			Name:              in.Name,
			YoutubeUrl:        textArg(in.YoutubeURL),
			Language:          in.Language,
			AutoFetch:         in.AutoFetch,
			PerDay:            int32(in.PerDay),
			AutoApprove:       in.AutoApprove,
			DefaultTemplateID: tu,
			Column9:           agentInstructionsJSON(in.AgentInstructions),
		}); err != nil {
			return mapChannelFK(err)
		}
		for _, rivalID := range in.CompetitorIDs {
			if _, err := q.LinkSource(ctx, gen.LinkSourceParams{OwnerChannelID: uid, RivalID: rivalID}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return in.ID, nil
}

// PatchChannel sửa đúng các trường có cờ; false = kênh không còn.
func (s *ChannelStore) PatchChannel(ctx context.Context, id string, p channels.Patch) (bool, error) {
	uid, err := uuidArg(id)
	if err != nil {
		return false, nil
	}
	var nameText *string
	if p.Name.Set {
		nameText = &p.Name.Value
	}
	var defaultTemplate pgtype.UUID
	if p.DefaultTemplateID.Set {
		defaultTemplate, err = uuidArg(p.DefaultTemplateID.Value)
		if err != nil {
			return false, err
		}
	}
	n, err := s.q.PatchChannel(ctx, gen.PatchChannelParams{
		NameSet:            p.Name.Set,
		Name:               textArg(nameText),
		YoutubeUrlSet:      p.YoutubeURL.Set,
		YoutubeUrl:         textArg(p.YoutubeURL.Value),
		LanguageSet:        p.Language.Set,
		Language:           p.Language.Value,
		StyleNoteSet:       p.StyleNote.Set,
		StyleNote:          textArg(p.StyleNote.Value),
		FooterSet:          p.DescriptionFooter.Set,
		Footer:             textArg(p.DescriptionFooter.Value),
		TagsSet:            p.AlwaysTags.Set,
		Tags:               p.AlwaysTags.Value,
		AutoFetchSet:       p.AutoFetch.Set,
		AutoFetch:          p.AutoFetch.Value,
		AutoApproveSet:     p.AutoApprove.Set,
		AutoApprove:        p.AutoApprove.Value,
		PerDaySet:          p.PerDay.Set,
		PerDay:             int32(p.PerDay.Value),
		DefaultTemplateSet: p.DefaultTemplateID.Set,
		DefaultTemplateID:  defaultTemplate,
		AgentSet:           p.AgentInstructions.Set,
		AgentKey:           p.AgentInstructions.Key,
		AgentValue:         agentInstructionValue(p.AgentInstructions.Value),
		BrandingSet:        p.Brief.Set,
		Brief:              p.Brief.Value,
		ID:                 uid,
	})
	if err != nil {
		return false, mapChannelFK(err)
	}
	return n > 0, nil
}

// mapChannelFK đổi lỗi khoá ngoại khi template default vừa bị xoá thành 409
// thân thiện thay vì 500. FK là backstop cho pre-check ở service.
func mapChannelFK(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return &channels.Error{Code: channels.CodeConflict,
			Message:  "Template vừa bị xoá hoặc đổi, tải lại rồi chọn lại mặc định.",
			Recovery: "Tải lại trang Hồ sơ kênh."}
	}
	return err
}

// DeleteChannel xoá kênh; link + template xoá theo (ON DELETE CASCADE).
func (s *ChannelStore) DeleteChannel(ctx context.Context, id string) (bool, error) {
	uid, err := uuidArg(id)
	if err != nil {
		return false, nil
	}
	n, err := s.q.DeleteChannel(ctx, uid)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// LinkSource thêm link (idempotent). inserted=false khi đã có link.
func (s *ChannelStore) LinkSource(ctx context.Context, id, rivalID string) (inserted bool, err error) {
	uid, err := uuidArg(id)
	if err != nil {
		return false, nil
	}
	err = s.withTx(ctx, func(q *gen.Queries) error {
		// Khoá hàng kênh trước: serialize các thao tác link đồng thời.
		if _, err := q.LockOwnerChannel(ctx, uid); err != nil {
			return err
		}
		n, err := q.LinkSource(ctx, gen.LinkSourceParams{OwnerChannelID: uid, RivalID: rivalID})
		if err != nil {
			return err
		}
		inserted = n > 0
		if !inserted {
			return nil
		}
		// Đối thủ đầu tiên (kênh vừa bỏ hết nguồn nên đã tạm dừng Tự lấy) bật lại
		// Tự lấy: cờ bật/tắt Tự lấy của kênh.
		count, err := q.CountSources(ctx, uid)
		if err != nil {
			return err
		}
		if count == 1 {
			_, err = q.SetAutoFetch(ctx, gen.SetAutoFetchParams{ID: uid, AutoFetch: true})
			return err
		}
		_, err = q.SetAutoFetchTouch(ctx, uid)
		return err
	})
	return inserted, err
}

// UnlinkSource xoá một link. paused=true khi đây là link cuối cùng: Tự lấy bị
// tắt (một kênh không còn đối thủ thì không có nguồn để tự lấy).
func (s *ChannelStore) UnlinkSource(ctx context.Context, id, rivalID string) (removed, paused bool, err error) {
	uid, err := uuidArg(id)
	if err != nil {
		return false, false, nil
	}
	err = s.withTx(ctx, func(q *gen.Queries) error {
		// Khoá hàng kênh trước: hai unlink đồng thời của hai đối thủ cuối phải
		// thấy nhau — bỏ cái cuối mới được tắt Tự lấy.
		if _, err := q.LockOwnerChannel(ctx, uid); err != nil {
			return err
		}
		n, err := q.UnlinkSource(ctx, gen.UnlinkSourceParams{OwnerChannelID: uid, RivalID: rivalID})
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
		removed = true
		left, err := q.CountSources(ctx, uid)
		if err != nil {
			return err
		}
		if left == 0 {
			if _, err := q.SetAutoFetch(ctx, gen.SetAutoFetchParams{ID: uid, AutoFetch: false}); err != nil {
				return err
			}
			paused = true
			return nil
		}
		_, err = q.SetAutoFetchTouch(ctx, uid)
		return err
	})
	return removed, paused, err
}

// SetSourceMinDuration đặt (hoặc xoá, khi minDurationSec nil) bộ lọc thời lượng
// Tự lấy của một đối thủ. found=false khi kênh chưa liên kết đối thủ đó.
func (s *ChannelStore) SetSourceMinDuration(ctx context.Context, id, rivalID string, minDurationSec *int) (found bool, err error) {
	uid, err := uuidArg(id)
	if err != nil {
		return false, nil
	}
	value := pgtype.Int4{}
	if minDurationSec != nil {
		value = int4Scan(*minDurationSec)
	}
	err = s.withTx(ctx, func(q *gen.Queries) error {
		if _, err := q.LockOwnerChannel(ctx, uid); err != nil {
			return err
		}
		n, err := q.SetSourceMinDuration(ctx, gen.SetSourceMinDurationParams{MinDurationSec: value, OwnerChannelID: uid, RivalID: rivalID})
		if err != nil {
			return err
		}
		if found = n > 0; !found {
			return nil
		}
		_, err = q.SetAutoFetchTouch(ctx, uid)
		return err
	})
	return found, err
}

// ── nhận diện ──────────────────────────────────────────────────────────────

// LoadIdentityRun đọc kênh để chuẩn bị một run nhận diện.
func (s *ChannelStore) LoadIdentityRun(ctx context.Context, id string) (channels.IdentityRun, bool, error) {
	view, found, err := s.GetChannel(ctx, id)
	if err != nil || !found {
		return channels.IdentityRun{}, found, err
	}
	c := view.Channel
	return channels.IdentityRun{
		ID:         c.ID,
		Name:       c.Name,
		StyleNote:  c.StyleNote,
		Branding:   c.Branding,
		AvatarPath: c.AvatarPath,
		BannerPath: c.BannerPath,
	}, true, nil
}

// AcquireIdentityLease giành quyền chạy; false = một run khác đang giữ lease.
func (s *ChannelStore) AcquireIdentityLease(ctx context.Context, in channels.LeaseInput) (bool, error) {
	uid, err := uuidArg(in.ChannelID)
	if err != nil {
		return false, nil
	}
	token, err := uuidArg(in.Token)
	if err != nil {
		return false, err
	}
	row, err := s.q.AcquireIdentityLease(ctx, gen.AcquireIdentityLeaseParams{
		ID:                 uid,
		IdentityPart:       pgtype.Text{String: in.Part, Valid: true},
		IdentityTaskID:     textArg(in.TaskID),
		IdentityLeaseOwner: pgtype.Text{String: in.Owner, Valid: true},
		IdentityLeaseToken: token,
		IdentityLeaseUntil: tsArg(&in.Until),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.Valid, nil
}

// HeartbeatIdentityLease kéo dài lease; false = đã mất quyền run.
func (s *ChannelStore) HeartbeatIdentityLease(ctx context.Context, id, token string, until time.Time) (bool, error) {
	uid, err := uuidArg(id)
	if err != nil {
		return false, nil
	}
	tu, err := uuidArg(token)
	if err != nil {
		return false, err
	}
	n, err := s.q.HeartbeatIdentityLease(ctx, gen.HeartbeatIdentityLeaseParams{
		ID: uid, IdentityLeaseToken: tu, IdentityLeaseUntil: tsArg(&until),
	})
	return n > 0, err
}

// IdentityLeaseLive báo còn run nào giữ lease sống không.
func (s *ChannelStore) IdentityLeaseLive(ctx context.Context, id string) (bool, error) {
	uid, err := uuidArg(id)
	if err != nil {
		return false, nil
	}
	live, err := s.q.LeaseIdentityLive(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return live, err
}

// PublishIdentity công bố tên + tài sản + branding trong một ghi, fence theo
// lease và tên đọc lúc bắt đầu. false = bị từ chối (mất lease, đổi tên, xoá).
func (s *ChannelStore) PublishIdentity(ctx context.Context, in channels.PublishInput) (bool, error) {
	uid, err := uuidArg(in.ChannelID)
	if err != nil {
		return false, nil
	}
	token, err := uuidArg(in.Token)
	if err != nil {
		return false, err
	}
	payload, err := json.Marshal(in.Branding)
	if err != nil {
		return false, err
	}
	n, err := mediaQueries(ctx, s.q).PublishIdentity(ctx, gen.PublishIdentityParams{
		ID:                 uid,
		IdentityLeaseToken: token,
		PublishName:        in.Name,
		Branding:           payload,
		AvatarPath:         in.AvatarPath,
		BannerPath:         in.BannerPath,
		ExpectedName:       in.ExpectedName,
	})
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// FailIdentity ghi lỗi của run (chỉ khi còn giữ lease) và nhả lease.
func (s *ChannelStore) FailIdentity(ctx context.Context, in channels.FailInput) (bool, error) {
	uid, err := uuidArg(in.ChannelID)
	if err != nil {
		return false, nil
	}
	token, err := uuidArg(in.Token)
	if err != nil {
		return false, err
	}
	n, err := s.q.FailIdentity(ctx, gen.FailIdentityParams{
		ID:                 uid,
		IdentityLeaseToken: token,
		IdentityErrorCode:  pgtype.Text{String: in.Code, Valid: true},
		IdentityError:      pgtype.Text{String: in.Message, Valid: true},
		IdentityRetryAt:    tsArg(in.RetryAt),
		IdentityFailedStep: pgtype.Text{String: in.Step, Valid: in.Step != ""},
	})
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ReleaseIdentityLease nhả lease (đường thành công đã nhả trong publish).
func (s *ChannelStore) ReleaseIdentityLease(ctx context.Context, id, token string) error {
	uid, err := uuidArg(id)
	if err != nil {
		return nil
	}
	tu, err := uuidArg(token)
	if err != nil {
		return err
	}
	return s.q.ReleaseIdentityLease(ctx, gen.ReleaseIdentityLeaseParams{ID: uid, IdentityLeaseToken: tu})
}

// MarkInterruptedIdentities đánh dấu các run 'running' có lease đã hết hạn là
// hỏng 'interrupted' (phục hồi sau sập/tắt app). Trả id các kênh đã đổi.
func (s *ChannelStore) MarkInterruptedIdentities(ctx context.Context) ([]string, error) {
	rows, err := s.q.MarkInterruptedIdentities(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowUUID(r))
	}
	return out, nil
}

// ChannelAssetPaths trả hai đường tương đối của tài sản đang dùng.
func (s *ChannelStore) ChannelAssetPaths(ctx context.Context, id string) (avatar, banner *string, found bool, err error) {
	uid, uerr := uuidArg(id)
	if uerr != nil {
		return nil, nil, false, nil
	}
	row, err := s.q.ChannelAssetPaths(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	return strPtr(row.AvatarPath), strPtr(row.BannerPath), true, nil
}

// ── bối cảnh nhận diện ─────────────────────────────────────────────────────

// Competitors trả đối thủ liên kết theo thứ tự liên kết (dòng đầu là tham
// chiếu nhận diện), kèm mô tả và avatar thật.
func (s *ChannelStore) Competitors(ctx context.Context, id string) ([]channels.Competitor, error) {
	uid, err := uuidArg(id)
	if err != nil {
		return nil, nil
	}
	rows, err := s.q.ListCompetitors(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]channels.Competitor, 0, len(rows))
	for _, r := range rows {
		out = append(out, channels.Competitor{
			ID:          r.ID,
			Title:       r.Title,
			Handle:      strPtr(r.Handle),
			Description: r.Description,
			AvatarURL:   strPtr(r.AvatarUrl),
		})
	}
	return out, nil
}

// ReferenceLanguageText trả tiêu đề + mô tả của đối thủ tham chiếu và vài tiêu
// đề video gần nhất của nó — đầu vào đoán ngôn ngữ lúc tạo kênh.
func (s *ChannelStore) ReferenceLanguageText(ctx context.Context, id string) (channels.LanguageSample, error) {
	uid, err := uuidArg(id)
	if err != nil {
		return channels.LanguageSample{}, nil
	}
	rows, err := s.q.ReferenceRivalText(ctx, uid)
	if err != nil {
		return channels.LanguageSample{}, err
	}
	if len(rows) == 0 {
		return channels.LanguageSample{}, nil
	}
	ref := rows[0]
	sample := channels.LanguageSample{
		Title:       ref.Title,
		Description: ref.Description,
	}
	// Đối thủ tham chiếu chưa được đọc video nào thì chỉ đoán theo tên/mô tả.
	titles, err := s.q.RecentRivalTitles(ctx, ref.ID)
	if err != nil {
		return sample, err
	}
	sample.Titles = titles
	return sample, nil
}

// AttachIdentityTask gắn id việc nền sau acquire; false = run đã mất lease.
func (s *ChannelStore) AttachIdentityTask(ctx context.Context, id, token, taskID string) (bool, error) {
	uid, err := uuidArg(id)
	if err != nil {
		return false, nil
	}
	tu, err := uuidArg(token)
	if err != nil {
		return false, err
	}
	n, err := s.q.AttachIdentityTask(ctx, gen.AttachIdentityTaskParams{
		ID: uid, IdentityLeaseToken: tu, IdentityTaskID: pgtype.Text{String: taskID, Valid: true},
	})
	return n > 0, err
}

// NextIdentityLeaseExpiry trả hạn lease sớm nhất còn sống (nil khi không có).
func (s *ChannelStore) NextIdentityLeaseExpiry(ctx context.Context) (*time.Time, error) {
	until, err := s.q.NextIdentityLeaseExpiry(ctx)
	if err != nil {
		return nil, err
	}
	if !until.Valid || until.Time.IsZero() || until.Time.Year() < 2000 {
		return nil, nil
	}
	t := until.Time
	return &t, nil
}
