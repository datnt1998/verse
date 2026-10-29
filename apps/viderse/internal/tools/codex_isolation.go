// Preflight cô lập của Codex + môi trường mỗi lần gọi mà adapter ảnh dùng.
//
// Chứng minh trên binary thật trước khi lời gọi ảnh nào được phép:
//  1. `exec` nhận đủ cờ ta dựa vào (json, ephemeral, ignore-user-config, cd,
//     skip-git-repo-check).
//  2. `codex sandbox` ép đúng permission profile của clone: đọc ngoài tập cho
//     phép thất bại, ghi thất bại, mạng tắt, và secret canary không đọc được.
//  3. `debug prompt-input` chứng minh capability bị từ chối: không chỉ dẫn
//     MCP/app/plugin/browser/computer/tool-search nào tới model, và skill của
//     host không được phát hiện ngoài CODEX_HOME.
//
// Khi điều gì không chứng minh được, kết quả là isolation_unverified — hàng
// ảnh giữ khoá; prompt hay argv không bao giờ được coi là biên an ninh.
package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IsolationState là trạng thái chứng minh cô lập.
type IsolationState string

// CodexImageReasoningModel là model reasoning của các lời gọi Codex.
const CodexImageReasoningModel = "gpt-6.1-sol"

const (
	IsolationVerified   IsolationState = "verified"
	IsolationUnverified IsolationState = "isolation_unverified"
)

// CodexIsolationCheck là một bằng chứng của preflight.
type CodexIsolationCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// CodexIsolation là kết quả preflight tổng hợp.
type CodexIsolation struct {
	State  IsolationState        `json:"state"`
	Checks []CodexIsolationCheck `json:"checks"`
}

// catalogModel lấy entry của model reasoning trong catalog; lỗi mô tả đúng
// những gì catalog thật sự chứa.
func catalogModel(value any, kind string) (map[string]any, error) {
	catalog, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("catalog model %s không tương thích: không có danh sách model", kind)
	}
	models, ok := catalog["models"].([]any)
	if !ok {
		return nil, fmt.Errorf("catalog model %s không tương thích: không có danh sách model", kind)
	}
	for _, m := range models {
		entry, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if entry["slug"] == CodexImageReasoningModel {
			if _, ok := entry["input_modalities"].([]any); !ok {
				return nil, fmt.Errorf("catalog model %s không tương thích: %s không liệt kê input modalities", kind, CodexImageReasoningModel)
			}
			return entry, nil
		}
	}
	return nil, fmt.Errorf("catalog model %s không tương thích: %s vắng mặt giữa %d model", kind, CodexImageReasoningModel, len(models))
}

// RestrictImageModelCatalog chỉ hạ metadata capability, không đụng trọng số
// model hay backend ảnh.
func RestrictImageModelCatalog(value any) (map[string]any, error) {
	model, err := catalogModel(value, "image")
	if err != nil {
		return nil, err
	}
	modalities, _ := model["input_modalities"].([]any)
	hasImage := false
	for _, m := range modalities {
		if m == "image" {
			hasImage = true
		}
	}
	if !hasImage {
		return nil, fmt.Errorf("catalog model image không tương thích: %s không nhận đầu vào ảnh", CodexImageReasoningModel)
	}
	restricted := map[string]any{}
	for k, v := range model {
		restricted[k] = v
	}
	restricted["tool_mode"] = "direct"
	restricted["apply_patch_tool_type"] = nil
	restricted["experimental_supported_tools"] = []any{}
	restricted["supports_search_tool"] = false
	return map[string]any{"models": []any{restricted}}, nil
}

// RestrictTextModelCatalog hạ catalog về mặt text-only cho các lời gọi cấm
// thấy ảnh và tool: cùng model, chỉ modality text, không tool.
func RestrictTextModelCatalog(value any) (map[string]any, error) {
	model, err := catalogModel(value, "text")
	if err != nil {
		return nil, err
	}
	restricted := map[string]any{}
	for k, v := range model {
		restricted[k] = v
	}
	restricted["input_modalities"] = []any{"text"}
	restricted["tool_mode"] = "direct"
	restricted["apply_patch_tool_type"] = nil
	restricted["experimental_supported_tools"] = []any{}
	restricted["supports_search_tool"] = false
	return map[string]any{"models": []any{restricted}}, nil
}

