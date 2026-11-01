package web

// AU4: bulk "chuyển trợ lý sang nhà cung cấp khác" + undo an toàn khi cấu hình
// đã bị người khác sửa. Chỉ quản trị được gọi; chỉ provider/model/reasoning của
// các vai đang dùng `from` (và fallback liên quan) bị đổi, mọi thiết lập khác
// giữ nguyên. Undo khôi phục đúng các giá trị AI đã đổi, không ghi đè phần còn
// lại của cấu hình.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/core/roles"
	"github.com/datnt1998/verse/platform/identity"
)

type swapSelection struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
}

type swapRow struct {
	Role string        `json:"role"`
	From swapSelection `json:"from"`
	To   swapSelection `json:"to"`
}

type swapSkipped struct {
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

type swapRequest struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Model string `json:"model"`
}

type swapUndoRequest struct {
	Token string `json:"token"`
}

type swapUndoRow struct {
	Row             swapRow
	FallbacksBefore []roles.ModelRef
	FallbacksAfter  []roles.ModelRef
}

type swapUndoEntry struct {
	expiresAt time.Time
	expected  string
	rows      []swapUndoRow
}

var configSwapStore = struct {
	sync.Mutex
	entries map[string]swapUndoEntry
}{entries: map[string]swapUndoEntry{}}

const swapUndoTTL = 30 * time.Minute

