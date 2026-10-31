package lessons

import (
	"fmt"
	"strings"
	"testing"
)

func observations(n int, level string) []Observation {
	out := []Observation{}
	for i := range n {
		out = append(out, Observation{ID: "issue", Story: fmt.Sprint(i), StoryTitle: "Truyện", PackID: "pack-a", PackName: "Gói A", Ref: "Master §7", RefName: "Đầu chương và cuối chương", Level: level, Chapter: 1})
	}
	return out
}

func TestLessonsCandidateAndAlertBoundaries(t *testing.T) {
	t.Run("candidate", func(t *testing.T) {
		if got := Aggregate(observations(1, "warning")); len(got) != 0 {
			t.Fatalf("one story proposed %d lessons; need two", len(got))
		}
		got := Aggregate(observations(2, "warning"))
		if len(got) != 1 || got[0].Evidence.Stories != 2 {
			t.Fatalf("two stories candidate: %+v", got)
		}
	})
	t.Run("alert", func(t *testing.T) {
		if got := Violations(observations(2, "error"), "pack-a", 3); len(got) != 1 || got[0].Alert {
			t.Fatalf("two error stories must not alert: %+v", got)
		}
		if got := Violations(observations(3, "error"), "pack-a", 3); len(got) != 1 || !got[0].Alert {
			t.Fatalf("three error stories must alert: %+v", got)
		}
		mixed := append(observations(2, "error"), Observation{ID: "warning", Story: "third", PackID: "pack-a", Ref: "Master §7", RefName: "Đầu chương và cuối chương", Level: "warning"})
		got := Violations(mixed, "pack-a", 4)
		if got[0].Alert || got[0].Stories != 3 || got[0].Total != 4 {
			t.Fatalf("warnings must not contribute to error alert: %+v", got)
		}
		if !strings.Contains(Preamble(got, "Gói A"), "Chưa có") {
			t.Fatal("missing explicit no-preamble message")
		}
	})
	t.Run("dedup", func(t *testing.T) {
		obs := observations(3, "error")
		obs = append(obs, obs[0])
		got := Aggregate(obs)
		if len(got) != 1 || got[0].Evidence.Errors != 3 || len(got[0].Evidence.Chips) != 1 {
			t.Fatalf("duplicate observation counted twice: %+v", got)
		}
	})
}

// Câu 6: mức không tách nhóm — Bắt sửa và Cảnh báo của hai truyện là MỘT bài học, bằng
// chứng đếm riêng từng loại (§M): 2 lần Bắt sửa + 1 Cảnh báo → errors 2, warnings 1.
func TestLessonsLevelsMergeIntoOneCandidate(t *testing.T) {
	obs := []Observation{
		{ID: "e1", Story: "a", StoryTitle: "Truyện A", PackID: "pack-a", PackName: "Gói A", Ref: "Master §7", RefName: "Đầu chương và cuối chương", Level: "error", Chapter: 2},
		{ID: "e2", Story: "a", StoryTitle: "Truyện A", PackID: "pack-a", PackName: "Gói A", Ref: "Master §7", RefName: "Đầu chương và cuối chương", Level: "error", Chapter: 4},
		{ID: "w", Story: "b", StoryTitle: "Truyện B", PackID: "pack-a", PackName: "Gói A", Ref: "Master §7", RefName: "Đầu chương và cuối chương", Level: "warning", Chapter: 3},
	}
	got := Aggregate(obs)
	if len(got) != 1 {
		t.Fatalf("merged levels produced %d lessons; need one", len(got))
	}
	if got[0].Evidence.Errors != 2 || got[0].Evidence.Warnings != 1 || got[0].Evidence.Stories != 2 {
		t.Fatalf("merged evidence: %+v", got[0].Evidence)
	}
	rows := Violations(obs, "pack-a", 4)
	if len(rows) != 1 || rows[0].Level != "error" || rows[0].Stories != 2 {
		t.Fatalf("violations row must merge levels too: %+v", rows)
	}
}

