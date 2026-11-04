package library

import (
	"context"
	"time"
)

// Vẽ thử ảnh (Template.dc.html › lib^cover "Vẽ thử"; Library.dc.html ›
// view=editor ảnh mẫu). Mỗi lần vẽ là MỘT ảnh thật, tốn đúng một lượt hạn mức
// của provider đang cấu hình; lần vẽ chạy nền nên đóng trang vẫn xong. Hai phạm
// vi dùng chung một đường vẽ:
//   - template: ảnh bìa thử theo prompt ảnh bìa đang chọn, giữ 12 lần gần nhất;
//   - prompt:   ảnh mẫu theo prompt đang mở, cũng giữ 12 lần gần nhất.
// Nguồn là video đã làm xong; chỉ ĐỌC artifact của job, không sửa job/editor.

// Trạng thái một lần vẽ thử.
const (
	TrialRunning   = "running"
	TrialReady     = "ready"
	TrialFailed    = "failed"
	TrialCancelled = "cancelled"
)

// TrialError là lỗi đã phân loại của một lần vẽ thử (không lộ khoá/provider
// secret; Message/Recovery lấy từ DrawError đã sanitize).
type TrialError struct {
	Code     string     `json:"code"`
	Message  string     `json:"message"`
	Recovery string     `json:"recovery,omitempty"`
	RetryAt  *time.Time `json:"retryAt,omitempty"`
}

// TrialQuota là hạn mức ảnh thật của provider đang cấu hình.
type TrialQuota struct {
	Provider string     `json:"provider"`
	Label    string     `json:"label"`
	Paid     bool       `json:"paid"`
	Left     int        `json:"left"`
	Total    int        `json:"total"`
	ResetAt  *time.Time `json:"resetAt,omitempty"`
}

// TrialCost là giá của đúng một lần vẽ thử. Images luôn 1 khi có ảnh trả phí;
// QuotaLeft = -1 khi provider không tính phí theo ảnh hoặc chưa đọc được.
type TrialCost struct {
	Images    int    `json:"images"`
	Provider  string `json:"provider"`
	Label     string `json:"label"`
	QuotaLeft int    `json:"quotaLeft"`
}

// TrialPrompt là prompt đang chọn mà lần vẽ thử sẽ dùng.
type TrialPrompt struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Composition string `json:"composition,omitempty"`
}

// TrialScene là một cảnh của video nguồn để chọn khi vẽ thử prompt cảnh.
type TrialScene struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// CoverTrial là một lần vẽ thử ảnh bìa của template.
type CoverTrial struct {
	ID          string      `json:"id"`
	TemplateID  string      `json:"-"`
	JobID       string      `json:"jobId"`
	JobTitle    string      `json:"jobTitle"`
	PromptID    string      `json:"promptId"`
	PromptName  string      `json:"promptName"`
	Composition string      `json:"composition"`
	State       string      `json:"state"`
	Provider    string      `json:"provider,omitempty"`
	Model       string      `json:"model,omitempty"`
	Width       int         `json:"width,omitempty"`
	Height      int         `json:"height,omitempty"`
	CostImages  int         `json:"costImages"`
	HookText    string      `json:"hookText,omitempty"`
	CreatedAt   time.Time   `json:"createdAt"`
	FinishedAt  *time.Time  `json:"finishedAt,omitempty"`
	Error       *TrialError `json:"error,omitempty"`
	MediaPath   string      `json:"-"`
}

// CoverTrialRequest là body POST /api/templates/{id}/cover-trials. Confirm
// phải true: máy chủ không vẽ khi UI chưa xác nhận đúng một ảnh.
type CoverTrialRequest struct {
	JobID   string `json:"jobId"`
	Confirm bool   `json:"confirm"`
}

