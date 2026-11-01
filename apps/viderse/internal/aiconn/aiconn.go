// Package aiconn quản lý kết nối AI của Viderse (hợp đồng dữ liệu §15): các
// nhà cung cấp model (đăng nhập gói, khoá API, chạy trên máy), danh mục model
// đã lấy và lưu trong Postgres, và gán model cho trợ lý (§15 AgentModels).
//
// Luật cứng:
//   - Khoá API không bao giờ ở dạng plaintext: secretbox niêm phong trước khi
//     vào DB; API chỉ trả keyHint.
//   - GET không gọi mạng: trạng thái suy từ store/auth.json local, danh mục
//     đọc từ DB. Chỉ POST/PATCH/test/models/check/refresh mới chạm provider.
//   - Một luồng đăng nhập chạy tại một thời điểm (khoá loginMu).
//   - core/auth là chủ token duy nhất cho gói đăng nhập (refresh + xoay vòng).
package aiconn

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
)

// ── Lỗi có mã reason ─────────────────────────────────────────────────────────

// Error là lỗi nghiệp vụ có mã reason ổn định cho API (§15 + mã mở rộng).
type Error struct {
	Status  int    // HTTP status gợi ý (400/409/422)
	Reason  string // domain.Reason*
	Message string // câu tiếng Việt cho trường "error"
	Fields  map[string]any
}

func (e *Error) Error() string { return e.Message }

// apiError dựng một *Error.
func apiError(status int, reason, message string) *Error {
	return &Error{Status: status, Reason: reason, Message: message}
}

