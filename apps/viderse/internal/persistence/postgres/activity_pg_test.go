// Store Postgres của việc nền §11: một đối tượng một việc, phục hồi sau sập,
// gộp thông báo 10 giây chịu được writer đồng thời.
//
// Chạy trên Postgres thật: provision schema riêng bằng pgtest với cặp role
// của Viderse rồi áp migration nhúng. Bỏ qua khi thiếu TEST_DATABASE_URL.
package postgres_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

func TestActivityStoreTaskLifecycleAndProgress(t *testing.T) {
	store := postgres.NewActivityStore(provision(t))
	svc := activity.New(store, nil)
	ctx := context.Background()
	object := domain.ObjectRef{Type: "tool", ID: "chrome", Label: "Dựng và kiểm tra video"}
	spec := domain.TaskSpec{
		Kind: domain.TaskSetup, Title: "Dựng và kiểm tra video",
		Object: object, Phase: "Đang tải",
		Action: domain.TaskAction{Label: "Huỷ", Op: domain.TaskOpCancel},
	}
	id, err := svc.Start(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.Start(ctx, spec)
	if err != nil || again != id {
		t.Fatalf("start lặp = %q, %v", again, err)
	}
	progress := &domain.TaskProgress{Done: 10, Total: 100, Groups: []domain.TaskGroupProgress{{Group: "videos", Done: 1, Total: 4}}}
	phase := "Đang tải 10%"
	if err := svc.Update(ctx, id, domain.TaskUpdate{Phase: &phase, Progress: progress}); err != nil {
		t.Fatal(err)
	}
	snap, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tasks) != 1 {
		t.Fatalf("tasks = %+v", snap.Tasks)
	}
	task := snap.Tasks[0]
	if task.Phase != phase || task.Progress == nil || task.Progress.Groups[0].Group != "videos" {
		t.Fatalf("task = %+v (progress %+v)", task, task.Progress)
	}
	if task.Action.Op != domain.TaskOpCancel {
		t.Fatalf("action = %+v", task.Action)
	}
	if err := svc.Finish(ctx, id); err != nil {
		t.Fatal(err)
	}
	snap, _ = svc.Snapshot(ctx)
	if len(snap.Tasks) != 0 {
		t.Fatalf("Finish không xoá: %+v", snap.Tasks)
	}
	// Update dòng đã xoá là no-op không lỗi.
	if err := svc.Update(ctx, id, domain.TaskUpdate{Phase: &phase}); err != nil {
		t.Fatal(err)
	}
}

