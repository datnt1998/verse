// Test từng adapter của Kết nối AI với httptest: thành công, 401, JSON hỏng,
// danh sách rỗng. Không gọi mạng thật.
package aiconn_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/aiconn"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func providerAt(server string) domain.Provider {
	return domain.Provider{ID: "p", Kind: domain.KindOpenAI, Auth: domain.AuthAPIKey, BaseURL: server}
}

func TestOpenAICompatCatalog(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		want    int
		auth    bool
		unreach bool
	}{
		{"ok", 200, `{"data":[
			{"id":"gpt-6.1","context_length":272000,"pricing":{"prompt":"0.000002","completion":"0.000008"},
			 "architecture":{"input_modalities":["text","image"],"output_modalities":["text"]}},
			{"id":"gpt-image-2","pricing":{"image":"0.04"},
			 "architecture":{"input_modalities":["text"],"output_modalities":["image"]}}
		]}`, 2, false, false},
		{"401", 401, `{"error":"bad key"}`, 0, true, false},
		{"503", 503, `upstream`, 0, false, true},
		{"hỏng", 200, `not json`, 0, false, false},
		{"rỗng", 200, `{"data":[]}`, 0, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/models" {
					t.Errorf("path = %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer sk-test" {
					t.Errorf("auth header = %q", r.Header.Get("Authorization"))
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			a := &aiconn.OpenAICompatAdapter{Client: srv.Client()}
			models, err := a.FetchCatalog(context.Background(), providerAt(srv.URL), "sk-test")
			switch {
			case tc.auth:
				if !aiconn.IsAuthError(err) {
					t.Fatalf("muốn lỗi auth: %v", err)
				}
			case tc.unreach:
				if !aiconn.IsUnreachable(err) {
					t.Fatalf("muốn lỗi unreachable: %v", err)
				}
			case tc.name == "hỏng":
				var ce *aiconn.CatalogError
				if !asCatalog(err, &ce) || ce.Kind != "failed" {
					t.Fatalf("muốn lỗi failed: %v", err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if len(models) != tc.want {
					t.Fatalf("models = %d", len(models))
				}
			}
			if tc.name == "ok" {
				if models[0].ContextK != 272 || models[0].InPerM == 0 {
					t.Fatalf("metadata = %+v", models[0])
				}
				wantCaps := []string{domain.CapText, domain.CapVision}
				if len(models[0].Caps) != len(wantCaps) {
					t.Fatalf("caps = %v", models[0].Caps)
				}
				if len(models[1].Caps) != 1 || models[1].Caps[0] != domain.CapImage {
					t.Fatalf("caps ảnh = %v", models[1].Caps)
				}
				if models[1].PerImage != 0.04 {
					t.Fatalf("perImage = %v", models[1].PerImage)
				}
			}
		})
	}
}

func asCatalog(err error, out **aiconn.CatalogError) bool {
	if err == nil {
		return false
	}
	ce, ok := err.(*aiconn.CatalogError)
	if ok {
		*out = ce
	}
	return ok
}

func TestOpenAICompatCheckModel(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"h"}}]}`))
	}))
	defer srv.Close()
	a := &aiconn.OpenAICompatAdapter{Client: srv.Client()}
	if _, err := a.CheckModel(context.Background(), providerAt(srv.URL), "k", "custom"); err != nil {
		t.Fatal(err)
	}
	if got["model"] != "custom" || got["max_tokens"] != float64(1) {
		t.Fatalf("payload = %+v", got)
	}
}

func TestGeminiCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "gk" {
			t.Errorf("key header = %q", r.Header.Get("x-goog-api-key"))
		}
		_, _ = w.Write([]byte(`{"models":[
			{"name":"models/gemini-3-pro","supportedGenerationMethods":["generateContent"],"inputTokenLimit":1048576},
			{"name":"models/text-embedding-004","supportedGenerationMethods":["embedContent"]}
		]}`))
	}))
	defer srv.Close()
	p := providerAt(srv.URL)
	p.Kind = domain.KindGemini
	a := &aiconn.GeminiAdapter{Client: srv.Client()}
	models, err := a.FetchCatalog(context.Background(), p, "gk")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "gemini-3-pro" || models[0].ContextK != 1048 {
		t.Fatalf("models = %+v", models)
	}
}

func TestGeminiAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":{"message":"API key not valid"}}`))
	}))
	defer srv.Close()
	p := providerAt(srv.URL)
	p.Kind = domain.KindGemini
	if _, err := (&aiconn.GeminiAdapter{Client: srv.Client()}).FetchCatalog(context.Background(), p, "bad"); !aiconn.IsAuthError(err) {
		t.Fatalf("muốn auth: %v", err)
	}
}

func TestAnthropicCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "ak" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("headers = %v", r.Header)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet-5"},{"id":"claude-opus-5"}]}`))
	}))
	defer srv.Close()
	p := providerAt(srv.URL)
	p.Kind = domain.KindAnthropic
	models, err := (&aiconn.AnthropicAdapter{Client: srv.Client()}).FetchCatalog(context.Background(), p, "ak")
	if err != nil || len(models) != 2 {
		t.Fatalf("models = %v %v", models, err)
	}
	for _, m := range models {
		if len(m.Caps) != 2 {
			t.Fatalf("claude phải đọc được ảnh: %+v", m)
		}
	}
}

