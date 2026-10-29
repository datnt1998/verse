package audio

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeTable(t *testing.T) {
	dict := []Pronunciation{{Written: "live", Spoken: "lai"}, {Written: "show", Spoken: "sô"}}
	opts := DefaultNormalizeOptions()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"số", "Anh ấy chờ 90 giây.", "Anh ấy chờ chín mươi giây."},
		{"nghìn", "1.045 người", "một nghìn không trăm bốn mươi lăm người"},
		{"triệu", "2.000.000 đồng", "hai triệu đồng"},
		{"linh", "101 căn", "một trăm linh một căn"},
		{"mốt tư lăm", "21 24 25", "hai mươi mốt hai mươi tư hai mươi lăm"},
		{"số 0", "0 giây", "không giây"},
		{"mã có số 0 đầu", "mã 007", "mã không không bảy"},
		{"số thứ tự", "thứ 1 và thứ 3", "thứ nhất và thứ ba"},
		{"thập phân vi", "3,5 điểm", "ba phẩy năm điểm"},
		{"thập phân en", "1.75 m", "một phẩy bảy mươi lăm mét"},
		{"phần trăm", "45% dân số", "bốn mươi lăm phần trăm dân số"},
		{"tiền đồng", "50.000đ", "năm mươi nghìn đồng"},
		{"tiền k", "anh có 50k", "anh có năm mươi nghìn"},
		{"tiền triệu", "5 triệu", "năm triệu"},
		{"tiền đô", "$5 và 20 USD", "năm đô la và hai mươi đô la"},
		{"ngày đầy đủ", "12/03/2024", "ngày mười hai tháng ba năm hai nghìn không trăm hai mươi tư"},
		{"ngày ngắn", "3/5 hôm ấy", "ngày ba tháng năm hôm ấy"},
		{"giờ phút", "14:30 chiều", "mười bốn giờ ba mươi phút chiều"},
		{"giờ h", "8h30 sáng", "tám giờ ba mươi phút sáng"},
		{"khoảng", "3-5 người", "ba đến năm người"},
		{"đơn vị", "chạy 5 km rồi 2 kg", "chạy năm ki-lô-mét rồi hai ki-lô-gam"},
		{"ký hiệu", "A & B", "A và B"},
		{"từ điển", "lên live stream", "lên lai stream"},
		{"viết tắt", "ở TP.HCM", "ở thành phố Hồ Chí Minh"},
		{"ngoặc vuông", "[cười] Anh ấy nói", "Anh ấy nói"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := Normalize(tc.in, opts, dict)
			if got != tc.want {
				t.Fatalf("Normalize(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeRecordsChanges(t *testing.T) {
	got, changes := Normalize("không quá 90 giây, lên live", DefaultNormalizeOptions(), []Pronunciation{{Written: "live", Spoken: "lai"}})
	if !strings.Contains(got, "chín mươi giây") || !strings.Contains(got, "lai") {
		t.Fatalf("unexpected text %q", got)
	}
	var kinds []string
	for _, c := range changes {
		kinds = append(kinds, c.Kind+":"+c.From+"→"+c.To)
	}
	want := []string{"pronunciation:live→lai", "number:90→chín mươi"}
	if strings.Join(kinds, "|") != strings.Join(want, "|") {
		t.Fatalf("changes = %v, want %v", kinds, want)
	}
}

func TestNormalizeSwitchesOff(t *testing.T) {
	opts := NormalizeOptions{} // 全关
	got, changes := Normalize("90 giây, 45%, [cười]", opts, nil)
	if got != "90 giây, 45%, [cười]" {
		t.Fatalf("all-off should keep text, got %q", got)
	}
	if len(changes) != 0 {
		t.Fatalf("all-off should record nothing, got %v", changes)
	}
}

func TestEnglishDetection(t *testing.T) {
	got := englishWords("Tôi bật live stream và nói ok với MC")
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "live") || !strings.Contains(joined, "stream") || !strings.Contains(joined, "ok") || !strings.Contains(joined, "MC") {
		t.Fatalf("englishWords missed tokens: %v", got)
	}
	if v := englishWords("Lâm Tri Hạ nhìn Phùng Chấn"); len(v) != 0 {
		t.Fatalf("vietnamese names flagged as english: %v", v)
	}
}

func TestEnglishFlagTable(t *testing.T) {
	cases := []struct {
		text string
		want []string // từ tiếng Anh được cờ; rỗng = câu thuần Việt
	}{
		// 4 dòng từng hiện nhầm chip "Từ tiếng Anh" (toàn âm tiết Việt).
		{"Một tờ giấy được đặt lên bục trước mặt tôi. Tiêu đề in đậm: Thư xin lỗi và cam kết khắc phục.", nil},
		{"Tôi gõ móng tay lên dòng chữ in đậm.", nil},
		{"“Vậy cho tôi nghe toàn bộ bản thu.”", nil},
		{"Tôi phóng to nó bằng hai ngón tay. Đường nét quen thuộc đến phát bực.", nil},
		// Âm tiết Việt trùng hình từ Anh vẫn không được cờ.
		{"Anh ta cam chịu, in cho ban quản lý một tờ có chữ “hiến” để bàn.", nil},
		{"Tiếng hot của còi vang lên; cơn bin đi qua đầu.", nil},
		// Âm tiết khó: mở bằng âm chính, đ/c/gh/ngh, vần ăc/âc/oac/êch/ươn/uynh…
		{"ấy gì các bậc nhấc éch nghệ khoách ựa ụa ọt ổi ửng ưu yểu yên ỵ.", nil},
		// Tiếng Anh thật phải bị cờ.
		{"Cô mở email rồi gửi file ngay.", []string{"email", "file"}},
		{"Đoàn phim làm marketing và quay livestream suốt đêm.", []string{"marketing", "livestream"}},
		{"Anh bảo ok, rồi đăng show lên Weibo.", []string{"ok", "show", "Weibo"}},
		{"Bài post nhận nhiều view, comment và share.", []string{"post", "view", "comment", "share"}},
		{"Scandal của casting bị cả team né; trend đổi trong credit mở đầu.", []string{"Scandal", "casting", "team", "trend", "credit"}},
		// Câu trộn lẫn: chỉ từ Anh được cờ, âm tiết Việt trùng hình bỏ qua.
		{"Trong video, MC mở hashtag cho bản tin của em.", []string{"video", "MC", "hashtag"}},
	}
	for _, tc := range cases {
		got := englishWords(tc.text)
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("englishWords(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestCountWords(t *testing.T) {
	if got := countWords("một hai ba"); got != 3 {
		t.Fatalf("countWords = %d", got)
	}
}

func TestNormalizeEuroAndBoundaries(t *testing.T) {
	cases := map[string]string{
		"giá 5€ thôi":   "giá năm euro thôi",
		"3,5 điểm":      "ba phẩy năm điểm",
		"5 mét":         "năm mét",
		"5 đô la":       "năm đô la",
		"20 nghìn đồng": "hai mươi nghìn đồng",
	}
	for in, want := range cases {
		if got, _ := Normalize(in, DefaultNormalizeOptions(), nil); got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripMarkdownEmphasis(t *testing.T) {
	cases := map[string]string{
		"bộ phim *Bữa Cơm Để Nguội*, là": "bộ phim Bữa Cơm Để Nguội, là",
		"**đậm** và _nghiêng_":           "đậm và nghiêng",
		"3 * 4 = 12":                     "3 * 4 = 12",
		"tên_biến *lệch_":                "tên_biến *lệch_",
	}
	for in, want := range cases {
		if got := stripMarkdown(in); got != want {
			t.Errorf("stripMarkdown(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFinalizeSegmentsStripsMarkdown(t *testing.T) {
	var parsed composeResponse
	if err := json.Unmarshal([]byte(`{"segments":[{"speaker":"dan","text":"Chương trình *Bếp Nhà Có Khách* gửi lời mời."}]}`), &parsed); err != nil {
		t.Fatal(err)
	}
	segs, err := finalizeSegments(parsed, ComposeInput{Normalize: DefaultNormalizeOptions()})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 || strings.Contains(segs[0].Text, "*") {
		t.Fatalf("segments = %+v", segs)
	}
}
