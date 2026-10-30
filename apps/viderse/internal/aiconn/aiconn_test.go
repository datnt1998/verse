// Test hành vi của kết nối AI theo hợp đồng dữ liệu §15.
package aiconn_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/aiconn"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
)

// ── fakes ──────────────────────────────────────────────────────────────────

type memStore struct {
	mu            sync.Mutex
	providers     map[string]domain.Provider
	models        map[string]map[string]domain.AIModel
	listProviders int
}

func newMemStore(seed ...domain.Provider) *memStore {
	s := &memStore{providers: map[string]domain.Provider{}, models: map[string]map[string]domain.AIModel{}}
	for _, p := range seed {
		s.providers[p.ID] = p
		s.models[p.ID] = map[string]domain.AIModel{}
	}
	return s
}

func (s *memStore) ListProviders(ctx context.Context) ([]aiconn.ProviderSummary, error) {
	s.mu.Lock()
	s.listProviders++
	s.mu.Unlock()
	rows, err := s.summaries(func(domain.Provider) bool { return true })
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// listProvidersCalls đếm lần đọc TOÀN BỘ provider — attention không được gọi.
func (s *memStore) listProvidersCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listProviders
}

// ListSubscribed chỉ trả thẻ đăng nhập gói (giống truy vấn thật).
func (s *memStore) ListSubscribed(ctx context.Context) ([]aiconn.ProviderSummary, error) {
	return s.summaries(func(p domain.Provider) bool { return p.Auth == domain.AuthSubscription })
}

func (s *memStore) summaries(keep func(domain.Provider) bool) ([]aiconn.ProviderSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]aiconn.ProviderSummary, 0, len(s.providers))
	for _, p := range s.providers {
		if !keep(p) {
			continue
		}
		out = append(out, aiconn.ProviderSummary{Provider: p, Count: len(s.models[p.ID])})
	}
	slices.SortFunc(out, func(a, b aiconn.ProviderSummary) int { return strings.Compare(a.Provider.ID, b.Provider.ID) })
	return out, nil
}

// UpdateProviderKey chỉ đổi khoá/địa chỉ/trạng thái (không đụng selected).
func (s *memStore) UpdateProviderKey(ctx context.Context, id string, encKey []byte, keyHint, baseURL string, status domain.AIStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.providers[id]
	p.EncKey, p.KeyHint, p.BaseURL, p.Status = encKey, keyHint, baseURL, status
	s.providers[id] = p
	return nil
}

// SearchModels lọc q/filter + keyset + limit như SQL.
func (s *memStore) SearchModels(ctx context.Context, providerID string, q aiconn.ModelQuery) (aiconn.ModelPage, error) {
	all, err := s.ListModels(ctx, providerID)
	if err != nil {
		return aiconn.ModelPage{}, err
	}
	p, _ := s.GetProvider(ctx, providerID)
	matched := make([]domain.AIModel, 0, len(all))
	for _, m := range all {
		if !matchesModelQuery(m, p, q) || (q.After != "" && m.ID <= q.After) {
			continue
		}
		matched = append(matched, m)
	}
	if q.Limit > 0 && len(matched) > q.Limit {
		matched = matched[:q.Limit]
	}
	return aiconn.ModelPage{Models: matched}, nil
}

func (s *memStore) CountModels(ctx context.Context, providerID string, q aiconn.ModelQuery) (int, error) {
	all, err := s.ListModels(ctx, providerID)
	if err != nil {
		return 0, err
	}
	p, _ := s.GetProvider(ctx, providerID)
	n := 0
	for _, m := range all {
		if matchesModelQuery(m, p, q) {
			n++
		}
	}
	return n, nil
}

func matchesModelQuery(m domain.AIModel, p domain.Provider, q aiconn.ModelQuery) bool {
	if q.Q != "" && !strings.Contains(strings.ToLower(m.ID), q.Q) {
		return false
	}
	switch q.Filter {
	case "", "all":
		return true
	case "selected":
		for _, id := range p.Selected {
			if id == m.ID {
				return true
			}
		}
		return false
	case "recommended":
		return m.Recommended
	}
	return false
}

func (s *memStore) GetProvider(ctx context.Context, id string) (domain.Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.providers[id], nil
}

func (s *memStore) UpsertProvider(ctx context.Context, p domain.Provider) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[p.ID] = p
	if s.models[p.ID] == nil {
		s.models[p.ID] = map[string]domain.AIModel{}
	}
	return nil
}

func (s *memStore) SetSelected(ctx context.Context, id string, selected []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.providers[id]
	p.Selected = selected
	s.providers[id] = p
	return nil
}

func (s *memStore) SetStatus(ctx context.Context, id string, st domain.AIStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.providers[id]
	p.Status = st
	s.providers[id] = p
	return nil
}

func (s *memStore) DeleteProvider(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.providers, id)
	delete(s.models, id)
	return nil
}

func (s *memStore) ListModels(ctx context.Context, providerID string) ([]domain.AIModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.AIModel, 0, len(s.models[providerID]))
	for _, m := range s.models[providerID] {
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b domain.AIModel) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func (s *memStore) GetModel(ctx context.Context, providerID, modelID string) (domain.AIModel, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.models[providerID][modelID]
	return m, ok, nil
}

