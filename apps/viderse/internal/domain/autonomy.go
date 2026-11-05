package domain

// Kiểm định ảnh và lần máy tự sửa lỗi (07 §20): điểm ảnh, ngưỡng tự duyệt cố
// định và mã lỗi thuộc biên tự sửa. Chính sách tự sửa là cố định sau V38 —
// không còn cấu hình mức tự chủ, và không đường nào ở đây tự đăng, tự xoá hay
// đổi template.

// AutoApproveMin là ngưỡng duyệt ảnh cố định sau V38: mọi ảnh phải đạt từ
// ngưỡng này mới được tự duyệt (>=, không >).
const AutoApproveMin = 85

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

// Mã lỗi thuộc biên tự sửa cố định (07 §20).
const (
	AutoFixSourceDownload = "source_download_failed"
	AutoFixImage          = "image_failed"
	AutoFixRender         = "render_failed"
	// AutoFixQuality chỉ còn để đọc lịch sử clone_jobs.auto_fix/clone_auto_actions
	// cũ; sau V38 video dưới 90 luôn hỏi nên mã này không đi vào nhánh tự sửa.
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
	// Điểm hoàn thiện dưới 90 luôn hỏi chủ kênh sau V38 → không tự sửa.
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
