package rules

import "testing"

func TestOpeningOverlapRunes(t *testing.T) {
	cases := []struct {
		name     string
		prevTail string
		opening  string
		want     int
	}{
		{
			name:     "越南语有 dấu、标点不同",
			prevTail: "Gió nổi lên, trời đã tối.",
			opening:  "Trời đã tối. Gió nổi lên.",
			want:     11, // "troi da toi" / "gio noi len" 各 11
		},
		{
			name:     "弯引号与直引号等价",
			prevTail: "“Đi thôi,” cô nói.",
			opening:  "\"Đi thôi,\" cô nói.",
			want:     14, // "di thoi co noi"
		},
		{
			name:     "大小写与多余空白",
			prevTail: "LÂM   VÃN   mở cửa",
			opening:  "lâm vãn mở cửa",
			want:     14, // "lam van mo cua"
		},
		{
			name:     "无公共字母",
			prevTail: "Lâm",
			opening:  "Tuyết",
			want:     0,
		},
		{
			name:     "空输入",
			prevTail: "",
			opening:  "Trời đã tối",
			want:     0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := OpeningOverlapRunes(tc.prevTail, tc.opening); got != tc.want {
				t.Fatalf("OpeningOverlapRunes(%q, %q) = %d, want %d", tc.prevTail, tc.opening, got, tc.want)
			}
		})
	}
}

func TestTrimLeadingOverlap(t *testing.T) {
	cases := []struct {
		name        string
		tail        string
		next        string
		minRunes    int
		wantRest    string
		wantTrimmed int
	}{
		{
			name:        "剪掉重复的句子收尾",
			tail:        "cô bước vào. Gió nổi lên, trời đã tối.",
			next:        "Trời đã tối. Gió lại nổi.",
			minRunes:    10,
			wantRest:    "Gió lại nổi.",
			wantTrimmed: 13,
		},
		{
			name:        "重叠短于阈值则不动",
			tail:        "trời đã tối",
			next:        "Trời đã tối. Gió lại nổi.",
			minRunes:    30,
			wantRest:    "Trời đã tối. Gió lại nổi.",
			wantTrimmed: 0,
		},
		{
			name:        "无重叠则原样返回",
			tail:        "Lâm Vãn mở cửa.",
			next:        "Tuyết rơi trên mái.",
			minRunes:    5,
			wantRest:    "Tuyết rơi trên mái.",
			wantTrimmed: 0,
		},
		{
			name:        "剩余文本去掉开头空白",
			tail:        "nàng khẽ nói",
			next:        "  Nàng khẽ nói rồi đi.",
			minRunes:    5,
			wantRest:    "rồi đi.",
			wantTrimmed: 15,
		},
		{
			name:        "弯引号同样参与比对",
			tail:        "“Đừng đi,” cô nói.",
			next:        "\"Đừng đi,\" cô nói rồi quay lưng.",
			minRunes:    10,
			wantRest:    "rồi quay lưng.",
			wantTrimmed: 18,
		},
		{
			name:        "共享词干但不同词不得剪进半个词",
			tail:        "Cô dừng lại trước bà.",
			next:        "Trước bàn đá, gió còn lạnh hơn.",
			minRunes:    5,
			wantRest:    "Trước bàn đá, gió còn lạnh hơn.",
			wantTrimmed: 0,
		},
		{
			name:        "tail 结尾整词重抄仍然剪",
			tail:        "Cô dừng lại trước bà.",
			next:        "Trước bà, gió còn lạnh hơn.",
			minRunes:    5,
			wantRest:    "gió còn lạnh hơn.",
			wantTrimmed: 10,
		},
		{
			name:        "变音符仍按词界判定",
			tail:        "Anh đứng giữa nghĩa địa trước bóng.",
			next:        "Trước bức tường cũ, anh dừng lại.",
			minRunes:    5,
			wantRest:    "Trước bức tường cũ, anh dừng lại.",
			wantTrimmed: 0,
		},
		{
			name:        "tail 中段出现的整词不贴尾不算",
			tail:        "trời đã tối rồi hắn mở đèn",
			next:        "Trời đã tối. Hắn mở đèn.",
			minRunes:    5,
			wantRest:    "Trời đã tối. Hắn mở đèn.",
			wantTrimmed: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rest, trimmed := TrimLeadingOverlap(tc.tail, tc.next, tc.minRunes)
			if rest != tc.wantRest || trimmed != tc.wantTrimmed {
				t.Fatalf("TrimLeadingOverlap(%q, %q, %d) = (%q, %d), want (%q, %d)",
					tc.tail, tc.next, tc.minRunes, rest, trimmed, tc.wantRest, tc.wantTrimmed)
			}
		})
	}
}
