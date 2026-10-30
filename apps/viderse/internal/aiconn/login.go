package aiconn

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Phase của một luồng đăng nhập OAuth — web poll để đổi UI từng bước.
const (
	LoginIdle       = "idle"       // không có luồng nào
	LoginAuthorize  = "authorize"  // đang chờ người dùng hoàn tất trên trình duyệt (authUrl)
	LoginCode       = "code"       // đang chờ nhập mã thiết bị (code + verifyUrl)
	LoginExchanging = "exchanging" // đã có mã, đang đổi lấy token
	LoginDone       = "done"
	LoginFailed     = "failed"
	LoginCancelled  = "cancelled"
)

// LoginSession là trạng thái một luồng đăng nhập (wire-level trong service).
type LoginSession struct {
	Phase     string
	AuthURL   string // phase authorize: URL cần mở
	Code      string // phase code: mã người dùng nhập
	VerifyURL string // phase code: trang nhập mã
	Account   string // phase done: nhãn tài khoản (email · plan)
	Error     string // phase failed: một câu lỗi đã che bí mật
	// ExpiresAt: hạn mã thiết bị do provider báo (expires_in); zero = không có.
	ExpiresAt time.Time
	StartedAt time.Time
}

// active báo luồng đang chạy (chưa done/failed/cancelled).
func (s *LoginSession) active() bool {
	switch s.Phase {
	case LoginAuthorize, LoginCode, LoginExchanging:
		return true
	}
	return s != nil && s.Phase != "" && s.Phase != LoginIdle &&
		s.Phase != LoginDone && s.Phase != LoginFailed && s.Phase != LoginCancelled
}

// session trả BẢN SAO session của provider (ok=false khi chưa từng đăng
// nhập) — goroutine đăng nhập sở hữu con trỏ gốc và sửa nó liên tục.
func (s *Service) session(providerID string) (*LoginSession, bool) {
	s.sessMu.RLock()
	defer s.sessMu.RUnlock()
	sess, ok := s.sess[providerID]
	if !ok {
		return nil, false
	}
	cp := *sess
	return &cp, true
}

// LoginStatus trả trạng thái luồng hiện tại (phase idle khi chưa có).
func (s *Service) LoginStatus(ctx context.Context, providerID string) (*LoginSession, error) {
	p, err := s.provider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if !SubscriptionKind(p.Kind) {
		return nil, apiError(http.StatusNotFound, domain.ReasonNotSubscription,
			"Nhà cung cấp này không đăng nhập")
	}
	if sess, ok := s.session(providerID); ok {
		return sess, nil
	}
	return &LoginSession{Phase: LoginIdle}, nil
}