// CoverTrialStatus là GET /api/templates/{id}/cover-trials. Ready=false chỉ
// khi thiếu đúng một điều kiện thật (chưa có video xong, thiếu prompt, provider
// chưa cấu hình, hết hạn mức); Blocked nói lý do đó, không khoá cả tính năng.
type CoverTrialStatus struct {
	Sources []PreviewSource `json:"sources"`
	History []CoverTrial    `json:"history"`
	Task    *PreviewTask    `json:"task,omitempty"`
	Quota   *TrialQuota     `json:"quota,omitempty"`
	Cost    TrialCost       `json:"cost"`
	Prompt  *TrialPrompt    `json:"prompt,omitempty"`
	Ready   bool            `json:"ready"`
	Blocked string          `json:"blockedReason,omitempty"`
}

// PromptSample là một lần vẽ thử ảnh mẫu của prompt.
type PromptSample struct {
	ID          string      `json:"id"`
	PromptID    string      `json:"promptId"`
	PromptName  string      `json:"promptName"`
	PromptKind  string      `json:"promptKind"`
	Composition string      `json:"composition,omitempty"`
	VideoID     string      `json:"videoId"`
	VideoTitle  string      `json:"videoTitle"`
	SceneID     string      `json:"sceneId,omitempty"`
	HookText    string      `json:"hookText,omitempty"`
	State       string      `json:"state"`
	Provider    string      `json:"provider,omitempty"`
	Model       string      `json:"model,omitempty"`
	Width       int         `json:"width,omitempty"`
	Height      int         `json:"height,omitempty"`
	CostImages  int         `json:"costImages"`
	CreatedAt   time.Time   `json:"createdAt"`
	FinishedAt  *time.Time  `json:"finishedAt,omitempty"`
	Error       *TrialError `json:"error,omitempty"`
	MediaPath   string      `json:"-"`
}

// PromptSampleRequest là body POST /api/image-prompts/{id}/samples.
type PromptSampleRequest struct {
	VideoID string `json:"videoId"`
	SceneID string `json:"sceneId,omitempty"`
	Confirm bool   `json:"confirm"`
}

// PromptSampleStatus là GET /api/image-prompts/{id}/samples.
type PromptSampleStatus struct {
	Sources []PreviewSource `json:"sources"`
	History []PromptSample  `json:"history"`
	Task    *PreviewTask    `json:"task,omitempty"`
	Quota   *TrialQuota     `json:"quota,omitempty"`
	Cost    TrialCost       `json:"cost"`
	Prompt  *TrialPrompt    `json:"prompt,omitempty"`
	Ready   bool            `json:"ready"`
	Blocked string          `json:"blockedReason,omitempty"`
}

// TrialStore là chỗ lưu lịch sử vẽ thử (migration 00037). Các hàm Insert/Update
// được gọi trong transaction của media.Publish (qua ctx) để hàng lịch sử và
// tham chiếu media cùng sống hoặc cùng chết.
type TrialStore interface {
	ListCoverTrials(ctx context.Context, templateID string, limit int) ([]CoverTrial, error)
	GetCoverTrial(ctx context.Context, templateID, id string) (*CoverTrial, error)
	InsertCoverTrial(ctx context.Context, trial CoverTrial) error
	UpdateCoverTrial(ctx context.Context, trial CoverTrial) error
	CoverTrialsBeyond(ctx context.Context, templateID string, keep int) ([]CoverTrial, error)
	DeleteCoverTrials(ctx context.Context, ids []string) error
	CoverTrialMediaPaths(ctx context.Context, templateID string) ([]string, error)
	FailRunningCoverTrials(ctx context.Context, at time.Time, message, recovery string) (int64, error)

	ListPromptSamples(ctx context.Context, promptID string, limit int) ([]PromptSample, error)
	GetPromptSample(ctx context.Context, promptID, id string) (*PromptSample, error)
	InsertPromptSample(ctx context.Context, sample PromptSample) error
	UpdatePromptSample(ctx context.Context, sample PromptSample) error
	PromptSamplesBeyond(ctx context.Context, promptID string, keep int) ([]PromptSample, error)
	DeletePromptSamples(ctx context.Context, ids []string) error
	PromptSampleMediaPaths(ctx context.Context, promptID string) ([]string, error)
	FailRunningPromptSamples(ctx context.Context, at time.Time, message, recovery string) (int64, error)
	ListPromptSampleIDs(ctx context.Context, promptIDs []string) (map[string][]string, error)
}

