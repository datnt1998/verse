package web

// HTTP của mục Cài đặt › Tài khoản nền tảng (V23) và đăng nhập Bilibili bằng
// mã QR (V24). Handler chỉ đọc yêu cầu → gọi service → ghi JSON; hình wire
// theo hợp đồng dữ liệu §24 / §24.1, tách hẳn khỏi kiểu domain.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// PlatformService là mặt nghiệp vụ web cần — hiện thực là platforms.Service.
type PlatformService interface {
	Status(ctx context.Context) ([]domain.PlatformStatus, error)
	Profiles(ctx context.Context, id domain.PlatformID, refresh bool) ([]domain.BrowserProfile, error)
	SetLogin(ctx context.Context, id domain.PlatformID, in platforms.LoginInput) (domain.PlatformLogin, error)
	ClearLogin(ctx context.Context, id domain.PlatformID) (domain.PlatformLogin, error)
	Check(ctx context.Context, id domain.PlatformID) (domain.PlatformLogin, error)
	StartQR(ctx context.Context, id domain.PlatformID) (domain.QRSession, error)
	PollQR(ctx context.Context, id domain.PlatformID, key string) (domain.QRPoll, error)
	Refresh(ctx context.Context, id domain.PlatformID) (domain.RefreshOutcome, domain.PlatformLogin, error)
}

// PlatformDiag là điểm tới của lỗi nội bộ 500: chỉ nguyên nhân đã che bí mật
// mới đi vào nhật ký chẩn đoán — API không bao giờ trả nguyên nhân thô.
type PlatformDiag func(msg string)

// MountPlatforms đăng ký các route /api/platforms* trên gateway, không ghi
// nhật ký chẩn đoán — mount qua MountPlatformsDiag trong môi trường thật.
func MountPlatforms(g *Gateway, svc PlatformService) {
	MountPlatformsDiag(g, svc, nil)
}

// MountPlatformsDiag giống MountPlatforms nhưng có điểm tới cho lỗi nội bộ:
// diag nhận bản sao đã qua logs.Redact của nguyên nhân lỗi 500.
func MountPlatformsDiag(g *Gateway, svc PlatformService, diag PlatformDiag) {
	g.Handle(http.MethodGet, "/api/platforms", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		listPlatforms(w, r, svc, diag)
	})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		m := method
		g.Handle(m, "/api/platforms/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
			routePlatforms(w, r, svc, diag)
		})
	}
}

// ── wire ───────────────────────────────────────────────────────────────────

type platformStatusWire struct {
	ID           string                 `json:"id"`
	Label        string                 `json:"label"`
	Uses         []string               `json:"uses"`
	Kinds        []string               `json:"kinds"`
	TargetHeight int                    `json:"targetHeight"`
	Login        platformLoginWire      `json:"login"`
	State        string                 `json:"state"`
	PausedUntil  *time.Time             `json:"pausedUntil,omitempty"`
	PauseReason  *string                `json:"pauseReason,omitempty"`
	FailureCount int                    `json:"failureCount,omitempty"`
	LastError    *platformLastErrorWire `json:"lastError,omitempty"`
	Usage        platformUsageWire      `json:"usage"`
}

type platformLoginWire struct {
	Type           string     `json:"type"`
	Saved          bool       `json:"saved"`
	Masked         *string    `json:"masked,omitempty"`
	Account        *string    `json:"account,omitempty"`
	BrowserProfile string     `json:"browserProfile,omitempty"`
	Method         *string    `json:"method"`
	Renewable      bool       `json:"renewable"`
	LastRefreshAt  *time.Time `json:"lastRefreshAt,omitempty"`
	RefreshError   *string    `json:"refreshError,omitempty"`
}

type platformLastErrorWire struct {
	Code  string  `json:"code"`
	At    string  `json:"at"`
	JobID *string `json:"jobId,omitempty"`
}

type platformUsageWire struct {
	Channels int  `json:"channels"`
	Videos   int  `json:"videos"`
	Rivals   *int `json:"rivals,omitempty"`
}