// AsAPIError đọc *Error từ một lỗi bất kỳ.
func AsAPIError(err error) (*Error, bool) {
	var ae *Error
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// ── Kiểu phụ thuộc (khai báo phía consumer) ─────────────────────────────────

// ProviderSummary là một dòng ai_providers kèm số model đã lưu — dùng cho
// GET danh sách mà không cần tải cả danh mục (tránh N+1).
type ProviderSummary struct {
	Provider domain.Provider
	Count    int
}

// ModelQuery là một trang danh mục của provider (lọc trong SQL).
type ModelQuery struct {
	Q      string   // chuỗi con của model id (không phân biệt hoa thường)
	Filter string   // "" | "all" | "selected" | "recommended"
	After  string   // con trỏ: chỉ model có id > After
	Limit  int      // tối đa một trang (store tự cắt nếu lớn hơn)
	OnlyID []string // filter "selected": chỉ các id này (rỗng = không lọc)
}

// ModelPage là một trang kết quả danh mục.
type ModelPage struct {
	Models  []domain.AIModel
	Total   int  // tổng khớp bộ lọc (không tính cursor/limit)
	HasMore bool // còn trang sau
}

// Store là phần persistence aiconn cần (postgres.AIConnStore thoả mãn).
type Store interface {
	// ListProviders trả mọi thẻ kèm số model trong MỘT truy vấn.
	ListProviders(ctx context.Context) ([]ProviderSummary, error)
	// ListSubscribed chỉ trả các thẻ đăng nhập gói + số model (đường đọc của
	// attention/status — giới hạn nhỏ, không tải provider khoá API).
	ListSubscribed(ctx context.Context) ([]ProviderSummary, error)
	GetProvider(ctx context.Context, id string) (domain.Provider, error)
	// UpsertProvider thêm một dòng provider mới.
	UpsertProvider(ctx context.Context, p domain.Provider) error
	// UpdateProviderKey chỉ đổi khoá/địa chỉ/trạng thái — KHÔNG đụng lựa
	// chọn model (PATCH khoá song song không được ghi đè lựa chọn mới).
	UpdateProviderKey(ctx context.Context, id string, encKey []byte, keyHint, baseURL string, status domain.AIStatus) error
	SetSelected(ctx context.Context, id string, selected []string) error
	SetStatus(ctx context.Context, id string, st domain.AIStatus) error
	DeleteProvider(ctx context.Context, id string) error
	ListModels(ctx context.Context, providerID string) ([]domain.AIModel, error)
	// SearchModels trả một trang danh mục, lọc/sắp/cắt trong SQL.
	SearchModels(ctx context.Context, providerID string, q ModelQuery) (ModelPage, error)
	// CountModels đếm model khớp bộ lọc (cho tổng của trang).
	CountModels(ctx context.Context, providerID string, q ModelQuery) (int, error)
	GetModel(ctx context.Context, providerID, modelID string) (domain.AIModel, bool, error)
	// ReplaceCatalog đổi toàn bộ danh mục tự động của provider (giữ model
	// manual) trong một transaction và đặt catalog_at = at.
	ReplaceCatalog(ctx context.Context, providerID string, models []domain.AIModel, at time.Time) error
	UpsertModel(ctx context.Context, providerID string, m domain.AIModel) error
}

// Settings là phần khoá/giá trị aiconn cần (postgres.SettingsStore thoả mãn).
type Settings interface {
	Get(ctx context.Context, key string, out any) (bool, error)
	Set(ctx context.Context, key string, value any) error
}

// Box là lớp niêm phong bí mật (secretbox.Box thoả mãn).
type Box interface {
	Seal(plaintext []byte) ([]byte, error)
	Reveal(sealed []byte) ([]byte, error)
}

// OAuthStatusLocal là trạng thái credential đọc local, không refresh.
type OAuthStatusLocal int

const (
	OAuthAbsent  OAuthStatusLocal = iota // chưa đăng nhập
	OAuthPresent                         // có credential trong kho
	OAuthRevoked                         // credential tồn tại nhưng refresh bị từ chối (đã ghi nhớ)
)

// Auth là phần core/auth aiconn cần: một kho OAuth, một chủ token.
// Credential(id) refresh khi cần (có thể gọi mạng — chỉ dùng ngoài GET).
type Auth interface {
	// LocalStatus đọc trạng thái credential của oauthID không gọi mạng.
	LocalStatus(ctx context.Context, oauthID string) (OAuthStatusLocal, error)
	// Credential trả credential còn hạn (refresh khi sắp hết hạn); khi refresh
	// bị từ chối (invalid_grant) service đánh dấu revoked để đọc sau không cần
	// gọi lại mạng.
	Credential(ctx context.Context, oauthID string) (access, accountID, label string, err error)
	// MarkRevoked ghi/làm mất ghi nhớ "credential đã bị thu hồi" cho oauthID.
	MarkRevoked(ctx context.Context, oauthID string, revoked bool) error
	// Login chạy luồng OAuth của provider và lưu credential mới; device chọn
	// luồng mã thiết bị.
	Login(ctx context.Context, oauthID string, device bool, ctrl LoginController, manual <-chan ManualCode) (label string, err error)
	// Logout xoá credential của oauthID.
	Logout(ctx context.Context, oauthID string) error
	// CLITokens trả bộ token tươi nhất để dựng auth.json cho Codex CLI cô lập
	// (refresh bên trong; không bao giờ trả refresh token ra ngoài).
	CLITokens(ctx context.Context, oauthID string) (access, accountID string, err error)
}

// LoginController là tập callback của luồng đăng nhập OAuth — mirror
// auth.Controller nhưng do consumer khai để không import core/auth.
type LoginController struct {
	OnAuth             func(url, instructions string)
	OnDeviceCode       func(userCode, verifyURL string)
	OnDeviceCodeExpiry func(expiresAt time.Time)
	OnProgress         func(message string)
}

// ManualCode là một lần dán mã của người dùng (mirror auth.ManualSubmission).
type ManualCode struct {
	Input  string
	Result chan<- error
}

// Adapter biết nói chuyện với một loại nhà cung cấp: lấy danh mục, thử kết
// nối, gọi ngắn một model. Mọi adapter được test bằng httptest.
type Adapter interface {
	// FetchCatalog lấy danh sách model sống từ provider. Trả *CatalogError để
	// phân loại auth/unreachable; các lỗi khác coi là lỗi gọi thường.
	FetchCatalog(ctx context.Context, p domain.Provider, apiKey string) ([]domain.AIModel, error)
	// CheckModel gọi một lượt ngắn tới model (hoặc kiểm tra metadata miễn phí
	// tương đương với provider cục bộ/đăng nhập) và trả caps+context đo được.
	CheckModel(ctx context.Context, p domain.Provider, apiKey, model string) (domain.AIModel, error)
}

// CatalogError phân loại lỗi gọi provider: key bị từ chối hay không kết nối
// được — quyết trạng thái thẻ (auth_failed / unreachable) và mã reason API.
type CatalogError struct {
	Kind    string // "auth" | "unreachable" | "failed"
	Message string
	Status  int // HTTP status của provider khi có (vd. 401)
}

func (e *CatalogError) Error() string { return e.Message }

// IsAuthError báo lỗi là từ chối xác thực (401/403 cụ thể).
func IsAuthError(err error) bool {
	var ce *CatalogError
	return errors.As(err, &ce) && ce.Kind == "auth"
}

// IsUnreachable báo lỗi là không kết nối được provider.
func IsUnreachable(err error) bool {
	var ce *CatalogError
	return errors.As(err, &ce) && ce.Kind == "unreachable"
}

// PaidUsage đếm ảnh trả phí đã charge trong ngày cho một provider imagegen —
// nguồn duy nhất của trường quota (không có API đọc hạn mức subscription).
type PaidUsage func(ctx context.Context, receiptName string, since time.Time) (int64, error)

// Executors là cổng aiconn hỏi composition root: ModelRef này có đường thực
// thi thật không. Composition root (cmd) buộc imagegen/core-llm; aiconn chỉ
// cần biết "có/không" để từ chối cấu hình không chạy được lúc PUT agents.
type Executors interface {
	// CanText báo ModelRef gọi text được (kind có client text + credential).
	CanText(ctx context.Context, ref domain.ModelRef) bool
	// CanDraw báo ModelRef vẽ ảnh được (kind + model có đường vẽ thật).
	CanDraw(ctx context.Context, ref domain.ModelRef) bool
}

// Options là phụ thuộc tiêm vào của Service.
type Options struct {
	Store    Store
	Settings Settings
	Box      Box
	Auth     Auth
	// Adapters: kind → adapter. Kind nào thiếu adapter thì POST của nó trả
	// lỗi cấu hình; chatgpt/grok vẫn có luồng login qua Auth.
	Adapters map[domain.ProviderKind]Adapter
	// PaidUsage đếm ảnh đã charge trong ngày (nil → quota không hiện).
	PaidUsage PaidUsage
	// DailyImageLimit: trần ảnh trả phí ngày của quota (≤0 → 50).
	DailyImageLimit int
	// QuotaReset trả mốc reset THẬT do provider báo (imagegen ghi nhớ
	// resets_at/retry_at khi provider hết hạn mức); false → dùng nửa đêm.
	QuotaReset func(provider string) (time.Time, bool)
	// Notify phát sự kiện "ai" (SSE) sau mỗi thay đổi; tham số là ref
	// (provider id hoặc "agents"). nil → không phát.
	Notify func(ref string)
	// StatusBump yêu cầu render lại /api/status (attention đổi). nil → bỏ.
	StatusBump func()
	// OnAgentsChanged chạy sau khi gán model trợ lý đổi (để composition root
	// dựng lại chuỗi fallback của imagegen). nil → bỏ.
	OnAgentsChanged func()
	// Now cho test; nil → time.Now.
	Now func() time.Time
	// ModelRegistry tra metadata (context, giá) để làm giàu danh mục —
	// nil → models.DefaultRegistry của core.
	ModelRegistry ModelLookup
	// Lifetime: vòng đời serve — login nền và warm catalog bám theo nó;
	// hủy nó là hủy mọi luồng đăng nhập/việc nền còn treo. nil → Background.
	Lifetime context.Context
	// Executors kiểm tra ModelRef có đường chạy thật (imagegen/core-llm do
	// composition root buộc); nil → PUT agents chỉ kiểm capability, không
	// kiểm được đường chạy.
	Executors Executors
	// Hosted: máy chủ được phục vụ cho người khác (PUBLIC_URL). Địa chỉ tuỳ
	// biến (ollama, openai_compat) phải phân giải ra địa chỉ công khai — không
	// loopback, link-local hay mạng riêng. Tắt → app một chủ trên máy cá nhân
	// giữ được Ollama ở localhost/LAN.
	Hosted bool
	// LookupIP phân giải tên host cho kiểm tra Hosted; nil → resolver hệ thống.
	LookupIP LookupIPFunc
}

// Service là mặt tiền nghiệp vụ của kết nối AI.
type Service struct {
	o Options

	loginMu sync.Mutex // một luồng đăng nhập toàn app tại một thời điểm
	cfgMu   sync.Mutex // tuần tự PATCH/PUT agents/DELETE: ranh giới in_use + credentials
	sessMu  sync.RWMutex
	sess    map[string]*LoginSession
	manuals map[string]chan ManualCode
	cancels map[string]context.CancelFunc
	work    sync.WaitGroup // login goroutines + warm catalog — join khi Close
}

// New dựng service trên các phụ thuộc đã tiêm.
func New(o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Lifetime == nil {
		o.Lifetime = context.Background()
	}
	if o.DailyImageLimit <= 0 {
		o.DailyImageLimit = 50
	}
	if o.Adapters == nil {
		o.Adapters = map[domain.ProviderKind]Adapter{}
	}
	return &Service{
		o:       o,
		sess:    map[string]*LoginSession{},
		manuals: map[string]chan ManualCode{},
		cancels: map[string]context.CancelFunc{},
	}
}

// lifetime trả context vòng đời của service (login/warm bám theo nó).
func (s *Service) lifetime() context.Context { return s.o.Lifetime }

// Go chạy fn trong một goroutine thuộc vòng đời service (tracked bởi Close).
func (s *Service) Go(fn func(context.Context)) {
	s.work.Add(1)
	go func() {
		defer s.work.Done()
		fn(s.lifetime())
	}()
}

// Close hủy mọi luồng đăng nhập đang chạy rồi chờ chúng kết thúc — gọi khi
// serve dừng (lifetime context đã hủy). Idempotent.
func (s *Service) Close() {
	s.sessMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.cancels))
	for _, c := range s.cancels {
		cancels = append(cancels, c)
	}
	s.sessMu.Unlock()
	for _, c := range cancels {
		c()
	}
	s.work.Wait()
}