// prepareModelCatalog chạy `debug models --bundled`, hạ catalog và ghi 0600
// vào home. Trả đường dẫn tệp.
func (c *Codex) prepareModelCatalog(ctx context.Context, exe, home, kind, filename string) (string, error) {
	res, err := c.d.Runner.Run(ctx, exe, []string{"debug", "models", "--bundled"}, Spec{
		EnvExact: map[string]string{
			"PATH":       filepath.Dir(exe) + ":" + "/usr/bin:/bin",
			"HOME":       home,
			"CODEX_HOME": home,
		},
		Timeout:  30 * time.Second,
		LogLimit: 8 * 1024 * 1024,
	})
	if err != nil {
		return "", err
	}
	var catalog any
	if err := json.Unmarshal([]byte(res.Stdout), &catalog); err != nil {
		return "", fmt.Errorf("catalog model %s không đọc được: %w", kind, err)
	}
	var restricted map[string]any
	if kind == "image" {
		restricted, err = RestrictImageModelCatalog(catalog)
	} else {
		restricted, err = RestrictTextModelCatalog(catalog)
	}
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(restricted)
	if err != nil {
		return "", err
	}
	file := filepath.Join(home, filename)
	if err := os.WriteFile(file, data, 0o600); err != nil {
		return "", err
	}
	return file, nil
}

// PrepareImageModelCatalog dựng catalog ảnh đã hạ cho một CODEX_HOME cô lập.
func (c *Codex) PrepareImageModelCatalog(ctx context.Context, exe, home string) (string, error) {
	return c.prepareModelCatalog(ctx, exe, home, "image", "image-model-catalog.json")
}

// PrepareTextModelCatalog dựng catalog text đã hạ cho một CODEX_HOME cô lập.
func (c *Codex) PrepareTextModelCatalog(ctx context.Context, exe, home string) (string, error) {
	return c.prepareModelCatalog(ctx, exe, home, "text", "text-model-catalog.json")
}

// AllowlistVerdict là kết quả kiểm một danh sách tool quảng cáo.
type AllowlistVerdict struct {
	OK    bool
	Names []string
}

// advertisedToolNames đọc tên tool từ danh sách entries: namespace.<con> cho
// namespace, tên trần cho function. nil khi hình dạng lạ.
func advertisedToolNames(tools any) []string {
	list, ok := tools.([]any)
	if !ok {
		return nil
	}
	names := []string{}
	for _, entry := range list {
		tool, ok := entry.(map[string]any)
		if !ok {
			return nil
		}
		if tool["type"] == "namespace" && tool["name"] != nil {
			children, ok := tool["tools"].([]any)
			if !ok {
				return nil
			}
			for _, item := range children {
				child, ok := item.(map[string]any)
				if !ok {
					return nil
				}
				ct, _ := child["type"].(string)
				if ct != "function" && ct != "custom" || child["name"] == nil {
					return nil
				}
				names = append(names, fmt.Sprintf("%v.%v", tool["name"], child["name"]))
			}
		} else if tool["type"] == "function" && tool["name"] != nil {
			names = append(names, fmt.Sprintf("%v", tool["name"]))
		} else {
			return nil
		}
	}
	return names
}

// InspectImageToolAllowlist kiểm danh sách tool quảng cáo cho lời gọi ảnh:
// bắt buộc image_gen.imagegen, cho phép thêm view_image; mọi tool khác từ chối.
func InspectImageToolAllowlist(tools any) AllowlistVerdict {
	names := advertisedToolNames(tools)
	if names == nil {
		return AllowlistVerdict{OK: false}
	}
	allowed := map[string]bool{
		"image_gen.imagegen":   true,
		"functions.view_image": true,
		"view_image":           true,
	}
	hasImagegen := false
	for _, n := range names {
		if !allowed[n] {
			return AllowlistVerdict{OK: false, Names: names}
		}
		if n == "image_gen.imagegen" {
			hasImagegen = true
		}
	}
	return AllowlistVerdict{OK: hasImagegen, Names: names}
}

// advertisementTools rút danh sách tool từ body request: top-level `tools`
// hoặc entry `additional_tools` trong `input`. Trả (tools, là-advertisement).
func advertisementTools(body any) (any, bool) {
	req, ok := body.(map[string]any)
	if !ok {
		return nil, false
	}
	if _, ok := req["model"].(string); !ok {
		return nil, false
	}
	if tools, present := req["tools"]; present {
		return tools, true
	}
	input, ok := req["input"].([]any)
	if !ok {
		return nil, false
	}
	var tools []any
	for _, item := range input {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if entry["type"] != "additional_tools" {
			continue
		}
		// Một request quảng cáo có thể mang nhiều entry additional_tools;
		// gộp hết, không để entry sau che entry trước.
		if list, ok := entry["tools"].([]any); ok {
			tools = append(tools, list...)
		}
	}
	return tools, true
}

