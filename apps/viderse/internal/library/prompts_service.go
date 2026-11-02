package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Kho Prompt ảnh: 7 prompt có sẵn đọc từ hằng số, prompt của bạn đọc
// từ image_prompts. Tên duy nhất không phân biệt hoa thường và không được
// trùng tên prompt có sẵn. Sửa/xoá prompt có sẵn trả 403; đổi loại hoặc xoá
// khi template còn dùng trả 409 in_use kèm danh sách template.

// PromptInput là body POST /api/image-prompts (hoặc nguồn nhân bản fromId).
type PromptInput struct {
	Name        string
	Kind        domain.PromptKind
	Composition string
	Prompt      *string // nil = lấy của fromId (hoặc rỗng khi tạo mới)
	FromID      string
}

// PromptPatch là body PATCH /api/image-prompts/:id; nil = giữ nguyên.
type PromptPatch struct {
	Name        *string
	Kind        *string
	Composition *string
	Prompt      *string
}

// ListPrompts trả prompt có sẵn trước rồi prompt của bạn theo updatedAt.
func (s *Service) ListPrompts(ctx context.Context, kind domain.PromptKind, q string) ([]domain.ImagePrompt, error) {
	if kind != "" && kind != domain.PromptScene && kind != domain.PromptCover {
		return nil, invalidInput("Loại chỉ dẫn không hợp lệ", "Chọn Ảnh cảnh hoặc Ảnh bìa")
	}
	query := strings.TrimSpace(q)
	if len([]rune(query)) > 200 {
		return nil, invalidInput("Từ khoá tìm quá dài", "Rút gọn còn tối đa 200 ký tự")
	}
	mine, err := s.store.ListPrompts(ctx, kind, foldText(query))
	if err != nil {
		return nil, err
	}
	out := filterPrompts(BuiltinPrompts(), kind, foldText(query))
	out = append(out, filterPrompts(mine, kind, foldText(query))...)
	sortPrompts(out)
	if out == nil {
		out = []domain.ImagePrompt{}
	}
	return out, nil
}

// PromptUsage gộp template đang dùng từng prompt bằng một lượt đọc templates
// (bảng dùng của Prompt ảnh; UI hiện "Đang dùng ở N template").
func (s *Service) PromptUsage(ctx context.Context) (map[string][]TemplateRef, error) {
	rows, err := s.store.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]TemplateRef)
	add := func(promptID string, ref TemplateRef) {
		if promptID == "" {
			return
		}
		out[promptID] = append(out[promptID], ref)
	}
	for _, row := range rows {
		ref := TemplateRef{ID: row.ID, Name: row.Name}
		add(row.Recipe.Scenes.ScenePromptID, ref)
		add(row.Recipe.Scenes.CoverPromptID, ref)
	}
	return out, nil
}

