package web

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
)

// GAP-40: bản đọc TTS xuất được qua API, có readNumbers, và nằm ở cột riêng
// "Bản đọc TTS" của ma trận Bản xuất (không lẫn vào cột Văn bản).
func TestExportNarrationColumn(t *testing.T) {
	g := &Gateway{}
	defer g.jobsHub().close()
	st, _ := storyFixture(t, []int{1, 2})

	post := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		return storyReq(t, g, "POST", "/x/exports", adminUser(), st, "/exports", body)
	}
	list := func() struct {
		Matrix []ExportRowAPI `json:"matrix"`
	} {
		t.Helper()
		rec := storyReq(t, g, "GET", "/x/exports", adminUser(), st, "/exports", "")
		if rec.Code != 200 {
			t.Fatalf("list: %d %s", rec.Code, rec.Body)
		}
		var v struct {
			Matrix []ExportRowAPI `json:"matrix"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}

	txt := post(`{"format":"txt","from":1,"to":2}`)
	if txt.Code != 200 {
		t.Fatalf("txt: %d %s", txt.Code, txt.Body)
	}
	var txtRes struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(txt.Body.Bytes(), &txtRes); err != nil {
		t.Fatal(err)
	}

	nar := post(`{"format":"narration","from":1,"to":2,"readNumbers":true}`)
	if nar.Code != 200 {
		t.Fatalf("narration: %d %s", nar.Code, nar.Body)
	}
	var narRes struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(nar.Body.Bytes(), &narRes); err != nil {
		t.Fatal(err)
	}
	// Bản đọc TTS phải khác tên với bản Văn bản, nếu không hai bản ghi đè nhau.
	if !isNarrationExport(narRes.Path) || narRes.Path == txtRes.Path {
		t.Fatalf("narration path = %q, txt path = %q", narRes.Path, txtRes.Path)
	}

	rows := list().Matrix
	if len(rows) != 1 {
		t.Fatalf("matrix rows = %d", len(rows))
	}
	row := rows[0]
	if row.Narration == nil || row.Narration.Path != narRes.Path {
		t.Fatalf("narration column = %+v (want %q)", row.Narration, narRes.Path)
	}
	if row.TXT == nil || row.TXT.Path != txtRes.Path {
		t.Fatalf("txt column = %+v (want %q)", row.TXT, txtRes.Path)
	}
	// Danh sách tệp không lộ đường dẫn tuyệt đối.
	if strings.Contains(nar.Body.String(), st.Dir) {
		t.Fatalf("narration result leaks a host path: %s", nar.Body)
	}
}

// GAP-40: tự xuất nhận bản đọc TTS trong formats[] (D3: “Bản đọc TTS xuất khi
// bạn cần” — không bật mặc định, nhưng cấu hình phải chấp nhận).
func TestAutoExportFormatsAcceptNarration(t *testing.T) {
	cfg := library.DefaultConfig()
	cfg.AutoExport.Formats = []string{"txt", "epub", "narration"}
	if msg := validateLibraryConfig(cfg); msg != "" {
		t.Fatalf("autoexport with narration rejected: %s", msg)
	}
	cfg.AutoExport.Formats = []string{"pdf"}
	if msg := validateLibraryConfig(cfg); !strings.Contains(msg, "pdf") {
		t.Fatalf("unknown autoexport format accepted: %q", msg)
	}
}
