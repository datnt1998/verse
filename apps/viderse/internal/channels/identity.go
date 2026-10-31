package channels

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/core/llm"
)

// Kích thước tài sản nhận diện (yt-factory branding.ts).
const (
	AvatarSize      = 800
	BannerWidth     = 2560
	BannerHeight    = 1440
	BannerSafeWidth = 1546
	// BannerSafeHeight là băng an toàn hiện đủ trên điện thoại (yt-factory).
	BannerSafeHeight = 423
	BannerMaxBytes   = 6 * 1024 * 1024
)

// Nhịp lease của một run nhận diện: TTL 60 s, tim mỗi 5 s (mất lease thì run
// huỷ và không được công bố).
const (
	identityLeaseTTL       = 60 * time.Second
	identityHeartbeatEvery = 5 * time.Second
)

// referenceMaxBytes và timeout tải avatar đối thủ (yt-factory: 8 MB / 20 s).
const (
	referenceMaxBytes = 8 * 1024 * 1024
	referenceTimeout  = 20 * time.Second
)

// allowedReferenceHost: chỉ CDN ảnh kênh YouTube được phép tải.
func allowedReferenceHost(host string) bool {
	return host == "yt3.googleusercontent.com" || host == "yt3.ggpht.com"
}

// TextModel là một model chữ của chuỗi Đạo diễn (core/llm.ClientModel thoả).
type TextModel interface {
	Generate(ctx context.Context, messages []llm.Message, tools []llm.ToolSpec, opts ...llm.CallOption) (*llm.LLMResponse, error)
}

// Drawer là đường vẽ ảnh (imagegen.Service qua adapter ở cmd). QuotaReset trả
// mốc tự thử lại thật của lần hạn mức gần nhất (rỗng = không biết).
type Drawer interface {
	Draw(ctx context.Context, req imagegen.Request) (imagegen.Receipt, error)
	QuotaReset() (time.Time, bool)
}

// AI là cấu hình trợ lý hiện tại; gọi lại mỗi lần chạy (cấu hình có thể đổi).
type AI interface {
	Director() []TextModel
	Drawer() Drawer
}

// IdentityInput là đầu vào của POST /api/owner-channels/{id}/branding.
type IdentityInput struct {
	Part      string
	Brief     string
	Direction string
}

// IdentityRun là dữ liệu đọc lúc bắt đầu run (fence publish theo Name).
type IdentityRun struct {
	ID         string
	Name       string
	StyleNote  *string
	Branding   domain.Branding
	AvatarPath *string
	BannerPath *string
}

// identityRun là trạng thái của một run đang chạy.
type identityRun struct {
	channelID    string
	token        string
	taskID       string
	part         string
	brief        string
	direction    string
	expectedName string
	previous     domain.Branding
	oldAvatar    *string
	oldBanner    *string
	styleNote    *string
	competitors  []Competitor
	needsText    bool
	writeBrief   bool
	proposeName  bool
	images       []string
	// step là bước đang chạy (chỉ goroutine của run đọc/ghi), ghi kèm lỗi.
	step string

	ctx    context.Context
	cancel context.CancelFunc
	lost   atomic.Bool
}

// GenerateIdentity giành quyền chạy rồi trả ngay id việc nền; run chạy nền.
// Run thứ hai cùng kênh → conflict. Run được đăng ký với service trước khi
// prepare nên Close không thể trả xong khi worker chưa spawn.
func (s *Service) GenerateIdentity(ctx context.Context, id string, in IdentityInput) (string, error) {
	if !s.admit() {
		return "", errConflict("Service kênh đang dừng.")
	}
	run, err := s.prepareIdentity(ctx, id, in)
	if err != nil {
		s.release()
		return "", err
	}
	go func() {
		defer s.release()
		s.executeIdentity(run)
	}()
	return run.taskID, nil
}