// ── kind metadata ───────────────────────────────────────────────────────────

// kindInfo là siêu dữ liệu tĩnh của một loại kết nối.
type kindInfo struct {
	auth      domain.AIAuth
	label     string
	needsKey  bool
	needsURL  bool // baseUrl bắt buộc
	customURL bool // baseUrl được phép tuỳ biến (local/server riêng)
	oauthID   string
	defURL    string
}

var kindTable = map[domain.ProviderKind]kindInfo{
	domain.KindChatGPT:      {auth: domain.AuthSubscription, label: "ChatGPT", oauthID: "openai-codex"},
	domain.KindGrok:         {auth: domain.AuthSubscription, label: "Grok", oauthID: "xai-oauth"},
	domain.KindOpenAI:       {auth: domain.AuthAPIKey, label: "OpenAI", needsKey: true, defURL: "https://api.openai.com/v1"},
	domain.KindGemini:       {auth: domain.AuthAPIKey, label: "Gemini", needsKey: true, defURL: "https://generativelanguage.googleapis.com"},
	domain.KindOpenRouter:   {auth: domain.AuthAPIKey, label: "OpenRouter", needsKey: true, defURL: "https://openrouter.ai/api/v1"},
	domain.KindDeepSeek:     {auth: domain.AuthAPIKey, label: "DeepSeek", needsKey: true, defURL: "https://api.deepseek.com"},
	domain.KindXAI:          {auth: domain.AuthAPIKey, label: "xAI", needsKey: true, defURL: "https://api.x.ai/v1"},
	domain.KindAnthropic:    {auth: domain.AuthAPIKey, label: "Anthropic", needsKey: true, defURL: "https://api.anthropic.com"},
	domain.KindOllama:       {auth: domain.AuthLocal, label: "Ollama", defURL: "http://127.0.0.1:11434", customURL: true},
	domain.KindOpenAICompat: {auth: domain.AuthAPIKey, label: "Tương thích OpenAI", needsKey: true, needsURL: true, customURL: true},
}

// validateBaseURL chuẩn hoá baseUrl của provider: chỉ http/https tuyệt đối,
// không userinfo (không cho credential nằm trong URL), không query/fragment
// (không cho khoá API trên query). Localhost/LAN/http được giữ — app local
// một chủ cần gọi Ollama và server tuỳ biến của chính chủ.
func validateBaseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return "", errors.New("Địa chỉ máy chủ không hợp lệ")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("Địa chỉ máy chủ phải là http hoặc https")
	}
	if u.Host == "" {
		return "", errors.New("Địa chỉ máy chủ thiếu host")
	}
	if u.User != nil {
		return "", errors.New("Địa chỉ máy chủ không được chứa thông tin đăng nhập")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("Địa chỉ máy chủ không được chứa query hoặc fragment")
	}
	u.RawQuery, u.Fragment = "", ""
	return strings.TrimRight(u.String(), "/"), nil
}

// SubscriptionKind báo kind do app quản thẻ (chatgpt, grok).
func SubscriptionKind(kind domain.ProviderKind) bool {
	return kindTable[kind].auth == domain.AuthSubscription
}

// OAuthID trả credential id của kind đăng nhập gói; "" với kind khác.
func OAuthID(kind domain.ProviderKind) string { return kindTable[kind].oauthID }

// ── trạng thái hiển thị ─────────────────────────────────────────────────────

// liveStatus suy trạng thái thẻ từ bản ghi DB + auth.json local — không mạng.
func (s *Service) liveStatus(ctx context.Context, p domain.Provider) (domain.AIStatus, error) {
	info := kindTable[p.Kind]
	if info.auth != domain.AuthSubscription {
		return p.Status, nil
	}
	if st, ok := s.session(p.ID); ok && st.active() {
		return domain.StatusLoginPending, nil
	}
	lst, err := s.o.Auth.LocalStatus(ctx, info.oauthID)
	if err != nil {
		return "", err
	}
	switch lst {
	case OAuthAbsent:
		return domain.StatusLoggedOut, nil
	case OAuthRevoked:
		return domain.StatusAuthFailed, nil
	}
	return domain.StatusReady, nil
}

// quotaOf dựng trường quota cho thẻ đăng nhập gói: ảnh trả phí đã charge hôm
// nay so với trần D7. Chỉ tính khi có PaidUsage và provider đang sẵn sàng.
func (s *Service) quotaOf(ctx context.Context, p domain.Provider, st domain.AIStatus) (*domain.Quota, error) {
	if s.o.PaidUsage == nil || info2Receipt[p.Kind] == "" || st != domain.StatusReady {
		return nil, nil
	}
	dayStart := localMidnight(s.o.Now())
	used, err := s.o.PaidUsage(ctx, info2Receipt[p.Kind], dayStart)
	if err != nil {
		return nil, err
	}
	resetAt := dayStart.AddDate(0, 0, 1)
	// Provider báo mốc reset thật (Codex/xAI resets_at) → hiện đúng mốc đó;
	// chỉ nhận mốc trong tương lai và không quá một chu kỳ ngày.
	if s.o.QuotaReset != nil {
		if at, ok := s.o.QuotaReset(info2Receipt[p.Kind]); ok && at.After(s.o.Now()) && at.Before(dayStart.AddDate(0, 0, 2)) {
			resetAt = at
		}
	}
	return &domain.Quota{
		Used:    used,
		Limit:   int64(s.o.DailyImageLimit),
		Period:  "day",
		ResetAt: resetAt.UTC(),
	}, nil
}