func (g *Gateway) swapProvider(w http.ResponseWriter, r *http.Request, user identity.User) {
	if user.Role != identity.RoleAdmin {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return
	}
	var body swapRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		jsonError(w, 400, "JSON không hợp lệ")
		return
	}
	body.From = strings.TrimSpace(body.From)
	body.To = strings.TrimSpace(body.To)
	body.Model = strings.TrimSpace(body.Model)
	if body.From == "" || body.To == "" {
		jsonError(w, 400, "Thiếu nhà cung cấp nguồn hoặc đích")
		return
	}
	if body.From == body.To {
		jsonError(w, 400, "Nhà cung cấp đích phải khác nhà cung cấp nguồn")
		return
	}

	t, err := g.resolveConfigScope("global", "", "")
	if err != nil {
		scopeError(w, err)
		return
	}
	configSwapStore.Lock()
	defer configSwapStore.Unlock()

	snapshot, err := configWriteSnapshot(t, r.Header.Get("If-Match"))
	if err != nil {
		if isStorageConflict(err) {
			g.writeConfigConflict(w, r, t, user)
		} else {
			scopeError(w, err)
		}
		return
	}
	cfg, exists, err := g.loadTargetConfig(r.Context(), t, user)
	if err != nil {
		scopeError(w, err)
		return
	}
	if !exists {
		jsonError(w, 400, "Chưa có cấu hình chung để chuyển nhà cung cấp")
		return
	}
	targetModel := body.Model
	if targetModel == "" {
		targetModel = firstProviderModel(cfg, body.To)
	}
	if targetModel == "" {
		jsonError(w, 400, fmt.Sprintf("Nhà cung cấp %s chưa có model nào để chuyển sang", body.To))
		return
	}
	if _, ok := cfg.Providers[body.To]; !ok {
		jsonError(w, 400, fmt.Sprintf("Nhà cung cấp %s chưa được cấu hình", body.To))
		return
	}
	if _, ok := cfg.Providers[body.To].ModelConfig(targetModel); !ok {
		jsonError(w, 400, fmt.Sprintf("Model %s không thuộc nhà cung cấp %s", targetModel, body.To))
		return
	}

	rows := make([]swapUndoRow, 0, 1+len(cfg.Roles))
	oldDefaultProvider := cfg.Provider
	// Mặc định dùng chung.
	if cfg.Provider == body.From {
		before := swapSelection{Provider: cfg.Provider, Model: cfg.ModelName, ReasoningEffort: cfg.ReasoningEffort}
		after := swapSelection{
			Provider:        body.To,
			Model:           targetModel,
			ReasoningEffort: compatibleThinking(cfg, body.To, targetModel, cfg.ReasoningEffort),
		}
		cfg.Provider, cfg.ModelName, cfg.ReasoningEffort = after.Provider, after.Model, after.ReasoningEffort
		rows = append(rows, swapUndoRow{Row: swapRow{Role: "default", From: before, To: after}})
	}
	// Vai ghi đè provider, hoặc ghim model nhưng thừa hưởng provider mặc định.
	names := make([]string, 0, len(cfg.Roles))
	for role := range cfg.Roles {
		names = append(names, role)
	}
	sort.Strings(names)
	for _, role := range names {
		rc := cfg.Roles[role]
		effective := rc.Provider
		if effective == "" {
			if rc.Model == "" {
				continue // thuần thừa hưởng; đổi mặc định là đủ
			}
			effective = oldDefaultProvider
		}
		if effective != body.From {
			continue
		}
		before := swapSelection{Provider: rc.Provider, Model: rc.Model, ReasoningEffort: rc.ReasoningEffort}
		after := swapSelection{
			Provider:        body.To,
			Model:           targetModel,
			ReasoningEffort: compatibleThinking(cfg, body.To, targetModel, rc.ReasoningEffort),
		}
		rc.Provider, rc.Model, rc.ReasoningEffort = after.Provider, after.Model, after.ReasoningEffort
		fallbacksBefore := append([]roles.ModelRef(nil), rc.Fallbacks...)
		rc.Fallbacks = dropFallbacksFor(rc.Fallbacks, body.From, body.To)
		cfg.Roles[role] = rc
		rows = append(rows, swapUndoRow{
			Row:             swapRow{Role: role, From: before, To: after},
			FallbacksBefore: fallbacksBefore,
			FallbacksAfter:  append([]roles.ModelRef(nil), rc.Fallbacks...),
		})
	}
	if len(rows) == 0 {
		jsonError(w, 400, fmt.Sprintf("Không có trợ lý nào đang dùng %s", body.From))
		return
	}

	expected := strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), "\"")
	if expected != "" && expected != "*" {
		if current := g.configVersion(r, t, user); current != "" && expected != current {
			g.writeConfigConflict(w, r, t, user)
			return
		}
	}
	if err := g.saveConfigWriteSnapshot(r.Context(), t, user, cfg, expected, snapshot); err != nil {
		if isStorageConflict(err) {
			g.writeConfigConflict(w, r, t, user)
			return
		}
		jsonError(w, 500, err.Error())
		return
	}
	token := newSwapToken()
	expires := time.Now().Add(swapUndoTTL)
	configSwapStore.entries[token] = swapUndoEntry{
		expiresAt: expires,
		expected:  swapExpectedFingerprint(rows),
		rows:      rows,
	}
	pruneSwapEntries(time.Now())

	// The config write is durable at this point. A failed worker reload must not
	// pretend it rolled back or hide the undo token: return the committed result
	// with a warning; the next turn/resume picks the new selection up through the
	// existing reload/restart path.
	warning := ""
	if err := g.refreshWorkerModelContexts(r.Context(), t.scope, t.project, ""); err != nil {
		g.configChanged(t.scope, t.project, "")
	}
	if err := g.syncWorkerModelSelection(r.Context(), t.scope, t.project, ""); err != nil {
		g.configChanged(t.scope, t.project, "")
		warning = "Đã chuyển nhà cung cấp trong cấu hình; trợ lý đang chạy sẽ dùng thiết lập mới ở lượt kế tiếp"
	}

	swapped := make([]swapRow, 0, len(rows))
	for _, row := range rows {
		swapped = append(swapped, row.Row)
	}
	result := map[string]any{
		"swapped":   swapped,
		"skipped":   []swapSkipped{},
		"undoToken": token,
		"expiresAt": expires.Format(time.RFC3339),
		"version":   g.configVersion(r, t, user),
	}
	if warning != "" {
		result["warning"] = warning
	}
	writeJSON(w, 200, result)
}

