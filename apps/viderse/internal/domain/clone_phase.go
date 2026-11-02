package domain

import (
	"strings"
)

// clonePhaseVerbs dịch pha máy chủ phát (`CloneProgress.Phase`) thành câu việc
// tiếng Việt ở dạng động từ, không kèm "đang" — ghép sau tên trợ lý:
// "Trinh sát đang tải video nguồn". Khoá có tiền tố bước khi cùng một mã mang
// nghĩa khác nhau (publish ở bước tải là lưu nguồn, ở bước lời kể là lưu lời
// kể); khoá không tiền tố là dự phòng.
//
// Mã gốc vẫn nằm nguyên trên dây (`CloneProgress.Phase`), trong checkpoint và
// log — bảng này chỉ đổi câu hiện cho người đọc. `web/src/i18n/screens.ts`
// (progressCopy.phases) giữ bảng tương ứng cho nhật ký.
var clonePhaseVerbs = map[string]string{
	"download:metadata":      "đọc thông tin nguồn",
	"download:download":      "tải video nguồn",
	"download:merge":         "ghép âm thanh và hình",
	"download:probe":         "kiểm tra nguồn",
	"download:transcode":     "chuyển mã nguồn",
	"download:import":        "nhập nguồn vào máy",
	"download:publish":       "lưu nguồn",
	"backdrop:refresh":       "quét nền của kênh",
	"backdrop:pick":          "chọn nền",
	"backdrop:fetch":         "tải clip nền",
	"transcript:decode":      "giải mã âm thanh nguồn",
	"transcript:vad":         "tách lời nói",
	"transcript:asr":         "nghe lời kể",
	"transcript:align":       "khớp phụ đề với tiếng",
	"transcript:publish":     "lưu lời kể",
	"images:images":          "vẽ ảnh",
	"scenes:keyframes":       "dựng ảnh cảnh",
	"score:measuring":        "đo độ hoàn thiện",
	"export:rendering":       "dựng video",
	"export:rendering-again": "dựng lại video",
	"export:uploading":       "lưu file video",
	"export:uploading-again": "lưu lại file video",
	"export:compressing":     "nén video",
	"metadata":               "đọc thông tin nguồn",
	"download":               "tải video nguồn",
	"merge":                  "ghép âm thanh và hình",
	"probe":                  "kiểm tra nguồn",
	"transcode":              "chuyển mã nguồn",
	"import":                 "nhập nguồn vào máy",
	"publish":                "lưu nguồn",
	"refresh":                "quét nền của kênh",
	"pick":                   "chọn nền",
	"fetch":                  "tải clip nền",
	"decode":                 "giải mã âm thanh nguồn",
	"vad":                    "tách lời nói",
	"asr":                    "nghe lời kể",
	"align":                  "khớp phụ đề với tiếng",
	"keyframes":              "dựng ảnh cảnh",
	"measuring":              "đo độ hoàn thiện",
	"rendering":              "dựng video",
	"rendering-again":        "dựng lại video",
	"uploading":              "lưu file video",
	"uploading-again":        "lưu lại file video",
	"compressing":            "nén video",
}

// clonePhaseVerb trả câu việc tiếng Việt cho pha của một bước; "" khi mã lạ để
// chỗ gọi dùng câu chung — không bao giờ hiện mã enum trần cho người đọc.
func clonePhaseVerb(step CloneStep, phase string) string {
	p := strings.TrimSpace(phase)
	if p == "" {
		return ""
	}
	key := strings.ToLower(p)
	if verb, ok := clonePhaseVerbs[string(step)+":"+key]; ok {
		return verb
	}
	return clonePhaseVerbs[key]
}
