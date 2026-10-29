package host

import (
	"context"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host/imp"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// newFlagTestHost 造一个最小 Host，只够驱动 cocreating 标记状态机与并发守卫。
// emitEvent 使用非阻塞通道，缓冲 events 即可，无需 observer。
// PauseForCoCreate 的运行态分支会调 Engine Abort（复用已验证的 Esc 暂停路径），
// 不在此单测；这里只覆盖非运行态与标记/守卫逻辑。
func newFlagTestHost(lc lifecycle, cocreating bool) *Host {
	return &Host{
		lifecycle:  lc,
		cocreating: cocreating,
		engine:     &engine{}, // acquireExclusive 查 engine.isRunning()（停止窗口门禁）
		events:     make(chan Event, 16),
	}
}

func TestPauseForCoCreate_NonRunningSetsFlag(t *testing.T) {
	h := newFlagTestHost(lifecycleIdle, false)
	if !h.PauseForCoCreate() {
		t.Fatal("Trạng thái idle phải cho phép vào đồng sáng tác theo giai đoạn")
	}
	if !h.cocreating {
		t.Error("Sau khi vào, cocreating phải là true")
	}
	if h.lifecycle != lifecycleIdle {
		t.Errorf("Vào khi không chạy không được đổi lifecycle, nhận %s", h.lifecycle)
	}
}

func TestPauseForCoCreate_RejectsCompleted(t *testing.T) {
	h := newFlagTestHost(lifecycleCompleted, false)
	if h.PauseForCoCreate() {
		t.Error("Sau khi toàn truyện hoàn thành không được cho vào đồng sáng tác theo giai đoạn")
	}
	if h.cocreating {
		t.Error("Sau khi từ chối không được bật cocreating")
	}
}

func TestPauseForCoCreate_RejectsReentrant(t *testing.T) {
	h := newFlagTestHost(lifecyclePaused, true)
	if h.PauseForCoCreate() {
		t.Error("Đang đồng sáng tác thì phải từ chối vào lại")
	}
}

func TestCancelCoCreate_ClearsFlag(t *testing.T) {
	h := newFlagTestHost(lifecyclePaused, true)
	h.CancelCoCreate()
	if h.cocreating {
		t.Error("Sau khi huỷ, cocreating phải được xoá")
	}
	if h.lifecycle != lifecyclePaused {
		t.Errorf("Huỷ không được đổi lifecycle, nhận %s", h.lifecycle)
	}
}

func TestCancelCoCreate_NoopWhenNotCocreating(t *testing.T) {
	h := newFlagTestHost(lifecycleRunning, false)
	h.CancelCoCreate() // 不应 panic，不应改状态
	if h.cocreating || h.lifecycle != lifecycleRunning {
		t.Error("Khi không đồng sáng tác, CancelCoCreate phải là no-op")
	}
}

func TestResumeFromCoCreate_RejectsEmptyDraft(t *testing.T) {
	h := newFlagTestHost(lifecyclePaused, true)
	if err := h.ResumeFromCoCreate("   "); err == nil {
		t.Fatal("draft rỗng phải báo lỗi")
	}
	if !h.cocreating {
		t.Error("draft rỗng trả về trước khi xoá cờ, cocreating phải giữ true")
	}
}

func TestResumeFromCoCreate_RejectsWhenNotCocreating(t *testing.T) {
	h := newFlagTestHost(lifecyclePaused, false)
	err := h.ResumeFromCoCreate("## Hướng tiếp theo\n- Vào tập hai")
	if err == nil || !strings.Contains(err.Error(), "Không ở chế độ đồng sáng tác") {
		t.Fatalf("Khi không đồng sáng tác phải báo «Không ở chế độ đồng sáng tác», nhận %v", err)
	}
}

func TestAcquireExclusive(t *testing.T) {
	cases := []struct {
		name       string
		lc         lifecycle
		cocreating bool
		exclusive  string
		wantErr    string // 空=期望放行
	}{
		{"running", lifecycleRunning, false, "", "đang chạy"},
		{"cocreating", lifecyclePaused, true, "", "đồng sáng tác theo giai đoạn"},
		{"busy", lifecycleIdle, false, "nhập", "Đang"},
		{"idle free", lifecycleIdle, false, "", ""},
		{"paused free", lifecyclePaused, false, "", ""},
	}
	// Abort 停止窗口：lifecycle 已置 paused 但引擎 goroutine 尚未退净，仍须拒绝——
	// 否则导入会与引擎收尾并发写同一 store。
	drain := newFlagTestHost(lifecyclePaused, false)
	drain.engine.running = true
	if err := drain.acquireExclusive("nhập"); err == nil {
		t.Fatal("Trong lúc engine đang dừng phải từ chối tác vụ độc quyền")
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newFlagTestHost(c.lc, c.cocreating)
			h.exclusive = c.exclusive
			err := h.acquireExclusive("nhập")
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("Phải cho phép, nhận %v", err)
				}
				if h.exclusive != "nhập" {
					t.Fatalf("Sau khi cho phép phải ghi nhận chiếm giữ, nhận %q", h.exclusive)
				}
				h.releaseExclusive()
				if h.exclusive != "" {
					t.Fatalf("Sau khi nhả, chiếm giữ phải được xoá, nhận %q", h.exclusive)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("Phải chứa %q, nhận %v", c.wantErr, err)
			}
			if !strings.Contains(err.Error(), "nhập") {
				t.Errorf("Thông điệp lỗi phải kèm action %q, nhận %v", "nhập", err)
			}
		})
	}
}

