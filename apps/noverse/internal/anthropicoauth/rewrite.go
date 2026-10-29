package anthropicoauth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf16"

	"github.com/datnt1998/verse/apps/noverse/internal/auth"
)

// Gói Claude Pro/Max chỉ nhận request mang dấu vân tay của Claude Code CLI. Toàn bộ phần
// dưới đây port từ omp (ai/src/providers/anthropic.ts, nhánh isOAuth): khối system nhận
// dạng, billing header + chữ ký cch, tiền tố tên tool, bộ beta và header.

const (
	billingPrefix  = "x-anthropic-billing-header:"
	cchPlaceholder = "cch=00000"
	// cchSeed là seed XXH64 của chữ ký cch (5 hex = 20 bit thấp của hash cả body).
	cchSeed uint64 = 0x4d659218e32a3268
	// billingSalt trộn vào dấu vân tay version của billing header (computeFingerprint của Claude Code).
	billingSalt = "59cf53e54c78"
	// billingMarker neo cch vào system[0]; trong JSON hợp lệ không thể xuất hiện trong chuỗi
	// người dùng vì dấu nháy bên trong bị escape.
	billingMarker = `"system":[{"type":"text","text":"` + billingPrefix
	// billingWindow: cch phải nằm trong ngần ấy byte sau marker.
	billingWindow = 150
	// longCacheTTL: nhánh OAuth của omp mặc định giữ cache 1 giờ (như Claude Code).
	longCacheTTL = "1h"
)

const (
	betaOAuth             = "oauth-2025-04-20"
	betaClaudeCode        = "claude-code-20250219"
	betaInterleaved       = "interleaved-thinking-2025-05-14"
	betaThinkingTokens    = "thinking-token-count-2026-05-13"
	betaContextManagement = "context-management-2025-06-27"
	betaCachingScope      = "prompt-caching-scope-2026-01-05"
	betaStructuredOutputs = "structured-outputs-2025-12-15"
	betaMidConvSystem     = "mid-conversation-system-2026-04-07"
	betaEffort            = "effort-2025-11-24"
	betaFallbackCredit    = "fallback-credit-2026-06-01"
)

// builtinToolNames: tool dựng sẵn của Anthropic, không thêm tiền tố.
var builtinToolNames = map[string]bool{"web_search": true, "code_execution": true, "text_editor": true, "computer": true}

func applyToolPrefix(name string) string {
	if builtinToolNames[strings.ToLower(name)] {
		return name
	}
	return auth.ClaudeToolPrefix + name
}

func stripToolPrefix(name string) string {
	if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(auth.ClaudeToolPrefix)) {
		return name
	}
	return name[len(auth.ClaudeToolPrefix):]
}

// requestShape là những gì rewrite rút ra từ body để chọn bộ beta.
type requestShape struct {
	HasTools  bool
	Thinking  bool
	Strict    bool // có strict tool hoặc output_config.format
	SessionID string
}

// claudeCodeBetas: bộ beta của Claude Code. Request "agent" (có tool hoặc thinking) dùng bộ
// đầy đủ, request tiện ích dùng bộ rút gọn. `context-1m` cố ý không bao giờ gửi: token thuê
// bao không có credit long-context nên Anthropic trả 429 với mọi model gate bằng beta đó.
func claudeCodeBetas(shape requestShape) []string {
	if !shape.HasTools && !shape.Thinking {
		return withStructured([]string{betaOAuth, betaInterleaved, betaThinkingTokens, betaContextManagement, betaCachingScope, betaStructuredOutputs}, shape)
	}
	betas := []string{betaClaudeCode, betaOAuth, betaInterleaved, betaThinkingTokens, betaContextManagement, betaCachingScope, betaMidConvSystem}
	if shape.Thinking {
		betas = append(betas, betaEffort)
	}
	betas = append(betas, betaFallbackCredit)
	return withStructured(betas, shape)
}

func withStructured(betas []string, shape requestShape) []string {
	if !shape.Strict {
		return betas
	}
	for _, b := range betas {
		if b == betaStructuredOutputs {
			return betas
		}
	}
	return append(betas, betaStructuredOutputs)
}

// stainlessOS / stainlessArch ánh xạ runtime Go sang giá trị Stainless trên wire.
func stainlessOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "MacOS"
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	case "freebsd":
		return "FreeBSD"
	}
	return "Other::" + runtime.GOOS
}

func stainlessArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	case "386":
		return "x86"
	}
	return "other::" + runtime.GOARCH
}

