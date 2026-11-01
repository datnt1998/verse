package aiconn_test

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

// authRecorder là máy chủ kiểu OpenAI ghi lại mọi header Authorization nhận được.
type authRecorder struct {
	*httptest.Server
	mu   sync.Mutex
	auth []string
}

func newAuthRecorder(t *testing.T) *authRecorder {
	t.Helper()
	r := &authRecorder{}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.auth = append(r.auth, req.Header.Get("Authorization"))
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"m1"}]}`))
	}))
	t.Cleanup(r.Close)
	return r
}

func (r *authRecorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.auth...)
}

func realAdapterService(t *testing.T, store *memStore, hosted bool, lookup func(context.Context, string) ([]net.IP, error)) *aiconn.Service {
	t.Helper()
	client := aiconn.NewCredentialClient(aiconn.CredentialClientOptions{
		Timeout: 5 * time.Second, Hosted: hosted, LookupIP: lookup,
	})
	adapter := &aiconn.OpenAICompatAdapter{Client: client}
	return aiconn.New(aiconn.Options{
		Store: store, Settings: newMemSettings(), Box: newFakeBox(t), Auth: newFakeAuth(),
		Adapters: map[domain.ProviderKind]aiconn.Adapter{
			domain.KindOpenAICompat: adapter,
			domain.KindOllama:       &aiconn.OllamaAdapter{Client: client},
		},
		Hosted:   hosted,
		LookupIP: lookup,
		Now:      func() time.Time { return time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC) },
	})
}

func TestPatchBaseURLNeverSendsStoredKeyToNewHost(t *testing.T) {
	ctx := context.Background()
	owner, attacker := newAuthRecorder(t), newAuthRecorder(t)
	svc := realAdapterService(t, newMemStore(), false, nil)

	view, err := svc.CreateProvider(ctx, aiconn.CreateProviderInput{
		Kind: domain.KindOpenAICompat, APIKey: "sk-owner-secret", BaseURL: owner.URL + "/v1",
	})
	if err != nil {
		t.Fatalf("tạo provider: %v", err)
	}
	if got := owner.seen(); len(got) != 1 || got[0] != "Bearer sk-owner-secret" {
		t.Fatalf("máy chủ chủ nhận %v", got)
	}

	// Đổi sang máy chủ khác mà không nhập lại khoá → từ chối, không có request nào tới máy lạ.
	newURL := attacker.URL + "/v1"
	_, err = svc.PatchProvider(ctx, view.ID, aiconn.PatchProviderInput{BaseURL: &newURL})
	ae, ok := aiconn.AsAPIError(err)
	if !ok || ae.Status != http.StatusBadRequest || ae.Reason != domain.ReasonMissingKey {
		t.Fatalf("PATCH baseUrl không kèm khoá = %v, muốn 400 missing_key", err)
	}
	if got := attacker.seen(); len(got) != 0 {
		t.Fatalf("khoá đã lưu bị gửi tới máy chủ mới: %v", got)
	}
	// Trạng thái đã lưu không đổi.
	if list, _ := svc.Providers(ctx); len(list) != 1 || list[0].BaseURL != owner.URL+"/v1" {
		t.Fatalf("địa chỉ bị đổi dù bị từ chối: %+v", list)
	}

	// Nhập khoá mới cùng lúc → chỉ khoá mới tới máy mới, khoá cũ không bao giờ.
	fresh := "sk-fresh"
	if _, err := svc.PatchProvider(ctx, view.ID, aiconn.PatchProviderInput{BaseURL: &newURL, APIKey: &fresh}); err != nil {
		t.Fatalf("PATCH kèm khoá mới: %v", err)
	}
	if got := attacker.seen(); len(got) != 1 || got[0] != "Bearer sk-fresh" {
		t.Fatalf("máy chủ mới nhận %v, muốn chỉ khoá mới", got)
	}
	for _, a := range attacker.seen() {
		if strings.Contains(a, "owner-secret") {
			t.Fatalf("khoá cũ lộ tới máy mới: %v", a)
		}
	}
}

func TestPatchBaseURLSameHostKeepsStoredKey(t *testing.T) {
	ctx := context.Background()
	owner := newAuthRecorder(t)
	svc := realAdapterService(t, newMemStore(), false, nil)
	view, err := svc.CreateProvider(ctx, aiconn.CreateProviderInput{
		Kind: domain.KindOpenAICompat, APIKey: "sk-owner-secret", BaseURL: owner.URL + "/v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	other := owner.URL + "/v2"
	if _, err := svc.PatchProvider(ctx, view.ID, aiconn.PatchProviderInput{BaseURL: &other}); err != nil {
		t.Fatalf("đổi đường dẫn cùng máy chủ phải giữ khoá: %v", err)
	}
	if got := owner.seen(); len(got) != 2 || got[1] != "Bearer sk-owner-secret" {
		t.Fatalf("máy chủ chủ nhận %v", got)
	}
}

func TestHostedRejectsInternalCustomAddresses(t *testing.T) {
	ctx := context.Background()
	hosts := map[string][]string{
		"ollama.public.example":     {"93.184.216.34"},
		"rebind.attacker.example":   {"93.184.216.34", "10.0.0.7"},
		"internal.attacker.example": {"192.168.1.20"},
		"metadata.attacker.example": {"169.254.169.254"},
		"loop6.attacker.example":    {"::1"},
		"tailnet.attacker.example":  {"100.100.100.100"},
	}
	lookup := func(_ context.Context, host string) ([]net.IP, error) {
		ips, ok := hosts[host]
		if !ok {
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
		var out []net.IP
		for _, s := range ips {
			out = append(out, net.ParseIP(s))
		}
		return out, nil
	}
	svc := realAdapterService(t, newMemStore(), true, lookup)

	reject := []struct {
		name string
		in   aiconn.CreateProviderInput
	}{
		{"ollama mặc định 127.0.0.1", aiconn.CreateProviderInput{Kind: domain.KindOllama}},
		{"ollama loopback", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://127.0.0.1:8334"}},
		{"ollama Garage S3", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://127.0.0.1:3900"}},
		{"ollama ipv6 loopback", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://[::1]:11434"}},
		{"ollama ipv4 nhúng ipv6", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://[::ffff:127.0.0.1]:11434"}},
		{"ollama mạng riêng", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://10.0.0.5:11434"}},
		{"ollama LAN", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://192.168.1.8:11434"}},
		{"ollama metadata", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://169.254.169.254"}},
		{"ollama unspecified", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://0.0.0.0:11434"}},
		{"ollama tên trỏ vào mạng riêng", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://internal.attacker.example"}},
		{"ollama tên trỏ metadata", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://metadata.attacker.example"}},
		{"ollama tên trỏ ::1", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://loop6.attacker.example"}},
		{"ollama tên trỏ CGNAT", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://tailnet.attacker.example"}},
		{"ollama tên có cả IP công khai lẫn riêng", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://rebind.attacker.example"}},
		{"ollama tên không phân giải được", aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://nowhere.invalid"}},
		{"compat loopback", aiconn.CreateProviderInput{Kind: domain.KindOpenAICompat, APIKey: "sk-x", BaseURL: "http://127.0.0.1:9999/v1"}},
		{"compat localhost", aiconn.CreateProviderInput{Kind: domain.KindOpenAICompat, APIKey: "sk-x", BaseURL: "http://localhost:9999/v1"}},
	}
	for _, tc := range reject {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateProvider(ctx, tc.in)
			ae, ok := aiconn.AsAPIError(err)
			if !ok || ae.Status != http.StatusBadRequest || ae.Reason != domain.ReasonInvalidBaseURL {
				t.Fatalf("lỗi = %v, muốn 400 invalid_base_url", err)
			}
		})
	}
	if list, _ := svc.Providers(ctx); len(list) != 0 {
		t.Fatalf("provider bị từ chối vẫn được lưu: %+v", list)
	}

	// Địa chỉ công khai vẫn dùng được (máy chủ không trả lời → thẻ unreachable nhưng không bị từ chối địa chỉ).
	_, err := svc.CreateProvider(ctx, aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: "http://ollama.public.example:1"})
	if ae, ok := aiconn.AsAPIError(err); ok && ae.Reason == domain.ReasonInvalidBaseURL {
		t.Fatalf("địa chỉ công khai bị từ chối: %v", err)
	}

	// PATCH sang địa chỉ nội bộ cũng bị chặn và không đổi gì đã lưu.
	list, _ := svc.Providers(ctx)
	if len(list) != 1 {
		t.Fatalf("providers = %+v", list)
	}
	internal := "http://127.0.0.1:8334"
	_, err = svc.PatchProvider(ctx, list[0].ID, aiconn.PatchProviderInput{BaseURL: &internal})
	if ae, ok := aiconn.AsAPIError(err); !ok || ae.Reason != domain.ReasonInvalidBaseURL {
		t.Fatalf("PATCH sang loopback = %v, muốn invalid_base_url", err)
	}
	if after, _ := svc.Providers(ctx); after[0].BaseURL != "http://ollama.public.example:1" {
		t.Fatalf("địa chỉ đổi dù bị từ chối: %s", after[0].BaseURL)
	}
}

func TestNotHostedKeepsLocalAddresses(t *testing.T) {
	// Không hosted (app một chủ trên máy cá nhân): Ollama ở loopback/LAN vẫn dùng được.
	svc := realAdapterService(t, newMemStore(), false, func(context.Context, string) ([]net.IP, error) {
		t.Fatal("không hosted thì không được phân giải DNS")
		return nil, nil
	})
	for _, u := range []string{"", "http://127.0.0.1:1", "http://192.168.1.8:1"} {
		_, err := svc.CreateProvider(context.Background(), aiconn.CreateProviderInput{Kind: domain.KindOllama, BaseURL: u})
		if ae, ok := aiconn.AsAPIError(err); ok && ae.Reason == domain.ReasonInvalidBaseURL {
			t.Fatalf("%q bị từ chối khi không hosted: %v", u, err)
		}
	}
}
