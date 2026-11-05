// Package channels là mảng Kênh của bạn (07 §8, §16): kênh do chủ sở hữu, đối
// thủ liên kết (dòng đầu là đối thủ tham chiếu nhận diện), template mặc định
// trong Thư viện, và bộ nhận diện do trợ lý tạo (tên + avatar + ảnh bìa kênh)
// với một run mỗi kênh, công bố nguyên tử và phục hồi sau sập.
//
// Mảng này không tự vẽ hay gọi model: interface AI (Director/Drawer) do
// cmd/viderse ghép vào, giống cách imagegen nhận provider. Store là interface
// phía người dùng — hiện thực Postgres ở internal/persistence/postgres.
package channels

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

// Mã lỗi ổn định của mảng kênh (trường "code" của body lỗi HTTP/CLI).
const (
	CodeInvalidInput = "invalid_input"
	CodeNotFound     = "not_found"
	CodeConflict     = "conflict"
	CodeInternal     = "internal_error"
	CodeQuota        = "quota"
	CodeCancelled    = "cancelled"
	CodeInterrupted  = "interrupted"
	CodeToolMissing  = "tool_missing"
)

// Error là lỗi nghiệp vụ của mảng kênh: mã ổn định, câu tiếng Việt cho người
// dùng, gợi ý sửa và mốc tự thử lại khi có (hạn mức ảnh).
type Error struct {
	Code     string
	Message  string
	Recovery string
	RetryAt  *time.Time
	// Field là tên trường lỗi kiểu `perDay` (400 có fields[]).
	Field string
}

func (e *Error) Error() string { return e.Message }

// CodeOf trích mã lỗi ổn định; lỗi lạ trả "" (web trả 500).
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func errInvalid(format string, args ...any) *Error {
	return &Error{Code: CodeInvalidInput, Message: fmt.Sprintf(format, args...)}
}

func errNotFound(format string, args ...any) *Error {
	return &Error{Code: CodeNotFound, Message: fmt.Sprintf(format, args...), Recovery: "Tải lại trang Kênh của bạn."}
}

func errConflict(format string, args ...any) *Error {
	return &Error{Code: CodeConflict, Message: fmt.Sprintf(format, args...), Recovery: "Chờ lượt đang chạy xong rồi thử lại."}
}

// Giới hạn đầu vào (cùng luật cho HTTP và CLI).
const (
	NameMaxRunes     = 200
	URLMaxRunes      = 2048
	StyleMaxRunes    = 8000
	FooterMaxRunes   = 1000
	AlwaysTagsMax    = 10
	AlwaysTagRunes   = 60
	// PerDay là nhịp video mỗi ngày (design #86: đồng nhất 1–20 PC/mobile).
	PerDayMin     = 1
	PerDayMax     = 20
	PerDayDefault = 5
	CompetitorsMax = 200
	BriefMaxRunes    = 2000
	DirectionMaxRune = 1000
)

// Field là một trường của patch: Set = client có gửi trường này (Value nil =
// gửi null tường minh cho trường nullable).
type Field[T any] struct {
	Set   bool
	Value T
}

// Patch là các trường Update được phép đổi; trường không Set giữ nguyên.
type Patch struct {
	Name              Field[string]
	YoutubeURL        Field[*string]
	Language          Field[string]
	StyleNote         Field[*string]
	DescriptionFooter Field[*string]
	AlwaysTags        Field[[]string]
	AutoFetch         Field[bool]
	AutoApprove       Field[bool]
	PerDay            Field[int]
	// DefaultTemplateID là template mặc định trong Thư viện; NOT NULL nên
	// service chặn null/"" trước khi vào store.
	DefaultTemplateID Field[string]
	// AgentInstructions là một khoá chỉ dẫn "Dặn trợ lý về kênh" cần gộp; giá
	// trị rỗng nghĩa là xoá khoá đó. Chỉ gộp đúng một trợ lý mỗi lần.
	AgentInstructions AgentInstructionPatch
	// Brief sửa Mô tả kênh trong branding của nhận diện (07 §16).
	Brief Field[string]
}

// AgentInstructionPatch là bản vá một trợ lý: Set + Key bắt buộc; Value rỗng =
// xoá, khác rỗng = đặt.
type AgentInstructionPatch struct {
	Set   bool
	Key   string
	Value string
}

// CreateInput là dữ liệu đã chuẩn hoá của một kênh mới.
type CreateInput struct {
	ID                string
	Name              string
	YoutubeURL        *string
	Language          string
	AutoFetch         bool
	AutoApprove       bool
	PerDay            int
	DefaultTemplateID string
	AgentInstructions domain.AgentInstructions
	CompetitorIDs     []string
}