// info2Receipt ánh provider id/kind → tên provider trong image_receipts.
var info2Receipt = map[domain.ProviderKind]string{
	domain.KindChatGPT: "codex",
	domain.KindGrok:    "grok",
}

// localMidnight trả nửa đêm giờ địa phương của ngày chứa t.
func localMidnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// ── liệt kê / đọc ───────────────────────────────────────────────────────────

// ProviderView là một thẻ nhà cung cấp đã đầy đủ trạng thái + quota.
// KHÔNG nhúng domain.Provider: view không bao giờ mang EncKey (ciphertext)
// nên không thể vô tình lọt vào JSON/API.
type ProviderView struct {
	ID        string
	Kind      domain.ProviderKind
	Auth      domain.AIAuth
	Label     string
	KeyHint   string
	BaseURL   string
	Status    domain.AIStatus
	CatalogAt int64
	Selected  []string
	CreatedAt int64
	UpdatedAt int64
	Quota     *domain.Quota
	Count     int // catalogCount
	// Plan là gói của credential đăng nhập gói (chatgpt: chatgpt_plan_type
	// trong id_token lúc login, lưu sẵn ở core/auth — KHÔNG decode trong GET).
	// Rỗng khi provider không mang thông tin gói (xAI).
	Plan string
	// DailyPaidImageLimit là trần ảnh trả phí mỗi ngày của provider khoá API
	// (D7, mặc định 50; 0 = không tự tiêu). Cấu hình sẵn của service — GET chỉ
	// chép số ra, không tính toán.
	DailyPaidImageLimit int
}

// viewOf dựng view công khai từ dòng provider — điểm duy nhất lược EncKey.
func viewOf(p domain.Provider, q *domain.Quota, count int, plan string, dailyLimit int) ProviderView {
	return ProviderView{
		ID: p.ID, Kind: p.Kind, Auth: p.Auth, Label: p.Label,
		KeyHint: p.KeyHint, BaseURL: p.BaseURL, Status: p.Status,
		CatalogAt: p.CatalogAt, Selected: p.Selected,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		Quota: q, Count: count, Plan: plan, DailyPaidImageLimit: dailyLimit,
	}
}

// planReader là phần tuỳ chọn của Auth đọc tên gói đã lưu của credential.
// Khai kiểu tuỳ chọn để Auth của test không phải giả lập nó.
type planReader interface {
	Plan(ctx context.Context, oauthID string) (string, error)
}

// planOf đọc gói đã lưu của credential đăng nhập gói — local, không mạng.
func (s *Service) planOf(ctx context.Context, kind domain.ProviderKind) string {
	info := kindTable[kind]
	if info.auth != domain.AuthSubscription {
		return ""
	}
	pr, ok := s.o.Auth.(planReader)
	if !ok {
		return ""
	}
	plan, err := pr.Plan(ctx, info.oauthID)
	if err != nil {
		return ""
	}
	return plan
}

// Providers trả toàn bộ thẻ với trạng thái sống — thuần local, phục vụ GET.
func (s *Service) Providers(ctx context.Context) ([]ProviderView, error) {
	rows, err := s.o.Store.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	return s.viewsOf(ctx, rows)
}

// viewsOf dựng ProviderView cho một tập dòng đã có số model — dùng chung
// cho GET và attention (subscription-only). Không gọi mạng, không đếm thêm.
func (s *Service) viewsOf(ctx context.Context, rows []ProviderSummary) ([]ProviderView, error) {
	views := make([]ProviderView, 0, len(rows))
	for _, row := range rows {
		st, err := s.liveStatus(ctx, row.Provider)
		if err != nil {
			return nil, err
		}
		row.Provider.Status = st
		q, err := s.quotaOf(ctx, row.Provider, st)
		if err != nil {
			return nil, err
		}
		views = append(views, viewOf(row.Provider, q, row.Count, s.planOf(ctx, row.Provider.Kind), s.o.DailyImageLimit))
	}
	return views, nil
}

// Subscribed trả các thẻ đăng nhập gói (giới hạn nhỏ) — đường đọc của
// attention/status, không kéo theo provider khoá API hay toàn danh mục.
func (s *Service) Subscribed(ctx context.Context) ([]ProviderView, error) {
	rows, err := s.o.Store.ListSubscribed(ctx)
	if err != nil {
		return nil, err
	}
	return s.viewsOf(ctx, rows)
}

// RefreshSubscribed lấy lại danh mục của các thẻ đăng nhập gói đang có
// credential. Chạy ở nền lúc khởi động và sau login; thất bại chỉ đổi trạng
// thái thẻ, không làm hỏng khởi động.
func (s *Service) RefreshSubscribed(ctx context.Context) error {
	rows, err := s.o.Store.ListSubscribed(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, row := range rows {
		p := row.Provider
		info := kindTable[p.Kind]
		lst, err := s.o.Auth.LocalStatus(ctx, info.oauthID)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.ID, err))
			continue
		}
		if lst != OAuthPresent {
			_ = s.o.Store.SetStatus(ctx, p.ID, domain.StatusLoggedOut)
			continue
		}
		models, err := s.fetchCatalog(ctx, p, "")
		if err != nil {
			// Đăng nhập vẫn tốt, chỉ danh mục chưa lấy được (Codex CLI vắng,
			// mạng lỗi) — giữ trạng thái sẵn sàng, giữ catalog cũ.
			errs = append(errs, fmt.Errorf("%s: %w", p.ID, err))
			continue
		}
		if err := s.o.Store.ReplaceCatalog(ctx, p.ID, models, s.o.Now()); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.ID, err))
			continue
		}
		_ = s.o.Store.SetStatus(ctx, p.ID, domain.StatusReady)
		s.changed(p.ID)
	}
	return errors.Join(errs...)
}

