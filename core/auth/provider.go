package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

// Controller nhận sự kiện từ một luồng đăng nhập (tương đương OAuthController
// của omp). Các callback có thể nil.
type Controller struct {
	// OnAuth được gọi khi người dùng cần mở URL; instructions mô tả bước tiếp
	// theo (vd. mã thiết bị cần nhập).
	OnAuth func(url, instructions string)
	// OnDeviceCode được gọi thêm trong luồng device-code với mã người dùng cần
	// nhập — UI có thể hiển thị trực tiếp mà không phải parse instructions.
	OnDeviceCode func(userCode, verifyURL string)
	// OnProgress báo tiến độ dạng văn bản ngắn.
	OnProgress func(message string)
}

func (c Controller) auth(url, instructions string) {
	if c.OnAuth != nil {
		c.OnAuth(url, instructions)
	}
}

func (c Controller) progress(message string) {
	if c.OnProgress != nil {
		c.OnProgress(message)
	}
}

// deviceCode báo mã thiết bị cho UI (nếu UI đăng ký nghe).
func (c Controller) deviceCode(userCode, verifyURL string) {
	if c.OnDeviceCode != nil {
		c.OnDeviceCode(userCode, verifyURL)
	}
}

// ConsoleController in URL đăng nhập và tiến độ ra w (dùng cho CLI/setup).
func ConsoleController(w io.Writer) Controller {
	return Controller{
		OnAuth: func(url, instructions string) {
			fmt.Fprintf(w, "\nMở URL sau trong trình duyệt:\n  %s\n", url)
			if instructions != "" {
				fmt.Fprintln(w, instructions)
			}
			fmt.Fprintln(w)
		},
		OnProgress: func(message string) { fmt.Fprintln(w, message) },
	}
}

// LoginOptions chọn biến thể của luồng đăng nhập.
type LoginOptions struct {
	// Device dùng luồng device-code (không cần callback server cục bộ) — hợp
	// với SSH/headless hoặc khi cổng callback bị chiếm.
	Device bool
	// From phân biệt nơi khởi động luồng trình duyệt ("cli" mặc định / "web"),
	// để trang callback chọn biến thể phù hợp (A4). Được truyền phía server,
	// không lộ ra URL.
	From string
	// Manual nhận mã dán tay (URL chuyển hướng hoặc `code#state`) từ UI trong khi luồng
	// đang chờ — provider hỗ trợ (PasteCode) đua nó với callback localhost; ở chế độ
	// Device đây là đường duy nhất. Nil = không có nhập tay.
	Manual <-chan ManualSubmission
}

// ManualSubmission là một lần dán mã của người dùng. Luồng đăng nhập kiểm tra rồi trả
// kết quả qua Result (nil = đã nhận, luồng tiếp tục đổi token; lỗi = mã không dùng được
// và luồng vẫn chờ lần dán tiếp theo). Result phải có buffer ≥ 1.
type ManualSubmission struct {
	Input  string
	Result chan<- error
}

// Provider là một nhà cung cấp đăng nhập OAuth.
type Provider interface {
	// ID là khoá trong kho credential và tên provider trong config (vd. "openai-codex").
	ID() string
	// Name là tên hiển thị.
	Name() string
	Login(ctx context.Context, ctrl Controller, opts LoginOptions) (Credential, error)
	Refresh(ctx context.Context, cred Credential) (Credential, error)
}

var registry = map[string]Provider{}

func register(p Provider) { registry[p.ID()] = p }

// Register đăng ký thêm một provider OAuth vào registry (ghi đè theo ID).
// Chủ yếu phục vụ test/stub; các provider tích hợp tự đăng ký qua init().
func Register(p Provider) { register(p) }

// Lookup tìm provider OAuth theo ID.
func Lookup(id string) (Provider, bool) {
	p, ok := registry[id]
	return p, ok
}

