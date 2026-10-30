package main

// Nối mảng Kết nối AI (§15) vào composition root: khoá máy, kho credential,
// adapter từng nhà cung cấp, service, route HTTP, chấm trạng thái và chuỗi
// provider của imagegen theo artist.draw / drawFallback.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/aiconn"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/core/auth"
	"github.com/datnt1998/verse/core/auth/codex"
	"github.com/datnt1998/verse/core/auth/xai"
	"github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/platform/events"
	"github.com/jackc/pgx/v5/pgxpool"
)

// aiAttentionSource đưa cảnh báo Kết nối AI (đăng nhập hết hạn, chạm trần
// ảnh ngày) lên chấm bánh răng của /api/status.
type aiAttentionSource struct{ svc *aiconn.Service }

func (s aiAttentionSource) Report(ctx context.Context) (status.Report, error) {
	items, err := s.svc.AttentionReport(ctx)
	if err != nil {
		return status.Report{}, err
	}
	out := make([]status.Attention, 0, len(items))
	for _, it := range items {
		out = append(out, status.Attention{Kind: it.Kind, Text: it.Text, Href: it.Href})
	}
	return status.Report{Attention: out}, nil
}

// aiExecutors buộc chuỗi ModelRef đã chọn vào đường chạy thật: mỗi ref image
// → một imagegen.Provider đúng provider+model (không thay thế ngầm), mỗi ref
// text → một llm.ChatModel đúng provider+model. Pipeline sau lấy `Drawer()`
// bên dưới; aiconn chỉ dùng CanText/CanDraw để từ chối cấu hình không chạy.
type aiExecutors struct {
	svc      *aiconn.Service
	http     *http.Client
	journal  imagegen.Journal
	limit    int
	runner   tools.Runner
	toolKit  *tools.Tools
	coreAuth *aiconn.CoreAuth

	mu     sync.RWMutex
	drawer *imagegen.Service
	chain  []domain.ModelRef
	text   []*llm.ClientModel
	// buildErr: chuỗi đã lưu có ref không dựng được — lỗi hiện ra khi vẽ.
	buildErr string
}

// refKey khoá theo provider+model (không theo kind) — hai thẻ cùng loại vẫn
// là hai đích khác nhau.
func refKey(ref domain.ModelRef) string { return ref.Provider + "\x00" + ref.Model }

// CanText: kind có client text của core/llm và credential đọc được.
func (e *aiExecutors) CanText(ctx context.Context, ref domain.ModelRef) bool {
	p, m, key, err := e.svc.RefInfo(ctx, ref)
	if err != nil {
		return false
	}
	if !hasCapability(m, domain.CapText) && !hasCapability(m, domain.CapVision) {
		return false
	}
	switch p.Kind {
	case domain.KindChatGPT:
		return true // backend Codex qua core/auth
	case domain.KindGrok:
		_, err := e.svc.SubscriptionToken(ctx, p.Kind)
		return err == nil
	case domain.KindOllama:
		return p.BaseURL != ""
	default:
		return key != "" && textProviderName(p.Kind) != ""
	}
}

// CanDraw: đúng cặp kind/model có đường vẽ + credential.
func (e *aiExecutors) CanDraw(ctx context.Context, ref domain.ModelRef) bool {
	p, _, key, err := e.svc.RefInfo(ctx, ref)
	if err != nil {
		return false
	}
	switch p.Kind {
	case domain.KindChatGPT:
		return ref.Model == imagegen.CodexImageModel
	case domain.KindGrok:
		if ref.Model != imagegen.GrokImageModel {
			return false
		}
		_, err := e.svc.SubscriptionToken(ctx, p.Kind)
		return err == nil
	case domain.KindOpenAI, domain.KindOpenAICompat, domain.KindXAI,
		domain.KindOpenRouter, domain.KindGemini:
		return key != ""
	}
	return false
}

// Drawer trả service vẽ đã buộc theo cấu hình hiện tại (pipeline dùng).
func (e *aiExecutors) Drawer() *imagegen.Service {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.drawer
}