// ChannelView là một kênh kèm đối thủ (thứ tự liên kết) và template mặc định
// trong Thư viện (nil khi template vừa bị xoá — FK RESTRICT nên chỉ có thể
// thấy nil trong khoảnh khắc đọc trước khi transaction xoá rollback).
type ChannelView struct {
	domain.Channel
	Sources         []domain.ChannelSource
	DefaultTemplate *domain.ChannelTemplate
}

// Competitor là đối thủ liên kết, bối cảnh cho prompt nhận diện.
type Competitor struct {
	ID          string
	Title       string
	Handle      *string
	Description string
	AvatarURL   *string
}

// LanguageSample là văn bản của đối thủ tham chiếu dùng để đoán ngôn ngữ.
type LanguageSample struct {
	Title       string
	Description string
	Titles      []string
}

// LeaseInput là một lần giành quyền chạy nhận diện.
type LeaseInput struct {
	ChannelID string
	Part      string
	TaskID    *string
	Owner     string
	Token     string
	Until     time.Time
}

// PublishInput công bố nhận diện; ExpectedName là tên đọc lúc bắt đầu run.
type PublishInput struct {
	ChannelID    string
	Token        string
	Name         string
	ExpectedName string
	Branding     domain.Branding
	AvatarPath   string // tương đối; rỗng = giữ tài sản cũ
	BannerPath   string
}

// FailInput ghi lỗi của một run.
type FailInput struct {
	ChannelID string
	Token     string
	Code      string
	Message   string
	RetryAt   *time.Time
	Step      string // domain.IdentityStepText | IdentityStepImages; rỗng = chưa tới bước nào
}

// Store là phần lưu trữ Service cần — hiện thực là postgres.ChannelStore.
// Mọi phương thức theo ctx; ghi nhiều dòng tự lo transaction.
type Store interface {
	ListChannels(ctx context.Context) ([]ChannelView, error)
	GetChannel(ctx context.Context, id string) (ChannelView, bool, error)
	ChannelExists(ctx context.Context, id string) (bool, error)
	RivalExists(ctx context.Context, rivalID string) (bool, error)
	TemplateExists(ctx context.Context, templateID string) (bool, error)
	CreateChannel(ctx context.Context, in CreateInput) (string, error)
	PatchChannel(ctx context.Context, id string, p Patch) (bool, error)
	DeleteChannel(ctx context.Context, id string) (bool, error)
	LinkSource(ctx context.Context, id, rivalID string) (bool, error)
	UnlinkSource(ctx context.Context, id, rivalID string) (removed, paused bool, err error)
	SetSourceMinDuration(ctx context.Context, id, rivalID string, minDurationSec *int) (found bool, err error)

	LoadIdentityRun(ctx context.Context, id string) (IdentityRun, bool, error)
	AcquireIdentityLease(ctx context.Context, in LeaseInput) (bool, error)
	HeartbeatIdentityLease(ctx context.Context, id, token string, until time.Time) (bool, error)
	IdentityLeaseLive(ctx context.Context, id string) (bool, error)
	PublishIdentity(ctx context.Context, in PublishInput) (bool, error)
	FailIdentity(ctx context.Context, in FailInput) (bool, error)
	ReleaseIdentityLease(ctx context.Context, id, token string) error
	AttachIdentityTask(ctx context.Context, id, token, taskID string) (bool, error)
	NextIdentityLeaseExpiry(ctx context.Context) (*time.Time, error)
	MarkInterruptedIdentities(ctx context.Context) ([]string, error)
	ChannelAssetPaths(ctx context.Context, id string) (avatar, banner *string, found bool, err error)
	Competitors(ctx context.Context, id string) ([]Competitor, error)
	ReferenceLanguageText(ctx context.Context, id string) (LanguageSample, error)
}

// StandardTemplates tạo/xoá template của thư viện — hiện thực là
// *library.Service (cmd ghép), để mảng kênh không import mảng thư viện.
type StandardTemplates interface {
	CreateStandardTemplate(ctx context.Context, channelName string) (templateID string, err error)
	// NameStandardTemplates đặt tên kèm kênh cho Mẫu chuẩn còn tên trần của
	// kênh (tạo lúc kênh chưa có tên).
	NameStandardTemplates(ctx context.Context, channelID, channelName string) error
	DeleteTemplate(ctx context.Context, templateID string) error
}

