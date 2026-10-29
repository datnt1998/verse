package assets

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// packSourceDir là thư mục PO giữ bản gốc của các file luật; gói nhúng chép
// từng byte từ đây (P0-3), test dưới bắt lệch khi PO cập nhật bản gốc.
const packSourceDir = "../docs/prd/rules"

// TestPackLoad_EmbeddedNuTanBaoThu: `style: nu-tan-bao-thu` phải nhận gói nhúng
// thay vì styles/<tên>.md, kèm đúng metadata khai trong goi.json.
func TestPackLoad_EmbeddedNuTanBaoThu(t *testing.T) {
	b := Load("nu-tan-bao-thu", LoadOptions{})
	if b.Pack == nil {
		t.Fatal("style nu-tan-bao-thu phải nhận gói nhúng")
	}
	if b.Pack.Ten == "" || b.Pack.SoChuong != 8 || b.Pack.NguoiNghe == "" {
		t.Fatalf("goi.json thiếu metadata: %+v", b.Pack)
	}
	if b.Pack.Thoai == nil || b.Pack.Thoai.Min != 40 || b.Pack.Thoai.Max != 55 {
		t.Fatalf("thoai của gói A phải là 40–55 (bộ A §2), nhận được %+v", b.Pack.Thoai)
	}
	if b.Pack.TiengMoiChuong == nil || b.Pack.TiengMoiChuong.Min != 1400 || b.Pack.TiengMoiChuong.Max != 1600 {
		t.Fatalf("tieng_moi_chuong của gói A phải là 1400–1600 (bộ A dòng Độ dài), nhận được %+v", b.Pack.TiengMoiChuong)
	}
	if len(b.Pack.Luat) != 3 {
		t.Fatalf("gói A phải có 3 file luat, nhận được %d", len(b.Pack.Luat))
	}
	if b.Pack.Luat[0].Name != "master-rules-v3.md" {
		t.Fatalf("luat phải mở đầu bằng Master theo thứ tự goi.json, nhận được %q", b.Pack.Luat[0].Name)
	}
}

// TestPackEmbeddedLuat_ByteIdentical: mọi file luat/*.md của gói nhúng phải
// trùng từng byte với bản gốc trong docs/prd/rules — chống trôi bản vá.
func TestPackEmbeddedLuat_ByteIdentical(t *testing.T) {
	for _, name := range []string{"nu-tan-bao-thu", "nguoc-hon-nhan", "nien-dai-van"} {
		b := Load(name, LoadOptions{})
		if b.Pack == nil {
			t.Fatalf("%s: gói nhúng phải được nhận diện", name)
		}
		for _, luat := range b.Pack.Luat {
			embedded, err := packsFS.ReadFile("packs/" + name + "/luat/" + luat.Name)
			if err != nil {
				t.Fatalf("%s: đọc luat/%s nhúng: %v", name, luat.Name, err)
			}
			source, err := os.ReadFile(filepath.Join(packSourceDir, luat.Name))
			if err != nil {
				t.Fatalf("%s: luat/%s không có bản gốc trong %s: %v", name, luat.Name, packSourceDir, err)
			}
			if string(embedded) != string(source) {
				t.Fatalf("%s: luat/%s lệch bản gốc %s/%s — chép lại từng byte, không sửa tay", name, luat.Name, packSourceDir, luat.Name)
			}
		}
	}
}

// TestPackChapterWordsMatchRules: `tieng_moi_chuong` trong goi.json phải khớp
// dòng "Độ dài" của luật bộ trong chính gói ("mỗi chương 1.400–1.600 tiếng").
// PO sửa độ dài chương ở bản luật mà quên goi.json thì test này bắt.
func TestPackChapterWordsMatchRules(t *testing.T) {
	for _, name := range []string{"nu-tan-bao-thu", "nguoc-hon-nhan", "nien-dai-van"} {
		b := Load(name, LoadOptions{})
		if b.Pack == nil {
			t.Fatalf("%s: gói nhúng phải được nhận diện", name)
		}
		words := &PackTiengMoiChuong{}
		if ok, err := b.Pack.Section("tieng_moi_chuong", words); err != nil || !ok {
			t.Fatalf("%s: goi.json phải khai tieng_moi_chuong (ok=%v, err=%v)", name, ok, err)
		}
		min, max := rulesDocChapterWords(t, name, b.Pack)
		if words.Min != min || words.Max != max {
			t.Fatalf("%s: goi.json tieng_moi_chuong = %d–%d, luật ghi %d–%d",
				name, words.Min, words.Max, min, max)
		}
	}
}