// TestExclusiveBlocksCreationEntries 守护 #2：后台独占作业（导入/仿写）进行中时，
// 不仅第二个后台作业被堵，创作写入口（Continue/Resume）与新后台作业也必须被堵，
// 否则 Continue 会在引擎被门禁拦下前就让 Arbiter 改状态、Resume/next 期间引擎可抢跑。
func TestExclusiveBlocksCreationEntries(t *testing.T) {
	h := newFlagTestHost(lifecycleIdle, false)
	h.exclusive = "nhập"
	if _, err := h.ImportFrom(context.Background(), imp.Options{}); err == nil {
		t.Error("Trong lúc tác vụ độc quyền, ImportFrom phải bị từ chối")
	}
	if err := h.Continue("Viết tiếp", ""); err == nil {
		t.Error("Trong lúc tác vụ độc quyền, Continue phải bị từ chối (phải chặn trước khi Arbiter phán định)")
	}
	if _, err := h.Resume(); err == nil {
		t.Error("Trong lúc tác vụ độc quyền, Resume phải bị từ chối")
	}
}

// TestStageCoCreate_OccupancyBlocksConcurrentEntries 验证共创窗口内独占性入口全部被堵：
// import/start/resume/continue 在 cocreating 期间都应被拒，补上 paused 期只查 ==running 的缺口。
func TestStageCoCreate_OccupancyBlocksConcurrentEntries(t *testing.T) {
	h := newFlagTestHost(lifecycleIdle, false)
	if !h.PauseForCoCreate() {
		t.Fatal("Vào đồng sáng tác theo giai đoạn thất bại")
	}

	if _, err := h.ImportFrom(context.Background(), imp.Options{}); err == nil {
		t.Error("Trong cửa sổ đồng sáng tác, ImportFrom phải bị từ chối")
	}
	if err := h.StartPrepared("Viết một truyện mới"); err == nil {
		t.Error("Trong cửa sổ đồng sáng tác, StartPrepared phải bị từ chối")
	}
	if _, err := h.Resume(); err == nil {
		t.Error("Trong cửa sổ đồng sáng tác, Resume phải bị từ chối")
	}
	if err := h.Continue("Viết tiếp", ""); err == nil {
		t.Error("Trong cửa sổ đồng sáng tác, Continue phải bị từ chối")
	}

	// 退出共创后占用解除（这里走 Cancel；Resume 干预路径归集成验证）
	h.CancelCoCreate()
	if h.cocreating {
		t.Fatal("Sau khi thoát, cờ chiếm giữ phải được gỡ")
	}
}

func TestBuildStoryStateSummary_NilStore(t *testing.T) {
	if got := buildStoryStateSummary(nil); got != "" {
		t.Errorf("nil store phải trả chuỗi rỗng, nhận %q", got)
	}
}

func TestBuildStoryStateSummary_Populated(t *testing.T) {
	dir := t.TempDir()
	st := store.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(100); err != nil {
		t.Fatal(err)
	}
	if err := st.Book.Save(domain.BookMetadata{Title: "Ảnh Chi Thi", Synopsis: "Thiếu niên truy tìm chiếc bóng đã mất."}); err != nil {
		t.Fatal(err)
	}
	p, _ := st.Progress.Load()
	p.CompletedChapters = []int{1, 2, 3}
	p.TotalWordCount = 12000
	if err := st.Progress.Save(p); err != nil {
		t.Fatal(err)
	}
	if err := st.Outline.SaveCompass(domain.StoryCompass{
		EndingDirection: "Nhân vật chính lên tới đỉnh cao tuyệt đỉnh",
		OpenThreads:     []string{"Huyết thù sư môn chưa báo"},
		EstimatedScale:  "Dự kiến 4-6 tập",
	}); err != nil {
		t.Fatal(err)
	}

	got := buildStoryStateSummary(st)
	for _, want := range []string{"Ảnh Chi Thi", "đã hoàn thành 3 chương", "chương tiếp theo là chương 4", "Nhân vật chính lên tới đỉnh cao tuyệt đỉnh", "Huyết thù sư môn chưa báo", "Dự kiến 4-6 tập"} {
		if !strings.Contains(got, want) {
			t.Errorf("Tóm tắt phải chứa %q, thực tế:\n%s", want, got)
		}
	}
}

func TestBuildStoryStateSummaryUsesDynamicPlanningWording(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(66); err != nil {
		t.Fatal(err)
	}
	if err := st.Outline.SaveLayeredOutline([]domain.VolumeOutline{{
		Index: 1, Title: "Tập một", Arcs: []domain.ArcOutline{
			{Index: 1, Chapters: []domain.OutlineEntry{{Title: "Một"}, {Title: "Hai"}}},
			{Index: 2, EstimatedChapters: 64},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	p, err := st.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	p.Layered = true
	if err := st.Progress.Save(p); err != nil {
		t.Fatal(err)
	}

	got := buildStoryStateSummary(st)
	if !strings.Contains(got, "hiện đã chi tiết hoá 2 chương (phần sau lập kế hoạch động theo cung)") {
		t.Fatalf("Cách diễn đạt tóm tắt lập kế hoạch động sai:\n%s", got)
	}
	if strings.Contains(got, "66") || strings.Contains(got, "kế hoạch 2 chương") {
		t.Fatalf("Tóm tắt lập kế hoạch động không được ngụ ý tổng số chương cố định:\n%s", got)
	}
}
