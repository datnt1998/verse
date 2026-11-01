package web

import (
	"net/http"
	"strings"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// ChannelHandler xử lý một route dạng /api/channels/<id>/<tail>; rest là các
// đoạn đường dẫn khớp ký tự "*" trong mẫu.
type ChannelHandler func(w http.ResponseWriter, r *http.Request, channelID string, rest []string)

type channelRoute struct {
	method  string
	pattern []string
	h       ChannelHandler
}

// HandleChannel đăng ký route /api/channels/<id>/<tail>. tail là các đoạn cách
// nhau bởi "/", đoạn "*" khớp mọi giá trị không rỗng (ví dụ "insights/*/apply").
// Mọi owner dùng chung một bộ định tuyến để route prefix không che nhau.
func (g *Gateway) HandleChannel(method, tail string, h ChannelHandler) {
	if g.channelRoutes == nil {
		g.channelRoutes = map[string][]channelRoute{}
	}
	if _, ok := g.channelRoutes[method]; !ok {
		routes := &g.channelRoutes
		g.Handle(method, "/api/channels/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
			rest := strings.TrimPrefix(r.URL.Path, "/api/channels/")
			id, tail, ok := strings.Cut(rest, "/")
			if !ok || id == "" || !cloneSegment(id) {
				webapp.Error(w, http.StatusNotFound, "không tìm thấy")
				return
			}
			segments := strings.Split(tail, "/")
			for _, route := range (*routes)[method] {
				if captured, match := matchChannelRoute(route.pattern, segments); match {
					route.h(w, r, id, captured)
					return
				}
			}
			webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		})
	}
	g.channelRoutes[method] = append(g.channelRoutes[method], channelRoute{method: method, pattern: strings.Split(tail, "/"), h: h})
}

func matchChannelRoute(pattern, segments []string) ([]string, bool) {
	if len(pattern) != len(segments) {
		return nil, false
	}
	captured := []string{}
	for i, p := range pattern {
		switch {
		case segments[i] == "":
			return nil, false
		case p == "*":
			captured = append(captured, segments[i])
		case p != segments[i]:
			return nil, false
		}
	}
	return captured, true
}
