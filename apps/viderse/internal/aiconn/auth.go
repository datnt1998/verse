package aiconn

// CoreAuth là hiện thực Auth trên core/auth: kho credential auth.json của
// app, Manager là chủ refresh duy nhất. Ghi nhớ "revoked" nằm trong bảng
// settings (ai.revoked.<oauthID>) để GET đọc được mà không cần gọi mạng.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datnt1998/verse/core/auth"
)

// CoreAuth bọc kho credential + manager của core/auth.
type CoreAuth struct {
	mgr *auth.Manager
	// revoked đọc/ghi cờ thu hồi theo oauthID (settings store).
	revoked RevokedStore
}

// RevokedStore là phần khoá/giá trị giữ cờ "credential bị thu hồi".
type RevokedStore interface {
	Get(ctx context.Context, key string, out any) (bool, error)
	Set(ctx context.Context, key string, value any) error
}

// revokedKey là khoá settings của cờ thu hồi của một credential.
func revokedKey(oauthID string) string { return "ai.revoked." + oauthID }

// NewCoreAuth dựng Auth trên kho credential của app.
func NewCoreAuth(mgr *auth.Manager, settings RevokedStore) *CoreAuth {
	return &CoreAuth{mgr: mgr, revoked: settings}
}

// NewCoreAuthDefault mở kho credential mặc định của app (~/.viderse/auth.json)
// rồi dựng Auth — tiện cho composition root; lỗi khi chưa Configure.
func NewCoreAuthDefault(settings RevokedStore) (*CoreAuth, error) {
	store, err := auth.DefaultStore()
	if err != nil {
		return nil, err
	}
	return NewCoreAuth(auth.NewManager(store), settings), nil
}

// Manager trả auth.Manager bên dưới — composition root dùng cho adapter khác
// (xai.ManagerTokens cho imagegen, codex.ManagerTokens cho LLM sau này).
func (a *CoreAuth) Manager() *auth.Manager { return a.mgr }

// LocalStatus đọc credential từ kho không gọi mạng.
func (a *CoreAuth) LocalStatus(ctx context.Context, oauthID string) (OAuthStatusLocal, error) {
	cred, ok, err := a.mgr.Store().Get(oauthID)
	if err != nil {
		return OAuthAbsent, fmt.Errorf("đọc kho đăng nhập: %w", err)
	}
	if !ok || cred.Access == "" {
		return OAuthAbsent, nil
	}
	var revoked bool
	if _, err := a.revoked.Get(ctx, revokedKey(oauthID), &revoked); err != nil {
		return OAuthAbsent, err
	}
	if revoked {
		return OAuthRevoked, nil
	}
	return OAuthPresent, nil
}

// Plan trả tên gói đã lưu của credential (chatgpt: chatgpt_plan_type trong
// id_token, ghi lúc login/refresh — openai_codex.go). Chỉ đọc kho local.
func (a *CoreAuth) Plan(_ context.Context, oauthID string) (string, error) {
	cred, ok, err := a.mgr.Store().Get(oauthID)
	if err != nil || !ok {
		return "", err
	}
	return cred.Plan, nil
}

// Credential trả credential còn hạn; refresh thất bại vì thu hồi được đánh
// dấu để lần sau đọc local.
func (a *CoreAuth) Credential(ctx context.Context, oauthID string) (access, accountID, label string, err error) {
	cred, err := a.mgr.Credential(ctx, oauthID)
	if err != nil {
		if errors.Is(err, auth.ErrNotLoggedIn) {
			return "", "", "", errNoCredential
		}
		return "", "", "", err
	}
	return cred.Access, cred.AccountID, cred.Label(), nil
}

