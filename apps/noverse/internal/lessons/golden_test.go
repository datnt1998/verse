package lessons_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/lessons"
)

// Golden violations-*.json của web/mock: bảng mục luật hay vấp theo gói. Mock
// từng tự gom nhóm + đếm truyện; giờ cả hai lớp (gói lessons và handler web)
// cùng đọc những file này, nên mọi lệch tính năng đều lộ ngay.
//
// `go test ./internal/lessons -update` ghi lại sau khi chủ đích đổi
// Violations/Preamble hoặc bảng quan sát.

var updateMockGoldens = flag.Bool("update", false, "ghi lại golden của mock dưới web/mock/golden")

func assertMockGolden(t *testing.T, name string, v any) {
	t.Helper()
	var pretty bytes.Buffer
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		t.Fatal(err)
	}
	// Kết thúc giống body handler (Encoder.Encode): đúng một newline.
	pretty.WriteByte('\n')
	path := filepath.Join("..", "..", "web", "mock", "golden", name)
	if *updateMockGoldens {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (chạy go test -update để tạo)", path, err)
	}
	if !bytes.Equal(pretty.Bytes(), want) {
		t.Fatalf("%s lệch output lessons.Violations; nếu chủ đích, chạy -update\ngot:\n%s", path, pretty.String())
	}
}

// packMeta mô tả ba gói thể loại thật (assets/packs): id, tên hiển thị, số
// truyện của mỗi gói trong fixture. Cùng bảng với goldenLessonStories của
// internal/entry/web — hai bên phải khớp, test web so byte-response với golden
// này để bắt lệch.
var goldenPacks = []struct {
	id, name string
	stories  int
}{
	{"nu-tan-bao-thu", "Nữ tần trọng sinh báo thù, vả mặt", 6},
	{"nguoc-hon-nhan", "Ngược hôn nhân, ly hôn, hối hận muộn", 4},
	{"nien-dai-van", "Niên đại văn, trọng sinh thập niên, làm giàu", 2},
}

