package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/diag"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/identity"
)

// storyWithPack ghi cấu hình truyện chọn gói thể loại (khoá style).
func storyWithPack(t *testing.T, st library.Story, style string) {
	t.Helper()
	dir := filepath.Join(st.Dir, ".ainovel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"style":"` + style + `"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// retentionOutline ghi đại cương 8 chương có đủ trường bản đồ giữ chân:
// lời hứa chương 3 (37,5%), cú lật chương 4 (50%), ba chương 5–6–7 cùng kiểu móc,
// chương 1 chỉ một câu hỏi treo, chương cuối được miễn.
func retentionOutline(t *testing.T, s *store.Store) {
	t.Helper()
	entries := make([]domain.OutlineEntry, 0, 8)
	for i := 1; i <= 8; i++ {
		entry := domain.OutlineEntry{
			Chapter:   i,
			Title:     "Chương",
			CoreEvent: "sự kiện chính",
			Hook:      "người nghe chờ gì",
			Scenes:    []string{},
		}
		entry.OpenQuestions = []string{"Ai đứng sau?", "Chiếc cặp đựng gì?"}
		if i == 1 {
			entry.OpenQuestions = []string{"Ai đổi tên tác giả?"}
		}
		if i == 8 {
			entry.OpenQuestions = []string{"Mọi câu hỏi đã trả lời chưa?"}
		}
		switch i {
		case 1:
			entry.HookType = "lat_thong_tin"
		case 2:
			entry.HookType = "cau_thoai_no"
		case 3:
			entry.Promise = "da_tra: Tri Hạ nối đúng câu bị ngắt trên live"
			entry.CoreEvent = "Cảnh lời hứa"
			entry.HookType = "quyet_dinh_treo"
		case 4:
			entry.MidpointTwist = true
			entry.CoreEvent = "Phùng Chấn đặt hàng vụ cướp kịch bản"
			entry.HookType = "lat_thong_tin"
		case 5, 6, 7:
			entry.HookType = "de_doa_moi"
		case 8:
			entry.HookType = "cau_thoai_no"
		}
		entries = append(entries, entry)
	}
	if err := s.Outline.SaveOutline(entries); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(len(entries)); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
}

func decodeFrame(t *testing.T, rec *httptest.ResponseRecorder) frameResponse {
	t.Helper()
	var out frameResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode frame: %v\n%s", err, rec.Body.String())
	}
	return out
}

// TestFramePackPrefillAndSource: chưa lưu gì → nhóm người nghe điền từ gói, nguồn "pack".
func TestFramePackPrefillAndSource(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, nil)
	storyWithPack(t, st, "nu-tan-bao-thu")

	rec := storyReq(t, g, http.MethodGet, "/x/frame", adminUser(), st, "/frame", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("frame: %d %s", rec.Code, rec.Body)
	}
	frame := decodeFrame(t, rec)
	if frame.Source != "pack" {
		t.Fatalf("source = %q, muốn pack", frame.Source)
	}
	if frame.Audience == nil || len(frame.Audience) != 1 || frame.Audience[0] != 1 {
		t.Fatalf("audience = %v, muốn [1] (Nữ 18–35 của gói)", frame.Audience)
	}
	if len(frame.Audiences) != 5 || frame.Audiences[0].Wants == "" || frame.Audiences[0].Label == "" {
		t.Fatalf("audiences = %+v", frame.Audiences)
	}
}

