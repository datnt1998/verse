package domain

import (
	"encoding/json"
	"fmt"
)

// Mức tự chủ của kênh (07 §20, 01 §9.4): ba mức sẵn + Tuỳ chỉnh. Máy chỉ tự
// làm trong phạm vi mức này; không có mức nào cho phép tự đăng, tự xoá hay
// đổi template.

// Preset là tên mức tự chủ.
const (
	PresetCareful  = "careful"
	PresetBalanced = "balanced"
	PresetAuto     = "auto"
	PresetCustom   = "custom"
)

// PickMode: cách lấy video mới của đối thủ.
const (
	PickSuggest = "suggest" // chỉ đề xuất video ≥ ×3, không tự lấy
	PickScore2  = "score2"  // tự lấy video ≥ ×2 mức thường
	PickAll     = "all"     // tự lấy mọi video mới
)

// PickScore2Min là mức vượt trội tối thiểu của chế độ Tự lấy "video ≥ ×2".
const PickScore2Min = 2.0

// ApproveMode: cách duyệt ảnh.
const (
	ApproveAsk   = "ask"   // luôn hỏi
	ApproveScore = "score" // tự duyệt khi mọi ảnh ≥ min
	ApproveAll   = "all"   // tự duyệt hết (chỉ qua Tuỳ chỉnh + xác nhận)
)

// OnError: khi gặp lỗi.
const (
	OnErrorStop      = "stop"
	OnErrorRetryOnce = "retry_once"
)

// BelowQuality: video chưa đạt 90 điểm sau khi dựng.
const (
	BelowAsk        = "ask"
	BelowFixThenAsk = "fix_then_ask"
)

// Ngưỡng điểm ảnh (D6): 70–95, bước 5, mặc định 85.
const (
	ApproveMinLow     = 70
	ApproveMinHigh    = 95
	ApproveMinStep    = 5
	ApproveMinDefault = 85
)

// Ưu tiên kênh (07 §20).
const (
	PrioritySteady = "steady"
	PriorityViews  = "views"
)

// AutonomyApprove là dòng "Duyệt ảnh".
type AutonomyApprove struct {
	Mode string `json:"mode"`
	Min  int    `json:"min"`
}

// Autonomy là owner_channels.autonomy. Preset do server suy ra từ các dòng
// (client không quyết định).
type Autonomy struct {
	Preset       string          `json:"preset"`
	Pick         string          `json:"pick"`
	Approve      AutonomyApprove `json:"approve"`
	OnError      string          `json:"onError"`
	BelowQuality string          `json:"belowQuality"`
}

// AutonomyFieldError là lỗi kiểm tra một dòng, kèm tên trường kiểu
// `autonomy.approve.min` để web chỉ đúng ô.
type AutonomyFieldError struct {
	Field   string
	Message string
}

func (e *AutonomyFieldError) Error() string { return e.Message }

var autonomyPresets = map[string]Autonomy{
	PresetCareful:  {Preset: PresetCareful, Pick: PickSuggest, Approve: AutonomyApprove{Mode: ApproveAsk, Min: ApproveMinDefault}, OnError: OnErrorStop, BelowQuality: BelowAsk},
	PresetBalanced: {Preset: PresetBalanced, Pick: PickAll, Approve: AutonomyApprove{Mode: ApproveAsk, Min: ApproveMinDefault}, OnError: OnErrorRetryOnce, BelowQuality: BelowAsk},
	PresetAuto:     {Preset: PresetAuto, Pick: PickAll, Approve: AutonomyApprove{Mode: ApproveScore, Min: ApproveMinDefault}, OnError: OnErrorRetryOnce, BelowQuality: BelowFixThenAsk},
}

// PresetAutonomy trả các dòng của một mức sẵn (careful|balanced|auto).
func PresetAutonomy(name string) (Autonomy, bool) {
	a, ok := autonomyPresets[name]
	return a, ok
}

// DefaultAutonomy là mức mặc định của kênh mới: Cân bằng.
func DefaultAutonomy() Autonomy { return autonomyPresets[PresetBalanced] }

// Validate kiểm các dòng; Preset không được kiểm vì Normalize ghi đè.
func (a Autonomy) Validate() error {
	switch a.Pick {
	case PickSuggest, PickScore2, PickAll:
	default:
		return &AutonomyFieldError{Field: "autonomy.pick", Message: "Cách lấy video không hợp lệ."}
	}
	switch a.Approve.Mode {
	case ApproveAsk, ApproveScore, ApproveAll:
	default:
		return &AutonomyFieldError{Field: "autonomy.approve.mode", Message: "Cách duyệt ảnh không hợp lệ."}
	}
	if a.Approve.Min < ApproveMinLow || a.Approve.Min > ApproveMinHigh || a.Approve.Min%ApproveMinStep != 0 {
		return &AutonomyFieldError{Field: "autonomy.approve.min", Message: fmt.Sprintf("Ngưỡng điểm phải từ %d đến %d, bước %d.", ApproveMinLow, ApproveMinHigh, ApproveMinStep)}
	}
	switch a.OnError {
	case OnErrorStop, OnErrorRetryOnce:
	default:
		return &AutonomyFieldError{Field: "autonomy.onError", Message: "Cách xử lý lỗi không hợp lệ."}
	}
	switch a.BelowQuality {
	case BelowAsk, BelowFixThenAsk:
	default:
		return &AutonomyFieldError{Field: "autonomy.belowQuality", Message: "Cách xử lý video chưa đạt không hợp lệ."}
	}
	return nil
}

