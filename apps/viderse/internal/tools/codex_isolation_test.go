// Test chuyển từ src/server/tools/codex-isolation.test.ts của yt-factory.
package tools

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
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

func TestCreateIsolatedCodexHomeCopiesOnlyAuth(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "auth.json"), []byte(`{"OPENAI_API_KEY":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "config.toml"), []byte("model='x'"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	home, err := CreateIsolatedCodexHome(source, dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(home, dest) {
		t.Fatalf("home = %q", home)
	}
	if fileExists(filepath.Join(home, "config.toml")) {
		t.Fatal("config lọt vào home cô lập")
	}
	if fileExists(filepath.Join(home, "skills")) {
		t.Fatal("skills lọt vào home cô lập")
	}
	auth, err := os.ReadFile(filepath.Join(home, "auth.json"))
	if err != nil || string(auth) != `{"OPENAI_API_KEY":null}` {
		t.Fatalf("auth = %q %v", auth, err)
	}
	info, _ := os.Stat(filepath.Join(home, "auth.json"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("auth mode = %o", info.Mode().Perm())
	}
	if _, err := CreateIsolatedCodexHome(filepath.Join(dest, "nowhere"), dest); err == nil {
		t.Fatal("thiếu auth vẫn tạo được home")
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