// rulesDocChapterWords đọc khoảng tiếng/chương từ dòng "| Độ dài |" của luật
// gói. Số viết kiểu Việt Nam ("1.400") nên bỏ dấu chấm trước khi đổi ra int.
func rulesDocChapterWords(t *testing.T, name string, p *Pack) (int, int) {
	t.Helper()
	pattern := regexp.MustCompile(`mỗi chương\s+([\d.]+)–([\d.]+)\s*tiếng`)
	var min, max int
	found := false
	for _, luat := range p.Luat {
		for _, line := range strings.Split(luat.Content, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "| Độ dài |") {
				continue
			}
			m := pattern.FindStringSubmatch(line)
			if m == nil {
				t.Fatalf("%s: dòng \"Độ dài\" của luat/%s không có \"mỗi chương X–Y tiếng\": %q", name, luat.Name, line)
			}
			lo, err := strconv.Atoi(strings.ReplaceAll(m[1], ".", ""))
			if err != nil {
				t.Fatalf("%s: số tiếng đầu %q trong luat/%s không đọc được: %v", name, m[1], luat.Name, err)
			}
			hi, err := strconv.Atoi(strings.ReplaceAll(m[2], ".", ""))
			if err != nil {
				t.Fatalf("%s: số tiếng cuối %q trong luat/%s không đọc được: %v", name, m[2], luat.Name, err)
			}
			if found {
				t.Fatalf("%s: luật có hơn một dòng \"Độ dài\" (luat/%s)", name, luat.Name)
			}
			min, max, found = lo, hi, true
		}
	}
	if !found {
		t.Fatalf("%s: không tìm thấy dòng \"Độ dài\" trong luật của gói", name)
	}
	if min <= 0 || max < min {
		t.Fatalf("%s: khoảng tiếng/chương vô lý: %d–%d", name, min, max)
	}
	return min, max
}

// TestPackLayerOrder_BookOverridesHomeOverridesEmbedded: tầng sau thay nguyên
// thư mục gói của tầng trước (nhúng → home → book), không trộn từng file.
func TestPackLayerOrder_BookOverridesHomeOverridesEmbedded(t *testing.T) {
	home := t.TempDir()
	book := t.TempDir()

	writeTestPack(t, filepath.Join(home, "packs", "nu-tan-bao-thu"),
		`{"ten":"Bản home","luat":["home.md"]}`, map[string]string{"home.md": "luật tầng home"}, "")
	writeTestPack(t, filepath.Join(book, "packs", "nu-tan-bao-thu"),
		`{"ten":"Bản sách","luat":["book.md"]}`, map[string]string{"book.md": "luật tầng sách"}, "")

	b := Load("nu-tan-bao-thu", LoadOptions{HomeStyleDir: home, BookStyleDir: book})
	if b.Pack == nil || b.Pack.Ten != "Bản sách" {
		t.Fatalf("gói tầng sách phải thắng, nhận được %+v", b.Pack)
	}
	if len(b.Pack.Luat) != 1 || b.Pack.Luat[0].Content != "luật tầng sách" {
		t.Fatalf("ghi đè phải lấy nguyên thư mục sách, nhận được %+v", b.Pack.Luat)
	}

	// Chỉ có tầng home thì home thắng gói nhúng.
	b = Load("nu-tan-bao-thu", LoadOptions{HomeStyleDir: home})
	if b.Pack == nil || b.Pack.Ten != "Bản home" {
		t.Fatalf("gói tầng home phải thắng gói nhúng, nhận được %+v", b.Pack)
	}

	// Tầng home/sách định nghĩa một tên hoàn toàn mới cũng được nhận làm gói.
	writeTestPack(t, filepath.Join(book, "packs", "sach-rieng"),
		`{"ten":"Gói riêng","luat":["a.md"]}`, map[string]string{"a.md": "nội dung"}, "văn phong riêng")
	b = Load("sach-rieng", LoadOptions{BookStyleDir: book})
	if b.Pack == nil || b.Pack.VanPhong != "văn phong riêng" {
		t.Fatalf("gói ở tầng sách phải được nhận diện, nhận được %+v", b.Pack)
	}
}

