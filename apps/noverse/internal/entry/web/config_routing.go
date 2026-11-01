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