// RunIdentity như GenerateIdentity nhưng chạy xong mới trả (CLI không có serve
// giữ tiến trình). Lỗi của run được trả về sau khi đã ghi trạng thái hỏng.
func (s *Service) RunIdentity(ctx context.Context, id string, in IdentityInput) (string, error) {
	if !s.admit() {
		return "", errConflict("Service kênh đang dừng.")
	}
	run, err := s.prepareIdentity(ctx, id, in)
	if err != nil {
		s.release()
		return "", err
	}
	defer s.release()
	return run.taskID, s.executeIdentity(run)
}

// prepareIdentity kiểm đầu vào, đọc kênh, mở việc nền và giành lease.
func (s *Service) prepareIdentity(ctx context.Context, id string, in IdentityInput) (*identityRun, error) {
	switch in.Part {
	case domain.IdentityAll, domain.IdentityName, domain.IdentityAvatar, domain.IdentityBanner:
	default:
		return nil, errInvalid("part phải là all, name, avatar hoặc banner.")
	}
	brief := strings.TrimSpace(in.Brief)
	if utf8.RuneCountInString(brief) > BriefMaxRunes {
		return nil, errInvalid("Mô tả kênh tối đa %d ký tự.", BriefMaxRunes)
	}
	direction := strings.TrimSpace(in.Direction)
	if utf8.RuneCountInString(direction) > DirectionMaxRune {
		return nil, errInvalid("Góp ý tối đa %d ký tự.", DirectionMaxRune)
	}
	loaded, found, err := s.store.LoadIdentityRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errNotFound("Kênh %q không tồn tại.", id)
	}
	// "Previous" = một bộ thiết kế đã tồn tại (look/summary/brief ảnh), không
	// chỉ Mô tả kênh chủ sửa — kênh đã ghi brief nhưng chưa từng tạo vẫn cần
	// bước chữ để có thiết kế rồi mới vẽ.
	hasPrevious := loaded.Branding.Look != "" || loaded.Branding.Avatar != "" || loaded.Branding.Banner != ""
	needsText := in.Part == domain.IdentityAll || in.Part == domain.IdentityName || !hasPrevious
	// Không gửi Mô tả kênh thì dùng mô tả đã lưu (yt-factory: brief của lần
	// trước) — model chỉ viết mô tả mới khi kênh chưa có.
	if brief == "" {
		brief = loaded.Branding.Brief
	}
	writeBrief := needsText && brief == ""
	var competitors []Competitor
	if needsText {
		competitors, err = s.store.Competitors(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	if writeBrief && len(competitors) == 0 {
		return nil, &Error{Code: CodeInvalidInput, Message: "Viết mô tả kênh hoặc liên kết một đối thủ trước khi tạo nhận diện.",
			Recovery: "Điền mô tả kênh rồi tạo lại."}
	}

	token, err := newUUID()
	if err != nil {
		return nil, err
	}
	base := s.baseCtx
	if base == nil {
		base = context.Background()
	}
	runCtx, cancel := context.WithCancel(base)
	run := &identityRun{
		channelID:    id,
		token:        token,
		part:         in.Part,
		brief:        brief,
		direction:    direction,
		expectedName: loaded.Name,
		previous:     loaded.Branding,
		oldAvatar:    loaded.AvatarPath,
		oldBanner:    loaded.BannerPath,
		styleNote:    loaded.StyleNote,
		competitors:  competitors,
		needsText:    needsText,
		writeBrief:   writeBrief,
		proposeName:  in.Part == domain.IdentityAll || in.Part == domain.IdentityName,
		ctx:          runCtx,
		cancel:       cancel,
	}
	for _, kind := range []string{domain.IdentityAvatar, domain.IdentityBanner} {
		if in.Part == domain.IdentityAll || in.Part == kind {
			run.images = append(run.images, kind)
		}
	}

	acquired, err := s.store.AcquireIdentityLease(ctx, LeaseInput{
		ChannelID: id,
		Part:      in.Part,
		Owner:     identityOwner(),
		Token:     token,
		Until:     s.now().Add(identityLeaseTTL),
	})
	if err != nil {
		cancel()
		return nil, err
	}
	if !acquired {
		cancel()
		return nil, errConflict("Đang có một lượt tạo nhận diện khác cho kênh này.")
	}
	// Task tạo SAU khi giữ lease: request thua lease không bao giờ tạo rồi xoá
	// task của run đang thắng (activity adopt theo object, không theo lease).
	taskID, err := s.startIdentityTask(ctx, run)
	if err != nil {
		cancel()
		_ = s.store.ReleaseIdentityLease(ctx, id, token)
		return nil, err
	}
	run.taskID = taskID
	attached, err := s.store.AttachIdentityTask(ctx, id, token, taskID)
	if err != nil || !attached {
		cancel()
		_ = s.tasks.Finish(ctx, taskID)
		if err == nil {
			err = errConflict("Đang có một lượt tạo nhận diện khác cho kênh này.")
		}
		return nil, err
	}
	s.emit(id)
	return run, nil
}

// startIdentity chạy nhận diện nền; lỗi của run đã được ghi vào kênh nên
// create/link không hỏng theo.
func (s *Service) startIdentity(view ChannelView, in IdentityInput) (string, error) {
	if !s.background {
		// CLI: tiến trình không sống đủ lâu để giữ lease — người dùng chạy
		// `viderse channel identity` (đồng bộ) khi muốn tạo nhận diện.
		return "", nil
	}
	return s.GenerateIdentity(context.Background(), view.ID, in)
}

// identityOwner là chủ lease: máy + tiến trình, đủ để nhìn ra ai đang giữ.
func identityOwner() string {
	host, _ := os.Hostname()
	return net.JoinHostPort(host, strconv.Itoa(os.Getpid()))
}

// startIdentityTask mở dòng việc nền "identity" cho kênh.
func (s *Service) startIdentityTask(ctx context.Context, run *identityRun) (string, error) {
	if s.tasks == nil {
		return "", nil
	}
	total := int64(0)
	if run.needsText {
		total++
	}
	total += int64(len(run.images)) + 1 // công bố
	return s.tasks.Start(ctx, domain.TaskSpec{
		Kind:   domain.TaskIdentity,
		Agent:  "artist",
		Title:  "Tạo bộ nhận diện",
		Object: channelObject(run.channelID, run.expectedName),
		Phase:  "Đang chuẩn bị tạo nhận diện",
		Progress: &domain.TaskProgress{
			Done:  0,
			Total: total,
		},
		Action: domain.TaskAction{Label: "Mở kênh", Href: channelHref(run.channelID)},
	})
}

func channelHref(id string) string { return "/channels/" + id + "?tab=settings" }

// executeIdentity chạy một run đã giữ lease: chữ → ảnh → công bố, mọi lỗi
// đều nhả lease và giữ nguyên nhận diện cũ.
func (s *Service) executeIdentity(run *identityRun) error {
	ctx := run.ctx
	defer run.cancel()
	// Tim lease; mất lease thì huỷ run và cấm công bố.
	stopHeartbeat := make(chan struct{})
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(identityHeartbeatEvery)
		defer ticker.Stop()
		for {
			select {
			case <-stopHeartbeat:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				ok, err := s.store.HeartbeatIdentityLease(ctx, run.channelID, run.token, s.now().Add(identityLeaseTTL))
				if err != nil || !ok {
					run.lost.Store(true)
					run.cancel()
					return
				}
			}
		}
	}()
	defer func() {
		close(stopHeartbeat)
		<-heartbeatDone
	}()

	// Thư mục làm việc: tài sản cuối nằm trong channels/<id>/, tệp nháp trong
	// thư mục con tạm và luôn bị dọn.
	relDir := "channels/" + run.channelID
	absDir, err := s.storage.Resolve(relDir)
	if err != nil {
		return s.failIdentity(run, &Error{Code: CodeInternal, Message: "Không mở được thư mục kênh."})
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return s.failIdentity(run, &Error{Code: CodeInternal, Message: "Không tạo được thư mục kênh."})
	}
	scratch := filepath.Join(absDir, ".staging-"+run.token[:8])
	if err := os.MkdirAll(scratch, 0o700); err != nil {
		return s.failIdentity(run, &Error{Code: CodeInternal, Message: "Không tạo được thư mục tạm."})
	}
	defer os.RemoveAll(scratch)

	var created []string // tệp cuối đã ghi; xoá nếu run hỏng
	defer func() {
		for _, f := range created {
			_ = os.Remove(f)
		}
	}()

	done := int64(0)
	progress := func(phase string, step bool) {
		if step {
			done++
		}
		if s.tasks == nil || run.taskID == "" {
			return
		}
		total := int64(len(run.images) + 1)
		if run.needsText {
			total++
		}
		_ = s.tasks.Update(ctx, run.taskID, domain.TaskUpdate{
			Phase:    &phase,
			Progress: &domain.TaskProgress{Done: done, Total: total},
		})
	}

	design := domain.Branding{
		Brief:   run.brief,
		Summary: run.previous.Summary,
		Look:    run.previous.Look,
		Avatar:  run.previous.Avatar,
		Banner:  run.previous.Banner,
	}
	name := run.expectedName

	if run.needsText {
		run.step = domain.IdentityStepText
		progress("Đang đặt tên và viết mô tả kênh", false)
		answer, err := s.generateText(ctx, run)
		if err != nil {
			return s.failIdentity(run, err)
		}
		design = brandingStateFromAnswer(answer, run.brief)
		if run.proposeName {
			if answer.Name == "" {
				return s.failIdentity(run, &Error{Code: CodeInternal, Message: "Đề xuất thiếu tên kênh.",
					Recovery: "Chạy lại tạo nhận diện."})
			}
			name = answer.Name
		}
		if run.writeBrief {
			if answer.Brief == "" {
				return s.failIdentity(run, &Error{Code: CodeInternal, Message: "Đề xuất thiếu mô tả kênh.",
					Recovery: "Chạy lại tạo nhận diện."})
			}
			design.Brief = answer.Brief
		}
		progress("Đã đặt tên và viết mô tả kênh", true)
	} else if run.brief != "" {
		// Chỉ vẽ lại một ảnh nhưng chủ sửa Mô tả kênh: giữ brief mới.
		design.Brief = run.brief
	}

	avatarRel := ""
	bannerRel := ""
	for _, kind := range run.images {
		run.step = domain.IdentityStepImages
		if run.lost.Load() {
			break
		}
		if kind == domain.IdentityAvatar {
			progress("Đang vẽ avatar 800×800", false)
			rel, err := s.drawAsset(ctx, run, kind, name, design, scratch, &created)
			if err != nil {
				return s.failIdentity(run, err)
			}
			avatarRel = rel
			progress("Đã vẽ avatar", true)
		} else {
			progress("Đang vẽ ảnh bìa kênh 2560×1440", false)
			rel, err := s.drawAsset(ctx, run, kind, name, design, scratch, &created)
			if err != nil {
				return s.failIdentity(run, err)
			}
			bannerRel = rel
			progress("Đã vẽ ảnh bìa kênh", true)
		}
	}

	if run.lost.Load() {
		return s.failIdentity(run, &Error{Code: CodeConflict,
			Message: "Mất quyền tạo nhận diện — một lượt khác đã chạy.", Recovery: "Chạy lại tạo nhận diện."})
	}
	if err := ctx.Err(); err != nil {
		return s.failIdentity(run, &Error{Code: CodeInterrupted,
			Message: "Lần tạo nhận diện dừng giữa chừng.", Recovery: "Chạy lại tạo nhận diện."})
	}
	progress("Đang công bố nhận diện", false)
	ok, err := s.store.PublishIdentity(ctx, PublishInput{
		ChannelID:    run.channelID,
		Token:        run.token,
		Name:         name,
		ExpectedName: run.expectedName,
		Branding:     design,
		AvatarPath:   avatarRel,
		BannerPath:   bannerRel,
	})
	if err != nil {
		return s.failIdentity(run, err)
	}
	if !ok {
		return s.failIdentity(run, &Error{Code: CodeConflict,
			Message:  "Kênh đã đổi trong lúc tạo nhận diện — kết quả bị huỷ.",
			Recovery: "Tải lại trang rồi chạy lại."})
	}
	progress("Đã công bố nhận diện", true)
	if name != run.expectedName {
		s.nameStandardTemplates(ctx, run.channelID, name)
	}

	// Chỉ tệp được giữ lại mới sống; tệp cũ bị thay thế dọn ngay.
	kept := map[string]bool{}
	if avatarRel != "" {
		kept[avatarRel] = true
	}
	if bannerRel != "" {
		kept[bannerRel] = true
	}
	s.removeSuperseded(absDir, run.oldAvatar, avatarRel, kept)
	s.removeSuperseded(absDir, run.oldBanner, bannerRel, kept)
	created = nil

	if s.tasks != nil && run.taskID != "" {
		_ = s.tasks.Finish(ctx, run.taskID)
		_, _ = s.tasks.Notify(ctx, "task_done", "Đã tạo nhận diện cho "+displayName(name)+".",
			channelHref(run.channelID), "Mở kênh")
	}
	s.emit(run.channelID)
	return nil
}

