package web

import (
	"encoding/json"
	"testing"
)

// E2EWORK-9: an unadopted folder is reported as such to readers allowed in it, never to others.
func TestUnadoptedProjectNotFoundIsActionable(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	writer := tg.login("writer", "writer-pass")
	for _, tc := range []struct {
		name, cookie, path string
		status             int
		candidate          bool
		message            string
	}{
		{"admin stories", admin, "/api/projects/cand/stories", 404, true, "Thư mục này chưa được nhận vào thư viện"},
		{"admin meta", admin, "/api/projects/cand", 404, true, "Thư mục này chưa được nhận vào thư viện"},
		{"writer without access", writer, "/api/projects/cand/stories", 403, false, "Bạn không có quyền vào dự án này."},
		{"missing folder", admin, "/api/projects/khong-co/stories", 404, false, "không tìm thấy dự án"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := tg.do("GET", tc.path, tc.cookie, "")
			var body struct {
				Error     string `json:"error"`
				Candidate bool   `json:"candidate"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || body.Candidate != tc.candidate || body.Error != tc.message {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
}
