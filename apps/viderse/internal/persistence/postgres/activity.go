package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
)

// ActivityStore là hiện thực Postgres của activity.Store trên background_tasks
// và notifications. Mỗi đối tượng (kind + object_ref) có tối đa một việc; gộp
// thông báo chạy trong transaction có advisory lock theo kind nên hai writer
// không thể cùng chèn hai dòng cho một cụm 10 giây.
type ActivityStore struct {
	db DBTXer
	q  *gen.Queries
}

// DBTXer là phần DBTX mà store cần thêm để mở transaction.
type DBTXer interface {
	gen.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// NewActivityStore dựng store trên pool/connection (cần Begin được).
func NewActivityStore(db DBTXer) *ActivityStore {
	return &ActivityStore{db: db, q: gen.New(db)}
}

var _ activity.Store = (*ActivityStore)(nil)

func uuidText(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}

func scanUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, fmt.Errorf("id việc nền không hợp lệ: %w", err)
	}
	return u, nil
}

func taskFromRow(r gen.BackgroundTask) activity.Task {
	t := activity.Task{
		ID:        uuidText(r.ID),
		Kind:      r.Kind,
		Agent:     r.Agent.String,
		Title:     r.Title,
		Phase:     r.Phase,
		State:     r.State,
		OwnerPID:  int(r.OwnerPid),
		CreatedAt: r.CreatedAt.Time,
		UpdatedAt: r.UpdatedAt.Time,
	}
	_ = json.Unmarshal(r.ObjectRef, &t.Object)
	if len(r.Progress) > 0 {
		var p domain.TaskProgress
		if err := json.Unmarshal(r.Progress, &p); err == nil {
			t.Progress = &p
		}
	}
	if r.EtaSec.Valid {
		eta := int(r.EtaSec.Int32)
		t.ETASec = &eta
	}
	_ = json.Unmarshal(r.Action, &t.Action)
	return t
}

func taskParams(spec domain.TaskSpec) (objectRef, progress, action []byte, eta pgtype.Int4, agent pgtype.Text, err error) {
	if objectRef, err = json.Marshal(spec.Object); err != nil {
		return nil, nil, nil, eta, agent, err
	}
	if spec.Progress != nil {
		if progress, err = json.Marshal(spec.Progress); err != nil {
			return nil, nil, nil, eta, agent, err
		}
	}
	if action, err = json.Marshal(spec.Action); err != nil {
		return nil, nil, nil, eta, agent, err
	}
	if spec.Agent != "" {
		agent = pgtype.Text{String: spec.Agent, Valid: true}
	}
	return objectRef, progress, action, eta, agent, nil
}

func (s *ActivityStore) FindTaskByObject(ctx context.Context, kind domain.TaskKind, object domain.ObjectRef) (*activity.Task, error) {
	raw, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	row, err := s.q.FindTaskByObject(ctx, gen.FindTaskByObjectParams{Kind: kind, ObjectRef: raw})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, activity.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t := taskFromRow(row)
	return &t, nil
}

func (s *ActivityStore) InsertTask(ctx context.Context, spec domain.TaskSpec) (*activity.Task, error) {
	objectRef, progress, action, eta, agent, err := taskParams(spec)
	if err != nil {
		return nil, err
	}
	row, err := s.q.InsertTask(ctx, gen.InsertTaskParams{
		Kind: spec.Kind, Agent: agent, Title: spec.Title,
		ObjectRef: objectRef, Phase: spec.Phase, Progress: progress,
		EtaSec: eta, Action: action, OwnerPid: int32(os.Getpid()),
	})
	if err != nil {
		return nil, err
	}
	t := taskFromRow(row)
	return &t, nil
}

func (s *ActivityStore) ResetTask(ctx context.Context, id string, spec domain.TaskSpec) error {
	objectRef, progress, action, eta, agent, err := taskParams(spec)
	if err != nil {
		return err
	}
	uid, err := scanUUID(id)
	if err != nil {
		return err
	}
	return s.q.ResetTask(ctx, gen.ResetTaskParams{
		Agent: agent, Title: spec.Title, ObjectRef: objectRef,
		Phase: spec.Phase, Progress: progress, EtaSec: eta, Action: action,
		OwnerPid: int32(os.Getpid()), ID: uid,
	})
}