// Storage giải đường dẫn tương đối trong DB ra tuyệt đối dưới thư mục dự án —
// hiện thực là *storage.Service.
type Storage interface {
	Root() string
	Resolve(rel string) (string, error)
}

// TaskInfo là dòng việc nền tối thiểu mảng kênh cần (activity.Task thoả qua
// adapter ở cmd): id để dọn khi xoá kênh.
type TaskInfo struct {
	ID string
}

// Tasks là việc nền + thông báo (activity.Service qua adapter ở cmd).
type Tasks interface {
	Start(ctx context.Context, spec domain.TaskSpec) (string, error)
	Update(ctx context.Context, id string, u domain.TaskUpdate) error
	Finish(ctx context.Context, id string) error
	FindTaskByObject(ctx context.Context, kind domain.TaskKind, object domain.ObjectRef) (*TaskInfo, error)
	Notify(ctx context.Context, kind, text, href, actionLabel string) (string, error)
}

// Options là phụ thuộc của Service.
type Options struct {
	Store     Store
	Templates StandardTemplates
	AI        AI
	Tasks     Tasks
	Storage   Storage
	Media     *media.Service
	Now       func() time.Time
	// HTTP tải avatar đối thủ (nil = client mặc định, chỉ https, không redirect).
	HTTP *http.Client
	// RunInBackground: serve = true (run nhận diện chạy nền, sống theo vòng đời
	// serve); CLI = false (tiến trình CLI không sống đủ lâu — CLI dùng
	// RunIdentity đồng bộ).
	RunInBackground bool
	// Diag nhận nguyên nhân kỹ thuật của lỗi nhận diện (đã được lớp gọi
	// redact); nil = bỏ. Lỗi lạ lưu vào kênh chỉ là câu trung lập.
	Diag func(msg string)
}

// Service là mặt nghiệp vụ của Kênh của bạn. An toàn cho goroutine; các run
// nhận diện chạy nền bám ctx do Start nhận, Close chờ chúng thoát.
type Service struct {
	store      Store
	templates  StandardTemplates
	ai         AI
	tasks      Tasks
	storage    Storage
	media      *media.Service
	now        func() time.Time
	http       *http.Client
	background bool
	notify     func(ref string)
	diagFn     func(string)

	mu      sync.Mutex
	baseCtx context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	closed  bool
	timer   *time.Timer
}

// New dựng Service; Start phải được gọi trước khi phục vụ để phục hồi run hỏng.
func New(opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		http:       opts.HTTP,
		diagFn:     opts.Diag,
		background: opts.RunInBackground,
		store:      opts.Store,
		templates:  opts.Templates,
		ai:         opts.AI,
		tasks:      opts.Tasks,
		storage:    opts.Storage,
		media:      opts.Media,
		now:        now,
	}
}

// SetRunInBackground bật/tắt run nhận diện chạy nền (serve bật; CLI để tắt).
func (s *Service) SetRunInBackground(v bool) { s.background = v }

// SetNotifier gắn hàm phát sự kiện SSE (nil = tắt, dùng cho CLI).
func (s *Service) SetNotifier(fn func(ref string)) { s.notify = fn }

func (s *Service) emit(ref string) {
	if s.notify != nil {
		s.notify(ref)
	}
}

// Start dựng ctx nền cho các run và đánh dấu run bị bỏ dở (lease hết hạn) là
// hỏng "interrupted" — không tự vẽ lại ảnh trả phí.
func (s *Service) Start(ctx context.Context) error {
	s.baseCtx, s.cancel = context.WithCancel(ctx)
	ids, err := s.store.MarkInterruptedIdentities(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		s.emit(id)
	}
	// Lease sống còn sót lại (chết trước khi hết hạn): hẹn reconcile đúng lúc
	// hết hạn thay vì để kênh hiện "generating" vô thời hạn. Không gọi Drawer.
	s.scheduleInterruptReconcile(ctx)
	return nil
}