// Director trả chuỗi model text đã buộc (pipeline dùng).
func (e *aiExecutors) Director() []*llm.ClientModel {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.text
}

// configure dựng lại cả hai chuỗi theo cấu hình trợ lý vừa lưu.
func (e *aiExecutors) configure(ctx context.Context) error {
	artist, err := e.svc.ResolveArtist(ctx)
	if err != nil {
		return err
	}
	agents, err := e.svc.Agents(ctx)
	if err != nil {
		return err
	}
	director, err := e.svc.ResolveDirector(ctx)
	if err != nil {
		return err
	}
	imageProviders, buildErr := e.imageProviders(ctx, artist.DrawChain)
	textModels := e.textModels(ctx, director)

	e.mu.Lock()
	defer e.mu.Unlock()
	fallback := agents.Artist.FallbackOn
	limit := e.limit
	e.chain = artist.DrawChain
	e.buildErr = buildErr
	e.drawer = imagegen.NewService(imagegen.Options{
		Providers: imageProviders, Fallback: &fallback, Journal: e.journal,
		DailyPaidImageLimit: &limit,
	})
	e.text = textModels
	return nil
}

// imageProviders dựng MỘT provider cho mỗi ModelRef, giữ nguyên thứ tự chuỗi.
// Ref không dựng được KHÔNG bị bỏ im lặng và KHÔNG bị thay bằng provider
// khác: nó giữ đúng vị trí bằng provider luôn lỗi nêu rõ ref, nên chuỗi dự
// phòng vẫn chạy và lỗi thật hiện ra ở lần vẽ.
func (e *aiExecutors) imageProviders(ctx context.Context, chain []domain.ModelRef) ([]imagegen.Provider, string) {
	out := make([]imagegen.Provider, 0, len(chain))
	var problems []string
	for _, ref := range chain {
		p, ok := e.imageProvider(ctx, ref)
		if !ok {
			problems = append(problems, ref.Provider+"/"+ref.Model)
			p = brokenProvider{ref: ref}
		}
		out = append(out, p)
	}
	return out, strings.Join(problems, ", ")
}

// brokenProvider giữ đúng vị trí của một ModelRef không dựng được đường vẽ
// và báo lỗi nêu rõ provider/model — lưới an toàn cho cấu hình cũ đã mất
// provider, không bao giờ vẽ bằng nguồn khác một cách âm thầm.
type brokenProvider struct{ ref domain.ModelRef }

func (b brokenProvider) Name() string  { return b.ref.Provider }
func (b brokenProvider) Model() string { return b.ref.Model }
func (b brokenProvider) Paid() bool    { return true }

func (b brokenProvider) Ready(context.Context) error {
	return imagegen.MarkUnpaid(&imagegen.DrawError{
		Code:     imagegen.ErrCodeToolMissing,
		Message:  "Chưa có đường vẽ cho " + b.ref.Model,
		Recovery: "Mở Cài đặt → Kết nối AI và chọn lại nguồn vẽ",
	})
}

func (b brokenProvider) Draw(context.Context, imagegen.Request, imagegen.JournalHandle) (imagegen.Receipt, error) {
	return imagegen.Receipt{}, b.Ready(context.Background())
}

func (b brokenProvider) Reconcile(context.Context, json.RawMessage, imagegen.Request) (imagegen.Receipt, bool, error) {
	return imagegen.Receipt{}, false, nil
}

