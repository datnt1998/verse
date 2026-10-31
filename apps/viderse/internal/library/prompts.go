package library

import (
	"sort"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Bảy prompt ảnh có sẵn của Viderse: một prompt ảnh cảnh
// "Theo nhận diện kênh" + sáu prompt ảnh bìa, mỗi bố cục một prompt. Đây là
// hằng số trong code — không có dòng trong image_prompts — nên id cố định,
// không sửa và không xoá được. Tên/nội dung lấy từ artboard Library (mảng PR)
// + 06-copy.

// Id có sẵn.
const (
	BuiltinScene            = "builtin:scene"
	BuiltinCoverClean       = "builtin:cover-clean"
	BuiltinCoverHook        = "builtin:cover-hook"
	BuiltinCoverCircleArrow = "builtin:cover-circle-arrow"
	BuiltinCoverHookArrow   = "builtin:cover-hook-arrow"
	BuiltinCoverSplit       = "builtin:cover-split"
	BuiltinCoverCloseup     = "builtin:cover-closeup"
)

// builtinPromptAt ghim mốc thời gian "có sẵn" — prompt hằng số không đổi theo
// lần chạy nên updatedAt phải ổn định để client cache được.
var builtinPromptAt = time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)

// BuiltinPrompts trả bảy prompt có sẵn theo thứ tự hiển thị: ảnh cảnh trước,
// rồi ảnh bìa theo tập ThumbnailStyles.
func BuiltinPrompts() []domain.ImagePrompt {
	scene := domain.ImagePrompt{
		ID:        BuiltinScene,
		Name:      "Theo nhận diện kênh",
		Kind:      domain.PromptScene,
		BuiltIn:   true,
		Prompt:    "Vẽ theo phong cách nhận diện của kênh đang làm video (app tự rút từ đối thủ tham chiếu khi tạo kênh, xem ở Hồ sơ kênh): cùng chất liệu, màu và ánh sáng với avatar và ảnh bìa kênh. Mỗi kênh ra một vẻ riêng. Mẫu chuẩn của kênh mới dùng prompt này.",
		UpdatedAt: builtinPromptAt,
	}
	covers := []domain.ImagePrompt{
		{
			ID:          BuiltinCoverClean,
			Name:        "Ảnh sạch",
			Kind:        domain.PromptCover,
			BuiltIn:     true,
			Composition: domain.ThumbClean,
			Prompt:      "Ảnh đẹp không chữ, để tiêu đề video tự nói.",
			UpdatedAt:   builtinPromptAt,
		},
		{
			ID:          BuiltinCoverHook,
			Name:        "Chữ hook",
			Kind:        domain.PromptCover,
			BuiltIn:     true,
			Composition: domain.ThumbHook,
			Prompt:      "1–4 chữ to gợi tò mò, Đạo diễn lấy từ chi tiết lạ nhất của câu chuyện. Nhân vật ở một bên, bên kia để trống cho chữ.",
			UpdatedAt:   builtinPromptAt,
		},
		{
			ID:          BuiltinCoverCircleArrow,
			Name:        "Khoanh tròn + mũi tên",
			Kind:        domain.PromptCover,
			BuiltIn:     true,
			Composition: domain.ThumbCircleArrow,
			Prompt:      "Khoanh tròn một chi tiết đáng ngờ có thật trong truyện, kèm mũi tên đỏ chỉ vào. Không chữ.",
			UpdatedAt:   builtinPromptAt,
		},
		{
			ID:          BuiltinCoverHookArrow,
			Name:        "Chữ hook + mũi tên",
			Kind:        domain.PromptCover,
			BuiltIn:     true,
			Composition: domain.ThumbHookArrow,
			Prompt:      "Chữ hook ở phần trống, mũi tên đỏ chỉ vào chi tiết chính của câu chuyện.",
			UpdatedAt:   builtinPromptAt,
		},
		{
			ID:          BuiltinCoverSplit,
			Name:        "Trước / sau",
			Kind:        domain.PromptCover,
			BuiltIn:     true,
			Composition: domain.ThumbSplit,
			Prompt:      "Chia đôi khung: bên trái lúc đầu, bên phải lúc sau, cùng nhân vật nhưng số phận khác hẳn. Không chữ.",
			UpdatedAt:   builtinPromptAt,
		},
		{
			ID:          BuiltinCoverCloseup,
			Name:        "Cận mặt cảm xúc",
			Kind:        domain.PromptCover,
			BuiltIn:     true,
			Composition: domain.ThumbCloseup,
			Prompt:      "Mặt nhân vật chiếm hai phần ba khung, cảm xúc cực rõ, nền mờ là nơi chốn của câu chuyện.",
			UpdatedAt:   builtinPromptAt,
		},
	}
	return append([]domain.ImagePrompt{scene}, covers...)
}

