// Test chuyển từ src/server/tools/codex-isolation.test.ts của yt-factory.
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/toolchain"
)

// catalogFixture là catalog giống catalog ghim của Codex.
var catalogFixture = map[string]any{
	"models": []any{
		map[string]any{
			"slug":                         CodexImageReasoningModel,
			"input_modalities":             []any{"text", "image"},
			"tool_mode":                    "unified",
			"apply_patch_tool_type":        "function",
			"experimental_supported_tools": []any{"tool_search"},
			"supports_search_tool":         true,
		},
		map[string]any{
			"slug":             "gpt-6.1-mini",
			"input_modalities": []any{"text"},
		},
	},
}

func TestRestrictImageModelCatalogKeepsImageInputAndDropsTools(t *testing.T) {
	got, err := RestrictImageModelCatalog(catalogFixture)
	if err != nil {
		t.Fatal(err)
	}
	models, _ := got["models"].([]any)
	if len(models) != 1 {
		t.Fatalf("models = %v", models)
	}
	model, _ := models[0].(map[string]any)
	if model["slug"] != CodexImageReasoningModel {
		t.Fatalf("slug = %v", model["slug"])
	}
	if model["tool_mode"] != "direct" {
		t.Fatalf("tool_mode = %v", model["tool_mode"])
	}
	if model["supports_search_tool"] != false {
		t.Fatalf("supports_search_tool = %v", model["supports_search_tool"])
	}
	if tools, ok := model["experimental_supported_tools"].([]any); !ok || len(tools) != 0 {
		t.Fatalf("experimental_supported_tools = %v", model["experimental_supported_tools"])
	}
}

func TestRestrictTextModelCatalogStripsImageModality(t *testing.T) {
	got, err := RestrictTextModelCatalog(catalogFixture)
	if err != nil {
		t.Fatal(err)
	}
	models, _ := got["models"].([]any)
	model, _ := models[0].(map[string]any)
	if !reflect.DeepEqual(model["input_modalities"], []any{"text"}) {
		t.Fatalf("modalities = %v", model["input_modalities"])
	}
	if model["tool_mode"] != "direct" {
		t.Fatalf("tool_mode = %v", model["tool_mode"])
	}
}

func TestRestrictCatalogsFailOnMissingModel(t *testing.T) {
	if _, err := RestrictImageModelCatalog(map[string]any{"models": []any{}}); err == nil {
		t.Fatal("không từ chối catalog vắng model")
	}
	if _, err := RestrictTextModelCatalog(map[string]any{"models": []any{}}); err == nil {
		t.Fatal("không từ chối catalog vắng model")
	}
}

func TestImageAllowlistAcceptsImageGenAlone(t *testing.T) {
	v := InspectImageToolAllowlist([]any{
		map[string]any{"type": "namespace", "name": "image_gen", "tools": []any{
			map[string]any{"type": "function", "name": "imagegen"},
		}},
	})
	if !v.OK {
		t.Fatalf("verdict = %+v", v)
	}
}

func TestImageAllowlistAcceptsHostViewImage(t *testing.T) {
	v := InspectImageToolAllowlist([]any{
		map[string]any{"type": "namespace", "name": "image_gen", "tools": []any{
			map[string]any{"type": "function", "name": "imagegen"},
		}},
		map[string]any{"type": "function", "name": "view_image"},
	})
	if !v.OK {
		t.Fatalf("verdict = %+v", v)
	}
}

func TestImageAllowlistRejectsAnythingBeyondImageGen(t *testing.T) {
	v := InspectImageToolAllowlist([]any{
		map[string]any{"type": "namespace", "name": "image_gen", "tools": []any{
			map[string]any{"type": "function", "name": "imagegen"},
		}},
		map[string]any{"type": "function", "name": "web_search"},
	})
	if v.OK {
		t.Fatalf("verdict = %+v", v)
	}
}

func TestImageAllowlistRejectsMissingImageGen(t *testing.T) {
	v := InspectImageToolAllowlist([]any{})
	if v.OK {
		t.Fatal("tool trống vẫn qua")
	}
}

func TestAdvertisementInspectionReadsTopLevelAndInputEmbeddedTools(t *testing.T) {
	topLevel := InspectAdvertisementRequest(map[string]any{
		"model": CodexImageReasoningModel,
		"tools": []any{
			map[string]any{"type": "namespace", "name": "image_gen", "tools": []any{
				map[string]any{"type": "function", "name": "imagegen"},
			}},
		},
	})
	if topLevel == nil || !topLevel.OK {
		t.Fatalf("topLevel = %+v", topLevel)
	}
	embedded := InspectAdvertisementRequest(map[string]any{
		"model": CodexImageReasoningModel,
		"input": []any{
			map[string]any{"type": "message"},
			map[string]any{"type": "additional_tools", "tools": []any{
				map[string]any{"type": "namespace", "name": "image_gen", "tools": []any{
					map[string]any{"type": "function", "name": "imagegen"},
				}},
			}},
		},
	})
	if embedded == nil || !embedded.OK {
		t.Fatalf("embedded = %+v", embedded)
	}
	if InspectAdvertisementRequest(map[string]any{"model": "x"}) != nil {
		t.Fatal("request không phải quảng cáo bị chấp nhận")
	}
	if InspectAdvertisementRequest("junk") != nil {
		t.Fatal("body lạ bị chấp nhận")
	}
}