// goldenObservations: cùng bảng quan sát với goldenLessonStories (internal/entry/
// web/mock_golden_test.go). RefName là tiêu đề mục mà reader.titles đã tra (gói
// nu-tan-bao-thu) hoặc rule_text (mục dùng chung).
func goldenObservations() []lessons.Observation {
	type row struct {
		story, title, pack string
		issues             [][3]any // {ref, level, chapter}
	}
	stories := []row{
		{"proj/01-tra-xanh", "Trà Xanh Cướp Kịch Bản", "nu-tan-bao-thu", [][3]any{
			{"Master §7", "error", 3}, {"Master §13", "warning", 3}, {"Bộ A §4.3", "warning", 3}, {"Xưng hô", "error", 2}, {"Master §11", "error", 1}, {"Master §15", "warning", 2}}},
		{"proj/02-em-gai", "Em Gái Nuôi Mặc Váy Cưới", "nu-tan-bao-thu", [][3]any{
			{"Master §7", "error", 2}, {"Master §7", "error", 5}, {"Master §13", "warning", 2}, {"Master §8", "error", 2}, {"Master §11", "error", 1}, {"Xưng hô", "error", 2}}},
		{"proj/03-me-ke", "Mẹ Kế Cướp Tiệm May", "nu-tan-bao-thu", [][3]any{
			{"Master §7", "error", 6}, {"Master §13", "warning", 1}, {"Master §15", "warning", 4}, {"Xưng hô", "error", 1}}},
		{"proj/04-thien-kim", "Thiên Kim Thật Trở Về", "nu-tan-bao-thu", [][3]any{
			{"Master §7", "error", 4}, {"Master §13", "warning", 1}}},
		{"proj/05-ban-than", "Bạn Thân Giật Chồng", "nu-tan-bao-thu", [][3]any{
			{"Master §15", "warning", 2}, {"Master §15", "warning", 4}}},
		{"proj/06-chi-dau", "Chị Dâu Bắt May Váy Cưới", "nu-tan-bao-thu", [][3]any{
			{"Bộ A §4.3", "warning", 1}, {"Bộ A §4.3", "warning", 2}, {"Master §11", "error", 3}}},
		{"proj/07-tong-tai", "Tổng Tài Lạnh Lùng Hối Hận", "nguoc-hon-nhan", [][3]any{
			{"Master §4", "error", 1}, {"Master §10", "error", 2}}},
		{"proj/08-nguoi-yeu-cu", "Người Yêu Cũ Quay Lại", "nguoc-hon-nhan", [][3]any{
			{"Master §4", "error", 1}, {"Master §4", "error", 2}, {"Master §10", "error", 1}, {"Master §8", "error", 3}}},
		{"proj/09-lam-tam", "Làm Tổng Tài Giả", "nguoc-hon-nhan", [][3]any{
			{"Master §10", "error", 1}, {"Master §13", "warning", 1}, {"Master §13", "warning", 4}}},
		{"proj/10-em-re", "Em Rể Vào Cửa Sau", "nguoc-hon-nhan", [][3]any{
			{"Master §7", "error", 1}, {"Xưng hô", "warning", 1}}},
		{"proj/11-co-dau", "Cô Dâu Nhỏ Thập Niên Tám Mươi", "nien-dai-van", [][3]any{
			{"Master §6", "error", 2}, {"Master §15", "warning", 2}, {"Master §15", "warning", 4}}},
		{"proj/12-ong-chu", "Ông Chủ Xưởng May", "nien-dai-van", [][3]any{
			{"Master §6", "error", 1}, {"Master §9", "warning", 1}}},
	}
	// Tiêu đề mục như reader.titles trả cho gói nu-tan-bao-thu; các gói khác
	// dùng cùng nhãn "Master"/"Bộ …" với tiêu đề tương đương (refName chỉ để
	// hiển thị).
	// Tiêu đề mục y hệt reader.titles (assets/packs/<id> SectionTitles); mục dùng
	// chung lấy từ rule_text. Ba gói Master dùng chung bộ tiêu đề này.
	// "Quy tắc của bạn" không có trong bảng: handler bỏ qua mục không tên
	// (refName rỗng) nên không thành quan sát.
	refNames := map[string]string{
		"Master §4": "Mở đầu: mốc 30 giây, 2 phút, 5 phút", "Master §6": "Leo thang",
		"Master §7": "Đầu chương và cuối chương", "Master §8": "Vùng giữa truyện (khoảng 40–70% thời lượng)",
		"Master §9": "Cú trả thưởng", "Master §10": "Ức chế và lòng thương (mới)",
		"Master §11": "Công lý và kết cục phản diện (mới)", "Master §13": "Văn phong, thoại, hài",
		"Master §15": "Viết cho audio", "Bộ A §4.3": "Trà xanh và phản diện",
		"Xưng hô": "Bảng xưng hô của truyện",
	}
	out := []lessons.Observation{}
	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	n := 0
	for i, s := range stories {
		for j, row := range s.issues {
			ref, level, ch := row[0].(string), row[1].(string), row[2].(int)
			n++
			out = append(out, lessons.Observation{
				ID: fmt.Sprintf("obs-%d", n), Story: s.story, StoryTitle: s.title, PackID: s.pack,
				Ref: ref, RefName: refNames[ref], Level: level, Chapter: ch,
				At: at.Add(time.Duration(i*len(s.issues)+j) * time.Hour),
			})
		}
	}
	return out
}

// violationsResponse là khung GET /api/system/violations?pack=… của handler;
// giữ nguyên thứ tự khoá để golden byte giống hệt body HTTP.
type violationsResponse struct {
	Rows     []lessons.Row `json:"rows"`
	Preamble string        `json:"preamble"`
	PackName string        `json:"packName"`
	Stories  int           `json:"stories"`
}

func TestMockGoldenViolations(t *testing.T) {
	observations := goldenObservations()
	for _, pack := range goldenPacks {
		rows := lessons.Violations(observations, pack.id, pack.stories)
		res := violationsResponse{Rows: rows, Preamble: lessons.Preamble(rows, pack.name), PackName: pack.name, Stories: pack.stories}
		assertMockGolden(t, "violations-"+pack.id+".json", res)
	}
}
