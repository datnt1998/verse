package postgres

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/scan"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ScanStore là kho của luồng quét đối thủ: chuyển nguyên văn ngữ nghĩa của
// scan-repository.ts — mọi ghi của trang/head-check/transition đều fenced bởi
// token kênh trong cùng giao dịch.
//
// Mọi giao dịch ghi khoá theo đúng một thứ tự: rival_channels → scan_state →
// scan_jobs → rival_videos. Fence chỉ được kiểm sau khi đã giữ khoá trạng
// thái, nên không còn khe "đọc token rồi mới ghi" và lệnh gỡ kênh không thể
// khoá chéo với một trang đang ghi.
type ScanStore struct {
	q  *gen.Queries
	db DBTXer
}

func NewScanStore(db DBTXer) *ScanStore {
	return &ScanStore{q: gen.New(db), db: db}
}

var _ scan.Store = (*ScanStore)(nil)

func (s *ScanStore) withTx(ctx context.Context, fn func(q *gen.Queries) error) error {
	return s.withTxConn(ctx, func(_ pgx.Tx, q *gen.Queries) error { return fn(q) })
}

// withTxConn như withTx nhưng đưa cả giao dịch cho nơi gọi, cho những bước cần
// câu lệnh ngoài sqlc (advisory lock) trong cùng giao dịch.
func (s *ScanStore) withTxConn(ctx context.Context, fn func(tx pgx.Tx, q *gen.Queries) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx, s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ── ánh lỗi + chuyển hàng ───────────────────────────────────────────────────

func missingChannel() error {
	return &scan.StoreError{Code: scan.StoreChannelMissing, Message: "Channel not found"}
}

func channelExists() error {
	return &scan.StoreError{Code: scan.StoreChannelExists, Message: "Channel is already being added"}
}

func leaseLost() error {
	return &scan.StoreError{Code: scan.StoreLeaseLost, Message: "Scan lease lost"}
}

func jobStopped() error {
	return &scan.StoreError{Code: scan.StoreJobStopped, Message: "Job is no longer running"}
}

func keyRange() error {
	return &scan.StoreError{Code: scan.StoreKeyRange, Message: "Discovery order key escaped the safe ordering range"}
}

func limit(msg string) error {
	return &scan.StoreError{Code: scan.StoreLimit, Message: msg}
}

func asErrNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func stateFromRow(r gen.ScanState) scan.State {
	return scan.State{
		ChannelID:         r.ChannelID,
		HistoryCap:        int(r.HistoryCap),
		IndexedVideos:     int(r.IndexedVideos),
		HistoryEnd:        r.HistoryEnd,
		HistoryCursor:     r.HistoryCursor.String,
		HistoryRemainder:  r.HistoryRemainder.String,
		HeadAnchors:       r.HeadAnchors,
		HeadNewestID:      r.HeadNewestID.String,
		HeadCheckedAt:     timePtr(r.HeadCheckedAt),
		DiscoveryCaughtUp: r.DiscoveryCaughtUp,
		Revision:          r.Revision,
		LeaseOwner:        r.LeaseOwner.String,
		LeaseToken:        uuidText(r.LeaseToken),
		LeaseUntil:        timePtr(r.LeaseUntil),
		UpdatedAt:         r.UpdatedAt.Time,
	}
}

func jobFromRow(r gen.ScanJob) *scan.Job {
	return &scan.Job{
		ID:            uuidText(r.ID),
		ChannelID:     r.ChannelID,
		Kind:          domain.ScanJobKind(r.Kind),
		State:         domain.ScanJobState(r.State),
		RequestedCap:  int4Ptr(r.RequestedCap),
		Cursor:        r.Cursor.String,
		CursorAt:      timePtr(r.CursorAt),
		Remainder:     r.Remainder.String,
		ReplayTarget:  r.ReplayTarget.String,
		LastVideoID:   r.LastVideoID.String,
		OrderBase:     int8Ptr(r.OrderBase),
		NextOrdinal:   int(r.NextOrdinal),
		Anchors:       r.Anchors,
		NewestID:      r.NewestID.String,
		Pages:         int(r.Pages),
		Scanned:       int(r.Scanned),
		Added:         int(r.Added),
		Attempts:      int(r.Attempts),
		Restarts:      int(r.Restarts),
		NextAttemptAt: timePtr(r.NextAttemptAt),
		ErrorCode:     r.ErrorCode,
		LastError:     r.LastError,
		Recovery:      r.Recovery,
		ReachedEnd:    r.ReachedEnd,
		TaskID:        uuidText(r.TaskID),
		StartedAt:     timePtr(r.StartedAt),
		UpdatedAt:     r.UpdatedAt.Time,
	}
}

func channelFromRow(r gen.RivalChannel) domain.RivalChannel {
	return domain.RivalChannel{
		ID:              r.ID,
		Handle:          r.Handle.String,
		Title:           r.Title,
		Description:     r.Description,
		AvatarURL:       r.AvatarUrl.String,
		URL:             r.Url,
		SubscriberCount: int8Ptr(r.SubscriberCount),
		MedianViews:     int8Ptr(r.MedianViews),
		ScannedAt:       timePtr(r.ScannedAt),
		AddedAt:         r.AddedAt.Time,
	}
}

func nargText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func textOf(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func int4Of(p *int64) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true}
}

func int8Of(p *int64) pgtype.Int8 {
	if p == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *p, Valid: true}
}