// scheduleInterruptReconcile hẹn một lần tới hạn lease gần nhất; mỗi lần chạy
// lại hẹn tiếp theo mốc mới nhất. Không idle polling: timer chạy đúng một
// goroutine tại mốc.
func (s *Service) scheduleInterruptReconcile(ctx context.Context) {
	until, err := s.store.NextIdentityLeaseExpiry(ctx)
	if err != nil || until == nil {
		return
	}
	// Trễ tính theo đồng hồ của service (fake trong test) để timer nhất quán
	// với MarkInterruptedIdentities; hạn đã qua thì đợi 1 s rồi đánh dấu.
	delay := until.Sub(s.now())
	if delay <= 0 {
		delay = time.Second
	}
	// s.timer chỉ được đọc/ghi trong mu — callback có thể tái trang bị đồng thời
	// với Close.
	s.storeLock().Lock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(delay, func() {
		s.storeLock().Lock()
		closed := s.closed
		s.storeLock().Unlock()
		if closed || s.baseCtx == nil || s.baseCtx.Err() != nil {
			return
		}
		if ids, err := s.store.MarkInterruptedIdentities(context.Background()); err == nil {
			for _, id := range ids {
				s.emit(id)
			}
		}
		s.scheduleInterruptReconcile(context.Background())
	})
	s.storeLock().Unlock()
}

// admit chặn run mới sau khi Close bắt đầu; prepare chỉ được chạy giữa admit
// và release tương ứng để Close không trả xong rồi worker vẫn được spawn.
func (s *Service) admit() bool {
	s.storeLock().Lock()
	defer s.storeLock().Unlock()
	if s.closed || s.baseCtx == nil || s.baseCtx.Err() != nil {
		return false
	}
	s.wg.Add(1)
	return true
}

// release tương ứng admit; gọi cả khi prepare thất bại.
func (s *Service) release() { s.wg.Done() }

