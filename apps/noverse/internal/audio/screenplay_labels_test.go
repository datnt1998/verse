package audio

import (
	"context"
	"strings"
	"testing"
)

// TestPlanReadingTextSpeakerLabels 锁死 ch03/ch04/ch05 的真实形态：
// "TRIỆU: …"、"TRIỆU (nhìn chiếc điều khiển): …"、整行提示 "LÂM" 都不再读出来，
// 台词保留，动作改成短叙述，并记下台词归属。
func TestPlanReadingTextSpeakerLabels(t *testing.T) {
	text := strings.Join([]string{
		"# Mười lăm giây",
		"",
		"TRIỆU: Muốn nói thì lên đây mà nói.",
		"",
		"LÂM: Cho tôi mượn máy.",
		"",
		"TRIỆU (nhìn chiếc điều khiển): Cậu ta tưởng sửa được một con số là cả phòng phải đứng lên vỗ tay à?",
		"",
		"LÂM",
		"Khóa thẻ rồi. Tôi vẫn vào được bằng cửa kỹ thuật.",
		"",
		"HÀN (im lặng): Đủ rồi.",
	}, "\n")

	plan := PlanReadingText(text)
	for _, gone := range []string{"TRIỆU:", "LÂM:", "TRIỆU (", "HÀN (", "\nLÂM\n"} {
		if strings.Contains(plan.Text, gone) {
			t.Fatalf("bản đọc còn nhãn %q:\n%s", gone, plan.Text)
		}
	}
	for _, keep := range []string{
		"Muốn nói thì lên đây mà nói.",
		"Cho tôi mượn máy.",
		"Khóa thẻ rồi. Tôi vẫn vào được bằng cửa kỹ thuật.",
		"Đủ rồi.",
	} {
		if !strings.Contains(plan.Text, keep) {
			t.Fatalf("mất lời thoại %q:\n%s", keep, plan.Text)
		}
	}
	if !strings.Contains(plan.Text, "Triệu nhìn chiếc điều khiển.") {
		t.Fatalf("动作括号要变成短叙述:\n%s", plan.Text)
	}
	if strings.Contains(plan.Text, "im lặng") {
		t.Fatalf("括号内容无意义时要丢掉:\n%s", plan.Text)
	}

	wantSpeakers := map[string]string{
		"Muốn nói thì lên đây mà nói.": "TRIỆU",
		"Cho tôi mượn máy.":            "LÂM",
		"Cậu ta tưởng sửa được một con số là cả phòng phải đứng lên vỗ tay à?": "TRIỆU",
		"Khóa thẻ rồi. Tôi vẫn vào được bằng cửa kỹ thuật.":                    "LÂM",
		"Đủ rồi.": "HÀN",
	}
	for text, want := range wantSpeakers {
		found := false
		for _, hint := range plan.Dialogue {
			if strings.Contains(hint.Text, text) {
				found = true
				if hint.Speaker != want {
					t.Fatalf("台词 %q gán %q, muốn %q", text, hint.Speaker, want)
				}
			}
		}
		if !found {
			t.Fatalf("không ghi nhận台词 %q: %+v", text, plan.Dialogue)
		}
	}
}

// TestPlanReadingTextTransitions 锁死转场行：整行丢掉，并记下下一块的起始，
// 供朗读段补长停顿。
func TestPlanReadingTextTransitions(t *testing.T) {
	for _, transition := range []string{"CẮT ĐEN.", "CẮT SANG", "CHUYỂN CẢNH", "MỜ DẦN", "FADE OUT", "CUT TO"} {
		text := "Đoạn trước khép lại.\n\n" + transition + "\n\nĐoạn sau mở ra."
		plan := PlanReadingText(text)
		if strings.Contains(plan.Text, transition) {
			t.Fatalf("转场 %q 仍被读出:\n%s", transition, plan.Text)
		}
		if len(plan.PauseBlocks) != 1 || !strings.Contains(plan.PauseBlocks[0], "Đoạn sau mở ra.") {
			t.Fatalf("转场 %q 没有记录下一块的起始: %+v", transition, plan.PauseBlocks)
		}
	}
	// 正文里的"cắt 50%…"不是转场。
	prose := "Ông bấm điều khiển; hai dòng chữ lớn in đậm: CẮT 50% NHÂN SỰ."
	if plan := PlanReadingText(prose); len(plan.Changed) != 0 {
		t.Fatalf("văn xuôi bị coi là转场: %+v", plan.Changed)
	}

	// 章末转场（ch08 的 CẮT ĐEN.）：标记成结尾，落地时最后一段拖长停顿。
	tail := PlanReadingText("Đoạn cuối khép lại.\n\nCẮT ĐEN.")
	if len(tail.PauseBlocks) != 1 || tail.PauseBlocks[0] != pauseAtEnd {
		t.Fatalf("chapter-end transition phải tạo pauseAtEnd: %+v", tail.PauseBlocks)
	}
	segs := applyReadingPlan([]Segment{{Source: "Đoạn cuối khép lại."}}, tail, ComposeInput{})
	if segs[0].PauseMs < transitionPauseMs {
		t.Fatalf("最后一段停顿 = %d, muốn >= %d", segs[0].PauseMs, transitionPauseMs)
	}
}