// applyOAuthHeaders đặt header của nhánh OAuth (buildAnthropicHeaders): Bearer thay x-api-key,
// UA/x-app/Stainless của Claude Code, bộ beta. Không đặt Accept-Encoding thủ công vì Go chỉ tự
// giải nén gzip khi header đó do transport tự thêm.
func applyOAuthHeaders(h http.Header, token, sessionID string, betas []string) {
	h.Del("x-api-key")
	h.Set("Accept", "application/json")
	h.Set("Content-Type", "application/json")
	h.Set("User-Agent", auth.ClaudeCodeUserAgent())
	if sessionID != "" {
		h.Set("X-Claude-Code-Session-Id", sessionID)
	}
	h.Set("X-Stainless-Arch", stainlessArch())
	h.Set("X-Stainless-Lang", "js")
	h.Set("X-Stainless-OS", stainlessOS())
	h.Set("X-Stainless-Package-Version", auth.ClaudeCodeSDKVersion)
	h.Set("X-Stainless-Retry-Count", "0")
	h.Set("X-Stainless-Runtime", "node")
	h.Set("X-Stainless-Runtime-Version", "v26.3.0")
	h.Set("X-Stainless-Timeout", "600")
	h.Set("anthropic-beta", strings.Join(betas, ","))
	h.Set("anthropic-dangerous-direct-browser-access", "true")
	h.Set("Authorization", "Bearer "+token)
	h.Set("x-app", "cli")
	h.Set("Connection", "keep-alive")
	if h.Get("anthropic-version") == "" {
		h.Set("anthropic-version", "2023-06-01")
	}
}

type rewriteOptions struct {
	AccountID string
	InstallID string
	SessionID string
}

// systemBlock giữ đúng thứ tự khoá type,text,cache_control: neo cch phụ thuộc vào tiền tố
// `"system":[{"type":"text","text":"x-anthropic-billing-header:`.
type systemBlock struct {
	Type         string          `json:"type"`
	Text         string          `json:"text"`
	CacheControl json.RawMessage `json:"cache_control,omitempty"`
}

// rewriteMessages biến body Messages API do litellm dựng thành body nhánh OAuth của omp:
// tiền tố tên tool (tools, tool_use, tool_choice), khối system billing + nhận dạng, tools:[]
// khi không có tool, metadata.user_id kiểu Claude Code, cache 1 giờ; rồi vá chữ ký cch.
func rewriteMessages(body []byte, o rewriteOptions) ([]byte, requestShape, error) {
	var shape requestShape
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, shape, fmt.Errorf("anthropic-oauth: body request không phải JSON object: %w", err)
	}

	// tools
	var tools []map[string]json.RawMessage
	if raw, ok := top["tools"]; ok && !isJSONNull(raw) {
		if err := json.Unmarshal(raw, &tools); err != nil {
			return nil, shape, fmt.Errorf("anthropic-oauth: tools không hợp lệ: %w", err)
		}
	}
	for _, tool := range tools {
		tool["name"] = encodeJSON(applyToolPrefix(jsonString(tool["name"])))
		if string(tool["strict"]) == "true" {
			shape.Strict = true
		}
		upgradeCacheControl(tool)
	}
	shape.HasTools = len(tools) > 0
	if tools == nil {
		tools = []map[string]json.RawMessage{}
	}
	top["tools"] = encodeJSON(tools)

	// tool_choice {"type":"tool","name":…}
	if raw, ok := top["tool_choice"]; ok && !isJSONNull(raw) {
		var tc map[string]json.RawMessage
		if json.Unmarshal(raw, &tc) == nil && jsonString(tc["type"]) == "tool" && tc["name"] != nil {
			tc["name"] = encodeJSON(applyToolPrefix(jsonString(tc["name"])))
			top["tool_choice"] = encodeJSON(tc)
		}
	}

	// thinking / output_config
	if raw, ok := top["thinking"]; ok && !isJSONNull(raw) {
		var th map[string]json.RawMessage
		if json.Unmarshal(raw, &th) == nil {
			shape.Thinking = jsonString(th["type"]) != "disabled"
		}
	}
	if raw, ok := top["output_config"]; ok && !isJSONNull(raw) {
		var oc map[string]json.RawMessage
		if json.Unmarshal(raw, &oc) == nil && oc["format"] != nil {
			shape.Strict = true
		}
	}

	// messages
	var firstUser string
	if raw, ok := top["messages"]; ok && !isJSONNull(raw) {
		var msgs []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &msgs); err != nil {
			return nil, shape, fmt.Errorf("anthropic-oauth: messages không hợp lệ: %w", err)
		}
		firstUser = firstWireUserText(msgs)
		changed := false
		for _, msg := range msgs {
			if rewriteMessageBlocks(msg) {
				changed = true
			}
		}
		if changed {
			top["messages"] = encodeJSON(msgs)
		}
	}

	// system: [billing, nhận dạng Claude Code, ...system gốc]
	system, err := buildSystemBlocks(top["system"], firstUser)
	if err != nil {
		return nil, shape, err
	}
	top["system"] = encodeJSON(system)

	// metadata.user_id
	meta := map[string]json.RawMessage{}
	if raw, ok := top["metadata"]; ok && !isJSONNull(raw) {
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, shape, fmt.Errorf("anthropic-oauth: metadata không hợp lệ: %w", err)
		}
	}
	shape.SessionID = o.SessionID
	if uid := jsonString(meta["user_id"]); isClaudeUserID(uid) {
		if s := extractMetadataSessionID(uid); s != "" {
			shape.SessionID = s
		}
	} else {
		meta["user_id"] = encodeJSON(claudeJSONUserID(o.InstallID, o.AccountID, o.SessionID))
	}
	top["metadata"] = encodeJSON(meta)

	out := bytes.TrimRight(encodeJSON(top), "\n")
	patchCCH(out)
	return out, shape, nil
}