func (s *memStore) ReplaceCatalog(ctx context.Context, providerID string, models []domain.AIModel, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.models[providerID]
	if bucket == nil {
		bucket = map[string]domain.AIModel{}
		s.models[providerID] = bucket
	}
	for id, m := range bucket {
		if !m.Manual {
			delete(bucket, id)
		}
	}
	for _, m := range models {
		bucket[m.ID] = m
	}
	p := s.providers[providerID]
	p.CatalogAt = at.UnixMilli()
	s.providers[providerID] = p
	return nil
}

func (s *memStore) UpsertModel(ctx context.Context, providerID string, m domain.AIModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.models[providerID] == nil {
		s.models[providerID] = map[string]domain.AIModel{}
	}
	s.models[providerID][m.ID] = m
	return nil
}

type memSettings struct {
	mu   sync.Mutex
	data map[string]any
}

func newMemSettings() *memSettings { return &memSettings{data: map[string]any{}} }

func (s *memSettings) Get(ctx context.Context, key string, out any) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	if !ok {
		return false, nil
	}
	switch dst := out.(type) {
	case *domain.AgentModels:
		*dst = v.(domain.AgentModels)
	case *bool:
		*dst = v.(bool)
	default:
		return false, fmt.Errorf("kiểu đích không hỗ trợ")
	}
	return true, nil
}

func (s *memSettings) Set(ctx context.Context, key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

// fakeBox là secretbox trong bộ nhớ dùng khoá máy tạm cho test.
func newFakeBox(t *testing.T) *secretbox.Box {
	t.Helper()
	return secretbox.New(t.TempDir())
}

// fakeAuth là Auth trong bộ nhớ.
type fakeAuth struct {
	mu       sync.Mutex
	local    map[string]aiconn.OAuthStatusLocal
	creds    map[string]string
	loginErr error
	// started đóng khi Login bắt đầu (test huỷ giữa chừng).
	started chan struct{}
	// block giữ Login tới khi ctx bị huỷ (mô phỏng chờ người dùng).
	block bool
	// loginScript chạy callback rồi trả kết quả.
	loginScript func(ctrl aiconn.LoginController, manual <-chan aiconn.ManualCode) (string, error)
	revoked     map[string]bool
	calls       []string
}

func newFakeAuth() *fakeAuth {
	return &fakeAuth{local: map[string]aiconn.OAuthStatusLocal{}, creds: map[string]string{}, revoked: map[string]bool{}}
}

func (a *fakeAuth) LocalStatus(ctx context.Context, oauthID string) (aiconn.OAuthStatusLocal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.local[oauthID], nil
}

func (a *fakeAuth) Credential(ctx context.Context, oauthID string) (string, string, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, "credential:"+oauthID)
	if err := a.loginErr; err != nil {
		return "", "", "", err
	}
	access, ok := a.creds[oauthID]
	if !ok {
		return "", "", "", aiconn.ErrNoCredential
	}
	return access, "acct", "me@example.com · plus", nil
}

func (a *fakeAuth) MarkRevoked(ctx context.Context, oauthID string, revoked bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revoked[oauthID] = revoked
	return nil
}

func (a *fakeAuth) Login(ctx context.Context, oauthID string, device bool, ctrl aiconn.LoginController, manual <-chan aiconn.ManualCode) (string, error) {
	if a.started != nil {
		select {
		case <-a.started:
		default:
			close(a.started)
		}
	}
	if a.block {
		ctrl.OnAuth("https://auth.example/authorize", "chờ")
		<-ctx.Done()
		return "", ctx.Err()
	}
	if a.loginScript != nil {
		return a.loginScript(ctrl, manual)
	}
	ctrl.OnAuth("https://auth.example/authorize", "hoàn tất")
	return "me@example.com · plus", nil
}

func (a *fakeAuth) Logout(ctx context.Context, oauthID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.creds, oauthID)
	a.local[oauthID] = aiconn.OAuthAbsent
	return nil
}

func (a *fakeAuth) CLITokens(ctx context.Context, oauthID string) (string, string, error) {
	access, ok := a.creds[oauthID]
	if !ok {
		return "", "", aiconn.ErrNoCredential
	}
	return access, "acct", nil
}

// fakeAdapter là Adapter có kết quả cố định.
type fakeAdapter struct {
	catalog []domain.AIModel
	err     error
	checked domain.AIModel
	checkn  string
	keySeen string
}

func (a *fakeAdapter) FetchCatalog(ctx context.Context, p domain.Provider, key string) ([]domain.AIModel, error) {
	a.keySeen = key
	if a.err != nil {
		return nil, a.err
	}
	return a.catalog, nil
}

func (a *fakeAdapter) CheckModel(ctx context.Context, p domain.Provider, key, model string) (domain.AIModel, error) {
	if a.err != nil {
		return domain.AIModel{}, a.err
	}
	m := a.checked
	m.ID = model
	return m, nil
}

// fixture dựng service với store/auth/settings/adapter cho test.
type fixture struct {
	svc      *aiconn.Service
	store    *memStore
	settings *memSettings
	auth     *fakeAuth
	box      *secretbox.Box
	adapter  *fakeAdapter
	notifies []string
	changed  int
}