// AdoptTask nhận lại một dòng đang sống: nội dung/trạng thái/owner được viết
// lại, tiến độ giữ nguyên trừ khi spec mang tiến độ mới.
func (s *ActivityStore) AdoptTask(ctx context.Context, id string, spec domain.TaskSpec) error {
	objectRef, progress, action, eta, agent, err := taskParams(spec)
	if err != nil {
		return err
	}
	uid, err := scanUUID(id)
	if err != nil {
		return err
	}
	return s.q.AdoptTask(ctx, gen.AdoptTaskParams{
		Agent: agent, Title: spec.Title, ObjectRef: objectRef,
		Phase: spec.Phase, Progress: progress, EtaSec: eta, State: string(domain.TaskRunning),
		Action: action, OwnerPid: int32(os.Getpid()), ID: uid,
	})
}

func (s *ActivityStore) UpdateTask(ctx context.Context, id string, u domain.TaskUpdate) error {
	uid, err := scanUUID(id)
	if err != nil {
		return err
	}
	arg := gen.UpdateTaskParams{ID: uid}
	if u.Phase != nil {
		arg.Phase = pgtype.Text{String: *u.Phase, Valid: true}
	}
	if u.Progress != nil {
		raw, err := json.Marshal(u.Progress)
		if err != nil {
			return err
		}
		arg.Progress = raw
	}
	if u.ETASec != nil {
		arg.EtaSec = pgtype.Int4{Int32: int32(*u.ETASec), Valid: true}
	}
	if u.State != "" {
		arg.State = string(u.State)
	}
	if u.Action != nil {
		raw, err := json.Marshal(u.Action)
		if err != nil {
			return err
		}
		arg.Action = raw
	}
	_, err = s.q.UpdateTask(ctx, arg)
	return err
}

func (s *ActivityStore) DeleteTask(ctx context.Context, id string) error {
	uid, err := scanUUID(id)
	if err != nil {
		return err
	}
	return s.q.DeleteTask(ctx, uid)
}

func (s *ActivityStore) ListTasks(ctx context.Context) ([]activity.Task, error) {
	rows, err := s.q.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]activity.Task, len(rows))
	for i, r := range rows {
		out[i] = taskFromRow(r)
	}
	return out, nil
}

func (s *ActivityStore) CountTasks(ctx context.Context, kind domain.TaskKind, state domain.TaskState) (int64, error) {
	return s.q.CountTasks(ctx, gen.CountTasksParams{Kind: kind, State: state})
}

func (s *ActivityStore) FailTasksByIDs(ctx context.Context, ids []string) ([]activity.Task, error) {
	uuids := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		u, err := scanUUID(id)
		if err != nil {
			return nil, err
		}
		uuids = append(uuids, u)
	}
	rows, err := s.q.FailTasksByIDs(ctx, uuids)
	if err != nil {
		return nil, err
	}
	out := make([]activity.Task, len(rows))
	for i, r := range rows {
		out[i] = taskFromRow(r)
	}
	return out, nil
}

func notificationFromRow(r gen.Notification) activity.Notification {
	n := activity.Notification{
		ID:          uuidText(r.ID),
		Kind:        r.Kind,
		Count:       int(r.Count),
		Text:        r.Text,
		Href:        r.Href,
		ActionLabel: r.ActionLabel,
		CreatedAt:   r.CreatedAt.Time,
	}
	if r.ReadAt.Valid {
		at := r.ReadAt.Time
		n.ReadAt = &at
	}
	return n
}

// UpsertNotification áp luật gộp 10 giây trong một transaction khoá theo kind
// (hashtext → advisory xact lock): hai writer đồng thời không thể cùng thấy
// "không có dòng trong cửa sổ" rồi cùng chèn. Trả về dòng đã ghi trong chính
// giao dịch đó — đọc lại sau commit có thể ra thông báo của writer khác.
func (s *ActivityStore) UpsertNotification(ctx context.Context, kind, text, href, actionLabel string, window time.Duration) (*activity.Notification, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row, err := upsertNotificationTx(ctx, tx, gen.New(tx), kind, text, href, actionLabel, window)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	n := notificationFromRow(*row)
	return &n, nil
}