func (g *Gateway) undoSwapProvider(w http.ResponseWriter, r *http.Request, user identity.User) {
	if user.Role != identity.RoleAdmin {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return
	}
	var body swapUndoRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		jsonError(w, 400, "JSON không hợp lệ")
		return
	}
	token := strings.TrimSpace(body.Token)
	if token == "" {
		jsonError(w, 400, "Thiếu mã hoàn tác")
		return
	}
	t, err := g.resolveConfigScope("global", "", "")
	if err != nil {
		scopeError(w, err)
		return
	}
	configSwapStore.Lock()
	defer configSwapStore.Unlock()
	entry, ok := configSwapStore.entries[token]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(configSwapStore.entries, token)
		jsonError(w, 410, "Mã hoàn tác đã hết hạn — chuyển lại nhà cung cấp nếu cần")
		return
	}

	snapshot, err := configWriteSnapshot(t, r.Header.Get("If-Match"))
	if err != nil {
		if isStorageConflict(err) {
			g.writeConfigConflict(w, r, t, user)
		} else {
			scopeError(w, err)
		}
		return
	}
	cfg, exists, err := g.loadTargetConfig(r.Context(), t, user)
	if err != nil {
		scopeError(w, err)
		return
	}
	if !exists {
		jsonError(w, 410, "Cấu hình chung không còn để hoàn tác")
		return
	}
	if got := swapCurrentFingerprint(cfg, entry.rows); got != entry.expected {
		g.writeConfigConflict(w, r, t, user)
		return
	}
	// Chỉ trả lại các giá trị AI đã đổi trong lần chuyển này.
	for _, row := range entry.rows {
		switch row.Row.Role {
		case "default":
			cfg.Provider = row.Row.From.Provider
			cfg.ModelName = row.Row.From.Model
			cfg.ReasoningEffort = row.Row.From.ReasoningEffort
		default:
			rc := cfg.Roles[row.Row.Role]
			rc.Provider = row.Row.From.Provider
			rc.Model = row.Row.From.Model
			rc.ReasoningEffort = row.Row.From.ReasoningEffort
			rc.Fallbacks = append([]roles.ModelRef(nil), row.FallbacksBefore...)
			cfg.Roles[row.Row.Role] = rc
		}
	}
	// The undo stores selections, never provider definitions or credentials.
	// Catalog edits are deliberately not part of the fingerprint; validate the
	// restored references against the current catalog before committing.
	if err := validateSwapUndoReferences(cfg, entry.rows); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":    "Không thể hoàn tác vì thiết lập AI đã thay đổi: " + err.Error(),
			"conflict": true,
		})
		return
	}
	expected := strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), "\"")
	if err := g.saveConfigWriteSnapshot(r.Context(), t, user, cfg, expected, snapshot); err != nil {
		if isStorageConflict(err) {
			g.writeConfigConflict(w, r, t, user)
			return
		}
		jsonError(w, 500, err.Error())
		return
	}
	delete(configSwapStore.entries, token)
	warning := ""
	if err := g.refreshWorkerModelContexts(r.Context(), t.scope, t.project, ""); err != nil {
		g.configChanged(t.scope, t.project, "")
	}
	if err := g.syncWorkerModelSelection(r.Context(), t.scope, t.project, ""); err != nil {
		g.configChanged(t.scope, t.project, "")
		warning = "Đã hoàn tác trong cấu hình; trợ lý đang chạy sẽ dùng thiết lập mới ở lượt kế tiếp"
	}
	restored := make([]swapRow, 0, len(entry.rows))
	for _, row := range entry.rows {
		restored = append(restored, swapRow{Role: row.Row.Role, From: row.Row.To, To: row.Row.From})
	}
	result := map[string]any{"restored": restored, "version": g.configVersion(r, t, user)}
	if warning != "" {
		result["warning"] = warning
	}
	writeJSON(w, 200, result)
}

func validateSwapUndoReferences(cfg bootstrap.Config, rows []swapUndoRow) error {
	validate := func(provider, model string) error {
		pc, ok := cfg.Providers[provider]
		if !ok {
			return fmt.Errorf("Nhà cung cấp %s không còn được cấu hình", provider)
		}
		if _, ok := pc.ModelConfig(model); !ok {
			return fmt.Errorf("Model %s không còn thuộc nhà cung cấp %s", model, provider)
		}
		return bootstrap.ValidateProvider(provider, pc)
	}
	for _, row := range rows {
		provider, model, effort := cfg.Provider, cfg.ModelName, cfg.ReasoningEffort
		if row.Row.Role != "default" {
			provider, model, _ = roles.Resolve(cfg.Roles, row.Row.Role, cfg.Provider, cfg.ModelName)
			effort = roles.ResolveReasoningEffort(cfg.Roles, row.Row.Role, cfg.ReasoningEffort)
			for _, fallback := range cfg.Roles[row.Row.Role].Fallbacks {
				if err := validate(fallback.Provider, fallback.Model); err != nil {
					return err
				}
			}
		}
		if err := validate(provider, model); err != nil {
			return err
		}
		if msg := checkThinking(cfg, provider, model, effort); msg != "" {
			return errors.New(msg)
		}
	}
	if msg := validateFallbackProviders(bootstrap.Config{}, cfg, false); msg != "" {
		return errors.New(msg)
	}
	return nil
}