func newFixture(t *testing.T, seed ...domain.Provider) *fixture {
	t.Helper()
	f := &fixture{
		store:    newMemStore(seed...),
		settings: newMemSettings(),
		auth:     newFakeAuth(),
		box:      newFakeBox(t),
		adapter: &fakeAdapter{
			catalog: []domain.AIModel{
				{ID: "alpha", Caps: []domain.Capability{domain.CapText}},
				{ID: "beta", Caps: []domain.Capability{domain.CapText, domain.CapVision}, Recommended: true},
				{ID: "gamma", Caps: []domain.Capability{domain.CapImage}},
			},
		},
	}
	f.svc = aiconn.New(aiconn.Options{
		Store: f.store, Settings: f.settings, Box: f.box, Auth: f.auth,
		Adapters: map[domain.ProviderKind]aiconn.Adapter{
			domain.KindOpenAI:       f.adapter,
			domain.KindOpenRouter:   f.adapter,
			domain.KindOllama:       f.adapter,
			domain.KindDeepSeek:     f.adapter,
			domain.KindOpenAICompat: f.adapter,
		},
		Notify:     func(ref string) { f.notifies = append(f.notifies, ref) },
		StatusBump: func() { f.changed++ },
		Now:        func() time.Time { return time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC) },
	})
	return f
}

func seedSubscriptions() []domain.Provider {
	return []domain.Provider{
		{ID: "chatgpt", Kind: domain.KindChatGPT, Auth: domain.AuthSubscription, Label: "ChatGPT",
			Status: domain.StatusReady, Selected: []string{"gpt-6.1-sol", "gpt-image-2"}},
		{ID: "grok", Kind: domain.KindGrok, Auth: domain.AuthSubscription, Label: "Grok",
			Status: domain.StatusReady, Selected: []string{"grok-imagine-image"}},
	}
}

// seedDefaultModels thêm đúng các model mà DefaultAgentModels trỏ tới.
func seedDefaultModels(t *testing.T, f *fixture) {
	t.Helper()
	ctx := context.Background()
	_ = f.store.UpsertModel(ctx, "chatgpt", domain.AIModel{ID: "gpt-6.1-sol",
		Caps: []domain.Capability{domain.CapText, domain.CapVision}, ContextK: 272})
	_ = f.store.UpsertModel(ctx, "chatgpt", domain.AIModel{ID: "gpt-image-2",
		Caps: []domain.Capability{domain.CapImage}})
	_ = f.store.UpsertModel(ctx, "grok", domain.AIModel{ID: "grok-imagine-image",
		Caps: []domain.Capability{domain.CapImage}})
}

// ── tạo / khoá / danh mục ──────────────────────────────────────────────────

func TestCreateProviderSealsKeyAndFetchesCatalog(t *testing.T) {
	f := newFixture(t)
	view, err := f.svc.CreateProvider(context.Background(), aiconn.CreateProviderInput{
		Kind: domain.KindOpenAI, APIKey: "sk-live-abc7c2e",
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != domain.StatusReady || view.Count != 3 || view.CatalogAt == 0 {
		t.Fatalf("view = %+v", view)
	}
	if view.KeyHint != "••••7c2e" {
		t.Fatalf("keyHint = %q", view.KeyHint)
	}
	// Khoá plaintext không bao giờ nằm trong bản ghi store.
	row := f.store.providers[view.ID]
	if string(row.EncKey) == "sk-live-abc7c2e" || strings.Contains(string(row.EncKey), "sk-live") {
		t.Fatal("store giữ plaintext khoá API")
	}
	if f.adapter.keySeen != "sk-live-abc7c2e" {
		t.Fatalf("adapter không nhận khoá: %q", f.adapter.keySeen)
	}
	if len(f.notifies) == 0 || f.notifies[0] != view.ID {
		t.Fatalf("không phát sự kiện ai: %v", f.notifies)
	}
}

func TestCreateProviderWrongKeyStillCreatesCard(t *testing.T) {
	f := newFixture(t)
	f.adapter.err = &aiconn.CatalogError{Kind: "auth", Status: 401, Message: "401"}
	view, err := f.svc.CreateProvider(context.Background(), aiconn.CreateProviderInput{
		Kind: domain.KindOpenAI, APIKey: "sk-bad",
	})
	ae, ok := aiconn.AsAPIError(err)
	if !ok || ae.Reason != domain.ReasonAuthFailed || ae.Status != http.StatusUnprocessableEntity {
		t.Fatalf("lỗi = %v", err)
	}
	if ae.Fields["status"] != 401 {
		t.Fatalf("fields = %+v", ae.Fields)
	}
	if view.ID == "" || view.Status != domain.StatusAuthFailed {
		t.Fatalf("thẻ không được tạo: %+v", view)
	}
	if f.store.providers[view.ID].Status != domain.StatusAuthFailed {
		t.Fatal("store không giữ auth_failed")
	}
}

func TestCreateProviderValidation(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name   string
		in     aiconn.CreateProviderInput
		status int
		reason string
	}{
		{"kind lạ", aiconn.CreateProviderInput{Kind: "acme"}, 400, domain.ReasonInvalidKind},
		{"managed", aiconn.CreateProviderInput{Kind: domain.KindChatGPT}, 409, domain.ReasonManaged},
		{"thiếu khoá", aiconn.CreateProviderInput{Kind: domain.KindOpenAI}, 400, domain.ReasonMissingKey},
		{"compat thiếu url", aiconn.CreateProviderInput{Kind: domain.KindOpenAICompat, APIKey: "k"}, 400, domain.ReasonMissingBaseURL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.CreateProvider(context.Background(), tc.in)
			ae, ok := aiconn.AsAPIError(err)
			if !ok || ae.Status != tc.status || ae.Reason != tc.reason {
				t.Fatalf("được %v, muốn %d %s", err, tc.status, tc.reason)
			}
		})
	}
}

