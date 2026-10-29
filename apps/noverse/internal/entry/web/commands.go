package web

import (
	"net/http"

	"github.com/datnt1998/verse/apps/noverse/internal/commands"
)

// serveCommands 处理 GET /api/commands（14 §A G11）：斜杠命令注册表，
// 供 web 的 popover "/" 与 trang hướng dẫn dùng chung, không chép tay danh sách.
// 已登录用户均可读取。
func (g *Gateway) serveCommands(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/api/commands" {
		return false
	}
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return true
	}
	writeJSON(w, http.StatusOK, commands.Specs())
	return true
}