// BuiltinPrompt tra một prompt có sẵn theo id; ok=false khi không phải id có
// sẵn (id người dùng là uuid).
func BuiltinPrompt(id string) (domain.ImagePrompt, bool) {
	for _, p := range BuiltinPrompts() {
		if p.ID == id {
			return p, true
		}
	}
	return domain.ImagePrompt{}, false
}

// IsBuiltinPromptID báo id là prompt có sẵn.
func IsBuiltinPromptID(id string) bool {
	_, ok := BuiltinPrompt(id)
	return ok
}

// builtinNameTaken báo tên trùng tên prompt có sẵn (không phân biệt hoa
// thường; DB chỉ chặn trùng giữa các prompt người dùng).
func builtinNameTaken(name string) bool {
	folded := strings.ToLower(strings.TrimSpace(name))
	for _, p := range BuiltinPrompts() {
		if strings.ToLower(p.Name) == folded {
			return true
		}
	}
	return false
}

// sortPrompts đặt prompt có sẵn trước (thứ tự BuiltinPrompts), rồi prompt
// người dùng theo updatedAt giảm dần.
func sortPrompts(list []domain.ImagePrompt) {
	order := make(map[string]int)
	for i, p := range BuiltinPrompts() {
		order[p.ID] = i
	}
	sort.SliceStable(list, func(i, j int) bool {
		oi, iok := order[list[i].ID]
		oj, jok := order[list[j].ID]
		switch {
		case iok && jok:
			return oi < oj
		case iok != jok:
			return iok
		}
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
}

// filterPrompts lọc theo kind và q (đã gấp dấu, chữ thường) trên tên + nội dung.
func filterPrompts(list []domain.ImagePrompt, kind domain.PromptKind, q string) []domain.ImagePrompt {
	out := make([]domain.ImagePrompt, 0, len(list))
	for _, p := range list {
		if kind != "" && p.Kind != kind {
			continue
		}
		if q != "" && !strings.Contains(foldText(p.Name), q) && !strings.Contains(foldText(p.Prompt), q) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// foldText hạ chữ và bỏ dấu tiếng Việt — cùng luật fold_title của migration
// để tìm trong code và tìm trong DB cho cùng kết quả.
func foldText(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'à', 'á', 'ạ', 'ả', 'ã', 'â', 'ầ', 'ấ', 'ậ', 'ẩ', 'ẫ', 'ă', 'ằ', 'ắ', 'ặ', 'ẳ', 'ẵ':
			b.WriteRune('a')
		case 'è', 'é', 'ẹ', 'ẻ', 'ẽ', 'ê', 'ề', 'ế', 'ệ', 'ể', 'ễ':
			b.WriteRune('e')
		case 'ì', 'í', 'ị', 'ỉ', 'ĩ':
			b.WriteRune('i')
		case 'ò', 'ó', 'ọ', 'ỏ', 'õ', 'ô', 'ồ', 'ố', 'ộ', 'ổ', 'ỗ', 'ơ', 'ờ', 'ớ', 'ợ', 'ở', 'ỡ':
			b.WriteRune('o')
		case 'ù', 'ú', 'ụ', 'ủ', 'ũ', 'ư', 'ừ', 'ứ', 'ự', 'ử', 'ữ':
			b.WriteRune('u')
		case 'ỳ', 'ý', 'ỵ', 'ỷ', 'ỹ':
			b.WriteRune('y')
		case 'đ':
			b.WriteRune('d')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
