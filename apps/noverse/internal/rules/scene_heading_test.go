package rules

import (
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// sceneHeadingViolation 取第一条 scene_heading 违规（target 是原行，不固定）。
func sceneHeadingViolation(vs []Violation) *Violation {
	for i := range vs {
		if vs[i].Rule == "scene_heading" {
			return &vs[i]
		}
	}
	return nil
}

// TestAdmissionRejectsSceneHeading 锁死硬规则本身：独立成行的剧本场景标题仍是
// 准入错误（产品是散文）。draft/edit 会先机械删行，所以这里只兜底。
func TestAdmissionRejectsSceneHeading(t *testing.T) {
	cases := map[string]string{
		"interior heading":          "NỘI. PHÒNG TRỰC HẠ TẦNG – 02:00\n\nMàn hình phủ đỏ.",
		"combined int ext heading":  "NỘI./NGOẠI. KHU HÀNH CHÍNH – BÃI XE – SAU ĐÓ\n\nÔng ký nhận.",
		"bold scene number":         "**CẢNH 7 – MONTAGE – TỪ SAU CUỘC HỌP**\n\nCô mở sơ đồ.",
		"hash scene number":         "## CẢNH 9\n\nCửa cabin khép lại.",
		"bare interior marker line": "EXT. BÃI XE – CHIỀU\n\nMưa bắt đầu rơi.",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			v := sceneHeadingViolation(Admission(text, Draft))
			if v == nil {
				t.Fatalf("Admission phải chặn scene_heading: %+v", Admission(text, Draft))
			}
			if v.Severity != SeverityError {
				t.Fatalf("scene_heading phải là error, nhận %v", v.Severity)
			}
		})
	}
	for _, prose := range []string{
		"Cảnh 3 của vở kịch diễn ra trong im lặng.",
		"CẢNH 3 của vở kịch khiến cô bật khóc.",
		"TỔN THẤT CẢNH 1: niềm tin của Trần suy giảm.",
		"CẢNH TƯỢNG ấy làm cô sững người.",
	} {
		if v := sceneHeadingViolation(Admission(prose, Final)); v != nil {
			t.Fatalf("văn kể %q không phải tiêu đề: %+v", prose, v)
		}
	}
}

// TestStripStandaloneSceneHeadings 锁死"先删再说"的机械清理：整行是标题才删，
// 夹在句子里的 "CẢNH 9: …" 属于叙述，必须原样保留。
func TestStripStandaloneSceneHeadings(t *testing.T) {
	text := "# Chương 8\n\n**CẢNH 7 – MONTAGE – TỪ SAU CUỘC HỌP**\n\nTrong phòng dự án AI, Tuyết Nhi mở sơ đồ.\n\nCẢNH 9\n\nCô đọc lại cảnh 9 của vở kịch.\n\n## NỘI. PHÒNG DEMO\n\nTiếng vỗ tay vang lên."
	cleaned, removed := StripStandaloneSceneHeadings(text)
	want := []string{"CẢNH 7 – MONTAGE – TỪ SAU CUỘC HỌP", "CẢNH 9", "NỘI. PHÒNG DEMO"}
	if len(removed) != len(want) {
		t.Fatalf("muốn bỏ %v, nhận %v", want, removed)
	}
	for i := range want {
		if removed[i] != want[i] {
			t.Fatalf("dòng bị bỏ thứ %d = %q, muốn %q", i, removed[i], want[i])
		}
	}
	for _, gone := range want {
		if strings.Contains(cleaned, gone) {
			t.Fatalf("nội dung còn sót tiêu đề %q: %q", gone, cleaned)
		}
	}
	for _, keep := range []string{"# Chương 8", "Trong phòng dự án AI, Tuyết Nhi mở sơ đồ.", "Cô đọc lại cảnh 9 của vở kịch.", "Tiếng vỗ tay vang lên."} {
		if !strings.Contains(cleaned, keep) {
			t.Fatalf("mất nội dung %q: %q", keep, cleaned)
		}
	}
	if strings.Contains(cleaned, "\n\n\n") {
		t.Fatalf("dòng bị bỏ không được để lại khoảng trắng ba dòng: %q", cleaned)
	}
}