// Close đóng cổng nhận run mới, huỷ các run đang chạy và chờ chúng thoát (chúng
// tự ghi "interrupted" qua đường lỗi nếu còn giữ lease).
func (s *Service) Close(ctx context.Context) error {
	s.storeLock().Lock()
	s.closed = true
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.storeLock().Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// storeLock dùng mu của Service (khai báo ẩn) để bảo vệ closed.
func (s *Service) storeLock() *sync.Mutex { return &s.mu }

// ── đọc ────────────────────────────────────────────────────────────────────

// List trả mọi kênh (tên rỗng cuối danh sách) kèm đối thủ và template.
func (s *Service) List(ctx context.Context) ([]ChannelView, error) {
	return s.store.ListChannels(ctx)
}

// Get trả một kênh; kênh lạ → not_found.
func (s *Service) Get(ctx context.Context, id string) (ChannelView, error) {
	view, found, err := s.store.GetChannel(ctx, id)
	if err != nil {
		return ChannelView{}, err
	}
	if !found {
		return ChannelView{}, errNotFound("Kênh %q không tồn tại.", id)
	}
	return view, nil
}

// ── tạo ────────────────────────────────────────────────────────────────────

// CreateRequest là đầu vào chưa chuẩn hoá của POST /api/owner-channels.
type CreateRequest struct {
	Name          string
	YoutubeURL    *string
	CompetitorIDs []string
	AutoFetch     *bool
	AutoApprove   *bool
	PerDay        *int
	Language      string
	// DefaultTemplateID chọn template có sẵn trong Thư viện làm mặc định; vắng
	// = tạo Mẫu chuẩn mới cho kênh.
	DefaultTemplateID *string
	// AgentInstructions là chỉ dẫn ban đầu cho các trợ lý; vắng/rỗng = chưa dặn.
	AgentInstructions map[string]string
}

// Create tạo kênh theo đúng luồng: nếu client chỉ định defaultTemplateID thì
// chỉ kiểm tồn tại (404 khi lạ, không tạo Mẫu chuẩn, shared hợp lệ); ngược lại
// tạo template Mẫu chuẩn trước, một transaction (kênh + đối thủ + default),
// hỏng thì xoá template bù trừ; sau đó đoán ngôn ngữ từ đối thủ tham chiếu và
// chạy tạo nhận diện nền. Trả kênh vừa đọc lại và id việc nền nhận diện (rỗng
// khi chưa có đối thủ để tham chiếu).
func (s *Service) Create(ctx context.Context, in CreateRequest) (ChannelView, string, error) {
	name := strings.TrimSpace(in.Name)
	if utf8.RuneCountInString(name) > NameMaxRunes {
		return ChannelView{}, "", errInvalid("Tên kênh tối đa %d ký tự.", NameMaxRunes)
	}
	var youtubeURL *string
	if in.YoutubeURL != nil {
		u := strings.TrimSpace(*in.YoutubeURL)
		if utf8.RuneCountInString(u) > URLMaxRunes {
			return ChannelView{}, "", errInvalid("Link YouTube tối đa %d ký tự.", URLMaxRunes)
		}
		if u != "" {
			youtubeURL = &u
		}
	}
	competitors := dedupeIDs(in.CompetitorIDs)
	if len(competitors) > CompetitorsMax {
		return ChannelView{}, "", errInvalid("Một kênh liên kết tối đa %d đối thủ.", CompetitorsMax)
	}
	perDay := PerDayDefault
	if in.PerDay != nil {
		perDay = *in.PerDay
	}
	if err := validPerDay(perDay); err != nil {
		return ChannelView{}, "", err
	}
	autoFetch := true
	if in.AutoFetch != nil {
		autoFetch = *in.AutoFetch
	}
	autoApprove := false
	if in.AutoApprove != nil {
		autoApprove = *in.AutoApprove
	}
	// Chỉ dẫn ban đầu (nếu có): cùng luật gộp/khoá với PATCH.
	instructions := domain.NewAgentInstructions()
	if len(in.AgentInstructions) > 0 {
		merged, err := domain.MergeAgentInstructions(nil, in.AgentInstructions)
		if err != nil {
			return ChannelView{}, "", errInvalid("%s", err.Error())
		}
		instructions = merged
	}
	language := in.Language
	if language == "" {
		language = domain.LangAuto
	}
	switch language {
	case domain.LangAuto, domain.LangEN, domain.LangVI:
	default:
		return ChannelView{}, "", errInvalid("Ngôn ngữ phải là auto, en hoặc vi.")
	}

	id, err := newUUID()
	if err != nil {
		return ChannelView{}, "", err
	}
	// Template mặc định: client chỉ định thì chỉ kiểm tồn tại (404 khi lạ, không
	// fallback tạo Mẫu chuẩn); vắng thì tạo Mẫu chuẩn trước và bù trừ nếu tx
	// hỏng để không bỏ lại rác thư viện.
	templateID := ""
	if in.DefaultTemplateID != nil {
		tid := strings.TrimSpace(*in.DefaultTemplateID)
		if tid == "" {
			return ChannelView{}, "", errInvalid("defaultTemplateId không được rỗng.")
		}
		ok, err := s.store.TemplateExists(ctx, tid)
		if err != nil {
			return ChannelView{}, "", err
		}
		if !ok {
			return ChannelView{}, "", &Error{Code: CodeNotFound, Message: "Template không tồn tại.",
				Recovery: "Chọn lại template trong Thư viện."}
		}
		templateID = tid
	} else {
		created, err := s.templates.CreateStandardTemplate(ctx, name)
		if err != nil {
			return ChannelView{}, "", fmt.Errorf("tạo Mẫu chuẩn: %w", err)
		}
		templateID = created
	}
	_, err = s.store.CreateChannel(ctx, CreateInput{
		ID:                id,
		Name:              name,
		YoutubeURL:        youtubeURL,
		Language:          language,
		AutoFetch:         autoFetch,
		AutoApprove:       autoApprove,
		PerDay:            perDay,
		DefaultTemplateID: templateID,
		AgentInstructions: instructions,
		CompetitorIDs:     competitors,
	})
	if err != nil {
		// Bù trừ best-effort trên ctx thoát khỏi hủy của request (ctx có thể đã
		// cancel — lỗi gốc gây ra hủy); lỗi xoá không che lỗi gốc. Chỉ xoá khi
		// ta vừa tạo (client chọn template có sẵn thì không được xoá).
		if in.DefaultTemplateID == nil {
			compensateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			_ = s.templates.DeleteTemplate(compensateCtx, templateID)
		}
		return ChannelView{}, "", err
	}
	// Ngôn ngữ đoán từ đối thủ tham chiếu (dòng liên kết đầu) khi client không
	// chỉ định; không đoán được thì giữ "auto".
	if language == domain.LangAuto && len(competitors) > 0 {
		if guessed := s.guessLanguage(ctx, id); guessed != "" {
			// Best-effort: kênh đã tạo rồi, đoán ngôn ngữ hỏng không đáng trả 500.
			_, _ = s.store.PatchChannel(ctx, id, Patch{Language: Field[string]{Set: true, Value: guessed}})
		}
	}
	view, err := s.Get(ctx, id)
	if err != nil {
		return ChannelView{}, "", err
	}
	s.emit(id)
	jobID := ""
	if len(view.Sources) > 0 {
		jobID, _ = s.startIdentity(view, IdentityInput{Part: domain.IdentityAll})
		// Đọc lại để hình trả về đã ở trạng thái generating + jobId (run vừa
		// giành lease ngay trong startIdentity).
		if again, gerr := s.Get(ctx, id); gerr == nil {
			view = again
		}
	}
	return view, jobID, nil
}

// ── sửa / xoá ──────────────────────────────────────────────────────────────

// UpdateRequest là các trường client gửi; nil = không đổi, con trỏ tới chuỗi
// rỗng = xoá giá trị.
type UpdateRequest struct {
	Name              *string
	YoutubeURL        **string
	Language          *string
	StyleNote         **string
	DescriptionFooter **string
	AlwaysTags        *[]string
	AutoFetch         *bool
	AutoApprove       *bool
	PerDay            *int
	Brief             *string
	// DefaultTemplateID đổi template mặc định trong Thư viện; "" hoặc null là
	// không hợp lệ vì kênh luôn phải có mặc định.
	DefaultTemplateID **string
	// AgentInstructions gộp chỉ dẫn cho đúng các trợ lý có khoá trong map;
	// trợ lý vắng khoá giữ nguyên; giá trị rỗng xoá chỉ dẫn của trợ lý đó.
	AgentInstructions map[string]string
}

// Update sửa hồ sơ kênh. Trường vắng giữ nguyên; tags được trim, bỏ rỗng và
// bỏ trùng; ngôn ngữ/nhịp kiểm cùng luật với Create.
func (s *Service) Update(ctx context.Context, id string, in UpdateRequest) (ChannelView, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return ChannelView{}, err
	}
	var p Patch
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return ChannelView{}, errInvalid("Tên kênh không được rỗng.")
		}
		if utf8.RuneCountInString(name) > NameMaxRunes {
			return ChannelView{}, errInvalid("Tên kênh tối đa %d ký tự.", NameMaxRunes)
		}
		p.Name = Field[string]{Set: true, Value: name}
	}
	if in.YoutubeURL != nil {
		var value *string
		if *in.YoutubeURL != nil {
			u := strings.TrimSpace(**in.YoutubeURL)
			if utf8.RuneCountInString(u) > URLMaxRunes {
				return ChannelView{}, errInvalid("Link YouTube tối đa %d ký tự.", URLMaxRunes)
			}
			if u != "" {
				value = &u
			}
		}
		p.YoutubeURL = Field[*string]{Set: true, Value: value}
	}
	if in.Language != nil {
		switch *in.Language {
		case domain.LangAuto, domain.LangEN, domain.LangVI:
			p.Language = Field[string]{Set: true, Value: *in.Language}
		default:
			return ChannelView{}, errInvalid("Ngôn ngữ phải là auto, en hoặc vi.")
		}
	}
	if in.StyleNote != nil {
		var value *string
		if *in.StyleNote != nil {
			v := strings.TrimSpace(**in.StyleNote)
			if utf8.RuneCountInString(v) > StyleMaxRunes {
				return ChannelView{}, errInvalid("Giọng văn tối đa %d ký tự.", StyleMaxRunes)
			}
			if v != "" {
				value = &v
			}
		}
		p.StyleNote = Field[*string]{Set: true, Value: value}
	}
	if in.DescriptionFooter != nil {
		var value *string
		if *in.DescriptionFooter != nil {
			v := strings.TrimSpace(**in.DescriptionFooter)
			if utf8.RuneCountInString(v) > FooterMaxRunes {
				return ChannelView{}, errInvalid("Đoạn cuối mô tả tối đa %d ký tự.", FooterMaxRunes)
			}
			if v != "" {
				value = &v
			}
		}
		p.DescriptionFooter = Field[*string]{Set: true, Value: value}
	}
	if in.AlwaysTags != nil {
		tags := normalizeTags(*in.AlwaysTags)
		if len(tags) > AlwaysTagsMax {
			return ChannelView{}, errInvalid("Tag luôn thêm tối đa %d tag.", AlwaysTagsMax)
		}
		p.AlwaysTags = Field[[]string]{Set: true, Value: tags}
	}
	if in.AutoFetch != nil {
		p.AutoFetch = Field[bool]{Set: true, Value: *in.AutoFetch}
	}
	if in.AutoApprove != nil {
		p.AutoApprove = Field[bool]{Set: true, Value: *in.AutoApprove}
	}
	if in.PerDay != nil {
		if err := validPerDay(*in.PerDay); err != nil {
			return ChannelView{}, err
		}
		p.PerDay = Field[int]{Set: true, Value: *in.PerDay}
	}
	if len(in.AgentInstructions) > 0 {
		cur, err := s.Get(ctx, id)
		if err != nil {
			return ChannelView{}, err
		}
		r := in.AgentInstructions
		if len(r) != 1 {
			return ChannelView{}, errInvalid("Mỗi lần chỉ gửi chỉ dẫn cho một trợ lý.")
		}
		key := ""
		for k := range r {
			key = k
		}
		merged, err := domain.MergeAgentInstructions(cur.AgentInstructions, r)
		if err != nil {
			return ChannelView{}, errInvalid("%s", err.Error())
		}
		value := merged[key]
		p.AgentInstructions = AgentInstructionPatch{Set: true, Key: key, Value: value}
	}
	if in.DefaultTemplateID != nil {
		tid := ""
		if *in.DefaultTemplateID != nil {
			tid = strings.TrimSpace(**in.DefaultTemplateID)
		}
		if tid == "" {
			return ChannelView{}, errInvalid("Template mặc định không được rỗng.")
		}
		ok, err := s.store.TemplateExists(ctx, tid)
		if err != nil {
			return ChannelView{}, err
		}
		if !ok {
			return ChannelView{}, &Error{Code: CodeNotFound, Message: "Template không tồn tại.",
				Recovery: "Chọn lại template trong Thư viện."}
		}
		p.DefaultTemplateID = Field[string]{Set: true, Value: tid}
	}
	if in.Brief != nil {
		brief := strings.TrimSpace(*in.Brief)
		if utf8.RuneCountInString(brief) > BriefMaxRunes {
			return ChannelView{}, errInvalid("Mô tả kênh tối đa %d ký tự.", BriefMaxRunes)
		}
		p.Brief = Field[string]{Set: true, Value: brief}
	}
	if _, err := s.store.PatchChannel(ctx, id, p); err != nil {
		return ChannelView{}, err
	}
	if p.Name.Set {
		s.nameStandardTemplates(ctx, id, p.Name.Value)
	}
	view, err := s.Get(ctx, id)
	if err != nil {
		return ChannelView{}, err
	}
	s.emit(id)
	return view, nil
}