func displayName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "kênh"
	}
	return name
}

// removeSuperseded xoá tệp cũ của một loại tài sản khi đã có tệp mới (giữ cả
// hai khi run chỉ vẽ một ảnh — tài sản kia không đổi).
func (s *Service) removeSuperseded(absDir string, oldPath *string, newRel string, kept map[string]bool) {
	if newRel == "" || oldPath == nil {
		return
	}
	oldRel := *oldPath
	if kept[oldRel] || oldRel == newRel {
		return
	}
	_ = os.Remove(filepath.Join(absDir, filepath.Base(oldRel)))
}

// failIdentity ghi lỗi (nếu còn lease). Chỉ khi bản ghi lỗi được chấp nhận mới
// cập nhật việc nền: run mất lease trước đó thì không đụng task có thể đã được
// owner mới nhận lại.
func (s *Service) failIdentity(run *identityRun, cause error) error {
	code, message, retryAt, recovery := classifyIdentityError(cause)
	var known *Error
	if !errors.As(cause, &known) && s.diagFn != nil {
		s.diagFn("tạo nhận diện " + run.channelID + ": " + cause.Error())
	}
	// Ghi lỗi trên ctx riêng: ctx của run có thể đã bị huỷ vì mất lease.
	writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	landed, err := s.store.FailIdentity(writeCtx, FailInput{
		ChannelID: run.channelID,
		Token:     run.token,
		Code:      code,
		Message:   message,
		RetryAt:   retryAt,
		Step:      run.step,
	})
	if err != nil && s.diagFn != nil {
		s.diagFn("ghi lỗi nhận diện " + run.channelID + ": " + err.Error())
	}
	if landed && s.tasks != nil && run.taskID != "" {
		phase := message
		_ = s.tasks.Update(writeCtx, run.taskID, domain.TaskUpdate{
			Phase: &phase,
			State: domain.TaskFailed,
			Action: &domain.TaskAction{
				Label: "Thử lại", Href: channelHref(run.channelID),
			},
		})
	}
	if landed {
		s.emit(run.channelID)
	}
	return &Error{Code: code, Message: message, Recovery: recovery, RetryAt: retryAt}
}