// StartLogin mở luồng OAuth của provider. mode: "browser" (mặc định, callback
// localhost + dán mã dự phòng) hoặc "device" (mã thiết bị); grok bỏ qua mode
// vì xAI chỉ có device-code. Một luồng tại một thời điểm toàn app.
func (s *Service) StartLogin(ctx context.Context, providerID, mode string) (*LoginSession, error) {
	p, err := s.provider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if !SubscriptionKind(p.Kind) {
		return nil, apiError(http.StatusNotFound, domain.ReasonNotSubscription,
			"Nhà cung cấp này không đăng nhập")
	}
	switch mode {
	case "", "browser", "device":
	default:
		return nil, apiError(http.StatusBadRequest, domain.ReasonInvalidMode,
			"Cách đăng nhập không hợp lệ")
	}

	if !s.loginMu.TryLock() {
		return nil, apiError(http.StatusConflict, domain.ReasonLoginInProgress,
			"Đang có một luồng đăng nhập chạy")
	}
	oauthID := kindTable[p.Kind].oauthID

	sess := &LoginSession{Phase: LoginExchanging, StartedAt: s.o.Now()}
	manual := make(chan aiconnManualAlias, 1)
	// Luồng sống theo vòng đời service (không phải request HTTP): đóng app
	// huỷ luồng; huỷ riêng request không làm mất đăng nhập đang chạy.
	flowCtx, cancel := context.WithCancel(s.lifetime())
	s.sessMu.Lock()
	s.sess[p.ID] = sess
	s.manuals[p.ID] = manual
	s.cancels[p.ID] = cancel
	s.sessMu.Unlock()
	s.changed(p.ID)

	ctrl := LoginController{
		OnAuth: func(url, _ string) {
			s.setPhase(p.ID, func(ss *LoginSession) {
				ss.Phase = LoginAuthorize
				ss.AuthURL = url
			})
		},
		OnDeviceCodeExpiry: func(expiresAt time.Time) {
			s.setPhase(p.ID, func(ss *LoginSession) { ss.ExpiresAt = expiresAt })
		},
		OnDeviceCode: func(code, verifyURL string) {
			s.setPhase(p.ID, func(ss *LoginSession) {
				ss.Phase = LoginCode
				ss.Code = code
				ss.VerifyURL = verifyURL
				ss.ExpiresAt = time.Time{}
				// Luồng mã thiết bị không có URL "uỷ quyền" riêng; UI mở
				// verifyUrl. Xoá authUrl của callback trước (nếu có) để không
				// hiện hai nút mở cùng một trang.
				ss.AuthURL = ""
			})
		},
		OnProgress: func(msg string) {
			if strings.Contains(msg, "đổi mã") {
				s.setPhase(p.ID, func(ss *LoginSession) { ss.Phase = LoginExchanging })
			}
		},
	}

	s.work.Add(1)
	go func() {
		defer s.work.Done()
		defer s.loginMu.Unlock()
		// Luồng OAuth có thể kéo dài nhiều phút chờ người dùng; timeout 15'
		// để khoá không bị giữ vô hạn khi ai đó bỏ dở.
		fctx, fcancel := context.WithTimeout(flowCtx, 15*time.Minute)
		defer fcancel()
		label, err := s.o.Auth.Login(fctx, oauthID, mode == "device" || kindTable[p.Kind].oauthID == "xai-oauth", ctrl, manual)
		s.sessMu.Lock()
		delete(s.manuals, p.ID)
		delete(s.cancels, p.ID)
		cur := s.sess[p.ID]
		s.sessMu.Unlock()
		switch {
		case err == nil:
			s.setPhase(p.ID, func(ss *LoginSession) {
				ss.Phase = LoginDone
				ss.Account = label
				ss.Error = ""
			})
			_ = s.o.Auth.MarkRevoked(s.lifetime(), oauthID, false)
		case errors.Is(flowCtx.Err(), context.Canceled):
			s.setPhase(p.ID, func(ss *LoginSession) { ss.Phase = LoginCancelled })
		default:
			msg := "Đăng nhập chưa thành công. Thử lại."
			if errors.Is(err, context.DeadlineExceeded) {
				msg = "Hết thời gian chờ đăng nhập. Thử lại."
			}
			s.setPhase(p.ID, func(ss *LoginSession) {
				ss.Phase = LoginFailed
				ss.Error = msg
			})
		}
		if cur != nil {
			s.changed(p.ID)
		}
	}()
	// Trả bản sao dưới cùng khoá với setPhase: goroutine đăng nhập sở hữu con
	// trỏ trong map và sửa nó ngay sau khi trả về.
	s.sessMu.Lock()
	cp := *sess
	s.sessMu.Unlock()
	return &cp, nil
}

// aiconnManualAlias giữ tên kênh rõ nghĩa trong file này.
type aiconnManualAlias = ManualCode

// setPhase đổi trạng thái session và báo thay đổi.
func (s *Service) setPhase(providerID string, fn func(*LoginSession)) {
	s.sessMu.Lock()
	if ss, ok := s.sess[providerID]; ok {
		fn(ss)
	}
	s.sessMu.Unlock()
	s.changed(providerID)
}

// CancelLogin dừng luồng đang chạy của provider (đóng hộp thoại là dừng hỏi).
func (s *Service) CancelLogin(ctx context.Context, providerID string) error {
	s.sessMu.Lock()
	cancel, ok := s.cancels[providerID]
	s.sessMu.Unlock()
	if ok {
		cancel()
		return nil
	}
	// Không có luồng sống: dọn session cũ nếu chủ động gọi.
	s.sessMu.Lock()
	delete(s.sess, providerID)
	s.sessMu.Unlock()
	return nil
}

// SubmitLoginCode đưa một mã dán tay vào luồng đang chờ (luồng trình duyệt
// của chatgpt; khi người dùng đăng nhập trên thiết bị khác mà callback
// localhost không tới được).
func (s *Service) SubmitLoginCode(ctx context.Context, providerID, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return apiError(http.StatusUnprocessableEntity, domain.ReasonInvalidCode, "Mã không hợp lệ")
	}
	s.sessMu.RLock()
	manual, ok := s.manuals[providerID]
	s.sessMu.RUnlock()
	if !ok {
		return apiError(http.StatusConflict, domain.ReasonLoginInProgress,
			"Không có luồng đăng nhập nào đang chờ mã")
	}
	result := make(chan error, 1)
	select {
	case manual <- ManualCode{Input: code, Result: result}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-result:
		if err != nil {
			return apiError(http.StatusUnprocessableEntity, domain.ReasonInvalidCode, "Mã không hợp lệ")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		return apiError(http.StatusUnprocessableEntity, domain.ReasonInvalidCode,
			"Luồng đăng nhập không nhận mã lúc này")
	}
}

