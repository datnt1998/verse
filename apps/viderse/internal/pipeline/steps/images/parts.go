package images

import (
	"strings"
)

// PromptPartKind là một phần của prompt đã gửi (07 §26). Thứ tự ghép cố định:
// scene → style → refs → learned → avoid. "learned" chỉ còn trong prompt cũ;
// prompt mới không ghép phần này nhưng vẫn đọc được nguyên văn đã gửi.
type PromptPartKind string

const (
	PartScene   PromptPartKind = "scene"
	PartStyle   PromptPartKind = "style"
	PartRefs    PromptPartKind = "refs"
	PartLearned PromptPartKind = "learned"
	PartAvoid   PromptPartKind = "avoid"
)

// PartOrder là thứ tự ghép của các phần trong một prompt.
var PartOrder = []PromptPartKind{PartScene, PartStyle, PartRefs, PartLearned, PartAvoid}

// PromptPartSource ghi nguồn của một phần: ai viết nó và lấy từ đâu.
type PromptPartSource struct {
	Type  string `json:"type"` // artist | artist_feedback | image_prompt | template | user
	ID    string `json:"id,omitempty"`
	Label string `json:"label"`
}

// PromptPart là một phần prompt kèm nguồn; Text là nguyên văn đã gửi.
type PromptPart struct {
	Kind   PromptPartKind   `json:"kind"`
	Text   string           `json:"text"`
	Source PromptPartSource `json:"source"`
}

// PromptParts là các phần của một prompt theo thứ tự ghép.
type PromptParts []PromptPart