// GetPrompt trả một prompt (có sẵn hoặc của bạn).
func (s *Service) GetPrompt(ctx context.Context, id string) (*domain.ImagePrompt, error) {
	p, err := s.promptByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// promptByID tra prompt có sẵn trước, rồi hàng trong DB.
func (s *Service) promptByID(ctx context.Context, id string) (*domain.ImagePrompt, error) {
	if p, ok := BuiltinPrompt(id); ok {
		return &p, nil
	}
	p, err := s.store.GetPrompt(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, notFound("Chỉ dẫn ảnh")
	}
	return p, nil
}

// CreatePrompt tạo mới hoặc nhân bản (fromId). Trùng tên là 409.
func (s *Service) CreatePrompt(ctx context.Context, in PromptInput) (*domain.ImagePrompt, error) {
	var src *domain.ImagePrompt
	if strings.TrimSpace(in.FromID) != "" {
		found, err := s.promptByID(ctx, in.FromID)
		if errors.Is(err, ErrNotFound) || CodeOf(err) == CodeNotFound {
			return nil, invalidFields("Chỉ dẫn nguồn không tồn tại", []FieldError{{"fromId", "Chỉ dẫn để nhân bản không tồn tại", "Chọn lại chỉ dẫn trong Thư viện"}})
		}
		if err != nil {
			return nil, err
		}
		src = found
	}
	kind := in.Kind
	if kind == "" && src != nil {
		kind = src.Kind
	}
	composition := in.Composition
	if composition == "" && src != nil {
		composition = src.Composition
	}
	prompt := ""
	switch {
	case in.Prompt != nil:
		prompt = *in.Prompt
	case src != nil:
		prompt = src.Prompt
	}
	name, err := s.validPromptName(ctx, in.Name, "")
	if err != nil {
		return nil, err
	}
	if fields := validatePromptFields(kind, composition, prompt); len(fields) > 0 {
		return nil, invalidFields("Chỉ dẫn ảnh không hợp lệ", fields)
	}
	now := s.now().UTC()
	p := domain.ImagePrompt{
		ID: s.newID(), Name: name, Kind: kind, Composition: composition,
		Prompt: strings.TrimSpace(prompt), UpdatedAt: now,
	}
	if err := s.store.InsertPrompt(ctx, p); err != nil {
		if isUniqueViolation(err) {
			return nil, nameTakenError(name)
		}
		return nil, err
	}
	s.emit(TopicPrompt, p.ID)
	return &p, nil
}

// PatchPrompt sửa prompt của bạn; prompt có sẵn trả 403.
func (s *Service) PatchPrompt(ctx context.Context, id string, in PromptPatch) (*domain.ImagePrompt, error) {
	if IsBuiltinPromptID(id) {
		return nil, builtinPromptError()
	}
	current, err := s.promptByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Kind != nil && *in.Kind != current.Kind {
		using, err := s.store.TemplatesUsingPrompt(ctx, id)
		if err != nil {
			return nil, err
		}
		if len(using) > 0 {
			return nil, inUsePromptError("Không đổi được loại chỉ dẫn khi mẫu video còn dùng.", using)
		}
		current.Kind = *in.Kind
	}
	if in.Composition != nil {
		current.Composition = *in.Composition
	}
	if in.Prompt != nil {
		current.Prompt = strings.TrimSpace(*in.Prompt)
	}
	if in.Name != nil {
		name, err := s.validPromptName(ctx, *in.Name, id)
		if err != nil {
			return nil, err
		}
		current.Name = name
	}
	if fields := validatePromptFields(current.Kind, current.Composition, current.Prompt); len(fields) > 0 {
		return nil, invalidFields("Chỉ dẫn ảnh không hợp lệ", fields)
	}
	current.UpdatedAt = s.now().UTC()
	if err := s.store.UpdatePrompt(ctx, *current); err != nil {
		if isUniqueViolation(err) {
			return nil, nameTakenError(current.Name)
		}
		if errors.Is(err, ErrInUse) {
			return nil, s.promptInUseError(ctx, id)
		}
		return nil, err
	}
	s.emit(TopicPrompt, id)
	return current, nil
}

// DeletePrompt xoá prompt của bạn; prompt có sẵn 403, đang dùng 409.
func (s *Service) DeletePrompt(ctx context.Context, id string) error {
	if IsBuiltinPromptID(id) {
		return builtinPromptError()
	}
	if _, err := s.promptByID(ctx, id); err != nil {
		return err
	}
	using, err := s.store.TemplatesUsingPrompt(ctx, id)
	if err != nil {
		return err
	}
	if len(using) > 0 {
		names := make([]string, 0, len(using))
		ids := make([]string, 0, len(using))
		for _, ref := range using {
			names = append(names, ref.Name)
			ids = append(ids, ref.ID)
		}
		return &Error{
			Code:     CodeInUse,
			Message:  "Đang dùng ở " + strings.Join(names, ", ") + ". Đổi chỉ dẫn ở mẫu video đó trước rồi xoá.",
			Recovery: "Bỏ chỉ dẫn khỏi các mẫu video này rồi xoá lại",
			UsedBy:   ids,
		}
	}
	if s.promptSamples != nil {
		s.promptSamples.CancelPrompt(ctx, id)
	}
	// Xoá hàng prompt trong CÙNG transaction dọn ảnh mẫu: image_prompt_samples
	// không có FK (prompt có sẵn không nằm trong image_prompts), nên xoá prompt
	// trước rồi dọn sau mà lỗi thì hàng mẫu mồ côi vĩnh viễn. Lỗi ở đây
	// rollback, prompt còn nguyên để người dùng thử lại.
	deletePrompt := func(txctx context.Context) error { return s.store.DeletePrompt(txctx, id) }
	if s.trialFiles != nil {
		err = s.trialFiles.DropPromptSamples(ctx, id, deletePrompt)
	} else {
		err = deletePrompt(ctx)
	}
	if err != nil {
		if errors.Is(err, ErrInUse) {
			return s.promptInUseError(ctx, id)
		}
		return err
	}
	s.emit(TopicPrompt, id)
	return nil
}

// promptInUseError đọc lại template đang dùng và dựng lỗi 409 in_use.
func (s *Service) promptInUseError(ctx context.Context, id string) error {
	using, err := s.store.TemplatesUsingPrompt(ctx, id)
	if err != nil || len(using) == 0 {
		return &Error{Code: CodeInUse, Message: "Chỉ dẫn đang được mẫu video dùng. Đổi chỉ dẫn ở mẫu video đó trước rồi thử lại.", Recovery: "Bỏ chỉ dẫn khỏi mẫu video rồi thử lại"}
	}
	names := make([]string, 0, len(using))
	ids := make([]string, 0, len(using))
	for _, ref := range using {
		names = append(names, ref.Name)
		ids = append(ids, ref.ID)
	}
	return &Error{
		Code:     CodeInUse,
		Message:  "Đang dùng ở " + strings.Join(names, ", ") + ". Đổi chỉ dẫn ở mẫu video đó trước rồi thử lại.",
		Recovery: "Bỏ chỉ dẫn khỏi các mẫu video này rồi thử lại",
		UsedBy:   ids,
	}
}

// validPromptName kiểm tên prompt: 1–60 ký tự, không trùng prompt có sẵn hay
// prompt khác.
func (s *Service) validPromptName(ctx context.Context, name, exceptID string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", invalidFields("Tên chỉ dẫn đang trống", []FieldError{{"name", "Tên chỉ dẫn đang trống", "Nhập tên chỉ dẫn"}})
	}
	if len([]rune(trimmed)) > domain.LimitPromptNameMaxChars {
		return "", invalidFields("Tên chỉ dẫn quá dài", []FieldError{{"name", "Tên chỉ dẫn quá dài", fmt.Sprintf("Rút gọn còn tối đa %d ký tự", domain.LimitPromptNameMaxChars)}})
	}
	if builtinNameTaken(trimmed) {
		return "", nameTakenError(trimmed)
	}
	taken, err := s.store.NameTaken(ctx, trimmed, exceptID)
	if err != nil {
		return "", err
	}
	if taken {
		return "", nameTakenError(trimmed)
	}
	return trimmed, nil
}