// nameStandardTemplates đặt tên kèm kênh cho Mẫu chuẩn còn tên trần. Lỗi chỉ
// ghi nhật ký: tên kênh đã lưu, tên template là phần phụ.
func (s *Service) nameStandardTemplates(ctx context.Context, id, name string) {
	if err := s.templates.NameStandardTemplates(ctx, id, name); err != nil && s.diagFn != nil {
		s.diagFn("đặt tên Mẫu chuẩn cho kênh " + id + ": " + err.Error())
	}
}

// Delete xoá kênh (link + template xoá theo) và thư mục tài sản của nó.
func (s *Service) Delete(ctx context.Context, id string) error {
	view, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	removed, err := s.store.DeleteChannel(ctx, id)
	if err != nil {
		return err
	}
	if !removed {
		return errNotFound("Kênh %q không tồn tại.", id)
	}
	// Dọn dòng việc nền còn sống của kênh (nếu có) rồi xoá tệp — tìm theo id đã
	// lưu trên kênh (không dựa vào nhãn object có thể khác lúc start).
	if s.tasks != nil && view.IdentityTaskID != nil {
		_ = s.tasks.Finish(ctx, *view.IdentityTaskID)
	}
	s.removeChannelFiles(id)
	s.emit(id)
	return nil
}

