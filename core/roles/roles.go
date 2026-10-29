// Package roles chứa phần dùng chung của cấu hình role → model: một role đặt
// tên (writer, editor…) có thể ghi đè provider/model/reasoning của app, kèm
// danh sách fallback. Danh sách role cụ thể thuộc về từng app; JSON shape
// (provider/model/fallbacks/reasoning_effort) phải giữ nguyên vì file cấu hình
// của người dùng đã lưu theo dạng này.
package roles

// ModelRef 表示一个 provider/model 组合。
type ModelRef struct {
	Provider string `json:"provider"` // provider 名称（Providers map 中的 key）
	Model    string `json:"model"`    // 模型名（原样透传，不做任何解析）
}

// RoleConfig 定义单个角色的模型覆盖。
type RoleConfig struct {
	Provider  string     `json:"provider"`            // 主 provider 名称（Providers map 中的 key）
	Model     string     `json:"model"`               // 主模型名（原样透传，不做任何解析）
	Fallbacks []ModelRef `json:"fallbacks,omitempty"` // 显式备用 provider/model 列表
	// ReasoningEffort 该角色的推理强度（off/low/medium/high/xhigh/max），空=继承顶层默认。
	// 由 llm.ParseThinkingLevel 校验后应用，越级值视为空。
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// Resolve trả provider/model hiệu lực của một role: mặc định là cặp mặc định
// của app, từng vế được Roles[role] ghi đè khi không rỗng. explicit=false khi
// role rỗng/"default" hoặc chưa có cấu hình — nơi gọi dùng để phân biệt "đang
// hiện mặc định" với "đã ghi đè tới đúng cặp mặc định".
func Resolve(all map[string]RoleConfig, role, defProvider, defModel string) (provider, model string, explicit bool) {
	provider, model = defProvider, defModel
	if role == "" || role == "default" {
		return provider, model, false
	}
	rc, ok := all[role]
	if !ok {
		return provider, model, false
	}
	if rc.Provider != "" {
		provider = rc.Provider
	}
	if rc.Model != "" {
		model = rc.Model
	}
	return provider, model, true
}

// ResolveReasoningEffort trả chuỗi reasoning hiệu lực của một role
// (off/low/medium/high/xhigh/max hoặc rỗng = không ghi đè).
// Ưu tiên: Roles[role].ReasoningEffort → defaultEffort → "". role rỗng hoặc
// "default" lấy thẳng defaultEffort; tính hợp lệ của giá trị do
// llm.ParseThinkingLevel quyết ở nơi áp dụng.
func ResolveReasoningEffort(all map[string]RoleConfig, role, defaultEffort string) string {
	if role != "" && role != "default" {
		if rc, ok := all[role]; ok && rc.ReasoningEffort != "" {
			return rc.ReasoningEffort
		}
	}
	return defaultEffort
}
