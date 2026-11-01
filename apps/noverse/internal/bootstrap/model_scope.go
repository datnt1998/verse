package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
)

// Nhà cung cấp + model + mức suy luận dùng chung cho cả ứng dụng (chủ dự án chốt 02/11).
// Lớp dự án/truyện không được quyết định các khoá này: truyện từng ghim
// provider=xai-oauth/model=grok-4.7 trong config riêng, nên khi gói thuê bao hết hạn
// người dùng đổi provider ở cài đặt chung mà truyện vẫn chạy provider cũ.
//
// StripModelSelection là nơi duy nhất thực thi quy tắc đó; merge (file + hosted),
// nội dung trả về của /api/config và đường khởi động worker đều đi qua đây.
var modelSelectionKeys = []string{"provider", "model", "reasoning_effort", "roles", "providers"}

// StripModelSelection bỏ mọi lựa chọn provider/model khỏi một lớp gần.
// Các khoá còn lại (ngân sách, quy tắc viết, TTS, duyệt từng chương…) vẫn theo cấp.
func StripModelSelection(cfg Config) Config {
	cfg.Provider = ""
	cfg.ModelName = ""
	cfg.ReasoningEffort = ""
	cfg.Roles = nil
	cfg.Providers = nil
	return cfg
}

// stripModelSelectionJSON bỏ cùng các khoá đó khỏi bản JSON thô để /api/config không
// hiển thị (và không ghi lại) ghim cũ ở cấp dự án/truyện.
func stripModelSelectionJSON(raw map[string]any) {
	if raw == nil {
		return
	}
	for _, key := range modelSelectionKeys {
		delete(raw, key)
	}
}

// HasModelSelectionJSON reports whether a raw project/story document still
// carries a provider/model pin. AU1 migrates those documents physically: the
// pin is not merely hidden from /api/config, it is removed from the stored
// document while every non-model setting (style, budget, notify, tts, rules…)
// stays exactly as the user wrote it.
func HasModelSelectionJSON(raw map[string]any) bool {
	if raw == nil {
		return false
	}
	for _, key := range modelSelectionKeys {
		if _, ok := raw[key]; ok {
			return true
		}
	}
	return false
}

// MigrateModelSelectionJSON removes the model-selection keys from a raw
// document and reports whether anything changed. Unknown keys are preserved.
func MigrateModelSelectionJSON(raw map[string]any) bool {
	if !HasModelSelectionJSON(raw) {
		return false
	}
	stripModelSelectionJSON(raw)
	return true
}

// MigrateConfigFile physically rewrites one project/story config.json when it
// still pins provider/model. A shared cross-process lock covers the whole
// read/strip/atomic-write transaction. It keeps the file mode and only touches
// the five model-selection keys; a damaged file is
// left alone (load already fails loud for it). It is best-effort: callers log
// and continue with the in-memory stripped config.
func MigrateConfigFile(path string) (bool, error) {
	changed := false
	err := withConfigFileLock(path, func() error {
		raw, err := ReadConfigJSON(path)
		if err != nil || raw == nil {
			return err
		}
		if !MigrateModelSelectionJSON(raw) {
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			return err
		}
		if err := writeConfigDataLocked(path, data, info.Mode().Perm()); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed, err
}

// MigrateConfigDocument returns the stripped document bytes and whether the
// stored document still pinned provider/model. Used for hosted layers, where
// the caller owns the CAS write.
func MigrateConfigDocument(data []byte) ([]byte, bool, error) {
	if len(data) == 0 {
		return data, false, nil
	}
	var raw map[string]any
	if err := json.Unmarshal(stripJSONComments(data), &raw); err != nil {
		return data, false, fmt.Errorf("tài liệu cấu hình không hợp lệ: %w", err)
	}
	if !MigrateModelSelectionJSON(raw) {
		return data, false, nil
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return data, false, err
	}
	return append(out, '\n'), true, nil
}