// InspectAdvertisementRequest kiểm một request đã bắt; nil khi không phải
// quảng cáo capability (request startup/refresh không liên quan).
func InspectAdvertisementRequest(body any) *AllowlistVerdict {
	tools, ok := advertisementTools(body)
	if !ok {
		return nil
	}
	v := InspectImageToolAllowlist(tools)
	return &v
}

// InspectTextAdvertisementRequest kiểm request text-mode: danh sách tool
// quảng cáo phải trống hoàn toàn.
func InspectTextAdvertisementRequest(body any) *AllowlistVerdict {
	tools, ok := advertisementTools(body)
	if !ok {
		return nil
	}
	names := advertisedToolNames(tools)
	if names == nil {
		return nil
	}
	return &AllowlistVerdict{OK: len(names) == 0, Names: names}
}

// codexDisabledFeatures là cờ feature ép tắt cho lời gọi clone (`--disable`).
var codexDisabledFeatures = []string{
	"apps", "plugins", "browser_use", "browser_use_external",
	"browser_use_full_cdp_access", "computer_use", "shell_tool", "unified_exec",
	"unified_exec_tty", "tool_suggest", "sleep_tool", "hooks", "skill_search",
	"skill_mcp_dependency_install", "mcp_oauth_refresh_coordination",
	"tool_call_mcp_elicitation", "code_mode", "multi_agent", "multi_agent_v2",
	"memories", "goals", "workspace_dependencies", "request_permissions_tool",
	"remote_plugin", "recommended_plugins", "realtime_conversation",
	"auth_elicitation", "daemon_auto_start", "shell_snapshot",
}

// codexEnabledFeatures là cờ feature ép bật. code_mode_host điều phối
// imagegen kể cả khi code_mode tắt — nó không phải cấp quyền: catalog lồng
// đã chứng minh riêng chỉ chứa ảnh.
var codexEnabledFeatures = []string{"skip_host_skill_discovery", "image_generation", "view_image", "code_mode_host"}

// codexTextEnabledFeatures là cờ bật cho lời gọi text-only.
var codexTextEnabledFeatures = []string{"skip_host_skill_discovery"}

// codexTextDisabledFeatures tắt rõ mặt ảnh cho lời gọi text-only, trên nền tắt chung.
var codexTextDisabledFeatures = append(append([]string{}, codexDisabledFeatures...),
	"image_generation", "view_image", "code_mode_host")

// forbiddenPromptMarkers là khối chỉ dẫn model-visible cấm còn sót lại.
var forbiddenPromptMarkers = []string{
	"apps_instructions",
	"plugins_instructions",
	"recommended_plugins",
	"tool_search",
}

// ClonePermissionProfile là tên profile quyền áp cho lời gọi ảnh (tên tuỳ ý).
const ClonePermissionProfile = "clone-images"

// IsolationConfigOverrides trả các `-c` override dựng biên đọc đã chứng minh:
// refs dir (và chỉ nó) đọc được + là workspace; temp dir OS và các gốc secret
// thường gặp bị từ chối hẳn. Giá trị là TOML inline nên không cần file config.
func IsolationConfigOverrides(refsDir string) []string {
	// Quy tắc theo-path chỉ nằm được trong bảng filesystem inline — bộ tách
	// dotted của `-c` coi đoạn khoá có nháy là literal.
	fsTable := `permissions.` + ClonePermissionProfile + `.filesystem={` +
		`":minimal"="read", "/tmp"="deny", "/private/tmp"="deny", "/var/folders"="deny", "/private/var/folders"="deny", ` +
		`":workspace_roots"={"."="read"}}`
	return []string{
		`default_permissions="` + ClonePermissionProfile + `"`,
		`permissions.` + ClonePermissionProfile + `.extends=":read-only"`,
		`permissions.` + ClonePermissionProfile + `.workspace_roots={"` + refsDir + `"=true}`,
		fsTable,
		"mcp_servers={}",
		"plugins={}",
		`shell_environment_policy.inherit="none"`,
		`approval_policy="never"`,
		`web_search="disabled"`,
		"agents.enabled=false",
		"tools.experimental_request_user_input.enabled=false",
		"tools.update_plan.enabled=false",
	}
}

// SandboxProbeArgs dựng argv `codex sandbox` cho một probe cơ học.
func SandboxProbeArgs(refsDir string, command []string) []string {
	args := []string{"sandbox"}
	for _, c := range IsolationConfigOverrides(refsDir) {
		args = append(args, "-c", c)
	}
	args = append(args, "--permission-profile", ClonePermissionProfile, "--")
	return append(args, command...)
}