// classifyIdentityError đổi lỗi bên trong thành mã + câu tiếng Việt + giờ thử
// lại. Chỉ lớp lỗi nghiệp vụ (channels.Error) và lỗi đường vẽ (DrawError, đã
// do provider dịch) được giữ nguyên câu; MỌI lỗi lạ trả câu trung lập — nguyên
// nhân (có thể chứa đường dẫn/chi tiết upstream) chỉ đi diag, không lưu DB hay
// trả UI.
func classifyIdentityError(err error) (code, message string, retryAt *time.Time, recovery string) {
	var de *imagegen.DrawError
	if errors.As(err, &de) {
		return de.Code, de.Message, de.RetryAt, de.Recovery
	}
	var ce *Error
	if errors.As(err, &ce) {
		return ce.Code, ce.Message, ce.RetryAt, ce.Recovery
	}
	if errors.Is(err, context.Canceled) {
		return CodeInterrupted, "Lần tạo nhận diện dừng giữa chừng.", nil, "Chạy lại tạo nhận diện."
	}
	return CodeInternal, "Tạo nhận diện hỏng.", nil, "Chạy lại tạo nhận diện."
}

// generateText gọi chuỗi model chữ của Đạo diễn (thử lần lượt) với avatar đối
// thủ làm tham chiếu nếu tải được.
func (s *Service) generateText(ctx context.Context, run *identityRun) (brandingAnswer, error) {
	models := s.director()
	if len(models) == 0 {
		return brandingAnswer{}, &Error{Code: CodeInternal,
			Message: "Chưa cấu hình model chữ cho Đạo diễn.", Recovery: "Mở Cài đặt › Trợ lý AI."}
	}
	var ref *referenceImage
	if len(run.competitors) > 0 {
		ref = s.fetchReference(ctx, run.competitors[0].AvatarURL)
	}
	prompt := buildBrandingTextPrompt(textPromptInput{
		CurrentName:  run.expectedName,
		ProposeName:  run.proposeName,
		Brief:        run.brief,
		Direction:    run.direction,
		StyleNote:    run.styleNote,
		Competitors:  run.competitors,
		HasAvatarRef: ref != nil,
	})
	content := []llm.ContentBlock{llm.TextBlock(prompt)}
	if ref != nil {
		content = append(content, llm.ImageBlock(base64.StdEncoding.EncodeToString(ref.data), ref.mime))
	}
	messages := []llm.Message{{Role: llm.RoleUser, Content: content, Timestamp: s.now()}}

	var lastErr error
	for _, model := range models {
		resp, err := model.Generate(ctx, messages, nil)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return brandingAnswer{}, err
			}
			continue
		}
		if resp == nil || resp.Message.TextContent() == "" {
			lastErr = &Error{Code: CodeInternal, Message: "Model chữ trả lời rỗng."}
			continue
		}
		return parseBrandingAnswer(resp.Message.TextContent())
	}
	if lastErr == nil {
		lastErr = &Error{Code: CodeInternal, Message: "Model chữ trả lời rỗng."}
	}
	return brandingAnswer{}, lastErr
}