// Câu 15 + câu 4: mục Master nhóm qua mọi gói, đề xuất “Mọi gói”. Dòng nguồn của mục dùng
// chung luôn ghi “· N gói” (kể cả “· 1 gói”), nên `evidence.pack` để trống ở mọi trường hợp.
func TestLessonsSharedRefGroupsAcrossPacks(t *testing.T) {
	obs := []Observation{
		{ID: "a", Story: "p1/s1", StoryTitle: "Truyện A", PackID: "pack-a", PackName: "Gói A", Ref: "Master §4", RefName: "Mở đầu", Level: "error", Chapter: 1},
		{ID: "b", Story: "p1/s2", StoryTitle: "Truyện B", PackID: "pack-a", PackName: "Gói A", Ref: "Master §4", RefName: "Mở đầu", Level: "warning", Chapter: 1},
	}
	got := Aggregate(obs)
	if len(got) != 1 {
		t.Fatalf("Master §4 in one pack = %d lessons; need one", len(got))
	}
	if one := got[0]; one.Scope != "all" || one.Evidence.PacksCount != 1 || one.Evidence.Pack != "" {
		t.Fatalf("shared ref must report packs, never a pack name: scope=%q pack=%q packs=%d", one.Scope, one.Evidence.Pack, one.Evidence.PacksCount)
	}
	obs = append(obs, Observation{ID: "c", Story: "p2/s1", StoryTitle: "Truyện C", PackID: "pack-b", PackName: "Gói B", Ref: "Master §4", RefName: "Mở đầu", Level: "error", Chapter: 5})
	got = Aggregate(obs)
	if len(got) != 1 {
		t.Fatalf("Master §4 across packs = %d lessons; need one", len(got))
	}
	lesson := got[0]
	if lesson.Scope != "all" || lesson.Evidence.PacksCount != 2 || lesson.Evidence.Pack != "" || lesson.Evidence.Stories != 3 {
		t.Fatalf("cross-pack lesson: scope=%q pack=%q packs=%d stories=%d", lesson.Scope, lesson.Evidence.Pack, lesson.Evidence.PacksCount, lesson.Evidence.Stories)
	}
	// Bảng vi phạm vẫn “theo gói”: số truyện tính trong gói đang xem; chuông mục dùng chung
	// tính trên mọi gói (câu 10) nên gói B (1 Bắt sửa) vẫn tắt khi cả vũ trụ mới 2 Bắt sửa.
	if rows := Violations(obs, "pack-b", 2); len(rows) != 1 || rows[0].Stories != 1 || rows[0].Total != 2 || rows[0].Alert || rows[0].AlertStories == nil || *rows[0].AlertStories != 2 {
		t.Fatalf("pack-scoped violations row: %+v", rows)
	}
}

// Câu 15: mục Bộ A §… thuộc gói A, không gộp với Bộ B §… của gói B — và cùng một nhãn
// “Bộ A” ở gói khác cũng là bài học của gói đó, không gộp qua gói.
func TestLessonsPackRefsStayPerPack(t *testing.T) {
	obs := []Observation{
		{ID: "a1", Story: "p1/s1", StoryTitle: "Truyện A", PackID: "pack-a", PackName: "Gói A", Ref: "Bộ A §4.3", RefName: "Trà xanh và phản diện", Level: "error", Chapter: 2},
		{ID: "a2", Story: "p1/s2", StoryTitle: "Truyện B", PackID: "pack-a", PackName: "Gói A", Ref: "Bộ A §4.3", RefName: "Trà xanh và phản diện", Level: "warning", Chapter: 2},
		{ID: "b1", Story: "p2/s1", StoryTitle: "Truyện C", PackID: "pack-b", PackName: "Gói B", Ref: "Bộ B §4.3", RefName: "Đối thủ và phản diện", Level: "error", Chapter: 2},
		{ID: "b2", Story: "p2/s2", StoryTitle: "Truyện D", PackID: "pack-b", PackName: "Gói B", Ref: "Bộ B §4.3", RefName: "Đối thủ và phản diện", Level: "error", Chapter: 3},
	}
	got := Aggregate(obs)
	if len(got) != 2 {
		t.Fatalf("Bộ A and Bộ B must stay two lessons, got %+v", got)
	}
	for _, lesson := range got {
		if lesson.Scope != "pack" || lesson.Evidence.PacksCount != 1 || lesson.Evidence.Pack == "" {
			t.Fatalf("pack ref must stay inside its pack: %+v", lesson)
		}
	}
	if got[0].PackID == got[1].PackID || got[0].ID == got[1].ID {
		t.Fatalf("Bộ A and Bộ B share one group: %+v", got)
	}
	obs = append(obs,
		Observation{ID: "c1", Story: "p3/s1", StoryTitle: "Truyện E", PackID: "pack-c", PackName: "Gói C", Ref: "Bộ A §4.3", RefName: "Trà xanh và phản diện", Level: "error", Chapter: 2},
		Observation{ID: "c2", Story: "p3/s2", StoryTitle: "Truyện F", PackID: "pack-c", PackName: "Gói C", Ref: "Bộ A §4.3", RefName: "Trà xanh và phản diện", Level: "warning", Chapter: 4})
	got = Aggregate(obs)
	if len(got) != 3 {
		t.Fatalf("the same Bộ A label in another pack must be its own lesson, got %+v", got)
	}
	seen := map[string]string{}
	for _, lesson := range got {
		if other, dup := seen[lesson.ID]; dup {
			t.Fatalf("%s and %s share lesson id %s", other, lesson.Refs[0], lesson.ID)
		}
		seen[lesson.ID] = lesson.Refs[0]
	}
}

