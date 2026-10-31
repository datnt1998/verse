package web

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/lessons"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/userrules"
)

func lessonGatewayFixture(t *testing.T) *configTestGateway {
	t.Helper()
	tg := newConfigTestGateway(t)
	for i := range 3 {
		dir := filepath.Join(tg.project, "kich_ban", fmt.Sprintf("%02d-story", i+1))
		io := store.NewIO(dir)
		if err := io.WriteJSON("story.json", map[string]string{"title": fmt.Sprintf("Truyện %d", i+1)}); err != nil {
			t.Fatal(err)
		}
		if err := io.WriteJSON(".ainovel/config.json", map[string]string{"style": "nu-tan-bao-thu"}); err != nil {
			t.Fatal(err)
		}
		st := store.NewStore(filepath.Join(dir, "output", "novel"))
		if err := st.Characters.Save([]domain.Character{{Name: "Lâm Trần", Aliases: []string{"Trần thiếu gia"}}}); err != nil {
			t.Fatal(err)
		}
		rv := domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "rewrite", Issues: []domain.ConsistencyIssue{{Type: "hook", Severity: "error", RequiresChange: true, RuleRef: "Master §7", Description: "Lâm Trần đi về nhà", Evidence: "Lâm Trần mang chiếc nhẫn rơi xuống bậc thang trước cửa", Suggestion: "Cho Lâm Trần nhận tin nhắn"}}}
		if err := st.World.SaveReview(rv); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			rv.Summary = "Lần chấm sau"
			if err := st.World.SaveReview(rv); err != nil {
				t.Fatal(err)
			}
		}
	}
	return tg
}