func TestCreateProviderUnreachableLocal(t *testing.T) {
	f := newFixture(t)
	f.adapter.err = &aiconn.CatalogError{Kind: "unreachable", Message: "down"}
	view, err := f.svc.CreateProvider(context.Background(), aiconn.CreateProviderInput{Kind: domain.KindOllama})
	ae, ok := aiconn.AsAPIError(err)
	if !ok || ae.Reason != domain.ReasonUnreachable {
		t.Fatalf("lỗi = %v", err)
	}
	if view.Status != domain.StatusUnreachable || view.BaseURL != "http://127.0.0.1:11434" {
		t.Fatalf("view = %+v", view)
	}
}

func TestModelsFilters(t *testing.T) {
	p := domain.Provider{ID: "p1", Kind: domain.KindOpenAI, Auth: domain.AuthAPIKey,
		Label: "OpenAI", Status: domain.StatusReady, Selected: []string{"alpha"}}
	f := newFixture(t, p)
	ctx := context.Background()
	for _, m := range f.adapter.catalog {
		if err := f.store.UpsertModel(ctx, "p1", m); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		q, filter string
		want      []string
	}{
		{"", "all", []string{"alpha", "beta", "gamma"}},
		{"", "", []string{"alpha", "beta", "gamma"}},
		{"", "selected", []string{"alpha"}},
		{"", "recommended", []string{"beta"}},
		{"be", "all", []string{"beta"}},
		{"AL", "all", []string{"alpha"}},
		{"zz", "all", nil},
	}
	for _, tc := range cases {
		page, err := f.svc.Models(ctx, "p1", tc.q, tc.filter, "", 0)
		got := page.Models
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, 0, len(got))
		for _, m := range got {
			ids = append(ids, m.ID)
		}
		if !slices.Equal(ids, tc.want) {
			t.Fatalf("q=%q filter=%q → %v, muốn %v", tc.q, tc.filter, ids, tc.want)
		}
	}
	if _, err := f.svc.Models(ctx, "p1", "", "bogus", "", 0); err == nil {
		t.Fatal("filter lạ phải lỗi")
	}
}

// ── xoá / in_use ───────────────────────────────────────────────────────────

func TestDeleteProviderInUseReportsAgents(t *testing.T) {
	p := domain.Provider{ID: "p1", Kind: domain.KindOpenAI, Auth: domain.AuthAPIKey,
		Label: "OpenAI", Status: domain.StatusReady, Selected: []string{"alpha"}}
	f := newFixture(t, append(seedSubscriptions(), p)...)
	ctx := context.Background()
	seedDefaultModels(t, f)
	_ = f.store.UpsertModel(ctx, "p1", domain.AIModel{ID: "alpha", Caps: []domain.Capability{domain.CapText}})
	agents := domain.DefaultAgentModels()
	agents.Director.Model = domain.ModelRef{Provider: "p1", Model: "alpha"}
	if _, err := f.svc.PutAgents(ctx, agents); err != nil {
		t.Fatal(err)
	}
	err := f.svc.DeleteProvider(ctx, "p1")
	ae, ok := aiconn.AsAPIError(err)
	if !ok || ae.Status != http.StatusConflict || ae.Reason != domain.ReasonInUse {
		t.Fatalf("lỗi = %v", err)
	}
	got, _ := ae.Fields["agents"].([]string)
	if !slices.Equal(got, []string{"director"}) {
		t.Fatalf("agents = %v", got)
	}
}

func TestDeleteManagedProviderRefused(t *testing.T) {
	f := newFixture(t, seedSubscriptions()...)
	err := f.svc.DeleteProvider(context.Background(), "chatgpt")
	ae, ok := aiconn.AsAPIError(err)
	if !ok || ae.Reason != domain.ReasonManaged {
		t.Fatalf("lỗi = %v", err)
	}
}

func TestPatchRejectsUnselectingModelInUse(t *testing.T) {
	f := newFixture(t, seedSubscriptions()...)
	ctx := context.Background()
	seedDefaultModels(t, f)
	if _, err := f.svc.PutAgents(ctx, domain.DefaultAgentModels()); err != nil {
		t.Fatal(err)
	}
	empty := []string{}
	_, err := f.svc.PatchProvider(ctx, "chatgpt", aiconn.PatchProviderInput{Selected: &empty})
	ae, ok := aiconn.AsAPIError(err)
	if !ok || ae.Reason != domain.ReasonInUse {
		t.Fatalf("lỗi = %v", err)
	}
}

// ── agents ─────────────────────────────────────────────────────────────────

func TestAgentsDefaultsAndResolvers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	got, err := f.svc.Agents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Director.Model.Model != "gpt-6.1-sol" || !got.Artist.FallbackOn {
		t.Fatalf("agents mặc định = %+v", got)
	}
	if len(got.Artist.DrawFallback) != 1 || got.Artist.DrawFallback[0].Provider != "grok" {
		t.Fatalf("drawFallback mặc định = %+v", got.Artist.DrawFallback)
	}
	dir, err := f.svc.ResolveDirector(ctx)
	if err != nil || len(dir) != 1 || dir[0].Model != "gpt-6.1-sol" {
		t.Fatalf("ResolveDirector = %v %v", dir, err)
	}
	artist, err := f.svc.ResolveArtist(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(artist.DrawChain) != 2 || artist.DrawChain[1].Provider != "grok" {
		t.Fatalf("DrawChain = %+v", artist.DrawChain)
	}
}

