package audio

import (
	"context"
	"strings"
	"testing"
)

// TestPrepareReadingText 锁死朗读文本的改写规则：
//   - 场景标题改成一句普通的话，不带 CẢNH/MONTAGE/NỘI./NGOẠI.；
//   - 紧跟标题的整行大写地点行同样改写成 "Bối cảnh: …"；
//   - 其余整行大写的句子按小写朗读；
//   - 正文与"cảnh 3 của vở kịch"原样保留。
func TestPrepareReadingText(t *testing.T) {
	text := strings.Join([]string{
		"# Dòng lệch lịch",
		"",
		"**CẢNH 7 – MONTAGE – TỪ SAU CUỘC HỌP ĐẾN TRƯỚC THỨ SÁU**",
		"",
		"PHÒNG DỰ ÁN AI.",
		"",
		"Trong phòng dự án AI, Tuyết Nhi mở sơ đồ tích hợp.",
		"",
		"LÂM",
		"",
		"— Lại cái module 2016 à?",
		"",
		"Cô đọc lại cảnh 3 của vở kịch trong đầu.",
	}, "\n")

	cleaned, changed := PrepareReadingText(text)
	wantChanged := []string{
		"CẢNH 7 – MONTAGE – TỪ SAU CUỘC HỌP ĐẾN TRƯỚC THỨ SÁU",
		"PHÒNG DỰ ÁN AI.",
	}
	if len(changed) != len(wantChanged) {
		t.Fatalf("changed = %v, muốn %v", changed, wantChanged)
	}
	for i := range wantChanged {
		if changed[i] != wantChanged[i] {
			t.Fatalf("changed[%d] = %q, muốn %q", i, changed[i], wantChanged[i])
		}
	}
	for _, gone := range []string{"CẢNH", "MONTAGE", "NỘI.", "NGOẠI.", "PHÒNG DỰ ÁN AI.", "LÂM"} {
		if strings.Contains(cleaned, gone) {
			t.Fatalf("bản đọc còn %q: %q", gone, cleaned)
		}
	}
	for _, keep := range []string{
		"# Dòng lệch lịch",
		"Bối cảnh: từ sau cuộc họp đến trước thứ sáu.",
		"Bối cảnh: phòng dự án ai.",
		"Trong phòng dự án AI, Tuyết Nhi mở sơ đồ tích hợp.",
		"lâm",
		"— Lại cái module 2016 à?",
		"Cô đọc lại cảnh 3 của vở kịch trong đầu.",
	} {
		if !strings.Contains(cleaned, keep) {
			t.Fatalf("mất nội dung %q: %q", keep, cleaned)
		}
	}
}

// TestPrepareReadingTextLeavesPlainProse 没有剧本痕迹的正文必须逐字返回。
func TestPrepareReadingTextLeavesPlainProse(t *testing.T) {
	text := "# Ca trực hai giờ\n\nLúc 02:00, trong phòng trực hạ tầng, ba dịch vụ giao dịch đồng loạt rớt khỏi bảng theo dõi.\n\n— Lại cái module 2016 à?"
	cleaned, changed := PrepareReadingText(text)
	if len(changed) != 0 {
		t.Fatalf("không được đổi dòng nào: %v", changed)
	}
	if cleaned != text {
		t.Fatalf("văn xuôi bị đổi:\n%q\nmuốn\n%q", cleaned, text)
	}
}