// Text ghép các phần bằng một dòng trống — đúng chuỗi gửi nhà cung cấp.
func (p PromptParts) Text() string {
	parts := make([]string, 0, len(p))
	for _, part := range p {
		text := strings.TrimRight(part.Text, "\n")
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

// Get trả nguyên văn của một phần (rỗng khi không có).
func (p PromptParts) Get(kind PromptPartKind) string {
	for _, part := range p {
		if part.Kind == kind {
			return part.Text
		}
	}
	return ""
}

// With thay nguyên văn của một phần đã có; phần chưa có được thêm vào đúng thứ
// tự ghép. Chỉ phần được sửa đổi — các phần khác giữ nguyên văn.
func (p PromptParts) With(kind PromptPartKind, text string, source PromptPartSource) PromptParts {
	out := make(PromptParts, 0, len(p)+1)
	found := false
	for _, part := range p {
		if part.Kind == kind {
			part.Text = text
			if source.Type != "" {
				part.Source = source
			}
			found = true
		}
		out = append(out, part)
	}
	if !found && strings.TrimSpace(text) != "" {
		out = append(out, PromptPart{Kind: kind, Text: text, Source: source})
		out = out.Sorted()
	}
	return out
}

// Sorted xếp các phần theo PartOrder; phần lạ giữ nguyên thứ tự cuối.
func (p PromptParts) Sorted() PromptParts {
	out := make(PromptParts, 0, len(p))
	for _, kind := range PartOrder {
		for _, part := range p {
			if part.Kind == kind {
				out = append(out, part)
			}
		}
	}
	for _, part := range p {
		known := false
		for _, kind := range PartOrder {
			if part.Kind == kind {
				known = true
				break
			}
		}
		if !known {
			out = append(out, part)
		}
	}
	return out
}

// PartOverrides là phần prompt chủ kênh sửa tay cho đúng một ảnh. Chỉ phần có
// mặt mới bị thay; các phần khác giữ nguyên.
type PartOverrides map[PromptPartKind]string

// Apply thay các phần được chỉ định bằng nguyên văn của chủ kênh.
func (o PartOverrides) Apply(parts PromptParts) PromptParts {
	if len(o) == 0 {
		return parts
	}
	out := append(PromptParts(nil), parts...)
	for _, kind := range PartOrder {
		text, ok := o[kind]
		if !ok {
			continue
		}
		if kind == PartLearned {
			// Ghi chú đã học chỉ đọc ở đây: sửa prompt ảnh không đổi Ghi chú
			// đã học của kênh (07 §19).
			continue
		}
		out = out.With(kind, text, PromptPartSource{Type: "user", Label: "Bạn sửa"})
	}
	return out.Sorted()
}

// PromptWrapper là phần văn bản KHÔNG sửa được đi kèm các phần prompt: hàng
// rào an toàn (mọi nguồn vẽ nhận) và chỉ dẫn công cụ của đúng nguồn vẽ (Codex
// nhận, Grok/HTTP không). 07 §26 chưa có chỗ cho nó — trường thêm `wrapper`;
// `sentText` vẫn là toàn bộ nguyên văn đã gửi.
type PromptWrapper struct {
	Guard []string `json:"guard,omitempty"`
	Tool  []string `json:"tool,omitempty"`
	// Provider là nguồn vẽ mà Tool thuộc về (rỗng khi không có Tool).
	Provider string `json:"provider,omitempty"`
}

// Text ghép các khối bằng một dòng trống: hàng rào trước, chỉ dẫn công cụ sau.
func (w PromptWrapper) Text() string {
	blocks := make([]string, 0, len(w.Guard)+len(w.Tool))
	for _, block := range append(append([]string(nil), w.Guard...), w.Tool...) {
		if text := strings.TrimRight(block, "\n"); text != "" {
			blocks = append(blocks, text)
		}
	}
	return strings.Join(blocks, "\n\n")
}

// JoinPrompt ghép nguyên văn gửi nhà cung cấp: hàng rào an toàn, các phần sửa
// được, rồi chỉ dẫn công cụ — mỗi khối cách nhau một dòng trống.
func JoinPrompt(w PromptWrapper, p PromptParts) string {
	blocks := make([]string, 0, len(w.Guard)+len(w.Tool)+1)
	for _, block := range w.Guard {
		if text := strings.TrimRight(block, "\n"); text != "" {
			blocks = append(blocks, text)
		}
	}
	if text := p.Text(); text != "" {
		blocks = append(blocks, text)
	}
	for _, block := range w.Tool {
		if text := strings.TrimRight(block, "\n"); text != "" {
			blocks = append(blocks, text)
		}
	}
	return strings.Join(blocks, "\n\n")
}

// RefRole là vai trò của một ảnh tham chiếu gửi kèm.
type RefRole string

const (
	RefLayout    RefRole = "layout"
	RefCharacter RefRole = "character"
	RefStyle     RefRole = "style"
	RefObject    RefRole = "object"
)

// RefSource ghi nguồn của ảnh tham chiếu: khung đối thủ, ảnh trong video hay
// tệp chủ kênh tải lên.
type RefSource struct {
	Type    string   `json:"type"` // rival_frame | video_image | upload
	Label   string   `json:"label"`
	VideoID string   `json:"videoId,omitempty"`
	AtSec   *float64 `json:"atSec,omitempty"`
	ImageID string   `json:"imageId,omitempty"`
}

// ImageRef là một ảnh tham chiếu đã gửi kèm một lần vẽ. Path/SHA256 là đường
// dẫn artifact trong video (để dựng URL khi đọc); URL do tầng API điền.
type ImageRef struct {
	ID     string    `json:"id"`
	Role   RefRole   `json:"role"`
	URL    string    `json:"url,omitempty"`
	Path   string    `json:"path,omitempty"`
	SHA256 string    `json:"sha256,omitempty"`
	Source RefSource `json:"source"`
}

// ValidRefRole cho biết vai trò có thuộc bốn vai trò của §26.
func ValidRefRole(role RefRole) bool {
	switch role {
	case RefLayout, RefCharacter, RefStyle, RefObject:
		return true
	}
	return false
}

// PromptParams là tham số gửi nhà cung cấp (kích thước canvas, chất lượng).
type PromptParams struct {
	Size    string `json:"size"`
	Quality string `json:"quality,omitempty"`
}

// VersionError là lỗi của một lần vẽ, đủ để giao diện nói lý do thật.
type VersionError struct {
	Reason  string  `json:"reason"` // quota | filter | network | provider
	RetryAt *string `json:"retryAt,omitempty"`
	Message string  `json:"message"`
}

// Trạng thái của một phiên bản ảnh (07 §26).
const (
	VersionDrawing   = "drawing"
	VersionReady     = "ready"
	VersionFailed    = "failed"
	VersionCancelled = "cancelled"
)

// State trả trạng thái của phiên bản: bản cũ chưa có trường này mà có tệp thì
// là bản đã vẽ xong.
func (v Version) State() string {
	if v.StateName != "" {
		return v.StateName
	}
	if v.Path != "" {
		return VersionReady
	}
	return VersionFailed
}

// FeedbackText trả góp ý sinh ra bản này (rỗng khi không có).
func (v Version) FeedbackText() string {
	if v.Feedback == nil {
		return ""
	}
	return *v.Feedback
}
