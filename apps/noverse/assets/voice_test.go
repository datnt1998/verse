package assets

import (
	"strings"
	"testing"
)

// TestVoice_RoundTripByteIdentical: ráp lại không ghi đè phải ra đúng
// nguyên văn bản trước khi chia khoá (đã lược dòng marker).
func TestVoice_RoundTripByteIdentical(t *testing.T) {
	raw := mustRead(voiceFS, "voice.md")
	sections := ParseVoice(raw)

	got, err := AssembleVoice(sections, nil)
	if err != nil {
		t.Fatalf("AssembleVoice không ghi đè mà lỗi: %v", err)
	}
	want := stripVoiceMarkers(raw)
	if got != want {
		t.Fatalf("Ráp lại phải bằng nguyên bản lược marker:\n%s", firstDiff(want, got))
	}
	if strings.Contains(got, "<!--") {
		t.Fatal("Voice đã ráp không được còn marker")
	}
}

// TestParseVoice_Keys: mọi mục đều có khoá riêng, gồm hai khoá bắt buộc của P0-6.
func TestParseVoice_Keys(t *testing.T) {
	sections := ParseVoice(mustRead(voiceFS, "voice.md"))
	if len(sections) == 0 {
		t.Fatal("voice.md phải chia thành nhiều mục")
	}
	keys := make(map[string]bool, len(sections))
	for _, s := range sections {
		if s.Key == "" {
			t.Fatal("voice.md còn nội dung trước marker đầu tiên (khoá rỗng)")
		}
		if keys[s.Key] {
			t.Fatalf("Khoá voice bị trùng: %s", s.Key)
		}
		if strings.TrimSpace(s.Text) == "" {
			t.Fatalf("Mục %s rỗng", s.Key)
		}
		keys[s.Key] = true
	}
	for _, required := range []string{"hoi-hop", "do-dai-doan"} {
		if !keys[required] {
			t.Fatalf("Thiếu khoá bắt buộc: %s", required)
		}
	}
}

// TestAssembleVoice_Drop: ghi đè nil xoá hẳn mục khỏi bản ráp.
func TestAssembleVoice_Drop(t *testing.T) {
	sections := ParseVoice(mustRead(voiceFS, "voice.md"))
	got, err := AssembleVoice(sections, map[string]*string{"hoi-hop": nil})
	if err != nil {
		t.Fatalf("AssembleVoice drop lỗi: %v", err)
	}
	if strings.Contains(got, "hồi hộp phóng đại") {
		t.Fatal("Mục hoi-hop phải bị xoá hẳn")
	}
	if !strings.Contains(got, "đoạn vụn 1-2 câu") {
		t.Fatal("Mục khác không được bị xoá theo")
	}
}

// TestAssembleVoice_Replace: ghi đè khác nil thay toàn bộ nội dung mục.
func TestAssembleVoice_Replace(t *testing.T) {
	sections := ParseVoice(mustRead(voiceFS, "voice.md"))
	repl := "- Mỗi chương bắt buộc kết bằng điểm móc hồi hộp."
	got, err := AssembleVoice(sections, map[string]*string{"hoi-hop": &repl})
	if err != nil {
		t.Fatalf("AssembleVoice replace lỗi: %v", err)
	}
	if strings.Contains(got, "không nhất thiết mỗi chương đều phải làm hồi hộp") {
		t.Fatal("Nội dung gốc của mục phải bị thay hoàn toàn")
	}
	if !strings.Contains(got, "bắt buộc kết bằng điểm móc hồi hộp") {
		t.Fatal("Nội dung thay thế phải xuất hiện đúng vị trí")
	}
	// Vị trí thay thế phải giữ đúng thứ tự mục: đứng sau mục bi-mat, trước do-dai-doan.
	iBiMat := strings.Index(got, "Bí mật phân kỳ giải phóng")
	iHoiHop := strings.Index(got, "bắt buộc kết bằng điểm móc")
	iDoDai := strings.Index(got, "đoạn vụn 1-2 câu")
	if !(iBiMat < iHoiHop && iHoiHop < iDoDai) {
		t.Fatalf("Mục bị thay đổi vị trí: bi-mat=%d hoi-hop=%d do-dai-doan=%d", iBiMat, iHoiHop, iDoDai)
	}
}

// TestAssembleVoice_UnknownKey: khoá lạ (lỗi chính tả của gói) phải trả lỗi.
func TestAssembleVoice_UnknownKey(t *testing.T) {
	sections := ParseVoice(mustRead(voiceFS, "voice.md"))
	repl := "x"
	if _, err := AssembleVoice(sections, map[string]*string{"hoi-hop-typo": &repl}); err == nil {
		t.Fatal("Khoá không tồn tại phải trả lỗi")
	}
	drop := map[string]*string{"khoa-linh-tinh": nil}
	if _, err := AssembleVoice(sections, drop); err == nil {
		t.Fatal("Khoá lạ dù xoá mục cũng phải trả lỗi")
	}
}

// stripVoiceMarkers trả về bản raw sau khi gỡ các dòng marker.
func stripVoiceMarkers(raw string) string {
	var b strings.Builder
	for line := range strings.Lines(raw) {
		if voiceMarkerRe.MatchString(strings.TrimSpace(line)) {
			continue
		}
		b.WriteString(line)
	}
	return b.String()
}