// provider đọc một thẻ; 404-style khi vắng.
func (s *Service) provider(ctx context.Context, id string) (domain.Provider, error) {
	p, err := s.o.Store.GetProvider(ctx, id)
	if err != nil {
		return domain.Provider{}, err
	}
	if p.ID == "" {
		return domain.Provider{}, apiError(http.StatusNotFound, "not_found", "Không có nhà cung cấp này")
	}
	return p, nil
}

// ModelPageSize là trần một trang danh mục: danh mục lớn nhất (OpenRouter)
// cỡ 400 model; mặc định 200, trần 500. Cursor lấy tiếp phần còn lại.
const (
	DefaultModelLimit = 200
	MaxModelLimit     = 500
)

// Models trả một trang danh mục của provider: lọc trong SQL (q/filter),
// sắp theo id, con trỏ `cursor` = id cuối trang trước. Limit mặc định 200,
// trần 500 — trang sau lấy tiếp bằng cursor, không cắt ngầm dữ liệu.
func (s *Service) Models(ctx context.Context, providerID, q, filter, cursor string, limit int) (ModelPage, error) {
	p, err := s.provider(ctx, providerID)
	if err != nil {
		return ModelPage{}, err
	}
	var onlyID []string
	switch filter {
	case "", "all":
	case "selected":
		onlyID = p.Selected
	case "recommended":
	default:
		return ModelPage{}, apiError(http.StatusBadRequest, "invalid_filter", "Bộ lọc không hợp lệ")
	}
	if limit <= 0 {
		limit = DefaultModelLimit
	}
	if limit > MaxModelLimit {
		limit = MaxModelLimit
	}
	page, err := s.o.Store.SearchModels(ctx, p.ID, ModelQuery{
		Q: strings.ToLower(strings.TrimSpace(q)), Filter: filter,
		After: cursor, Limit: limit + 1, // +1 để phát hiện còn trang
		OnlyID: onlyID,
	})
	if err != nil {
		return ModelPage{}, err
	}
	page.HasMore = len(page.Models) > limit
	if page.HasMore {
		page.Models = page.Models[:limit]
	}
	page.Total, err = s.o.Store.CountModels(ctx, p.ID, ModelQuery{
		Q: strings.ToLower(strings.TrimSpace(q)), Filter: filter, OnlyID: onlyID,
	})
	if err != nil {
		return ModelPage{}, err
	}
	return page, nil
}

// ── api key ────────────────────────────────────────────────────────────────

// RefInfo trả provider + model đã lưu của một ModelRef kèm khoá API đã mở
// (rỗng với thẻ gói/local) — composition root dựng client text/vẽ từ đây.
func (s *Service) RefInfo(ctx context.Context, ref domain.ModelRef) (domain.Provider, domain.AIModel, string, error) {
	p, err := s.provider(ctx, ref.Provider)
	if err != nil {
		return domain.Provider{}, domain.AIModel{}, "", err
	}
	m, ok, err := s.o.Store.GetModel(ctx, p.ID, ref.Model)
	if err != nil {
		return domain.Provider{}, domain.AIModel{}, "", err
	}
	if !ok {
		return domain.Provider{}, domain.AIModel{}, "", apiError(http.StatusUnprocessableEntity,
			domain.ReasonUnknownModel, "Không có model "+ref.Model+" trong danh mục của "+p.Label+".")
	}
	key, err := s.apiKeyOf(p)
	if err != nil {
		return domain.Provider{}, domain.AIModel{}, "", err
	}
	return p, m, key, nil
}

// SubscriptionToken trả access token OAuth đang dùng của một thẻ gói (grok
// cần nó để gọi API xAI bằng giao thức OpenAI-compatible).
func (s *Service) SubscriptionToken(ctx context.Context, kind domain.ProviderKind) (string, error) {
	info := kindTable[kind]
	if info.oauthID == "" {
		return "", apiError(http.StatusBadRequest, domain.ReasonNotSubscription,
			"Nhà cung cấp này không đăng nhập")
	}
	access, _, _, err := s.o.Auth.Credential(ctx, info.oauthID)
	if err != nil {
		return "", err
	}
	return access, nil
}

// apiKeyOf mở khoá đã niêm phong của provider ("" khi không có).
func (s *Service) apiKeyOf(p domain.Provider) (string, error) {
	if len(p.EncKey) == 0 {
		return "", nil
	}
	plain, err := s.o.Box.Reveal(p.EncKey)
	if err != nil {
		return "", fmt.Errorf("không mở được khoá API của %s: %w", p.ID, err)
	}
	return string(plain), nil
}

// ── tạo / sửa / xoá provider ───────────────────────────────────────────────

// CreateProviderInput là đầu vào POST /api/ai/providers.
type CreateProviderInput struct {
	Kind    domain.ProviderKind `json:"kind"`
	APIKey  string              `json:"apiKey,omitempty"`
	BaseURL string              `json:"baseUrl,omitempty"`
}

