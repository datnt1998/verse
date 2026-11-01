package web

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/platform/toolchain"
)

// statusLeakMarkers là những thứ chỉ được xuất hiện ở Nâng cao: đường dẫn nội
// bộ, URL tải và traceback của uv.
var statusLeakMarkers = []string{
	"/opt/ainovel",
	"https://github.com/astral-sh/python-build-standalone",
	"Traceback (most recent call last)",
}

// realProvisionFailure chạy lượt chuẩn bị THẬT của engine (không phải một
// thông điệp dựng sẵn): uv giả nằm đúng chỗ app quản nên không chạm mạng, còn
// bước Python thất bại với chẩn đoán đầy đủ như uv thật. Trả mốc cuối cùng mà
// reporter nhận — đúng thứ worker công bố lên máy chủ.
func realProvisionFailure(t *testing.T) voice.ProvisionState {
	t.Helper()
	last, _ := realProvisionFailureHome(t, "", nil)
	return last
}

// realProvisionFailureHome là bản đầy đủ: trả cả home để bài test khẳng định
// đường dẫn nội bộ không rò ra thành viên. script rỗng = uv giả in chẩn đoán
// rồi thoát 1; prepare (nếu có) dựng thêm điều kiện hỏng trước khi chạy.
func realProvisionFailureHome(t *testing.T, script string, prepare func(home string)) (voice.ProvisionState, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("script uv giả dùng sh")
	}
	home := t.TempDir()
	root := filepath.Join(home, "bin", "uv")
	version := "v-" + toolchain.UvPin().Version
	dir := filepath.Join(root, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if script == "" {
		script = `#!/bin/sh
case "$1" in
  python)
    cat >&2 <<'OUT'
error: failed to download CPython 3.12 from https://github.com/astral-sh/python-build-standalone/releases
  at /opt/ainovel/python/cpython-3.12.7
Traceback (most recent call last):
OUT
    exit 1 ;;
esac
exit 0
`
	}
	if err := os.WriteFile(filepath.Join(dir, toolchain.UvBinaryName()), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "current"), []byte(version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if prepare != nil {
		prepare(home)
	}

	eng, err := voice.NewEngine(voice.Config{Home: home, Model: voice.DefaultModel, Device: "cpu"})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	prov, ok := eng.(interface {
		Provision(context.Context, voice.ProvisionReporter) error
	})
	if !ok {
		t.Fatal("engine không có Provision")
	}
	var last voice.ProvisionState
	if err := prov.Provision(context.Background(), func(st voice.ProvisionState) { last = st }); err == nil {
		t.Fatal("Provision thành công dù uv giả thoát 1")
	}
	if last.State != voice.ProvisionStateError || last.Error == nil {
		t.Fatalf("mốc cuối = %+v, muốn error", last)
	}
	return last, home
}

// Lỗi thật của adapter (uv in đường dẫn/URL/traceback) chỉ được ra khỏi máy
// chủ qua logTail của Nâng cao; câu trạng thái là câu 8.2 của mã lỗi.
func TestProvisionFailureStatusKeepsDiagnosticsInAdminTail(t *testing.T) {
	last := realProvisionFailure(t)

	body := statusErrorOf(last)
	if body.Code != voice.ProvisionErrUVFailed {
		t.Fatalf("code = %q, muốn %q", body.Code, voice.ProvisionErrUVFailed)
	}
	if body.Message != provisionReason(voice.ProvisionErrUVFailed) {
		t.Fatalf("message = %q, muốn câu 8.2", body.Message)
	}
	for _, marker := range statusLeakMarkers {
		if strings.Contains(body.Message, marker) {
			t.Fatalf("câu trạng thái rò %q: %q", marker, body.Message)
		}
	}

	tail := strings.Join(workerLogTail(last.Error, 6), "\n")
	if tail == "" {
		t.Fatal("admin không còn chẩn đoán nào")
	}
	if !strings.Contains(tail, "cài Python 3.12 thất bại") {
		t.Fatalf("logTail mất lý do kỹ thuật: %q", tail)
	}
	for _, marker := range statusLeakMarkers {
		if !strings.Contains(tail, marker) {
			t.Fatalf("logTail thiếu %q: %q", marker, tail)
		}
	}

	// Toàn bộ payload thành viên (không có logTail) không được mang chẩn đoán.
	payload, err := json.Marshal(ttsStatusResponse{State: voice.ProvisionStateError, Error: body})
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range statusLeakMarkers {
		if strings.Contains(string(payload), marker) {
			t.Fatalf("payload thành viên rò %q: %s", marker, payload)
		}
	}
}

// Cùng một lỗi thật đi qua HTTP: thành viên chỉ thấy câu 8.2, quản trị thấy
// chẩn đoán trong logTail.
func TestHostedStatusMemberOnlyGetsPublicReasonForRealFailure(t *testing.T) {
	last := realProvisionFailure(t)
	tg, jobs, _ := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	writer := tg.login("writer", "writer-pass")
	w := hostedWorker(t, jobs, "mac-provfail")
	state, err := json.Marshal(last)
	if err != nil {
		t.Fatal(err)
	}
	setWorkerState(t, jobs, w.ID, string(state))

	memberRR := tg.do("GET", "/api/tts/status", writer, "")
	if memberRR.Code != 200 {
		t.Fatalf("member status = %d %s", memberRR.Code, memberRR.Body.String())
	}
	for _, marker := range statusLeakMarkers {
		if strings.Contains(memberRR.Body.String(), marker) {
			t.Fatalf("member response rò %q: %s", marker, memberRR.Body.String())
		}
	}
	memberView := decodeJSONBody(t, memberRR)
	errBody, _ := memberView["error"].(map[string]any)
	if errBody == nil || errBody["code"] != voice.ProvisionErrUVFailed ||
		errBody["message"] != provisionReason(voice.ProvisionErrUVFailed) {
		t.Fatalf("member error = %v", memberView["error"])
	}
	if _, ok := memberView["logTail"]; ok {
		t.Fatalf("member saw the log tail: %v", memberView["logTail"])
	}

	adminRR := tg.do("GET", "/api/tts/status", admin, "")
	if adminRR.Code != 200 {
		t.Fatalf("admin status = %d %s", adminRR.Code, adminRR.Body.String())
	}
	adminView := decodeJSONBody(t, adminRR)
	adminErr, _ := adminView["error"].(map[string]any)
	if adminErr == nil || adminErr["message"] != provisionReason(voice.ProvisionErrUVFailed) {
		t.Fatalf("admin error = %v", adminView["error"])
	}
	tail, _ := adminView["logTail"].([]any)
	joined := make([]string, 0, len(tail))
	for _, line := range tail {
		if s, ok := line.(string); ok {
			joined = append(joined, s)
		}
	}
	adminTail := strings.Join(joined, "\n")
	for _, marker := range statusLeakMarkers {
		if !strings.Contains(adminTail, marker) {
			t.Fatalf("admin logTail thiếu %q: %q", marker, adminTail)
		}
	}
}

// Lỗi thật có đường dẫn trong chính câu lỗi (thư mục Python không tạo được)
// cũng không được rò ra thành viên: câu 8.2 thay chỗ đó, còn Nâng cao vẫn thấy
// nguyên nhân trong logTail.
func TestProvisionFailureStatusKeepsPathInAdminTailOnly(t *testing.T) {
	last, home := realProvisionFailureHome(t, "#!/bin/sh\nexit 0\n", func(home string) {
		// <home>/python là tệp: bước chuẩn bị Python hỏng với đường dẫn thật
		// trong thông điệp lỗi.
		if err := os.WriteFile(filepath.Join(home, "python"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	if last.Error == nil || !strings.Contains(last.Error.Message, home) {
		t.Fatalf("lỗi thật phải mang đường dẫn %q: %+v", home, last.Error)
	}

	body := statusErrorOf(last)
	if body.Code != voice.ProvisionErrUVFailed {
		t.Fatalf("code = %q, muốn %q", body.Code, voice.ProvisionErrUVFailed)
	}
	if strings.Contains(body.Message, home) || body.Message != provisionReason(voice.ProvisionErrUVFailed) {
		t.Fatalf("câu thành viên rò đường dẫn: %q", body.Message)
	}
	payload, err := json.Marshal(ttsStatusResponse{State: voice.ProvisionStateError, Error: body})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), home) {
		t.Fatalf("payload thành viên rò đường dẫn: %s", payload)
	}
	tail := strings.Join(workerLogTail(last.Error, 6), "\n")
	if !strings.Contains(tail, home) {
		t.Fatalf("logTail mất nguyên nhân kỹ thuật: %q", tail)
	}
}

// Mọi mã lỗi chuẩn bị đều có câu 8.2 riêng: mã lạ không được rơi về câu
// "không bật lên được" của start_failed khi lý do thật khác (ví dụ interrupted
// = dừng giữa chừng, chưa rõ kết quả).
func TestProvisionPublicReasonCoversEveryCode(t *testing.T) {
	codes := []string{
		voice.ProvisionErrNoDisk,
		voice.ProvisionErrDownloadFailed,
		voice.ProvisionErrHashMismatch,
		voice.ProvisionErrUVFailed,
		voice.ProvisionErrDepsFailed,
		voice.ProvisionErrModelFailed,
		voice.ProvisionErrStartFailed,
		voice.ProvisionErrTokenRejected,
		voice.ProvisionErrInterrupted,
	}
	for _, code := range codes {
		reason := provisionReason(code)
		if strings.TrimSpace(reason) == "" {
			t.Fatalf("mã %q không có câu công khai", code)
		}
		if code == voice.ProvisionErrInterrupted && reason == provisionReason(voice.ProvisionErrStartFailed) {
			t.Fatal("interrupted rơi về câu start_failed")
		}
	}

	last := voice.ProvisionState{
		State:   voice.ProvisionStateError,
		Percent: voice.ProvisionPercentUnknown,
		Error: &voice.ProvisionError{
			Code:    voice.ProvisionErrInterrupted,
			Message: "máy đọc tắt giữa lượt uv sync tại /opt/ainovel/voice/vieneu",
			LogTail: "Traceback (most recent call last):",
		},
	}
	body := statusErrorOf(last)
	if body.Code != voice.ProvisionErrInterrupted ||
		body.Message != provisionReason(voice.ProvisionErrInterrupted) {
		t.Fatalf("statusErrorOf = %+v, muốn interrupted + câu riêng", body)
	}
	if strings.Contains(body.Message, "/opt/ainovel") || strings.Contains(body.Message, "Traceback") {
		t.Fatalf("câu interrupted rò chẩn đoán: %q", body.Message)
	}
	tail := strings.Join(workerLogTail(last.Error, 6), "\n")
	if !strings.Contains(tail, "/opt/ainovel") || !strings.Contains(tail, "Traceback") {
		t.Fatalf("logTail mất chẩn đoán interrupted: %q", tail)
	}
}