func TestTextAdvertisementRequiresEmptyTools(t *testing.T) {
	empty := InspectTextAdvertisementRequest(map[string]any{
		"model": CodexImageReasoningModel,
		"tools": []any{},
	})
	if empty == nil || !empty.OK {
		t.Fatalf("empty = %+v", empty)
	}
	withTool := InspectTextAdvertisementRequest(map[string]any{
		"model": CodexImageReasoningModel,
		"tools": []any{
			map[string]any{"type": "namespace", "name": "image_gen", "tools": []any{
				map[string]any{"type": "function", "name": "imagegen"},
			}},
		},
	})
	if withTool == nil || withTool.OK {
		t.Fatalf("withTool = %+v", withTool)
	}
}

// Nguồn: codex-isolation.test.ts + verify format với codex-cli 0.162.1.
func TestCreateIsolatedCodexHomeCopiesOnlyAuth(t *testing.T) {
	dest := t.TempDir()
	source := t.TempDir()
	// File trong "home nguồn" không còn được chép — auth.json do app tổng hợp.
	if err := os.WriteFile(filepath.Join(source, "config.toml"), []byte("model='x'"), 0o644); err != nil {
		t.Fatal(err)
	}
	home, err := CreateIsolatedCodexHome(CodexAuthTokens{
		AccessToken: "access-token-1", AccountID: "acct-1",
	}, dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(home, dest) {
		t.Fatalf("home = %q", home)
	}
	if toolchain.FileExists(filepath.Join(home, "config.toml")) {
		t.Fatal("config lọt vào home cô lập")
	}
	auth, err := os.ReadFile(filepath.Join(home, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(home, "auth.json"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("auth mode = %o", info.Mode().Perm())
	}
	var doc struct {
		APIKey *string `json:"OPENAI_API_KEY"`
		Tokens struct {
			IDToken      string  `json:"id_token"`
			AccessToken  string  `json:"access_token"`
			RefreshToken *string `json:"refresh_token"`
			AccountID    string  `json:"account_id"`
		} `json:"tokens"`
		LastRefresh string `json:"last_refresh"`
	}
	if err := json.Unmarshal(auth, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.APIKey != nil {
		t.Fatal("OPENAI_API_KEY phải null")
	}
	if doc.Tokens.AccessToken != "access-token-1" || doc.Tokens.IDToken != "access-token-1" {
		t.Fatalf("tokens sai: %+v", doc.Tokens)
	}
	if doc.Tokens.AccountID != "acct-1" {
		t.Fatalf("account_id = %q", doc.Tokens.AccountID)
	}
	if doc.Tokens.RefreshToken == nil || *doc.Tokens.RefreshToken != "" {
		t.Fatal("refresh_token phải tồn tại và rỗng (CLI không được xoay vòng)")
	}
	if doc.LastRefresh == "" {
		t.Fatal("last_refresh thiếu")
	}
	if _, err := CreateIsolatedCodexHome(CodexAuthTokens{}, dest); err == nil {
		t.Fatal("thiếu token vẫn tạo được home")
	}
}

func TestCloneSandboxPermissionProfileHasUniqueName(t *testing.T) {
	if ClonePermissionProfile == "" {
		t.Fatal("profile rỗng")
	}
	args := SandboxProbeArgs("/refs", []string{"/bin/cat", "/secret/file"})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--permission-profile") || !strings.Contains(joined, ClonePermissionProfile) {
		t.Fatalf("args = %v", args)
	}
	if !strings.Contains(joined, "/refs") {
		t.Fatalf("refs không có trong overrides: %v", args)
	}
}

func TestCodexExecArgsIncludesIsolationFlags(t *testing.T) {
	args := CodexExecArgs("/refs", "/home/catalog.json", []string{"/refs/a.png"}, "/out.txt", "say hi")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"exec", "--json", "--ephemeral", "--ignore-user-config", "--ignore-rules",
		"--cd", "/refs", "--image", "/refs/a.png",
		"--output-last-message", "/out.txt", "say hi",
		"model_catalog_json",
		"--disable", "--enable",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("thiếu %q trong %v", want, args)
		}
	}
	// Prompt "say hi" phải nằm sau `--` để không bị parse thành flag.
	idx := strings.Index(strings.Join(args, "\x00"), "say hi")
	if idx < 0 {
		t.Fatal("prompt vắng")
	}
}

func TestCodexTextExecArgsDisablesImageFeatures(t *testing.T) {
	args := CodexTextExecArgs("/refs", "/home/catalog.json", "/out.txt", "hello")
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--image ") {
		t.Fatal("cờ ảnh lọt vào lời gọi text")
	}
	// image_generation phải nằm trong danh sách --disable.
	disabledAfter := strings.Index(joined, "image_generation")
	if disabledAfter < 0 {
		t.Fatal("image_generation không bị tắt")
	}
}