// nameTakenError dựng lỗi 409 trùng tên prompt.
func nameTakenError(name string) *Error {
	return &Error{
		Code:     CodeConflict,
		Message:  "Đã có chỉ dẫn tên \"" + name + "\"",
		Recovery: "Đặt tên khác, hoặc nhân bản chỉ dẫn có sẵn rồi sửa",
	}
}

// builtinPromptError dựng lỗi 403 cho prompt có sẵn.
func builtinPromptError() *Error {
	return &Error{
		Code:     CodeBuiltin,
		Message:  "Chỉ dẫn có sẵn của Viderse không sửa và không xoá được",
		Recovery: "Nhân bản để có bản của bạn rồi sửa",
	}
}

// inUsePromptError dựng lỗi 409 in_use kèm template đang dùng.
func inUsePromptError(msg string, using []TemplateRef) *Error {
	ids := make([]string, 0, len(using))
	names := make([]string, 0, len(using))
	for _, ref := range using {
		ids = append(ids, ref.ID)
		names = append(names, ref.Name)
	}
	return &Error{
		Code:     CodeInUse,
		Message:  msg + " Đang dùng ở " + strings.Join(names, ", ") + ".",
		Recovery: "Bỏ chọn chỉ dẫn ở các mẫu video này rồi thử lại",
		UsedBy:   ids,
	}
}