func TestPutAgentsCapabilityChecks(t *testing.T) {
	p := domain.Provider{ID: "p1", Kind: domain.KindOpenAI, Auth: domain.AuthAPIKey,
		Label: "OpenAI", Status: domain.StatusReady,
		Selected: []string{"text-only", "reader", "painter"}}
	f := newFixture(t, append(seedSubscriptions(), p)...)
	ctx := context.Background()
	seedDefaultModels(t, f)
	_ = f.store.UpsertModel(ctx, "p1", domain.AIModel{ID: "text-only", Caps: []domain.Capability{domain.CapText}})
	_ = f.store.UpsertModel(ctx, "p1", domain.AIModel{ID: "reader", Caps: []domain.Capability{domain.CapText, domain.CapVision}})
	_ = f.store.UpsertModel(ctx, "p1", domain.AIModel{ID: "painter", Caps: []domain.Capability{domain.CapImage}})

	base := domain.DefaultAgentModels()
	base.Director.Model = domain.ModelRef{Provider: "p1", Model: "text-only"}
	base.Artist.PromptModel = domain.ModelRef{Provider: "p1", Model: "reader"}
	base.Artist.Draw = domain.ModelRef{Provider: "p1", Model: "painter"}
	base.Artist.DrawFallback = nil

	if _, err := f.svc.PutAgents(ctx, base); err != nil {
		t.Fatalf("cấu hình hợp lệ bị từ chối: %v", err)
	}

	cases := []struct {
		name  string
		mut   func(*domain.AgentModels)
		field string
		need  string
	}{
		{"draw thiếu image", func(a *domain.AgentModels) {
			a.Artist.Draw = domain.ModelRef{Provider: "p1", Model: "reader"}
		}, "artist.draw", "image"},
		{"promptModel thiếu vision", func(a *domain.AgentModels) {
			a.Artist.PromptModel = domain.ModelRef{Provider: "p1", Model: "text-only"}
		}, "artist.promptModel", "vision"},
		{"drawFallback thiếu image", func(a *domain.AgentModels) {
			a.Artist.DrawFallback = []domain.ModelRef{{Provider: "p1", Model: "reader"}}
		}, "artist.drawFallback", "image"},
		{"director thiếu text", func(a *domain.AgentModels) {
			a.Director.Model = domain.ModelRef{Provider: "p1", Model: "painter"}
		}, "director.model", "text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agents := base
			tc.mut(&agents)
			_, err := f.svc.PutAgents(ctx, agents)
			ae, ok := aiconn.AsAPIError(err)
			if !ok || ae.Reason != domain.ReasonCapabilityMissing {
				t.Fatalf("lỗi = %v", err)
			}
			if ae.Fields["need"] != tc.need || ae.Fields["field"] != tc.field {
				t.Fatalf("fields = %+v", ae.Fields)
			}
		})
	}
}

func TestPutAgentsRejectsUnselectedOrUnknownModel(t *testing.T) {
	p := domain.Provider{ID: "p1", Kind: domain.KindOpenAI, Auth: domain.AuthAPIKey,
		Label: "OpenAI", Status: domain.StatusReady, Selected: []string{"alpha"}}
	f := newFixture(t, append(seedSubscriptions(), p)...)
	ctx := context.Background()
	seedDefaultModels(t, f)
	_ = f.store.UpsertModel(ctx, "p1", domain.AIModel{ID: "alpha", Caps: []domain.Capability{domain.CapText}})
	_ = f.store.UpsertModel(ctx, "p1", domain.AIModel{ID: "beta", Caps: []domain.Capability{domain.CapText}})

	agents := domain.DefaultAgentModels()
	agents.Director.Model = domain.ModelRef{Provider: "p1", Model: "beta"}
	_, err := f.svc.PutAgents(ctx, agents)
	if ae, ok := aiconn.AsAPIError(err); !ok || ae.Reason != domain.ReasonUnknownModel {
		t.Fatalf("model chưa chọn: %v", err)
	}
	agents.Director.Model = domain.ModelRef{Provider: "p1", Model: "nope"}
	_, err = f.svc.PutAgents(ctx, agents)
	if ae, ok := aiconn.AsAPIError(err); !ok || ae.Reason != domain.ReasonUnknownModel {
		t.Fatalf("model lạ: %v", err)
	}
}

func TestPutAgentsDefaultsDrawFallbackToOtherSource(t *testing.T) {
	f := newFixture(t, seedSubscriptions()...)
	ctx := context.Background()
	seedDefaultModels(t, f)

	agents := domain.DefaultAgentModels()
	agents.Artist.Draw = domain.ModelRef{Provider: "grok", Model: "grok-imagine-image"}
	agents.Artist.DrawFallback = nil
	got, err := f.svc.PutAgents(ctx, agents)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Artist.DrawFallback) != 1 || got.Artist.DrawFallback[0].Provider != "chatgpt" {
		t.Fatalf("drawFallback = %+v", got.Artist.DrawFallback)
	}
	if f.changed == 0 {
		t.Fatal("không bump status")
	}
}

