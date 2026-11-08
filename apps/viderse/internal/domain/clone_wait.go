package domain

import "strings"

// Presentation reasons are independent of quota's historical retry/parking code.
const (
	CloneWaitSourceAuth = "source_auth"
	CloneWaitSource     = "source"
	CloneWaitStorage    = "storage"
	CloneWaitTransient  = "transient"
	CloneWaitImageQuota = "image_quota"
)

// CloneWaitReason also reads jobs stored before waitReason existed. Legacy text
// is used only to classify the cause; it is never echoed to the owner.
func CloneWaitReason(step CloneStep, err *CloneError) string {
	if err == nil || err.Code != CloneErrQuota {
		return ""
	}
	if step == CloneImages {
		return CloneWaitImageQuota
	}
	switch err.WaitReason {
	case CloneWaitSourceAuth, CloneWaitSource, CloneWaitStorage, CloneWaitTransient:
		return err.WaitReason
	}
	if step == CloneDownload {
		text := strings.ToLower(err.Message)
		if strings.Contains(text, "not a bot") || strings.Contains(text, "sign in") || strings.Contains(text, "cookie") {
			return CloneWaitSourceAuth
		}
		if strings.Contains(text, "không đủ chỗ") || strings.Contains(text, "no space left") || strings.Contains(text, "disk quota") {
			return CloneWaitStorage
		}
		if strings.Contains(text, "tải nguồn ngừng tiến triển") || strings.Contains(text, "tải nguồn vượt giới hạn thời gian") || strings.Contains(text, "transcode nguồn bị timeout") {
			return CloneWaitTransient
		}
		return CloneWaitSource
	}
	if step == CloneBackdrop {
		return CloneWaitSource
	}
	return CloneWaitTransient
}

func cloneWaitLabel(reason string) string {
	switch reason {
	case CloneWaitImageQuota:
		return "Chờ hạn mức ảnh"
	case CloneWaitSourceAuth:
		return "Chờ xác thực YouTube"
	case CloneWaitSource:
		return "Chờ tải lại nguồn"
	case CloneWaitStorage:
		return "Chờ dung lượng lưu trữ"
	default:
		return "Chờ thử lại"
	}
}