// rewriteMessageBlocks thêm tiền tố tên cho tool_use và nâng cache_control; true nếu đã sửa.
func rewriteMessageBlocks(msg map[string]json.RawMessage) bool {
	raw := msg["content"]
	if len(raw) == 0 || raw[0] != '[' {
		return false
	}
	var blocks []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return false
	}
	changed := false
	for _, block := range blocks {
		if jsonString(block["type"]) == "tool_use" {
			block["name"] = encodeJSON(applyToolPrefix(jsonString(block["name"])))
			changed = true
		}
		if upgradeCacheControl(block) {
			changed = true
		}
	}
	if changed {
		msg["content"] = encodeJSON(blocks)
	}
	return changed
}

// upgradeCacheControl nâng cache_control ephemeral chưa có ttl lên 1h (nhánh OAuth mặc định
// "long"), kể cả block lồng trong tool_result; true nếu có thay đổi.
func upgradeCacheControl(m map[string]json.RawMessage) bool {
	changed := false
	if raw, ok := m["cache_control"]; ok {
		if up, ok := upgradedCacheControl(raw); ok {
			m["cache_control"] = up
			changed = true
		}
	}
	if raw := m["content"]; len(raw) > 0 && raw[0] == '[' {
		var nested []map[string]json.RawMessage
		if json.Unmarshal(raw, &nested) == nil {
			inner := false
			for _, n := range nested {
				if upgradeCacheControl(n) {
					inner = true
				}
			}
			if inner {
				m["content"] = encodeJSON(nested)
				changed = true
			}
		}
	}
	return changed
}

func upgradedCacheControl(raw json.RawMessage) (json.RawMessage, bool) {
	var cc map[string]any
	if json.Unmarshal(raw, &cc) != nil || cc["type"] != "ephemeral" || cc["ttl"] != nil {
		return raw, false
	}
	cc["ttl"] = longCacheTTL
	return encodeJSON(cc), true
}

func buildSystemBlocks(raw json.RawMessage, firstUser string) ([]systemBlock, error) {
	var existing []systemBlock
	switch {
	case len(raw) == 0 || isJSONNull(raw):
	case raw[0] == '"':
		existing = []systemBlock{{Type: "text", Text: jsonString(raw)}}
	default:
		if err := json.Unmarshal(raw, &existing); err != nil {
			return nil, fmt.Errorf("anthropic-oauth: system không hợp lệ: %w", err)
		}
	}
	hasBilling := false
	for i := range existing {
		if existing[i].Type == "" {
			existing[i].Type = "text"
		}
		if strings.HasPrefix(existing[i].Text, billingPrefix) {
			hasBilling = true
		}
		if len(existing[i].CacheControl) > 0 {
			if up, ok := upgradedCacheControl(existing[i].CacheControl); ok {
				existing[i].CacheControl = up
			}
		}
	}
	if hasBilling {
		return existing, nil
	}
	blocks := make([]systemBlock, 0, len(existing)+2)
	blocks = append(blocks,
		systemBlock{Type: "text", Text: billingHeader(firstUser)},
		systemBlock{Type: "text", Text: auth.ClaudeCodeIdentity, CacheControl: json.RawMessage(`{"type":"ephemeral","ttl":"` + longCacheTTL + `"}`)},
	)
	return append(blocks, existing...), nil
}