func TestLessonsEditedTextRejectsNamesAndPlots(t *testing.T) {
	for _, text := range []string{"Đừng để Lâm Trần. nghỉ ngơi ở cuối chương", "Bắt đầu bằng cảnh chiếc nhẫn rơi xuống bậc thang trước cửa"} {
		if err := ValidateText(text, []string{"Lâm Trần"}, []string{"chiếc nhẫn rơi xuống bậc thang trước cửa"}); err == nil {
			t.Fatalf("accepted story-specific lesson %q", text)
		}
	}
	if err := ValidateText("Kiểm tra mục Master §7 khi lên đại cương", []string{"Lâm Trần"}, nil); err != nil {
		t.Fatal(err)
	}
}

// Câu 10: chuông của mục dùng chung tính trên mọi gói (2 Bắt sửa ở gói A + 1 ở gói B = 3),
// nên cả hai bảng gói đều bật chuông dù cột số truyện vẫn đếm theo gói. Mục `Bộ …` không đổi.
func TestViolationsSharedRefAlertsAcrossPacks(t *testing.T) {
	obs := []Observation{
		{ID: "a1", Story: "p1/s1", StoryTitle: "Truyện A", PackID: "pack-a", PackName: "Gói A", Ref: "Master §7", RefName: "Đầu chương", Level: "error", Chapter: 1},
		{ID: "a2", Story: "p1/s2", StoryTitle: "Truyện B", PackID: "pack-a", PackName: "Gói A", Ref: "Master §7", RefName: "Đầu chương", Level: "error", Chapter: 2},
		{ID: "b1", Story: "p2/s1", StoryTitle: "Truyện C", PackID: "pack-b", PackName: "Gói B", Ref: "Master §7", RefName: "Đầu chương", Level: "error", Chapter: 3},
		{ID: "a3", Story: "p1/s3", StoryTitle: "Truyện D", PackID: "pack-a", PackName: "Gói A", Ref: "Bộ A §4.3", RefName: "Trà xanh", Level: "error", Chapter: 4},
		{ID: "a4", Story: "p1/s4", StoryTitle: "Truyện E", PackID: "pack-a", PackName: "Gói A", Ref: "Bộ A §4.3", RefName: "Trà xanh", Level: "error", Chapter: 5},
	}
	row := func(rows []Row, ref string) Row {
		for _, r := range rows {
			if r.Ref == ref {
				return r
			}
		}
		t.Fatalf("no row for %s: %+v", ref, rows)
		return Row{}
	}
	for _, view := range []struct {
		pack           string
		total, stories int
	}{{"pack-a", 4, 2}, {"pack-b", 1, 1}} {
		rows := Violations(obs, view.pack, view.total)
		shared := row(rows, "Master §7")
		if !shared.Alert || shared.AlertScope != "all" || shared.AlertStories == nil || *shared.AlertStories != AlertStories {
			t.Fatalf("%s · shared row must alert on every pack: %+v", view.pack, shared)
		}
		if shared.Stories != view.stories || shared.Total != view.total || shared.Level != "error" {
			t.Fatalf("%s · pack-scoped counts must not move: %+v", view.pack, shared)
		}
		if !strings.Contains(Preamble(rows, view.pack), "Master §7") {
			t.Fatalf("%s · preamble must use the same alert set: %+v", view.pack, rows)
		}
	}
	own := row(Violations(obs, "pack-a", 4), "Bộ A §4.3")
	if own.Alert || own.AlertScope != "" || own.AlertStories != nil || own.Stories != 2 {
		t.Fatalf("Bộ row must stay pack-scoped and unchanged: %+v", own)
	}
	// Ba Bắt sửa trong đúng gói vẫn bật chuông như trước.
	obs = append(obs, Observation{ID: "a5", Story: "p1/s5", StoryTitle: "Truyện F", PackID: "pack-a", PackName: "Gói A", Ref: "Bộ A §4.3", RefName: "Trà xanh", Level: "error", Chapter: 6})
	own = row(Violations(obs, "pack-a", 5), "Bộ A §4.3")
	if !own.Alert || own.AlertScope != "" || own.AlertStories != nil {
		t.Fatalf("Bộ row bell must still come from its own pack: %+v", own)
	}
	// Mục dùng chung chỉ có Cảnh báo vẫn trả `alertStories: 0` (không bỏ khoá) để FE không đoán.
	obs = append(obs, Observation{ID: "a6", Story: "p1/s6", StoryTitle: "Truyện G", PackID: "pack-a", PackName: "Gói A", Ref: "Master §13", RefName: "Văn phong", Level: "warning", Chapter: 7})
	warn := row(Violations(obs, "pack-a", 5), "Master §13")
	if warn.Alert || warn.AlertScope != "all" || warn.AlertStories == nil || *warn.AlertStories != 0 || warn.Stories != 1 {
		t.Fatalf("warning-only shared row must still report alertStories 0: %+v", warn)
	}
}