func TestActivityStoreReconcileFailsOnlyDeadOwners(t *testing.T) {
	pool := provision(t)
	store := postgres.NewActivityStore(pool)
	svc := activity.New(store, nil)
	ctx := context.Background()
	id, err := svc.Start(ctx, domain.TaskSpec{
		Kind: domain.TaskSetup, Title: "Công cụ vẽ ảnh",
		Object: domain.ObjectRef{Type: "tool", ID: "codex"}, Phase: "Đang cài",
	})
	if err != nil {
		t.Fatal(err)
	}
	live, err := svc.Start(ctx, domain.TaskSpec{
		Kind: domain.TaskSetup, Title: "Xử lý hình và tiếng",
		Object: domain.ObjectRef{Type: "tool", ID: "ffmpeg"}, Phase: "Đang cài",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Dòng đầu giả lập process đã chết; dòng sau thuộc process này.
	if _, err := pool.Exec(ctx, `UPDATE background_tasks SET owner_pid = 999999 WHERE id = $1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	tasks, err := store.ListTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.ID == live && task.OwnerPID != os.Getpid() {
			t.Fatalf("owner live = %d, muốn %d", task.OwnerPID, os.Getpid())
		}
	}
	n, err := svc.Reconcile(ctx)
	if err != nil || n != 1 {
		t.Fatalf("reconcile = %d, %v (chỉ dòng chủ đã chết)", n, err)
	}
	if got, _ := store.FindTaskByObject(ctx, domain.TaskSetup, domain.ObjectRef{Type: "tool", ID: "ffmpeg"}); got == nil || got.State != domain.TaskRunning {
		t.Fatalf("việc của process sống bị đổi: %+v", got)
	}
	found, err := store.FindTaskByObject(ctx, domain.TaskSetup, domain.ObjectRef{Type: "tool", ID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != id || found.State != domain.TaskFailed {
		t.Fatalf("task = %+v", found)
	}
	if found.Action.Op != domain.TaskOpRetry {
		t.Fatalf("action = %+v", found.Action)
	}
	if n, _ := svc.Reconcile(ctx); n != 0 {
		t.Fatalf("reconcile lần hai = %d", n)
	}
}

// Lịch sử lỗi bền trên Postgres: 25 mục → trang 20 nhưng count7d = 25; đọc
// thông báo và Finish không làm mất lịch sử.
func TestActivityStoreErrorHistoryDurable(t *testing.T) {
	store := postgres.NewActivityStore(provision(t))
	svc := activity.New(store, nil)
	ctx := context.Background()
	for i := 0; i < 25; i++ {
		if _, err := svc.Notify(ctx, "needs_attention", "Thiếu ổ lưu trữ", "#/settings?section=storage", ""); err != nil {
			t.Fatal(err)
		}
	}
	items, err := svc.Errors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 20 {
		t.Fatalf("trang = %d, muốn 20", len(items))
	}
	count, err := svc.ErrorsCount(ctx)
	if err != nil || count != 25 {
		t.Fatalf("count = %d, %v; muốn 25", count, err)
	}
	if err := svc.MarkRead(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if count, _ := svc.ErrorsCount(ctx); count != 25 {
		t.Fatalf("đọc thông báo làm mất lịch sử: %d", count)
	}
	if err := store.InsertErrorEvent(ctx, activity.ErrorEvent{Kind: "task_failed", What: "Cài Chrome", Why: "Tải hỏng", Op: domain.TaskOpRetry}); err != nil {
		t.Fatal(err)
	}
	items, _ = svc.Errors(ctx)
	var found bool
	for _, it := range items {
		if it.Kind == "task_failed" && it.Action.Op == domain.TaskOpRetry && it.Why == "Tải hỏng" {
			found = true
		}
	}
	if !found {
		t.Fatalf("mất lỗi việc nền: %+v", items)
	}
}

func TestActivityStoreNotificationMergeAndMarkRead(t *testing.T) {
	store := postgres.NewActivityStore(provision(t))
	svc := activity.New(store, nil)
	ctx := context.Background()
	first, err := svc.Notify(ctx, "action_result", "Đã xuất video", "#/videos/1", "Mở")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Notify(ctx, "action_result", "Đã xuất video (2)", "#/videos/1", "Mở")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("không gộp: %s / %s", first, second)
	}
	other, err := svc.Notify(ctx, "needs_attention", "Không thấy ổ T7", "#/settings?section=storage", "")
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Fatal("kind khác phải tạo dòng riêng")
	}
	snap, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Unread) != 2 {
		t.Fatalf("unread = %+v", snap.Unread)
	}
	for _, n := range snap.Unread {
		if n.ID == first && n.Count != 2 {
			t.Fatalf("count = %d, muốn 2", n.Count)
		}
	}
	if err := svc.MarkRead(ctx, []string{first}); err != nil {
		t.Fatal(err)
	}
	snap, _ = svc.Snapshot(ctx)
	if len(snap.Unread) != 1 || snap.Unread[0].ID != other {
		t.Fatalf("sau mark một: %+v", snap.Unread)
	}
	if err := svc.MarkRead(ctx, nil); err != nil {
		t.Fatal(err)
	}
	snap, _ = svc.Snapshot(ctx)
	if len(snap.Unread) != 0 {
		t.Fatalf("mark tất cả còn %+v", snap.Unread)
	}
}

func TestActivityStoreNotificationMergeSerializesConcurrentWriters(t *testing.T) {
	store := postgres.NewActivityStore(provision(t))
	svc := activity.New(store, nil)
	ctx := context.Background()
	var wg sync.WaitGroup
	ids := make([]string, 8)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := svc.Notify(ctx, "action_result", "Cùng một việc", "#/x", "")
			if err == nil {
				ids[i] = id
			}
		}(i)
	}
	wg.Wait()
	snap, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Unread) != 1 {
		t.Fatalf("writer đồng thời tạo %d dòng (ids %v)", len(snap.Unread), ids)
	}
	if snap.Unread[0].Count != 8 {
		t.Fatalf("count = %d, muốn 8", snap.Unread[0].Count)
	}
}

// poolGatedDB chặn câu đọc chạy thẳng trên pool (ngoài giao dịch): đủ để
// dựng cảnh writer khác ghi xen giữa commit và lúc trả kết quả.
type poolGatedDB struct {
	postgres.DBTXer
	g *gate
}

func (d poolGatedDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if err := d.g.before(ctx, sql); err != nil {
		return failedRow{err}
	}
	return d.DBTXer.QueryRow(ctx, sql, args...)
}

// Thông báo trả về là dòng writer này đã ghi, kể cả khi writer khác chèn
// thông báo cùng kind ngay sau khi giao dịch của nó commit.
func TestActivityStoreNotificationReturnsOwnWrite(t *testing.T) {
	pool := provision(t)
	g := newGate("LatestNotification")
	mine := postgres.NewActivityStore(poolGatedDB{DBTXer: pool, g: g})
	other := postgres.NewActivityStore(pool)
	ctx := context.Background()

	type result struct {
		n   *activity.Notification
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := mine.UpsertNotification(ctx, "needs_attention", "Kênh A lỗi", "#/a", "Mở", 10*time.Second)
		done <- result{n, err}
	}()

	var got result
	select {
	case <-g.reached:
		// Đã commit và đang đọc lại trên pool: writer khác chèn dòng mới.
		if _, err := other.UpsertNotification(ctx, "needs_attention", "Kênh B lỗi", "#/b", "Mở", 0); err != nil {
			t.Fatal(err)
		}
		close(g.resume)
		got = <-done
	case got = <-done:
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.n.Text != "Kênh A lỗi" || got.n.Href != "#/a" {
		t.Fatalf("trả về thông báo của writer khác: %+v", got.n)
	}
	snap, err := activity.New(postgres.NewActivityStore(pool), nil).Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range snap.Unread {
		if n.ID == got.n.ID && n.Text != "Kênh A lỗi" {
			t.Fatalf("id %s thuộc dòng %q", n.ID, n.Text)
		}
	}
}

func TestActivityStoreFindTaskByObjectNotFound(t *testing.T) {
	store := postgres.NewActivityStore(provision(t))
	_, err := store.FindTaskByObject(context.Background(), domain.TaskSetup, domain.ObjectRef{Type: "tool", ID: "none"})
	if err != activity.ErrNotFound {
		t.Fatalf("err = %v", err)
	}
}

func TestActivityMemberCannotReadOrConsumeAdminStorageNotifications(t *testing.T) {
	pool := provision(t)
	store := postgres.NewActivityStore(pool)
	svc := activity.New(store, nil)
	ctx := context.Background()
	publicID, err := svc.Notify(ctx, "action_result", "public", "#/library", "")
	if err != nil {
		t.Fatal(err)
	}
	// More administrative entries than either page limit: filtering after
	// LIMIT would hide the legitimate member notice and corrupt count7d.
	for range 55 {
		if _, err := pool.Exec(ctx, `INSERT INTO notifications(id,kind,text,audience,object_ref)
			VALUES(gen_random_uuid(),'needs_attention','private storage','admin','{"type":"storage","id":"active"}')`); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO error_events(kind,what,audience,object_ref)
			VALUES('needs_attention','private storage','admin','{"type":"storage"}')`); err != nil {
			t.Fatal(err)
		}
	}
	member, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(member.Unread) != 1 || member.Unread[0].ID != publicID {
		t.Fatalf("member saw admin notifications or lost public page: %d entries", len(member.Unread))
	}
	items, err := svc.Errors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	count, err := svc.ErrorsCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || count != 1 {
		t.Fatalf("member errors leaked: page=%d count=%d", len(items), count)
	}
	adminCtx := activity.WithViewer(ctx, true)
	admin, err := svc.Snapshot(adminCtx)
	if err != nil {
		t.Fatal(err)
	}
	adminID := admin.Unread[0].ID
	for _, n := range admin.Unread {
		if n.ID != publicID {
			adminID = n.ID
			break
		}
	}
	if err := svc.MarkRead(ctx, []string{adminID}); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkRead(ctx, nil); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE audience='admin' AND read_at IS NULL`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 55 {
		t.Fatalf("member consumed admin notices: remaining=%d", remaining)
	}
	if err := svc.MarkRead(adminCtx, nil); err != nil {
		t.Fatal(err)
	}
	spec := activity.NotificationSpec{
		Kind: "needs_attention", Text: "scoped storage failure", Audience: activity.AudienceAll,
		Object: &domain.ObjectRef{Type: "storage", ID: "a"},
	}
	first, err := svc.NotifyScoped(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Object = &domain.ObjectRef{Type: "storage", ID: "b"}
	second, err := svc.NotifyScoped(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Object = &domain.ObjectRef{Type: "storage", ID: "a"}
	repeated, err := svc.NotifyScoped(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || repeated != first {
		t.Fatalf("merge key ignored storage object: %s %s %s", first, second, repeated)
	}
	member, err = svc.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(member.Unread) != 0 {
		t.Fatalf("storage object was not forced admin-only: %+v", member.Unread)
	}
	admin, err = svc.Snapshot(adminCtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(admin.Unread) != 2 {
		t.Fatalf("admin scoped notices: %+v", admin.Unread)
	}
	for _, n := range admin.Unread {
		if n.Object == nil || n.Audience != activity.AudienceAdmin {
			t.Fatalf("scope metadata missing: %+v", n)
		}
		if n.ID == first && n.Count != 2 {
			t.Fatalf("repeat count=%d", n.Count)
		}
	}
}