// TestFramePutValidatesAndSaves: PUT hợp lệ lưu đúng trường, nguồn thành "user";
// audience ngoài 0..4 và ô chữ nhiều dòng / quá 200 ký tự bị 400.
func TestFramePutValidatesAndSaves(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, nil)
	storyWithPack(t, st, "nu-tan-bao-thu")
	// Có truyện trước thì Điểm khác là nội dung thật; truyện đầu hiển thị trạng thái rỗng.
	if err := os.WriteFile(filepath.Join(st.ProjectDir, library.ProjectFileName), []byte(`{"name":"Dự án"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(st.ProjectDir, "kich_ban", "truyen-truoc"), 0o755); err != nil {
		t.Fatal(err)
	}

	rec := storyReq(t, g, http.MethodPut, "/x/frame", adminUser(), st, "/frame",
		`{"audience":[2],"difference":"Nghề lồng tiếng","sequelSeed":"Ai gửi lại kịch bản?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT frame: %d %s", rec.Code, rec.Body)
	}
	frame := decodeFrame(t, rec)
	if frame.Source != "user" || frame.Difference != "Nghề lồng tiếng" || frame.SequelSeed != "Ai gửi lại kịch bản?" {
		t.Fatalf("frame sau PUT = %+v", frame)
	}
	if len(frame.Audience) != 1 || frame.Audience[0] != 2 {
		t.Fatalf("audience = %v, muốn [2]", frame.Audience)
	}
	stored, err := s.Outline.LoadStoryRetention()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stored.NguoiNghe, []int{2}) || stored.DiemKhac != "Nghề lồng tiếng" {
		t.Fatalf("story_retention = %+v", stored)
	}
	// Đọc lại lần hai vẫn giữ nguồn "user" (dấu nằm trong meta, không mất khi khởi động lại).
	if again := decodeFrame(t, storyReq(t, g, http.MethodGet, "/x/frame", adminUser(), st, "/frame", "")); again.Source != "user" {
		t.Fatalf("nguồn sau khi đọc lại = %q", again.Source)
	}

	for _, body := range []string{
		`{"audience":[5]}`,
		`{"audience":[-1]}`,
		`{"audience":[1,1]}`,
		`{"difference":"dòng một\ndòng hai"}`,
		`{"sequelSeed":"` + longText(201) + `"}`,
	} {
		rec := storyReq(t, g, http.MethodPut, "/x/frame", adminUser(), st, "/frame", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("PUT %s → %d, muốn 400 (%s)", body, rec.Code, rec.Body)
		}
	}
}