// CreateProvider tạo thẻ mới và lấy danh mục ngay. Khoá sai vẫn tạo thẻ với
// status auth_failed (để Đổi khoá) nhưng trả 422 như hợp đồng.
func (s *Service) CreateProvider(ctx context.Context, in CreateProviderInput) (ProviderView, error) {
	info, ok := kindTable[in.Kind]
	if !ok {
		return ProviderView{}, apiError(http.StatusBadRequest, domain.ReasonInvalidKind, "Loại nhà cung cấp không hợp lệ")
	}
	if info.auth == domain.AuthSubscription {
		return ProviderView{}, apiError(http.StatusConflict, domain.ReasonManaged,
			"Nhà cung cấp này đăng nhập trên thẻ sẵn có, không thêm bằng khoá")
	}
	in.APIKey = strings.TrimSpace(in.APIKey)
	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if info.needsKey && in.APIKey == "" {
		return ProviderView{}, apiError(http.StatusBadRequest, domain.ReasonMissingKey, "Cần khoá API")
	}
	if info.needsURL && in.BaseURL == "" {
		return ProviderView{}, apiError(http.StatusBadRequest, domain.ReasonMissingBaseURL, "Cần địa chỉ máy chủ")
	}
	// Chỉ kind "địa chỉ tuỳ biến" nhận baseUrl: ollama (máy local/LAN) và
	// openai_compat (server do chủ sở hữu tự cấu hình). Kind cố định bỏ qua
	// trường này để không chuyển tiếp credential tới địa chỉ lạ.
	if in.BaseURL != "" && !info.customURL {
		return ProviderView{}, apiError(http.StatusBadRequest, domain.ReasonInvalidBaseURL,
			"Nhà cung cấp này không nhận địa chỉ máy chủ tuỳ biến")
	}
	if in.BaseURL == "" {
		in.BaseURL = info.defURL
	} else {
		base, err := validateBaseURL(in.BaseURL)
		if err != nil {
			return ProviderView{}, apiError(http.StatusBadRequest, domain.ReasonInvalidBaseURL, err.Error())
		}
		in.BaseURL = base
	}
	if info.customURL {
		if err := s.checkHostedAddress(ctx, in.BaseURL); err != nil {
			return ProviderView{}, err
		}
	}

	var enc []byte
	var hint string
	if in.APIKey != "" {
		var err error
		enc, err = s.o.Box.Seal([]byte(in.APIKey))
		if err != nil {
			return ProviderView{}, err
		}
		hint = secretbox.Hint(in.APIKey)
	}
	p := domain.Provider{
		ID:       newProviderID(in.Kind),
		Kind:     in.Kind,
		Auth:     info.auth,
		Label:    info.label,
		EncKey:   enc,
		KeyHint:  hint,
		BaseURL:  in.BaseURL,
		Status:   domain.StatusReady,
		Selected: []string{},
	}
	models, err := s.fetchCatalog(ctx, p, in.APIKey)
	st := domain.StatusReady
	var apiErr *Error
	switch {
	case err == nil:
	case IsAuthError(err):
		st = domain.StatusAuthFailed
		var ce *CatalogError
		errors.As(err, &ce)
		apiErr = apiError(http.StatusUnprocessableEntity, domain.ReasonAuthFailed,
			"Nhà cung cấp từ chối khoá API này. Kiểm tra lại khoá.")
		apiErr.Fields = map[string]any{"status": ce.Status}
	case IsUnreachable(err):
		st = domain.StatusUnreachable
		apiErr = apiError(http.StatusUnprocessableEntity, domain.ReasonUnreachable,
			"Chưa kết nối được nhà cung cấp này.")
	default:
		st = domain.StatusUnreachable
		apiErr = apiError(http.StatusUnprocessableEntity, domain.ReasonUnreachable,
			"Không lấy được danh sách model.")
	}
	p.Status = st
	p.CatalogAt = millis(s.o.Now())
	if err := s.o.Store.UpsertProvider(ctx, p); err != nil {
		return ProviderView{}, err
	}
	if len(models) > 0 {
		if err := s.o.Store.ReplaceCatalog(ctx, p.ID, models, s.o.Now()); err != nil {
			return ProviderView{}, err
		}
	}
	s.changed(p.ID)
	view := viewOf(p, nil, len(models), s.planOf(ctx, p.Kind), s.o.DailyImageLimit)
	if apiErr != nil {
		return view, apiErr
	}
	return view, nil
}

// PatchProviderInput là đầu vào PATCH /api/ai/providers/:id.
type PatchProviderInput struct {
	Selected *[]string `json:"selected,omitempty"`
	APIKey   *string   `json:"apiKey,omitempty"`
	BaseURL  *string   `json:"baseUrl,omitempty"`
}

// PatchProvider đổi lựa chọn model, khoá hoặc địa chỉ của một thẻ.
// Đổi khoá/địa chỉ kích lấy lại danh mục ngay (chữ "Kết nối và lấy model" của
// nút Đổi khoá). Bỏ chọn model đang được trợ lý dùng → 409 in_use.
// Toàn bộ bước ghi nằm dưới cfgMu — cùng ranh giới với PutAgents/Delete.
func (s *Service) PatchProvider(ctx context.Context, id string, in PatchProviderInput) (ProviderView, error) {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()

	p, err := s.provider(ctx, id)
	if err != nil {
		return ProviderView{}, err
	}
	info := kindTable[p.Kind]

	if in.Selected != nil {
		if err := s.checkSelectedUsable(ctx, p, *in.Selected); err != nil {
			return ProviderView{}, err
		}
		p.Selected = dedupe(*in.Selected)
		// Ghi riêng trường selected: không đụng khoá/địa chỉ/trạng thái.
		if err := s.o.Store.SetSelected(ctx, p.ID, p.Selected); err != nil {
			return ProviderView{}, err
		}
	}
	keyChanged := false
	if in.APIKey != nil {
		if info.auth != domain.AuthAPIKey {
			return ProviderView{}, apiError(http.StatusBadRequest, domain.ReasonManaged,
				"Nhà cung cấp này không nhận khoá API")
		}
		key := strings.TrimSpace(*in.APIKey)
		if key == "" {
			return ProviderView{}, apiError(http.StatusBadRequest, domain.ReasonMissingKey, "Cần khoá API")
		}
		enc, err := s.o.Box.Seal([]byte(key))
		if err != nil {
			return ProviderView{}, err
		}
		p.EncKey, p.KeyHint = enc, secretbox.Hint(key)
		keyChanged = true
	}
	if in.BaseURL != nil {
		base := strings.TrimRight(strings.TrimSpace(*in.BaseURL), "/")
		if base == "" {
			return ProviderView{}, apiError(http.StatusBadRequest, domain.ReasonMissingBaseURL, "Cần địa chỉ máy chủ")
		}
		if !info.customURL {
			return ProviderView{}, apiError(http.StatusBadRequest, domain.ReasonInvalidBaseURL,
				"Nhà cung cấp này không nhận địa chỉ máy chủ tuỳ biến")
		}
		valid, err := validateBaseURL(base)
		if err != nil {
			return ProviderView{}, apiError(http.StatusBadRequest, domain.ReasonInvalidBaseURL, err.Error())
		}
		if err := s.checkHostedAddress(ctx, valid); err != nil {
			return ProviderView{}, err
		}
		// Khoá đã lưu chỉ được cấp cho máy chủ cũ: đổi sang máy chủ khác mà
		// không nhập khoá mới thì khoá cũ sẽ bị gửi tới địa chỉ lạ.
		if in.APIKey == nil && len(p.EncKey) > 0 && !sameHost(p.BaseURL, valid) {
			return ProviderView{}, apiError(http.StatusBadRequest, domain.ReasonMissingKey,
				"Đổi sang máy chủ khác cần nhập lại khoá API")
		}
		p.BaseURL = valid
		keyChanged = true
	}

	if keyChanged {
		key, err := s.apiKeyOf(p)
		if err != nil {
			return ProviderView{}, err
		}
		models, ferr := s.fetchCatalog(ctx, p, key)
		switch {
		case ferr == nil:
			p.Status = domain.StatusReady
		case IsAuthError(ferr):
			p.Status = domain.StatusAuthFailed
		default:
			p.Status = domain.StatusUnreachable
		}
		// Ghi khoá/địa chỉ/trạng thái mới; selected đã ghi riêng ở trên nên
		// không bị bản ghi này đè — và PATCH khác chờ cfgMu nên không tranh.
		if err := s.o.Store.UpdateProviderKey(ctx, p.ID, p.EncKey, p.KeyHint, p.BaseURL, p.Status); err != nil {
			return ProviderView{}, err
		}
		if p.Status == domain.StatusReady {
			p.CatalogAt = millis(s.o.Now())
			if err := s.o.Store.ReplaceCatalog(ctx, p.ID, models, s.o.Now()); err != nil {
				return ProviderView{}, err
			}
			s.changed(p.ID)
			return viewOf(p, nil, len(models), s.planOf(ctx, p.Kind), s.o.DailyImageLimit), nil
		}
		s.changed(p.ID)
		var ce *CatalogError
		if errors.As(ferr, &ce) && ce.Kind == "auth" {
			ae := apiError(http.StatusUnprocessableEntity, domain.ReasonAuthFailed,
				"Nhà cung cấp từ chối khoá API này. Kiểm tra lại khoá.")
			ae.Fields = map[string]any{"status": ce.Status}
			return viewOf(p, nil, 0, s.planOf(ctx, p.Kind), s.o.DailyImageLimit), ae
		}
		return viewOf(p, nil, 0, s.planOf(ctx, p.Kind), s.o.DailyImageLimit), apiError(http.StatusUnprocessableEntity,
			domain.ReasonUnreachable, "Chưa kết nối được nhà cung cấp này.")
	}

	if in.Selected != nil {
		s.changed(p.ID)
	}
	models, err := s.o.Store.ListModels(ctx, p.ID)
	if err != nil {
		return ProviderView{}, err
	}
	return viewOf(p, nil, len(models), s.planOf(ctx, p.Kind), s.o.DailyImageLimit), nil
}