// billingHeader: "x-anthropic-billing-header: cc_version=<ver>.<sha256(salt+msg[4,7,20]+ver)[:3]>;
// cc_entrypoint=cli; cch=00000;" — msg là văn bản user đầu tiên, chỉ số theo đơn vị UTF-16 như JS.
func billingHeader(firstUser string) string {
	units := utf16.Encode([]rune(firstUser))
	var k strings.Builder
	for _, i := range []int{4, 7, 20} {
		if i < len(units) {
			k.WriteString(string(utf16.Decode([]uint16{units[i]})))
		} else {
			k.WriteString("0")
		}
	}
	version := auth.ClaudeCodeVersion()
	sum := sha256.Sum256([]byte(billingSalt + k.String() + version))
	suffix := hex.EncodeToString(sum[:])[:3]
	return fmt.Sprintf("%s cc_version=%s.%s; cc_entrypoint=cli; %s;", billingPrefix, version, suffix, cchPlaceholder)
}

// patchCCH ghi chữ ký cch vào chỗ giữ chỗ trong body (tại chỗ). Hash tính trên body còn
// placeholder, đúng như Claude Code. Trả false nếu không có/không neo được billing header.
func patchCCH(body []byte) bool {
	idx := bytes.Index(body, []byte(billingMarker))
	if idx < 0 {
		return false
	}
	from := idx + len(billingMarker)
	pos := bytes.Index(body[from:], []byte(cchPlaceholder))
	if pos < 0 || pos > billingWindow {
		return false
	}
	cch := formatCCH(xxh64(body, cchSeed))
	copy(body[from+pos+4:], cch)
	return true
}

// firstWireUserText: văn bản user đầu tiên (không rỗng) làm hạt giống fingerprint. Message user
// chỉ chứa tool_result bị bỏ qua (omp xếp chúng vào role riêng); gặp ảnh trước khi có chữ → "".
func firstWireUserText(msgs []map[string]json.RawMessage) string {
	for _, msg := range msgs {
		if jsonString(msg["role"]) != "user" {
			continue
		}
		raw := msg["content"]
		if len(raw) == 0 {
			continue
		}
		if raw[0] == '"' {
			if s := jsonString(raw); strings.TrimSpace(s) != "" {
				return s
			}
			continue
		}
		var blocks []map[string]json.RawMessage
		if json.Unmarshal(raw, &blocks) != nil {
			continue
		}
		hasImage := false
		for _, b := range blocks {
			switch jsonString(b["type"]) {
			case "text":
				if s := jsonString(b["text"]); strings.TrimSpace(s) != "" {
					return s
				}
			case "image":
				hasImage = true
			}
		}
		if hasImage {
			return ""
		}
	}
	return ""
}

var cloakingUserID = regexp.MustCompile(`^user_[0-9a-fA-F]{64}_account_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}_session_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func isClaudeUserID(uid string) bool {
	return cloakingUserID.MatchString(uid) || jsonUserIDSession(uid) != ""
}

func jsonUserIDSession(uid string) string {
	if uid == "" || uid[0] != '{' {
		return ""
	}
	var parsed struct {
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal([]byte(uid), &parsed) != nil {
		return ""
	}
	return parsed.SessionID
}

func extractMetadataSessionID(uid string) string {
	if cloakingUserID.MatchString(uid) {
		return uid[strings.LastIndex(uid, "_session_")+len("_session_"):]
	}
	return jsonUserIDSession(uid)
}

// claudeJSONUserID: metadata.user_id dạng JSON của Claude Code. device_id suy ra ổn định từ
// mã cài đặt (+ tài khoản nếu có) như deriveClaudeDeviceId của omp.
func claudeJSONUserID(installID, accountID, sessionID string) string {
	userID := struct {
		DeviceID    string `json:"device_id"`
		SessionID   string `json:"session_id"`
		AccountUUID string `json:"account_uuid,omitempty"`
	}{DeviceID: deriveDeviceID(installID, accountID), SessionID: sessionID, AccountUUID: accountID}
	return string(bytes.TrimRight(encodeJSON(userID), "\n"))
}

func deriveDeviceID(installID, accountID string) string {
	h := sha256.New()
	if accountID != "" {
		h.Write([]byte("ainovel-claude-device-id-v2\x00" + installID + "\x00" + accountID))
	} else {
		h.Write([]byte("ainovel-claude-device-id-v1:" + installID))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// encodeJSON mã hoá không escape HTML (giữ nguyên <, >, & trong văn bản truyện).
func encodeJSON(v any) json.RawMessage {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return bytes.TrimRight(buf.Bytes(), "\n")
}

func isJSONNull(raw json.RawMessage) bool { return string(bytes.TrimSpace(raw)) == "null" }

func jsonString(raw json.RawMessage) string {
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}

// formatCCH định dạng chữ ký cch: 20 bit thấp của hash, 5 hex.
func formatCCH(hash uint64) string { return fmt.Sprintf("%05x", hash&0xfffff) }
