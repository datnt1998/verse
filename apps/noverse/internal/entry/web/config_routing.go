package web

import (
	"reflect"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
)

// changedRoutingField reports (in Vietnamese) the first provider/TTS field in
// a project or story layer that sets where requests go, or what kind of
// endpoint answers them, to something other than what the layer already
// stores. The credentials of those calls come from the global layer, so only
// the global layer may choose the destination. Echoing the stored value back
// is allowed: the settings screen saves the whole layer on every edit, and the
// hosted merge ignores such stored values anyway (bootstrap.HostedLayerConfig).
func changedRoutingField(saved, next bootstrap.Config) string {
	for name, pc := range next.Providers {
		old := saved.Providers[name]
		// Clearing a stored value never redirects anything; only a new or changed one does.
		if (pc.BaseURL != "" && pc.BaseURL != old.BaseURL) ||
			(pc.Type != "" && pc.Type != old.Type) ||
			(len(pc.Extra) > 0 && !reflect.DeepEqual(pc.Extra, old.Extra)) {
			return "Chỉ có thể đổi địa chỉ, giao thức và tuỳ chọn kết nối của dịch vụ mô hình trong cài đặt chung, không đặt riêng cho dự án hoặc truyện"
		}
	}
	if next.TTS.Endpoint != "" && next.TTS.Endpoint != saved.TTS.Endpoint {
		return "Chỉ có thể đặt địa chỉ máy chủ giọng đọc trong cài đặt chung, không đặt riêng cho dự án hoặc truyện"
	}
	return ""
}

// changedAppWideField reports (in Vietnamese) the first provider/model choice
// in a project or story layer that differs from what the layer already stores.
// The owner decided (02/11) that provider, model, reasoning effort and
// fallbacks apply to the whole app: a story that pinned its own provider kept
// writing with an expired subscription after the shared provider was switched.
// Echoing a stored value is allowed so an older client that saves the whole
// layer keeps working; the value is stripped before it is written back, which
// migrates the stale pin away.
func changedAppWideField(saved, next bootstrap.Config) string {
	modelMessage := "Model và mức suy luận dùng chung cho cả ứng dụng — đổi ở Tuỳ chỉnh › Trợ lý AI, không đặt riêng cho dự án hoặc truyện"
	if next.Provider != "" && next.Provider != saved.Provider {
		return modelMessage
	}
	if next.ModelName != "" && next.ModelName != saved.ModelName {
		return modelMessage
	}
	if next.ReasoningEffort != "" && next.ReasoningEffort != saved.ReasoningEffort {
		return modelMessage
	}
	for key, role := range next.Roles {
		old := saved.Roles[key]
		if (role.Provider != "" && role.Provider != old.Provider) ||
			(role.Model != "" && role.Model != old.Model) ||
			(role.ReasoningEffort != "" && role.ReasoningEffort != old.ReasoningEffort) ||
			(len(role.Fallbacks) > 0 && !reflect.DeepEqual(role.Fallbacks, old.Fallbacks)) {
			return modelMessage
		}
	}
	for name, pc := range next.Providers {
		old := saved.Providers[name]
		if len(pc.Models) > 0 && !reflect.DeepEqual(pc.Models, old.Models) {
			return "Danh sách model dùng chung cho cả ứng dụng — thêm hoặc bớt model ở Kết nối AI, không đặt riêng cho dự án hoặc truyện"
		}
	}
	return ""
}