// TestFramePermissions: người chỉ đọc xem được；写请求在路由级门禁被 404 隐藏。
func TestFramePermissions(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, nil)
	storyWithPack(t, st, "nu-tan-bao-thu")
	reader := identity.User{Username: "reader", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{st.Project: identity.ProjectView}}

	if rec := storyReq(t, g, http.MethodGet, "/x/frame", reader, st, "/frame", ""); rec.Code != http.StatusOK {
		t.Errorf("reader GET frame: %d %s", rec.Code, rec.Body)
	}
	rec := storyReq(t, g, http.MethodPut, "/x/frame", reader, st, "/frame", `{"difference":"x"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("reader PUT frame: %d, muốn 404", rec.Code)
	}
}

// TestRetentionMapAndChecks: vị trí lời hứa/cú lật, bốn dòng kiểm, móc thứ ba bị báo,
// chương 1 thiếu câu hỏi treo, chương cuối được miễn.
func TestRetentionMapAndChecks(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, nil)
	retentionOutline(t, s)
	for _, n := range []int{1, 2, 3} {
		if err := s.Drafts.SaveFinalChapter(n, "nội dung chương"); err != nil {
			t.Fatal(err)
		}
		if err := s.Progress.StartChapter(n); err != nil {
			t.Fatal(err)
		}
		if err := s.Progress.MarkChapterComplete(n, 3, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Progress.StartChapter(4); err != nil {
		t.Fatal(err)
	}

	rec := storyReq(t, g, http.MethodGet, "/x/retention", adminUser(), st, "/retention", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("retention: %d %s", rec.Code, rec.Body)
	}
	var out retentionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, rec.Body.String())
	}
	if len(out.Chapters) != 8 {
		t.Fatalf("chapters = %d, muốn 8", len(out.Chapters))
	}
	if out.Chapters[0].Status != "written" || out.Chapters[3].Status != "writing" || out.Chapters[4].Status != "unwritten" {
		t.Fatalf("status = %q / %q / %q, muốn written / writing / unwritten",
			out.Chapters[0].Status, out.Chapters[3].Status, out.Chapters[4].Status)
	}
	if out.PromiseAt == nil || *out.PromiseAt != 0.375 {
		t.Fatalf("promiseAt = %v, muốn 0.375 (chương 3/8)", out.PromiseAt)
	}
	if out.TwistAt == nil || *out.TwistAt != 0.5 {
		t.Fatalf("twistAt = %v, muốn 0.5 (chương 4/8)", out.TwistAt)
	}
	ch3 := out.Chapters[2]
	if ch3.Payoff == nil || ch3.Payoff.Tag != "promise" || ch3.Payoff.Text != "Cảnh lời hứa" {
		t.Fatalf("payoff chương 3 = %+v", ch3.Payoff)
	}
	if ch3.Hook == nil || ch3.Hook.Type != "decision" || ch3.Hook.Waiting == "" {
		t.Fatalf("hook chương 3 = %+v", ch3.Hook)
	}
	if out.Chapters[3].Payoff == nil || out.Chapters[3].Payoff.Tag != "midpoint" {
		t.Fatalf("payoff chương 4 = %+v", out.Chapters[3].Payoff)
	}
	if out.Chapters[7].Payoff == nil || out.Chapters[7].Payoff.Tag != "climax" {
		t.Fatalf("payoff chương 8 = %+v", out.Chapters[7].Payoff)
	}

	byRef := map[string]retentionCheck{}
	for _, check := range out.Checks {
		byRef[check.Ref] = check
	}
	if len(out.Checks) != 4 {
		t.Fatalf("checks = %+v", out.Checks)
	}
	// Dòng kiểm đạt (§3, §8 ở đây) gửi chapters: [] trên dây, không bao giờ null.
	if strings.Contains(rec.Body.String(), `"chapters":null`) {
		t.Fatalf("chapters của dòng kiểm đạt phải là [], body = %s", rec.Body)
	}
	if check := byRef["Master §3"]; !check.OK || check.Text != "Cảnh lời hứa (Tri Hạ nối đúng câu bị ngắt trên live) nằm ở 38%, trong vùng 35–60%." {
		t.Errorf("§3 = %+v, muốn đạt (37,5%% trong 35–60%%)", check)
	}
	if check := byRef["Master §8"]; !check.OK || check.Text != "Cú lật giữa truyện (Phùng Chấn đặt hàng vụ cướp kịch bản) nằm ở 50%, trong vùng 45–60%." {
		t.Errorf("§8 = %+v, muốn đạt (50%% trong 45–60%%)", check)
	}
	hook := byRef["Master §7"]
	if hook.OK || len(hook.Chapters) != 3 || hook.Chapters[0] != 5 || hook.Chapters[2] != 7 {
		t.Errorf("§7 = %+v, muốn báo chương 5, 6, 7", hook)
	}
	if hook.Text != "Chương 5, 6 và 7 cùng kết bằng “Đe dọa mới”. Đổi chương 6 sang kiểu móc khác." {
		t.Errorf("§7 text = %q", hook.Text)
	}
	questions := byRef["Master §2"]
	if questions.OK || len(questions.Chapters) != 1 || questions.Chapters[0] != 1 {
		t.Errorf("§2 = %+v, muốn báo chương 1", questions)
	}
	if questions.Text != "Chương 1 chỉ có 1 câu hỏi đang treo, nên có 2–4." {
		t.Errorf("§2 text = %q", questions.Text)
	}
}

// TestRetentionQuestionsBoundary: 1 câu hỏi → báo đúng câu của artboard; chương
// cuối có 1 câu vẫn đạt (được miễn); 5 câu → báo "hơn 4".
func TestRetentionQuestionsBoundary(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, nil)
	entries := []domain.OutlineEntry{
		{Chapter: 1, Title: "Một", OpenQuestions: []string{"Ai?"}, HookType: "lat_thong_tin"},
		{Chapter: 2, Title: "Hai", OpenQuestions: []string{"Ai?", "Gì?"}, HookType: "de_doa_moi"},
		{Chapter: 3, Title: "Ba", OpenQuestions: []string{"a?", "b?", "c?", "d?", "e?"}, HookType: "quyet_dinh_treo"},
		{Chapter: 4, Title: "Bốn", OpenQuestions: []string{"Ai?"}, HookType: "cau_thoai_no"},
	}
	if err := s.Outline.SaveOutline(entries); err != nil {
		t.Fatal(err)
	}
	if err := s.Progress.Init(len(entries)); err != nil {
		t.Fatal(err)
	}
	var out retentionResponse
	rec := storyReq(t, g, http.MethodGet, "/x/retention", adminUser(), st, "/retention", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var check retentionCheck
	for _, c := range out.Checks {
		if c.Ref == "Master §2" {
			check = c
		}
	}
	if check.OK {
		t.Fatalf("§2 phải báo: %+v", check)
	}
	if len(check.Chapters) != 2 || check.Chapters[0] != 1 || check.Chapters[1] != 3 {
		t.Fatalf("§2 chapters = %v, muốn [1 3] (chương 4 được miễn)", check.Chapters)
	}
	if check.Text != "Chương 1 và 3 chưa đúng 2–4 câu hỏi đang treo." {
		t.Fatalf("§2 text = %q", check.Text)
	}
}

// TestRetentionZoneBoundaries: hai vùng 35–60% và 45–60% tính cả hai đầu
// (chương 7 và 12 của 20 chương), ngoài vùng thì báo kèm chương vi phạm.
func TestRetentionZoneBoundaries(t *testing.T) {
	build := func(promiseAt, twistAt int) ([]domain.OutlineEntry, diag.RetentionMap) {
		entries := make([]domain.OutlineEntry, 0, 20)
		for i := range 20 {
			entry := domain.OutlineEntry{Chapter: i + 1, CoreEvent: "sự kiện", HookType: "lat_thong_tin"}
			if i+1 == promiseAt {
				entry.Promise = "da_tra: cảnh lời hứa"
			}
			if i+1 == twistAt {
				entry.MidpointTwist = true
			}
			entries = append(entries, entry)
		}
		return entries, diag.BuildRetentionMap(&diag.Snapshot{Outline: entries})
	}
	for _, tc := range []struct {
		name     string
		promise  int
		wantOK   bool
		wantText string
	}{
		{"cận dưới 35%", 7, true, "Cảnh lời hứa (cảnh lời hứa) nằm ở 35%, trong vùng 35–60%."},
		{"cận trên 60%", 12, true, "Cảnh lời hứa (cảnh lời hứa) nằm ở 60%, trong vùng 35–60%."},
		{"dưới 35%", 6, false, "Cảnh lời hứa nằm ở 30%, ngoài vùng 35–60%."},
		{"trên 60%", 13, false, "Cảnh lời hứa nằm ở 65%, ngoài vùng 35–60%."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries, rmap := build(tc.promise, 0)
			check := promiseCheck(rmap, entries)
			if check.OK != tc.wantOK || check.Text != tc.wantText {
				t.Fatalf("§3 = %+v, muốn ok=%v text=%q", check, tc.wantOK, tc.wantText)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		twist  int
		wantOK bool
	}{
		{"cận dưới 45%", 9, true},
		{"cận trên 60%", 12, true},
		{"dưới 45%", 8, false},
		{"trên 60%", 13, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries, rmap := build(0, tc.twist)
			check := twistCheck(rmap, entries)
			if check.OK != tc.wantOK {
				t.Fatalf("§8 = %+v, muốn ok=%v", check, tc.wantOK)
			}
		})
	}
}

// TestRetentionZoneUnrounded: vùng so bằng tỉ lệ chưa làm tròn (Câu 19) — 8/23 =
// 34,8% hiển thị "35%" nhưng vẫn ngoài vùng 35–60%; 5/8 = 62,5% ngoài vùng.
func TestRetentionZoneUnrounded(t *testing.T) {
	build := func(total, promise int) ([]domain.OutlineEntry, diag.RetentionMap) {
		entries := make([]domain.OutlineEntry, 0, total)
		for i := 1; i <= total; i++ {
			entry := domain.OutlineEntry{Chapter: i, CoreEvent: "sự kiện", HookType: "lat_thong_tin"}
			if i == promise {
				entry.Promise = "da_tra: cảnh lời hứa"
			}
			entries = append(entries, entry)
		}
		return entries, diag.BuildRetentionMap(&diag.Snapshot{Outline: entries})
	}
	entries, rmap := build(23, 8)
	if check := promiseCheck(rmap, entries); check.OK {
		t.Fatalf("8/23 = 34,8%% phải ngoài vùng 35–60%%: %+v", check)
	}
	entries, rmap = build(8, 5)
	check := promiseCheck(rmap, entries)
	if check.OK || check.Text != "Cảnh lời hứa nằm ở 63%, ngoài vùng 35–60%." {
		t.Fatalf("5/8 = 62,5%% phải ngoài vùng: %+v", check)
	}
}

// TestRetentionSingleFewQuestionText: câu chữ khớp artboard khi chỉ một chương thiếu.
func TestRetentionSingleFewQuestionText(t *testing.T) {
	entries := make([]domain.OutlineEntry, 0, 8)
	for i := range 8 {
		entry := domain.OutlineEntry{Chapter: i + 1, HookType: "lat_thong_tin"}
		entry.OpenQuestions = []string{"Ai?", "Gì?"}
		if i == 0 {
			entry.OpenQuestions = []string{"Ai?"}
		}
		entries = append(entries, entry)
	}
	rmap := diag.BuildRetentionMap(&diag.Snapshot{Outline: entries})
	check := questionsCheck(rmap)
	if check.OK || check.Text != "Chương 1 chỉ có 1 câu hỏi đang treo, nên có 2–4." {
		t.Fatalf("§2 = %+v", check)
	}
}

// TestFrameNoPackLeavesAudienceEmpty: không tự đoán nhóm khi chưa có gói;
// vẫn trả năm nhóm Master để hiện dữ liệu đã lưu và cho người dùng chọn.
func TestFrameNoPackLeavesAudienceEmpty(t *testing.T) {
	g := &Gateway{}
	st, _ := storyFixture(t, nil)

	rec := storyReq(t, g, http.MethodGet, "/x/frame", adminUser(), st, "/frame", "")
	frame := decodeFrame(t, rec)
	if len(frame.Audiences) != len(domain.AudienceLabels) || len(frame.Audience) != 0 || frame.Audience == nil {
		t.Fatalf("frame không gói phải có bộ chọn Master: %+v", frame)
	}
	rec = storyReq(t, g, http.MethodPut, "/x/frame", adminUser(), st, "/frame", `{"audience":[1]}`)
	if rec.Code != http.StatusOK || !slices.Equal(decodeFrame(t, rec).Audience, []int{1}) {
		t.Fatalf("PUT audience không gói → %d, muốn lưu [1]: %s", rec.Code, rec.Body)
	}
}

// TestFrameAudienceVariants: gói C khai hai biến thể → audience [2,1] (C-Nam
// trước, C-Nữ sau); PUT giữ đúng thứ tự khách chọn và lưu mảng chỉ số (Câu 18).
func TestFrameAudienceVariants(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, nil)
	storyWithPack(t, st, "nien-dai-van")

	frame := decodeFrame(t, storyReq(t, g, http.MethodGet, "/x/frame", adminUser(), st, "/frame", ""))
	if len(frame.Audience) != 2 || frame.Audience[0] != 2 || frame.Audience[1] != 1 {
		t.Fatalf("audience gói C = %v, muốn [2 1]", frame.Audience)
	}

	rec := storyReq(t, g, http.MethodPut, "/x/frame", adminUser(), st, "/frame", `{"audience":[1,2]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT frame: %d %s", rec.Code, rec.Body)
	}
	if again := decodeFrame(t, rec); len(again.Audience) != 2 || again.Audience[0] != 1 || again.Audience[1] != 2 {
		t.Fatalf("audience sau PUT = %v, muốn [1 2]", again.Audience)
	}
	stored, err := s.Outline.LoadStoryRetention()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stored.NguoiNghe, []int{1, 2}) {
		t.Fatalf("nguoi_nghe đã lưu = %v", stored.NguoiNghe)
	}

	// Truyện đã chọn một biến thể thì chỉ còn một số.
	rec = storyReq(t, g, http.MethodPut, "/x/frame", adminUser(), st, "/frame", `{"audience":[1]}`)
	if again := decodeFrame(t, rec); len(again.Audience) != 1 || again.Audience[0] != 1 {
		t.Fatalf("audience một biến thể = %v", again.Audience)
	}
}