// ── check model ────────────────────────────────────────────────────────────

func TestCheckModelAddsManual(t *testing.T) {
	p := domain.Provider{ID: "p1", Kind: domain.KindOpenAI, Auth: domain.AuthAPIKey,
		Label: "OpenAI", Status: domain.StatusReady, Selected: []string{"alpha"}}
	f := newFixture(t, p)
	ctx := context.Background()
	f.adapter.checked = domain.AIModel{Caps: []domain.Capability{domain.CapText, domain.CapVision}, ContextK: 128}
	res, err := f.svc.CheckModel(ctx, "p1", "custom-vl")
	if err != nil || !res.OK || res.ContextK != 128 {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	m, ok, _ := f.store.GetModel(ctx, "p1", "custom-vl")
	if !ok || !m.Manual {
		t.Fatalf("model chưa thành manual: %+v", m)
	}
	// Lấy danh mục lại không xoá model manual.
	_ = f.store.ReplaceCatalog(ctx, "p1", f.adapter.catalog, time.Now())
	if _, ok, _ := f.store.GetModel(ctx, "p1", "custom-vl"); !ok {
		t.Fatal("ReplaceCatalog xoá model manual")
	}
}

// ── login ─────────────────────────────────────────────────────────────────

func TestLoginFlowPhasesAndStatus(t *testing.T) {
	f := newFixture(t, seedSubscriptions()...)
	ctx := context.Background()
	gate := make(chan struct{})
	f.auth.loginScript = func(ctrl aiconn.LoginController, manual <-chan aiconn.ManualCode) (string, error) {
		ctrl.OnDeviceCode("ABCD-1234", "https://auth.example/device")
		<-gate
		return "me@example.com · plus", nil
	}
	sess, err := f.svc.StartLogin(ctx, "chatgpt", "")
	if err != nil {
		t.Fatal(err)
	}
	if sess == nil {
		t.Fatal("không có session")
	}
	// Chờ controller cập nhật phase.
	deadline := time.Now().Add(2 * time.Second)
	var got *aiconn.LoginSession
	for time.Now().Before(deadline) {
		got, _ = f.svc.LoginStatus(ctx, "chatgpt")
		if got.Phase == aiconn.LoginCode {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got == nil || got.Phase != aiconn.LoginCode || got.Code != "ABCD-1234" ||
		got.VerifyURL != "https://auth.example/device" {
		t.Fatalf("phase = %+v", got)
	}
	// Luồng thứ hai bị chặn.
	if _, err := f.svc.StartLogin(ctx, "grok", ""); err == nil {
		t.Fatal("hai luồng đăng nhập song song")
	}
	close(gate)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ = f.svc.LoginStatus(ctx, "chatgpt")
		if got.Phase == aiconn.LoginDone {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got.Phase != aiconn.LoginDone || got.Account == "" {
		t.Fatalf("phase cuối = %+v", got)
	}
}

func TestLoginRejectsNonSubscriptionAndBadMode(t *testing.T) {
	p := domain.Provider{ID: "p1", Kind: domain.KindOpenAI, Auth: domain.AuthAPIKey, Label: "OpenAI"}
	f := newFixture(t, p)
	if _, err := f.svc.StartLogin(context.Background(), "p1", ""); err == nil {
		t.Fatal("provider khoá API nhận login")
	}
	f2 := newFixture(t, seedSubscriptions()...)
	if _, err := f2.svc.StartLogin(context.Background(), "chatgpt", "telepathy"); err == nil {
		t.Fatal("mode lạ được chấp nhận")
	}
}

func TestLogoutClearsCredential(t *testing.T) {
	f := newFixture(t, seedSubscriptions()...)
	ctx := context.Background()
	f.auth.local["openai-codex"] = aiconn.OAuthPresent
	f.auth.creds["openai-codex"] = "access"
	if err := f.svc.Logout(ctx, "chatgpt"); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.auth.creds["openai-codex"]; ok {
		t.Fatal("credential còn sau logout")
	}
	views, err := f.svc.Providers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range views {
		if v.ID == "chatgpt" && v.Status != domain.StatusLoggedOut {
			t.Fatalf("status = %s", v.Status)
		}
	}
}

// ── sẵn sàng / chú ý ──────────────────────────────────────────────────────

func TestChatGPTReady(t *testing.T) {
	f := newFixture(t, seedSubscriptions()...)
	ctx := context.Background()
	f.auth.creds["openai-codex"] = "access"
	if ok, err := f.svc.ChatGPTReady(ctx); err != nil || !ok {
		t.Fatalf("có credential: %v %v", ok, err)
	}
	delete(f.auth.creds, "openai-codex")
	if ok, err := f.svc.ChatGPTReady(ctx); err != nil || ok {
		t.Fatalf("chưa đăng nhập: %v %v", ok, err)
	}
	f.auth.loginErr = errors.New("oauth: invalid_grant (revoked)")
	if ok, err := f.svc.ChatGPTReady(ctx); err != nil || ok {
		t.Fatalf("bị thu hồi: %v %v", ok, err)
	}
	if !f.auth.revoked["openai-codex"] {
		t.Fatal("chưa đánh dấu revoked")
	}
	f.auth.loginErr = errors.New("dial tcp: connection refused")
	if _, err := f.svc.ChatGPTReady(ctx); err == nil {
		t.Fatal("lỗi mạng phải trả ra để caller thử lại")
	}
}

func TestProvidersStatusDerivedFromLocalAuth(t *testing.T) {
	f := newFixture(t, seedSubscriptions()...)
	ctx := context.Background()
	f.auth.local["openai-codex"] = aiconn.OAuthPresent
	f.auth.local["xai-oauth"] = aiconn.OAuthRevoked
	views, err := f.svc.Providers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]domain.AIStatus{}
	for _, v := range views {
		byID[v.ID] = v.Status
	}
	if byID["chatgpt"] != domain.StatusReady || byID["grok"] != domain.StatusAuthFailed {
		t.Fatalf("status = %v", byID)
	}
}

func TestAttentionReportLoginAndQuota(t *testing.T) {
	f := newFixture(t, seedSubscriptions()...)
	ctx := context.Background()
	f.auth.local["openai-codex"] = aiconn.OAuthAbsent
	f.auth.local["xai-oauth"] = aiconn.OAuthPresent
	f.svc = aiconn.New(aiconn.Options{
		Store: f.store, Settings: f.settings, Box: f.box, Auth: f.auth,
		Adapters:        map[domain.ProviderKind]aiconn.Adapter{},
		DailyImageLimit: 2,
		PaidUsage: func(ctx context.Context, provider string, since time.Time) (int64, error) {
			if provider == "grok" {
				return 2, nil
			}
			return 0, nil
		},
	})
	items, err := f.svc.AttentionReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v", items)
	}
	kinds := []string{items[0].Kind, items[1].Kind}
	if !slices.Contains(kinds, domain.AttentionLoginExpired) || !slices.Contains(kinds, domain.AttentionQuotaLow) {
		t.Fatalf("kinds = %v", kinds)
	}
	if items[0].Href != aiconn.ConnectHref || items[1].Href != aiconn.ConnectHref {
		t.Fatalf("href = %q", items[0].Href)
	}
}

func TestChatGPTLoginForToolStatus(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if got, err := f.svc.ChatGPTLogin(ctx); err != nil || got != "not_logged_in" {
		t.Fatalf("vắng credential: %q %v", got, err)
	}
	f.auth.local["openai-codex"] = aiconn.OAuthPresent
	if got, _ := f.svc.ChatGPTLogin(ctx); got != "chatgpt" {
		t.Fatalf("có credential: %q", got)
	}
	f.auth.local["openai-codex"] = aiconn.OAuthRevoked
	if got, _ := f.svc.ChatGPTLogin(ctx); got != "not_logged_in" {
		t.Fatalf("bị thu hồi: %q", got)
	}
}

func TestModelsLimit(t *testing.T) {
	p := domain.Provider{ID: "p1", Kind: domain.KindOpenRouter, Auth: domain.AuthAPIKey,
		Label: "OpenRouter", Status: domain.StatusReady, Selected: []string{}}
	f := newFixture(t, p)
	ctx := context.Background()
	catalog := make([]domain.AIModel, 500)
	for i := range catalog {
		catalog[i] = domain.AIModel{ID: fmt.Sprintf("model-%03d", i), Caps: []domain.Capability{domain.CapText}}
	}
	if err := f.store.ReplaceCatalog(ctx, "p1", catalog, time.Now()); err != nil {
		t.Fatal(err)
	}
	page, err := f.svc.Models(ctx, "p1", "", "all", "", 0)
	got := page.Models
	if err != nil || len(got) != aiconn.DefaultModelLimit {
		t.Fatalf("mặc định: %d %v", len(got), err)
	}
	if got[len(got)-1].ID != fmt.Sprintf("model-%03d", aiconn.DefaultModelLimit-1) {
		t.Fatalf("cắt sai thứ tự: %s", got[len(got)-1].ID)
	}
	if page, _ := f.svc.Models(ctx, "p1", "", "all", "", 50); len(page.Models) != 50 {
		t.Fatalf("limit 50: %d", len(got))
	}
	if page, _ := f.svc.Models(ctx, "p1", "", "all", "", 10000); len(page.Models) != aiconn.MaxModelLimit {
		t.Fatalf("limit trần: %d", len(got))
	}
	if page, _ := f.svc.Models(ctx, "p1", "model-49", "all", "", 0); len(page.Models) != 10 {
		t.Fatalf("q: %d", len(got))
	}
}

// ── trang danh mục: cursor, q, filter, total ───────────────────────────────

func TestModelsPaginationKeepsOrderAndTotal(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.adapter.catalog = nil
	for i := 0; i < 7; i++ {
		caps := []domain.Capability{domain.CapText}
		if i%2 == 0 {
			caps = append(caps, domain.CapVision)
		}
		f.adapter.catalog = append(f.adapter.catalog, domain.AIModel{
			ID: fmt.Sprintf("m-%02d", i), Caps: caps, Recommended: i < 3,
		})
	}
	view, err := f.svc.CreateProvider(ctx, aiconn.CreateProviderInput{Kind: domain.KindOpenAI, APIKey: "sk-page"})
	if err != nil {
		t.Fatal(err)
	}

	var ids []string
	cursor := ""
	for page := 0; page < 10; page++ {
		got, err := f.svc.Models(ctx, view.ID, "", "all", cursor, 3)
		if err != nil {
			t.Fatal(err)
		}
		if got.Total != 7 {
			t.Fatalf("total = %d, muốn 7", got.Total)
		}
		for _, m := range got.Models {
			ids = append(ids, m.ID)
		}
		if !got.HasMore {
			break
		}
		cursor = got.Models[len(got.Models)-1].ID
	}
	if len(ids) != 7 || ids[0] != "m-00" || ids[6] != "m-06" {
		t.Fatalf("id theo trang = %v", ids)
	}
	// Lọc q + filter vẫn giữ total của cả bộ lọc, không của trang.
	got, err := f.svc.Models(ctx, view.ID, "m-0", "recommended", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 3 || len(got.Models) != 2 || !got.HasMore {
		t.Fatalf("trang lọc = %+v", got)
	}
	if _, err := f.svc.Models(ctx, view.ID, "", "bogus", "", 0); err == nil {
		t.Fatal("filter lạ phải bị từ chối")
	}
}

// ── URL máy chủ ────────────────────────────────────────────────────────────

func TestBaseURLPolicy(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	create := func(kind domain.ProviderKind, in aiconn.CreateProviderInput) error {
		in.Kind = kind
		if in.APIKey == "" && kind == domain.KindOpenAICompat {
			in.APIKey = "sk-compat"
		}
		_, err := f.svc.CreateProvider(ctx, in)
		return err
	}
	cases := []struct {
		name string
		kind domain.ProviderKind
		url  string
		ok   bool
	}{
		{"compat http loopback", domain.KindOpenAICompat, "http://127.0.0.1:9999/v1", true},
		{"compat thiếu scheme", domain.KindOpenAICompat, "127.0.0.1:9999", false},
		{"compat userinfo", domain.KindOpenAICompat, "http://user:pass@127.0.0.1:9999", false},
		{"compat query", domain.KindOpenAICompat, "http://127.0.0.1:9999?key=x", false},
		{"openai cố định", domain.KindOpenAI, "http://127.0.0.1:9999", false},
		{"ollama local", domain.KindOllama, "http://192.168.1.8:11434", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := create(tc.kind, aiconn.CreateProviderInput{BaseURL: tc.url})
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v, muốn ok = %v", err, tc.ok)
			}
		})
	}
}

// ── executor: cấu hình không có đường chạy bị từ chối ─────────────────────

// denyExecutors từ chối mọi ModelRef (mô phỏng kind không có client).
type denyExecutors struct{ text, draw bool }

func (d denyExecutors) CanText(context.Context, domain.ModelRef) bool { return d.text }
func (d denyExecutors) CanDraw(context.Context, domain.ModelRef) bool { return d.draw }

func TestPutAgentsRejectsModelWithoutExecutor(t *testing.T) {
	f := newFixture(t, seedSubscriptions()...)
	seedDefaultModels(t, f)
	svc := aiconn.New(aiconn.Options{
		Store: f.store, Settings: f.settings, Box: f.box, Auth: f.auth,
		Now:       func() time.Time { return time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC) },
		Executors: denyExecutors{text: false, draw: true},
	})
	agents, err := svc.Agents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.PutAgents(context.Background(), agents)
	ae, ok := aiconn.AsAPIError(err)
	if !ok || ae.Reason != domain.ReasonNoExecutor {
		t.Fatalf("lỗi = %v", err)
	}
	if ae.Fields["field"] != "director.model" {
		t.Fatalf("fields = %+v", ae.Fields)
	}
}