// checkSelectedUsable chặn bỏ chọn model đang được trợ lý dùng và model id
// không có trong danh mục.
func (s *Service) checkSelectedUsable(ctx context.Context, p domain.Provider, next []string) error {
	inUse, err := s.modelsInUse(ctx, p.ID)
	if err != nil {
		return err
	}
	nextSet := map[string]bool{}
	for _, id := range next {
		nextSet[id] = true
	}
	for id := range inUse {
		if !nextSet[id] {
			return &Error{Status: http.StatusConflict, Reason: domain.ReasonInUse,
				Message: "Model này đang được một trợ lý dùng.",
				Fields:  map[string]any{"agents": inUse[id]}}
		}
	}
	for _, id := range next {
		if _, ok, err := s.o.Store.GetModel(ctx, p.ID, id); err != nil {
			return err
		} else if !ok {
			return apiError(http.StatusUnprocessableEntity, domain.ReasonUnknownModel,
				fmt.Sprintf("Không có model %s trong danh mục của %s.", id, p.Label))
		}
	}
	return nil
}

// modelsInUse trả model-id → các trợ lý đang gán nó trên provider này.
func (s *Service) modelsInUse(ctx context.Context, providerID string) (map[string][]string, error) {
	agents, err := s.Agents(ctx)
	if err != nil {
		return nil, err
	}
	uses := map[string][]string{}
	add := func(agent string, refs ...domain.ModelRef) {
		for _, r := range refs {
			if r.Provider == providerID {
				uses[r.Model] = append(uses[r.Model], agent)
			}
		}
	}
	add("director", agents.Director.Model)
	add("director", agents.Director.Fallback...)
	add("artist", agents.Artist.PromptModel)
	add("artist", agents.Artist.Draw)
	add("artist", agents.Artist.DrawFallback...)
	for k, v := range uses {
		uses[k] = dedupeStrings(v)
	}
	return uses, nil
}

// agentsUsingProvider trả các trợ lý đang dùng BẤT KỲ model nào của provider.
func (s *Service) agentsUsingProvider(ctx context.Context, providerID string) ([]string, error) {
	inUse, err := s.modelsInUse(ctx, providerID)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, agents := range inUse {
		for _, a := range agents {
			set[a] = true
		}
	}
	out := make([]string, 0, len(set))
	for a := range set {
		out = append(out, a)
	}
	return out, nil
}

// DeleteProvider xoá một thẻ: 409 in_use khi đang được trợ lý gán; thẻ do
// app quản (chatgpt/grok) không xoá được. Chạy dưới cfgMu cùng PutAgents/
// PatchProvider nên kiểm tra→xoá là một ranh giới nguyên tử (không chen
// lệnh gán trợ lý giữa hai bước).
func (s *Service) DeleteProvider(ctx context.Context, id string) error {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()

	p, err := s.provider(ctx, id)
	if err != nil {
		return err
	}
	if SubscriptionKind(p.Kind) {
		return apiError(http.StatusConflict, domain.ReasonManaged,
			"Nhà cung cấp này do app quản — đăng xuất thay vì xoá")
	}
	agents, err := s.agentsUsingProvider(ctx, p.ID)
	if err != nil {
		return err
	}
	if len(agents) > 0 {
		return &Error{Status: http.StatusConflict, Reason: domain.ReasonInUse,
			Message: "Nhà cung cấp này đang được một trợ lý dùng.",
			Fields:  map[string]any{"agents": agents}}
	}
	if err := s.o.Store.DeleteProvider(ctx, id); err != nil {
		return err
	}
	s.changed(id)
	return nil
}

// ── test / check ───────────────────────────────────────────────────────────

// TestResult là kết quả POST .../test.
type TestResult struct {
	OK    bool   `json:"ok"`
	Ms    int64  `json:"ms"`
	Error string `json:"error,omitempty"`
}

