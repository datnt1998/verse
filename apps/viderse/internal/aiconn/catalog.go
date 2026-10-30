package aiconn

import (
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/core/models"
)

// ModelLookup là phần registry metadata aiconn cần (core/models.ModelRegistry
// thoả mãn); tiêm vào để test không phụ thuộc baseline biên dịch.
type ModelLookup interface {
	Resolve(pattern string) (*models.ModelEntry, bool)
}

// registry trả lookup đã tiêm hoặc registry mặc định của core.
func (s *Service) lookup(id string) (*models.ModelEntry, bool) {
	if s.o.ModelRegistry != nil {
		return s.o.ModelRegistry.Resolve(id)
	}
	return models.DefaultRegistry().Resolve(id)
}

// modelRegistryKey là tiền tố provider trong registry cho từng kind — dùng để
// thử "provider/model" trước khi resolve tên trần (id openrouter đã có tiền
// tố vendor thì resolve trực tiếp).
var modelRegistryKey = map[domain.ProviderKind]string{
	domain.KindOpenAI:     "openai",
	domain.KindGemini:     "gemini",
	domain.KindDeepSeek:   "deepseek",
	domain.KindXAI:        "grok",
	domain.KindAnthropic:  "anthropic",
	domain.KindOpenRouter: "",
}

// enrich điền context/giá/caps từ registry metadata lên danh mục vừa lấy.
func (s *Service) enrich(kind domain.ProviderKind, in []domain.AIModel) []domain.AIModel {
	out := make([]domain.AIModel, len(in))
	for i, m := range in {
		out[i] = s.enrichModel(kind, m)
	}
	return out
}

// enrichModel điền metadata cho một model: registry trước (giá + context có
// thật), rồi suy luận caps theo tên nếu adapter chưa khai báo.
func (s *Service) enrichModel(kind domain.ProviderKind, m domain.AIModel) domain.AIModel {
	if key, ok := modelRegistryKey[kind]; ok {
		patterns := []string{m.ID}
		if key != "" && !strings.Contains(m.ID, "/") {
			patterns = append([]string{key + "/" + m.ID}, patterns...)
		}
		for _, pat := range patterns {
			if e, ok := s.lookup(pat); ok {
				if m.ContextK == 0 && e.ContextWindow > 0 {
					m.ContextK = e.ContextWindow / 1000
				}
				if m.InPerM == 0 && e.InputCostPer1M > 0 {
					m.InPerM = e.InputCostPer1M
				}
				if m.OutPerM == 0 && e.OutputCostPer1M > 0 {
					m.OutPerM = e.OutputCostPer1M
				}
				break
			}
		}
	}
	// Caps nil = adapter không có ý kiến (suy theo tên); caps rỗng = adapter
	// đã nói rõ model không làm việc nào trong ba capability của app (vd model
	// embedding) — không được đoán đè.
	if m.Caps == nil {
		m.Caps = inferCaps(kind, m.ID)
	}
	if !m.Recommended && isRecommended(kind, m.ID) {
		m.Recommended = true
	}
	return m
}

// inferCaps suy ra capability từ id model khi provider không tự khai báo —
// heuristic thận trọng: model đa phương thức phổ biến được nhận; còn lại
// "text" để người dùng tự gán thêm qua models/check khi cần.
func inferCaps(kind domain.ProviderKind, id string) []domain.Capability {
	l := strings.ToLower(id)
	// Model vẽ ảnh chuyên dụng.
	imageHints := []string{"gpt-image", "dall-e", "imagen", "image", "nano-banana", "seedream", "flux", "-vl-", "kolors"}
	for _, h := range imageHints {
		if strings.Contains(l, h) {
			// "-vl-" và một số model "image" thực ra là đọc ảnh (vision), không vẽ.
			if h == "-vl-" || strings.Contains(l, "vl-") || strings.Contains(l, "vision") {
				return []domain.Capability{domain.CapText, domain.CapVision}
			}
			return []domain.Capability{domain.CapImage}
		}
	}
	// Chỉ xử lý không phải sinh chữ/ảnh: bỏ qua khỏi gán trợ lý nhưng vẫn liệt kê.
	skipHints := []string{"embed", "tts", "whisper", "moderation", "transcribe", "rerank", "audio", "guard"}
	for _, h := range skipHints {
		if strings.Contains(l, h) {
			return []domain.Capability{}
		}
	}
	// Vision theo tên phổ biến.
	visionHints := []string{"vision", "-vl", "4o", "gemini", "claude", "gpt-4", "gpt-5", "gpt-6", "pixtral", "llava", "minicpm-v", "qwen2-vl", "qwen3-vl", "grok-4", "sonnet", "opus"}
	for _, h := range visionHints {
		if strings.Contains(l, h) {
			return []domain.Capability{domain.CapText, domain.CapVision}
		}
	}
	return []domain.Capability{domain.CapText}
}

// isRecommended đánh dấu model nên hiện ở lọc "Gợi ý" — bộ chọn thận trọng:
// đúng model mặc định của app và flagship phổ biến của từng nhà cung cấp.
func isRecommended(kind domain.ProviderKind, id string) bool {
	l := strings.ToLower(id)
	switch kind {
	case domain.KindChatGPT:
		return l == "gpt-6.1-sol" || l == "gpt-image-2"
	case domain.KindGrok:
		return l == "grok-imagine-image" || strings.HasPrefix(l, "grok-4.")
	case domain.KindOllama:
		return false
	case domain.KindOpenRouter:
		return strings.Contains(l, "gemini-3-pro") || strings.Contains(l, "claude-sonnet") ||
			strings.Contains(l, "gemini-3.1-flash-image") || strings.Contains(l, "nano-banana")
	case domain.KindGemini:
		return strings.Contains(l, "gemini-3-pro") || strings.Contains(l, "gemini-3.5-flash") ||
			strings.Contains(l, "gemini-3.1-flash-image")
	case domain.KindAnthropic:
		return strings.Contains(l, "claude-sonnet") || strings.Contains(l, "claude-opus")
	case domain.KindOpenAI:
		return strings.HasPrefix(l, "gpt-5") || strings.HasPrefix(l, "gpt-6")
	case domain.KindXAI:
		return strings.HasPrefix(l, "grok-4")
	case domain.KindDeepSeek:
		return l == "deepseek-chat" || l == "deepseek-reasoner"
	}
	return false
}