// TestRetentionEmptyWhenNoFields: dàn ý cũ chưa khai trường giữ chân nào → empty,
// không chạy phép kiểm nào (Câu 19), nhưng vẫn trả các dòng đại cương thật.
func TestRetentionEmptyWhenNoFields(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, nil)
	entries := make([]domain.OutlineEntry, 0, 4)
	for i := 1; i <= 4; i++ {
		entries = append(entries, domain.OutlineEntry{Chapter: i, Title: "Chương", CoreEvent: "sự kiện"})
	}
	if err := s.Outline.SaveOutline(entries); err != nil {
		t.Fatal(err)
	}
	rec := storyReq(t, g, http.MethodGet, "/x/retention", adminUser(), st, "/retention", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("retention: %d %s", rec.Code, rec.Body)
	}
	var out retentionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, rec.Body.String())
	}
	if !out.Empty {
		t.Fatalf("empty = false, muốn true: %+v", out)
	}
	if len(out.Checks) != 0 {
		t.Fatalf("checks = %+v, muốn rỗng khi chưa có bản đồ giữ chân", out.Checks)
	}
	if len(out.Chapters) != 4 {
		t.Fatalf("chapters = %d, muốn 4", len(out.Chapters))
	}
}

// TestRetentionSeedHook: `seed` chỉ hợp lệ ở chương cuối; chương khác bị bỏ móc
// và dòng kiểm §7 nói rõ lý do (Câu 19).
func TestRetentionSeedHook(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, nil)
	entries := []domain.OutlineEntry{
		{Chapter: 1, Title: "Một", Hook: "ai đứng sau", HookType: "crisis"},
		{Chapter: 2, Title: "Hai", Hook: "còn gì nữa", HookType: "seed"},
		{Chapter: 3, Title: "Ba", Hook: "phần hai mở bằng gì", HookType: "seed"},
	}
	if err := s.Outline.SaveOutline(entries); err != nil {
		t.Fatal(err)
	}
	rec := storyReq(t, g, http.MethodGet, "/x/retention", adminUser(), st, "/retention", "")
	var out retentionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, rec.Body.String())
	}
	if out.Chapters[0].Hook == nil || out.Chapters[0].Hook.Type != "threat" {
		t.Fatalf("hook chương 1 (crisis) = %+v, muốn threat", out.Chapters[0].Hook)
	}
	if out.Chapters[1].Hook != nil {
		t.Fatalf("hook chương 2 = %+v, muốn bỏ vì seed không hợp lệ", out.Chapters[1].Hook)
	}
	if out.Chapters[2].Hook == nil || out.Chapters[2].Hook.Type != "seed" || out.Chapters[2].Hook.Waiting == "" {
		t.Fatalf("hook chương cuối = %+v, muốn seed", out.Chapters[2].Hook)
	}
	var hookCheck retentionCheck
	for _, check := range out.Checks {
		if check.Ref == "Master §7" {
			hookCheck = check
		}
	}
	if hookCheck.OK || hookCheck.Text != "Chương 2 dùng “Hạt giống phần hai” khi chưa phải chương cuối; kiểu này chỉ hợp lệ ở chương cuối." {
		t.Fatalf("§7 = %+v", hookCheck)
	}
}