// removeChannelFiles xoá thư mục channels/<id> (best-effort: kênh đã xoá khỏi
// DB rồi, không có gì để bù trừ).
func (s *Service) removeChannelFiles(id string) {
	if s.storage == nil || s.media != nil {
		return
	}
	dir, err := s.storage.Resolve("channels/" + id)
	if err != nil {
		return
	}
	_ = os.RemoveAll(dir)
}

// ── liên kết đối thủ ───────────────────────────────────────────────────────

// Link liên kết một đối thủ đang theo dõi (idempotent). Kênh chưa có nhận diện
// và vừa có đối thủ đầu tiên thì chạy tạo nhận diện nền như lúc tạo kênh.
func (s *Service) Link(ctx context.Context, id, rivalID string) (ChannelView, error) {
	view, err := s.Get(ctx, id)
	if err != nil {
		return ChannelView{}, err
	}
	rivalID = strings.TrimSpace(rivalID)
	if rivalID == "" {
		return ChannelView{}, errInvalid("Thiếu rivalId.")
	}
	ok, err := s.store.RivalExists(ctx, rivalID)
	if err != nil {
		return ChannelView{}, err
	}
	if !ok {
		return ChannelView{}, &Error{Code: CodeNotFound,
			Message:  fmt.Sprintf("Đối thủ %s chưa được theo dõi.", rivalID),
			Recovery: "Thêm đối thủ ở trang Đối thủ trước."}
	}
	inserted, err := s.store.LinkSource(ctx, id, rivalID)
	if err != nil {
		return ChannelView{}, err
	}
	view, err = s.Get(ctx, id)
	if err != nil {
		return ChannelView{}, err
	}
	s.emit(id)
	if inserted && len(view.Sources) == 1 && !hasIdentity(view.Channel) {
		if _, ierr := s.startIdentity(view, IdentityInput{Part: domain.IdentityAll}); ierr == nil {
			if again, gerr := s.Get(ctx, id); gerr == nil {
				view = again
			}
		}
	}
	return view, nil
}