// TrialFiles dọn media của lịch sử vẽ thử khi template/prompt bị xoá. Gắn ở
// cmd vì chỉ ở đó mới có media.Service; nil thì template/prompt vẫn xoá được
// (media giữ tham chiếu, không bị GC quét nhầm).
//
// Hai hàm nhận commit chạy commit đó TRONG cùng transaction nhả tham chiếu
// media, nên hàng cha (và cascade cover_trials) không thể biến mất trước khi
// media được nhả — hoặc ngược lại. Lỗi trả về nghĩa là chưa commit gì: hàng +
// tham chiếu còn nguyên để người dùng thử lại.
type TrialFiles interface {
	// PathsForTemplate đọc media_path của lịch sử vẽ thử template. Lỗi phải
	// chặn xoá: chưa biết phải nhả gì thì xoá sẽ bỏ lại blob đã bind mà không
	// còn hàng nào để thử lại.
	PathsForTemplate(ctx context.Context, templateID string) ([]string, error)
	// UnbindFiles nhả paths và chạy commit trong cùng transaction (commit xoá
	// hàng sở hữu path nên chạy trước vòng nhả binding). commit phải chạy đúng
	// một lần kể cả khi paths rỗng; có paths mà thiếu media thì trả lỗi thay vì
	// xoá hàng cha.
	UnbindFiles(ctx context.Context, paths []string, commit func(context.Context) error) error
	// DropPromptSamples xoá hàng ảnh mẫu của prompt, nhả media_path của chúng
	// rồi chạy commit trong cùng transaction. promptID còn nguyên khi lỗi.
	DropPromptSamples(ctx context.Context, promptID string, commit func(context.Context) error) error
}

// CoverTrialBuilder là phần nghiệp vụ template vẽ thử do package covertrial
// cài; Thư viện chỉ giữ hợp đồng để web/CLI không phụ thuộc vòng.
type CoverTrialBuilder interface {
	Status(ctx context.Context, templateID string) (*CoverTrialStatus, error)
	Start(ctx context.Context, templateID string, req CoverTrialRequest) (*CoverTrialStatus, error)
	Cancel(ctx context.Context, templateID, trialID string) error
	ImagePath(ctx context.Context, templateID, trialID string) (string, error)
	CancelTemplate(ctx context.Context, templateID string)
}

// PromptSampleBuilder là phần nghiệp vụ prompt vẽ thử ảnh mẫu.
type PromptSampleBuilder interface {
	Status(ctx context.Context, promptID string) (*PromptSampleStatus, error)
	Start(ctx context.Context, promptID string, req PromptSampleRequest) (*PromptSampleStatus, error)
	Cancel(ctx context.Context, promptID, sampleID string) error
	CancelPrompt(ctx context.Context, promptID string)
	ImagePath(ctx context.Context, promptID, sampleID string) (string, error)
	Scenes(ctx context.Context, promptID, videoID string) ([]TrialScene, error)
	SampleIDs(ctx context.Context, promptIDs []string) (map[string][]string, error)
}

// BindCoverTrial gắn phần vẽ thử template (cmd gọi sau khi dựng Service).
func (s *Service) BindCoverTrial(builder CoverTrialBuilder) { s.coverTrials = builder }

// BindPromptSamples gắn phần vẽ thử prompt ảnh mẫu.
func (s *Service) BindPromptSamples(builder PromptSampleBuilder) { s.promptSamples = builder }

// BindTrialFiles gắn phần dọn media của lịch sử vẽ thử.
func (s *Service) BindTrialFiles(files TrialFiles) { s.trialFiles = files }