func lessonResponse(t *testing.T, tg *configTestGateway, cookie string, state string) []lessons.Lesson {
	t.Helper()
	res := tg.do("GET", "/api/system/lessons?state="+state, cookie, "")
	if res.Code != 200 {
		t.Fatalf("lessons list: %d %s", res.Code, res.Body.String())
	}
	var body struct {
		Lessons []lessons.Lesson `json:"lessons"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Lessons
}

func TestSystemLessonsPermissions(t *testing.T) {
	tg := newConfigTestGateway(t)
	for _, role := range []string{"reader", "writer"} {
		cookie := tg.login(role, role+"-pass")
		for _, route := range []struct{ method, path, body string }{{"GET", "/api/system/lessons", ""}, {"GET", "/api/system/violations?pack=A", ""}, {"POST", "/api/system/lessons/unknown", `{"action":"keep"}`}} {
			if res := tg.do(route.method, route.path, cookie, route.body); res.Code != 403 {
				t.Errorf("%s %s %s = %d; want 403", role, route.method, route.path, res.Code)
			}
		}
		if body := decodeResp(t, tg.do("GET", "/api/me", cookie, "")); body["pendingLessons"] != nil {
			t.Fatalf("non-admin count exposed: %v", body)
		}
	}
}

// Câu 15 · endpoint: mục Master nhóm qua mọi gói (một bài học, 2 gói), mục Bộ A / Bộ B ở lại từng gói;
// bảng vi phạm vẫn đếm theo gói đang xem và chuông cần Bắt sửa ở ≥ 3 truyện của gói đó.
func TestSystemLessonsCrossPackGrouping(t *testing.T) {
	tg := newConfigTestGateway(t)
	for _, story := range []struct{ name, pack, ref, level string }{
		{"01-a1", "nu-tan-bao-thu", "Master §4", "error"},
		{"02-a2", "nu-tan-bao-thu", "Master §4", "error"},
		{"03-a3", "nu-tan-bao-thu", "Master §4", "warning"},
		{"04-b1", "nguoc-hon-nhan", "Master §4", "error"},
		{"05-a4", "nu-tan-bao-thu", "Bộ A §4.3", "error"},
		{"06-a5", "nu-tan-bao-thu", "Bộ A §4.3", "error"},
		{"07-b2", "nguoc-hon-nhan", "Bộ B §4.3", "error"},
		{"08-b3", "nguoc-hon-nhan", "Bộ B §4.3", "error"},
		{"09-a6", "nu-tan-bao-thu", "Xưng hô", "error"},
		{"10-b4", "nguoc-hon-nhan", "Xưng hô", "warning"},
		{"11-a7", "nu-tan-bao-thu", "Master §4", "error"},
	} {
		dir := filepath.Join(tg.project, "kich_ban", story.name)
		io := store.NewIO(dir)
		if err := io.WriteJSON("story.json", map[string]string{"title": "Truyện " + story.name}); err != nil {
			t.Fatal(err)
		}
		if err := io.WriteJSON(".ainovel/config.json", map[string]string{"style": story.pack}); err != nil {
			t.Fatal(err)
		}
		st := store.NewStore(filepath.Join(dir, "output", "novel"))
		rv := domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "rewrite", Issues: []domain.ConsistencyIssue{{Type: "hook", Severity: story.level,
			RequiresChange: story.level == "error", RuleRef: story.ref, Description: "Nhịp chương chưa đúng", Evidence: "trích nguyên văn", Suggestion: "sửa lại nhịp"}}}
		if err := st.World.SaveReview(rv); err != nil {
			t.Fatal(err)
		}
	}
	admin := tg.login("admin", "admin-pass")
	list := lessonResponse(t, tg, admin, "pending")
	if len(list) != 4 {
		t.Fatalf("cross-pack grouping produced %d lessons: %+v", len(list), list)
	}
	byRef := map[string]lessons.Lesson{}
	for _, l := range list {
		byRef[l.Refs[0]] = l
	}
	shared, ok := byRef["Master §4"]
	if !ok {
		t.Fatalf("no lesson for Master §4: %+v", list)
	}
	if shared.Scope != "all" || shared.Evidence.PacksCount != 2 || shared.Evidence.Pack != "" || shared.Evidence.Stories != 5 || shared.Evidence.Errors != 4 || shared.Evidence.Warnings != 1 {
		t.Fatalf("Master §4 must be one cross-pack lesson: %+v", shared)
	}
	if shared.ID != lessons.ID(lessons.GroupKey(shared.PackID, "Master §4")) {
		t.Fatalf("lesson id is not the merged group key: %s", shared.ID)
	}
	for _, ref := range []string{"Bộ A §4.3", "Bộ B §4.3"} {
		l, ok := byRef[ref]
		if !ok || l.Scope != "pack" || l.Evidence.PacksCount != 1 || l.Evidence.Stories != 2 {
			t.Fatalf("%s must stay a lesson of its own pack: %+v", ref, l)
		}
	}
	if byRef["Bộ A §4.3"].PackID == byRef["Bộ B §4.3"].PackID {
		t.Fatalf("Bộ A merged into Bộ B's pack: %+v", list)
	}
	// Mục xưng hô không có trong tệp luật của gói vẫn thành bài học, tên mục lấy cố định
	// (“Bảng xưng hô của truyện”), không lấy chiều gọi vì có tên nhân vật.
	address, ok := byRef["Xưng hô"]
	if !ok || address.Scope != "all" || address.Evidence.PacksCount != 2 || address.Evidence.Errors != 1 || address.Evidence.Warnings != 1 {
		t.Fatalf("Xưng hô must group across packs without names: %+v", list)
	}
	if !strings.Contains(address.Text, "Bảng xưng hô của truyện") {
		t.Fatalf("address lesson text: %s", address.Text)
	}
	body := decodeResp(t, tg.do("GET", "/api/system/violations?pack=A", admin, ""))
	rows := map[string]map[string]any{}
	for _, row := range body["rows"].([]any) {
		m := row.(map[string]any)
		rows[m["ref"].(string)] = m
	}
	// Gói A có 7 truyện; mục dùng chung chỉ đếm truyện của gói A dù bài học gộp qua 2 gói.
	// Chuông của mục dùng chung tính trên mọi gói: 4 Bắt sửa (a1, a2, a7 ở A + b1 ở B).
	if row := rows["Master §4"]; row["stories"] != float64(4) || row["total"] != float64(7) || row["level"] != "error" || row["alert"] != true || row["alertScope"] != "all" || row["alertStories"] != float64(4) {
		t.Fatalf("gói A · Master §4 row: %+v", body)
	}
	// Mục `Bộ …` giữ chuông theo gói và không mang phạm vi chuông.
	if row := rows["Bộ A §4.3"]; row["stories"] != float64(2) || row["alert"] != false || row["alertScope"] != nil || row["alertStories"] != nil {
		t.Fatalf("gói A · Bộ A §4.3 row: %+v", body)
	}
	if row := rows["Xưng hô"]; row["stories"] != float64(1) || row["refName"] != "Bảng xưng hô của truyện" || row["alertScope"] != "all" || row["alertStories"] != float64(1) {
		t.Fatalf("gói A · Xưng hô row: %+v", body)
	}
	other := decodeResp(t, tg.do("GET", "/api/system/violations?pack=B", admin, ""))
	for _, row := range other["rows"].([]any) {
		m := row.(map[string]any)
		if m["ref"] == "Master §4" && (m["stories"] != float64(1) || m["total"] != float64(4) || m["alert"] != true || m["alertScope"] != "all" || m["alertStories"] != float64(4)) {
			t.Fatalf("gói B · Master §4 row (bell over every pack): %+v", other)
		}
	}
}

func TestSystemLessonsAggregationPrivacyAndLifecycle(t *testing.T) {
	tg := lessonGatewayFixture(t)
	admin := tg.login("admin", "admin-pass")
	list := lessonResponse(t, tg, admin, "pending")
	if len(list) != 1 {
		t.Fatalf("expected one repeated rule lesson, got %+v", list)
	}
	l := list[0]
	if strings.Contains(l.Text, "Lâm Trần") || strings.Contains(l.Text, "chiếc nhẫn") || !strings.Contains(l.Text, "Master §7") {
		t.Fatalf("unsafe/non-rule lesson text: %s", l.Text)
	}
	if l.Evidence.Errors != 4 || l.Evidence.Stories != 3 || len(l.Evidence.Chips) != 3 {
		t.Fatalf("rounds/evidence aggregation: %+v", l.Evidence)
	}
	// Mục Master không thuộc gói nào: dòng nguồn luôn là “· 1 gói”, không mang tên gói (câu 4).
	if l.Scope != "all" || l.Evidence.PacksCount != 1 || l.Evidence.Pack != "" {
		t.Fatalf("shared ref must report its pack count without a pack name: scope=%q %+v", l.Scope, l.Evidence)
	}
	body := decodeResp(t, tg.do("GET", "/api/system/violations?pack=A", admin, ""))
	rows := body["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["stories"] != float64(3) || rows[0].(map[string]any)["total"] != float64(3) || rows[0].(map[string]any)["alert"] != true || rows[0].(map[string]any)["alertScope"] != "all" || rows[0].(map[string]any)["alertStories"] != float64(3) {
		t.Fatalf("violations aggregation: %v", body)
	}
	if body := decodeResp(t, tg.do("GET", "/api/me", admin, "")); body["pendingLessons"] != float64(1) {
		t.Fatalf("admin pending count: %v", body)
	}
	base := "/api/system/lessons/" + l.ID
	for _, text := range []string{"Không để Lâm Trần. nghỉ ở cuối chương", "Không để Trần thiếu gia nghỉ ở cuối chương", "chiếc nhẫn rơi xuống bậc thang trước cửa"} {
		payload, _ := json.Marshal(map[string]string{"action": "edit", "text": text})
		if res := tg.do("POST", base, admin, string(payload)); res.Code != 400 {
			t.Fatalf("accepted story-specific edit: %d %s", res.Code, res.Body.String())
		}
	}
	// Captured before keep: even another planning/reapply cannot mutate this story.
	old := store.NewStore(t.TempDir())
	if _, err := userrules.LessonContext(old, l.PackID); err != nil {
		t.Fatal(err)
	}
	if res := tg.do("POST", base, admin, `{"action":"keep","scope":"pack"}`); res.Code != 200 {
		t.Fatalf("keep: %d %s", res.Code, res.Body.String())
	}
	kept := lessonResponse(t, tg, admin, "kept")
	if len(kept) != 1 || kept[0].Scope != "pack" || kept[0].DecidedAt == nil {
		t.Fatalf("kept: %+v", kept)
	}
	ls, err := lessons.Open()
	if err != nil {
		t.Fatal(err)
	}
	packRules, err := ls.Rules(l.PackID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range packRules {
		if r.Preferences == l.Text && r.Source == "pack:"+l.PackID+":lessons" {
			found = true
		}
	}
	if !found {
		t.Fatalf("keep did not write pack layer: %+v", packRules)
	}
	other, err := ls.Rules("nguoc-hon-nhan")
	if err != nil || len(other) != 0 {
		t.Fatalf("pack rule leaked: %+v %v", other, err)
	}
	fresh := store.NewStore(t.TempDir())
	ctx, err := userrules.LessonContext(fresh, l.PackID)
	if err != nil || !strings.Contains(ctx, l.Text) {
		t.Fatalf("Architect missing kept lesson: %q %v", ctx, err)
	}
	if !strings.Contains(ctx, body["preamble"].(string)) {
		t.Fatalf("Architect preamble differs from system display: %q vs %v", ctx, body["preamble"])
	}
	svc := userrules.NewService(fresh, nil, rules.LoadOptions{})
	snap, err := svc.Build(context.Background(), "")
	if err != nil || !strings.Contains(snap.Preferences, l.Text) {
		t.Fatalf("rule snapshot missing lesson: %+v %v", snap, err)
	}
	oldCtx, err := userrules.LessonContext(old, l.PackID)
	if err != nil || strings.Contains(oldCtx, l.Text) {
		t.Fatalf("existing story changed: %q %v", oldCtx, err)
	}
	if res := tg.do("POST", base, admin, `{"action":"undo"}`); res.Code != 200 {
		t.Fatalf("undo keep: %d %s", res.Code, res.Body.String())
	}
	unkept, err := ls.Rules(l.PackID)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range unkept {
		if r.Preferences == l.Text {
			t.Fatal("undo left kept rule in layer")
		}
	}
	if res := tg.do("POST", base, admin, `{"action":"keep","scope":"all"}`); res.Code != 200 {
		t.Fatalf("keep all: %d %s", res.Code, res.Body.String())
	}
	all, err := ls.Rules("nguoc-hon-nhan")
	if err != nil || len(all) != 1 || all[0].Source != "default:lessons" {
		t.Fatalf("system default lesson: %+v %v", all, err)
	}
	effective, err := userrules.NewFileRules(nil, rules.LoadOptions{HomeRulesDir: filepath.Join(tg.home, ".ainovel", "rules")}, "").Effective(context.Background())
	if err != nil || !strings.Contains(effective.Snapshot.Preferences, l.Text) {
		t.Fatalf("system default layer missing kept lesson: %+v %v", effective, err)
	}
	if res := tg.do("POST", base, admin, `{"action":"undo"}`); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if res := tg.do("POST", base, admin, `{"action":"drop"}`); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if pending := lessonResponse(t, tg, admin, "pending"); len(pending) != 0 {
		t.Fatalf("dropped lesson proposed again: %+v", pending)
	}
	if res := tg.do("POST", base, admin, `{"action":"undo"}`); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if pending := lessonResponse(t, tg, admin, "pending"); len(pending) != 1 {
		t.Fatalf("undo drop: %+v", pending)
	}
}
