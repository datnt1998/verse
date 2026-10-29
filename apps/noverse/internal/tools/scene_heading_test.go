package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
)

// TestDraftChapterStripsSceneHeadings 锁死"不再拒收、就地删行"：带剧本场景标题
// 的草稿必须落盘成功，正文里没有标题行，回执说明删了哪一行并提示把地点/时间
// 写进叙述。写与续写两条路径都要生效。
func TestDraftChapterStripsSceneHeadings(t *testing.T) {
	heading := "**CẢNH 8 – NỘI. PHÒNG DEMO TẦNG 30 – THỨ SÁU**"
	body := heading + "\n\nTiếng vỗ tay vang lên ở phòng demo.\n\nCẢNH 9\n\nCửa cabin khép lại khi thang vẫn đứng tầng 12."
	for _, mode := range []string{"write", "append"} {
		t.Run(mode, func(t *testing.T) {
			s := store.NewStore(t.TempDir())
			if err := s.Init(); err != nil {
				t.Fatalf("Init: %v", err)
			}
			if err := s.Progress.Init(10); err != nil {
				t.Fatalf("InitProgress: %v", err)
			}
			if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
				t.Fatalf("UpdatePhase: %v", err)
			}
			args, _ := json.Marshal(map[string]any{"chapter": 1, "content": body, "mode": mode})
			raw, err := NewDraftChapterTool(s).Execute(context.Background(), args)
			if err != nil {
				t.Fatalf("dòng tiêu đề cảnh không được làm hỏng lượt ghi: %v", err)
			}

			draft, err := s.Drafts.LoadDraft(1)
			if err != nil {
				t.Fatalf("LoadDraft: %v", err)
			}
			for _, gone := range []string{"CẢNH 8", "CẢNH 9", "NỘI."} {
				if strings.Contains(draft, gone) {
					t.Fatalf("bản nháp còn sót %q: %q", gone, draft)
				}
			}
			for _, keep := range []string{"Tiếng vỗ tay vang lên ở phòng demo.", "Cửa cabin khép lại khi thang vẫn đứng tầng 12."} {
				if !strings.Contains(draft, keep) {
					t.Fatalf("bản nháp mất nội dung %q: %q", keep, draft)
				}
			}

			var out struct {
				Written bool     `json:"written"`
				Removed []string `json:"removed_scene_headings"`
				Note    string   `json:"scene_heading_note"`
			}
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if !out.Written {
				t.Fatalf("回执必须报告写入成功: %s", raw)
			}
			wantRemoved := []string{"CẢNH 8 – NỘI. PHÒNG DEMO TẦNG 30 – THỨ SÁU", "CẢNH 9"}
			if len(out.Removed) != len(wantRemoved) {
				t.Fatalf("removed_scene_headings = %v, muốn %v", out.Removed, wantRemoved)
			}
			for i := range wantRemoved {
				if out.Removed[i] != wantRemoved[i] {
					t.Fatalf("removed_scene_headings[%d] = %q, muốn %q", i, out.Removed[i], wantRemoved[i])
				}
			}
			if !strings.Contains(out.Note, "CẢNH 8") || !strings.Contains(out.Note, "câu kể") {
				t.Fatalf("回执要说清删了哪一行并提示写进叙述: %q", out.Note)
			}
		})
	}
}