// §J câu 4: mục dùng chung có số Bắt sửa tính trên mọi gói khác số trong gói đang
// xem thì lời nhắc ghi số liên gói ("6 truyện ở mọi gói"); bằng nhau, hoặc mục riêng
// của gói, vẫn ghi "n/tổng truyện".
func TestPreambleSharedRefUsesCrossPackCount(t *testing.T) {
	obs := []Observation{
		// Master §7: 1 Bắt sửa trong gói B, 5 ở gói A ⇒ liên gói 6, gói B thấy 1/4.
		{ID: "b7", Story: "p2/s1", StoryTitle: "Truyện C", PackID: "pack-b", PackName: "Gói B", Ref: "Master §7", RefName: "Đầu chương và cuối chương", Level: "error", Chapter: 1},
		// Master §10: 3 Bắt sửa, cả 3 đều trong gói B ⇒ liên gói 3 = trong gói 3.
		{ID: "b10a", Story: "p2/s2", StoryTitle: "Truyện D", PackID: "pack-b", PackName: "Gói B", Ref: "Master §10", RefName: "Điểm nhìn", Level: "error", Chapter: 2},
		{ID: "b10b", Story: "p2/s3", StoryTitle: "Truyện E", PackID: "pack-b", PackName: "Gói B", Ref: "Master §10", RefName: "Điểm nhìn", Level: "error", Chapter: 3},
		{ID: "b10c", Story: "p2/s4", StoryTitle: "Truyện F", PackID: "pack-b", PackName: "Gói B", Ref: "Master §10", RefName: "Điểm nhìn", Level: "error", Chapter: 4},
		// Mục riêng của gói: chuông theo gói, không bao giờ ghi "mọi gói".
		{ID: "bown1", Story: "p2/s1", StoryTitle: "Truyện C", PackID: "pack-b", PackName: "Gói B", Ref: "Bộ B §4.3", RefName: "Trà xanh", Level: "error", Chapter: 1},
		{ID: "bown2", Story: "p2/s2", StoryTitle: "Truyện D", PackID: "pack-b", PackName: "Gói B", Ref: "Bộ B §4.3", RefName: "Trà xanh", Level: "error", Chapter: 2},
		{ID: "bown3", Story: "p2/s3", StoryTitle: "Truyện E", PackID: "pack-b", PackName: "Gói B", Ref: "Bộ B §4.3", RefName: "Trà xanh", Level: "error", Chapter: 3},
	}
	for i := range 5 {
		obs = append(obs, Observation{
			ID: fmt.Sprintf("a%d", i), Story: fmt.Sprintf("p1/s%d", i), StoryTitle: "Truyện A",
			PackID: "pack-a", PackName: "Gói A", Ref: "Master §7",
			RefName: "Đầu chương và cuối chương", Level: "error", Chapter: i + 2,
		})
	}
	got := Preamble(Violations(obs, "pack-b", 4), "Gói B")
	for _, want := range []string{
		"Master §7 (đầu chương và cuối chương, 6 truyện ở mọi gói)",
		"Master §10 (điểm nhìn, 3/4 truyện)",
		"Bộ B §4.3 (trà xanh, 3/4 truyện)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("thiếu %q trong lời nhắc: %s", want, got)
		}
	}
}

func TestApprovedSystemLessonsRemainDefaultsWithoutGlobalRuleScope(t *testing.T) {
	s := NewStore(t.TempDir())
	if _, err := s.Sync([]Lesson{
		{ID: "system", Scope: "all", State: "kept", Text: "Kiểm tra mở đầu", Refs: []string{"Master §7"}},
		{ID: "pack", Scope: "pack", PackID: "pack-a", State: "kept", Text: "Kiểm tra hồi đáp", Refs: []string{"Bộ A §4.3"}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, pack := range []string{"", "pack-a", "pack-b"} {
		candidates, err := s.Rules(pack)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if pack == "pack-a" {
			want = 2
		}
		if len(candidates) != want || candidates[0].Source != "default:lessons" || candidates[0].Preferences != "Kiểm tra mở đầu" {
			t.Fatalf("system default must apply without a global rules scope for %q: %+v", pack, candidates)
		}
		if pack == "pack-a" && candidates[1].Source != "pack:pack-a:lessons" {
			t.Fatalf("pack lesson scope changed: %+v", candidates)
		}
	}
}