func (g *Gateway) writeConfigConflict(w http.ResponseWriter, r *http.Request, t configTarget, user identity.User) {
	current, loadErr := g.buildConfigResponse(r.Context(), t, user)
	if loadErr != nil {
		scopeError(w, loadErr)
		return
	}
	version := g.configVersion(r, t, user)
	current["version"] = version
	writeVersionToken(w, version)
	writeJSON(w, http.StatusConflict, map[string]any{
		"error": "Đã có thay đổi từ nơi khác", "conflict": true, "version": version, "current": current,
	})
}

func (g *Gateway) configVersion(r *http.Request, t configTarget, user identity.User) string {
	if t.hosted {
		docs, err := g.hostedConfigDocuments()
		if err != nil {
			return ""
		}
		token, err := docs.Version(r.Context(), configActor(user), t.project, t.story.Story)
		if err != nil {
			return ""
		}
		return token
	}
	return fileVersion(t.path)
}

func isStorageConflict(err error) bool {
	return errors.Is(err, errs.ErrStorageConflict)
}

func firstProviderModel(cfg bootstrap.Config, provider string) string {
	pc, ok := cfg.Providers[provider]
	if !ok {
		return ""
	}
	for _, model := range pc.Models {
		if name := strings.TrimSpace(model.Name); name != "" {
			return name
		}
	}
	return ""
}

func compatibleThinking(cfg bootstrap.Config, provider, model, level string) string {
	if strings.TrimSpace(level) == "" {
		return ""
	}
	lv, err := llm.ParseThinkingLevel(level)
	if err != nil {
		return ""
	}
	probe, err := cfg.ProbeModel(provider, model)
	if err != nil {
		return ""
	}
	if !llm.ThinkingPolicyFor(probe).Allows(lv) {
		return ""
	}
	return string(lv)
}

func dropFallbacksFor(fallbacks []roles.ModelRef, providers ...string) []roles.ModelRef {
	drop := map[string]struct{}{}
	for _, provider := range providers {
		drop[provider] = struct{}{}
	}
	out := make([]roles.ModelRef, 0, len(fallbacks))
	for _, fallback := range fallbacks {
		if _, bad := drop[fallback.Provider]; bad {
			continue
		}
		out = append(out, fallback)
	}
	return out
}

// swapExpectedFingerprint pins exactly the AI values the swap produced. Undo
// refuses if any of them changed after the swap (another tab, a role edit, a
// new model), so it never overwrites a concurrent decision.
func swapExpectedFingerprint(rows []swapUndoRow) string {
	type rowState struct {
		Role      string
		Selection swapSelection
		Fallbacks []roles.ModelRef
	}
	states := make([]rowState, 0, len(rows))
	for _, row := range rows {
		states = append(states, rowState{
			Role: row.Row.Role, Selection: row.Row.To,
			Fallbacks: append([]roles.ModelRef(nil), row.FallbacksAfter...),
		})
	}
	return valueVersion(states)
}

// swapCurrentFingerprint reads the same fields from the config as it is now.
func swapCurrentFingerprint(cfg bootstrap.Config, rows []swapUndoRow) string {
	type rowState struct {
		Role      string
		Selection swapSelection
		Fallbacks []roles.ModelRef
	}
	states := make([]rowState, 0, len(rows))
	for _, row := range rows {
		state := rowState{Role: row.Row.Role}
		if row.Row.Role == "default" {
			state.Selection = swapSelection{Provider: cfg.Provider, Model: cfg.ModelName, ReasoningEffort: cfg.ReasoningEffort}
		} else {
			rc := cfg.Roles[row.Row.Role]
			state.Selection = swapSelection{Provider: rc.Provider, Model: rc.Model, ReasoningEffort: rc.ReasoningEffort}
			state.Fallbacks = append([]roles.ModelRef(nil), rc.Fallbacks...)
		}
		states = append(states, state)
	}
	return valueVersion(states)
}

func newSwapToken() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("swap-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func pruneSwapEntries(now time.Time) {
	for token, entry := range configSwapStore.entries {
		if now.After(entry.expiresAt) {
			delete(configSwapStore.entries, token)
		}
	}
}
