// Ca test store của Kết nối AI trên Postgres thật (pgtest): vòng đời provider
// và danh mục model, giữ model manual qua ReplaceCatalog, cascade khi xoá, và
// bất biến "khoá API không bao giờ ở plaintext trong DB".
//
// Chạy khi có TEST_DATABASE_URL như các store test khác của Viderse.
package postgres_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/aiconn"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
)

func TestAIConnStoreSeedsManagedProviders(t *testing.T) {
	store := postgres.NewAIConnStore(provision(t))
	ctx := context.Background()
	rows, err := store.ListProviders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]domain.Provider{}
	for _, row := range rows {
		byID[row.Provider.ID] = row.Provider
	}
	chatgpt, ok := byID["chatgpt"]
	if !ok || chatgpt.Auth != domain.AuthSubscription || len(chatgpt.Selected) != 2 {
		t.Fatalf("chatgpt = %+v", chatgpt)
	}
	models, err := store.ListModels(ctx, "chatgpt")
	if err != nil || len(models) == 0 {
		t.Fatalf("model mặc định thiếu: %v %v", models, err)
	}
}

func TestAIConnStoreProviderRoundTrip(t *testing.T) {
	store := postgres.NewAIConnStore(provision(t))
	ctx := context.Background()
	p := domain.Provider{
		ID: "openai-abc123", Kind: domain.KindOpenAI, Auth: domain.AuthAPIKey,
		Label: "OpenAI", EncKey: []byte{0x01, 0x02, 0xff}, KeyHint: "••••7c2e",
		BaseURL: "https://api.openai.com/v1", Status: domain.StatusReady,
		Selected: []string{"gpt-a", "gpt-b"},
	}
	if err := store.UpsertProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetProvider(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.KeyHint != p.KeyHint || !bytes.Equal(got.EncKey, p.EncKey) ||
		len(got.Selected) != 2 || got.Selected[1] != "gpt-b" {
		t.Fatalf("roundtrip = %+v", got)
	}
	// Ghi từng trường: lựa chọn model và khoá/trạng thái không ghi đè nhau
	// (hai tab sửa hai trường khác nhau).
	if err := store.SetSelected(ctx, p.ID, []string{"gpt-a"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateProviderKey(ctx, p.ID, p.EncKey, p.KeyHint, p.BaseURL, domain.StatusAuthFailed); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetProvider(ctx, p.ID)
	if got.Status != domain.StatusAuthFailed || len(got.Selected) != 1 {
		t.Fatalf("update = %+v", got)
	}
	if err := store.SetStatus(ctx, p.ID, domain.StatusUnreachable); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.GetProvider(ctx, p.ID); got.Status != domain.StatusUnreachable {
		t.Fatalf("status = %s", got.Status)
	}
	if err := store.DeleteProvider(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.GetProvider(ctx, p.ID); got.ID != "" {
		t.Fatal("xoá provider không hiệu lực")
	}
}

func TestAIConnStoreReplaceCatalogKeepsManual(t *testing.T) {
	store := postgres.NewAIConnStore(provision(t))
	ctx := context.Background()
	p := domain.Provider{ID: "compat-1", Kind: domain.KindOpenAICompat, Auth: domain.AuthAPIKey,
		Label: "Tương thích OpenAI", BaseURL: "http://127.0.0.1:9999", Selected: []string{}}
	if err := store.UpsertProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertModel(ctx, p.ID, domain.AIModel{ID: "manual-one", Manual: true,
		Caps: []domain.Capability{domain.CapText}}); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	if err := store.ReplaceCatalog(ctx, p.ID, []domain.AIModel{
		{ID: "auto-one", Caps: []domain.Capability{domain.CapText}, ContextK: 128, InPerM: 0.5},
	}, at); err != nil {
		t.Fatal(err)
	}
	models, err := store.ListModels(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]domain.AIModel{}
	for _, m := range models {
		ids[m.ID] = m
	}
	if _, ok := ids["manual-one"]; !ok {
		t.Fatal("ReplaceCatalog xoá model manual")
	}
	auto, ok := ids["auto-one"]
	if !ok || auto.ContextK != 128 || auto.InPerM != 0.5 {
		t.Fatalf("auto = %+v", auto)
	}
	got, _ := store.GetProvider(ctx, p.ID)
	if got.CatalogAt != at.UnixMilli() {
		t.Fatalf("catalogAt = %d", got.CatalogAt)
	}
	// Lần thay thứ hai xoá auto cũ.
	if err := store.ReplaceCatalog(ctx, p.ID, []domain.AIModel{{ID: "auto-two"}}, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	models, _ = store.ListModels(ctx, p.ID)
	if len(models) != 2 {
		t.Fatalf("models = %+v", models)
	}
	if err := store.DeleteProvider(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
}

// Bất biến bảo mật: sau khi tạo provider qua service thật (secretbox thật),
// cột key_enc trong DB không chứa plaintext khoá ở bất kỳ biểu diễn nào.
func TestAIConnKeyNeverPlaintextInDB(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	store := postgres.NewAIConnStore(pool)
	settings := postgres.NewSettingsStore(pool)
	dir := t.TempDir()
	box := secretbox.New(dir)

	adapter := &staticAdapter{}
	svc := aiconn.New(aiconn.Options{
		Store: store, Settings: settings, Box: box,
		Adapters: map[domain.ProviderKind]aiconn.Adapter{domain.KindOpenAI: adapter},
	})
	secret := "sk-live-VERYSECRET7c2e"
	if _, err := svc.CreateProvider(ctx, aiconn.CreateProviderInput{
		Kind: domain.KindOpenAI, APIKey: secret,
	}); err != nil {
		t.Fatal(err)
	}

	var enc []byte
	var hint string
	err := pool.QueryRow(ctx,
		`SELECT key_enc, key_hint FROM ai_providers WHERE kind = 'openai' LIMIT 1`).Scan(&enc, &hint)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) == 0 {
		t.Fatal("key_enc rỗng")
	}
	if bytes.Contains(enc, []byte(secret)) || strings.Contains(string(enc), "VERYSECRET") {
		t.Fatal("DB giữ plaintext khoá API")
	}
	if hint != "••••7c2e" {
		t.Fatalf("hint = %q", hint)
	}
	// Mọi dòng/cột text trong hai bảng không được chứa khoá.
	for _, q := range []string{
		`SELECT coalesce(string_agg(id || label || key_hint || base_url || selected::text, ' '), '') FROM ai_providers`,
		`SELECT coalesce(string_agg(id || caps::text, ' '), '') FROM ai_models`,
	} {
		var blob string
		if err := pool.QueryRow(ctx, q).Scan(&blob); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(blob, secret) || strings.Contains(blob, "VERYSECRET") {
			t.Fatalf("bảng chứa plaintext khoá: %q", blob)
		}
	}
	// Khoá máy đúng thì mở lại được (không mất dữ liệu khi kho còn nguyên).
	sealed, err := os.ReadFile(filepath.Join(dir, "secret.key"))
	if err != nil || len(sealed) != 32 {
		t.Fatalf("secret.key: %d %v", len(sealed), err)
	}
}

// staticAdapter trả một model cố định — chỉ cần để tạo provider.
type staticAdapter struct{}

func (a *staticAdapter) FetchCatalog(ctx context.Context, p domain.Provider, key string) ([]domain.AIModel, error) {
	return []domain.AIModel{{ID: "static-model", Caps: []domain.Capability{domain.CapText}}}, nil
}

func (a *staticAdapter) CheckModel(ctx context.Context, p domain.Provider, key, model string) (domain.AIModel, error) {
	return domain.AIModel{ID: model, Caps: []domain.Capability{domain.CapText}}, nil
}

// Hai tab sửa hai trường khác nhau của cùng thẻ không được ghi đè lẫn nhau:
// PATCH chọn model và PATCH đổi khoá chạy song song thì cả hai đều còn.
func TestAIConnFieldWritesDoNotClobber(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	store := postgres.NewAIConnStore(pool)
	settings := postgres.NewSettingsStore(pool)
	box := secretbox.New(t.TempDir())

	adapter := &staticAdapter{}
	svc := aiconn.New(aiconn.Options{
		Store: store, Settings: settings, Box: box,
		Adapters: map[domain.ProviderKind]aiconn.Adapter{
			domain.KindOpenAI: adapter, domain.KindOpenAICompat: adapter,
		},
	})
	view, err := svc.CreateProvider(ctx, aiconn.CreateProviderInput{
		Kind: domain.KindOpenAI, APIKey: "sk-old-1111",
	})
	if err != nil {
		t.Fatal(err)
	}
	selected := []string{"static-model"}
	newKey := "sk-new-2222"
	var wg sync.WaitGroup
	var selErr, keyErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, selErr = svc.PatchProvider(ctx, view.ID, aiconn.PatchProviderInput{Selected: &selected})
	}()
	go func() {
		defer wg.Done()
		_, keyErr = svc.PatchProvider(ctx, view.ID, aiconn.PatchProviderInput{APIKey: &newKey})
	}()
	wg.Wait()
	if selErr != nil || keyErr != nil {
		t.Fatalf("patch = %v %v", selErr, keyErr)
	}
	got, err := store.GetProvider(ctx, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Selected) != 1 || got.Selected[0] != "static-model" {
		t.Fatalf("lựa chọn model bị ghi đè: %+v", got.Selected)
	}
	if got.KeyHint != "••••2222" {
		t.Fatalf("khoá mới không được ghi: %q", got.KeyHint)
	}
}

// Model đã thêm tay giữ cờ manual qua lần lấy danh mục tự động sau —
// nếu mất cờ, lần ReplaceCatalog sau sẽ xoá nó và model biến mất khỏi UI.
func TestAIConnManualFlagSurvivesCatalogRefresh(t *testing.T) {
	store := postgres.NewAIConnStore(provision(t))
	ctx := context.Background()
	p := domain.Provider{ID: "compat-manual", Kind: domain.KindOpenAICompat, Auth: domain.AuthAPIKey,
		Label: "Tương thích OpenAI", BaseURL: "http://127.0.0.1:9998", Selected: []string{}}
	if err := store.UpsertProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	// models/check thêm model tay.
	if err := store.UpsertModel(ctx, p.ID, domain.AIModel{ID: "manual-model", Manual: true,
		Caps: []domain.Capability{domain.CapVision}}); err != nil {
		t.Fatal(err)
	}
	// Danh mục tự động sau đó CŨNG chứa model này (provider bắt đầu trả nó).
	if err := store.ReplaceCatalog(ctx, p.ID, []domain.AIModel{
		{ID: "manual-model", Caps: []domain.Capability{domain.CapVision}, Manual: false},
		{ID: "auto-model", Caps: []domain.Capability{domain.CapText}},
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.GetModel(ctx, p.ID, "manual-model")
	if err != nil || !ok {
		t.Fatalf("model tay biến mất: %v %v", ok, err)
	}
	if !got.Manual {
		t.Fatal("cờ manual bị hạ sau lần refresh danh mục")
	}
}

// Danh sách theo thứ tự thêm: thẻ cũ nhất đứng đầu (web đưa thẻ lỗi lên trước,
// còn lại giữ thứ tự này).
func TestAIConnListProvidersOldestFirst(t *testing.T) {
	store := postgres.NewAIConnStore(provision(t))
	ctx := context.Background()
	for _, id := range []string{"openai-first", "deepseek-second", "openrouter-third"} {
		kind := domain.KindOpenAI
		if strings.HasPrefix(id, "deepseek") {
			kind = domain.KindDeepSeek
		} else if strings.HasPrefix(id, "openrouter") {
			kind = domain.KindOpenRouter
		}
		p := domain.Provider{ID: id, Kind: kind, Auth: domain.AuthAPIKey, Label: id,
			EncKey: []byte{1}, KeyHint: "••••0001", BaseURL: "https://example.test/v1", Status: domain.StatusReady}
		if err := store.UpsertProvider(ctx, p); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	rows, err := store.ListProviders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, row := range rows {
		if strings.HasSuffix(row.Provider.ID, "-first") || strings.HasSuffix(row.Provider.ID, "-second") || strings.HasSuffix(row.Provider.ID, "-third") {
			got = append(got, row.Provider.ID)
		}
	}
	want := []string{"openai-first", "deepseek-second", "openrouter-third"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("thứ tự = %v, muốn %v", got, want)
	}
}
