package host

import (
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

// TestHostReopen 守护 /reopen 的用户级重开出口：完本是重决策，重开只能由用户显式
// 发起——未完结拒绝、运行中拒绝；重开成功把 phase 回退 writing，附带的续写方向登记为
// 待处理干预（PendingSteer），恢复时先经 Arbiter 裁定注入再续跑。
func TestHostReopen(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	h := &Host{store: st, events: make(chan Event, 8)}

	if err := st.Progress.Init(2); err != nil {
		t.Fatal(err)
	}
	if err := h.Reopen(""); err == nil {
		t.Fatal("Truyện chưa hoàn thành phải bị từ chối mở lại")
	}

	_ = st.Progress.UpdatePhase(domain.PhaseWriting)
	if err := st.Progress.MarkComplete(); err != nil {
		t.Fatal(err)
	}
	if err := h.Reopen("Mở tập mới với đại hạn tám mươi năm"); err != nil {
		t.Fatalf("Mở lại truyện đã hoàn thành phải thành công: %v", err)
	}
	p, _ := st.Progress.Load()
	if p.Phase != domain.PhaseWriting {
		t.Fatalf("Sau khi mở lại phase phải là writing, nhận được %s", p.Phase)
	}
	if len(p.PendingRewrites) != 0 || p.ReopenedFromComplete {
		t.Fatalf("Mở lại để viết tiếp không được mang ngữ nghĩa viết lại: %+v", p)
	}
	// 重开计数必须落盘：再完结的 progress digest 才会与上次不同——checkpoint 同 digest
	// 幂等去重，字节相同的再完结无新 checkpoint，StopGuard 会把成功完本误判为空转终止。
	if p.ReopenCount != 1 {
		t.Fatalf("Số lần mở lại phải là 1, nhận được %d", p.ReopenCount)
	}
	meta, _ := st.RunMeta.Load()
	if meta == nil || !strings.Contains(meta.PendingSteer, "đại hạn tám mươi năm") {
		t.Fatalf("Hướng viết tiếp phải được đăng ký là can thiệp đang chờ xử lý, nhận được %+v", meta)
	}

	running := &Host{store: st, lifecycle: lifecycleRunning, events: make(chan Event, 1)}
	if err := running.Reopen(""); err == nil {
		t.Fatal("Khi engine đang chạy phải từ chối mở lại")
	}
}