// validatePromptFields kiểm loại/bố cục/nội dung prompt.
func validatePromptFields(kind domain.PromptKind, composition, prompt string) []FieldError {
	fields := make([]FieldError, 0, 3)
	switch kind {
	case domain.PromptScene:
		if composition != "" {
			fields = append(fields, FieldError{"composition", "Chỉ dẫn ảnh cảnh không có bố cục bìa", "Bỏ trống bố cục với chỉ dẫn ảnh cảnh"})
		}
	case domain.PromptCover:
		if composition == "" {
			fields = append(fields, FieldError{"composition", "Chỉ dẫn ảnh bìa phải chọn bố cục", "Chọn một trong: " + strings.Join(domain.ThumbnailStyles(), ", ")})
		} else if !contains(domain.ThumbnailStyles(), composition) {
			fields = append(fields, FieldError{"composition", "Bố cục bìa không hợp lệ", "Chọn một trong: " + strings.Join(domain.ThumbnailStyles(), ", ")})
		}
	default:
		fields = append(fields, FieldError{"kind", "Loại chỉ dẫn không hợp lệ", "Chọn Ảnh cảnh hoặc Ảnh bìa"})
	}
	if len([]rune(prompt)) > domain.LimitImagePromptMaxChars {
		fields = append(fields, FieldError{"prompt", "Chỉ dẫn quá dài", fmt.Sprintf("Rút gọn còn tối đa %d ký tự", domain.LimitImagePromptMaxChars)})
	}
	return fields
}

// ── xuất / nhập tệp .json ──────────────────────────────────────────────────

// PromptExportFormat là dấu nhận dạng tệp prompt.
const PromptExportFormat = "viderse-image-prompt"

// PromptExport là hình tệp xuất (không có usedBy; samples để trống).
type PromptExport struct {
	Format      string `json:"format"`
	Version     int    `json:"version"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Composition string `json:"composition,omitempty"`
	Prompt      string `json:"prompt"`
	ExportedAt  string `json:"exportedAt"`
}

// ExportPrompt trả tệp .json của một prompt (có sẵn cũng xuất được).
func (s *Service) ExportPrompt(ctx context.Context, id string) (*PromptExport, error) {
	p, err := s.promptByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &PromptExport{
		Format: PromptExportFormat, Version: 1, Name: p.Name, Kind: p.Kind,
		Composition: p.Composition, Prompt: p.Prompt,
		ExportedAt: s.now().UTC().Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// ImportPrompt nhập tệp .json thành prompt mới (id mới); trùng tên là 409.
func (s *Service) ImportPrompt(ctx context.Context, data []byte) (*domain.ImagePrompt, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var in PromptExport
	notPromptFile := invalidInput("Tệp không phải chỉ dẫn của Viderse", "Chọn tệp .json xuất từ Thư viện › Chỉ dẫn ảnh")
	if err := dec.Decode(&in); err != nil || in.Format != PromptExportFormat {
		return nil, notPromptFile
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, notPromptFile
	}
	if in.Version != 1 {
		return nil, invalidInput("Tệp chỉ dẫn thuộc phiên bản khác", "Xuất lại từ phiên bản hiện tại rồi nhập")
	}
	prompt := in.Prompt
	created, err := s.CreatePrompt(ctx, PromptInput{
		Name: in.Name, Kind: in.Kind, Composition: in.Composition, Prompt: &prompt,
	})
	if CodeOf(err) != CodeConflict {
		return created, err
	}
	// Trùng tên là chuyện thường khi nhập cùng tệp hai lần: thêm hậu tố
	// "(nhập)" rồi "(nhập 2)", "(nhập 3)"… thay vì chặn người dùng.
	for attempt := 1; attempt <= 99; attempt++ {
		suffix := " (nhập)"
		if attempt > 1 {
			suffix = fmt.Sprintf(" (nhập %d)", attempt)
		}
		name := truncateRunes(strings.TrimSpace(in.Name), domain.LimitPromptNameMaxChars-len([]rune(suffix))) + suffix
		created, err = s.CreatePrompt(ctx, PromptInput{
			Name: name, Kind: in.Kind, Composition: in.Composition, Prompt: &prompt,
		})
		if CodeOf(err) != CodeConflict {
			return created, err
		}
	}
	return nil, err
}
