package host

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"strings"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/noverse/internal/bootstrap"
	"github.com/datnt1998/noverse/internal/domain"
	storepkg "github.com/datnt1998/noverse/internal/store"
)

// Gói P · 27/10 — ask_user tắt: can thiệp trả `ask` bị máy chủ từ chối (không phải
// hỏi lại model), tự chọn phương án bám dàn ý rồi phán định tiếp qua cùng một kênh;
// nhật ký có dòng “Trọng tài tự chọn: … (vì bạn đã tắt Hỏi khi phân vân)” và
// PendingAsk (“Đang chờ bạn”) không bao giờ được đặt.
func TestInterventionAutoChoosesAskWhenDisabled(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.RunMeta.Init("default", "test", "test"); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	model := &scriptedChatModel{fn: func(msgs []llmtypes.Message) llmtypes.Message {
		if calls.Add(1) == 1 {
			return testTextMsg(`{
				"answer":"Chương 5 dài hơn dàn ý hai cảnh.",
				"rules":null,"hold":null,"reopen":null,
				"ask":{"question":"Giữ hai cảnh rồi dời dàn ý, hay cắt cho khớp?",
					"options":[
						{"label":"Cắt cho khớp dàn ý","reply":"Cắt hai cảnh thừa","is_default":true},
						{"label":"Giữ nguyên, dời dàn ý","reply":"Giữ cả hai cảnh","is_default":false}],
					"chapter":5},
				"dispatch":null,"reason":"cần người dùng phán quyết"
			}`)
		}
		return testTextMsg(`{
			"answer":"Đã ghi nhận: cắt hai cảnh thừa, giữ nguyên đại cương.",
			"rules":null,"hold":null,"reopen":null,"ask":null,"dispatch":null,
			"reason":"triển khai lựa chọn bám dàn ý"
		}`)
	}}

	h := &Host{
		cfg:                bootstrap.Config{}, // AskUser nil = tắt (mặc định)
		store:              st,
		events:             make(chan Event, 32),
		streamCh:           make(chan string, 4),
		engine:             &engine{},
		models:             &bootstrap.ModelSet{Default: bootstrap.NewSwappableModel("test", "m", model, nil)},
		interventionPrompt: "sys",
	}
	h.observer = newObserver(st, h.emitEvent, h.emitDelta, h.emitClear, h.contextWindowForAgent)
	if err := h.doIntervention("chương 5 dài hơn dàn ý", false); err != nil {
		t.Fatalf("can thiệp phải được xử lý, không phải lỗi: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("máy chủ phải phán định lại trên phương án đã chọn (2 lượt), nhận %d", got)
	}
	meta, err := st.RunMeta.Load()
	if err != nil || meta == nil {
		t.Fatalf("RunMeta: %v %v", err, meta)
	}
	if meta.PendingAsk != nil {
		t.Fatal("PendingAsk không được đặt khi ask_user tắt — thẻ Đang chờ bạn không bao giờ xuất hiện")
	}
	var summaries []string
	for {
		select {
		case ev := <-h.events:
			summaries = append(summaries, ev.Summary)
		default:
			for _, want := range []string{
				"Trọng tài tự chọn: Cắt hai cảnh thừa (vì bạn đã tắt Hỏi khi phân vân)",
				"Đã ghi nhận: cắt hai cảnh thừa, giữ nguyên đại cương.",
			} {
				found := false
				for _, s := range summaries {
					if strings.Contains(s, want) {
						found = true
					}
				}
				if !found {
					t.Fatalf("thiếu dòng nhật ký %q trong %v", want, summaries)
				}
			}
			return
		}
	}
}

// ask_user tắt nhưng còn câu hỏi treo từ phiên trước (cờ vừa bị tắt giữa chừng)
// → NewHost/snapshot không chiếu PendingAsk: thẻ “Đang chờ bạn” không xuất hiện.
func TestSnapshotHidesPendingAskWhenDisabled(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.RunMeta.Init("default", "test", "test"); err != nil {
		t.Fatal(err)
	}
	if err := st.RunMeta.SetPendingAsk(domain.PendingAsk{
		Question: "Giữ hay cắt?", Chapter: 5,
		Options: []domain.PendingAskOption{{Label: "Cắt", IsDefault: true}, {Label: "Giữ"}},
	}); err != nil {
		t.Fatal(err)
	}
	h := &Host{
		cfg:    bootstrap.Config{},
		store:  st,
		events: make(chan Event, 8),
	}
	h.observer = newObserver(st, h.emitEvent, h.emitDelta, h.emitClear, h.contextWindowForAgent)
	if snap := h.Snapshot(); snap.PendingAsk != nil {
		t.Fatalf("snapshot không được chiếu PendingAsk khi ask_user tắt: %+v", snap.PendingAsk)
	}
	on := true
	hOn := &Host{
		cfg:    bootstrap.Config{AskUser: &on},
		store:  st,
		events: make(chan Event, 8),
	}
	hOn.observer = newObserver(st, hOn.emitEvent, hOn.emitDelta, hOn.emitClear, hOn.contextWindowForAgent)
	if snap := hOn.Snapshot(); snap.PendingAsk == nil {
		t.Fatal("snapshot phải chiếu PendingAsk khi ask_user bật")
	}
}