// Providers liệt kê mọi provider OAuth đã đăng ký, sắp theo ID.
func Providers() []Provider {
	out := make([]Provider, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// DeviceOnly báo provider id chỉ có luồng device-code (không có callback trình duyệt).
func DeviceOnly(id string) bool {
	p, ok := Lookup(id)
	if !ok {
		return false
	}
	d, ok := p.(interface{ DeviceOnly() bool })
	return ok && d.DeviceOnly()
}

// PasteCode báo provider id nhận mã dán tay (LoginOptions.Manual) — nơi trình duyệt không
// với tới máy chạy ainovel (mở từ thiết bị khác) vẫn đăng nhập được.
func PasteCode(id string) bool {
	p, ok := Lookup(id)
	if !ok {
		return false
	}
	d, ok := p.(interface{ PasteCode() bool })
	return ok && d.PasteCode()
}

// NotLoggedInError bọc ErrNotLoggedIn kèm hướng dẫn đăng nhập.
func NotLoggedInError(id string) error {
	return fmt.Errorf("%w %s — chạy `ainovel-cli login %s` để đăng nhập", ErrNotLoggedIn, id, id)
}

// defaultRefreshSkew: refresh sớm trước hạn để request dài không mang token hết hạn giữa chừng.
const defaultRefreshSkew = 5 * time.Minute

// errUnchanged báo Update không cần ghi lại kho.
var errUnchanged = errors.New("credential unchanged")

// Login chạy luồng đăng nhập của provider id và lưu credential vào store.
func Login(ctx context.Context, store *Store, id string, ctrl Controller, opts LoginOptions) (Credential, error) {
	provider, ok := Lookup(id)
	if !ok {
		return Credential{}, fmt.Errorf("provider %q không hỗ trợ đăng nhập OAuth", id)
	}
	cred, err := provider.Login(ctx, ctrl, opts)
	if err != nil {
		return Credential{}, err
	}
	if err := store.Put(id, cred); err != nil {
		return Credential{}, fmt.Errorf("lưu credential: %w", err)
	}
	return cred, nil
}

// Manager cấp credential còn hạn, tự refresh và lưu lại token mới.
type Manager struct {
	store *Store
	skew  time.Duration
	now   func() time.Time
	// lookup cho phép test thay provider; mặc định dùng registry.
	lookup func(id string) (Provider, bool)
	// mu tuần tự hoá refresh trong cùng tiến trình; khoá file lo phần liên tiến trình.
	mu sync.Mutex
}

// NewManager tạo Manager trên kho chỉ định.
func NewManager(store *Store) *Manager {
	return &Manager{store: store, skew: defaultRefreshSkew, now: time.Now, lookup: Lookup}
}

// Store trả về kho credential bên dưới.
func (m *Manager) Store() *Store { return m.store }

var (
	defaultManagerOnce sync.Once
	defaultManager     *Manager
	defaultManagerErr  error
)

// DefaultManager trả về Manager dùng chung của tiến trình trên kho mặc định.
func DefaultManager() (*Manager, error) {
	defaultManagerOnce.Do(func() {
		store, err := DefaultStore()
		if err != nil {
			defaultManagerErr = err
			return
		}
		defaultManager = NewManager(store)
	})
	return defaultManager, defaultManagerErr
}

// Credential trả về credential còn hạn của provider, refresh nếu sắp hết hạn.
func (m *Manager) Credential(ctx context.Context, id string) (Credential, error) {
	cred, ok, err := m.store.Get(id)
	if err != nil {
		return Credential{}, err
	}
	if !ok {
		return Credential{}, NotLoggedInError(id)
	}
	if !m.expiring(cred) {
		return cred, nil
	}
	return m.refresh(ctx, id, m.expiring)
}

// Invalidate buộc refresh sau khi server từ chối access token stale (HTTP 401).
// Nếu tiến trình khác đã refresh xong (token trong kho khác stale) thì dùng luôn
// token mới, không refresh lần nữa.
func (m *Manager) Invalidate(ctx context.Context, id, stale string) (Credential, error) {
	return m.refresh(ctx, id, func(cur Credential) bool { return cur.Access == stale })
}

func (m *Manager) expiring(cred Credential) bool {
	return cred.Expires == 0 || !m.now().Add(m.skew).Before(cred.ExpiresAt())
}

func (m *Manager) refresh(ctx context.Context, id string, need func(Credential) bool) (Credential, error) {
	provider, ok := m.lookup(id)
	if !ok {
		return Credential{}, fmt.Errorf("provider OAuth %q không được hỗ trợ", id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var out Credential
	err := m.store.Update(id, func(cur Credential, ok bool) (Credential, bool, error) {
		if !ok {
			return cur, false, NotLoggedInError(id)
		}
		if !need(cur) {
			out = cur
			return cur, true, errUnchanged
		}
		if cur.Refresh == "" {
			return cur, true, fmt.Errorf("phiên đăng nhập %s đã hết hạn và không có refresh token — chạy `ainovel-cli login %s`", id, id)
		}
		next, err := provider.Refresh(ctx, cur)
		if err != nil {
			if errors.Is(err, ErrXAITierDenied) {
				// Giới hạn gói, không phải token hỏng: đăng nhập lại không giúp được gì.
				return cur, true, err
			}
			return cur, true, fmt.Errorf("làm mới phiên đăng nhập %s thất bại (thử `ainovel-cli login %s`): %w", id, id, err)
		}
		out = next
		return next, true, nil
	})
	if err != nil && !errors.Is(err, errUnchanged) {
		return Credential{}, err
	}
	return out, nil
}