// TestApplyReadingPlan 锁死落到朗读段上的效果：台词段 speaker 命中分演角色，
// 未知标记被清掉；转场后第一段的前一段拿到更长的 pauseMs。
func TestApplyReadingPlan(t *testing.T) {
	in := ComposeInput{Characters: []ComposeSpeaker{
		{ID: "lam", Name: "Lâm"},
		{ID: "trieu", Name: "Triệu"},
	}}
	plan := ReadingPlan{
		Dialogue: []DialogueHint{{Speaker: "LÂM", Text: "Cho tôi mượn máy."}},
		PauseBlocks: []string{
			"Đoạn sau mở ra.",
		},
	}
	segs := []Segment{
		{ID: "s0001", Speaker: "LÂM", Source: "Cho tôi mượn máy.", Flags: []string{FlagUnknownSpeaker}, SpeakerGuess: []SpeakerPick{{Who: "lâm", P: 0.4}}},
		{ID: "s0002", Speaker: NarratorID, Source: "Đoạn trước khép lại."},
		{ID: "s0003", Speaker: NarratorID, Source: "Đoạn sau mở ra."},
	}
	got := applyReadingPlan(segs, plan, in)
	if got[0].Speaker != "lam" {
		t.Fatalf("speaker = %q, muốn lam", got[0].Speaker)
	}
	if len(got[0].Flags) != 0 || got[0].SpeakerGuess != nil {
		t.Fatalf("phải xoá cờ unknown: %+v", got[0])
	}
	if got[1].PauseMs < transitionPauseMs {
		t.Fatalf("前一段停顿 = %d, muốn >= %d", got[1].PauseMs, transitionPauseMs)
	}
	if got[2].PauseMs != 0 {
		t.Fatalf("后一段不该被加停顿: %d", got[2].PauseMs)
	}
}

// TestComposeDropsLabelsAndAddsPause 端到端：prompt 里没有 TRIỆU:/LÂM: 与
// CẮT ĐEN，模型给出的段按分演角色归属，转场处有长停顿。
func TestComposeDropsLabelsAndAddsPause(t *testing.T) {
	chapter := strings.Join([]string{
		"# Mười lăm giây",
		"",
		"TRIỆU: Muốn nói thì lên đây mà nói.",
		"",
		"LÂM: Cho tôi mượn máy.",
		"",
		"CẮT ĐEN.",
		"",
		"Sáng hôm sau, hội trường trống.",
	}, "\n")
	plan := PlanReadingText(chapter)
	dialogue := plan.Dialogue[0].Text
	next := "Sáng hôm sau, hội trường trống."
	model := &fakeChatModel{reply: `{"segments":[
		{"speaker":"trieu","text":` + jsonString(dialogue) + `,"tone":"","pauseMs":0},
		{"speaker":"dan","text":` + jsonString(next) + `,"tone":"","pauseMs":0}
	]}`}

	composer := &ChatComposer{Model: model, ID: "test/model"}
	result, err := composer.Compose(context.Background(), ComposeInput{
		Chapter: 3, Title: "Mười lăm giây", Text: chapter,
		Characters: []ComposeSpeaker{{ID: "trieu", Name: "Triệu"}},
	})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	for _, gone := range []string{"TRIỆU:", "LÂM:", "CẮT ĐEN"} {
		if strings.Contains(model.user, gone) {
			t.Fatalf("prompt còn %q:\n%s", gone, model.user)
		}
	}
	if strings.Contains(result.Segments[0].Source, "TRIỆU") {
		t.Fatalf("段内含说话人标签: %q", result.Segments[0].Source)
	}
	if result.Segments[0].Speaker != "trieu" {
		t.Fatalf("speaker = %q", result.Segments[0].Speaker)
	}
	if result.Segments[0].PauseMs < transitionPauseMs {
		t.Fatalf("转场处停顿 = %d, muốn >= %d", result.Segments[0].PauseMs, transitionPauseMs)
	}
}

func jsonString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s) + `"`
}