// imageProvider dựng đúng provider cho một ref; false khi kind/model không có
// đường vẽ (aiconn đã chặn lúc PUT, đây là lưới an toàn cho cấu hình cũ).
func (e *aiExecutors) imageProvider(ctx context.Context, ref domain.ModelRef) (imagegen.Provider, bool) {
	p, _, _, err := e.svc.RefInfo(ctx, ref)
	if err != nil {
		return nil, false
	}
	keyFn := func(ctx context.Context) (string, error) {
		_, _, k, err := e.svc.RefInfo(ctx, ref)
		if err != nil {
			return "", err
		}
		if k == "" {
			return "", errors.New("chưa có khoá API")
		}
		return k, nil
	}
	switch p.Kind {
	case domain.KindChatGPT:
		if ref.Model != imagegen.CodexImageModel {
			return nil, false
		}
		return imagegen.NewCodexProvider(imagegen.CodexDeps{
			Codex:  e.toolKit.Codex,
			Runner: e.runner,
			Auth: func(ctx context.Context) (tools.CodexAuthTokens, error) {
				return codexTokens(ctx, e.coreAuth)
			},
		}), true
	case domain.KindGrok:
		if ref.Model != imagegen.GrokImageModel {
			return nil, false
		}
		return imagegen.NewGrokProvider(imagegen.GrokDeps{
			Tokens:        xai.ManagerTokens(e.coreAuth.Manager()),
			Client:        e.http,
			ResolveFfmpeg: func(ctx context.Context) (string, error) { return resolveFfmpeg(ctx, e.toolKit) },
			Runner:        e.runner,
			BaseURL:       p.BaseURL,
		}), true
	case domain.KindOpenAI, domain.KindOpenAICompat, domain.KindXAI:
		return imagegen.NewAPIProvider(e.apiDeps(p, ref, imagegen.StyleOpenAIImages, keyFn)), true
	case domain.KindOpenRouter:
		return imagegen.NewAPIProvider(e.apiDeps(p, ref, imagegen.StyleOpenRouter, keyFn)), true
	case domain.KindGemini:
		return imagegen.NewAPIProvider(e.apiDeps(p, ref, imagegen.StyleGemini, keyFn)), true
	}
	return nil, false
}

// apiDeps điền phần chung của provider vẽ HTTP.
func (e *aiExecutors) apiDeps(p domain.Provider, ref domain.ModelRef, style imagegen.APIStyle, key func(context.Context) (string, error)) imagegen.APIDeps {
	return imagegen.APIDeps{
		Name: p.ID, Model: ref.Model, Style: style, Client: e.http,
		BaseURL: p.BaseURL, APIKey: key,
		ResolveFfmpeg: func(ctx context.Context) (string, error) { return resolveFfmpeg(ctx, e.toolKit) },
		Runner:        e.runner,
	}
}

// textModels dựng client text cho từng ModelRef của Đạo diễn, giữ thứ tự.
func (e *aiExecutors) textModels(ctx context.Context, chain []domain.ModelRef) []*llm.ClientModel {
	out := make([]*llm.ClientModel, 0, len(chain))
	for _, ref := range chain {
		if m, err := e.textModel(ctx, ref); err == nil {
			out = append(out, m)
		}
	}
	return out
}

// textModel dựng một client text cho ref (chatgpt qua codex backend, grok qua
// token OAuth, còn lại qua litellm với khoá/địa chỉ của thẻ).
func (e *aiExecutors) textModel(ctx context.Context, ref domain.ModelRef) (*llm.ClientModel, error) {
	p, _, key, err := e.svc.RefInfo(ctx, ref)
	if err != nil {
		return nil, err
	}
	switch p.Kind {
	case domain.KindChatGPT:
		return codex.NewChatModel(ref.Model, codex.Config{
			Tokens: codex.ManagerTokens(e.coreAuth.Manager()),
		})
	case domain.KindGrok:
		token, err := e.svc.SubscriptionToken(ctx, p.Kind)
		if err != nil {
			return nil, err
		}
		return llm.NewModel("grok", ref.Model, llm.WithAPIKey(token), llm.WithBaseURL(p.BaseURL))
	}
	name := textProviderName(p.Kind)
	if name == "" {
		return nil, fmt.Errorf("không có client text cho %s", p.Kind)
	}
	opts := []llm.ModelOption{llm.WithAPIKey(key)}
	if p.BaseURL != "" {
		opts = append(opts, llm.WithBaseURL(p.BaseURL))
	}
	return llm.NewModel(name, ref.Model, opts...)
}

