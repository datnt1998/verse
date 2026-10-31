package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateProjectAutoSlugAndDuplicateName(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	// Không gửi slug: server tự sinh từ tên hiển thị.
	w := tg.do("POST", "/api/projects", admin, `{"name":"Kênh truyện audio"}`)
	if w.Code != 201 {
		t.Fatalf("create without slug -> %d %s", w.Code, w.Body.String())
	}
	p := decodeJSONBody(t, w)
	if p["id"] != "kenh-truyen-audio" || p["name"] != "Kênh truyện audio" {
		t.Fatalf("project: %v", p)
	}
	if _, err := os.Stat(filepath.Join(tg.root, "kenh-truyen-audio", "project.json")); err != nil {
		t.Fatalf("project.json: %v", err)
	}

	// Trùng tên (không phân biệt hoa thường) → 409 bằng lời thường, không có thư mục mới.
	w = tg.do("POST", "/api/projects", admin, `{"name":"  KÊNH truyện AUDIO "}`)
	if w.Code != 409 {
		t.Fatalf("duplicate name -> %d %s", w.Code, w.Body.String())
	}
	if msg := decodeJSONBody(t, w)["error"]; msg != "Đã có dự án cùng tên — đặt tên khác nhé" {
		t.Fatalf("duplicate message: %v", msg)
	}

	// Tên khác nhưng cùng slug (bỏ dấu) → thêm hậu tố, không báo lỗi.
	w = tg.do("POST", "/api/projects", admin, `{"name":"Kenh truyen audio!"}`)
	if w.Code != 201 {
		t.Fatalf("same slug other name -> %d %s", w.Code, w.Body.String())
	}
	if id := decodeJSONBody(t, w)["id"]; id != "kenh-truyen-audio-2" {
		t.Fatalf("suffix id: %v", id)
	}

	// Tên trùng tên dự án có sẵn từ project.json (không phải slug).
	if code := tg.do("POST", "/api/projects", admin, `{"slug":"khac","name":"Kênh truyện audio"}`).Code; code != 409 {
		t.Fatalf("explicit slug but duplicate name -> %d", code)
	}

	// Thiếu cả tên lẫn slug → 400 tiếng Việt.
	w = tg.do("POST", "/api/projects", admin, `{"name":"   "}`)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Nhập tên dự án") {
		t.Fatalf("empty -> %d %s", w.Code, w.Body.String())
	}

	// Tên chỉ có ký tự lạ → slug mặc định hợp lệ, không 500.
	if code := tg.do("POST", "/api/projects", admin, `{"name":"output"}`).Code; code != 201 {
		t.Fatalf("reserved-looking name -> %d", code)
	}
}

func TestCreateProjectShareGlobal(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	home, _ := os.UserHomeDir()
	mustWriteFile := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWriteFile(filepath.Join(home, ".ainovel", "config.json"),
		`{"roles":{"writer":{"provider":"p","model":"m1"}},"style":"kịch tính"}`)
	mustWriteFile(filepath.Join(home, ".ainovel", "rules", "quy-tac.md"), "- Không dùng từ cổ\n")
	mustWriteFile(filepath.Join(home, ".ainovel", "rules", "van-phong.md"), "Câu ngắn.\n")
	mustWriteFile(filepath.Join(home, ".ainovel", "rules", "README.txt"), "không phải quy tắc\n")

	// Shared model configuration must not reintroduce retired global rules.
	for _, body := range []string{`{"name":"Dùng chung"}`, `{"name":"Dùng chung rõ","shareGlobal":true}`} {
		w := tg.do("POST", "/api/projects", admin, body)
		if w.Code != 201 {
			t.Fatalf("create %s -> %d %s", body, w.Code, w.Body.String())
		}
		dir := filepath.Join(tg.root, decodeJSONBody(t, w)["id"].(string))
		if entries, _ := os.ReadDir(filepath.Join(dir, ".ainovel", "rules")); len(entries) != 0 {
			t.Fatalf("new projects must not import retired rules: %v", entries)
		}
		if cfg, err := os.ReadFile(filepath.Join(dir, ".ainovel", "config.json")); err == nil && strings.Contains(string(cfg), "roles") {
			t.Fatalf("shared project must not own roles: %s", cfg)
		}
	}

	// An unshared model configuration snapshots roles, never global style/rules.
	w := tg.do("POST", "/api/projects", admin, `{"name":"Riêng","shareGlobal":false}`)
	if w.Code != 201 {
		t.Fatalf("create unshared -> %d %s", w.Code, w.Body.String())
	}
	dir := filepath.Join(tg.root, decodeJSONBody(t, w)["id"].(string))
	if entries, _ := os.ReadDir(filepath.Join(dir, ".ainovel", "rules")); len(entries) != 0 {
		t.Fatalf("unshared project must start with no rules, got %d files", len(entries))
	}
	cfg, err := os.ReadFile(filepath.Join(dir, ".ainovel", "config.json"))
	if err != nil || !strings.Contains(string(cfg), `"model": "m1"`) && !strings.Contains(string(cfg), `"model":"m1"`) || strings.Contains(string(cfg), "kịch tính") {
		t.Fatalf("unshared project config: %s %v", cfg, err)
	}
}