// Unlink bỏ một liên kết. Bỏ đối thủ cuối cùng thì Tự lấy tự tắt; paused báo
// cho UI ghi lý do. Nhận diện đã có không đổi.
func (s *Service) Unlink(ctx context.Context, id, rivalID string) (ChannelView, bool, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return ChannelView{}, false, err
	}
	removed, paused, err := s.store.UnlinkSource(ctx, id, rivalID)
	if err != nil {
		return ChannelView{}, false, err
	}
	if !removed {
		return ChannelView{}, false, &Error{Code: CodeNotFound,
			Message:  fmt.Sprintf("Đối thủ %s chưa được liên kết với kênh.", rivalID),
			Recovery: "Tải lại trang Kênh của bạn."}
	}
	view, err := s.Get(ctx, id)
	if err != nil {
		return ChannelView{}, false, err
	}
	s.emit(id)
	return view, paused, nil
}

// MaxSourceMinDurationSec là trần bộ lọc thời lượng Tự lấy của một đối thủ.
const MaxSourceMinDurationSec = 12 * 3600

// SetSourceMinDuration đặt bộ lọc Tự lấy theo đối thủ: chỉ lấy video dài từ
// minDurationSec giây (nil = bỏ lọc). Chỉ ảnh hưởng video thấy từ lần quét sau.
func (s *Service) SetSourceMinDuration(ctx context.Context, id, rivalID string, minDurationSec *int) (ChannelView, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return ChannelView{}, err
	}
	if minDurationSec != nil && (*minDurationSec < 1 || *minDurationSec > MaxSourceMinDurationSec) {
		return ChannelView{}, errInvalid("Thời lượng tối thiểu phải từ 1 giây tới %d giờ.", MaxSourceMinDurationSec/3600)
	}
	found, err := s.store.SetSourceMinDuration(ctx, id, rivalID, minDurationSec)
	if err != nil {
		return ChannelView{}, err
	}
	if !found {
		return ChannelView{}, &Error{Code: CodeNotFound,
			Message:  fmt.Sprintf("Đối thủ %s chưa được liên kết với kênh.", rivalID),
			Recovery: "Tải lại trang Kênh của bạn."}
	}
	view, err := s.Get(ctx, id)
	if err != nil {
		return ChannelView{}, err
	}
	s.emit(id)
	return view, nil
}

// ── template luân phiên ────────────────────────────────────────────────────

func foundChannel(view ChannelView) bool { return view.ID != "" }

// ── tiện ích ───────────────────────────────────────────────────────────────

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func channelObject(id, label string) domain.ObjectRef {
	if label == "" {
		label = "Kênh"
	}
	return domain.ObjectRef{Type: "channel", ID: id, Label: label}
}

func dedupeIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func normalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > AlwaysTagRunes {
			tag = string([]rune(tag)[:AlwaysTagRunes])
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tag)
	}
	return out
}

// validPerDay kiểm nhịp video mỗi ngày (1–20, không clamp).
func validPerDay(n int) error {
	if n < PerDayMin || n > PerDayMax {
		return &Error{Code: CodeInvalidInput,
			Message:  fmt.Sprintf("Nhịp mỗi ngày phải trong khoảng %d–%d.", PerDayMin, PerDayMax),
			Field:    "perDay",
			Recovery: "Chọn lại số video mỗi ngày."}
	}
	return nil
}

// hasIdentity báo kênh đã có nhận diện đã công bố. Chỉ Mô tả kênh chủ sửa
// (branding.brief) KHÔNG tính: kênh đã viết mô tả nhưng chưa tạo vẫn cần chạy.
// Tên do chủ đặt cũng không tính.
func hasIdentity(c domain.Channel) bool {
	return c.AvatarPath != nil || c.BannerPath != nil ||
		c.Branding.Look != "" || c.Branding.Avatar != "" || c.Branding.Summary != ""
}