// TestRetentionOldHookTypes: bảng chuyển kiểu móc cũ sang Master §7 (Câu 15).
func TestRetentionOldHookTypes(t *testing.T) {
	entries := []domain.OutlineEntry{
		{Chapter: 1, HookType: "crisis"},
		{Chapter: 2, HookType: "mystery"},
		{Chapter: 3, HookType: "choice"},
		{Chapter: 4, HookType: "desire"},
		{Chapter: 5, HookType: "emotion"},
	}
	want := []string{"threat", "info", "decision", "decision", "line"}
	for i, entry := range entries {
		hook := retentionHookOf(entry, i == len(entries)-1)
		if hook == nil || hook.Type != want[i] {
			t.Fatalf("chương %d (%s) = %+v, muốn %s", i+1, entry.HookType, hook, want[i])
		}
	}
}

func TestStoryFieldsLegacyRead(t *testing.T) {
	g := &Gateway{}
	st, s := storyFixture(t, nil)
	stored := domain.StoryRetention{NguoiNghe: []int{1}, DiemKhac: "Không có other_stories trong ngữ cảnh để đối chiếu truyện trước. Khác mẫu bằng nghề hậu kỳ âm thanh."}
	if err := s.Outline.SaveStoryRetention(stored); err != nil {
		t.Fatal(err)
	}
	frame := decodeFrame(t, storyReq(t, g, http.MethodGet, "/x/frame", adminUser(), st, "/frame", ""))
	if frame.Difference != "" {
		t.Errorf("no previous story must have empty comparison: %q", frame.Difference)
	}
	if len(frame.Audiences) != 5 || frame.Audiences[1].Label != "Nữ 18–35" || !slices.Equal(frame.Audience, []int{1}) {
		t.Errorf("legacy stored audience must render without a pack: %+v", frame)
	}
	again, err := s.Outline.LoadStoryRetention()
	if err != nil || again.DiemKhac != stored.DiemKhac {
		t.Fatalf("read must not rewrite existing data: %+v, %v", again, err)
	}
}

func longText(n int) string {
	out := make([]rune, 0, n)
	for range n {
		out = append(out, 'a')
	}
	return string(out)
}