// codexExecOptions là đầu vào dựng argv `codex exec`.
type codexExecOptions struct {
	refsDir          string
	modelCatalog     string
	prompt           string
	hasPrompt        bool
	images           []string
	outputFile       string
	enabledFeatures  []string
	disabledFeatures []string
}

// execArgs là bộ dựng argv chung: mỗi lời gọi = CODEX_HOME chỉ-chứa-auth +
// catalog model đã hạ do app dựng, cwd chỉ-refs, JSONL ephemeral. Catalog bắt
// buộc để metadata code_mode_only kèm theo không ghi đè `--disable code_mode`.
func execArgs(o codexExecOptions) []string {
	args := []string{
		"exec",
		"--json",
		"--ephemeral",
		"--skip-git-repo-check",
		"--ignore-user-config",
		"--ignore-rules",
		"--cd", o.refsDir,
		"--color", "never",
		"-m", CodexImageReasoningModel,
		"-c", "model_catalog_json=" + strconv.Quote(o.modelCatalog),
	}
	for _, f := range o.disabledFeatures {
		args = append(args, "--disable", f)
	}
	for _, f := range o.enabledFeatures {
		args = append(args, "--enable", f)
	}
	for _, c := range IsolationConfigOverrides(o.refsDir) {
		args = append(args, "-c", c)
	}
	for _, img := range o.images {
		args = append(args, "--image", img)
	}
	if o.outputFile != "" {
		args = append(args, "--output-last-message", o.outputFile)
	}
	if o.hasPrompt {
		args = append(args, "--", o.prompt)
	}
	return args
}

// CodexExecArgs dựng argv `codex exec` mà adapter ảnh chạy.
func CodexExecArgs(refsDir, modelCatalog string, images []string, outputFile, prompt string) []string {
	return execArgs(codexExecOptions{
		refsDir: refsDir, modelCatalog: modelCatalog, images: images,
		outputFile: outputFile, prompt: prompt, hasPrompt: prompt != "",
		enabledFeatures: codexEnabledFeatures, disabledFeatures: codexDisabledFeatures,
	})
}

// CodexTextExecArgs dựng argv `codex exec` cho lời gọi text-only (soạn
// metadata): mặt ảnh tắt rõ và không có cờ ảnh.
func CodexTextExecArgs(refsDir, modelCatalog, outputFile, prompt string) []string {
	return execArgs(codexExecOptions{
		refsDir: refsDir, modelCatalog: modelCatalog,
		outputFile: outputFile, prompt: prompt, hasPrompt: prompt != "",
		enabledFeatures: codexTextEnabledFeatures, disabledFeatures: codexTextDisabledFeatures,
	})
}

// CreateIsolatedCodexHome dựng CODEX_HOME mỗi lời gọi: mode 0700, chỉ chứa
// bản sao auth (auth.json 0600) — không config, session, MCP, app hay skill.
// Trả đường dẫn home; caller xoá sau lời gọi.
func CreateIsolatedCodexHome(sourceHome, destParent string) (string, error) {
	srcAuth := filepath.Join(sourceHome, "auth.json")
	data, err := os.ReadFile(srcAuth)
	if err != nil {
		return "", fmt.Errorf("không có Codex auth.json — đăng nhập trước: %w", err)
	}
	if err := os.MkdirAll(destParent, 0o700); err != nil {
		return "", err
	}
	home, err := os.MkdirTemp(destParent, "call-")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(home, 0o700); err != nil {
		return "", err
	}
	authDest := filepath.Join(home, "auth.json")
	if err := os.WriteFile(authDest, data, 0o600); err != nil {
		return "", err
	}
	return home, nil
}