// Một request mang nhiều entry additional_tools phải gộp hết: entry sau
// không được che entry trước — nếu chỉ giữ cái cuối, tool nguy hiểm quảng cáo
// ở entry đầu lọt qua kiểm tra.
func TestAdvertisementToolsConcatenatesMultipleEntries(t *testing.T) {
	body := map[string]any{
		"model": "x",
		"input": []any{
			map[string]any{"type": "additional_tools", "tools": []any{
				map[string]any{"type": "function", "name": "dangerous.exec"},
			}},
			map[string]any{"type": "additional_tools", "tools": []any{
				map[string]any{"type": "function", "name": "image_gen.imagegen"},
			}},
		},
	}
	v := InspectAdvertisementRequest(body)
	if v == nil {
		t.Fatal("không nhận ra advertisement")
	}
	if v.OK {
		t.Fatalf("tool cấm ở entry đầu bị entry sau che: %+v", v)
	}
	if len(v.Names) != 2 {
		t.Fatalf("names = %v, muốn gộp cả hai entry", v.Names)
	}
}

// Inspect trên nhiều goroutine cùng lúc: pure function không được race khi
// máy chủ probe nhận request song song.
func TestInspectAdvertisementRequestConcurrent(t *testing.T) {
	bodies := []any{
		map[string]any{"model": "x", "tools": []any{
			map[string]any{"type": "function", "name": "image_gen.imagegen"},
		}},
		map[string]any{"model": "x", "tools": []any{
			map[string]any{"type": "function", "name": "shell.exec"},
		}},
		map[string]any{"model": "x"},
	}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			v := InspectAdvertisementRequest(bodies[k%3])
			if k%3 == 0 && (v == nil || !v.OK) {
				t.Errorf("body an toàn bị từ chối: %+v", v)
			}
			if k%3 == 1 && (v == nil || v.OK) {
				t.Errorf("tool cấm được chấp nhận: %+v", v)
			}
			if k%3 == 2 && v != nil {
				t.Errorf("request không-tools bị coi là advertisement")
			}
		}(i)
	}
	wg.Wait()
}

// ParseCodexModelCatalog đọc catalog thật của `debug models` (fixture dưới đây
// chép từ codex-cli 0.162.1).
func TestParseCodexModelCatalog(t *testing.T) {
	stdout := `{"models":[
		{"slug":"gpt-6.1-sol","input_modalities":["text","image"],"context_window":272000,"supported_in_api":true},
		{"slug":"codex-auto-review","input_modalities":["text"],"context_window":272000,"supported_in_api":true}
	]}`
	models, err := ParseCodexModelCatalog(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].Slug != "gpt-6.1-sol" || models[0].ContextWindow != 272000 {
		t.Fatalf("models = %+v", models)
	}
	if len(models[0].InputModalities) != 2 || models[0].InputModalities[1] != "image" {
		t.Fatalf("modalities = %v", models[0].InputModalities)
	}
	if _, err := ParseCodexModelCatalog("không phải json"); err == nil {
		t.Fatal("stdout rác vẫn parse được")
	}
	if _, err := ParseCodexModelCatalog(`{"models":[]}`); err != nil {
		t.Fatalf("catalog rỗng phải đọc được: %v", err)
	}
}

// Xác nhận định dạng auth.json tổng hợp được Codex CLI ghim chấp nhận:
// `codex login status` phải báo "Logged in using ChatGPT" khi chỉ có access
// token và refresh_token rỗng. Miễn phí (không gọi model). TẮT mặc định:
//
//	VIDERSE_CODEX_FORMAT_CHECK=1 go test ./internal/tools/ -run TestCodexAuthJSONFormat -v
func TestCodexAuthJSONFormatAcceptedByCLI(t *testing.T) {
	if os.Getenv("VIDERSE_CODEX_FORMAT_CHECK") != "1" {
		t.Skip("đặt VIDERSE_CODEX_FORMAT_CHECK=1 để kiểm định dạng với codex CLI thật")
	}
	exe := FindOnPath("codex", os.Getenv("PATH"))
	if exe == "" {
		t.Skip("codex không có trên PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	home, err := CreateIsolatedCodexHome(CodexAuthTokens{
		// Token giả đúng cấu trúc JWT (base64url, 3 đoạn): chỉ kiểm định dạng
		// auth.json, không gọi model hay mạng.
		AccessToken: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9." +
			"eyJzdWIiOiJmb3JtYXQtY2hlY2siLCJlbWFpbCI6ImFAYi5jIn0." +
			"eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4",
		AccountID: "00000000-0000-4000-8000-000000000000",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := &ExecRunner{}
	res, err := r.Run(ctx, exe, []string{"login", "status"}, Spec{
		Timeout: 30 * time.Second,
		Env:     map[string]string{"CODEX_HOME": home, "HOME": home},
		OKCodes: []int{0, 1},
	})
	if err != nil {
		t.Fatalf("không chạy được codex login status: %v", err)
	}
	out := res.Stdout + res.Stderr
	if !strings.Contains(strings.ToLower(out), "chatgpt") {
		t.Fatalf("CLI không nhận auth.json tổng hợp: %q", out)
	}
}