// Logout đăng xuất một nhà cung cấp gói: huỷ luồng đang chạy (nếu có), xoá
// credential khỏi kho và xoá ghi nhớ revoked.
func (s *Service) Logout(ctx context.Context, providerID string) error {
	p, err := s.provider(ctx, providerID)
	if err != nil {
		return err
	}
	if !SubscriptionKind(p.Kind) {
		return apiError(http.StatusNotFound, domain.ReasonNotSubscription,
			"Nhà cung cấp này không đăng nhập")
	}
	// Huỷ luồng đang chạy TRƯỚC, rồi chờ nó thả loginMu: nhờ vậy không có
	// khả năng credential vừa xoá lại bị luồng đăng nhập ghi đè ngay sau.
	_ = s.CancelLogin(ctx, providerID)
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	oauthID := kindTable[p.Kind].oauthID
	if err := s.o.Auth.Logout(ctx, oauthID); err != nil {
		return err
	}
	_ = s.o.Auth.MarkRevoked(ctx, oauthID, false)
	s.sessMu.Lock()
	delete(s.sess, providerID)
	s.sessMu.Unlock()
	s.changed(p.ID)
	return nil
}

// ── sẵn sàng & token cho consumer khác ──────────────────────────────────────

// isRevokedErr báo lỗi refresh do credential bị thu hồi (invalid_grant), đối
// lập với lỗi mạng thoáng qua — chỉ lỗi thu hồi mới đổi trạng thái thẻ.
func isRevokedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid_grant") ||
		strings.Contains(msg, "invalid_token") ||
		strings.Contains(msg, "refresh token") && strings.Contains(msg, "revok")
}

// ChatGPTReady báo gói ChatGPT sẵn sàng gọi (credential còn hạn hoặc refresh
// được). Đây là lời gọi CHỦ ĐỘNG (bước setup / test): nó được phép gọi mạng.
// Trả (false, nil) khi chắc chắn chưa/đã mất đăng nhập; lỗi trả ra chỉ khi
// không kết luận được (mạng, kho hỏng) để caller thử lại.
func (s *Service) ChatGPTReady(ctx context.Context) (bool, error) {
	_, _, _, err := s.o.Auth.Credential(ctx, kindTable[domain.KindChatGPT].oauthID)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, errNoCredential):
		return false, nil
	case isRevokedErr(err):
		if mErr := s.o.Auth.MarkRevoked(ctx, kindTable[domain.KindChatGPT].oauthID, true); mErr != nil {
			return false, mErr
		}
		s.changed("chatgpt")
		return false, nil
	}
	return false, err
}

// errNoCredential là sentinel của Auth.Credential khi kho không có credential
// (adapter auth bọc auth.ErrNotLoggedIn thành nó).
var errNoCredential = errors.New("chưa đăng nhập")

// ErrNoCredential xuất sentinel để adapter Auth (aiconnauth) wrap đúng lỗi.
var ErrNoCredential = errNoCredential

// GrokReady tương tự ChatGPTReady cho gói Grok/xAI.
func (s *Service) GrokReady(ctx context.Context) (bool, error) {
	_, _, _, err := s.o.Auth.Credential(ctx, kindTable[domain.KindGrok].oauthID)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, errNoCredential):
		return false, nil
	case isRevokedErr(err):
		if mErr := s.o.Auth.MarkRevoked(ctx, kindTable[domain.KindGrok].oauthID, true); mErr != nil {
			return false, mErr
		}
		s.changed("grok")
		return false, nil
	}
	return false, err
}

// ChatGPTLogin trả kiểu đăng nhập Codex cho tools.CodexStatus (adapter
// toolsvc.Deps.ChatGPT của MachineSetup): "chatgpt" khi có credential,
// "not_logged_in" khi vắng, "unknown" khi không đọc được kho. Thuần local.
func (s *Service) ChatGPTLogin(ctx context.Context) (string, error) {
	lst, err := s.o.Auth.LocalStatus(ctx, kindTable[domain.KindChatGPT].oauthID)
	if err != nil {
		return "unknown", err
	}
	switch lst {
	case OAuthAbsent:
		return "not_logged_in", nil
	case OAuthRevoked:
		return "not_logged_in", nil
	}
	return "chatgpt", nil
}

// GrokLogin trạng thái đăng nhập gói Grok (local): "signed" | "out".
func (s *Service) GrokLogin(ctx context.Context) (string, error) {
	lst, err := s.o.Auth.LocalStatus(ctx, kindTable[domain.KindGrok].oauthID)
	if err != nil {
		return "out", err
	}
	if lst == OAuthPresent {
		return "signed", nil
	}
	return "out", nil
}

// CLITokens trả (access token tươi, account id) cho adapter dựng auth.json
// của CODEX_HOME cô lập — điểm duy nhất Codex CLI nhận token; refresh token
// không bao giờ rời core/auth.
func (s *Service) CLITokens(ctx context.Context) (access, accountID string, err error) {
	access, accountID, err = s.o.Auth.CLITokens(ctx, kindTable[domain.KindChatGPT].oauthID)
	if err != nil {
		if errors.Is(err, errNoCredential) {
			return "", "", fmt.Errorf("chưa đăng nhập ChatGPT: %w", err)
		}
		return "", "", err
	}
	return access, accountID, nil
}
