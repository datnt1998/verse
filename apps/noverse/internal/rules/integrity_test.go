package rules

import (
	"strings"
	"testing"
)

func admissionHas(vs []Violation, rule string) bool {
	for _, v := range vs {
		if v.Rule == rule {
			return true
		}
	}
	return false
}

func TestAdmission(t *testing.T) {
	clean := "Ngọc mở cửa tiệm lúc sáng sớm.\n\nMưa rơi trên mái ngói cũ.\n\nHắn khép cửa lại rồi đi vào trong."
	cases := []struct {
		name      string
		text      string
		stage     Stage
		wantRules []string
	}{
		{name: "clean draft", text: clean, stage: Draft},
		{name: "clean final", text: clean, stage: Final},

		// think_tag
		{name: "think open", text: "<think>hmm, should rain tonight</think>\n\n" + clean, stage: Draft, wantRules: []string{"think_tag"}},
		{name: "thinking close only", text: clean + "\n\n</thinking>", stage: Final, wantRules: []string{"think_tag", "truncated_end"}},
		{name: "uppercase tag", text: clean + "\n\n<THINK>random</THINK>", stage: Draft, wantRules: []string{"think_tag"}},
		{name: "word think in prose", text: "Hắn nghĩ đến chuyện hôm qua.\n\nMưa vẫn rơi đều trên mái.", stage: Draft},
		{name: "thinker dialogue", text: "\"Ông là một thinker,\" cô nói.\n\nHắn cười nhẹ rồi bước đi.", stage: Final},

		// code_fence
		{name: "fence line", text: "Đoạn mở đầu.\n\n```go\nfmt.Println()\n```\n\nĐoạn kết.", stage: Draft, wantRules: []string{"code_fence"}},
		{name: "fence at end", text: clean + "\n\n```", stage: Final, wantRules: []string{"code_fence", "truncated_end"}},

		// meta_commentary
		{name: "vn opener", text: "Dưới đây là chương 5 theo đại cương.\n\n" + clean, stage: Draft, wantRules: []string{"meta_commentary"}},
		{name: "vn closer", text: clean + "\n\nHy vọng chương này đạt yêu cầu, hãy cho tôi biết nếu cần sửa.", stage: Draft, wantRules: []string{"meta_commentary"}},
		{name: "en opener", text: "Here is chapter 5.\n\n" + clean, stage: Draft, wantRules: []string{"meta_commentary"}},
		{name: "as an ai", text: clean + "\n\nAs an AI, I cannot continue this scene.", stage: Final, wantRules: []string{"meta_commentary"}},
		{name: "rewritten note", text: clean + "\n\nPhần này đã được viết lại theo yêu cầu.", stage: Draft, wantRules: []string{"meta_commentary"}},
		{name: "meta mid-story ok", text: "Đoạn mở.\n\n\"Hy vọng chương này sẽ ổn,\" bản thảo của cô có dòng như vậy.\n\nĐoạn kết.", stage: Draft},
		{name: "letter dialogue mid-story", text: "Đoạn mở.\n\n\"Tôi sẽ viết thư cho anh,\" cô nói, giọng khẽ như mưa rơi xa.\n\nĐoạn kết.", stage: Draft},
		{name: "dialogue last paragraph ok", text: clean + "\n\n“Hãy cho tôi biết nếu anh còn sống,” cô nói.", stage: Draft},
		{name: "dash dialogue last paragraph ok", text: clean + "\n\n— Nếu bạn cần, cứ gọi.", stage: Draft},
		{name: "quoted meta in last paragraph ok", text: clean + "\n\n“Hy vọng chương này tới tay anh,” cô đọc lại bức thư.", stage: Draft},

		// scene_heading — nhãn kịch bản đứng riêng một dòng
		{name: "interior heading", text: "NỘI. PHÒNG TRỰC HẠ TẦNG – 02:00\n\n" + clean, stage: Draft, wantRules: []string{"scene_heading"}},
		{name: "combined int ext heading", text: clean + "\n\nNỘI./NGOẠI. KHU HÀNH CHÍNH – BÃI XE – SAU ĐÓ\n\n" + clean, stage: Draft, wantRules: []string{"scene_heading"}},
		{name: "bold scene number", text: "**CẢNH 7 – MONTAGE – TỪ SAU CUỘC HỌP**\n\n" + clean, stage: Final, wantRules: []string{"scene_heading"}},
		{name: "bare scene number", text: clean + "\n\nCẢNH 9\n\n" + clean, stage: Final, wantRules: []string{"scene_heading"}},
		{name: "scene word in prose ok", text: "CẢNH TƯỢNG ấy làm cô sững người.\n\nCảnh 3 của vở kịch diễn ra trong im lặng.", stage: Final},
		{name: "uppercase scene reference is prose", text: "CẢNH 3 của vở kịch khiến cô bật khóc.", stage: Final},
		{name: "emphasized scene reference is prose", text: "**CẢNH 3 của vở kịch khiến cô bật khóc.**\n\n" + clean, stage: Final},
		// truncated_end — chỉ Final mới chặn
		{name: "no end punct final", text: "Hắn bước ra ngoài rồi nhìn về phía xa mà không nói gì thêm nữa cả", stage: Final, wantRules: []string{"truncated_end"}},
		{name: "no end punct draft ok", text: "Hắn bước ra ngoài rồi nhìn về phía xa mà không nói gì thêm nữa cả", stage: Draft},
		{name: "ends with period", text: clean, stage: Final},
		{name: "dialogue close quote", text: "\"Đi ngay đi,\" hắn nói.\n\n\"Ừ, em biết rồi”", stage: Final},
		{name: "ends with dash", text: "Nếu như lúc ấy cô quay đầu—", stage: Final},
		{name: "ends with ellipsis", text: "Nhưng rồi mọi chuyện đã quá xa…", stage: Final},
		{name: "ends with wave", text: "Đừng quên nói với tớ nhé~", stage: Final},
		{name: "ends with straight quote", text: "Hắn chỉ nói một từ \"đi\"", stage: Final},
		{name: "ends with comma", text: "Rồi hắn kể lại mọi chuyện, từ đầu đến cuối,", stage: Final, wantRules: []string{"truncated_end"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vs := Admission(tc.text, tc.stage)
			for _, want := range tc.wantRules {
				if !admissionHas(vs, want) {
					t.Fatalf("muốn violation %q, nhận: %+v", want, vs)
				}
			}
			if len(vs) != len(tc.wantRules) {
				t.Fatalf("muốn %d violation %v, nhận %d: %+v", len(tc.wantRules), tc.wantRules, len(vs), vs)
			}
			for _, v := range vs {
				if v.Severity != SeverityError {
					t.Fatalf("admission severity phải là error: %+v", v)
				}
				if strings.TrimSpace(v.Message) == "" {
					t.Fatalf("admission violation thiếu hướng dẫn sửa: %+v", v)
				}
			}
		})
	}
}

// TestAdmissionMetaPosition 元叙述只认首段/末段；同一句话在中间段落出现
// 时属于正文内容（角色台词、书信、叙述），不得误报。
func TestAdmissionMetaPosition(t *testing.T) {
	mid := "Ngọc mở cửa tiệm.\n\nGhi chú của cô viết: \"dưới đây là chương cuối cuốn sổ tay\".\n\nHắn đọc xong rồi cất nó đi."
	if vs := Admission(mid, Final); admissionHas(vs, "meta_commentary") {
		t.Fatalf("cụm meta ở đoạn giữa không được báo: %+v", vs)
	}
}
