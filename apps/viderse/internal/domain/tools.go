package domain

// SetupStepID là id của một bước Chuẩn bị máy (W1): năm bước cài công cụ tự
// chạy theo thứ tự, bước cuối là đăng nhập ChatGPT cần người dùng.
type SetupStepID = string

const (
	SetupStepYtDlp         SetupStepID = "yt-dlp"
	SetupStepFfmpeg        SetupStepID = "ffmpeg"
	SetupStepTranscription SetupStepID = "transcription"
	SetupStepChrome        SetupStepID = "chrome"
	SetupStepCodex         SetupStepID = "codex"
	SetupStepCodexLogin    SetupStepID = "codex-login"
)

// SetupStepIDs là thứ tự cố định của sáu bước trên màn Chuẩn bị máy.
var SetupStepIDs = []SetupStepID{
	SetupStepYtDlp, SetupStepFfmpeg, SetupStepTranscription,
	SetupStepChrome, SetupStepCodex, SetupStepCodexLogin,
}

// SetupNeed là đánh giá một bước cần gì: ready (đã sẵn sàng), install (app tự
// sửa bằng cách cài), login (cần người dùng đăng nhập), fix (hỏng cần xử lý).
type SetupNeed = string

const (
	SetupNeedReady   SetupNeed = "ready"
	SetupNeedInstall SetupNeed = "install"
	SetupNeedLogin   SetupNeed = "login"
	SetupNeedFix     SetupNeed = "fix"
)

// SetupCheck là kết quả đánh giá một bước; Error là câu tiếng Việt giải thích
// khi need là fix (đi qua màn setup và thẻ công cụ). Provided = công cụ có sẵn
// trên máy (Chrome hệ thống) nên hàng hiện “Có sẵn” thay vì dung lượng tải.
type SetupCheck struct {
	Need     SetupNeed
	Error    string
	Provided bool
	// SizeBytes là dung lượng của bước: số đã đo sau cài (ready), xấp xỉ
	// manifest (chưa cài), 0 (provided/không đo được).
	SizeBytes int64
}

// InstallPhase tái dùng tên pha của bộ cài (checking|downloading|installing|
// self-check|activating|done|failed|cancelled) dưới dạng chuỗi, để gói
// orchestration không phụ thuộc gói tools.
type InstallProgress struct {
	Phase      string
	Detail     string
	BytesDone  int64
	BytesTotal *int64 // nil = chưa biết tổng
	StartedAt  int64  // unix seconds; 0 = không rõ
}

// Cài thất bại của các bước công cụ đều qua TaskSpec dưới đây trên Tasks.