// ── attention chỉ đọc thẻ đăng nhập gói ───────────────────────────────────

func TestAttentionReadsOnlySubscribedCards(t *testing.T) {
	f := newFixture(t, seedSubscriptions()...)
	// Một provider khoá API trong DB — không được đọc trên đường attention.
	_ = f.store.UpsertProvider(context.Background(), domain.Provider{
		ID: "openai-api", Kind: domain.KindOpenAI, Auth: domain.AuthAPIKey,
		Label: "OpenAI", Status: domain.StatusReady,
	})
	f.auth.local["openai-codex"] = aiconn.OAuthAbsent
	items, err := f.svc.AttentionReport(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 || items[0].Href != "#/settings?section=connect" {
		t.Fatalf("items = %+v", items)
	}
	if got := f.store.listProvidersCalls(); got != 0 {
		t.Fatalf("attention gọi ListProviders %d lần — phải chỉ đọc subscription", got)
	}
}

// ── vòng đời: huỷ serve phải kết thúc luồng đăng nhập đang chờ ─────────────

// Luồng đăng nhập bám vòng đời service chứ không bám request: Close() huỷ và
// join nó, nên serve dừng thì không còn OAuth treo hay ghi credential sau đó.
func TestCloseCancelsInFlightLogin(t *testing.T) {
	f := newFixture(t, seedSubscriptions()...)
	f.auth.started = make(chan struct{})
	f.auth.block = true
	ctx := context.Background()
	if _, err := f.svc.StartLogin(ctx, "chatgpt", "browser"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-f.auth.started:
	case <-time.After(2 * time.Second):
		t.Fatal("luồng đăng nhập không bắt đầu")
	}
	done := make(chan struct{})
	go func() { f.svc.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Close không join được luồng đăng nhập")
	}
	sess, err := f.svc.LoginStatus(context.Background(), "chatgpt")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Phase != aiconn.LoginCancelled {
		t.Fatalf("phase = %q, muốn cancelled", sess.Phase)
	}
}