func trialNotBound() error {
	return &Error{
		Code:     CodeToolMissing,
		Message:  "Chưa sẵn sàng vẽ thử ảnh",
		Recovery: "Khởi động lại ứng dụng rồi thử lại",
	}
}

// CoverTrialStatus trả nguồn video, 12 lần gần nhất, hạn mức và lý do chặn.
func (s *Service) CoverTrialStatus(ctx context.Context, templateID string) (*CoverTrialStatus, error) {
	if s.coverTrials == nil {
		return nil, trialNotBound()
	}
	return s.coverTrials.Status(ctx, templateID)
}

// StartCoverTrial xếp một lần vẽ thử ảnh bìa chạy nền.
func (s *Service) StartCoverTrial(ctx context.Context, templateID string, req CoverTrialRequest) (*CoverTrialStatus, error) {
	if s.coverTrials == nil {
		return nil, trialNotBound()
	}
	return s.coverTrials.Start(ctx, templateID, req)
}

// CancelCoverTrial huỷ lần vẽ đang chạy; chỉ huỷ khi người dùng bấm huỷ.
func (s *Service) CancelCoverTrial(ctx context.Context, templateID, trialID string) error {
	if s.coverTrials == nil {
		return trialNotBound()
	}
	return s.coverTrials.Cancel(ctx, templateID, trialID)
}

// CoverTrialImagePath trả đường dẫn media của một lần vẽ đã xong.
func (s *Service) CoverTrialImagePath(ctx context.Context, templateID, trialID string) (string, error) {
	if s.coverTrials == nil {
		return "", trialNotBound()
	}
	return s.coverTrials.ImagePath(ctx, templateID, trialID)
}

// PromptSampleStatus trả nguồn, ảnh mẫu và hạn mức của prompt.
func (s *Service) PromptSampleStatus(ctx context.Context, promptID string) (*PromptSampleStatus, error) {
	if s.promptSamples == nil {
		return nil, trialNotBound()
	}
	return s.promptSamples.Status(ctx, promptID)
}

// StartPromptSample xếp một lần vẽ ảnh mẫu chạy nền.
func (s *Service) StartPromptSample(ctx context.Context, promptID string, req PromptSampleRequest) (*PromptSampleStatus, error) {
	if s.promptSamples == nil {
		return nil, trialNotBound()
	}
	return s.promptSamples.Start(ctx, promptID, req)
}

// CancelPromptSample huỷ lần vẽ ảnh mẫu đang chạy.
func (s *Service) CancelPromptSample(ctx context.Context, promptID, sampleID string) error {
	if s.promptSamples == nil {
		return trialNotBound()
	}
	return s.promptSamples.Cancel(ctx, promptID, sampleID)
}

// PromptSampleImagePath trả đường dẫn media của một ảnh mẫu đã xong.
func (s *Service) PromptSampleImagePath(ctx context.Context, promptID, sampleID string) (string, error) {
	if s.promptSamples == nil {
		return "", trialNotBound()
	}
	return s.promptSamples.ImagePath(ctx, promptID, sampleID)
}

// PromptScenes liệt kê cảnh của video nguồn để chọn khi vẽ thử prompt cảnh.
func (s *Service) PromptScenes(ctx context.Context, promptID, videoID string) ([]TrialScene, error) {
	if s.promptSamples == nil {
		return nil, trialNotBound()
	}
	return s.promptSamples.Scenes(ctx, promptID, videoID)
}

// PromptSampleIDs trả media id ảnh mẫu mới nhất của từng prompt cho wire
// ImagePrompt.samples (tối đa 12, mới nhất trước).
func (s *Service) PromptSampleIDs(ctx context.Context, promptIDs []string) map[string][]string {
	if s.promptSamples == nil || len(promptIDs) == 0 {
		return map[string][]string{}
	}
	out, err := s.promptSamples.SampleIDs(ctx, promptIDs)
	if err != nil {
		return map[string][]string{}
	}
	return out
}
