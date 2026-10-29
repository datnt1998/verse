// Package web là HTTP gateway của vidverse serve: SPA nhúng (web/ build ra
// dist/) cùng thiết lập lần đầu, đăng nhập và phiên của platform/webapp.
// Route /api nghiệp vụ gắn thêm bằng Server.Handle khi có.
package web

import (
	"embed"
	"io/fs"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

//go:embed all:dist
var ui embed.FS

// New dựng handler của app. Cookie phiên mang tên riêng của Vidverse: trình
// duyệt dùng chung cookie giữa các cổng của cùng một host, trùng tên với app
// khác thì đăng nhập app này sẽ đá phiên app kia.
func New(accounts *identity.Store) *webapp.Server {
	dist, err := fs.Sub(ui, "dist")
	if err != nil {
		// embed đảm bảo dist tồn tại khi build; lỗi ở đây là binary hỏng.
		panic(err)
	}
	return webapp.New(accounts, webapp.Options{
		AppName:    "Vidverse",
		CookieName: "vidverse_session",
		UI:         dist,
	})
}