// TestEditChapterStripsSceneHeadings 定点编辑落盘后同样机械删掉整行标题：
// 不退回给 writer 重发，并在回执里说明删了什么。
func TestEditChapterStripsSceneHeadings(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init(10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	enterEditWritingPhase(t, s)
	original := "Cửa cabin khép lại khi thang vẫn đứng tầng 12."
	if err := s.Drafts.SaveDraft(2, original); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	queueCompletedChapterForEdit(t, s, 2, len([]rune(original)))

	args, _ := json.Marshal(map[string]any{
		"chapter":    2,
		"old_string": original,
		"new_string": "**CẢNH 9 – NỘI. SẢNH THANG MÁY TẦNG 12**\n\n" + original,
	})
	raw, err := NewEditChapterTool(s).Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("edit thêm tiêu đề cảnh không được bị chặn: %v", err)
	}

	draft, err := s.Drafts.LoadDraft(2)
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if strings.Contains(draft, "CẢNH 9") || strings.Contains(draft, "NỘI.") {
		t.Fatalf("bản nháp còn sót tiêu đề cảnh: %q", draft)
	}
	if !strings.Contains(draft, original) {
		t.Fatalf("bản nháp mất nội dung đã có: %q", draft)
	}
	var out struct {
		Removed []string `json:"removed_scene_headings"`
		Note    string   `json:"scene_heading_note"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(out.Removed) != 1 || !strings.Contains(out.Removed[0], "CẢNH 9") {
		t.Fatalf("回执要列出被删的标题行: %s", raw)
	}
	if !strings.Contains(out.Note, "câu kể") {
		t.Fatalf("回执要提示把地点/时间写进叙述: %q", out.Note)
	}
}

// TestContextToolSanitizesScreenplayHeadings 锁死"源头净化"：从剧本导入的大纲
// 注入 writer 上下文时改成"Bối cảnh: …"普通句，地点与时间留住，writer 没有
// 可抄的标题行；store 里的大纲文件不改。
func TestContextToolSanitizesScreenplayHeadings(t *testing.T) {
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{
			Chapter:   1,
			Title:     "Ca trực hai giờ",
			CoreEvent: "CẢNH 1 — NỘI. PHÒNG TRỰC HẠ TẦNG — 02:00 ĐÊM TRƯỚC BUỔI HỌP. Server sập; Lâm sửa và xác nhận dịch vụ trở lại trong 10 phút.",
			Scenes:    []string{"NỘI. PHÒNG TRỰC", "Trần nhắc cuộc họp sáng mai"},
		},
		{
			Chapter:   2,
			Title:     "Trước hai trăm người",
			CoreEvent: "CẢNH 2 – NỘI. HỘI TRƯỜNG THIÊN CƠ – BUỔI SÁNG: Triệu trình diễn ví 15 phút trước 200 người.",
			Scenes:    []string{"Cô đọc lại cảnh 3 của vở kịch trong đầu"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Drafts.SaveChapterPlan(domain.ChapterPlan{
		Chapter: 1,
		Title:   "Ca trực hai giờ",
		Goal:    "CẢNH 1 — NỘI. PHÒNG TRỰC HẠ TẦNG — 02:00: mở chương giữa ca trực.",
		Contract: domain.ChapterContract{
			RequiredBeats: []string{"CẢNH 1: dịch vụ đồng loạt rớt; Lâm khôi phục trong 10 phút."},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Save(&domain.Progress{Phase: domain.PhaseWriting, TotalChapters: 2}); err != nil {
		t.Fatal(err)
	}

	raw, err := newTestContextTool(s, References{}, "default").Execute(context.Background(), json.RawMessage(`{"chapter":1}`))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	working := payload["working_memory"].(map[string]any)
	current := working["current_chapter_outline"].(map[string]any)
	coreEvent, _ := current["core_event"].(string)
	if strings.Contains(coreEvent, "CẢNH 1") || strings.Contains(coreEvent, "NỘI.") {
		t.Fatalf("core_event còn tiêu đề kịch bản: %q", coreEvent)
	}
	for _, keep := range []string{"Bối cảnh: phòng trực hạ tầng, 02:00 đêm trước buổi họp.", "Server sập; Lâm sửa và xác nhận dịch vụ trở lại trong 10 phút."} {
		if !strings.Contains(coreEvent, keep) {
			t.Fatalf("core_event mất %q: %q", keep, coreEvent)
		}
	}
	if !strings.Contains(string(raw), "cảnh 3 của vở kịch") {
		t.Fatalf("văn xuôi có chữ 'cảnh' phải được giữ nguyên: %s", raw)
	}
	if !strings.Contains(string(raw), "Bối cảnh: hội trường thiên cơ, buổi sáng.") {
		t.Fatalf("upcoming_outline phải được净化 cùng: %s", raw)
	}
	if strings.Contains(string(raw), "CẢNH 2") || strings.Contains(string(raw), "CẢNH 1:") {
		t.Fatalf("payload còn tiêu đề kịch bản: %s", raw)
	}
	plan := working["chapter_plan"].(map[string]any)
	if goal, _ := plan["goal"].(string); strings.Contains(goal, "CẢNH 1") {
		t.Fatalf("chapter_plan.goal còn tiêu đề kịch bản: %q", goal)
	}
	contract := plan["contract"].(map[string]any)
	beats, _ := contract["required_beats"].([]any)
	if len(beats) != 1 || strings.Contains(beats[0].(string), "CẢNH 1") {
		t.Fatalf("required_beats còn tiêu đề kịch bản: %v", beats)
	}

	stored, err := s.Outline.LoadOutline()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored[0].CoreEvent, "CẢNH 1") {
		t.Fatalf("outline.json phải giữ nguyên: %q", stored[0].CoreEvent)
	}
}