// Login chạy luồng OAuth của provider và lưu credential vào kho. device
// chọn luồng mã thiết bị (grok luôn device vì xAI không có callback).
//
// Cầu mã dán tay: goroutine chuyển tiếp có context RIÊNG, luôn bị huỷ và
// được join trước khi trả về — core/auth không chờ vô hạn khi luồng kết
// thúc sớm (thành công, lỗi sớm, hoặc mã bị từ chối).
func (a *CoreAuth) Login(ctx context.Context, oauthID string, device bool, ctrl LoginController, manual <-chan ManualCode) (string, error) {
	bctx, bcancel := context.WithCancel(ctx)
	defer bcancel()

	bridge := make(chan auth.ManualSubmission, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case code, ok := <-manual:
				if !ok {
					return
				}
				res := make(chan error, 1)
				select {
				case bridge <- auth.ManualSubmission{Input: code.Input, Result: res}:
				case <-bctx.Done():
					return
				}
				// Chờ kết quả CÓ huỷ: provider không nhận mã (vd. luồng chỉ
				// callback) thì ta không treo ở đây.
				var err error
				select {
				case err = <-res:
				case <-bctx.Done():
					err = bctx.Err()
				}
				select {
				case code.Result <- err:
				case <-bctx.Done():
				}
			case <-bctx.Done():
				return
			}
		}
	}()

	cred, err := auth.Login(ctx, a.mgr.Store(), oauthID, auth.Controller{
		OnAuth:             ctrl.OnAuth,
		OnDeviceCode:       ctrl.OnDeviceCode,
		OnDeviceCodeExpiry: ctrl.OnDeviceCodeExpiry,
		OnProgress:         ctrl.OnProgress,
	}, auth.LoginOptions{
		Device: device,
		From:   "web",
		Manual: bridge,
	})
	// Huỷ cầu TRƯỚC khi join: core/auth trả về nghĩa là lượt đăng nhập đã
	// kết thúc, cầu không còn việc.
	bcancel()
	<-done
	if err != nil {
		return "", err
	}
	return cred.Label(), nil
}

// MarkRevoked ghi/xoá cờ thu hồi của credential để GET sau đọc local.
func (a *CoreAuth) MarkRevoked(ctx context.Context, oauthID string, revoked bool) error {
	return a.revoked.Set(ctx, revokedKey(oauthID), revoked)
}

// Logout xoá credential của provider khỏi kho.
func (a *CoreAuth) Logout(ctx context.Context, oauthID string) error {
	if _, err := a.mgr.Store().Delete(oauthID); err != nil {
		return fmt.Errorf("xoá credential: %w", err)
	}
	return nil
}

// Một lượt Codex CLI có thể chạy tới 20 phút (imagegen chặn timeout ở 20'),
// CLI tự refresh khi access token còn ≤5 phút, và bước chuẩn bị (probe, ghi
// home, dựng catalog) tốn thêm chút thời gian. auth.json cô lập cố tình
// KHÔNG có refresh token nên lần refresh đó sẽ thất bại ⇒ phải còn dư cả
// quãng 20' + 5' + 1' trước khi giao token cho CLI.
const (
	cliMaxCall         = 20 * time.Minute
	cliProactiveWindow = 5 * time.Minute
	cliPrepareSlack    = time.Minute
	cliRefreshWindow   = cliMaxCall + cliProactiveWindow + cliPrepareSlack
)

// needsCLIRefresh báo access token còn quá ít thời gian cho một lượt CLI.
func needsCLIRefresh(cred auth.Credential, now time.Time) bool {
	if cred.Expires == 0 {
		return false
	}
	return cred.ExpiresAt().Sub(now) < cliRefreshWindow
}

// safeForCLICall báo token đủ thời gian cho TRỌN một lượt CLI (gọi + cửa sổ
// refresh chủ động + chuẩn bị). Token không khai hạn (Expires == 0) được
// chấp nhận: không có gì để kiểm.
func safeForCLICall(cred auth.Credential, now time.Time) bool {
	if cred.Expires == 0 {
		return true
	}
	return cred.ExpiresAt().Sub(now) >= cliRefreshWindow
}

// CLITokens trả access token tươi + account id để dựng auth.json cô lập —
// refresh token ở lại trong core/auth, không bao giờ ra ngoài. Token sắp hết
// thì BUỘC refresh; refresh hỏng hoặc token vẫn không đủ thời gian cho một
// lượt CLI là LỖI (không bao giờ phát hành auth.json mà CLI sẽ tự refresh
// rồi thất bại giữa lượt vẽ).
func (a *CoreAuth) CLITokens(ctx context.Context, oauthID string) (access, accountID string, err error) {
	cred, err := a.mgr.Credential(ctx, oauthID)
	if err != nil {
		if errors.Is(err, auth.ErrNotLoggedIn) {
			return "", "", errNoCredential
		}
		return "", "", err
	}
	if needsCLIRefresh(cred, time.Now()) {
		// Invalidate chỉ refresh khi token trong kho vẫn là token ta vừa đọc
		// (tiến trình khác refresh trước thì dùng luôn bản mới).
		fresh, rerr := a.mgr.Invalidate(ctx, oauthID, cred.Access)
		if rerr != nil {
			return "", "", fmt.Errorf("không làm mới được phiên ChatGPT đủ cho một lượt Codex: %w", rerr)
		}
		cred = fresh
	}
	if !safeForCLICall(cred, time.Now()) {
		return "", "", fmt.Errorf("phiên ChatGPT còn quá ít thời gian cho một lượt Codex — hãy đăng nhập lại")
	}
	return cred.Access, cred.AccountID, nil
}
