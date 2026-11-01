package voice

import (
	"fmt"
	"time"
)

// Bộ trạng thái "chuẩn bị bộ đọc" dùng chung worker ↔ server ↔ API ↔ UI.
// Giá trị là chuỗi ổn định vì đi thẳng ra JSON; thêm giá trị mới thì thêm,
// không đổi giá trị cũ.
const (
	// ProvisionStateMissing: chưa cài, chưa bắt đầu.
	ProvisionStateMissing = "missing"
	// ProvisionStateProvisioning: đang chuẩn bị; Step + Percent (0–100) đi kèm.
	ProvisionStateProvisioning = "provisioning"
	// ProvisionStateStarting: đang bật máy chủ, chờ /health — không đo %.
	ProvisionStateStarting = "starting"
	// ProvisionStateReady: máy chủ trả /health ok.
	ProvisionStateReady = "ready"
	// ProvisionStateError: lượt chuẩn bị thất bại; Error.Code nói lý do.
	ProvisionStateError = "error"
	// ProvisionStateOffline: CHỈ server suy ra từ lần kết nối cuối của worker.
	ProvisionStateOffline = "offline"
)

// Các bước của lượt chuẩn bị, theo thứ tự cố định.
const (
	ProvisionStepTools  = "tools"  // uv ghim
	ProvisionStepPython = "python" // CPython 3.12 do uv tải
	ProvisionStepSource = "source" // mã nguồn VieNeu đúng tag ghim
	ProvisionStepDeps   = "deps"   // uv sync --frozen
	ProvisionStepModel  = "model"  // tải mô hình giọng đọc
)

// Mã lỗi ổn định của lượt chuẩn bị. UI dịch sang câu tiếng Việt, không hiện mã.
const (
	ProvisionErrNoDisk         = "no_disk"
	ProvisionErrDownloadFailed = "download_failed"
	ProvisionErrHashMismatch   = "hash_mismatch"
	ProvisionErrUVFailed       = "uv_failed"
	ProvisionErrDepsFailed     = "deps_failed"
	ProvisionErrModelFailed    = "model_failed"
	ProvisionErrStartFailed    = "start_failed"
	ProvisionErrTokenRejected  = "token_rejected"
)

// ProvisionPercentUnknown là phần trăm khi bước đang chạy không đo được: UI
// hiện spinner + chữ thay vì bịa con số.
const ProvisionPercentUnknown = -1

// ProvisionError là lý do một lượt chuẩn bị thất bại.
type ProvisionError struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	LogTail string `json:"logTail,omitempty"`
}

func (e *ProvisionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Code
}

// ProvisionState là ảnh chụp trạng thái chuẩn bị bộ đọc tại một thời điểm.
type ProvisionState struct {
	State   string          `json:"state"`
	Step    string          `json:"step,omitempty"`
	Percent int             `json:"percent"`
	Message string          `json:"message,omitempty"`
	Error   *ProvisionError `json:"error,omitempty"`

	Engine  string `json:"engine"`
	Model   string `json:"model"`
	Version string `json:"version"`

	RTF       float64 `json:"rtf,omitempty"`
	DiskBytes int64   `json:"diskBytes,omitempty"`
	Attempts  int     `json:"attempts,omitempty"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// ProvisionReporter nhận từng mốc trạng thái của lượt chuẩn bị. Engine gọi
// tuần tự trên goroutine của lượt chạy; nil thì bỏ qua.
type ProvisionReporter func(ProvisionState)

// Provisioning cho biết lượt chuẩn bị đang chạy (dùng để giữ trạng thái cũ
// khi một mốc chỉ đổi bước).
func (s ProvisionState) Provisioning() bool { return s.State == ProvisionStateProvisioning }
