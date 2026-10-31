package web

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestHostedProjectUniverseReviewDocuments(t *testing.T) {
	f := provisionHostedGateway(t, "universe-reviews", "story")
	actor := &library.Actor{Username: f.owner.Username}
	other, _, err := f.repo.CreateStory(t.Context(), actor, f.project, "Truyện cùng vũ trụ", "")
	if err != nil {
		t.Fatal(err)
	}
	var reviewed *store.Store
	for _, story := range []string{f.story, other.Story} {
		engine, err := bootstrap.OpenStoryStoreInternal(t.Context(), f.pool, f.project, story)
		if err != nil {
			t.Fatal(err)
		}
		if err := engine.Docs().WriteJSON("characters.json", []domain.Character{{ID: "ba-tu", Name: "Bà Tư", Role: "Hàng xóm"}}); err != nil {
			t.Fatal(err)
		}
		if story == f.story {
			reviewed = engine
		}
	}
	archived := domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "rewrite", RuleAware: true,
		Issues: []domain.ConsistencyIssue{recapIssue("Nhắc lại chuyện cũ ở chương đầu.")}}
	if err := reviewed.World.SaveReview(archived); err != nil {
		t.Fatal(err)
	}
	if err := reviewed.World.SaveReview(domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "accept", RuleAware: true}); err != nil {
		t.Fatal(err)
	}
	global := domain.ReviewEntry{Chapter: 2, Scope: "global", Verdict: "rewrite", RuleAware: true,
		Issues: []domain.ConsistencyIssue{recapIssue("Nhắc lại chuyện cũ trong bài tổng hợp.")}}
	if err := reviewed.World.SaveReview(global); err != nil {
		t.Fatal(err)
	}
	tg := &testGateway{t: t, g: f.g}
	viewer := tg.login("hosted-viewer", "mat-khau-12345")
	w := tg.do(http.MethodGet, "/api/projects/"+f.project+"/universe", viewer, "")
	if w.Code != http.StatusOK {
		t.Fatalf("hosted universe: %d %s", w.Code, w.Body.String())
	}
	var response universeBody
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Counts.Stories != 2 || len(response.Checks) != 2 {
		t.Fatalf("hosted universe must include current and archived review documents: %s", w.Body.String())
	}
	byChapter := make(map[int]universeCheck)
	for _, check := range response.Checks {
		if check.Story != f.story || check.Ref != universeCheckRef || check.QuoteID == "" {
			t.Fatalf("wrong hosted review identity: %+v", check)
		}
		byChapter[check.Ch] = check
	}
	if byChapter[1].Text != archived.Issues[0].Description || byChapter[2].Text != global.Issues[0].Description {
		t.Fatalf("wrong hosted review documents: %+v", response.Checks)
	}
	outsider := tg.login("hosted-outsider", "mat-khau-12345")
	w = tg.do(http.MethodGet, "/api/projects/"+f.project+"/universe", outsider, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("outsider universe: %d %s", w.Code, w.Body.String())
	}
}