// TestProvider gọi lấy danh mục lại (lượt ngắn rẻ nhất) và cập nhật status.
// Kết quả luôn 200-level: {ok:false} khi provider từ chối/không kết nối được.
func (s *Service) TestProvider(ctx context.Context, id string) (TestResult, error) {
	p, err := s.provider(ctx, id)
	if err != nil {
		return TestResult{}, err
	}
	if SubscriptionKind(p.Kind) {
		// Đăng nhập gói: lượt "ngắn" là lấy token + danh mục CLI/API.
		lst, err := s.o.Auth.LocalStatus(ctx, kindTable[p.Kind].oauthID)
		if err != nil {
			return TestResult{}, err
		}
		if lst == OAuthAbsent {
			return TestResult{OK: false, Error: "logged_out"}, nil
		}
	}
	key, err := s.apiKeyOf(p)
	if err != nil {
		return TestResult{}, err
	}
	start := s.o.Now()
	models, err := s.fetchCatalog(ctx, p, key)
	ms := s.o.Now().Sub(start).Milliseconds()
	if err != nil {
		st := domain.StatusUnreachable
		reason := "unreachable"
		if IsAuthError(err) {
			st = domain.StatusAuthFailed
			reason = "auth_failed"
		}
		if p.Auth != domain.AuthSubscription {
			// Lỗi lưu trạng thái phải lộ ra: báo "test xong" mà thẻ vẫn giữ
			// trạng thái cũ là nói dối.
			if serr := s.o.Store.SetStatus(ctx, p.ID, st); serr != nil {
				return TestResult{}, serr
			}
		}
		s.changed(p.ID)
		return TestResult{OK: false, Ms: ms, Error: reason}, nil
	}
	if p.Auth != domain.AuthSubscription {
		if err := s.o.Store.SetStatus(ctx, p.ID, domain.StatusReady); err != nil {
			return TestResult{}, err
		}
	}
	if err := s.o.Store.ReplaceCatalog(ctx, p.ID, models, s.o.Now()); err != nil {
		return TestResult{}, err
	}
	s.changed(p.ID)
	return TestResult{OK: true, Ms: ms}, nil
}

// CheckModelResult là kết quả POST .../models/check.
type CheckModelResult struct {
	OK       bool                `json:"ok"`
	Ms       int64               `json:"ms"`
	Caps     []domain.Capability `json:"caps,omitempty"`
	ContextK int                 `json:"contextK,omitempty"`
	Error    string              `json:"error,omitempty"`
}

// CheckModel gọi một lượt ngắn tới model rồi thêm nó vào danh mục với
// manual: true (lối "Không thấy model? Thêm theo tên").
func (s *Service) CheckModel(ctx context.Context, id, model string) (CheckModelResult, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return CheckModelResult{}, apiError(http.StatusBadRequest, domain.ReasonUnknownModel, "Cần tên model")
	}
	p, err := s.provider(ctx, id)
	if err != nil {
		return CheckModelResult{}, err
	}
	key, err := s.apiKeyOf(p)
	if err != nil {
		return CheckModelResult{}, err
	}
	start := s.o.Now()
	m, err := s.checkModel(ctx, p, key, model)
	ms := s.o.Now().Sub(start).Milliseconds()
	if err != nil {
		// Phân loại đúng: từ chối khoá, không kết nối được, model không tồn
		// tại, còn lại là provider gọi hỏng (model không nhận chat…).
		reason := "failed"
		if IsAuthError(err) {
			reason = "auth_failed"
			if p.Auth != domain.AuthSubscription {
				_ = s.o.Store.SetStatus(ctx, p.ID, domain.StatusAuthFailed)
			}
		} else if IsUnreachable(err) {
			reason = "unreachable"
		} else {
			var ce *CatalogError
			if errors.As(err, &ce) && ce.Status == http.StatusNotFound {
				reason = "not_found"
			}
		}
		s.changed(p.ID)
		return CheckModelResult{OK: false, Ms: ms, Error: reason}, nil
	}
	m.Manual = true
	if err := s.o.Store.UpsertModel(ctx, p.ID, m); err != nil {
		return CheckModelResult{}, err
	}
	s.changed(p.ID)
	return CheckModelResult{OK: true, Ms: ms, Caps: m.Caps, ContextK: m.ContextK}, nil
}

// hostedCatalogGuard chặn cuộc gọi ra địa chỉ nội bộ cả với thẻ đã lưu từ
// trước (tạo khi chưa hosted, hay DNS đổi về sau), không chỉ lúc tạo/sửa.
func (s *Service) hostedCatalogGuard(ctx context.Context, p domain.Provider) error {
	if p.BaseURL == "" {
		return nil
	}
	if err := s.checkHostedAddress(ctx, p.BaseURL); err != nil {
		return &CatalogError{Kind: "failed", Message: err.Error()}
	}
	return nil
}

// fetchCatalog gọi adapter của kind (không mạng trong GET — chỉ caller chủ
// động gọi hàm này).
func (s *Service) fetchCatalog(ctx context.Context, p domain.Provider, apiKey string) ([]domain.AIModel, error) {
	ad, ok := s.o.Adapters[p.Kind]
	if !ok {
		return nil, &CatalogError{Kind: "failed", Message: "không có adapter cho " + p.Kind}
	}
	if err := s.hostedCatalogGuard(ctx, p); err != nil {
		return nil, err
	}
	models, err := ad.FetchCatalog(ctx, p, apiKey)
	if err != nil {
		return nil, err
	}
	return s.enrich(p.Kind, models), nil
}

func (s *Service) checkModel(ctx context.Context, p domain.Provider, apiKey, model string) (domain.AIModel, error) {
	ad, ok := s.o.Adapters[p.Kind]
	if !ok {
		return domain.AIModel{}, &CatalogError{Kind: "failed", Message: "không có adapter cho " + p.Kind}
	}
	if err := s.hostedCatalogGuard(ctx, p); err != nil {
		return domain.AIModel{}, err
	}
	m, err := ad.CheckModel(ctx, p, apiKey, model)
	if err != nil {
		return domain.AIModel{}, err
	}
	return s.enrichModel(p.Kind, m), nil
}

// changed phát sự kiện ai + yêu cầu status render lại.
func (s *Service) changed(ref string) {
	if s.o.Notify != nil {
		s.o.Notify(ref)
	}
	if s.o.StatusBump != nil {
		s.o.StatusBump()
	}
}

// newProviderID sinh id "kind-xxxxxx" (hex ngẫu nhiên).
func newProviderID(kind domain.ProviderKind) string {
	return kind + "-" + randomHex(4)
}

// millis trả Unix mili-giây.
func millis(t time.Time) int64 { return t.UnixMilli() }

// dedupe bỏ phần tử trùng, giữ thứ tự.
func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func dedupeStrings(in []string) []string { return dedupe(in) }

// randomHex trả n*2 ký tự hex ngẫu nhiên.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	const hexdigits = "0123456789abcdef"
	out := make([]byte, n*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}
