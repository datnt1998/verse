package host

import (
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/llmconfig"
)

// cfgForNewBook 构造能通过 ValidateBase 的最小配置（本地假 provider 不发请求）。
func cfgForNewBook(dir string, advanceMode string) bootstrap.Config {
	return bootstrap.Config{
		OutputDir:   filepath.Join(dir, "output", "novel"),
		Provider:    "testprovider",
		ModelName:   "test-model",
		AdvanceMode: advanceMode,
		Providers: map[string]llmconfig.ProviderConfig{
			"testprovider": {Type: "openai", APIKey: "sk-test", BaseURL: "http://127.0.0.1:1"},
		},
	}
}

// advance_mode=review 只在建书时初始化 RunMeta；已有书保留持久化模式（GAP-10 配套键）。
func TestNew_AppliesAdvanceModeOnFreshBookOnly(t *testing.T) {
	dir := t.TempDir()

	h, err := New(cfgForNewBook(dir, "review"), assets.Load("default", assets.LoadOptions{}))
	if err != nil {
		t.Fatalf("host.New: %v", err)
	}
	h.Close()

	st := storepkg.NewStore(filepath.Join(dir, "output", "novel"))
	meta, err := st.RunMeta.Load()
	if err != nil || meta == nil {
		t.Fatalf("RunMeta.Load: %v %v", meta, err)
	}
	if meta.AdvanceMode != domain.ChapterAdvanceReview {
		t.Fatalf("Truyện mới phải khởi tạo theo advance_mode=review, nhận được %q", meta.AdvanceMode)
	}

	// 同一目录重新建 Host：持久化的 review 不能被配置覆盖回 auto。
	h2, err := New(cfgForNewBook(dir, "auto"), assets.Load("default", assets.LoadOptions{}))
	if err != nil {
		t.Fatalf("host.New reopen: %v", err)
	}

	h2.Close()
	meta, err = st.RunMeta.Load()
	if err != nil || meta == nil {
		t.Fatalf("RunMeta.Load: %v %v", meta, err)
	}
	if meta.AdvanceMode != domain.ChapterAdvanceReview {
		t.Fatalf("Truyện đã có phải giữ review, nhận được %q", meta.AdvanceMode)
	}
}

func TestNew_FreshBookDefaultsToAuto(t *testing.T) {
	dir := t.TempDir()
	h, err := New(cfgForNewBook(dir, ""), assets.Load("default", assets.LoadOptions{}))
	if err != nil {
		t.Fatalf("host.New: %v", err)
	}
	defer h.Close()

	st := storepkg.NewStore(filepath.Join(dir, "output", "novel"))
	meta, err := st.RunMeta.Load()
	if err != nil || meta == nil {
		t.Fatalf("RunMeta.Load: %v %v", meta, err)
	}
	if meta.AdvanceMode != domain.ChapterAdvanceAuto {
		t.Fatalf("Mặc định phải là auto, nhận được %q", meta.AdvanceMode)
	}
}
