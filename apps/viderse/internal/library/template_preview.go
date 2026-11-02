package library

// Hợp đồng bản xem thử template: bản xem thử là một project editor (bản sao
// project template, đổ nội dung ảnh/âm thanh/phụ đề của một video đã xong),
// không phải video dựng sẵn. Nghiệp vụ dựng nằm ở internal/preview; Service
// Thư viện chỉ giữ trạng thái, nguồn và uỷ quyền để route không phụ thuộc
// package editor/pipeline.

import (
	"context"
	"errors"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// PreviewBuilding là trạng thái chỉ tồn tại khi có việc nền đang chạy.
const PreviewBuilding = "building"

// TemplatePreviewRecord là trạng thái bền của bản xem thử sau khi project đã
// commit. TemplateRevision là revision đọc lúc bắt đầu dựng: sửa template giữa
// chừng để lại revision cũ nên bản xem thử thành cũ, không giả làm mới.
// RunID là lượt dựng đã đăng ký lúc bắt đầu; ghi nhận chỉ thành công khi cột
// preview_run_id vẫn là RunID, nên lượt cũ không thể công bố đè lượt mới.
type TemplatePreviewRecord struct {
	ProjectID        string
	JobID            string
	Minutes          int
	TemplateRevision int64
	BuiltAt          time.Time
	RunID            string
}

// PreviewError là lần dựng hỏng gần nhất (cùng shape với domain.PreviewError).
type PreviewError = domain.PreviewError

// PreviewSource là một video đã xong có thể dùng làm nguồn xem thử.
// Disabled/Reason nói rõ video không dựng được với recipe hiện tại (ví dụ
// video đã chọn "không vẽ ảnh" trong khi template cần ảnh cảnh) thay vì ẩn
// video hoặc để người dùng gặp lỗi sau khi bấm dựng.
type PreviewSource struct {
	JobID       string  `json:"jobId"`
	Title       string  `json:"title"`
	ChannelName string  `json:"channelName,omitempty"`
	DurationSec float64 `json:"durationSec"`
	Disabled    bool    `json:"disabled,omitempty"`
	Reason      string  `json:"reason,omitempty"`
}

// PreviewRequest là body POST /api/templates/{id}/preview.
type PreviewRequest struct {
	JobID   string `json:"jobId,omitempty"`
	Minutes int    `json:"minutes,omitempty"`
}

// PreviewTask là dòng việc nền đang dựng bản xem thử.
type PreviewTask struct {
	ID         string `json:"id"`
	Phase      string `json:"phase,omitempty"`
	State      string `json:"state"`
	Done       int64  `json:"done"`
	Total      int64  `json:"total"`
	ActionHref string `json:"actionHref,omitempty"`
}

// PreviewStatus là GET /api/templates/{id}/preview.
type PreviewStatus struct {
	State            string       `json:"state"`
	BuiltAt          *time.Time   `json:"builtAt,omitempty"`
	ProjectID        string       `json:"projectId,omitempty"`
	JobID            string       `json:"jobId,omitempty"`
	Minutes          int          `json:"minutes,omitempty"`
	TemplateRevision int64        `json:"templateRevision,omitempty"`
	Task             *PreviewTask `json:"task,omitempty"`
	// Error là lỗi dựng gần nhất; bản xem thử cũ (ProjectID) vẫn giữ nguyên.
	Error   *PreviewError   `json:"error,omitempty"`
	Sources []PreviewSource `json:"sources"`
}

// PreviewBuilder là nghiệp vụ dựng bản xem thử (internal/preview). nil ở CLI
// hoặc test chỉ đọc metadata.
type PreviewBuilder interface {
	Sources(ctx context.Context) ([]PreviewSource, error)
	Status(ctx context.Context, templateID string) (*PreviewStatus, error)
	Start(ctx context.Context, templateID string, req PreviewRequest) (*PreviewStatus, error)
	Cancel(ctx context.Context, templateID string) error
	// Drop xoá project xem thử theo mã project (template có thể đã bị xoá).
	Drop(ctx context.Context, projectID string) error
}

// PreviewStore ghi trạng thái bản xem thử; postgres.LibraryStore hiện thực.
type PreviewStore interface {
	// BeginTemplatePreview nhận một lượt dựng: ghi token lượt dựng và xoá lỗi
	// của lượt trước. Lỗi ErrNotFound khi template đã bị xoá.
	BeginTemplatePreview(ctx context.Context, id, runID string) error
	// SetTemplatePreview ghi bản đã dựng xong với CAS preview_run_id: chỉ lượt
	// đang giữ token được công bố. Trả project xem thử trước đó (nếu khác) để
	// bên gọi dọn sau khi đã công bố; ErrPreviewSuperseded khi lượt đã cũ.
	SetTemplatePreview(ctx context.Context, id string, preview TemplatePreviewRecord) (previous string, err error)
	// SetTemplatePreviewFailure ghi lỗi có hành động của lượt dựng và nhả token
	// trong cùng một câu lệnh; bản xem thử cũ không đổi.
	SetTemplatePreviewFailure(ctx context.Context, id, runID string, failure *PreviewError) error
	// ClearTemplatePreviewRun nhả token nếu vẫn là của runID (huỷ/không công bố).
	ClearTemplatePreviewRun(ctx context.Context, id, runID string) error
	// RegisterTemplatePreviewProject ghi danh mục project xem thử TRƯỚC khi tạo
	// project, để project vừa tạo đã bị ẩn và còn dấu để dọn sau crash.
	RegisterTemplatePreviewProject(ctx context.Context, id, projectID, jobID string, minutes int) error
	// PreviewProjectIDs trả mọi project xem thử từng tạo (không phụ thuộc con
	// trỏ hiện tại) để danh sách project editor ẩn chúng.
	PreviewProjectIDs(ctx context.Context) ([]string, error)
	PreviewProjectIDsForTemplate(ctx context.Context, id string) ([]string, error)
	ForgetPreviewProject(ctx context.Context, projectID string) error
	// PreviewProjectRows trả danh mục kèm project đang là con trỏ hiện tại để
	// lần khởi động dọn project mồ côi.
	PreviewProjectRows(ctx context.Context) ([]PreviewProjectRow, error)
}

// PreviewProjectRow là một dòng danh mục project xem thử.
type PreviewProjectRow struct {
	ProjectID  string
	TemplateID string // rỗng khi template đã bị xoá
	Current    bool   // template đang trỏ tới project này
}

// ErrPreviewSuperseded: lượt dựng đã mất token (bị huỷ hoặc có lượt mới).
var ErrPreviewSuperseded = errors.New("lượt dựng bản xem thử đã bị thay")

// BindPreviewBuilder gắn nghiệp vụ dựng bản xem thử sau khi editor/pipeline
// đã dựng (serve dựng Thư viện trước chúng).
func (s *Service) BindPreviewBuilder(builder PreviewBuilder) {
	s.preview = builder
	if store, ok := s.store.(PreviewStore); ok {
		s.previewStore = store
	}
}

// RecordTemplatePreview ghi bản xem thử đã dựng xong. Builder gọi sau khi
// project commit; lỗi ở đây để project mới không được nhận. Trả project cũ
// (nếu có) để builder dọn sau khi công bố thành công.
func (s *Service) RecordTemplatePreview(ctx context.Context, id string, record TemplatePreviewRecord) (string, error) {
	if s.previewStore == nil {
		return "", invalidInput("Máy chủ chưa sẵn sàng ghi bản xem thử", "Khởi động lại Viderse rồi thử lại")
	}
	return s.previewStore.SetTemplatePreview(ctx, id, record)
}

// BeginTemplatePreview nhận lượt dựng mới và xoá lỗi của lượt trước.
func (s *Service) BeginTemplatePreview(ctx context.Context, id, runID string) error {
	if s.previewStore == nil {
		return invalidInput("Máy chủ chưa sẵn sàng dựng bản xem thử", "Khởi động lại Viderse rồi thử lại")
	}
	return s.previewStore.BeginTemplatePreview(ctx, id, runID)
}

// FailTemplatePreview ghi lỗi có hành động của lượt dựng.
func (s *Service) FailTemplatePreview(ctx context.Context, id, runID string, failure *PreviewError) error {
	if s.previewStore == nil {
		return nil
	}
	return s.previewStore.SetTemplatePreviewFailure(ctx, id, runID, failure)
}

// ClearTemplatePreviewRun nhả token của lượt dựng đã huỷ.
func (s *Service) ClearTemplatePreviewRun(ctx context.Context, id, runID string) error {
	if s.previewStore == nil {
		return nil
	}
	return s.previewStore.ClearTemplatePreviewRun(ctx, id, runID)
}

// RegisterTemplatePreviewProject ghi danh mục project xem thử trước khi tạo.
func (s *Service) RegisterTemplatePreviewProject(ctx context.Context, id, projectID, jobID string, minutes int) error {
	if s.previewStore == nil {
		return invalidInput("Máy chủ chưa sẵn sàng dựng bản xem thử", "Khởi động lại Viderse rồi thử lại")
	}
	return s.previewStore.RegisterTemplatePreviewProject(ctx, id, projectID, jobID, minutes)
}

// PreviewProjectIDsForTemplate trả project xem thử của một template.
func (s *Service) PreviewProjectIDsForTemplate(ctx context.Context, id string) ([]string, error) {
	if s.previewStore == nil {
		return []string{}, nil
	}
	return s.previewStore.PreviewProjectIDsForTemplate(ctx, id)
}

// ForgetPreviewProject bỏ dấu một project xem thử đã xoá xong.
func (s *Service) ForgetPreviewProject(ctx context.Context, projectID string) error {
	if s.previewStore == nil || projectID == "" {
		return nil
	}
	return s.previewStore.ForgetPreviewProject(ctx, projectID)
}

// PreviewProjectRows trả danh mục project xem thử cho lần dọn khởi động.
func (s *Service) PreviewProjectRows(ctx context.Context) ([]PreviewProjectRow, error) {
	if s.previewStore == nil {
		return []PreviewProjectRow{}, nil
	}
	return s.previewStore.PreviewProjectRows(ctx)
}

// PreviewSources trả video đã xong có thể làm nguồn xem thử.
func (s *Service) PreviewSources(ctx context.Context) ([]PreviewSource, error) {
	if s.preview == nil {
		return []PreviewSource{}, nil
	}
	return s.preview.Sources(ctx)
}

// PreviewStatus trả trạng thái bản xem thử; khi builder vắng, chỉ suy từ cột
// preview_revision đã có.
func (s *Service) PreviewStatus(ctx context.Context, id string) (*PreviewStatus, error) {
	if s.preview != nil {
		return s.preview.Status(ctx, id)
	}
	tpl, err := s.store.GetTemplate(ctx, id)
	if err != nil {
		if isErrNotFound(err) {
			return nil, notFound("Mẫu video")
		}
		return nil, err
	}
	if tpl == nil {
		return nil, notFound("Mẫu video")
	}
	return &PreviewStatus{
		State: tpl.PreviewState(), BuiltAt: tpl.PreviewBuiltAt,
		ProjectID: tpl.PreviewProjectID, JobID: tpl.PreviewJobID,
		Minutes: tpl.PreviewMinutes, Error: tpl.PreviewError, Sources: []PreviewSource{},
	}, nil
}

// StartTemplatePreview dựng bản xem thử trong nền và trả trạng thái đang dựng.
func (s *Service) StartTemplatePreview(ctx context.Context, id string, req PreviewRequest) (*PreviewStatus, error) {
	if s.preview == nil {
		return nil, invalidInput("Máy chủ chưa sẵn sàng dựng bản xem thử", "Khởi động lại Viderse rồi thử lại")
	}
	return s.preview.Start(ctx, id, req)
}

// CancelTemplatePreview huỷ lần dựng đang chạy (không xoá bản đã có).
func (s *Service) CancelTemplatePreview(ctx context.Context, id string) error {
	if s.preview == nil {
		return nil
	}
	return s.preview.Cancel(ctx, id)
}

// DropTemplatePreview dọn project xem thử theo mã project (best effort; hàng
// template có thể đã bị xoá trước đó).
func (s *Service) DropTemplatePreview(ctx context.Context, projectID string) error {
	if s.preview == nil || projectID == "" {
		return nil
	}
	return s.preview.Drop(ctx, projectID)
}

// PreviewEditorProjectIDs trả các project xem thử để danh sách project editor
// không hiện chúng. Rỗng khi store không hỗ trợ (CLI/test metadata).
func (s *Service) PreviewEditorProjectIDs(ctx context.Context) (map[string]bool, error) {
	store, ok := s.store.(interface {
		PreviewProjectIDs(context.Context) ([]string, error)
	})
	if !ok {
		return map[string]bool{}, nil
	}
	ids, err := store.PreviewProjectIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

var _ = domain.PreviewNone