// TestPackStyleFallback_NoPackDir: style không trùng thư mục gói nào thì hành vi
// giữ nguyên như cũ (đọc styles/<tên>.md).
func TestPackStyleFallback_NoPackDir(t *testing.T) {
	b := Load("default", LoadOptions{})
	if b.Pack != nil {
		t.Fatal("style default không có gói, Pack phải nil")
	}
	if b.Styles["default"] == "" {
		t.Fatal("styles/default.md phải vẫn nạp")
	}
}

// TestPackManifest_IgnoresUnknownFields: goi.json có trường lạ (lát sau thêm)
// phải được bỏ qua, không bị từ chối.
func TestPackManifest_IgnoresUnknownFields(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, filepath.Join(dir, "packs", "goi-tuong-lai"),
		`{"ten":"Tương lai","so_chuong":5,"truong_moi":{"a":1},"luat":["a.md"]}`,
		map[string]string{"a.md": "luật"}, "")
	b := Load("goi-tuong-lai", LoadOptions{HomeStyleDir: dir})
	if b.Pack == nil || b.Pack.SoChuong != 5 {
		t.Fatalf("goi.json có trường lạ vẫn phải nạp được, nhận được %+v", b.Pack)
	}
}

// TestPackVoiceOverride_RemovesSections (P0-6): gói A tắt `hoi-hop` và
// `do-dai-doan`, nên voice đã ráp không còn hai câu trái Master ở voice.md.
func TestPackVoiceOverride_RemovesSections(t *testing.T) {
	b := Load("nu-tan-bao-thu", LoadOptions{})
	for _, banned := range []string{
		"không nhất thiết mỗi chương đều phải làm hồi hộp",
		"tránh xé nhỏ văn bản thành các đoạn vụn",
	} {
		if strings.Contains(b.Voice, banned) {
			t.Fatalf("gói A phải tắt mục chứa %q", banned)
		}
	}
	if !strings.Contains(b.Voice, "Tiêu chuẩn viết tiểu thuyết") {
		t.Fatal("các mục voice không bị ghi đè phải giữ nguyên")
	}
}

// TestPackVoiceOverride_UnknownKeyFails: khoá voice không tồn tại là lỗi nạp —
// Bundle.Err mang thông báo nêu tên gói và khoá, Pack bị bỏ qua (không panic).
func TestPackVoiceOverride_UnknownKeyFails(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, filepath.Join(dir, "packs", "goi-loi"),
		`{"ten":"Lỗi","luat":["a.md"],"voice":{"khoa-lac":null}}`,
		map[string]string{"a.md": "luật"}, "")
	b := Load("goi-loi", LoadOptions{HomeStyleDir: dir})
	if b.Err == nil {
		t.Fatal("khoá voice lạ phải thành Bundle.Err")
	}
	if !strings.Contains(b.Err.Error(), "goi-loi") || !strings.Contains(b.Err.Error(), "khoa-lac") {
		t.Fatalf("thông báo lỗi phải nêu tên gói và khoá, nhận được: %v", b.Err)
	}
	if b.Pack != nil {
		t.Fatal("gói lỗi phải bị bỏ qua — Pack nil")
	}
}

// TestPackManifest_Error: goi.json hỏng cũng chảy qua Bundle.Err, không panic.
func TestPackManifest_Error(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packs", "goi-hong", "goi.json"), `{khoa`)
	b := Load("goi-hong", LoadOptions{HomeStyleDir: dir})
	if b.Err == nil || !strings.Contains(b.Err.Error(), "goi-hong") {
		t.Fatalf("goi.json hỏng phải thành Bundle.Err nêu tên gói, nhận được: %v", b.Err)
	}
}

