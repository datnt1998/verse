package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/datnt1998/noverse/internal/host"
)

// TestStatusBarShowsUpdateHint 锁死 P0-2 的 TUI 侧契约：后台检查发现有新版本时，
// 状态栏常驻一条越南语短提示，引导用户跑 ainovel-cli update。
func TestStatusBarShowsUpdateHint(t *testing.T) {
	out := ansi.Strip(renderStatusBar(host.UISnapshot{
		ModelName: "test-model",
	}, "/tmp/output", "Noverse có bản mới v1.2.3 · chạy ainovel-cli update", 120))
	if !strings.Contains(out, "Noverse có bản mới v1.2.3") || !strings.Contains(out, "ainovel-cli update") {
		t.Fatalf("状态栏缺少新版本提示：%q", out)
	}
	// 提示单独占位，不覆盖现有的模型/目录信息。
	if !strings.Contains(out, "test-model") || !strings.Contains(out, "./output") {
		t.Fatalf("更新提示不应挤掉状态栏原有内容：%q", out)
	}
}

func TestLatestReleaseMsgSetsStatusBarHint(t *testing.T) {
	m := NewModel(nil, "v1.2.0")
	next, _, handled := m.handleRuntimeMsg(latestReleaseMsg{Latest: "v1.3.0", HasUpdate: true})
	if !handled {
		t.Fatal("latestReleaseMsg must be handled")
	}
	m = next.(Model)
	if !strings.Contains(m.updateHint, "v1.3.0") {
		t.Fatalf("updateHint = %q", m.updateHint)
	}
	// 23-brand §5: câu hiển thị dùng thương hiệu Noverse; tên lệnh giữ nguyên.
	if !strings.Contains(m.updateHint, "Noverse có bản mới") || !strings.Contains(m.updateHint, "ainovel-cli update") {
		t.Fatalf("updateHint wording = %q", m.updateHint)
	}
}

// TestCheckLatestReleaseSkipsDev dev 构建不该触发后台检查：命令立即返回 nil。
func TestCheckLatestReleaseSkipsDev(t *testing.T) {
	cmd := checkLatestRelease("dev")
	if msg := cmd(); msg != nil {
		t.Fatalf("dev build should skip update check, got %v", msg)
	}
}
