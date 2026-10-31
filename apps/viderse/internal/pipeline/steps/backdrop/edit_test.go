package backdrop

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

func testItem(id, video, source, status string) Item {
	v := Item{ID: id, ChannelID: "twitch:a", VideoID: video, Kind: "clip", URL: "https://example.test/" + video, StartSec: 0, EndSec: 30, Source: source, Status: status}
	if status == "ready" {
		v.MediaID = new("media-" + id)
	}
	return v
}
func editFixture(t *testing.T, fill bool, items []Item) (*harness, Editor, EditState) {
	t.Helper()
	count := new(3)
	if fill {
		count = nil
	}
	h := newHarness(t, recipe(count, 30, "twitch:a"), nil)
	h.store.seed("twitch:a", 10, 300)
	m := Seal(Manifest{WantedCount: len(items), Fallback: "none", Items: items})
	if fill {
		m.TargetSec = new(len(items) * 30)
		m = Seal(m)
	}
	template, err := ResolveTemplate(context.Background(), h.s.Job.TemplateSnapshot, h.store)
	if err != nil {
		t.Fatal(err)
	}
	return h, Editor{Deps: h.d}, EditState{JobID: h.s.Job.ID, Manifest: &m, Template: template}
}
func mustEdit(t *testing.T, e Editor, state EditState, r EditRequest) EditResult {
	t.Helper()
	result, err := e.Edit(context.Background(), state, r)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func assertCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *domain.CloneError
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("error=%v want code=%s", err, code)
	}
}
func TestFullRerollKeepsOwnerAndInvalidatesBackdrop(t *testing.T) {
	_, e, state := editFixture(t, false, []Item{testItem("b1", "twitch:a-v0", "random", "ready"), testItem("b2", "twitch:a-v1", "owner", "ready"), testItem("b3", "twitch:a-v2", "random", "ready")})
	result := mustEdit(t, e, state, EditRequest{Action: "reroll", Revision: state.Manifest.Revision})
	if result.InvalidateFrom != domain.CloneBackdrop || len(result.Manifest.Items) != 3 {
		t.Fatal(result)
	}
	owner := state.Manifest.Items[1]
	found := false
	for _, v := range result.Manifest.Items {
		if v.Source == "owner" {
			found = true
			if !reflect.DeepEqual(v, owner) {
				t.Fatal("owner changed")
			}
		} else if v.Status != "pending" || v.VideoID == "twitch:a-v0" || v.VideoID == "twitch:a-v2" {
			t.Fatal(v)
		}
	}
	if !found {
		t.Fatal("owner lost")
	}
}
func TestEditRevisionBusyAndSupersededRefusal(t *testing.T) {
	_, e, state := editFixture(t, false, []Item{testItem("b1", "twitch:a-v0", "random", "ready")})
	original := *state.Manifest
	_, err := e.Edit(context.Background(), state, EditRequest{Action: "remove", ItemID: "b1", Revision: "old"})
	assertCode(t, err, domain.CloneErrConflict)
	state.Busy = true
	_, err = e.Edit(context.Background(), state, EditRequest{Action: "remove", ItemID: "b1"})
	assertCode(t, err, domain.CloneErrConflict)
	state.Busy = false
	state.Superseded = true
	_, err = e.Edit(context.Background(), state, EditRequest{Action: "remove", ItemID: "b1"})
	assertCode(t, err, domain.CloneErrNotFound)
	if !reflect.DeepEqual(*state.Manifest, original) {
		t.Fatal("refused edit mutated state")
	}
}
func TestRemoveOnlyRebuildsAndLowersCount(t *testing.T) {
	_, e, state := editFixture(t, false, []Item{testItem("b1", "twitch:a-v0", "random", "ready"), testItem("b2", "twitch:a-v1", "random", "ready")})
	r := mustEdit(t, e, state, EditRequest{Action: "remove", ItemID: "b1"})
	if r.InvalidateFrom != domain.CloneBuild || r.Manifest.WantedCount != 1 || len(r.Manifest.Items) != 1 || r.Manifest.Items[0].ID != "b2" {
		t.Fatal(r)
	}
}
func TestSwapChecksWindowAndEvidenceCatalogue(t *testing.T) {
	h, e, state := editFixture(t, false, []Item{testItem("b1", "twitch:a-v0", "random", "ready")})
	_, err := e.Edit(context.Background(), state, EditRequest{Action: "swap", ItemID: "b1", VideoChoice: VideoChoice{ChannelID: "twitch:a", VideoID: "twitch:a-v1", StartSec: new(290.0), EndSec: new(330.0)}})
	assertCode(t, err, domain.CloneErrInvalidRequest)
	r := mustEdit(t, e, state, EditRequest{Action: "swap", ItemID: "b1", VideoChoice: VideoChoice{ChannelID: "twitch:a", VideoID: "twitch:a-v1", StartSec: new(20.0), EndSec: new(50.0)}})
	v := r.Manifest.Items[0]
	if v.Source != "owner" || v.Status != "pending" || v.StartSec != 20 || v.EndSec != 50 || v.ID != "b1" || r.InvalidateFrom != domain.CloneBackdrop {
		t.Fatal(r)
	}
	view, err := Evidence(context.Background(), h.store, EvidenceInput{Manifest: &r.Manifest, TemplateSnapshot: h.s.Job.TemplateSnapshot})
	if err != nil {
		t.Fatal(err)
	}
	if !view.RandomPicks || view.Items[0].ChannelTitle == nil || *view.Items[0].VideoDurationSec != 300 {
		t.Fatal(view)
	}
}
func TestFillRemoveReducesSourceGoalAndExhaustedManifest(t *testing.T) {
	for _, tc := range []struct{ target, n, want int }{{100, 4, 70}, {300, 2, 30}} {
		items := []Item{}
		for i := range tc.n {
			items = append(items, testItem(string(rune('a'+i)), "v", "random", "ready"))
		}
		_, e, state := editFixture(t, true, items)
		state.Manifest.TargetSec = new(tc.target)
		r := mustEdit(t, e, state, EditRequest{Action: "remove", ItemID: items[0].ID})
		if *r.Manifest.TargetSec != tc.want || r.Manifest.WantedCount != tc.n-1 {
			t.Fatal(r)
		}
	}
}
func TestAddRaisesOnlyFillSourceGoal(t *testing.T) {
	for _, fill := range []bool{false, true} {
		_, e, state := editFixture(t, fill, []Item{testItem("b1", "twitch:a-v0", "random", "ready")})
		r := mustEdit(t, e, state, EditRequest{Action: "add", VideoChoice: VideoChoice{ChannelID: "twitch:a", VideoID: "twitch:a-v1", StartSec: new(10.0), EndSec: new(55.0)}})
		if r.Manifest.WantedCount != 2 || r.Manifest.Items[1].ID != "b2" {
			t.Fatal(r)
		}
		if fill {
			if *r.Manifest.TargetSec != 75 {
				t.Fatal(r)
			}
		} else if r.Manifest.TargetSec != nil {
			t.Fatal(r)
		}
	}
}
func TestFillFullRerollAndSingleReroll(t *testing.T) {
	_, e, state := editFixture(t, true, []Item{testItem("b1", "twitch:a-v0", "random", "ready"), testItem("b2", "twitch:a-v1", "owner", "ready"), testItem("b3", "twitch:a-v2", "random", "ready")})
	state.Manifest.TargetSec = new(100)
	r := mustEdit(t, e, state, EditRequest{Action: "reroll"})
	if *r.Manifest.TargetSec != 100 || r.Manifest.WantedCount != 4 || len(r.Manifest.Items) != 4 || r.Manifest.Items[0].ID != "b2" {
		t.Fatal(r)
	}
	one := mustEdit(t, e, state, EditRequest{Action: "reroll", ItemID: "b1"})
	if len(one.Manifest.Items) != 3 || one.Manifest.Items[0].ID != "b1" || one.Manifest.Items[0].VideoID == "twitch:a-v0" || *one.Manifest.TargetSec != 100 {
		t.Fatal(one)
	}
}
func TestEvidenceCoverageBeforeAfterAndStoredMode(t *testing.T) {
	h, _, state := editFixture(t, true, []Item{testItem("b1", "twitch:a-v0", "random", "ready"), testItem("b2", "twitch:a-v1", "random", "pending")})
	h.speed(1, true)
	view, err := Evidence(context.Background(), h.store, EvidenceInput{TemplateSnapshot: h.s.Job.TemplateSnapshot})
	if err != nil {
		t.Fatal(err)
	}
	if view.Coverage.Mode != "fill" || view.Coverage.FixedSec != 10 || view.Coverage.ChannelSec != 0 {
		t.Fatal(view)
	}
	view, err = Evidence(context.Background(), h.store, EvidenceInput{Manifest: state.Manifest, TemplateSnapshot: h.s.Job.TemplateSnapshot, VideoSec: new(300.0), Busy: true})
	if err != nil {
		t.Fatal(err)
	}
	if view.Coverage.ChannelSec != 30 || *view.Coverage.VideoSec != 300 || !view.Busy {
		t.Fatal(view)
	}
	state.Manifest.TargetSec = nil
	view, err = Evidence(context.Background(), h.store, EvidenceInput{Manifest: state.Manifest, TemplateSnapshot: h.s.Job.TemplateSnapshot})
	if err != nil || view.Coverage.Mode != "count" {
		t.Fatal(view, err)
	}
}
func TestUnknownOwnerProbeReportsRecovery(t *testing.T) {
	h, e, state := editFixture(t, false, []Item{testItem("b1", "twitch:a-v0", "random", "ready")})
	h.store.videos["twitch:a"][1].DurationSec = nil
	e.Deps.ProbeVideo = func(context.Context, string, string) (domain.BackdropChannelVideo, error) {
		return domain.BackdropChannelVideo{}, &tools.YtDlpError{Code: tools.YtDlpLoginRequired, Message: "members only"}
	}
	_, err := e.Edit(context.Background(), state, EditRequest{Action: "swap", ItemID: "b1", VideoChoice: VideoChoice{ChannelID: "twitch:a", VideoID: "twitch:a-v1"}})
	assertCode(t, err, domain.CloneErrInvalidRequest)
	var typed *domain.CloneError
	if !errors.As(err, &typed) || typed.Recovery == nil || *typed.Recovery == "" {
		t.Fatal(err)
	}
}
func TestEvidenceSharedSpeedIsReadOnly(t *testing.T) {
	h, _, state := editFixture(t, true, []Item{testItem("b1", "twitch:a-v0", "random", "ready"), testItem("b2", "twitch:a-v1", "owner", "ready"), testItem("b3", "twitch:a-v2", "random", "pending"), testItem("b4", "twitch:a-v3", "random", "failed")})
	state.Manifest.TargetSec = new(77)
	before, _ := json.Marshal(state.Manifest)
	for _, speed := range []float64{.5, 2} {
		h.speed(speed, true)
		view, err := Evidence(context.Background(), h.store, EvidenceInput{Manifest: state.Manifest, TemplateSnapshot: h.s.Job.TemplateSnapshot, VideoSec: new(100.0)})
		if err != nil {
			t.Fatal(err)
		}
		if view.Coverage.ChannelSec != 60/speed || view.Coverage.FixedSec != 10/speed || view.Revision != state.Manifest.Revision {
			t.Fatal(view)
		}
		after, _ := json.Marshal(state.Manifest)
		if string(before) != string(after) {
			t.Fatal("read model mutated manifest")
		}
	}
}
func TestEditsKeepStoredGoalsAfterSpeedChanges(t *testing.T) {
	for _, speed := range []float64{.5, 2} {
		h, e, state := editFixture(t, true, []Item{testItem("b1", "twitch:a-v0", "random", "ready"), testItem("b2", "twitch:a-v1", "owner", "ready"), testItem("b3", "twitch:a-v2", "random", "ready")})
		h.speed(speed, false)
		template, err := ResolveTemplate(context.Background(), h.s.Job.TemplateSnapshot, h.store)
		if err != nil {
			t.Fatal(err)
		}
		state.Template = template
		r := mustEdit(t, e, state, EditRequest{Action: "add", VideoChoice: VideoChoice{ChannelID: "twitch:a", VideoID: "twitch:a-v3", StartSec: new(10.0), EndSec: new(55.0)}})
		if *r.Manifest.TargetSec != 135 {
			t.Fatal(r)
		}
		state.Manifest = &r.Manifest
		r = mustEdit(t, e, state, EditRequest{Action: "remove", ItemID: "b1"})
		if *r.Manifest.TargetSec != 105 {
			t.Fatal(r)
		}
		state.Manifest = &r.Manifest
		r = mustEdit(t, e, state, EditRequest{Action: "reroll"})
		random, owner := 0, 0
		for _, v := range r.Manifest.Items {
			if v.Source == "random" {
				random++
			} else {
				owner++
			}
		}
		if *r.Manifest.TargetSec != 105 || random != 1 || owner != 2 {
			t.Fatal(r)
		}
		view, err := Evidence(context.Background(), h.store, EvidenceInput{Manifest: &r.Manifest, TemplateSnapshot: h.s.Job.TemplateSnapshot})
		if err != nil || view.Coverage.ChannelSec != 30/speed {
			t.Fatal(view, err)
		}
	}
}
func TestEditBeforeStepUsesSpeedExpandedInitialGoal(t *testing.T) {
	for _, speed := range []float64{.5, 2} {
		h, e, state := editFixture(t, true, nil)
		h.speed(speed, true)
		template, err := ResolveTemplate(context.Background(), h.s.Job.TemplateSnapshot, h.store)
		if err != nil {
			t.Fatal(err)
		}
		state.Template = template
		state.Manifest = nil
		state.VideoSec = new(130.0)
		r := mustEdit(t, e, state, EditRequest{Action: "add", VideoChoice: VideoChoice{ChannelID: "twitch:a", VideoID: "twitch:a-v0", StartSec: new(0.0), EndSec: new(30.0)}})
		if float64(*r.Manifest.TargetSec) != 130*speed-10+30 {
			t.Fatal(r)
		}
	}
}
func TestMissingSnapshotRetainsIdentityCoverage(t *testing.T) {
	h, _, state := editFixture(t, false, []Item{testItem("b1", "twitch:a-v0", "random", "ready")})
	for _, raw := range [][]byte{nil, []byte("broken")} {
		view, err := Evidence(context.Background(), h.store, EvidenceInput{Manifest: state.Manifest, TemplateSnapshot: raw, VideoSec: new(100.0)})
		if err != nil || view.Coverage.ChannelSec != 30 || view.Coverage.FixedSec != 0 || view.Coverage.Mode != "count" {
			t.Fatal(view, err)
		}
	}
	view, err := Evidence(context.Background(), h.store, EvidenceInput{})
	if view != nil || err != nil {
		t.Fatal(view, err)
	}
}
func TestOwnerWindowBoundaryValidation(t *testing.T) {
	for _, choice := range []VideoChoice{{StartSec: new(1.0)}, {EndSec: new(20.0)}, {StartSec: new(-1.0), EndSec: new(20.0)}, {StartSec: new(.5), EndSec: new(20.0)}, {StartSec: new(20.0), EndSec: new(20.0)}, {StartSec: new(0.0), EndSec: new(9.0)}, {StartSec: new(0.0), EndSec: new(601.0)}, {StartSec: new(math.NaN()), EndSec: new(20.0)}, {StartSec: new(0.0), EndSec: new(math.Inf(1))}} {
		if _, _, err := OwnerWindow(choice, new(1000.0), 30, 0); err == nil {
			t.Fatal(choice)
		}
	}
	if _, _, err := OwnerWindow(VideoChoice{}, nil, 30, 0); err == nil {
		t.Fatal("unknown duration accepted")
	}
	if _, _, err := OwnerWindow(VideoChoice{}, new(5.0), 30, 0); err == nil {
		t.Fatal("short video accepted")
	}
	a, b, err := OwnerWindow(VideoChoice{}, new(20.0), 60, 0)
	if err != nil || a != 0 || b != 20 {
		t.Fatal(a, b, err)
	}
}
func TestEditFailureBoundaries(t *testing.T) {
	h, e, state := editFixture(t, false, []Item{testItem("b1", "twitch:a-v0", "owner", "ready")})
	for _, r := range []EditRequest{{Action: "remove", ItemID: "absent"}, {Action: "swap", ItemID: "absent"}, {Action: "reroll"}, {Action: "add", VideoChoice: VideoChoice{ChannelID: "twitch:a"}}, {Action: "invalid"}} {
		if _, err := e.Edit(context.Background(), state, r); err == nil {
			t.Fatal(r)
		}
	}
	state.Manifest.WantedCount = 40
	if _, err := e.Edit(context.Background(), state, EditRequest{Action: "add"}); err == nil {
		t.Fatal("limit accepted")
	}
	state.Manifest.WantedCount = 1
	state.Template.Backdrop = nil
	if _, err := e.Edit(context.Background(), state, EditRequest{Action: "add"}); err == nil {
		t.Fatal("random source absent")
	}
	h.store.videos["twitch:a"][1].UnavailableAt = &testNow
	if _, err := e.Edit(context.Background(), state, EditRequest{Action: "swap", ItemID: "b1", VideoChoice: VideoChoice{ChannelID: "twitch:a", VideoID: "twitch:a-v1"}}); err == nil {
		t.Fatal("unavailable owner choice")
	}
}