// Normalize ghi Preset theo các dòng: trùng một mức sẵn (không tính ngưỡng
// điểm, như artboard) thì mức đó, còn lại là Tuỳ chỉnh. "Tự duyệt hết" luôn
// là Tuỳ chỉnh vì không mức sẵn nào có nó.
func (a Autonomy) Normalize() Autonomy {
	a.Preset = PresetCustom
	for _, name := range []string{PresetCareful, PresetBalanced, PresetAuto} {
		p := autonomyPresets[name]
		if a.Pick == p.Pick && a.Approve.Mode == p.Approve.Mode && a.OnError == p.OnError && a.BelowQuality == p.BelowQuality {
			a.Preset = name
			break
		}
	}
	return a
}

// AutoPicks báo Tự lấy có chạy: chỉ khi không ở chế độ chỉ đề xuất.
func (a Autonomy) AutoPicks() bool { return a.Pick != PickSuggest }

// RaisesTrust báo `next` giao thêm việc cho máy so với `a` ở mức cần xác nhận
// (D6): chuyển sang mức Tự chạy, hoặc bật "Tự duyệt hết".
func (a Autonomy) RaisesTrust(next Autonomy) bool {
	if next.Approve.Mode == ApproveAll && a.Approve.Mode != ApproveAll {
		return true
	}
	return next.Preset == PresetAuto && a.Preset != PresetAuto
}

// ParseAutonomy đọc cột JSON; cột rỗng hoặc hỏng đọc như Cân bằng để một dòng
// cũ không chặn cả danh sách kênh.
func ParseAutonomy(raw []byte) Autonomy {
	if len(raw) == 0 {
		return DefaultAutonomy()
	}
	var a Autonomy
	if json.Unmarshal(raw, &a) != nil || a.Validate() != nil {
		return DefaultAutonomy()
	}
	return a.Normalize()
}

// ImageScoreParts là bốn mục điểm ảnh của Kiểm định (0–100).
type ImageScoreParts struct {
	Match     int `json:"match"`
	Artifacts int `json:"artifacts"`
	Identity  int `json:"identity"`
	Duplicate int `json:"duplicate"`
}

// ScoreFact là một dòng bằng chứng trong Reason của điểm ảnh.
type ScoreFact struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

// ScoreReason cùng hình insight.Reason (07 §20): Kiểm định giải thích điểm.
type ScoreReason struct {
	Agent string      `json:"agent"`
	Text  string      `json:"text"`
	Facts []ScoreFact `json:"facts"`
}

// ImageScore là điểm Kiểm định của một ảnh vừa vẽ (0–100) kèm bốn mục và lý do.
type ImageScore struct {
	Value  int             `json:"value"`
	Parts  ImageScoreParts `json:"parts"`
	Reason ScoreReason     `json:"reason"`
	At     string          `json:"at,omitempty"`
}

// AutoFix ghi lần máy tự thử lại một lỗi (job.autoFix, 07 §20).
type AutoFix struct {
	At         string `json:"at"`
	Code       string `json:"code"`
	Result     string `json:"result"` // retrying | recovered | failed
	Step       string `json:"step,omitempty"`
	Generation int    `json:"generation"`
}

// Mã lỗi tự sửa được của Mức tự chủ (07 §20).
const (
	AutoFixSourceDownload = "source_download_failed"
	AutoFixImage          = "image_failed"
	AutoFixRender         = "render_failed"
	// AutoFixQuality: video chưa đạt Độ hoàn thiện — chỉ khi belowQuality là
	// fix_then_ask; "sửa" là đo lại một lần rồi mới hỏi.
	AutoFixQuality = "quality_gate"
)

// Kết quả của một lần tự thử lại.
const (
	AutoFixRetrying  = "retrying"
	AutoFixRecovered = "recovered"
	AutoFixFailed    = "failed"
)

// AutoFixCodeOf phân loại một step hỏng thành mã tự sửa được; "" nghĩa là lỗi
// cần người (thiếu công cụ, đăng nhập hết hạn, yêu cầu sai, template đổi…)
// nên báo ngay.
func AutoFixCodeOf(step CloneStep, e *CloneError) string {
	if e == nil {
		return ""
	}
	if e.Code == CloneErrQualityGate && step == CloneScore {
		return AutoFixQuality
	}
	switch e.Code {
	case CloneErrSourceUnavailable, CloneErrInternal:
	default:
		return ""
	}
	switch step {
	case CloneDownload:
		return AutoFixSourceDownload
	case CloneImages:
		return AutoFixImage
	case CloneBuild, CloneScore, CloneExport:
		return AutoFixRender
	}
	return ""
}