// upsertNotificationTx ghi hoặc gộp một thông báo trong giao dịch sẵn có: khoá
// theo kind bằng advisory xact lock rồi áp luật gộp (cùng kind trong window thì
// tăng count và giữ created_at của lần đầu).
func upsertNotificationTx(ctx context.Context, tx pgx.Tx, q *gen.Queries, kind, text, href, actionLabel string, window time.Duration) (*gen.Notification, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, kind); err != nil {
		return nil, err
	}
	latest, err := q.LatestNotification(ctx, kind)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// không có dòng nào của kind — chèn mới bên dưới.
	case err != nil:
		return nil, err
	case time.Since(latest.CreatedAt.Time) <= window:
		uid, err := scanUUID(uuidText(latest.ID))
		if err != nil {
			return nil, err
		}
		if err := q.BumpNotification(ctx, gen.BumpNotificationParams{
			Text: text, Href: href, ActionLabel: actionLabel, ID: uid,
		}); err != nil {
			return nil, err
		}
		merged, err := q.LatestNotification(ctx, kind)
		if err != nil {
			return nil, err
		}
		return &merged, nil
	}
	row, err := q.InsertNotification(ctx, gen.InsertNotificationParams{
		Kind: kind, Text: text, Href: href, ActionLabel: actionLabel,
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListUnreadNotifications đọc thông báo chưa đọc, mới nhất trước.
func (s *ActivityStore) ListUnreadNotifications(ctx context.Context, limit int) ([]activity.Notification, error) {
	rows, err := s.q.ListUnreadNotifications(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]activity.Notification, 0, len(rows))
	for i, r := range rows {
		if i >= limit {
			break
		}
		out = append(out, notificationFromRow(r))
	}
	return out, nil
}

// MarkRead đánh dấu đã đọc; ids rỗng nghĩa là tất cả.
func (s *ActivityStore) MarkRead(ctx context.Context, ids []string) error {
	if ids == nil {
		ids = []string{} // nil = tất cả; JSON null không so được với '[]'
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return s.q.MarkNotificationsRead(ctx, raw)
}

// InsertErrorEvent ghi một lỗi bền (lịch sử 7 ngày của mục Báo lỗi).
func (s *ActivityStore) InsertErrorEvent(ctx context.Context, e activity.ErrorEvent) error {
	return insertErrorEventTx(ctx, s.q, e)
}

// insertErrorEventTx ghi một lần lỗi trong giao dịch sẵn có (đối tượng liên
// quan mã hoá thành object_ref).
func insertErrorEventTx(ctx context.Context, q *gen.Queries, e activity.ErrorEvent) error {
	var objectRef []byte
	if e.Object != nil {
		encoded, err := json.Marshal(e.Object)
		if err != nil {
			return err
		}
		objectRef = encoded
	}
	return q.InsertErrorEvent(ctx, gen.InsertErrorEventParams{
		Kind: e.Kind, What: e.What, Why: e.Why, Href: e.Href,
		ActionLabel: e.ActionLabel, Op: e.Op, ObjectRef: objectRef,
	})
}

// ListErrorEvents đọc lỗi từ since, mới nhất trước.
func (s *ActivityStore) ListErrorEvents(ctx context.Context, since time.Time, limit int) ([]activity.ErrorEvent, error) {
	rows, err := s.q.ListErrorEvents(ctx, gen.ListErrorEventsParams{
		Since: pgtype.Timestamptz{Time: since, Valid: true}, RowLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]activity.ErrorEvent, 0, len(rows))
	for _, r := range rows {
		ev := activity.ErrorEvent{
			ID: uuidText(r.ID), At: r.At.Time, Kind: r.Kind, What: r.What,
			Why: r.Why, Href: r.Href, ActionLabel: r.ActionLabel, Op: r.Op,
		}
		if len(r.ObjectRef) > 0 {
			var ref domain.ObjectRef
			if err := json.Unmarshal(r.ObjectRef, &ref); err == nil {
				ev.Object = &ref
			}
		}
		out = append(out, ev)
	}
	return out, nil
}

// CountErrorEvents đếm lỗi từ since.
func (s *ActivityStore) CountErrorEvents(ctx context.Context, since time.Time) (int, error) {
	n, err := s.q.CountErrorEvents(ctx, pgtype.Timestamptz{Time: since, Valid: true})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