// TestPackSection: khoá section của goi.json decode được; vắng trả false; JSON
// của khoá hỏng trả lỗi — các lát sau dùng Section() thay vì sửa pack.go.
func TestPackSection(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, filepath.Join(dir, "packs", "goi-sec"),
		`{"ten":"Sec","luat":["a.md"],"fatigue_words":{"muc":7}}`,
		map[string]string{"a.md": "luật"}, "")
	b := Load("goi-sec", LoadOptions{HomeStyleDir: dir})
	if b.Err != nil || b.Pack == nil {
		t.Fatalf("nạp gói lỗi: %v", b.Err)
	}
	var fw struct {
		Muc int `json:"muc"`
	}
	ok, err := b.Pack.Section("fatigue_words", &fw)
	if !ok || err != nil || fw.Muc != 7 {
		t.Fatalf("Section(fatigue_words) = %v, %v, %+v", ok, err, fw)
	}
	ok, err = b.Pack.Section("khong-co", &fw)
	if ok || err != nil {
		t.Fatal("khoá vắng phải trả (false, nil)")
	}
	var bad int
	ok, err = b.Pack.Section("ten", &bad)
	if !ok || err == nil {
		t.Fatal("khoá sai kiểu phải trả (true, error)")
	}
}

// TestPackVoiceOverride_Replacement: giá trị chuỗi trong voice thay toàn bộ
// nội dung mục thay vì xoá.
func TestPackVoiceOverride_Replacement(t *testing.T) {
	dir := t.TempDir()
	writeTestPack(t, filepath.Join(dir, "packs", "goi-thay"),
		`{"ten":"Thay","luat":["a.md"],"voice":{"hoi-hop":"- Điểm móc tùy ý gói."}}`,
		map[string]string{"a.md": "luật"}, "")
	b := Load("goi-thay", LoadOptions{HomeStyleDir: dir})
	if !strings.Contains(b.Voice, "- Điểm móc tùy ý gói.") {
		t.Fatal("ghi đè chuỗi phải thay nội dung mục")
	}
	if strings.Contains(b.Voice, "không nhất thiết mỗi chương đều phải làm hồi hộp") {
		t.Fatal("nội dung mục bị thay không được còn sót")
	}
}

// TestPackRulesSection_OrderAndVerbatim: RulesSection phải dẫn file luat theo
// đúng thứ tự goi.json và giữ nguyên văn nội dung.
func TestPackRulesSection_OrderAndVerbatim(t *testing.T) {
	dir := t.TempDir()
	sentinel := "Câu đánh dấu duy nhất này phải xuất hiện nguyên văn."
	writeTestPack(t, filepath.Join(dir, "packs", "goi-thu-tu"),
		`{"ten":"Thứ tự","luat":["b.md","a.md"]}`,
		map[string]string{"a.md": "luật A", "b.md": sentinel}, "")
	b := Load("goi-thu-tu", LoadOptions{HomeStyleDir: dir})
	section := b.Pack.RulesSection()
	if !strings.Contains(section, sentinel) {
		t.Fatal("nội dung luat phải đi vào section nguyên văn")
	}
	if strings.Index(section, sentinel) > strings.Index(section, "luật A") {
		t.Fatal("thứ tự file phải theo goi.json (b.md trước a.md)")
	}
	if !strings.Contains(section, "Luật gói thể loại") || !strings.Contains(section, "[LỖI]") {
		t.Fatal("section phải có heading và ghi chú thứ tự ưu tiên")
	}
}

func writeTestPack(t *testing.T, dir, manifest string, luat map[string]string, vanPhong string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "goi.json"), manifest)
	for name, content := range luat {
		writeFile(t, filepath.Join(dir, "luat", name), content)
	}
	if vanPhong != "" {
		writeFile(t, filepath.Join(dir, "van-phong.md"), vanPhong)
	}
}