// TestSanitizeSceneHeadingText 锁死大纲注入前的改写：标题变"Bối cảnh: …"普通句，
// 地点与时间留住；整段只有标记时删掉；全小写的叙述不动。
func TestSanitizeSceneHeadingText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "embedded heading keeps setting and time",
			in:   "CẢNH 1 — NỘI. PHÒNG TRỰC HẠ TẦNG — 02:00 ĐÊM TRƯỚC BUỔI HỌP. Server sập; Lâm sửa trong 10 phút.",
			want: "Bối cảnh: phòng trực hạ tầng, 02:00 đêm trước buổi họp. Server sập; Lâm sửa trong 10 phút.",
		},
		{
			name: "colon separated heading capitalizes prose",
			in:   "CẢNH 8 – NỘI. PHÒNG DEMO TẦNG 30 – THỨ SÁU: giao dịch thử thành công.",
			want: "Bối cảnh: phòng demo tầng 30, thứ sáu. Giao dịch thử thành công.",
		},
		{
			name: "bare scene number is dropped",
			in:   "CẢNH 9",
			want: "",
		},
		{
			name: "direction marker before prose is dropped",
			in:   "Mở đầu. NỘI./NGOẠI. Sau đó ông ký.",
			want: "Mở đầu. Sau đó ông ký.",
		},
		{
			name: "lowercase scene reference untouched",
			in:   "Cảnh 3 của vở kịch diễn ra trong im lặng.",
			want: "Cảnh 3 của vở kịch diễn ra trong im lặng.",
		},
		{
			name: "uppercase scene reference in prose untouched",
			in:   "CẢNH 3 của vở kịch khiến cô bật khóc.",
			want: "CẢNH 3 của vở kịch khiến cô bật khóc.",
		},
		{
			name: "loss label with scene number untouched",
			in:   "TỔN THẤT CẢNH 1: niềm tin của Trần suy giảm.",
			want: "TỔN THẤT CẢNH 1: niềm tin của Trần suy giảm.",
		},
		{
			name: "plain prose untouched",
			in:   "Tuyết Nhi phát hiện code lạ, Hàn thúc tiến độ.",
			want: "Tuyết Nhi phát hiện code lạ, Hàn thúc tiến độ.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeSceneHeadingText(tc.in); got != tc.want {
				t.Fatalf("SanitizeSceneHeadingText(%q)\n nhận %q\n muốn %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSanitizeOutlineEntryKeepsStoreShape 锁死"只净化副本"：返回的条目被改写，
// 输入条目（store 里的大纲）保持原样。
func TestSanitizeOutlineEntryKeepsStoreShape(t *testing.T) {
	const original = "CẢNH 1 — NỘI. PHÒNG TRỰC HẠ TẦNG — 02:00. Server sập; Lâm sửa."
	entry := domain.OutlineEntry{
		Chapter:   1,
		Title:     "Ca trực hai giờ",
		CoreEvent: original,
		Scenes:    []string{"NỘI. PHÒNG TRỰC", "Trần nhắc cuộc họp sáng mai"},
	}
	clean := SanitizeOutlineEntry(entry)
	if strings.Contains(clean.CoreEvent, "CẢNH 1") || strings.Contains(clean.CoreEvent, "NỘI.") {
		t.Fatalf("core_event còn tiêu đề kịch bản: %q", clean.CoreEvent)
	}
	if !strings.Contains(clean.CoreEvent, "Bối cảnh: phòng trực hạ tầng, 02:00.") {
		t.Fatalf("core_event mất bối cảnh/thời gian: %q", clean.CoreEvent)
	}
	if !strings.Contains(clean.CoreEvent, "Server sập; Lâm sửa.") {
		t.Fatalf("core_event mất sự kiện: %q", clean.CoreEvent)
	}
	if clean.Scenes[0] != "Bối cảnh: phòng trực." {
		t.Fatalf("cảnh chỉ có marker phải giữ bối cảnh dạng câu: %q", clean.Scenes[0])
	}
	if clean.Scenes[1] != "Trần nhắc cuộc họp sáng mai" {
		t.Fatalf("cảnh văn xuôi bị đổi: %q", clean.Scenes[1])
	}
	if entry.CoreEvent != original {
		t.Fatalf("không được sửa entry gốc: %q", entry.CoreEvent)
	}
}