func TestOllamaCatalogAndCheck(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[
			{"name":"llama3.2:3b","details":{"families":["llama"]}},
			{"name":"llava:latest","details":{"families":["llama","clip"]}}
		]}`))
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"capabilities":["completion","vision"],
			"model_info":{"llama.context_length":131072}}`))
	})
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != false {
			t.Errorf("stream phải false: %v", body)
		}
		_, _ = w.Write([]byte(`{"response":"h","done":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	p := providerAt(srv.URL)
	p.Kind = domain.KindOllama
	a := &aiconn.OllamaAdapter{Client: srv.Client()}
	models, err := a.FetchCatalog(context.Background(), p, "")
	if err != nil || len(models) != 2 {
		t.Fatalf("models = %v %v", models, err)
	}
	if len(models[1].Caps) != 2 {
		t.Fatalf("llava phải có vision: %+v", models[1])
	}
	checked, err := a.CheckModel(context.Background(), p, "", "llava:latest")
	if err != nil {
		t.Fatal(err)
	}
	if checked.ContextK != 131 || len(checked.Caps) != 2 {
		t.Fatalf("checked = %+v", checked)
	}
}

func TestOllamaDownIsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // tắt để dial thất bại
	p := providerAt(url)
	p.Kind = domain.KindOllama
	_, err := (&aiconn.OllamaAdapter{}).FetchCatalog(context.Background(), p, "")
	if !aiconn.IsUnreachable(err) {
		t.Fatalf("muốn unreachable: %v", err)
	}
}

func TestChatGPTAdapterCatalogAddsImageModel(t *testing.T) {
	a := &aiconn.ChatGPTAdapter{Catalog: func(ctx context.Context) ([]domain.AIModel, error) {
		return []domain.AIModel{{ID: "gpt-6.1-sol", Caps: []domain.Capability{domain.CapText, domain.CapVision}}}, nil
	}}
	models, err := a.FetchCatalog(context.Background(), domain.Provider{}, "")
	if err != nil || len(models) != 2 {
		t.Fatalf("models = %v %v", models, err)
	}
	if models[1].ID != "gpt-image-2" {
		t.Fatalf("thiếu model ảnh: %+v", models[1])
	}
	if _, err := a.CheckModel(context.Background(), domain.Provider{}, "", "nope"); err == nil {
		t.Fatal("model lạ vẫn qua check")
	}
}

func TestEnrichFillsPricesFromRegistry(t *testing.T) {
	f := newFixture(t)
	f.adapter.catalog = []domain.AIModel{{ID: "deepseek-chat"}}
	view, err := f.svc.CreateProvider(context.Background(), aiconn.CreateProviderInput{
		Kind: domain.KindDeepSeek, APIKey: "sk-enrich",
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := f.svc.Models(context.Background(), view.ID, "", "all", "", 0)
	if err != nil || len(page.Models) != 1 {
		t.Fatalf("models = %v %v", page, err)
	}
	if page.Models[0].ContextK == 0 || page.Models[0].InPerM == 0 || len(page.Models[0].Caps) == 0 {
		t.Fatalf("registry không điền metadata: %+v", page.Models[0])
	}
}

// /api/show là nguồn quyết định capability: model chỉ embedding (bge-m3…) KHÔNG
// được gán "text" — nếu không trợ lý có thể chọn nó làm model text rồi hỏng.
func TestOllamaEmbeddingOnlyModelHasNoTextCap(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[
			{"name":"bge-m3:latest","details":{"families":["bert"]}},
			{"name":"qwen3:8b","details":{"families":["qwen3"]}}
		]}`))
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Model == "bge-m3:latest" {
			_, _ = w.Write([]byte(`{"capabilities":["embedding"],"model_info":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"capabilities":["completion"],"model_info":{"qwen3.context_length":32768}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	p := providerAt(srv.URL)
	p.Kind = domain.KindOllama
	models, err := (&aiconn.OllamaAdapter{Client: srv.Client()}).FetchCatalog(context.Background(), p, "")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]domain.AIModel{}
	for _, m := range models {
		byID[m.ID] = m
	}
	if len(byID["bge-m3:latest"].Caps) != 0 {
		t.Fatalf("embedding-only bị gán caps: %+v", byID["bge-m3:latest"])
	}
	if len(byID["qwen3:8b"].Caps) != 1 || byID["qwen3:8b"].ContextK != 32 {
		t.Fatalf("qwen3 = %+v", byID["qwen3:8b"])
	}
}

// Bản Ollama cũ không trả capabilities: rơi về heuristic families, và model chỉ
// embedding vẫn không có text.
func TestOllamaShowWithoutCapabilitiesFallsBack(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"name":"llava:latest","details":{"families":["llama","clip"]}}]}`))
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"model_info":{}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	p := providerAt(srv.URL)
	p.Kind = domain.KindOllama
	models, err := (&aiconn.OllamaAdapter{Client: srv.Client()}).FetchCatalog(context.Background(), p, "")
	if err != nil || len(models) != 1 {
		t.Fatalf("models = %v %v", models, err)
	}
	if len(models[0].Caps) != 2 {
		t.Fatalf("llava phải có text+vision: %+v", models[0])
	}
}

// Client mang credential không theo redirect khác máy chủ hay hạ cấp https.
func TestSameOriginRedirect(t *testing.T) {
	mk := func(scheme, host string) *http.Request {
		r, _ := http.NewRequest(http.MethodGet, scheme+"://"+host+"/x", nil)
		return r
	}
	same := mk("https", "api.example.com")
	if err := aiconn.SameOriginRedirect(mk("https", "api.example.com"), []*http.Request{same}); err != nil {
		t.Fatalf("cùng máy chủ phải cho qua: %v", err)
	}
	if err := aiconn.SameOriginRedirect(mk("https", "evil.example.com"), []*http.Request{same}); err == nil {
		t.Fatal("cross-origin phải bị chặn")
	}
	secure := mk("https", "api.example.com")
	if err := aiconn.SameOriginRedirect(mk("http", "api.example.com"), []*http.Request{secure}); err == nil {
		t.Fatal("https→http phải bị chặn")
	}
}