// textProviderName ánh kind Kết nối AI sang tên provider của core/llm.
func textProviderName(kind domain.ProviderKind) string {
	switch kind {
	case domain.KindOpenAI, domain.KindOpenAICompat:
		return "openai"
	case domain.KindGemini:
		return "gemini"
	case domain.KindOpenRouter:
		return "openrouter"
	case domain.KindDeepSeek:
		return "deepseek"
	case domain.KindXAI:
		return "grok"
	case domain.KindAnthropic:
		return "anthropic"
	case domain.KindOllama:
		return "ollama"
	}
	return ""
}

// hasCapability báo caps chứa need.
func hasCapability(m domain.AIModel, need domain.Capability) bool {
	for _, c := range m.Caps {
		if c == need {
			return true
		}
	}
	return false
}

// aiWiring là kết quả nối Kết nối AI: Service cho HTTP/tools và Executors
// (đường vẽ + model text đã buộc) cho các phase pipeline sau.
type aiWiring struct {
	Service   *aiconn.Service
	Executors *aiExecutors
}

// wireAI dựng toàn bộ Kết nối AI. Service trả cho block khác dùng
// (toolsvc.Deps.ChatGPT cần ChatGPTLogin); Executors là consumer-side thật
// của cấu hình trợ lý — pipeline lấy Drawer()/Director() từ đó.
func wireAI(
	ctx context.Context,
	gateway *web.Gateway,
	pool *pgxpool.Pool,
	settings *postgres.SettingsStore,
	hub *events.Hub,
	statusSvc *status.Service,
	cfgDir string,
	toolKit *tools.Tools,
	runner tools.Runner,
) (*aiWiring, error) {
	box := secretbox.New(cfgDir)
	authStore, err := auth.DefaultStore()
	if err != nil {
		return nil, fmt.Errorf("mở kho đăng nhập: %w", err)
	}
	manager := auth.NewManager(authStore)
	coreAuth := aiconn.NewCoreAuth(manager, settings)
	store := postgres.NewAIConnStore(pool)
	receipts := postgres.NewImageReceiptStore(pool)

	// Danh mục ChatGPT chạy `codex debug models --bundled` trên binary đã chọn;
	// Codex có thể chưa cài — trả lỗi rõ để thẻ báo unreachable.
	chatgptCatalog := func(ctx context.Context) ([]domain.AIModel, error) {
		exe := toolKit.Codex.Resolve(ctx)
		if exe == "" {
			return nil, errors.New("chưa có Codex CLI")
		}
		infos, err := toolKit.Codex.DebugModels(ctx, exe)
		if err != nil {
			return nil, err
		}
		out := make([]domain.AIModel, 0, len(infos))
		for _, info := range infos {
			var caps []domain.Capability
			vision := false
			for _, m := range info.InputModalities {
				if m == "image" {
					vision = true
				}
			}
			caps = append(caps, domain.CapText)
			if vision {
				caps = append(caps, domain.CapVision)
			}
			out = append(out, domain.AIModel{
				ID: info.Slug, Caps: caps, ContextK: info.ContextWindow / 1000,
			})
		}
		return out, nil
	}
	// Danh mục text của Grok đi qua API xAI với credential OAuth của app.
	grokModels := func(ctx context.Context) ([]domain.AIModel, error) {
		list, err := xai.ListModels(ctx, xai.DefaultBaseURL, xai.ManagerTokens(manager))
		if err != nil {
			return nil, err
		}
		out := make([]domain.AIModel, 0, len(list))
		for _, m := range list {
			caps := []domain.Capability{domain.CapText}
			if len(m.Slug) >= 6 && m.Slug[:6] == "grok-4" {
				caps = append(caps, domain.CapVision)
			}
			out = append(out, domain.AIModel{ID: m.Slug, Caps: caps})
		}
		return out, nil
	}

	// Client dùng chung: giới hạn redirect cross-origin/hạ cấp https để khoá
	// API không bị chuyển tiếp sang máy khác (SSRF phần redirect).
	httpClient := aiconn.NewCredentialClient(30 * time.Second)
	limit := imagegen.DefaultDailyPaidImageLimit

	var svc *aiconn.Service
	exec := &aiExecutors{
		http: httpClient, journal: receipts, limit: limit,
		runner: runner, toolKit: toolKit, coreAuth: coreAuth,
	}
	svc = aiconn.New(aiconn.Options{
		Store: store, Settings: settings, Box: box, Auth: coreAuth,
		Adapters: map[domain.ProviderKind]aiconn.Adapter{
			domain.KindChatGPT:      &aiconn.ChatGPTAdapter{Catalog: chatgptCatalog},
			domain.KindGrok:         &aiconn.GrokAdapter{Models: grokModels},
			domain.KindOpenAI:       &aiconn.OpenAICompatAdapter{Client: httpClient},
			domain.KindOpenRouter:   &aiconn.OpenAICompatAdapter{Client: httpClient},
			domain.KindDeepSeek:     &aiconn.OpenAICompatAdapter{Client: httpClient},
			domain.KindXAI:          &aiconn.OpenAICompatAdapter{Client: httpClient},
			domain.KindOpenAICompat: &aiconn.OpenAICompatAdapter{Client: httpClient},
			domain.KindGemini:       &aiconn.GeminiAdapter{Client: httpClient},
			domain.KindAnthropic:    &aiconn.AnthropicAdapter{Client: httpClient},
			domain.KindOllama:       &aiconn.OllamaAdapter{Client: httpClient},
		},
		PaidUsage: func(ctx context.Context, provider string, since time.Time) (int64, error) {
			return receipts.Charged(ctx, provider, since)
		},
		DailyImageLimit: limit,
		QuotaReset: func(provider string) (time.Time, bool) {
			if exec.drawer == nil {
				return time.Time{}, false
			}
			return exec.drawer.QuotaReset(provider)
		},
		Notify:     web.Notifier(hub, "ai"),
		StatusBump: func() { statusSvc.Bump() },
		Lifetime:   ctx,
		Executors:  exec,
		OnAgentsChanged: func() {
			// Dựng lại ĐÚNG chuỗi đã lưu; lỗi dựng thể hiện khi Draw, không
			// nuốt im lặng.
			if err := exec.configure(context.Background()); err != nil {
				fmt.Printf("Cấu hình trợ lý AI: %v\n", err)
			}
		},
	})
	exec.svc = svc
	statusSvc.RegisterSource(aiAttentionSource{svc: svc})
	web.MountAI(gateway, svc)

	// Dựng chuỗi vẽ/text ban đầu từ cấu hình đã lưu.
	if err := exec.configure(ctx); err != nil {
		return nil, err
	}

	// Làm ấm danh mục của thẻ đăng nhập gói ở nền: GET luôn đọc DB nên
	// không phải chờ mạng; xong thì bump status để UI tải lại. Bám vòng đời
	// serve — dừng app là dừng luồng ấm, không rò goroutine.
	svc.Go(func(ctx context.Context) {
		warm, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		if err := svc.RefreshSubscribed(warm); err != nil {
			fmt.Printf("Danh mục Kết nối AI: %v\n", err)
		}
		statusSvc.Bump()
	})
	return &aiWiring{Service: svc, Executors: exec}, nil
}

// codexTokens lấy token CLI-safe từ core/auth (tự refresh nếu còn dưới 15
// phút) để dựng auth.json cho Codex CLI cô lập.
func codexTokens(ctx context.Context, coreAuth *aiconn.CoreAuth) (tools.CodexAuthTokens, error) {
	access, accountID, err := coreAuth.CLITokens(ctx, auth.OpenAICodexID)
	if err != nil {
		return tools.CodexAuthTokens{}, err
	}
	return tools.CodexAuthTokens{AccessToken: access, AccountID: accountID}, nil
}

// resolveFfmpeg trả đường ffmpeg đã chọn cho bước transcode ảnh Grok.
func resolveFfmpeg(ctx context.Context, kit *tools.Tools) (string, error) {
	res := kit.Ffmpeg.Resolve(ctx)
	if res == nil || res.Ffmpeg == "" {
		return "", errors.New("chưa có ffmpeg")
	}
	return res.Ffmpeg, nil
}
