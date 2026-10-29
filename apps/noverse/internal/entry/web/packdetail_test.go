package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
)

func packByName(t *testing.T, name string) *assets.Pack {
	t.Helper()
	for _, p := range assets.ListPacks(assets.DefaultLoadOptions("")) {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("không thấy gói %q", name)
	return nil
}

// TestRuleCountsSkipsLegend: dòng định nghĩa nhãn không tính là mục luật.
func TestRuleCountsSkipsLegend(t *testing.T) {
	text := "## 0. Cách dùng\n\n  - **[LỖI]:** bắt buộc.\n  - **[CẢNH BÁO]:** sửa hoặc ghi lý do.\n\n" +
		"## 1. Mục\n\n- **[LỖI] một mục bắt sửa\n- **[CẢNH BÁO]** một mục cảnh báo\n- **[LỖI]** mục thứ hai\n"
	if errs, wrns := ruleCounts(text); errs != 2 || wrns != 1 {
		t.Fatalf("ruleCounts = %d/%d, muốn 2/1 (bỏ hai dòng chú giải)", errs, wrns)
	}
	sections := ruleSections(text, "Master")
	if len(sections) != 2 {
		t.Fatalf("sections = %+v", sections)
	}
	if sections[0].Ref != "Master §0" || sections[0].Level != "" {
		t.Errorf("§0 = %+v, muốn không mức nặng (chỉ có chú giải)", sections[0])
	}
	if sections[1].Level != "error" {
		t.Errorf("§1 level = %q, muốn error", sections[1].Level)
	}
}

// TestPackDetailLayersAndSections: số mục bắt sửa / cảnh báo đếm từ tệp luật thật
// của gói Nữ tần báo thù, và danh sách § của lớp Master.
func TestPackDetailLayersAndSections(t *testing.T) {
	detail := buildPackDetail(packByName(t, "nu-tan-bao-thu"))
	if !detail.HasRules || !detail.Available {
		t.Fatalf("hasRules/available = %v/%v, muốn true/true", detail.HasRules, detail.Available)
	}
	if detail.Desc != "Trọng sinh, vả mặt, lấy lại từng thứ trước đám đông." {
		t.Errorf("desc = %q, muốn mo_ta của goi.json", detail.Desc)
	}
	if len(detail.Layers) != 3 {
		t.Fatalf("layers = %+v", detail.Layers)
	}
	want := []packRuleLayer{
		{Key: "master", Title: "Luật chung · Master", Desc: "22 mục cho mọi truyện audio giọng AI tiếng Việt", Errors: 43, Warnings: 50},
		{Key: "pack", Title: "Luật thể loại · Bộ A", Desc: "Luật riêng của thể loại này. Không ghi đè mục nào của Master.", Errors: 22, Warnings: 11},
		{Key: "measure", Title: "Cách đo · Định dạng", Desc: "Cách đo thoại, câu, đoạn khi chấm tự động.", Errors: 0, Warnings: 0},
	}
	for i, w := range want {
		if detail.Layers[i] != w {
			t.Errorf("layer %d = %+v, muốn %+v", i, detail.Layers[i], w)
		}
	}
	if len(detail.Sections) != 23 {
		t.Fatalf("sections = %d, muốn 23 §", len(detail.Sections))
	}
	byRef := map[string]packRuleSection{}
	for _, s := range detail.Sections {
		byRef[s.Ref] = s
	}
	if s := byRef["Master §7"]; s.Title != "Đầu chương và cuối chương" || s.Level != "error" || s.Errors != 3 || s.Warnings != 1 {
		t.Errorf("§7 = %+v, muốn error 3/1", s)
	}
	if s := byRef["Master §9"]; s.Title != "Cú trả thưởng" || s.Level != "warning" || s.Errors != 0 || s.Warnings != 3 {
		t.Errorf("§9 = %+v, muốn warning 0/3", s)
	}
	if s := byRef["Master §20"]; s.Level != "" || s.Errors != 0 || s.Warnings != 0 {
		t.Errorf("§20 = %+v, muốn không mức nặng, 0/0", s)
	}
}

// TestPackDetailMetrics: chín chip ngưỡng số đo của gói (danh sách "gói dùng
// chuẩn nào"), tách khỏi `params` — hàng thông số của artboard.
func TestPackDetailMetrics(t *testing.T) {
	rows := buildPackDetail(packByName(t, "nu-tan-bao-thu")).Metrics
	if len(rows) != 9 {
		t.Fatalf("metrics = %+v", rows)
	}
	want := []packMeasureRow{
		{Key: "thoai", Name: "Tỷ lệ thoại", Limit: "40–55%"},
		{Key: "cau_dai_nhat", Name: "Câu dài nhất", Limit: "≤ 40"},
		{Key: "doan_dai_nhat", Name: "Đoạn dài nhất", Limit: "≤ 90"},
		{Key: "quang_khong_thoai", Name: "Quãng dài không thoại", Limit: "≤ 250"},
		{Key: "tu_thu_tuc", Name: "Từ thủ tục", Limit: "≤ 10"},
		{Key: "tu_tieng_anh", Name: "Từ tiếng Anh dễ đọc sai", Limit: "0"},
		{Key: "cum_lap", Name: "Cụm 5 tiếng lặp lại", Limit: "0"},
		{Key: "trung_mo_dau", Name: "Trùng mở đầu truyện cũ", Limit: "≤ 20%"},
		{Key: "tu_lap", Name: "Từ lặp", Limit: "≤ 0,67 / 1.000 tiếng"},
	}
	for i, w := range want {
		if rows[i] != w {
			t.Errorf("row %d = %+v, muốn %+v", i, rows[i], w)
		}
	}
}

// TestPackDetailOverridesAndVoice: ghi đè văn phong đếm từ goi.json (hiện mọi gói
// để null nên 0 mục); giọng đọc lấy từ khối audio, mặc định suy từ nhóm người nghe.
func TestPackDetailOverridesAndVoice(t *testing.T) {
	for _, tc := range []struct {
		pack  string
		voice string
	}{
		{"nu-tan-bao-thu", "Một giọng nữ kể cả truyện"},
		{"nguoc-hon-nhan", "Một giọng nữ kể cả truyện"},
		{"nien-dai-van", "Một giọng kể cả truyện"},
	} {
		detail := buildPackDetail(packByName(t, tc.pack))
		if detail.Overrides.Count != 0 || len(detail.Overrides.Keys) != 0 || len(detail.Overrides.Rows) != 0 {
			t.Errorf("%s overrides = %+v, muốn 0 mục (goi.json để null)", tc.pack, detail.Overrides)
		}
		if detail.Voice != tc.voice {
			t.Errorf("%s voice = %q, muốn %q", tc.pack, detail.Voice, tc.voice)
		}
	}
}

// TestPackDetailParams: hàng thông số của chi tiết gói là sáu chip chữ cố định
// soạn từ `do_do`/`thoai`, đúng thứ tự artboard (Câu 7 bullet 4).
func TestPackDetailParams(t *testing.T) {
	want := []packParam{
		{Key: "chapters", Text: "8 chương"},
		{Key: "dialogue", Text: "Thoại 40–55%"},
		{Key: "sentence", Text: "Câu ≤ 40 tiếng · đoạn ≤ 90 tiếng"},
		{Key: "quiet", Text: "Quãng không thoại ≤ 250 tiếng"},
		{Key: "procedural", Text: "Từ thủ tục ≤ 10 mỗi chương"},
		{Key: "opening", Text: "220 tiếng mở đầu trùng truyện cũ ≤ 20%"},
	}
	got := buildPackDetail(packByName(t, "nu-tan-bao-thu")).Params
	if len(got) != len(want) {
		t.Fatalf("params = %+v", got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("param %d = %+v, muốn %+v", i, got[i], w)
		}
	}
	// Khoảng thoại theo gói, không viết cứng.
	if params := buildPackDetail(packByName(t, "nien-dai-van")).Params; len(params) < 2 || params[1].Text != "Thoại 30–45%" {
		t.Fatalf("params gói C = %+v", params)
	}
}

// TestPackShortAndMeta: tên ngắn lấy từ `ten_ngan`, dòng phụ là số chương +
// khoảng thoại (Câu 7, Câu 12).
func TestPackShortAndMeta(t *testing.T) {
	for _, tc := range []struct {
		pack  string
		short string
		meta  string
	}{
		{"nu-tan-bao-thu", "Nữ tần báo thù", "8 chương · thoại 40–55%"},
		{"nguoc-hon-nhan", "Ngược hôn nhân", "7 chương · thoại 35–50%"},
		{"nien-dai-van", "Niên đại văn", "10 chương · thoại 30–45%"},
	} {
		pack := packByName(t, tc.pack)
		if got := packShort(pack); got != tc.short {
			t.Errorf("%s short = %q, muốn %q", tc.pack, got, tc.short)
		}
		if got := packMeta(pack); got != tc.meta {
			t.Errorf("%s meta = %q, muốn %q", tc.pack, got, tc.meta)
		}
		if detail := buildPackDetail(pack); !detail.Available || !detail.HasRules {
			t.Errorf("%s phải có luật chấm và chọn được: %+v", tc.pack, detail)
		}
	}
}

// TestPackShortFallsBackToTen: gói cũ chưa có `ten_ngan` thì bỏ tiền tố "Gói "
// và phần sau dấu phẩy (Câu 12).
func TestPackShortFallsBackToTen(t *testing.T) {
	for _, tc := range []struct{ ten, want string }{
		{"Gói Mạt thế, sinh tồn", "Mạt thế"},
		{"Cung đấu, tranh sủng", "Cung đấu"},
		{"Niên đại văn", "Niên đại văn"},
	} {
		if got := packShort(&assets.Pack{Ten: tc.ten}); got != tc.want {
			t.Errorf("packShort(%q) = %q, muốn %q", tc.ten, got, tc.want)
		}
	}
}

// TestPackVoiceOverridesRows: mỗi dòng chỉ ghi giá trị mới (Câu 6); mục để null
// là tắt chứ không phải ghi đè nên không tính.
func TestPackVoiceOverridesRows(t *testing.T) {
	value := "Câu ngắn, nhiều thoại"
	got := packVoiceOverrides(&assets.Pack{Voice: map[string]*string{"hoi-hop": &value, "do-dai-doan": nil}})
	want := packOverrides{Count: 1, Keys: []string{"hoi-hop"}, Rows: []packOverrideRow{{Key: "hoi-hop", Value: value}}}
	if got.Count != want.Count || len(got.Keys) != 1 || got.Keys[0] != want.Keys[0] || len(got.Rows) != 1 || got.Rows[0] != want.Rows[0] {
		t.Fatalf("overrides = %+v, muốn %+v", got, want)
	}
}

// TestPackDetailAudio: `audio` trả nguyên khối của goi.json (D11), không suy
// `giong` từ nhóm người nghe như `voice` — giao diện cần biết gói KHAI gì.
func TestPackDetailAudio(t *testing.T) {
	for _, tc := range []struct {
		pack string
		want packAudio
	}{
		{"nu-tan-bao-thu", packAudio{Mode: "single", Gender: "nu"}},
		{"nguoc-hon-nhan", packAudio{Mode: "single"}},
		{"nien-dai-van", packAudio{Mode: "single"}},
	} {
		if got := buildPackDetail(packByName(t, tc.pack)).Audio; got != tc.want {
			t.Errorf("%s audio = %+v, muốn %+v", tc.pack, got, tc.want)
		}
	}
}

// TestPresetsPacksAudioJSON: khoá trên dây là `audio.che_do` / `audio.giong`
// (FeAudio3 đọc thẳng), và gói tự thêm khai `cast`/`nam` được trả nguyên.
func TestPresetsPacksAudioJSON(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	packDir := filepath.Join(tg.home, ".ainovel", "style", "packs", "goi-audio")
	if err := os.MkdirAll(filepath.Join(packDir, "luat"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "goi.json"),
		[]byte(`{"ten":"Gói audio","so_chuong":9,"nguoi_nghe":"Nam trẻ","audio":{"che_do":"cast","giong":"nam"},"luat":["a.md"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "luat", "a.md"), []byte("luật"), 0o644); err != nil {
		t.Fatal(err)
	}

	packs := decodeResp(t, tg.do("GET", "/api/config/presets", admin, ""))["packs"].([]any)
	audioOf := func(id string) map[string]any {
		t.Helper()
		for _, item := range packs {
			pack := item.(map[string]any)
			if pack["id"] == id {
				audio, ok := pack["audio"].(map[string]any)
				if !ok {
					t.Fatalf("%s thiếu khối audio: %v", id, pack)
				}
				return audio
			}
		}
		t.Fatalf("thiếu gói %q trong packs: %v", id, packs)
		return nil
	}

	if got := audioOf("goi-audio"); got["che_do"] != "cast" || got["giong"] != "nam" {
		t.Fatalf("goi-audio audio = %v", got)
	}
	// Câu 7/Câu 12: thẻ gói đọc tên ngắn và dòng phụ từ goi.json.
	for _, item := range packs {
		pack := item.(map[string]any)
		if pack["id"] != "nu-tan-bao-thu" {
			continue
		}
		if pack["short"] != "Nữ tần báo thù" || pack["meta"] != "8 chương · thoại 40–55%" {
			t.Fatalf("nu-tan-bao-thu short/meta = %v / %v", pack["short"], pack["meta"])
		}
		if pack["desc"] != "Trọng sinh, vả mặt, lấy lại từng thứ trước đám đông." {
			t.Fatalf("nu-tan-bao-thu desc = %v", pack["desc"])
		}
		if pack["available"] != true || pack["hasRules"] != true {
			t.Fatalf("nu-tan-bao-thu available/hasRules = %v / %v", pack["available"], pack["hasRules"])
		}
	}
	if got := audioOf("nu-tan-bao-thu"); got["che_do"] != "single" || got["giong"] != "nu" {
		t.Fatalf("nu-tan-bao-thu audio = %v", got)
	}
	// Gói không khai `giong`: để rỗng, không suy từ nhóm người nghe.
	if got := audioOf("nguoc-hon-nhan"); got["che_do"] != "single" || got["giong"] != "" {
		t.Fatalf("nguoc-hon-nhan audio = %v", got)
	}
}

// TestPresetsPacksChapterWordsJSON: Câu 1 — `packs[].chapterWords` là
// {min,max} đọc từ goi.json `tieng_moi_chuong`, và là null tường minh (khoá vẫn
// có mặt) với gói không khai độ dài chương.
func TestPresetsPacksChapterWordsJSON(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")

	// Gói tự thêm ở tầng home, không khai `tieng_moi_chuong`.
	packDir := filepath.Join(tg.home, ".ainovel", "style", "packs", "goi-thieu-do-dai")
	if err := os.MkdirAll(filepath.Join(packDir, "luat"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "goi.json"),
		[]byte(`{"ten":"Gói thiếu độ dài","so_chuong":5,"luat":["a.md"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "luat", "a.md"), []byte("luật"), 0o644); err != nil {
		t.Fatal(err)
	}

	packs := decodeResp(t, tg.do("GET", "/api/config/presets", admin, ""))["packs"].([]any)
	packByID := func(id string) map[string]any {
		t.Helper()
		for _, item := range packs {
			pack := item.(map[string]any)
			if pack["id"] == id {
				return pack
			}
		}
		t.Fatalf("thiếu gói %q trong packs: %v", id, packs)
		return nil
	}

	for _, id := range []string{"nu-tan-bao-thu", "nguoc-hon-nhan", "nien-dai-van"} {
		words, ok := packByID(id)["chapterWords"].(map[string]any)
		if !ok {
			t.Fatalf("%s: chapterWords phải là object, nhận được %v", id, packByID(id)["chapterWords"])
		}
		if words["min"] != float64(1400) || words["max"] != float64(1600) {
			t.Fatalf("%s: chapterWords = %v, muốn 1400–1600", id, words)
		}
	}

	// Khoá phải CÓ MẶT với giá trị null: giao diện phân biệt "gói không khai"
	// với "chuỗi rỗng" mà không cần đoán.
	missing := packByID("goi-thieu-do-dai")
	words, ok := missing["chapterWords"]
	if !ok {
		t.Fatalf("goi-thieu-do-dai: thiếu khoá chapterWords: %v", missing)
	}
	if words != nil {
		t.Fatalf("goi-thieu-do-dai: chapterWords phải là null, nhận được %v", words)
	}
}