func (s *Service) director() []TextModel {
	if s.ai == nil {
		return nil
	}
	return s.ai.Director()
}

func (s *Service) drawer() Drawer {
	if s.ai == nil {
		return nil
	}
	return s.ai.Drawer()
}

// referenceImage là avatar đối thủ đã tải, kèm mime thật.
type referenceImage struct {
	data []byte
	mime string
}

// fetchReference tải avatar đối thủ (chỉ CDN YouTube, https, ≤ 8 MB, không đi
// theo redirect). Lỗi/mất mạng trả nil: run vẫn thiết kế từ phần chữ.
func (s *Service) fetchReference(ctx context.Context, avatarURL *string) *referenceImage {
	if avatarURL == nil || *avatarURL == "" {
		return nil
	}
	parsed, err := url.Parse(*avatarURL)
	if err != nil || parsed.Scheme != "https" || !allowedReferenceHost(parsed.Hostname()) {
		return nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, referenceTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil
	}
	client := s.http
	if client == nil {
		client = &http.Client{
			Timeout: referenceTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	mime := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, referenceMaxBytes+1))
	if err != nil || len(data) == 0 || len(data) > referenceMaxBytes {
		return nil
	}
	return &referenceImage{data: data, mime: mime}
}

// drawAsset vẽ một tài sản từ brief của chính nó, ghi tệp cuối nguyên tử và
// trả đường dẫn tương đối. Ảnh bìa hạ chất lượng JPEG tới khi ≤ 6 MB.
func (s *Service) drawAsset(ctx context.Context, run *identityRun, kind, name string, design domain.Branding, scratch string, created *[]string) (string, error) {
	drawer := s.drawer()
	if drawer == nil {
		return "", &Error{Code: CodeInternal,
			Message: "Chưa cấu hình nguồn vẽ ảnh.", Recovery: "Mở Cài đặt › Kết nối AI."}
	}
	width, height := AvatarSize, AvatarSize
	brief := design.Avatar
	if kind == domain.IdentityBanner {
		width, height = BannerWidth, BannerHeight
		brief = design.Banner
	}
	if strings.TrimSpace(brief) == "" {
		return "", &Error{Code: CodeInternal,
			Message: "Thiếu brief ảnh của nhận diện.", Recovery: "Chạy lại tạo nhận diện."}
	}
	stem, err := newUUID()
	if err != nil {
		return "", err
	}
	pngPath := filepath.Join(scratch, stem+".png")
	_, err = drawer.Draw(ctx, imagegen.Request{
		Key:    "identity:" + run.channelID + ":" + run.token + ":" + kind,
		Prompt: buildBrandingImagePrompt(imagePromptInput{Kind: kind, Name: name, Design: brief, Look: design.Look, Direction: run.direction}),
		Output: pngPath,
		Width:  width,
		Height: height,
	})
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", s.drawError(err)
	}
	defer os.Remove(pngPath)

	// Provider có thể trả kích thước khác request (Images API chỉ có vài size)
	// — canvas phải được bảo đảm trong file công bố: cover-crop + resize về
	// đúng 800×800 / 2560×1440 thuần Go trước khi encode.
	src, err := decodeImage(pngPath)
	if err != nil {
		return "", err
	}
	relDir := "channels/" + run.channelID
	if kind == domain.IdentityAvatar {
		rel := relDir + "/" + stem + "-avatar.png"
		abs, err := s.storage.Resolve(rel)
		if err != nil {
			return "", err
		}
		if err := writePNGAtomic(coverScale(src, AvatarSize, AvatarSize), abs); err != nil {
			return "", err
		}
		*created = append(*created, abs)
		return rel, nil
	}
	rel := relDir + "/" + stem + "-banner.jpg"
	abs, err := s.storage.Resolve(rel)
	if err != nil {
		return "", err
	}
	if err := encodeBanner(coverScale(src, BannerWidth, BannerHeight), abs); err != nil {
		return "", err
	}
	*created = append(*created, abs)
	return rel, nil
}

// drawError đổi lỗi vẽ thành lỗi nghiệp vụ, lấy mốc thử lại thật khi hạn mức.
func (s *Service) drawError(err error) error {
	var de *imagegen.DrawError
	if !errors.As(err, &de) {
		return err
	}
	retryAt := de.RetryAt
	if de.Code == imagegen.ErrCodeQuota && retryAt == nil {
		if drawer := s.drawer(); drawer != nil {
			if t, ok := drawer.QuotaReset(); ok {
				retryAt = &t
			}
		}
	}
	return &Error{Code: de.Code, Message: de.Message, Recovery: de.Recovery, RetryAt: retryAt}
}
