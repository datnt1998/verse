package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

const apiPremise = `## Thể loại và giọng điệu
Trinh thám.
## Định vị thể loại
Một vụ án.
## Xung đột cốt lõi
Sự thật và tình thân.
## Mục tiêu nhân vật chính
Tìm người mất tích.
## Hướng kết cục
Trở về nhà.
## Vùng cấm sáng tác
Không phép màu.
## Điểm bán hàng khác biệt
Nhân chứng câm.
## Móc câu khác biệt
Bản nhạc giấu lời khai.
## Cam kết cốt lõi
Giải đáp manh mối.
## Tính phù hợp với truyện ngắn
Một đêm.
`

func assertFoundationAPI(t *testing.T, s *store.Store, get func(string) *httptest.ResponseRecorder) {
	t.Helper()
	readPremise := func() host.StoryPremise {
		t.Helper()
		w := get("/premise")
		if w.Code != http.StatusOK {
			t.Fatalf("premise: %d %s", w.Code, w.Body.String())
		}
		var result host.StoryPremise
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	empty := readPremise()
	if empty.Sections == nil || len(empty.Sections) != 0 || len(empty.Missing) != 9 || empty.UpdatedAt != nil {
		t.Fatalf("empty premise = %+v", empty)
	}
	if w := get("/world-rules"); w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty world rules: %d %s", w.Code, w.Body.String())
	}
	if err := s.RunMeta.Save(domain.RunMeta{PlanningTier: domain.PlanningTierShort}); err != nil {
		t.Fatal(err)
	}
	if err := s.Outline.SavePremise(apiPremise); err != nil {
		t.Fatal(err)
	}
	complete := readPremise()
	if len(complete.Sections) != 10 || complete.Missing == nil || len(complete.Missing) != 0 || complete.UpdatedAt == nil {
		t.Fatalf("complete premise = %+v", complete)
	}
	at, err := s.Docs().ModifiedAt("premise.md")
	if err != nil {
		t.Fatal(err)
	}
	if !complete.UpdatedAt.Equal(at) {
		t.Fatalf("premise timestamp = %v, stored = %v", complete.UpdatedAt, at)
	}
	if complete.Sections[0].Heading != "Thể loại và giọng điệu" || complete.Sections[0].Text != "Trinh thám." {
		t.Fatalf("premise text = %+v", complete.Sections[0])
	}
	if err := s.Outline.SavePremise(strings.Replace(apiPremise, "## Cam kết cốt lõi\nGiải đáp manh mối.\n", "", 1)); err != nil {
		t.Fatal(err)
	}
	partial := readPremise()
	if !reflect.DeepEqual(partial.Missing, []string{"Cam kết cốt lõi"}) || len(partial.Sections) != 9 {
		t.Fatalf("partial premise = %+v", partial)
	}
	if err := s.Outline.SavePremise(apiPremise + "## Phong tục địa phương\nĐổi tên vào mùa mưa.\n"); err != nil {
		t.Fatal(err)
	}
	custom := readPremise()
	if len(custom.Sections) != 11 || custom.Sections[10].Heading != "Phong tục địa phương" || custom.Sections[10].Text != "Đổi tên vào mùa mưa." || len(custom.Missing) != 0 {
		t.Fatalf("custom premise section = %+v", custom)
	}
	wantRules := []domain.WorldRule{{Category: "society", Rule: "Lời thề ràng buộc người nói.", Boundary: "Không ép người khác thề."}}
	if err := s.World.SaveWorldRules(wantRules); err != nil {
		t.Fatal(err)
	}
	w := get("/world-rules")
	if w.Code != http.StatusOK {
		t.Fatalf("world rules: %d %s", w.Code, w.Body.String())
	}
	var rules []domain.WorldRule
	if err := json.Unmarshal(w.Body.Bytes(), &rules); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rules, wantRules) {
		t.Fatalf("world rules = %+v", rules)
	}
	if err := s.Characters.Save([]domain.Character{{ID: "nv:lan", Name: "Lan", Role: "Nhân vật chính"}}); err != nil {
		t.Fatal(err)
	}
	w = get("/brain")
	if w.Code != http.StatusOK {
		t.Fatalf("brain: %d %s", w.Code, w.Body.String())
	}
	var brain naoBrainResponse
	if err := json.Unmarshal(w.Body.Bytes(), &brain); err != nil {
		t.Fatal(err)
	}
	foundCharacter, foundRule := false, false
	for _, entity := range brain.Entities {
		switch entity.Type {
		case "char":
			if entity.Name == "Lan" && entity.Role == "Nhân vật chính" && entity.Category == "" {
				foundCharacter = true
			}
		case "rule":
			if entity.ID == "rule:0" && entity.Name == wantRules[0].Rule && entity.Desc == wantRules[0].Boundary && entity.Category == wantRules[0].Category {
				foundRule = true
			}
		}
	}
	if !foundCharacter || !foundRule {
		t.Fatalf("brain must preserve characters and rule category: %+v", brain.Entities)
	}
}

func TestFoundationAPIFile(t *testing.T) {
	st, s := storyFixture(t, nil)
	g := &Gateway{}
	assertFoundationAPI(t, s, func(rest string) *httptest.ResponseRecorder {
		return storyReq(t, g, http.MethodGet, "/x"+rest, adminUser(), st, rest, "")
	})
}

func TestFoundationAPIHostedAndWorkerSnapshot(t *testing.T) {
	f := provisionHostedMedia(t)
	tg := &testGateway{t: t, g: f.fx.g}
	owner := tg.login("hosted-owner", "mat-khau-12345")
	outsider := tg.login("hosted-outsider", "mat-khau-12345")
	s, err := f.fx.g.openFactStore(f.st, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/projects/" + f.st.Project + "/stories/" + f.st.Story
	get := func(rest string) *httptest.ResponseRecorder { return tg.do(http.MethodGet, base+rest, owner, "") }
	assertFoundationAPI(t, s, get)
	for _, rest := range []string{"/premise", "/world-rules"} {
		if w := tg.do(http.MethodGet, base+rest, outsider, ""); w.Code != http.StatusForbidden {
			t.Fatalf("outsider %s: %d %s", rest, w.Code, w.Body.String())
		}
	}
	if err := s.RunMeta.Save(domain.RunMeta{PlanningTier: domain.PlanningTierLong}); err != nil {
		t.Fatal(err)
	}
	want := domain.StoryCompass{EndingDirection: "Trở về nhà.", EstimatedScale: "Ba tập.", OpenThreads: []string{"Người mất tích."}}
	if err := s.Outline.SaveCompass(want); err != nil {
		t.Fatal(err)
	}
	installConfigWorkerLauncher(t, f.fx)
	w := get("/snapshot")
	if w.Code != http.StatusOK {
		t.Fatalf("hosted worker snapshot: %d %s", w.Code, w.Body.String())
	}
	var snap host.UISnapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.PlanningTier != domain.PlanningTierLong || !reflect.DeepEqual(snap.OpenThreads, want.OpenThreads) || snap.CompassDirection != want.EndingDirection || snap.CompassScale != want.EstimatedScale {
		t.Fatalf("hosted worker snapshot = %+v", snap)
	}
}