// cliEventLine là một dòng JSONL từ `codex exec --json` dùng để rút chẩn đoán.
type cliEventLine struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// probeToolAdvertisement bắt quảng cáo capability thật của CLI qua loopback.
// Endpoint trả 400 không có model response: không có success giả, request ảnh,
// credential của chủ, request provider hay quota nào liên quan.
func (c *Codex) probeToolAdvertisement(ctx context.Context, opt struct {
	Exe      string
	RefsDir  string
	Parent   string
	Name     string
	Prompt   string
	Prepare  func(ctx context.Context, exe, home string) (string, error)
	ExecArgs func(refsDir, modelCatalog, prompt string) []string
	Inspect  func(body any) *AllowlistVerdict
}) CodexIsolationCheck {
	home := filepath.Join(opt.Parent, "tool-advertisement-home-"+opt.Name)
	_ = os.MkdirAll(home, 0o700)
	defer os.RemoveAll(home)

	// Canary không-phải-credential chọn nhánh capability ChatGPT ở local.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	claims, _ := json.Marshal(map[string]any{
		"email": "preflight@example.invalid",
		"exp":   time.Now().Unix() + 3600,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_plan_type":  "pro",
			"chatgpt_user_id":    "local-preflight-canary",
			"user_id":            "local-preflight-canary",
			"chatgpt_account_id": "00000000-0000-4000-8000-000000000000",
		},
	})
	jwt := header + "." + base64.RawURLEncoding.EncodeToString(claims) + ".local-preflight-canary"
	authJSON, _ := json.Marshal(map[string]any{
		"OPENAI_API_KEY": nil,
		"tokens": map[string]any{
			"id_token":      jwt,
			"access_token":  jwt,
			"refresh_token": "local-preflight-canary",
			"account_id":    "00000000-0000-4000-8000-000000000000",
		},
		"last_refresh": time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err := os.WriteFile(filepath.Join(home, "auth.json"), authJSON, 0o600); err != nil {
		return CodexIsolationCheck{Name: opt.Name, OK: false, Detail: err.Error()}
	}

	modelCatalog, err := opt.Prepare(ctx, opt.Exe, home)
	if err != nil {
		return CodexIsolationCheck{Name: opt.Name, OK: false, Detail: err.Error()}
	}

	var capturedMu sync.Mutex
	var captured *AllowlistVerdict
	var conns sync.WaitGroup
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2*1024*1024))
		if err == nil {
			var parsed any
			if json.Unmarshal(body, &parsed) == nil {
				if v := opt.Inspect(parsed); v != nil {
					capturedMu.Lock()
					captured = v
					capturedMu.Unlock()
				}
			}
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"local capability inspection; no model response"}}`))
	})}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return CodexIsolationCheck{Name: opt.Name, OK: false, Detail: err.Error()}
	}
	// ConnState đếm conn đang mở: Close() đuổi hết conn, Wait() chờ handler cuối
	// ghi xong trước khi đọc captured — chặn race verdict-rỗng khi request của
	// CLI đến đúng lúc Run trả.
	server.ConnState = func(_ net.Conn, st http.ConnState) {
		switch st {
		case http.StateNew:
			conns.Add(1)
		case http.StateClosed, http.StateHijacked:
			conns.Done()
		}
	}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	url := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
	args := opt.ExecArgs(opt.RefsDir, modelCatalog, opt.Prompt)
	inject := []string{
		"-c", "chatgpt_base_url=" + strconv.Quote(url),
		"-c", "model_provider=image-preflight",
		"-c", "model_providers.image-preflight={name=\"OpenAI\",base_url=" + strconv.Quote(url+"/v1") + `,wire_api="responses",requires_openai_auth=true,supports_websockets=false,request_max_retries=0,stream_max_retries=0}`,
		"--disable", "enable_request_compression",
	}
	args = append([]string{args[0]}, append(inject, args[1:]...)...)

	diagnostic := ""
	if _, err := c.d.Runner.Run(ctx, opt.Exe, args, Spec{
		Dir: opt.RefsDir,
		EnvExact: map[string]string{
			"PATH":       filepath.Dir(opt.Exe) + ":" + "/usr/bin:/bin",
			"HOME":       home,
			"CODEX_HOME": home,
		},
		Timeout:  15 * time.Second,
		LogLimit: 1024,
	}); err != nil {
		var pfe *ProcessFailedError
		if !errors.As(err, &pfe) {
			return CodexIsolationCheck{Name: opt.Name, OK: false, Detail: err.Error()}
		}
		diagnostic = lastChars(strings.TrimSpace(pfe.Stderr), 500)
		for _, line := range strings.Split(pfe.Stdout, "\n") {
			var ev cliEventLine
			if json.Unmarshal([]byte(line), &ev) == nil && ev.Error != nil && ev.Error.Message != "" {
				diagnostic = lastChars(ev.Error.Message, 500)
			}
		}
	}
	_ = server.Close()
	conns.Wait()
	capturedMu.Lock()
	verdict := captured
	capturedMu.Unlock()
	if verdict == nil {
		return CodexIsolationCheck{Name: opt.Name, OK: false,
			Detail: "không bắt được quảng cáo capability: " + diagnostic}
	}
	detail := "advertised: " + strings.Join(verdict.Names, ", ")
	if len(verdict.Names) == 0 {
		detail = "advertised: (empty)"
	}
	return CodexIsolationCheck{Name: opt.Name, OK: verdict.OK, Detail: detail}
}

// IsolationPreflight chạy mọi bằng chứng biên trên exe. Rẻ (không mạng, không
// gọi model, không cần đăng nhập) trừ `debug prompt-input` là render local.
func (c *Codex) IsolationPreflight(ctx context.Context, exe string) (CodexIsolation, error) {
	var checks []CodexIsolationCheck
	work, err := os.MkdirTemp("", "viderse-codex-iso-")
	if err != nil {
		return CodexIsolation{}, err
	}
	defer os.RemoveAll(work)
	refsDir := filepath.Join(work, "refs")
	secretDir := filepath.Join(work, "secret")
	codexHome := filepath.Join(work, "codex-home")
	for _, d := range []string{refsDir, secretDir, codexHome} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return CodexIsolation{}, err
		}
	}
	if err := os.WriteFile(filepath.Join(refsDir, "refs.txt"), []byte("REFS-OK"), 0o644); err != nil {
		return CodexIsolation{}, err
	}
	if err := os.WriteFile(filepath.Join(secretDir, "secret.txt"), []byte("CANARY-SECRET"), 0o600); err != nil {
		return CodexIsolation{}, err
	}
	// Cùng môi trường y hệt lời gọi thật: HOME là thư mục bỏ đi nên probe thấy
	// điều lời gọi thật thấy, không phải skill ~/.codex của chủ.
	env := map[string]string{
		"PATH":       filepath.Dir(exe) + ":" + "/usr/bin:/bin",
		"HOME":       codexHome,
		"CODEX_HOME": codexHome,
	}

	allCodes := make([]int, 0, 255)
	for i := 1; i <= 255; i++ {
		allCodes = append(allCodes, i)
	}
	// Con sandboxed thoát khác-0 khi bị từ chối — đó CHÍNH là tín hiệu đạt, nên
	// mọi mã thoát đều là dữ liệu và ProcessFailedError mang đuôi.
	sandbox := func(command []string) (Result, error) {
		return c.d.Runner.Run(ctx, exe, SandboxProbeArgs(refsDir, command), Spec{
			Timeout: 60 * time.Second, EnvExact: env, OKCodes: allCodes,
		})
	}

	// 1. Mặt cờ exec
	if res, err := c.d.Runner.Run(ctx, exe, []string{"exec", "--help"}, Spec{Timeout: 30 * time.Second}); err != nil {
		checks = append(checks, CodexIsolationCheck{Name: "exec-flags", OK: false, Detail: err.Error()})
	} else {
		required := []string{"--json", "--ephemeral", "--ignore-user-config", "--cd", "--skip-git-repo-check", "--output-last-message"}
		var missing []string
		for _, f := range required {
			if !strings.Contains(res.Stdout, f) {
				missing = append(missing, f)
			}
		}
		detail := "all required exec flags present"
		if len(missing) > 0 {
			detail = "missing flags: " + strings.Join(missing, ", ")
		}
		checks = append(checks, CodexIsolationCheck{
			Name: "exec-flags", OK: len(missing) == 0, Detail: detail,
		})
	}
	{
		host := "codex-code-mode-host"
		if runtime.GOOS == "windows" {
			host = "codex-code-mode-host.exe"
		}
		realExe, err := filepath.EvalSymlinks(exe)
		if err != nil {
			realExe = exe
		}
		res, err := c.d.Runner.Run(ctx, filepath.Join(filepath.Dir(realExe), host), []string{"--help"}, Spec{
			Timeout:  30 * time.Second,
			EnvExact: map[string]string{"PATH": "/usr/bin:/bin", "HOME": work, "CODEX_HOME": codexHome},
		})
		if err != nil || !strings.Contains(res.Stdout, "codex-code-mode-host") {
			detail := "required code-mode host missing or broken"
			if err != nil {
				detail += ": " + err.Error()
			}
			checks = append(checks, CodexIsolationCheck{Name: "image-tool-host", OK: false, Detail: detail})
		} else {
			checks = append(checks, CodexIsolationCheck{Name: "image-tool-host", OK: true, Detail: "required companion executable starts"})
		}
	}

	// 2a. refs đọc được trong profile
	if r, err := sandbox([]string{"/bin/cat", filepath.Join(refsDir, "refs.txt")}); err != nil {
		checks = append(checks, CodexIsolationCheck{Name: "refs-readable", OK: false, Detail: err.Error()})
	} else {
		ok := r.Code == 0 && strings.Contains(r.Stdout, "REFS-OK")
		detail := fmt.Sprintf("exit %d: %s", r.Code, strings.TrimSpace(r.Stderr))
		if r.Code == 0 {
			detail = "workspace refs readable"
		}
		checks = append(checks, CodexIsolationCheck{Name: "refs-readable", OK: ok, Detail: detail})
	}

	// 2b. secret canary bị từ chối — với deny theo-path tường minh (luật đã kiểm)
	{
		denyArgs := []string{
			"sandbox",
			"-c", `permissions.` + ClonePermissionProfile + `.extends=":read-only"`,
			"-c", `permissions.` + ClonePermissionProfile + `.filesystem.` + secretDir + `="deny"`,
			"--permission-profile", ClonePermissionProfile,
			"--", "/bin/cat", filepath.Join(secretDir, "secret.txt"),
		}
		r, err := c.d.Runner.Run(ctx, exe, denyArgs, Spec{Timeout: 60 * time.Second, EnvExact: env, OKCodes: allCodes})
		if err != nil {
			checks = append(checks, CodexIsolationCheck{Name: "secret-denied", OK: false, Detail: err.Error()})
		} else {
			denied := r.Code != 0 || !strings.Contains(r.Stdout, "CANARY-SECRET")
			detail := "deny rule blocked the secret read"
			if !denied {
				detail = fmt.Sprintf("secret readable (exit %d)", r.Code)
			}
			checks = append(checks, CodexIsolationCheck{Name: "secret-denied", OK: denied, Detail: detail})
		}
	}

	// 2c. ghi bị từ chối dưới profile (extends read-only)
	{
		target := filepath.Join(refsDir, "should-not-exist")
		r, err := sandbox([]string{"/bin/sh", "-c", `echo x > "` + target + `"`})
		if err != nil {
			checks = append(checks, CodexIsolationCheck{Name: "writes-denied", OK: false, Detail: err.Error()})
		} else {
			denied := !fileExists(target)
			_ = os.Remove(target)
			detail := "write attempt blocked"
			if !denied {
				detail = fmt.Sprintf("write succeeded (exit %d)", r.Code)
			}
			checks = append(checks, CodexIsolationCheck{Name: "writes-denied", OK: denied, Detail: detail})
		}
	}

	// 2d. mạng bị từ chối
	if r, err := sandbox([]string{"/usr/bin/curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "--max-time", "5", "https://example.com"}); err != nil {
		checks = append(checks, CodexIsolationCheck{Name: "network-denied", OK: false, Detail: err.Error()})
	} else {
		denied := !strings.Contains(r.Stdout, "200")
		detail := "outbound request blocked"
		if !denied {
			detail = "curl reached the network (code " + strings.TrimSpace(r.Stdout) + ")"
		}
		checks = append(checks, CodexIsolationCheck{Name: "network-denied", OK: denied, Detail: detail})
	}

	// 3. Mặt capability model nhìn thấy
	{
		args := []string{"debug", "prompt-input"}
		for _, f := range codexDisabledFeatures {
			args = append(args, "--disable", f)
		}
		for _, f := range codexEnabledFeatures {
			args = append(args, "--enable", f)
		}
		for _, ov := range IsolationConfigOverrides(refsDir) {
			args = append(args, "-c", ov)
		}
		args = append(args, "probe")
		res, err := c.d.Runner.Run(ctx, exe, args, Spec{Timeout: 60 * time.Second, EnvExact: env})
		if err != nil {
			checks = append(checks, CodexIsolationCheck{Name: "capabilities-denied", OK: false, Detail: err.Error()})
		} else {
			checks = append(checks, inspectPromptInput(res.Stdout, work, codexHome))
		}
	}

	// 4. Quảng cáo tool qua loopback cho ảnh và text
	checks = append(checks, c.probeToolAdvertisement(ctx, struct {
		Exe      string
		RefsDir  string
		Parent   string
		Name     string
		Prompt   string
		Prepare  func(ctx context.Context, exe, home string) (string, error)
		ExecArgs func(refsDir, modelCatalog, prompt string) []string
		Inspect  func(body any) *AllowlistVerdict
	}{
		Exe: exe, RefsDir: refsDir, Parent: work,
		Name:   "image-tool-allowlist",
		Prompt: "Inspect local capabilities only; do not generate images.",
		Prepare: func(ctx context.Context, exe, home string) (string, error) {
			return c.PrepareImageModelCatalog(ctx, exe, home)
		},
		ExecArgs: func(refsDir, modelCatalog, prompt string) []string {
			return CodexExecArgs(refsDir, modelCatalog, nil, "", prompt)
		},
		Inspect: InspectAdvertisementRequest,
	}))
	checks = append(checks, c.probeToolAdvertisement(ctx, struct {
		Exe      string
		RefsDir  string
		Parent   string
		Name     string
		Prompt   string
		Prepare  func(ctx context.Context, exe, home string) (string, error)
		ExecArgs func(refsDir, modelCatalog, prompt string) []string
		Inspect  func(body any) *AllowlistVerdict
	}{
		Exe: exe, RefsDir: refsDir, Parent: work,
		Name:   "text-tool-advertisement",
		Prompt: "Inspect local capabilities only; reply with plain text.",
		Prepare: func(ctx context.Context, exe, home string) (string, error) {
			return c.PrepareTextModelCatalog(ctx, exe, home)
		},
		ExecArgs: func(refsDir, modelCatalog, prompt string) []string {
			return CodexTextExecArgs(refsDir, modelCatalog, "", prompt)
		},
		Inspect: InspectTextAdvertisementRequest,
	}))

	state := IsolationVerified
	for _, chk := range checks {
		if !chk.OK {
			state = IsolationUnverified
			break
		}
	}
	return CodexIsolation{State: state, Checks: checks}, nil
}

// promptInputBlock là một khối của `debug prompt-input` JSON.
type promptInputBlock struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	InternalMetadata *struct {
		ContentItemKinds []string `json:"content_item_kinds"`
	} `json:"internal_chat_message_metadata_passthrough"`
}

var skillRootsRe = regexp.MustCompile(`(?i)Skill roots`)
var skillRootEntryRe = regexp.MustCompile("`r\\d+` = `([^`]+)`")

// inspectPromptInput rút phán quyết capabilities-denied từ stdout của
// `debug prompt-input`: không kind/text cấm nào được lọt, và mọi skill root
// phải nằm trong thư mục làm việc của chính probe.
func inspectPromptInput(stdout, work, codexHome string) CodexIsolationCheck {
	var blocks []promptInputBlock
	if err := json.Unmarshal([]byte(stdout), &blocks); err != nil {
		return CodexIsolationCheck{Name: "capabilities-denied", OK: false,
			Detail: "prompt-input không đọc được: " + err.Error()}
	}
	var kinds, texts []string
	for _, b := range blocks {
		if b.InternalMetadata != nil {
			kinds = append(kinds, b.InternalMetadata.ContentItemKinds...)
		}
		for _, c := range b.Content {
			if c.Text != "" {
				texts = append(texts, c.Text)
			}
		}
	}
	var leaked []string
	for _, k := range kinds {
		for _, m := range forbiddenPromptMarkers {
			if strings.Contains(k, m) || strings.Contains(m, k) {
				leaked = append(leaked, k)
			}
		}
	}
	for _, m := range forbiddenPromptMarkers {
		for _, t := range texts {
			if strings.Contains(t, "<"+m+">") {
				leaked = append(leaked, m)
			}
		}
	}
	own := func(p string) bool {
		norm := strings.TrimPrefix(p, "/private")
		return strings.HasPrefix(norm, work) || strings.HasPrefix(norm, codexHome) ||
			strings.HasPrefix(p, work) || strings.HasPrefix(p, codexHome)
	}
	offRoots := false
	for _, t := range texts {
		if !skillRootsRe.MatchString(t) {
			continue
		}
		for _, m := range skillRootEntryRe.FindAllStringSubmatch(t, -1) {
			if !own(m[1]) {
				offRoots = true
			}
		}
	}
	ok := len(leaked) == 0 && !offRoots
	detail := "no app/plugin/MCP/web/tool-search instructions; skill roots confined"
	if !ok {
		parts := append(leaked, "")
		if offRoots {
			parts = append(parts, "host skill roots")
		}
		detail = "leaked: " + strings.Join(parts, ", ")
	}
	return CodexIsolationCheck{Name: "capabilities-denied", OK: ok, Detail: detail}
}

func lastChars(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

// CodexDisabledFeatures trả bản sao danh sách: dữ liệu ghim/bảng hằng là bất biến,
// caller không chỉnh được bản gốc trong process.
func CodexDisabledFeatures() []string { return append([]string(nil), codexDisabledFeatures...) }

// CodexEnabledFeatures trả bản sao danh sách: dữ liệu ghim/bảng hằng là bất biến,
// caller không chỉnh được bản gốc trong process.
func CodexEnabledFeatures() []string { return append([]string(nil), codexEnabledFeatures...) }

// CodexTextEnabledFeatures trả bản sao danh sách: dữ liệu ghim/bảng hằng là bất biến,
// caller không chỉnh được bản gốc trong process.
func CodexTextEnabledFeatures() []string { return append([]string(nil), codexTextEnabledFeatures...) }
