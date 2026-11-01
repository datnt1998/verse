package aiconn_test

// DNS rebinding: kiểm địa chỉ lúc lưu baseUrl không đủ — tên miền có thể trả
// IP công khai lúc kiểm rồi trả 127.0.0.1 lúc client nối thật. Các test dưới
// dùng resolver giả lập đúng kịch bản đó và khẳng định không request nào tới
// được máy chủ nội bộ.

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/aiconn"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// rebindingResolver trả IP công khai cho các lần phân giải được liệt kê (lúc
// lưu cấu hình, lúc kiểm ngay trước khi gọi) và địa chỉ nội bộ cho lần nối thật.
type rebindingResolver struct {
	mu       sync.Mutex
	calls    int
	public   map[int]bool // số thứ tự lần phân giải trả IP công khai
	rebounds []string
}

func newRebindingResolver(rebounds []string, publicCalls ...int) *rebindingResolver {
	public := make(map[int]bool, len(publicCalls))
	for _, n := range publicCalls {
		public[n] = true
	}
	return &rebindingResolver{public: public, rebounds: rebounds}
}

func (r *rebindingResolver) lookup(_ context.Context, host string) ([]net.IP, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.public[r.calls] {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}
	var out []net.IP
	for _, s := range r.rebounds {
		out = append(out, net.ParseIP(s))
	}
	return out, nil
}

func (r *rebindingResolver) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// hitCounter là máy chủ kiểu OpenAI trên loopback đếm số request tới được nó.
type hitCounter struct {
	*httptest.Server
	mu   sync.Mutex
	hits int
}

func newHitCounter(t *testing.T) *hitCounter {
	t.Helper()
	h := &hitCounter{}
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h.mu.Lock()
		h.hits++
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"m1"}]}`))
	}))
	t.Cleanup(h.Close)
	return h
}

func (h *hitCounter) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hits
}

// portOf trả cổng của máy chủ thử để ghép vào tên miền giả lập.
func portOf(t *testing.T, rawURL string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(rawURL, "http://"))
	if err != nil {
		t.Fatalf("URL thử không có cổng: %s", rawURL)
	}
	return port
}

func TestHostedClientRefusesRebindingAtDial(t *testing.T) {
	// Mỗi địa chỉ nội bộ mà DNS rebinding có thể trỏ về đều bị chặn ở tầng nối.
	for _, rebound := range []string{"127.0.0.1", "::1", "10.0.0.7", "169.254.169.254", "192.168.1.20"} {
		t.Run(rebound, func(t *testing.T) {
			srv := newHitCounter(t)
			resolver := newRebindingResolver([]string{rebound})
			client := aiconn.NewCredentialClient(aiconn.CredentialClientOptions{
				Timeout: 5 * time.Second, Hosted: true, LookupIP: resolver.lookup,
			})
			base := "http://rebind.attacker.example:" + portOf(t, srv.URL)

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/v1/models", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer sk-secret")
			if _, err := client.Do(req); err == nil {
				t.Fatal("request tới địa chỉ nội bộ sau rebinding phải bị từ chối")
			} else if strings.Contains(err.Error(), rebound) {
				t.Fatalf("lỗi làm lộ địa chỉ nội bộ: %v", err)
			}
			// Nối lại lần hai vẫn phải qua kiểm tra (keep-alive/DNS đổi tiếp).
			if _, err := client.Do(req.Clone(context.Background())); err == nil {
				t.Fatal("lần nối thứ hai tới địa chỉ nội bộ phải bị từ chối")
			}
			if got := srv.count(); got != 0 {
				t.Fatalf("máy chủ nội bộ nhận %d request — rebinding lọt qua", got)
			}
			if resolver.count() < 2 {
				t.Fatalf("resolver chỉ được gọi %d lần — guard không kiểm ở tầng dial", resolver.count())
			}
		})
	}
}

func TestHostedCatalogFetchRefusesRebinding(t *testing.T) {
	// Cùng kịch bản nhưng qua service: địa chỉ được kiểm công khai lúc tạo và
	// lúc guard ngay trước lượt gọi, rồi DNS đổi về loopback đúng lúc nối.
	srv := newHitCounter(t)
	resolver := newRebindingResolver([]string{"127.0.0.1"}, 1, 2, 4)
	svc := realAdapterService(t, newMemStore(), true, resolver.lookup)

	view, err := svc.CreateProvider(context.Background(), aiconn.CreateProviderInput{
		Kind:    domain.KindOpenAICompat,
		APIKey:  "sk-owner-secret",
		BaseURL: "http://rebind.attacker.example:" + portOf(t, srv.URL) + "/v1",
	})
	ae, ok := aiconn.AsAPIError(err)
	if !ok || ae.Status != http.StatusUnprocessableEntity || ae.Reason != domain.ReasonUnreachable {
		t.Fatalf("lỗi = %v, muốn 422 unreachable (địa chỉ công khai được lưu nhưng không nối được)", err)
	}
	if got := srv.count(); got != 0 {
		t.Fatalf("máy chủ nội bộ nhận %d request — rebinding lọt qua", got)
	}
	if view.Status != domain.StatusUnreachable {
		t.Fatalf("status = %q, muốn unreachable", view.Status)
	}

	// Khoá đã lưu vẫn không đi tới máy chủ nội bộ ở các lượt sau (check model).
	res, err := svc.CheckModel(context.Background(), view.ID, "m1")
	if err != nil {
		t.Fatalf("CheckModel: %v", err)
	}
	if res.OK || res.Error != "unreachable" {
		t.Fatalf("CheckModel = %+v, muốn unreachable", res)
	}
	if got := srv.count(); got != 0 {
		t.Fatalf("máy chủ nội bộ nhận %d request ở lượt check model", got)
	}
}

func TestDesktopClientStillDialsLoopback(t *testing.T) {
	// Không hosted (app một chủ trên máy cá nhân): Ollama/compat ở loopback vẫn nối được.
	srv := newHitCounter(t)
	client := aiconn.NewCredentialClient(aiconn.CredentialClientOptions{Timeout: 5 * time.Second})
	resp, err := client.Get(srv.URL + "/v1/models")
	if err != nil {
		t.Fatalf("chế độ desktop phải nối được loopback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || srv.count() != 1 {
		t.Fatalf("status = %d, hits = %d", resp.StatusCode, srv.count())
	}

	// Cùng đường qua service: provider loopback tạo được và lấy danh mục thật.
	svc := realAdapterService(t, newMemStore(), false, nil)
	if _, err := svc.CreateProvider(context.Background(), aiconn.CreateProviderInput{
		Kind: domain.KindOpenAICompat, APIKey: "sk-local", BaseURL: srv.URL + "/v1",
	}); err != nil {
		t.Fatalf("tạo provider loopback ở chế độ desktop: %v", err)
	}
	if srv.count() != 2 {
		t.Fatalf("hits = %d, muốn 2", srv.count())
	}
}