type profileWire struct {
	Browser string `json:"browser"`
	Profile string `json:"profile"`
	Name    string `json:"name"`
	Email   string `json:"email,omitempty"`
}

type qrSessionWire struct {
	QRCodeKey string    `json:"qrcodeKey"`
	QRImage   string    `json:"qrImage"`
	LoginURL  string    `json:"loginUrl"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type qrPollWire struct {
	Status  string  `json:"status"`
	Account *string `json:"account,omitempty"`
}

type refreshWire struct {
	platformLoginWire
	Status string `json:"status"`
}

func loginToWire(l domain.PlatformLogin) platformLoginWire {
	out := platformLoginWire{
		Type:          l.Type,
		Saved:         l.Saved,
		Masked:        l.Masked,
		Account:       l.Account,
		Renewable:     l.Renewable,
		LastRefreshAt: l.LastRefreshAt,
		RefreshError:  l.RefreshError,
	}
	if l.Browser != "" && l.Profile != "" {
		out.BrowserProfile = l.Browser + ":" + l.Profile
	}
	if l.Method != "" {
		method := l.Method
		out.Method = &method
	}
	return out
}

// ── handlers ───────────────────────────────────────────────────────────────

func listPlatforms(w http.ResponseWriter, r *http.Request, svc PlatformService, diag PlatformDiag) {
	list, err := svc.Status(r.Context())
	if err != nil {
		writeInternalError(w, err, diag)
		return
	}
	out := make([]platformStatusWire, 0, len(list))
	for _, st := range list {
		usage := platformUsageWire{Channels: st.Usage.Channels, Videos: st.Usage.Videos, Rivals: st.Usage.Rivals}
		wire := platformStatusWire{
			ID:           st.ID,
			Label:        st.Label,
			Uses:         st.Uses,
			Kinds:        st.Kinds,
			TargetHeight: st.TargetHeight,
			Login:        loginToWire(st.Login),
			State:        st.State,
			PausedUntil:  st.PausedUntil,
			PauseReason:  st.PauseReason,
			FailureCount: st.FailureCount,
			Usage:        usage,
		}
		if st.LastError != nil {
			wire.LastError = &platformLastErrorWire{
				Code:  st.LastError.Code,
				At:    st.LastError.At.Format(time.RFC3339),
				JobID: st.LastError.JobID,
			}
		}
		out = append(out, wire)
	}
	webapp.WriteJSON(w, http.StatusOK, out)
}

// routePlatforms khớp phần sau /api/platforms/ theo hình
// {id}/login[/profiles|/check|/refresh|/qr[/{key}]].
func routePlatforms(w http.ResponseWriter, r *http.Request, svc PlatformService, diag PlatformDiag) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/platforms/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "login" {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	id := parts[0]
	tail := parts[2:]
	ctx := r.Context()
	switch {
	case len(tail) == 0 && r.Method == http.MethodPut:
		putLogin(w, r, svc, diag, id)
	case len(tail) == 0 && r.Method == http.MethodDelete:
		login, err := svc.ClearLogin(ctx, id)
		if err != nil {
			writePlatformError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, loginToWire(login))
	case len(tail) == 1 && tail[0] == "profiles" && r.Method == http.MethodGet:
		refresh := r.URL.Query().Get("refresh") == "1"
		profiles, err := svc.Profiles(ctx, id, refresh)
		if err != nil {
			writePlatformError(w, err, diag)
			return
		}
		out := make([]profileWire, 0, len(profiles))
		for _, p := range profiles {
			out = append(out, profileWire{Browser: p.Browser, Profile: p.Profile, Name: p.Name, Email: p.Email})
		}
		webapp.WriteJSON(w, http.StatusOK, out)
	case len(tail) == 1 && tail[0] == "check" && r.Method == http.MethodPost:
		login, err := svc.Check(ctx, id)
		if err != nil {
			writePlatformError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, loginToWire(login))
	case len(tail) == 1 && tail[0] == "refresh" && r.Method == http.MethodPost:
		outcome, login, err := svc.Refresh(ctx, id)
		if err != nil {
			writePlatformError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, refreshWire{platformLoginWire: loginToWire(login), Status: outcome})
	case len(tail) == 1 && tail[0] == "qr" && r.Method == http.MethodPost:
		session, err := svc.StartQR(ctx, id)
		if err != nil {
			writePlatformError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, qrSessionWire{
			QRCodeKey: session.QRCodeKey,
			QRImage:   session.QRImage,
			LoginURL:  session.LoginURL,
			ExpiresAt: session.ExpiresAt,
		})
	case len(tail) == 2 && tail[0] == "qr" && r.Method == http.MethodGet:
		poll, err := svc.PollQR(ctx, id, tail[1])
		if err != nil {
			writePlatformError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, qrPollWire{Status: poll.Status, Account: poll.Account})
	default:
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
	}
}

// loginBody là body của PUT /api/platforms/:id/login — một trong hai hình.
type loginBody struct {
	Cookie  string `json:"cookie"`
	Browser string `json:"browser"`
	Profile string `json:"profile"`
}

func putLogin(w http.ResponseWriter, r *http.Request, svc PlatformService, diag PlatformDiag, id string) {
	var body loginBody
	if !webapp.DecodeJSON(w, r, &body) {
		return
	}
	login, err := svc.SetLogin(r.Context(), id, platforms.LoginInput{
		Cookie:  body.Cookie,
		Browser: body.Browser,
		Profile: body.Profile,
	})
	if err != nil {
		writePlatformError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, loginToWire(login))
}

// loginReason là interface mà accounts.LoginError hiện thực — web map sang
// status/`reason` mà không phải import gói accounts.
type loginReason interface {
	LoginReason() string
}

// writePlatformError map lỗi nghiệp vụ sang status + `reason` ổn định theo
// hợp đồng: 404 nền tảng lạ, 400 body sai/không áp dụng, 409 đang bận,
// 422 nền tảng từ chối, 502 không kết nối được. Lỗi ngoài tập đó là lỗi nội
// bộ: trả vỏ trung lập và chuyển nguyên nhân cho nhật ký chẩn đoán đã che.
func writePlatformError(w http.ResponseWriter, err error, diag PlatformDiag) {
	switch {
	case errors.Is(err, platforms.ErrPlatformUnknown):
		webapp.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "Nền tảng không tồn tại."})
		return
	case errors.Is(err, platforms.ErrInvalidLoginBody):
		webapp.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "Thông tin đăng nhập không hợp lệ.", "reason": "invalid"})
		return
	case errors.Is(err, platforms.ErrLoginUnsupported):
		webapp.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "Nền tảng này không có thao tác đăng nhập này.", "reason": "unsupported"})
		return
	}
	var coded loginReason
	if errors.As(err, &coded) {
		status := http.StatusUnprocessableEntity
		switch coded.LoginReason() {
		case "invalid":
			status = http.StatusBadRequest
		case "available-unused":
			status = http.StatusBadGateway
		case "unavailable":
			status = http.StatusBadGateway
		case "busy":
			status = http.StatusConflict
		}
		webapp.WriteJSON(w, status, map[string]any{"error": err.Error(), "reason": coded.LoginReason()})
		return
	}
	writeInternalError(w, err, diag)
}

// writeInternalError trả vỏ 500 trung lập — nguyên nhân (đường dẫn, DSN,
// driver, token lẫn trong lỗi) chỉ đi tới diag thay vì ra JSON.
func writeInternalError(w http.ResponseWriter, err error, diag PlatformDiag) {
	if diag != nil && err != nil {
		diag(err.Error())
	}
	webapp.Error(w, http.StatusInternalServerError, "Có lỗi nội bộ xảy ra. Thử lại sau; lỗi đã được ghi nhật ký.")
}