// scanNewUUID sinh uuid v4 cho hàng mới (việc quét).
func scanNewUUID() string {
	var b [16]byte
	_, _ = crand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// newLeaseToken là token fence (uuid v4) của một lượt giữ kênh.
func newLeaseToken() string { return scanNewUUID() }

// uuidOf đổi token/uuid text thành pgtype; lỗi parse không thể xảy ra với
// token sinh nội bộ nên trả zero (Valid=false → lọt các điều kiện khớp).
func uuidOf(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

func tsOf(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func valueOf(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func valueOfI64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func int32Narg(p *int32) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *p, Valid: true}
}

func timestamptzNarg(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func containsID(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// ── khoá theo thứ tự cố định ────────────────────────────────────────────────

// lockChannelStateTx giữ khoá parent rồi trạng thái của kênh — bước mở đầu bắt
// buộc của mọi giao dịch ghi trong luồng quét. Hàng trạng thái được tạo lười
// nếu chưa có; nhánh ON CONFLICT của câu ensure lấy khoá hàng đã có.
//
// Mốc thời gian trả về được đọc SAU khi đã giữ cả hai khoá: đó là mốc duy nhất
// dùng để xét fence có còn hạn hay không, nên một lease hết hạn trong lúc chờ
// khoá không thể ghi tiếp. `in.Now` chỉ dùng cho mốc metadata.
func (s *ScanStore) lockChannelStateTx(ctx context.Context, q *gen.Queries, channelID string) (scan.State, time.Time, error) {
	if _, err := q.ScanLockChannel(ctx, channelID); err != nil {
		if asErrNoRows(err) {
			return scan.State{}, time.Time{}, missingChannel()
		}
		return scan.State{}, time.Time{}, err
	}
	row, err := q.ScanEnsureState(ctx, channelID)
	if asErrNoRows(err) {
		return scan.State{}, time.Time{}, missingChannel()
	}
	if err != nil {
		return scan.State{}, time.Time{}, err
	}
	return stateFromRow(row), time.Now(), nil
}

// leaseHeld kiểm fence trên hàng trạng thái vừa khoá: đúng chủ và còn hạn.
func leaseHeld(state scan.State, lease scan.Lease, now time.Time) error {
	if state.LeaseToken == "" || state.LeaseToken != lease.Token {
		return leaseLost()
	}
	if state.LeaseUntil == nil || !state.LeaseUntil.After(now) {
		return leaseLost()
	}
	return nil
}

// ── kênh ────────────────────────────────────────────────────────────────────

func (s *ScanStore) Channel(ctx context.Context, id string) (domain.RivalChannel, error) {
	row, err := s.q.ScanGetChannel(ctx, id)
	if asErrNoRows(err) {
		return domain.RivalChannel{}, missingChannel()
	}
	if err != nil {
		return domain.RivalChannel{}, err
	}
	return channelFromRow(row), nil
}

// DeleteChannel xoá kênh: khoá parent trước (cùng thứ tự với trang đang ghi)
// rồi xoá dòng Việc nền; việc quét, trạng thái và video đi theo cascade.
func (s *ScanStore) DeleteChannel(ctx context.Context, id string) error {
	return s.withTx(ctx, func(q *gen.Queries) error {
		if _, err := q.ScanLockChannel(ctx, id); err != nil {
			if asErrNoRows(err) {
				return missingChannel()
			}
			return err
		}
		if err := q.ScanDeleteChannelTasks(ctx, id); err != nil {
			return err
		}
		n, err := q.ScanDeleteChannel(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return missingChannel()
		}
		return nil
	})
}

func (s *ScanStore) EnsureState(ctx context.Context, channelID string) (scan.State, error) {
	row, err := s.q.ScanEnsureState(ctx, channelID)
	if asErrNoRows(err) {
		return scan.State{}, missingChannel()
	}
	if err != nil {
		return scan.State{}, err
	}
	return stateFromRow(row), nil
}

// ── fence kênh ──────────────────────────────────────────────────────────────

// AcquireChannelLease giữ fence khi trống hoặc hết hạn. Kênh chưa có trạng
// thái được gieo trước; nil khi một holder khác còn hạn (kênh vắng coi như bận
// để worker bỏ qua nhẹ nhàng).
func (s *ScanStore) AcquireChannelLease(ctx context.Context, channelID, owner string, ttl time.Duration) (*scan.Lease, error) {
	now := time.Now()
	until := now.Add(ttl)
	token := newLeaseToken()
	got := false
	err := s.withTx(ctx, func(q *gen.Queries) error {
		if _, _, err := s.lockChannelStateTx(ctx, q, channelID); err != nil {
			return err
		}
		_, err := q.ScanAcquireChannelLease(ctx, gen.ScanAcquireChannelLeaseParams{
			ChannelID: channelID,
			Owner:     pgtype.Text{String: owner, Valid: true},
			Token:     uuidOf(token),
			Until:     tsOf(until),
			Now:       tsOf(now),
		})
		if asErrNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		got = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !got {
		return nil, nil
	}
	return &scan.Lease{ChannelID: channelID, Token: token, Owner: owner, Until: until}, nil
}

func (s *ScanStore) RenewChannelLease(ctx context.Context, lease scan.Lease) (*scan.Lease, error) {
	until := time.Now().Add(scan.LeaseTTL)
	n, err := s.q.ScanRenewChannelLease(ctx, gen.ScanRenewChannelLeaseParams{
		ChannelID: lease.ChannelID,
		Token:     uuidOf(lease.Token),
		Until:     tsOf(until),
	})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	next := lease
	next.Until = until
	return &next, nil
}

func (s *ScanStore) ReleaseChannelLease(ctx context.Context, lease scan.Lease) error {
	return s.q.ScanReleaseChannelLease(ctx, gen.ScanReleaseChannelLeaseParams{
		ChannelID: lease.ChannelID,
		Token:     uuidOf(lease.Token),
	})
}

// ── commit trang (fenced, một giao dịch) ───────────────────────────────────

// CommitPage ghi một trang đã đọc: hồ sơ + video + trạng thái + việc nguyên tử
// dưới fence; null không xoá giá trị đã biết; id của kênh khác bị bỏ qua;
// position/origin/first_seen bất biến (chuyển từ commitScanPage).
func (s *ScanStore) CommitPage(ctx context.Context, in scan.CommitPageInput) (scan.CommitPageResult, error) {
	var out scan.CommitPageResult
	if len(in.Videos) > scan.PageMax {
		return out, limit(fmt.Sprintf("scan page exceeds the %d video limit", scan.PageMax))
	}
	if len(in.Remainder) > scan.PageMax {
		return out, limit(fmt.Sprintf("scan remainder exceeds the %d video limit", scan.PageMax))
	}
	baseline := in.Origin == domain.OriginBaseline
	discovery := in.Origin == domain.OriginDiscovery
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	checkedAt := now

	err := s.withTx(ctx, func(q *gen.Queries) error {
		// Parent trước: trang baseline có hồ sơ chính là dòng kênh đầu tiên
		// (insert thuần — 0 dòng nghĩa là một lượt thêm khác đã thắng); mọi
		// đường khác khoá dòng parent đã có.
		created := false
		if baseline && in.JobID == "" && in.Profile != nil {
			n, err := q.ScanInsertChannel(ctx, gen.ScanInsertChannelParams{
				ID:              in.ChannelID,
				Handle:          textOf(in.Profile.Handle),
				Title:           in.Profile.Title,
				Description:     textOf(in.Profile.Description),
				AvatarUrl:       textOf(in.Profile.AvatarURL),
				Url:             in.Profile.URL,
				SubscriberCount: int8Of(in.Profile.SubscriberCount),
			})
			if err != nil {
				return err
			}
			if n == 0 {
				return channelExists()
			}
			created = true
		} else if _, err := q.ScanLockChannel(ctx, in.ChannelID); err != nil {
			if asErrNoRows(err) {
				return missingChannel()
			}
			return err
		}
		state, fenceNow, err := s.lockStateTx(ctx, q, in.ChannelID)
		if err != nil {
			return err
		}
		if created {
			// Add: gieo fence ngay trong giao dịch trang đầu.
			if _, err := q.ScanAcquireChannelLease(ctx, gen.ScanAcquireChannelLeaseParams{
				ChannelID: in.ChannelID,
				Owner:     pgtype.Text{String: in.Lease.Owner, Valid: true},
				Token:     uuidOf(in.Lease.Token),
				Until:     tsOf(in.Lease.Until),
				Now:       tsOf(now),
			}); err != nil {
				if asErrNoRows(err) {
					return leaseLost()
				}
				return err
			}
		} else if err := leaseHeld(state, in.Lease, fenceNow); err != nil {
			return err
		}

		// Việc của trang (nếu có) phải thuộc kênh, cùng loại và còn chạy.
		var job *scan.Job
		if in.JobID != "" {
			row, err := q.ScanGetJob(ctx, uuidOf(in.JobID))
			if asErrNoRows(err) {
				return jobStopped()
			}
			if err != nil {
				return err
			}
			job = jobFromRow(row)
			if job.ChannelID != in.ChannelID || job.State != domain.ScanRunning {
				return jobStopped()
			}
			if (job.Kind == domain.ScanHistory) != (in.Origin == domain.OriginHistory) {
				return jobStopped()
			}
		} else if discovery {
			return limit("discovery pages belong to a run job")
		}

		// Gốc thứ tự: discovery dùng khối khoá cố định của run, việc khác nối
		// sau vị trí lớn nhất.
		var orderBase int64
		ordinal0 := 0
		anchors := []string(nil)
		if discovery {
			if job.OrderBase == nil {
				return limit("discovery run has no reserved order block")
			}
			orderBase = *job.OrderBase
			ordinal0 = job.NextOrdinal
			anchors = job.Anchors
			if ordinal0 < 0 {
				return keyRange()
			}
		}

		// Dedup + khớp anchor (đo trước upsert).
		seen := map[string]bool{}
		videos := make([]youtube.ScannedVideo, 0, len(in.Videos))
		for _, v := range in.Videos {
			if seen[v.ID] {
				continue
			}
			seen[v.ID] = true
			videos = append(videos, v)
		}
		// Trần khối của run tính trên trang đã khử trùng (số khoá thật sự dùng).
		if discovery && ordinal0+len(videos) > scan.DiscoveryBlock {
			return keyRange()
		}
		var overlap *string
		if discovery {
			for _, v := range videos {
				if containsID(anchors, v.ID) {
					id := v.ID
					overlap = &id
					break
				}
			}
		}

		// Chủ sở hữu + khoá sắp xếp: id của kênh khác bị bỏ; của mình là hàng
		// đã biết (origin/position bất biến).
		ids := make([]string, 0, len(videos))
		for _, v := range videos {
			ids = append(ids, v.ID)
		}
		existing, err := s.videoRefsTx(ctx, q, ids)
		if err != nil {
			return err
		}
		type planned struct {
			video    youtube.ScannedVideo
			position int64
		}
		var plan []planned
		newPositions := map[int64]bool{}
		added := 0
		if !discovery {
			tail, err := q.ScanMaxPosition(ctx, in.ChannelID)
			if err != nil {
				return err
			}
			for _, v := range videos {
				if row, ok := existing[v.ID]; ok {
					if row.channelID != in.ChannelID {
						continue
					}
					plan = append(plan, planned{v, row.position})
					continue
				}
				key := tail + int64(added)
				if key > scan.KeyLimit {
					return keyRange()
				}
				plan = append(plan, planned{v, key})
				newPositions[key] = true
				added++
			}
		} else {
			for i, v := range videos {
				if row, ok := existing[v.ID]; ok {
					if row.channelID != in.ChannelID {
						continue
					}
					plan = append(plan, planned{v, row.position})
					continue
				}
				key := orderBase + int64(ordinal0) + int64(i)
				if key < -scan.KeyLimit || key > scan.KeyLimit {
					return keyRange()
				}
				plan = append(plan, planned{v, key})
				newPositions[key] = true
				added++
			}
		}

		// Xung đột khoá với hàng khác id cùng kênh (đủ hiếm để là lỗi trang).
		for _, row := range existing {
			if row.channelID != in.ChannelID || seen[row.id] {
				continue
			}
			if newPositions[row.position] {
				return limit("scan page collides with an existing ordering key")
			}
		}

		// Mức thường hiện tại (parent đã khoá) — score của hàng mới.
		var median *int64
		if channel, err := q.ScanGetChannel(ctx, in.ChannelID); err == nil && channel.MedianViews.Valid {
			m := channel.MedianViews.Int64
			median = &m
		}

		for _, p := range plan {
			if err := s.upsertVideoTx(ctx, q, in.ChannelID, p.video, p.position, in.Origin, now, checkedAt, median); err != nil {
				return err
			}
		}

		// Bản kế tiếp (dùng cho cả trạng thái lẫn việc). Trang baseline lưu
		// bookmark vào state để việc lịch sử đọc tiếp đúng chỗ dừng của lần
		// đọc đầu; run discovery job-less không còn nên không có đường nào
		// khác ghi bookmark.
		rawCursor, rawRemainder, err := serializeContinuation(in.NextCursor, in.Remainder)
		if err != nil {
			return err
		}
		saveBookmark := baseline && job == nil
		ended := in.NextCursor == nil && len(in.Remainder) == 0
		// Mốc "đã kiểm tra video mới" của state chỉ ghi ở trang không có việc
		// (baseline): run discovery mang mốc trên việc, và chỉ head check mới
		// được đẩy cửa sổ quét lại của kênh.
		markHead := in.JobID == "" && (baseline || discovery) && in.NewestVideoID != nil
		if err := q.ScanCommitState(ctx, gen.ScanCommitStateParams{
			ChannelID:     in.ChannelID,
			Added:         int32(added),
			Ended:         ended,
			SaveBookmark:  saveBookmark,
			Cursor:        nargText(rawCursor),
			Remainder:     nargText(rawRemainder),
			MarkHead:      markHead,
			NewestID:      nargText(valueOf(in.NewestVideoID)),
			MarkDiscovery: discovery,
			SetAnchors:    baseline,
			Anchors:       baselineAnchorIDs(videos),
			CheckedAt:     tsOf(checkedAt),
			Now:           tsOf(now),
		}); err != nil {
			return err
		}

		// Ghi trang vào việc đang chạy; hàng trống = việc đã dừng → rollback.
		if job != nil {
			lastID := ""
			if l := len(videos); l > 0 {
				lastID = videos[l-1].ID
			}
			var nextOrdinal *int32
			var newestID *string
			if discovery {
				next := int32(ordinal0 + len(videos))
				nextOrdinal = &next
				if in.NewestVideoID != nil {
					v := valueOf(in.NewestVideoID)
					newestID = &v
				}
			}
			n, err := q.ScanCommitJobPage(ctx, gen.ScanCommitJobPageParams{
				ID:          uuidOf(job.ID),
				ChannelID:   in.ChannelID,
				Cursor:      nargText(rawCursor),
				Remainder:   nargText(rawRemainder),
				LastID:      nargText(lastID),
				SeenTarget:  job.ReplayTarget != "" && (job.ReplayTarget == lastID || containsID(ids, job.ReplayTarget)),
				Scanned:     int32(len(videos)),
				Added:       int32(added),
				Ended:       ended,
				NextOrdinal: int32Narg(nextOrdinal),
				NewestID:    nargText(valueOf(newestID)),
				Now:         tsOf(now),
			})
			if err != nil {
				return err
			}
			if n == 0 {
				return jobStopped()
			}
			out.JobID = job.ID
		}

		out.Scanned = len(videos)
		out.Added = added
		out.Overlap = overlap != nil
		out.NextOrdinal = ordinal0
		if discovery {
			out.NextOrdinal = ordinal0 + len(videos)
		}
		return nil
	})
	return out, err
}

// lockStateTx tạo lười rồi giữ khoá hàng trạng thái (parent đã khoá trước) và
// trả mốc thời gian đọc sau khoá — mốc dùng để xét fence.
func (s *ScanStore) lockStateTx(ctx context.Context, q *gen.Queries, channelID string) (scan.State, time.Time, error) {
	row, err := q.ScanEnsureState(ctx, channelID)
	if asErrNoRows(err) {
		return scan.State{}, time.Time{}, missingChannel()
	}
	if err != nil {
		return scan.State{}, time.Time{}, err
	}
	return stateFromRow(row), time.Now(), nil
}

// serializeContinuation là cursor/remainder của trang ở dạng lưu bền (rỗng khi
// không còn chỗ đi tiếp).
func serializeContinuation(next *youtube.ChannelCursor, remainder []youtube.ScannedVideo) (string, string, error) {
	var rawCursor, rawRemainder string
	if next != nil {
		raw, err := youtube.SerializeChannelCursor(*next)
		if err != nil {
			return "", "", err
		}
		rawCursor = raw
	}
	if len(remainder) > 0 {
		raw, err := youtube.SerializeScannedVideos(remainder)
		if err != nil {
			return "", "", err
		}
		rawRemainder = raw
	}
	return rawCursor, rawRemainder, nil
}

// baselineAnchorIDs là anchors của lần ghi đầu kênh: id trang baseline theo
// thứ tự kênh, tối đa 150 (giống ensureState).
func baselineAnchorIDs(videos []youtube.ScannedVideo) []string {
	out := make([]string, 0, len(videos))
	for _, v := range videos {
		if len(out) >= scan.HeadAnchorMax {
			break
		}
		out = append(out, v.ID)
	}
	return out
}

// videoRef là một hàng đã biết của bảng video.
type videoRef struct {
	id        string
	channelID string
	position  int64
}

func (s *ScanStore) videoRefsTx(ctx context.Context, q *gen.Queries, ids []string) (map[string]videoRef, error) {
	out := map[string]videoRef{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.ScanVideoRefs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = videoRef{id: r.ID, channelID: r.ChannelID, position: r.Position}
	}
	return out, nil
}

func (s *ScanStore) upsertVideoTx(ctx context.Context, q *gen.Queries, channelID string, v youtube.ScannedVideo, position int64, origin domain.ScanOrigin, now, checkedAt time.Time, median *int64) error {
	medianVal := valueOfI64(median)
	return q.ScanUpsertVideo(ctx, gen.ScanUpsertVideoParams{
		ID:            v.ID,
		ChannelID:     channelID,
		Title:         v.Title,
		DurationSec:   int4Of(v.DurationSec),
		ViewCount:     int8Of(v.ViewCount),
		PublishedText: textOf(v.PublishedText),
		PublishedAt:   timestamptzNarg(v.PublishedAt),
		Median:        medianVal,
		MedianF:       float64(medianVal),
		Position:      position,
		Origin:        string(origin),
		FirstSeen:     tsOf(checkedAt),
		Updated:       tsOf(now),
	})
}

// ── head check (fenced, một giao dịch) ─────────────────────────────────────

// CompleteHeadCheck chốt một lượt kiểm tra đầu kênh: mức thường từ 150 hàng
// mới nhất, tính lại điểm cả kênh khi mức thường đổi, anchors + mốc "đã đuổi
// kịp", và (nếu truyền) việc discovery đang chạy đi tới complete — tất cả trong
// một tx dưới fence.
func (s *ScanStore) CompleteHeadCheck(ctx context.Context, channelID string, in scan.HeadCheckInput) (scan.HeadCheckResult, error) {
	var out scan.HeadCheckResult
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	err := s.withTx(ctx, func(q *gen.Queries) error {
		state, fenceNow, err := s.lockChannelStateTx(ctx, q, channelID)
		if err != nil {
			return err
		}
		if err := leaseHeld(state, in.Lease, fenceNow); err != nil {
			return err
		}
		channel, err := q.ScanGetChannel(ctx, channelID)
		if asErrNoRows(err) {
			return missingChannel()
		}
		if err != nil {
			return err
		}
		median, err := s.medianViewsTx(ctx, q, channelID)
		if err != nil {
			return err
		}
		var old *int64
		if channel.MedianViews.Valid {
			v := channel.MedianViews.Int64
			old = &v
		}
		if err := q.ScanUpdateChannelHead(ctx, gen.ScanUpdateChannelHeadParams{
			ID:          channelID,
			MedianViews: pgtype.Int8{Int64: valueOfI64(median), Valid: median != nil},
			At:          tsOf(in.CheckedAt),
		}); err != nil {
			return err
		}
		// Điểm cả kênh chỉ tính lại khi mức thường thật sự đổi: trang đã upsert
		// hàng chạm tới với mức hiện tại, nên mức không đổi thì không có gì lệch.
		if !sameMedian(old, median) {
			if _, err := q.ScanRecomputeScores(ctx, gen.ScanRecomputeScoresParams{
				ChannelID: channelID,
				Median:    valueOfI64(median),
				MedianF:   float64(valueOfI64(median)),
			}); err != nil {
				return err
			}
		}
		anchors := in.Anchors
		if len(anchors) == 0 {
			rows, err := q.ScanRecentAnchorIDs(ctx, channelID)
			if err != nil {
				return err
			}
			anchors = rows
		}
		if len(anchors) > scan.HeadAnchorMax {
			anchors = anchors[:scan.HeadAnchorMax]
		}
		var newest *string
		if in.NewestID != nil {
			newest = in.NewestID
		} else if len(anchors) > 0 {
			newest = &anchors[0]
		}
		if err := q.ScanSetHeadState(ctx, gen.ScanSetHeadStateParams{
			ChannelID: channelID,
			Anchors:   anchors,
			NewestID:  nargText(valueOf(newest)),
			CheckedAt: tsOf(in.CheckedAt),
			Now:       tsOf(now),
		}); err != nil {
			return err
		}
		if in.JobID != "" {
			n, err := q.ScanFinishDiscoveryJob(ctx, gen.ScanFinishDiscoveryJobParams{
				ID:        uuidOf(in.JobID),
				ChannelID: channelID,
				Now:       tsOf(now),
			})
			if err != nil {
				return err
			}
			if n == 0 {
				return jobStopped()
			}
			if err := pruneFinishedTx(ctx, q, channelID, domain.ScanDiscovery); err != nil {
				return err
			}
		}
		out.MedianViews = median
		out.CheckedAt = in.CheckedAt
		return nil
	})
	return out, err
}

// sameMedian so sánh null-safe hai mức thường.
func sameMedian(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// medianViewsTx là mức thường: trung vị view của 150 hàng mới nhất theo thứ
// tự kênh (hàng thiếu lượt xem rơi khỏi mẫu trung vị).
func (s *ScanStore) medianViewsTx(ctx context.Context, q *gen.Queries, channelID string) (*int64, error) {
	rows, err := q.ScanMedianSample(ctx, channelID)
	if err != nil {
		return nil, err
	}
	values := make([]*float64, 0, len(rows))
	for _, r := range rows {
		if !r.Valid {
			values = append(values, nil)
			continue
		}
		v := float64(r.Int64)
		values = append(values, &v)
	}
	m := youtube.Median(values)
	if m == nil {
		return nil, nil
	}
	rounded := int64(*m + 0.5)
	return &rounded, nil
}

// ── trạng thái + điều khiển lịch sử ────────────────────────────────────────

func (s *ScanStore) Status(ctx context.Context, channelID string) (scan.StatusSnapshot, error) {
	var snap scan.StatusSnapshot
	channel, err := s.Channel(ctx, channelID)
	if err != nil {
		return snap, err
	}
	snap.Channel = channel
	state, err := s.EnsureState(ctx, channelID)
	if err != nil {
		return snap, err
	}
	snap.State = state
	snap.HasState = true
	if live, err := s.q.ScanLiveJob(ctx, gen.ScanLiveJobParams{ChannelID: channelID, Kind: string(domain.ScanHistory)}); err == nil {
		snap.HistoryJob = jobFromRow(live)
	} else if asErrNoRows(err) {
		if latest, lerr := s.q.ScanLatestJob(ctx, gen.ScanLatestJobParams{ChannelID: channelID, Kind: string(domain.ScanHistory)}); lerr == nil {
			snap.HistoryJob = jobFromRow(latest)
		} else if !asErrNoRows(lerr) {
			return snap, lerr
		}
	} else {
		return snap, err
	}
	if live, err := s.q.ScanLiveJob(ctx, gen.ScanLiveJobParams{ChannelID: channelID, Kind: string(domain.ScanDiscovery)}); err == nil {
		snap.DiscoveryJob = jobFromRow(live)
	} else if !asErrNoRows(err) {
		return snap, err
	}
	live, err := s.workerLeaseLive(ctx)
	if err != nil {
		return snap, err
	}
	snap.WorkerLive = live
	return snap, nil
}

// workerLeaseLive cho Status biết có worker nào đang giữ lease toàn cục.
func (s *ScanStore) workerLeaseLive(ctx context.Context) (bool, error) {
	return s.q.ScanWorkerLeaseLive(ctx, tsOf(time.Now()))
}

// StartBackfill tạo (hoặc trả việc đang sống) của quét lịch sử. Tab đã hết
// thì không có gì để quét: trả nil, không tạo việc. Việc dừng được đưa về hàng
// chờ như bản TS (paused/error; capped/cancelled khi chỉ số còn thiếu cap).
func (s *ScanStore) StartBackfill(ctx context.Context, channelID string) (*scan.Job, error) {
	var out *scan.Job
	err := s.withTx(ctx, func(q *gen.Queries) error {
		state, _, err := s.lockChannelStateTx(ctx, q, channelID)
		if err != nil {
			return err
		}
		if state.HistoryEnd {
			return nil
		}
		now := time.Now()
		job, err := s.liveJobTx(ctx, q, channelID, domain.ScanHistory)
		if err != nil {
			return err
		}
		if job == nil {
			// Không còn việc sống: chỉ việc cancelled vừa dừng là còn tiếp được.
			latest, err := s.latestJobTx(ctx, q, channelID, domain.ScanHistory)
			if err != nil {
				return err
			}
			if latest != nil && latest.State == domain.ScanCancelled {
				job = latest
			}
		}
		if job != nil && job.State != domain.ScanComplete {
			switch job.State {
			case domain.ScanQueued, domain.ScanRunning, domain.ScanBackoff:
				out = job
				return nil
			case domain.ScanPaused, domain.ScanError, domain.ScanCancelled:
				if err := q.ScanRequeueJob(ctx, gen.ScanRequeueJobParams{ID: uuidOf(job.ID), Now: tsOf(now)}); err != nil {
					return err
				}
				job.State = domain.ScanQueued
				out = job
				return nil
			case domain.ScanCapped:
				if state.IndexedVideos < state.HistoryCap {
					if err := q.ScanRequeueJob(ctx, gen.ScanRequeueJobParams{ID: uuidOf(job.ID), Now: tsOf(now)}); err != nil {
						return err
					}
					job.State = domain.ScanQueued
				}
				out = job
				return nil
			}
			out = job
			return nil
		}
		if state.IndexedVideos >= state.HistoryCap {
			return &scan.StoreError{Code: scan.StoreAlreadyCapped,
				Message: fmt.Sprintf("History cap already reached (%d)", state.HistoryCap)}
		}
		cursor, cursorErr := resumableCursor(state)
		if cursorErr != nil {
			return cursorErr
		}
		row, err := q.ScanInsertHistoryJob(ctx, gen.ScanInsertHistoryJobParams{
			ID:        uuidOf(scanNewUUID()),
			ChannelID: channelID,
			State:     string(domain.ScanQueued),
			Cap:       pgtype.Int4{Int32: int32(state.HistoryCap), Valid: true},
			Cursor:    nargText(cursor),
			Remainder: nargText(state.HistoryRemainder),
			Now:       tsOf(now),
		})
		if err != nil {
			return err
		}
		if err := q.ScanClearHistoryBookmark(ctx, gen.ScanClearHistoryBookmarkParams{
			ChannelID: channelID,
			Now:       tsOf(now),
		}); err != nil {
			return err
		}
		if err := pruneFinishedTx(ctx, q, channelID, domain.ScanHistory); err != nil {
			return err
		}
		out = jobFromRow(row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// pruneFinishedTx dọn các việc đã dừng của một kênh/loại, chừa lại đúng dòng
// đã dừng mới nhất — chạy trong giao dịch vừa chèn lượt mới nên số dòng của
// một kênh không lớn dần theo số lần thay lượt hay số giờ chạy. Việc còn sống
// không bị đụng tới, và dòng giữ lại đúng là dòng mà danh sách đối thủ đọc.
func pruneFinishedTx(ctx context.Context, q *gen.Queries, channelID string, kind domain.ScanJobKind) error {
	return q.ScanPruneFinishedJobs(ctx, gen.ScanPruneFinishedJobsParams{
		ChannelID: channelID,
		Kind:      string(kind),
	})
}

// liveJobTx là việc còn sống của một loại (cùng tập trạng thái với chỉ mục
// duy nhất) — nil khi không có.
func (s *ScanStore) liveJobTx(ctx context.Context, q *gen.Queries, channelID string, kind domain.ScanJobKind) (*scan.Job, error) {
	row, err := q.ScanLiveJob(ctx, gen.ScanLiveJobParams{ChannelID: channelID, Kind: string(kind)})
	if asErrNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return jobFromRow(row), nil
}

// latestJobTx là việc gần nhất của một loại kể cả đã xong.
func (s *ScanStore) latestJobTx(ctx context.Context, q *gen.Queries, channelID string, kind domain.ScanJobKind) (*scan.Job, error) {
	row, err := q.ScanLatestJob(ctx, gen.ScanLatestJobParams{ChannelID: channelID, Kind: string(kind)})
	if asErrNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return jobFromRow(row), nil
}

// resumableCursor kiểm + trả cursor đã lưu cho lần tiếp tục.
func resumableCursor(state scan.State) (string, error) {
	if state.HistoryCursor == "" {
		return "", nil
	}
	if len(state.HistoryCursor) > scan.CursorMaxBytes {
		return "", &scan.StoreError{Code: scan.StoreCursorInvalid, Message: "Stored scan cursor is too large"}
	}
	cursor, err := youtube.ParseChannelCursor(state.HistoryCursor)
	if err != nil {
		return "", &scan.StoreError{Code: scan.StoreCursorInvalid, Message: err.Error()}
	}
	raw, err := youtube.SerializeChannelCursor(cursor)
	if err != nil {
		return "", &scan.StoreError{Code: scan.StoreCursorInvalid, Message: err.Error()}
	}
	if len(raw) > scan.CursorMaxBytes {
		return "", &scan.StoreError{Code: scan.StoreCursorInvalid, Message: "Stored scan cursor is too large"}
	}
	return raw, nil
}

// SetHistoryCap đổi giới hạn việc lịch sử: việc capped mà chỉ số còn thiếu
// được mở lại (requeue); việc còn chờ mà chỉ số đã đủ bị hạ về capped.
func (s *ScanStore) SetHistoryCap(ctx context.Context, channelID string, cap int) error {
	return s.withTx(ctx, func(q *gen.Queries) error {
		state, _, err := s.lockChannelStateTx(ctx, q, channelID)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := q.ScanUpdateHistoryCap(ctx, gen.ScanUpdateHistoryCapParams{
			ChannelID: channelID,
			Cap:       int32(cap),
			Now:       tsOf(now),
		}); err != nil {
			return err
		}
		job, err := s.liveJobTx(ctx, q, channelID, domain.ScanHistory)
		if err != nil || job == nil {
			return err
		}
		if err := q.ScanSetRequestedCap(ctx, gen.ScanSetRequestedCapParams{
			ID:  uuidOf(job.ID),
			Cap: pgtype.Int4{Int32: int32(cap), Valid: true},
			Now: tsOf(now),
		}); err != nil {
			return err
		}
		if job.State == domain.ScanCapped && state.IndexedVideos < cap {
			return q.ScanRequeueJob(ctx, gen.ScanRequeueJobParams{ID: uuidOf(job.ID), Now: tsOf(now)})
		}
		if (job.State == domain.ScanQueued || job.State == domain.ScanBackoff) && state.IndexedVideos >= cap {
			return q.ScanMarkJobCapped(ctx, gen.ScanMarkJobCappedParams{ID: uuidOf(job.ID), Now: tsOf(now)})
		}
		return nil
	})
}

// ControlBackfill là điều khiển của người dùng lên việc lịch sử: pause khi
// đang hàng chờ/chạy/chờ lại; resume đưa việc dừng (paused/error/capped còn
// thiếu cap, cancelled còn thiếu cap) về hàng chờ; cancel dừng việc.
func (s *ScanStore) ControlBackfill(ctx context.Context, channelID string, action domain.ScanAction) error {
	return s.withTx(ctx, func(q *gen.Queries) error {
		state, _, err := s.lockChannelStateTx(ctx, q, channelID)
		if err != nil {
			return err
		}
		now := time.Now()
		job, err := s.liveJobTx(ctx, q, channelID, domain.ScanHistory)
		if err != nil {
			return err
		}
		if job == nil {
			if action != domain.ScanActionResume {
				return nil // không có việc: pause/cancel là no-op
			}
			latest, err := s.latestJobTx(ctx, q, channelID, domain.ScanHistory)
			if err != nil || latest == nil || latest.State != domain.ScanCancelled {
				return err
			}
			job = latest
		}
		switch action {
		case domain.ScanActionPause:
			if job.State == domain.ScanQueued || job.State == domain.ScanRunning || job.State == domain.ScanBackoff {
				return q.ScanPauseJob(ctx, gen.ScanPauseJobParams{ID: uuidOf(job.ID), Now: tsOf(now)})
			}
			return nil
		case domain.ScanActionResume:
			switch job.State {
			case domain.ScanPaused, domain.ScanError:
				return q.ScanRequeueJob(ctx, gen.ScanRequeueJobParams{ID: uuidOf(job.ID), Now: tsOf(now)})
			case domain.ScanCapped, domain.ScanCancelled:
				if state.IndexedVideos >= state.HistoryCap {
					return &scan.StoreError{Code: scan.StoreAlreadyCapped,
						Message: fmt.Sprintf("History cap already reached (%d)", state.HistoryCap)}
				}
				return q.ScanRequeueJob(ctx, gen.ScanRequeueJobParams{ID: uuidOf(job.ID), Now: tsOf(now)})
			}
			return nil
		case domain.ScanActionCancel:
			return q.ScanCancelJob(ctx, gen.ScanCancelJobParams{ID: uuidOf(job.ID), Now: tsOf(now)})
		}
		return nil
	})
}

// StartDiscovery xếp một lượt kiểm tra video mới. Lịch tự động (replaceStopped
// false) không đụng việc còn sống — kể cả paused/error, những việc chờ người
// dùng quyết định. Nút check/Tiếp tục (replaceStopped true) đưa việc
// paused/error/capped về hàng chờ, hoặc tạo run mới khi chỉ còn việc đã huỷ.
func (s *ScanStore) StartDiscovery(ctx context.Context, channelID string, replaceStopped bool) (bool, error) {
	created := false
	err := s.withTx(ctx, func(q *gen.Queries) error {
		state, _, err := s.lockChannelStateTx(ctx, q, channelID)
		if err != nil {
			return err
		}
		now := time.Now()
		live, err := s.liveJobTx(ctx, q, channelID, domain.ScanDiscovery)
		if err != nil {
			return err
		}
		if live != nil {
			switch live.State {
			case domain.ScanQueued, domain.ScanRunning, domain.ScanBackoff:
				return nil
			}
			if !replaceStopped {
				return nil
			}
			// Bỏ hẳn lượt đã dừng (giữ dòng cũ làm lịch sử) rồi khoá khối mới:
			// lượt mới kiểm tra lại từ đầu head hiện tại, không mang ordinal/
			// cursor/anchors của lượt hỏng sang (một run gap_incomplete đã chạm
			// 10.000 không thể tiếp tục chính nó). Vẫn trong khoá parent/state
			// nên không ai chen giữa hai bước.
			if err := q.ScanCancelJob(ctx, gen.ScanCancelJobParams{ID: uuidOf(live.ID), Now: tsOf(now)}); err != nil {
				return err
			}
		}
		base, anchors, err := s.reserveDiscoveryBlockTx(ctx, q, channelID, state)
		if err != nil {
			return err
		}
		if _, err := q.ScanInsertDiscoveryJob(ctx, gen.ScanInsertDiscoveryJobParams{
			ID:        uuidOf(scanNewUUID()),
			ChannelID: channelID,
			OrderBase: pgtype.Int8{Int64: base, Valid: true},
			Anchors:   anchors,
			Now:       tsOf(now),
		}); err != nil {
			return err
		}
		if err := pruneFinishedTx(ctx, q, channelID, domain.ScanDiscovery); err != nil {
			return err
		}
		created = true
		return nil
	})
	return created, err
}

// reserveDiscoveryBlockTx khoá trước một khối 10.000 khoá ngay dưới vị trí nhỏ
// nhất của kênh, và trả anchors của run (head đã lưu ∪ hàng mới nhất không đến
// từ quét lịch sử). Khối giữ nguyên suốt run, kể cả khi tạm dừng/khởi động
// lại; chỉ khi khối sắp chạm miền an toàn mới dồn lại thứ tự một lần rồi khoá
// khối mới — nhờ vậy các trang của một run không bao giờ phải đánh số lại.
func (s *ScanStore) reserveDiscoveryBlockTx(ctx context.Context, q *gen.Queries, channelID string, state scan.State) (int64, []string, error) {
	min, err := q.ScanMinPosition(ctx, channelID)
	if err != nil {
		return 0, nil, err
	}
	if min-scan.DiscoveryBlock < -scan.KeyLimit {
		if err := q.ScanRebaseOrder(ctx, channelID); err != nil {
			return 0, nil, err
		}
		min = 0
	}
	anchors, err := s.discoveryAnchorsTx(ctx, q, channelID, state)
	if err != nil {
		return 0, nil, err
	}
	return min - scan.DiscoveryBlock, anchors, nil
}

// discoveryAnchorsTx hợp nhất head đã lưu với các hàng mới nhất không đến từ
// quét lịch sử, cắt còn AnchorMax.
func (s *ScanStore) discoveryAnchorsTx(ctx context.Context, q *gen.Queries, channelID string, state scan.State) ([]string, error) {
	recent, err := q.ScanRecentAnchorIDs(ctx, channelID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(state.HeadAnchors)+len(recent))
	for _, id := range append(append([]string{}, state.HeadAnchors...), recent...) {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		if len(out) >= scan.AnchorMax {
			break
		}
	}
	return out, nil
}

// ── mặt worker ─────────────────────────────────────────────────────────────

func (s *ScanStore) NextRunnableJob(ctx context.Context, skip map[string]bool) (*scan.Job, error) {
	// Kênh bận được loại trong câu truy vấn, trước LIMIT: lọc sau thì 50 việc
	// của kênh bận sẽ che mất việc của kênh rảnh phía sau.
	rows, err := s.q.ScanRunnableJobs(ctx, gen.ScanRunnableJobsParams{
		Now:  tsOf(time.Now()),
		Skip: skipChannels(skip),
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return jobFromRow(rows[0]), nil
}

// skipChannels chuyển tập kênh bận sang mảng cho câu truy vấn; nil và mảng
// rỗng đều nghĩa là không bỏ kênh nào.
func skipChannels(skip map[string]bool) []string {
	if len(skip) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(skip))
	for id, busy := range skip {
		if busy {
			out = append(out, id)
		}
	}
	sort.Strings(out) // thứ tự cố định để câu truy vấn ổn định khi chạy lại
	return out
}

// NextRunnableJobForChannel lọc ngay trong câu truy vấn: CLI chạy nội tuyến chỉ
// thấy việc của kênh mình (kênh khác không chen được vào lượt chọn).
func (s *ScanStore) NextRunnableJobForChannel(ctx context.Context, channelID string) (*scan.Job, error) {
	row, err := s.q.ScanRunnableJobForChannel(ctx, gen.ScanRunnableJobForChannelParams{
		ChannelID: channelID,
		Now:       tsOf(time.Now()),
	})
	if asErrNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return jobFromRow(row), nil
}

func (s *ScanStore) Job(ctx context.Context, jobID string) (*scan.Job, error) {
	row, err := s.q.ScanGetJob(ctx, uuidOf(jobID))
	if asErrNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return jobFromRow(row), nil
}

// ClaimJob đánh dấu việc running dưới fence: khoá kênh rồi trạng thái, kiểm
// token, rồi mới tới hàng việc.
func (s *ScanStore) ClaimJob(ctx context.Context, jobID string, lease scan.Lease) (*scan.Job, error) {
	var out *scan.Job
	now := time.Now()
	err := s.withTx(ctx, func(q *gen.Queries) error {
		row, err := q.ScanGetJob(ctx, uuidOf(jobID))
		if asErrNoRows(err) {
			return jobStopped()
		}
		if err != nil {
			return err
		}
		job := jobFromRow(row)
		if job.ChannelID != lease.ChannelID {
			return jobStopped()
		}
		state, fenceNow, err := s.lockChannelStateTx(ctx, q, lease.ChannelID)
		if err != nil {
			return err
		}
		if err := leaseHeld(state, lease, fenceNow); err != nil {
			return err
		}
		claimed, err := q.ScanClaimJob(ctx, gen.ScanClaimJobParams{
			ID:  uuidOf(jobID),
			Now: tsOf(now),
		})
		if asErrNoRows(err) {
			return nil // không runnable nữa
		}
		if err != nil {
			return err
		}
		out = jobFromRow(claimed)
		return nil
	})
	return out, err
}

// fencedJob là khung của mọi ghi transition của worker: khoá parent rồi trạng
// thái, việc còn tồn tại, thuộc kênh của lease, fence còn hiệu lực — rồi mới
// chạy ghi (câu ghi còn tự đòi việc đang running).
func (s *ScanStore) fencedJob(ctx context.Context, jobID string, lease scan.Lease, write func(q *gen.Queries) error) error {
	return s.withTx(ctx, func(q *gen.Queries) error {
		row, err := q.ScanGetJob(ctx, uuidOf(jobID))
		if asErrNoRows(err) {
			return jobStopped()
		}
		if err != nil {
			return err
		}
		if jobFromRow(row).ChannelID != lease.ChannelID {
			return jobStopped()
		}
		state, fenceNow, err := s.lockChannelStateTx(ctx, q, lease.ChannelID)
		if err != nil {
			return err
		}
		if err := leaseHeld(state, lease, fenceNow); err != nil {
			return err
		}
		return write(q)
	})
}

func (s *ScanStore) SetJobBackoff(ctx context.Context, jobID string, lease scan.Lease, b scan.Backoff) error {
	return s.fencedJob(ctx, jobID, lease, func(q *gen.Queries) error {
		n, err := q.ScanSetJobBackoff(ctx, gen.ScanSetJobBackoffParams{
			ID:        uuidOf(jobID),
			Attempts:  int32(b.Attempts),
			NextAt:    tsOf(b.NextAttemptAt),
			ErrorCode: b.ErrorCode,
			LastError: scan.SanitizeError(b.Err),
			Recovery:  b.Recovery,
			Now:       tsOf(time.Now()),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return jobStopped()
		}
		return nil
	})
}

func (s *ScanStore) SetJobError(ctx context.Context, jobID string, lease scan.Lease, e scan.JobError) error {
	return s.fencedJob(ctx, jobID, lease, func(q *gen.Queries) error {
		var attempts pgtype.Int4
		if e.Attempts != nil {
			attempts = pgtype.Int4{Int32: int32(*e.Attempts), Valid: true}
		}
		n, err := q.ScanSetJobError(ctx, gen.ScanSetJobErrorParams{
			ID:        uuidOf(jobID),
			Attempts:  attempts,
			ErrorCode: e.ErrorCode,
			LastError: scan.SanitizeError(e.Err),
			Recovery:  e.Recovery,
			Now:       tsOf(time.Now()),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return jobStopped()
		}
		return nil
	})
}

func (s *ScanStore) SetJobCapped(ctx context.Context, jobID string, lease scan.Lease) error {
	return s.fencedJob(ctx, jobID, lease, func(q *gen.Queries) error {
		n, err := q.ScanSetJobCapped(ctx, gen.ScanSetJobCappedParams{
			ID:  uuidOf(jobID),
			Now: tsOf(time.Now()),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return jobStopped()
		}
		return nil
	})
}

func (s *ScanStore) SetJobPaused(ctx context.Context, jobID string, lease scan.Lease, e scan.JobError) error {
	return s.fencedJob(ctx, jobID, lease, func(q *gen.Queries) error {
		n, err := q.ScanSetJobPaused(ctx, gen.ScanSetJobPausedParams{
			ID:        uuidOf(jobID),
			ErrorCode: e.ErrorCode,
			LastError: scan.SanitizeError(e.Err),
			Recovery:  e.Recovery,
			Now:       tsOf(time.Now()),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return jobStopped()
		}
		return nil
	})
}

func (s *ScanStore) CompleteJob(ctx context.Context, jobID string, lease scan.Lease) error {
	return s.fencedJob(ctx, jobID, lease, func(q *gen.Queries) error {
		n, err := q.ScanCompleteJob(ctx, gen.ScanCompleteJobParams{
			ID:  uuidOf(jobID),
			Now: tsOf(time.Now()),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return jobStopped()
		}
		return pruneFinishedTx(ctx, q, lease.ChannelID, domain.ScanHistory)
	})
}

func (s *ScanStore) RestartJobCursor(ctx context.Context, jobID string, lease scan.Lease, cause error) error {
	return s.fencedJob(ctx, jobID, lease, func(q *gen.Queries) error {
		n, err := q.ScanRestartJobCursor(ctx, gen.ScanRestartJobCursorParams{
			ID:        uuidOf(jobID),
			LastError: scan.SanitizeError(cause),
			Now:       tsOf(time.Now()),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return jobStopped()
		}
		return nil
	})
}

func (s *ScanStore) FindKnownVideoIDs(ctx context.Context, channelID string, ids []string) (map[string]bool, error) {
	known := map[string]bool{}
	const chunk = 500
	for start := 0; start < len(ids); start += chunk {
		end := start + chunk
		if end > len(ids) {
			end = len(ids)
		}
		rows, err := s.q.ScanVideoRefs(ctx, ids[start:end])
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if r.ChannelID == channelID {
				known[r.ID] = true
			}
		}
	}
	return known, nil
}

// SyncScanTask ghi dòng Việc nền của một việc quét trong cùng giao dịch, theo
// đúng thứ tự khoá parent → state → job: kênh và việc phải còn sống, dòng việc
// được tạo mới hoặc nhận lại (giữ nguyên tiến độ đang hiện), rồi nối vào việc
// — 0 dòng nối là job_stopped và cuốn theo cả dòng vừa tạo. Nhờ vậy lệnh gỡ
// kênh không thể chen giữa tạo và nối để bỏ lại một dòng mồ côi.
func (s *ScanStore) SyncScanTask(ctx context.Context, jobID, channelID string, spec domain.TaskSpec) (scan.TaskSync, error) {
	var out scan.TaskSync
	err := s.withTxConn(ctx, func(tx pgx.Tx, q *gen.Queries) error {
		if _, _, err := s.lockChannelStateTx(ctx, q, channelID); err != nil {
			return err
		}
		row, err := q.ScanGetJob(ctx, uuidOf(jobID))
		if asErrNoRows(err) {
			return jobStopped()
		}
		if err != nil {
			return err
		}
		job := jobFromRow(row)
		if job.ChannelID != channelID {
			return jobStopped()
		}
		// Nội dung hiển thị lấy từ việc đang bị khoá, không lấy từ ảnh chụp của
		// nơi gọi: việc đã dừng (hết/huỷ/chạm trần) thì không ghi dòng nào, còn
		// trạng thái, câu mô tả và nút bấm phải khớp việc ngay lúc ghi — một
		// ảnh chụp "đang chạy" cũ không thể dựng lại dòng running cho việc vừa
		// bị tạm dừng. Nhờ vậy tiến độ của trang cũng không kéo việc đã tạm
		// dừng trở lại thành đang chạy.
		view, live := scan.TaskPresentation(*job)
		if !live {
			return jobStopped()
		}
		spec.Phase = view.Phase
		spec.Action = view.Action
		arg, err := scanTaskParams(spec, view.State)
		if err != nil {
			return err
		}
		found, err := q.FindTaskByObject(ctx, gen.FindTaskByObjectParams{
			Kind:      string(spec.Kind),
			ObjectRef: arg.ObjectRef,
		})
		taskID := ""
		previous := "" // trạng thái dòng Việc nền trước lần ghi này
		switch {
		case err == nil:
			taskID = uuidText(found.ID)
			previous = found.State
			adopt := gen.AdoptTaskParams{
				Agent: arg.Agent, Title: arg.Title, ObjectRef: arg.ObjectRef,
				Phase: arg.Phase, Progress: arg.Progress, EtaSec: arg.EtaSec,
				State: arg.State, Action: arg.Action, OwnerPid: arg.OwnerPid, ID: found.ID,
			}
			if err := q.AdoptTask(ctx, adopt); err != nil {
				return err
			}
		case asErrNoRows(err):
			created, err := q.ScanInsertTask(ctx, gen.ScanInsertTaskParams{
				Kind: arg.Kind, Agent: arg.Agent, Title: arg.Title, ObjectRef: arg.ObjectRef,
				Phase: arg.Phase, Progress: arg.Progress, EtaSec: arg.EtaSec,
				State: arg.State, Action: arg.Action, OwnerPid: arg.OwnerPid,
			})
			if err != nil {
				return err
			}
			taskID = uuidText(created.ID)
		default:
			return err
		}
		n, err := q.ScanLinkTask(ctx, gen.ScanLinkTaskParams{ID: uuidOf(jobID), Task: uuidOf(taskID)})
		if err != nil {
			return err
		}
		if n == 0 {
			return jobStopped()
		}
		out = scan.TaskSync{TaskID: taskID, View: view}
		// Chỉ lần chuyển thật sự vào failed mới báo: dòng đã failed từ trước
		// (lần làm mới) hay ảnh chụp cũ sau khi việc đã chạy lại thì không báo
		// lại. Thông báo và dòng lỗi ghi ngay trong giao dịch này, cùng với
		// dòng việc, nên không thể báo hai lần hay báo mà thiếu việc.
		if view.State == domain.TaskFailed && previous != domain.TaskFailed {
			notice := scan.FailureNotice(spec.Title, channelID, view)
			row, err := upsertNotificationTx(ctx, tx, q,
				notice.Kind, notice.Text, notice.Href, notice.ActionLabel, activity.NotifyMergeWindow)
			if err != nil {
				return err
			}
			if err := insertErrorEventTx(ctx, q, activity.ErrorEvent{
				Kind: notice.Kind, What: notice.Text, Why: notice.Text,
				Href: notice.Href, ActionLabel: notice.ActionLabel, Object: &spec.Object,
			}); err != nil {
				return err
			}
			out.NotificationID = uuidText(row.ID)
		}
		return nil
	})
	if err != nil {
		return scan.TaskSync{}, err
	}
	return out, nil
}

type scanTaskArg struct {
	Kind      string
	Agent     pgtype.Text
	Title     string
	ObjectRef []byte
	Phase     string
	Progress  []byte
	EtaSec    pgtype.Int4
	State     string
	Action    []byte
	OwnerPid  int32
}

func scanTaskParams(spec domain.TaskSpec, state domain.TaskState) (scanTaskArg, error) {
	var arg scanTaskArg
	objectRef, err := json.Marshal(spec.Object)
	if err != nil {
		return arg, err
	}
	action, err := json.Marshal(spec.Action)
	if err != nil {
		return arg, err
	}
	arg = scanTaskArg{
		Kind:      string(spec.Kind),
		Title:     spec.Title,
		ObjectRef: objectRef,
		Phase:     spec.Phase,
		State:     string(state),
		Action:    action,
		OwnerPid:  int32(os.Getpid()),
	}
	if spec.Agent != "" {
		arg.Agent = pgtype.Text{String: spec.Agent, Valid: true}
	}
	if spec.Progress != nil {
		progress, err := json.Marshal(spec.Progress)
		if err != nil {
			return arg, err
		}
		arg.Progress = progress
	}
	return arg, nil
}

func (s *ScanStore) ClearTaskLink(ctx context.Context, jobID string) error {
	return s.q.ScanClearTaskLink(ctx, uuidOf(jobID))
}

// ── lịch + thăm dò giờ đăng ────────────────────────────────────────────────

func (s *ScanStore) DueChannels(ctx context.Context, rescanHours int, now time.Time) ([]string, error) {
	return s.q.ScanDueChannels(ctx, gen.ScanDueChannelsParams{
		Hours: int32(rescanHours),
		Now:   tsOf(now),
	})
}

// NextWakeup là lần đọc lịch duy nhất của vòng rảnh: Actionable báo có việc
// làm ngay (không thể bỏ sót việc do process khác tạo), Next là hạn tương lai
// gần nhất, kể cả fence sắp hết hạn của kênh còn việc/chờ thăm dò.
func (s *ScanStore) NextWakeup(ctx context.Context, rescanHours int, now time.Time) (scan.Wakeup, error) {
	row, err := s.q.ScanWakeup(ctx, gen.ScanWakeupParams{
		Hours: int32(rescanHours),
		Now:   tsOf(now),
	})
	if err != nil {
		return scan.Wakeup{}, err
	}
	out := scan.Wakeup{Actionable: row.Actionable}
	if row.NextAt.Valid {
		at := row.NextAt.Time
		out.Next = &at
	}
	return out, nil
}

func (s *ScanStore) NextUploadCandidate(ctx context.Context, now time.Time) (*scan.UploadCandidate, error) {
	row, err := s.q.ScanNextUploadCandidate(ctx, tsOf(now))
	if asErrNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &scan.UploadCandidate{VideoID: row.ID, ChannelID: row.ChannelID}, nil
}

// RecordUploadTime ghi kết quả đọc giờ đăng dưới fence: token phải còn hạn
// và video vẫn thuộc kênh của lease trong cùng giao dịch. Hàng đã đổi chủ
// hoặc biến mất thì không ghi gì (kết quả thăm dò cũ không được đè hàng mới).
func (s *ScanStore) RecordUploadTime(ctx context.Context, lease scan.Lease, videoID string, at *time.Time, hasCaptions bool, now time.Time) error {
	return s.withTx(ctx, func(q *gen.Queries) error {
		state, fenceNow, err := s.lockChannelStateTx(ctx, q, lease.ChannelID)
		if err != nil {
			return err
		}
		if err := leaseHeld(state, lease, fenceNow); err != nil {
			return err
		}
		_, err = q.ScanRecordUploadTime(ctx, gen.ScanRecordUploadTimeParams{
			ID:          videoID,
			ChannelID:   lease.ChannelID,
			Uploaded:    timestamptzNarg(at),
			Now:         tsOf(now),
			HasCaptions: hasCaptions,
		})
		return err
	})
}

func (s *ScanStore) PauseUploads(ctx context.Context, until time.Time) error {
	return s.q.ScanPauseUploads(ctx, tsOf(until))
}

func (s *ScanStore) BumpUploadFailures(ctx context.Context) (int, error) {
	n, err := s.q.ScanBumpUploadFailures(ctx)
	return int(n), err
}

func (s *ScanStore) ResetUploadFailures(ctx context.Context) error {
	return s.q.ScanResetUploadFailures(ctx)
}

// ── lease worker (platform_state 'youtube') ────────────────────────────────

// AcquireWorkerLease trả chính owner vừa ghi (gia hạn/nhả so với cột
// scanner_owner, không phải tên nền tảng).
func (s *ScanStore) AcquireWorkerLease(ctx context.Context, owner string, ttl time.Duration) (*scan.WorkerLease, error) {
	now := time.Now()
	until := now.Add(ttl)
	stored, err := s.q.ScanAcquireWorkerLease(ctx, gen.ScanAcquireWorkerLeaseParams{
		Owner: pgtype.Text{String: owner, Valid: true},
		Until: tsOf(until),
		Now:   tsOf(now),
	})
	if asErrNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &scan.WorkerLease{Owner: stored.String, Until: until}, nil
}

func (s *ScanStore) RenewWorkerLease(ctx context.Context, lease scan.WorkerLease, ttl time.Duration) (*scan.WorkerLease, error) {
	until := time.Now().Add(ttl)
	n, err := s.q.ScanRenewWorkerLease(ctx, gen.ScanRenewWorkerLeaseParams{
		Owner: pgtype.Text{String: lease.Owner, Valid: true},
		Until: tsOf(until),
	})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	return &scan.WorkerLease{Owner: lease.Owner, Until: until}, nil
}

func (s *ScanStore) ReleaseWorkerLease(ctx context.Context, lease scan.WorkerLease) error {
	return s.q.ScanReleaseWorkerLease(ctx, pgtype.Text{String: lease.Owner, Valid: true})
}