// TestPrepareReadingTextStripsDirectionLabels 锁死 ch06 的真实形态：
// "NỘI VÀ NGOẠI – KHU HÀNH CHÍNH …" 不读方向标记，只留地点与时间。
func TestPrepareReadingTextStripsDirectionLabels(t *testing.T) {
	text := "# Thuyền cứu sinh\n\nNỘI VÀ NGOẠI – KHU HÀNH CHÍNH ĐẾN BÃI XE – SAU ĐÓ.\n\nTại khu hành chính, một tờ thông báo được đẩy qua mặt bàn."
	cleaned, changed := PrepareReadingText(text)
	if len(changed) != 1 {
		t.Fatalf("changed = %v", changed)
	}
	for _, gone := range []string{"NỘI", "NGOẠI", "nội và ngoại", "– khu"} {
		if strings.Contains(cleaned, gone) {
			t.Fatalf("bản đọc còn %q: %q", gone, cleaned)
		}
	}
	if !strings.Contains(cleaned, "Bối cảnh: khu hành chính đến bãi xe – sau đó.") {
		t.Fatalf("thiếu câu bối cảnh tự nhiên: %q", cleaned)
	}
}

// TestNormalizeSlashes 锁死 "/" 的读法：绝不对普通斜杠读 "trên"。
func TestNormalizeSlashes(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "Hạn chót là 28/10.", want: "Hạn chót là ngày hai mươi tám tháng mười."},
		{in: "Chọn và/hoặc cả hai.", want: "Chọn hoặc cả hai."},
		{in: "Gửi A/B cho tôi.", want: "Gửi A hoặc B cho tôi."},
		{in: "Còn 21/23 số tiền.", want: "Còn hai mươi mốt, hai mươi ba số tiền."},
		{in: "Thang dừng ở tầng 32/1.", want: "Thang dừng ở tầng ba mươi hai, một."},
	}
	for _, tc := range cases {
		got, _ := Normalize(tc.in, DefaultNormalizeOptions(), nil)
		if got != tc.want {
			t.Fatalf("Normalize(%q) = %q, muốn %q", tc.in, got, tc.want)
		}
		if strings.Contains(got, "trên") && !strings.Contains(tc.in, "21/23") {
			t.Fatalf("không được đọc \"trên\" cho \"/\" thường: %q → %q", tc.in, got)
		}
	}
}

// TestComposeSkipsScreenplayHeadings 锁死整条 soạn bản đọc：发给模型的章节正文
// 不再有场景标题/地点 slugline，且没有任何 CẢNH/MONTAGE/NỘI。
func TestComposeSkipsScreenplayHeadings(t *testing.T) {
	model := &fakeChatModel{reply: `{"segments":[{"speaker":"dan","text":"Trong phòng dự án AI, Tuyết Nhi mở sơ đồ tích hợp.","tone":"","pauseMs":0}]}`}
	chapter := strings.Join([]string{
		"# Dòng lệch lịch",
		"",
		"**CẢNH 7 – MONTAGE – TỪ SAU CUỘC HỌP ĐẾN TRƯỚC THỨ SÁU**",
		"",
		"**PHÒNG DỰ ÁN AI.**",
		"",
		"Trong phòng dự án AI, Tuyết Nhi mở sơ đồ tích hợp.",
		"",
		"Cô đọc lại cảnh 3 của vở kịch trong đầu.",
	}, "\n")

	composer := &ChatComposer{Model: model, ID: "test/model"}
	if _, err := composer.Compose(context.Background(), ComposeInput{
		Chapter: 7, Title: "Dòng lệch lịch", Text: chapter,
	}); err != nil {
		t.Fatalf("Compose: %v", err)
	}

	prompt := model.user
	for _, gone := range []string{"CẢNH", "MONTAGE", "NỘI."} {
		if strings.Contains(prompt, gone) {
			t.Fatalf("prompt soạn bản đọc còn %q:\n%s", gone, prompt)
		}
	}
	for _, keep := range []string{
		"Bối cảnh: từ sau cuộc họp đến trước thứ sáu.",
		"Trong phòng dự án AI, Tuyết Nhi mở sơ đồ tích hợp.",
		"Cô đọc lại cảnh 3 của vở kịch trong đầu.",
	} {
		if !strings.Contains(prompt, keep) {
			t.Fatalf("prompt soạn bản đọc mất %q:\n%s", keep, prompt)
		}
	}
}
