package tools_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/assets"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/library"
	"github.com/datnt1998/noverse/internal/llmcontract"
	"github.com/datnt1998/noverse/internal/store"
	"github.com/datnt1998/noverse/internal/tools"
)

func retentionStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	return s
}

func retentionArgs(t *testing.T, value any) json.RawMessage {
	t.Helper()
	// Các phép thử giữ chân đi qua cửa sổ cái thật khi có truyện trước.
	// Cấp đủ mười trục để không bị cửa đó che lỗi đang kiểm.
	if fields, ok := value.(map[string]any); ok && fields["type"] == "premise" && fields["axes"] == nil {
		fields["axes"] = []string{"A showbiz", "Đoàn phim", "Biên kịch", "Bạn diễn", "Mẹ già", "Bản ghi âm gốc", "Livestream", "Mỏ hỗn sắc", "Câu thoại bất công", "Sự nghiệp riêng"}
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func retentionPack(t *testing.T, name string) *assets.Pack {
	t.Helper()
	bundle := assets.Load(name, assets.LoadOptions{})
	if bundle.Err != nil {
		t.Fatal(bundle.Err)
	}
	if bundle.Pack == nil {
		t.Fatal("embedded pack missing")
	}
	return bundle.Pack
}

func TestRetentionRoundTrip(t *testing.T) {
	s := retentionStore(t, t.TempDir())
	foundation := tools.NewSaveFoundationTool(s)
	ctx := context.Background()
	old := json.RawMessage(`[{"chapter":1,"title":"Cũ","core_event":"Bị hại","hook":"Ai đến?","scenes":[]}]`)
	if _, err := foundation.Execute(ctx, retentionArgs(t, map[string]any{"type": "outline", "axes": nil, "content": old})); err != nil {
		t.Fatal(err)
	}
	entries, err := s.Outline.LoadOutline()
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(retentionArgs(t, entries))
	for _, key := range []string{"open_questions", "promise", "hook_type", "midpoint_twist"} {
		if strings.Contains(encoded, key) {
			t.Fatalf("old outline unexpectedly includes %s: %s", key, encoded)
		}
	}
	entry := domain.OutlineEntry{Chapter: 1, Title: "Mới", CoreEvent: "Đối chất", Hook: "Người đứng sau lộ diện", Scenes: []string{"Tòa án"}, OpenQuestions: []string{"Ai giấu thư?"}, Promise: "da_tra: cô thắng tại tòa", HookType: "lat_thong_tin", MidpointTwist: true}
	if _, err := foundation.Execute(ctx, retentionArgs(t, map[string]any{"type": "outline", "axes": nil, "content": []domain.OutlineEntry{entry}})); err != nil {
		t.Fatal(err)
	}
	entries, err = s.Outline.LoadOutline()
	if err != nil || len(entries) != 1 || !reflect.DeepEqual(entries[0], entry) {
		t.Fatalf("outline round trip=%+v, err=%v", entries, err)
	}
	invalid := entry
	invalid.HookType = "mystery"
	if _, err := foundation.Execute(ctx, retentionArgs(t, map[string]any{"type": "outline", "axes": nil, "content": []domain.OutlineEntry{invalid}})); err == nil || !strings.Contains(err.Error(), "hook_type") {
		t.Fatalf("new outline must reject old hook types: %v", err)
	}
	p, err := s.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	p.Phase = domain.PhaseWriting
	if err := s.Progress.Save(p); err != nil {
		t.Fatal(err)
	}
	revise := tools.NewReviseOutlineTool(s)
	withoutRetention := json.RawMessage(`{"from_chapter":1,"reconcile_completed":null,"reason":"Giữ quy hoạch cũ","replacement":[{"title":"Cũ","core_event":"Bị hại","hook":"Ai đến?","scenes":[],"open_questions":null,"promise":null,"hook_type":null,"midpoint_twist":null}]}`)
	if err := llmcontract.ValidateJSON(revise.Schema(), withoutRetention); err != nil {
		t.Fatalf("strict optional retention fields must accept null: %v", err)
	}
	if _, err := revise.Execute(ctx, withoutRetention); err != nil {
		t.Fatal(err)
	}
	entry.Promise = "da_tra: cô lấy lại cửa tiệm"
	if _, err := tools.NewReviseOutlineTool(s).Execute(ctx, retentionArgs(t, map[string]any{"from_chapter": 1, "replacement": []domain.OutlineEntry{entry}, "reason": "Đổi cảnh trả lời hứa"})); err != nil {
		t.Fatal(err)
	}
	entries, err = s.Outline.LoadOutline()
	if err != nil || !reflect.DeepEqual(entries[0], entry) {
		t.Fatalf("revised round trip=%+v, err=%v", entries, err)
	}
	planArgs := map[string]any{"chapter": 1, "title": entry.Title, "goal": "Thắng kiện", "conflict": "Mất cửa tiệm", "hook": entry.Hook, "open_questions": entry.OpenQuestions, "promise": entry.Promise, "hook_type": entry.HookType, "midpoint_twist": true}
	if err := llmcontract.ValidateJSON(tools.NewPlanChapterTool(s).Schema(), retentionArgs(t, planArgs)); err != nil {
		t.Fatalf("plan schema rejected retention fields: %v", err)
	}
	if _, err := tools.NewPlanChapterTool(s).Execute(ctx, retentionArgs(t, planArgs)); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Drafts.LoadChapterPlan(1)
	if err != nil || plan == nil || plan.Promise != entry.Promise || plan.HookType != entry.HookType || !plan.MidpointTwist || !reflect.DeepEqual(plan.OpenQuestions, entry.OpenQuestions) {
		t.Fatalf("plan round trip=%+v, err=%v", plan, err)
	}
}

func TestPackRetentionFoundation(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"nu-tan-bao-thu", "nguoc-hon-nhan", "nien-dai-van"} {
		t.Run(name, func(t *testing.T) {
			s := retentionComparisonStore(t)
			pack := retentionPack(t, name)
			tool := tools.NewSaveFoundationTool(s, pack)
			if _, err := tool.Execute(ctx, retentionArgs(t, map[string]any{"type": "premise", "axes": nil, "content": "# Tiền đề cốt truyện\n\nMột người trở về.", "diem_khac": "Nữ thợ sửa đồng hồ cứu cả làng"})); err != nil {
				t.Fatal(err)
			}
			fields, err := s.Outline.LoadStoryRetention()
			if err != nil {
				t.Fatal(err)
			}
			wantAudience, _ := domain.AudienceIndices(pack.NguoiNghe)
			if name == "nien-dai-van" {
				wantAudience = nil
			}
			if !reflect.DeepEqual(fields.NguoiNghe, wantAudience) {
				t.Fatalf("default audience=%v, want %v", fields.NguoiNghe, wantAudience)
			}
			old := json.RawMessage(`[{"chapter":1,"title":"Mở","core_event":"Trở về","hook":"Ai giữ thư?","scenes":[]}]`)
			_, err = tool.Execute(ctx, retentionArgs(t, map[string]any{"type": "outline", "axes": nil, "content": old}))
			for _, key := range []string{"open_questions", "promise", "hook_type", "midpoint_twist"} {
				if err == nil || !strings.Contains(err.Error(), key) {
					t.Fatalf("missing %s was not reported: %v", key, err)
				}
			}
			valid := json.RawMessage(`[{"chapter":1,"title":"Mở","core_event":"Trở về","hook":"Ai giữ thư?","scenes":[],"open_questions":[],"promise":"đếm ngược","hook_type":"lat_thong_tin","midpoint_twist":false}]`)
			if name == "nien-dai-van" {
				_, err := tool.Execute(ctx, retentionArgs(t, map[string]any{"type": "outline", "axes": nil, "content": valid}))
				if err == nil || !strings.Contains(err.Error(), "nguoi_nghe") {
					t.Fatalf("compound audience must require architect choice: %v", err)
				}
				if _, err := tool.Execute(ctx, retentionArgs(t, map[string]any{"type": "premise", "axes": nil, "content": "# Tiền đề", "nguoi_nghe": []int{3}})); err == nil {
					t.Fatal("compound pack accepted audience not named in pack")
				}
				if _, err := tool.Execute(ctx, retentionArgs(t, map[string]any{"type": "premise", "axes": nil, "content": "# Tiền đề", "nguoi_nghe": []int{1}})); err != nil {
					t.Fatal(err)
				}
			}
			saved, err := tool.Execute(ctx, retentionArgs(t, map[string]any{"type": "outline", "axes": nil, "content": valid}))
			if err != nil {
				t.Fatalf("false midpoint, empty final questions, custom hook should be accepted: %v", err)
			}
			if !strings.Contains(string(saved), "hat_giong_phan_hai") {
				t.Fatalf("missing sequel seed should warn, not block: %s", saved)
			}
			for _, content := range []json.RawMessage{
				json.RawMessage(`[{}]`),
				json.RawMessage(`[{"chapter":1,"core_event":"Trở về","estimated_chapters":8}]`),
			} {
				if _, err := tool.Execute(ctx, retentionArgs(t, map[string]any{"type": "outline", "axes": nil, "content": content})); err == nil || !strings.Contains(err.Error(), "midpoint_twist") {
					t.Fatalf("sparse chapter bypassed required fields: %v", err)
				}
			}
			layered := json.RawMessage(`[{"index":1,"title":"Tập","arcs":[{"index":1,"title":"Cung","chapters":[{"chapter":1,"title":"Mở","core_event":"Trở về"}]}]}]`)
			if _, err := tool.Execute(ctx, retentionArgs(t, map[string]any{"type": "layered_outline", "axes": nil, "content": layered})); err == nil || !strings.Contains(err.Error(), "hook_type") {
				t.Fatalf("layered chapters bypassed required fields: %v", err)
			}
			if _, err := tools.NewReviseOutlineTool(s, pack).Execute(ctx, retentionArgs(t, map[string]any{"from_chapter": 1, "replacement": old, "reason": "Bổ sung móc"})); err == nil || !strings.Contains(err.Error(), "open_questions") {
				t.Fatalf("revise bypassed required fields: %v", err)
			}
			if _, err := tool.Execute(ctx, retentionArgs(t, map[string]any{"type": "premise", "axes": nil, "content": "# Tiền đề", "diem_khac": ""})); err != nil {
				t.Fatal(err)
			}
			_, err = tool.Execute(ctx, retentionArgs(t, map[string]any{"type": "outline", "axes": nil, "content": valid}))
			if err == nil || !strings.Contains(err.Error(), "diem_khac") {
				t.Fatalf("missing story difference not reported: %v", err)
			}
		})
	}
}

func TestRetentionArchitectSiblingContext(t *testing.T) {
	root := t.TempDir()
	lib, err := library.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.CreateProject("du-an", library.ProjectFile{Name: "Cùng kênh"}); err != nil {
		t.Fatal(err)
	}
	current, _, err := lib.CreateStory("du-an", "current", "")
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := lib.CreateStory("du-an", "other", "")
	if err != nil {
		t.Fatal(err)
	}
	currentStore := retentionStore(t, filepath.Join(current.Dir, "output", "novel"))
	otherStore := retentionStore(t, filepath.Join(other.Dir, "output", "novel"))
	if err := currentStore.Book.Save(domain.BookMetadata{Title: "Không được đưa chính mình", Synopsis: "Truyện hiện tại"}); err != nil {
		t.Fatal(err)
	}
	if err := otherStore.Book.Save(domain.BookMetadata{Title: "Tiệm đồng hồ", Synopsis: "Người thợ tìm lại gia đình"}); err != nil {
		t.Fatal(err)
	}
	if err := otherStore.Outline.SavePremise("# Tiền đề\n\nNgười thợ giữ lại lá thư."); err != nil {
		t.Fatal(err)
	}
	if err := currentStore.Outline.SaveStoryRetention(domain.StoryRetention{NguoiNghe: []int{1}, DiemKhac: "Nghề mới"}); err != nil {
		t.Fatal(err)
	}
	tool := tools.NewContextTool(currentStore, tools.References{}, "", tools.NewStyleStatsIndex(currentStore))
	data, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Foundation struct {
			Others    []struct{ Title, Premise string } `json:"other_stories"`
			Retention domain.StoryRetention             `json:"story_retention"`
		} `json:"foundation_memory"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Foundation.Others) != 1 || payload.Foundation.Others[0].Title != "Tiệm đồng hồ" || !strings.Contains(payload.Foundation.Others[0].Premise, "lá thư") {
		t.Fatalf("siblings=%+v", payload.Foundation.Others)
	}
	if payload.Foundation.Retention.DiemKhac != "Nghề mới" {
		t.Fatalf("retention=%+v", payload.Foundation.Retention)
	}
}

func TestStoryFieldsSchemaAndNormalization(t *testing.T) {
	pack := retentionPack(t, "nu-tan-bao-thu")
	for _, audience := range []any{nil, []int{}} {
		s := retentionComparisonStore(t)
		tool := tools.NewSaveFoundationTool(s, pack)
		args := map[string]any{"type": "premise", "axes": nil, "content": "# Tiền đề", "nguoi_nghe": audience, "diem_khac": "Không có other_stories để so sánh."}
		if _, err := tool.Execute(context.Background(), retentionArgs(t, args)); err != nil {
			t.Fatal(err)
		}
		fields, err := s.Outline.LoadStoryRetention()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(fields.NguoiNghe, []int{1}) || fields.DiemKhac != "" {
			t.Errorf("prefill and empty comparison = %+v, want audience [1], empty difference", fields)
		}
		props := tool.Schema()["properties"].(map[string]any)
		description := props["diem_khac"].(map[string]any)["description"].(string)
		if !strings.Contains(description, "khóa nội bộ") || !strings.Contains(description, "để trống") {
			t.Errorf("difference schema must forbid internal keys and allow no comparison: %s", description)
		}
	}
	t.Run("first story needs no comparison", func(t *testing.T) {
		lib, err := library.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := lib.CreateProject("du-an", library.ProjectFile{Name: "Dự án"}); err != nil {
			t.Fatal(err)
		}
		current, _, err := lib.CreateStory("du-an", "truyen", "")
		if err != nil {
			t.Fatal(err)
		}
		s := retentionStore(t, filepath.Join(current.Dir, "output", "novel"))
		tool := tools.NewSaveFoundationTool(s, pack)
		if _, err := tool.Execute(context.Background(), retentionArgs(t, map[string]any{"type": "premise", "axes": nil, "content": "# Tiền đề", "diem_khac": "Không được giả làm so sánh khi chưa có truyện trước"})); err != nil {
			t.Fatal(err)
		}
		fields, err := s.Outline.LoadStoryRetention()
		if err != nil || fields.DiemKhac != "" {
			t.Fatalf("first story comparison must stay empty: %+v, %v", fields, err)
		}
		outline := json.RawMessage(`[{"chapter":1,"title":"Mở","core_event":"Trở về","hook":"Ai giữ thư?","scenes":[],"open_questions":[],"promise":"đếm ngược","hook_type":"lat_thong_tin","midpoint_twist":false}]`)
		if _, err := tool.Execute(context.Background(), retentionArgs(t, map[string]any{"type": "outline", "axes": nil, "content": outline})); err != nil {
			t.Fatalf("first story must not require invented difference: %v", err)
		}
	})
}

func retentionComparisonStore(t *testing.T) *store.Store {
	t.Helper()
	lib, err := library.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.CreateProject("du-an", library.ProjectFile{Name: "Dự án"}); err != nil {
		t.Fatal(err)
	}
	current, _, err := lib.CreateStory("du-an", "truyen", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := lib.CreateStory("du-an", "truyen-truoc", ""); err != nil {
		t.Fatal(err)
	}
	return retentionStore(t, filepath.Join(current.Dir, "output", "novel"))
}
