package aiconn

import (
	"context"
	"net/http"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Các capability mỗi vai trò đòi hỏi (§15): Đạo diễn cần text; Hoạ sĩ cần
// vision để viết mô tả ảnh và image để vẽ (cả chuỗi dự phòng).
const (
	needText   = "text"
	needVision = "vision"
	needImage  = "image"
)

// Agents đọc gán model của các trợ lý; chưa cấu hình → mặc định §15.
func (s *Service) Agents(ctx context.Context) (domain.AgentModels, error) {
	var agents domain.AgentModels
	found, err := s.o.Settings.Get(ctx, domain.KeyAIAgents, &agents)
	if err != nil {
		return domain.AgentModels{}, err
	}
	if !found {
		return domain.DefaultAgentModels(), nil
	}
	return normalizeAgents(agents), nil
}

// normalizeAgents điền phần thiếu theo luật §15: fallbackOn mặc định bật và
// drawFallback mặc định là nguồn dự phòng còn lại khi draw là một trong hai
// nguồn gói.
func normalizeAgents(a domain.AgentModels) domain.AgentModels {
	if a.Artist.FallbackOn && len(a.Artist.DrawFallback) == 0 {
		other := otherDrawSource(a.Artist.Draw)
		if other.Provider != "" {
			a.Artist.DrawFallback = []domain.ModelRef{other}
		}
	}
	return a
}

// otherDrawSource trả nguồn vẽ "còn lại" của hai nguồn gói.
func otherDrawSource(draw domain.ModelRef) domain.ModelRef {
	switch {
	case draw.Provider == "chatgpt" || (draw.Provider == "codex"):
		return domain.ModelRef{Provider: "grok", Model: "grok-imagine-image"}
	case draw.Provider == "grok":
		return domain.ModelRef{Provider: "chatgpt", Model: "gpt-image-2"}
	}
	return domain.ModelRef{}
}

// PutAgents kiểm tra rồi lưu AgentModels: mọi ModelRef phải tồn tại trong
// danh mục đã lưu VÀ đang được chọn; thiếu capability → 422
// capability_missing kèm need như hợp đồng; ModelRef không có đường chạy
// thật (kind/model không có executor) → 422 no_executor. Chạy dưới cfgMu
// cùng PatchProvider/DeleteProvider nên gán và đổi khoá/xoá thẻ không chen
// nhau.
func (s *Service) PutAgents(ctx context.Context, agents domain.AgentModels) (domain.AgentModels, error) {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()

	agents = normalizeAgents(agents)
	if err := s.validateAgents(ctx, agents); err != nil {
		return domain.AgentModels{}, err
	}
	if err := s.o.Settings.Set(ctx, domain.KeyAIAgents, agents); err != nil {
		return domain.AgentModels{}, err
	}
	s.changed("agents")
	if s.o.OnAgentsChanged != nil {
		s.o.OnAgentsChanged()
	}
	return agents, nil
}

// validateAgents kiểm từng ModelRef: tồn tại + đã chọn + capability, rồi —
// khi composition root cung cấp Executors — có đường chạy thật cho vai trò.
func (s *Service) validateAgents(ctx context.Context, a domain.AgentModels) error {
	check := func(field string, ref domain.ModelRef, need string) error {
		model, err := s.usableModel(ctx, ref)
		if err != nil {
			return err
		}
		if !hasCap(model.Caps, need) {
			return &Error{Status: http.StatusUnprocessableEntity, Reason: domain.ReasonCapabilityMissing,
				Message: "Model " + ref.Model + " không làm được việc này.",
				Fields:  map[string]any{"need": need, "field": field, "model": ref.Model}}
		}
		return s.checkExecutable(ctx, field, ref, need)
	}
	if err := check("director.model", a.Director.Model, needText); err != nil {
		return err
	}
	switch a.Director.Effort {
	case "auto", "low", "medium", "high":
	default:
		return apiError(http.StatusUnprocessableEntity, "invalid_effort", "Mức suy nghĩ không hợp lệ")
	}
	for _, ref := range a.Director.Fallback {
		if err := check("director.fallback", ref, needText); err != nil {
			return err
		}
	}
	if err := check("artist.promptModel", a.Artist.PromptModel, needVision); err != nil {
		return err
	}
	if err := check("artist.draw", a.Artist.Draw, needImage); err != nil {
		return err
	}
	for _, ref := range a.Artist.DrawFallback {
		if err := check("artist.drawFallback", ref, needImage); err != nil {
			return err
		}
	}
	return nil
}

// checkExecutable báo ModelRef của một vai trò có đường chạy thật. need =
// text|vision → đường text (director/prompt); need = image → đường vẽ.
// Executors nil (test không buộc) → coi như chạy được, chỉ capability kiểm.
func (s *Service) checkExecutable(ctx context.Context, field string, ref domain.ModelRef, need string) error {
	if s.o.Executors == nil {
		return nil
	}
	var ok bool
	if need == needImage {
		ok = s.o.Executors.CanDraw(ctx, ref)
	} else {
		ok = s.o.Executors.CanText(ctx, ref)
	}
	if !ok {
		return &Error{Status: http.StatusUnprocessableEntity, Reason: domain.ReasonNoExecutor,
			Message: "Model " + ref.Model + " chưa có đường chạy cho việc này.",
			Fields:  map[string]any{"need": need, "field": field, "model": ref.Model}}
	}
	return nil
}

// usableModel trả model của một ModelRef: provider và model phải tồn tại và
// model phải đang được chọn (ô chọn của Trợ lý AI chỉ liệt kê model đã chọn).
func (s *Service) usableModel(ctx context.Context, ref domain.ModelRef) (domain.AIModel, error) {
	p, err := s.provider(ctx, ref.Provider)
	if err != nil {
		return domain.AIModel{}, err
	}
	model, ok, err := s.o.Store.GetModel(ctx, p.ID, ref.Model)
	if err != nil {
		return domain.AIModel{}, err
	}
	if !ok {
		return domain.AIModel{}, &Error{Status: http.StatusUnprocessableEntity,
			Reason:  domain.ReasonUnknownModel,
			Message: "Không có model " + ref.Model + " trong danh mục của " + p.Label + ".",
			Fields:  map[string]any{"model": ref.Model}}
	}
	for _, id := range p.Selected {
		if id == ref.Model {
			return model, nil
		}
	}
	return domain.AIModel{}, &Error{Status: http.StatusUnprocessableEntity,
		Reason:  domain.ReasonUnknownModel,
		Message: "Model " + ref.Model + " chưa được chọn ở Kết nối AI.",
		Fields:  map[string]any{"model": ref.Model}}
}

// hasCap báo caps chứa need.
func hasCap(caps []domain.Capability, need string) bool {
	for _, c := range caps {
		if c == need {
			return true
		}
	}
	return false
}

// ResolveDirector trả chuỗi ModelRef của Đạo diễn theo thứ tự gọi: model
// chính rồi đến fallback — pipeline ghi modelRef vào job khi step bắt đầu.
func (s *Service) ResolveDirector(ctx context.Context) ([]domain.ModelRef, error) {
	agents, err := s.Agents(ctx)
	if err != nil {
		return nil, err
	}
	out := []domain.ModelRef{agents.Director.Model}
	out = append(out, agents.Director.Fallback...)
	return out, nil
}

// ArtistResolution là lựa chọn đầy đủ của Hoạ sĩ: model viết mô tả ảnh và
// chuỗi nguồn vẽ theo thứ tự (draw trước, drawFallback sau khi fallbackOn).
type ArtistResolution struct {
	PromptModel domain.ModelRef
	DrawChain   []domain.ModelRef
}

// ResolveArtist trả lựa chọn của Hoạ sĩ; DrawChain rỗng khi tắt dự phòng và
// chỉ có model chính.
func (s *Service) ResolveArtist(ctx context.Context) (ArtistResolution, error) {
	agents, err := s.Agents(ctx)
	if err != nil {
		return ArtistResolution{}, err
	}
	chain := []domain.ModelRef{agents.Artist.Draw}
	if agents.Artist.FallbackOn {
		chain = append(chain, agents.Artist.DrawFallback...)
	}
	return ArtistResolution{PromptModel: agents.Artist.PromptModel, DrawChain: chain}, nil
}
