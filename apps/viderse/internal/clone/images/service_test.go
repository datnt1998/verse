package images

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

type editStore struct {
	m             model.Manifest
	commits       int
	change        jobedit.Change
	sceneRevision int
	sceneStatus   domain.CloneStatus
	artifacts     []domain.CloneArtifact
}

func (s *editStore) Read(_ context.Context, id string, step domain.CloneStep) (jobedit.Snapshot, error) {
	raw, err := json.Marshal(s.m)
	if err != nil {
		return jobedit.Snapshot{}, err
	}
	if step == domain.CloneScenes {
		revision := s.sceneRevision
		if revision == 0 {
			revision = 1
		}
		raw, err = json.Marshal(struct {
			Version  int `json:"version"`
			Revision int `json:"revision"`
		}{1, revision})
		if err != nil {
			return jobedit.Snapshot{}, err
		}
		return jobedit.Snapshot{Job: domain.CloneJob{ID: id, Generation: 1}, Step: domain.CloneStepRecord{Step: step, Status: s.sceneStatus, Checkpoint: raw}, Manifest: raw}, nil
	}
	return jobedit.Snapshot{Job: domain.CloneJob{ID: id, Generation: 1}, Step: domain.CloneStepRecord{Step: step, Checkpoint: raw}, Manifest: raw, Artifacts: s.artifacts}, nil
}
func (s *editStore) Commit(_ context.Context, _ jobedit.Snapshot, c jobedit.Change) error {
	s.commits++
	s.change = c
	return json.Unmarshal(c.Checkpoint, &s.m)
}
func testService() (*Service, *editStore) {
	n := 1
	a := model.Item{ID: "scene-1", Kind: "scene", Narration: "scene", Status: "review", ReviewVersion: &n}
	b := model.Item{ID: "thumbnail", Kind: "thumbnail", Width: 1280, Height: 720, Status: "review", ReviewVersion: &n}
	for _, i := range []*model.Item{&a, &b} {
		i.Versions = []model.Version{{Version: 1, SourceFingerprint: model.SourceFingerprint(*i)}}
	}
	store := &editStore{m: model.Manifest{Version: 1, Revision: 3, SceneRevision: 1, Style: "full", Items: []model.Item{a, b}}}
	return &Service{Edits: &jobedit.Service{Store: store}}, store
}
func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var ce *domain.CloneError
	if !errors.As(err, &ce) || ce.Code != code {
		t.Fatalf("got %v, want %s", err, code)
	}
}
func TestMutationRequiresCurrentRevision(t *testing.T) {
	s, store := testService()
	for _, rev := range []*int{nil, new(int)} {
		_, err := s.ApproveAll(context.Background(), "job", RevisionInput{ExpectedRevision: rev})
		requireCode(t, err, domain.CloneErrConflict)
	}
	if store.commits != 0 {
		t.Fatal("committed stale mutation")
	}
}
func TestRedrawAllConfirmationMatchesEligibleCount(t *testing.T) {
	s, store := testService()
	rev := 3
	_, err := s.RedrawAll(context.Background(), "job", ResetInput{ExpectedRevision: &rev, ConfirmCount: 1})
	requireCode(t, err, domain.CloneErrConflict)
	if store.commits != 0 {
		t.Fatal("charged without confirmation")
	}
}

func TestRedrawAllAppliesSharedFeedback(t *testing.T) {
	s, store := testService()
	s.Edits.Media = &stepstest.FakeMedia{Root: t.TempDir()}
	var input ResetInput
	if err := json.Unmarshal([]byte(`{"expectedRevision":3,"confirmCount":2,"feedback":"Use warmer light"}`), &input); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedrawAll(context.Background(), "job", input); err != nil {
		t.Fatal(err)
	}
	for _, item := range store.m.Items {
		if item.Direction != "Use warmer light" {
			t.Fatalf("%s direction = %q", item.ID, item.Direction)
		}
	}
}

func TestImageViewsExposeAuthorizedArtifactURLs(t *testing.T) {
	s, store := testService()
	store.m.Items[0].Versions[0].Path = "images/a.webp"
	store.m.Items[0].Versions[0].SHA256 = "hash"
	store.m.Items[0].Reference.Path = "scenes/reference.jpg"
	store.artifacts = []domain.CloneArtifact{
		{ID: "image-artifact", Path: "clone/job/hash/images/a.webp", SHA256: "hash"},
		{ID: "reference-artifact", Path: "clone/job/ref/scenes/reference.jpg"},
	}
	view, err := s.Read(context.Background(), "job")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(view.Items[0])
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	version := wire["versions"].([]any)[0].(map[string]any)
	if version["originalUrl"] != "/api/clone/job/artifacts/image-artifact" || wire["referenceUrl"] != "/api/clone/job/artifacts/reference-artifact" {
		t.Fatalf("missing authorized URLs: %s", raw)
	}
}
func TestCoverRejectsMagicOnlyAndNonImagesBeforeEncoding(t *testing.T) {
	for _, data := range [][]byte{[]byte("video renamed.png"), {137, 80, 78, 71, 13, 10, 26, 10}} {
		file := filepath.Join(t.TempDir(), "cover.png")
		if err := os.WriteFile(file, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateCover(file); err == nil {
			t.Fatal("accepted malformed image")
		}
	}
}
func TestCoverAcceptsDecodedPngRegardlessOfExtension(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cover.bin")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	mime, err := ValidateCover(file)
	if err != nil || mime != "image/png" {
		t.Fatalf("%s %v", mime, err)
	}
}
func TestVideoOverrideNullAndOmissionAreDistinct(t *testing.T) {
	var in OverrideInput
	if err := json.Unmarshal([]byte(`{"scenePromptId":null,"noImages":false}`), &in); err != nil {
		t.Fatal(err)
	}
	if !in.ScenePromptID.Set || in.ScenePromptID.Value != nil || in.CoverPromptID.Set || in.NoImages == nil || *in.NoImages {
		t.Fatalf("lost nullable override: %+v", in)
	}
}
func TestOverridePlanDoesNotChargeUndrawnItems(t *testing.T) {
	s, store := testService()
	store.m.Items[0].Versions = nil
	store.m.Items[0].ReviewVersion = nil
	store.m.Items[0].Status = "pending"
	s.Prompts = promptCatalog{}
	s.QuotaLeft = func(context.Context) (int, error) { return 7, nil }
	id := "scene-custom"
	plan, err := s.PlanOverrides(context.Background(), "job", OverrideInput{ScenePromptID: OptionalString{Set: true, Value: &id}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Cost.Images != 0 || plan.Redraw.Scenes != 0 || plan.QuotaLeft != 7 {
		t.Fatalf("wrong plan %+v", plan)
	}
}

type promptCatalog struct{}

func (promptCatalog) GetPrompt(_ context.Context, id string) (*domain.ImagePrompt, error) {
	return &domain.ImagePrompt{ID: id, Kind: domain.PromptScene, Prompt: "custom visual direction"}, nil
}
func TestRedrawPublishesCheckpointAndQueuesOnlySelectedImage(t *testing.T) {
	s, store := testService()
	media := &stepstest.FakeMedia{Root: t.TempDir()}
	s.Edits.Media = media
	rev := 3
	feedback := "Use warmer light"
	out, err := s.Redraw(context.Background(), "job", "scene-1", RedrawInput{ExpectedRevision: &rev, Prompt: &feedback})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Accepted || len(out.Pending) != 1 || out.Pending[0] != "scene-1" || store.change.InvalidateFrom != domain.CloneImages || !store.change.Rerun {
		t.Fatalf("not queued: %+v %+v", out, store.change)
	}
	if store.m.Items[0].Direction != feedback || store.m.Items[1].Status != "review" || len(store.m.Items[0].Versions) != 1 {
		t.Fatalf("redraw lost history or touched cover: %+v", store.m)
	}
	if media.Commits != 1 || len(media.Published) != 1 {
		t.Fatalf("manifest not published atomically: %+v", media)
	}
}
func TestApproveAllCommitsBuildInvalidation(t *testing.T) {
	s, store := testService()
	s.Edits.Media = &stepstest.FakeMedia{Root: t.TempDir()}
	rev := 3
	_, err := s.ApproveAll(context.Background(), "job", RevisionInput{ExpectedRevision: &rev})
	if err != nil {
		t.Fatal(err)
	}
	if !model.ImagesApproved(store.m) || store.change.InvalidateFrom != domain.CloneBuild || store.change.Rerun {
		t.Fatalf("wrong approval commit: %+v", store.change)
	}
}
func TestSceneOverridePersistsWithoutResettingCover(t *testing.T) {
	s, store := testService()
	s.Edits.Media = &stepstest.FakeMedia{Root: t.TempDir()}
	s.Prompts = promptCatalog{}
	s.QuotaLeft = func(context.Context) (int, error) { return 9, nil }
	rev := 3
	id := "custom-scene"
	plan, err := s.ApplyOverrides(context.Background(), "job", OverrideInput{ExpectedRevision: &rev, ScenePromptID: OptionalString{Set: true, Value: &id}, ConfirmCount: new(1)})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Redraw.Scenes != 1 || plan.Redraw.Cover || !plan.Rebuild || store.m.Overrides.ScenePromptID != id || store.m.Items[1].Status != "review" {
		t.Fatalf("bad video override: %+v %+v", plan, store.m)
	}
}
func TestApprovalRejectsChangedOrRunningSceneSource(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		revision   int
		status     domain.CloneStatus
	}{{"changed", domain.CloneErrStale, 2, domain.CloneSucceeded}, {"running", domain.CloneErrConflict, 1, domain.CloneRunning}} {
		t.Run(tc.name, func(t *testing.T) {
			s, store := testService()
			store.sceneRevision = tc.revision
			store.sceneStatus = tc.status
			rev := 3
			_, err := s.ApproveAll(context.Background(), "job", RevisionInput{ExpectedRevision: &rev})
			requireCode(t, err, tc.code)
			if store.commits != 0 {
				t.Fatal("approved stale source")
			}
		})
	}
}
func TestFinalCoverApprovalReleasesInitialBuildGate(t *testing.T) {
	s, store := testService()
	s.Edits.Media = &stepstest.FakeMedia{Root: t.TempDir()}
	n := 1
	store.m.Items[0].Status = "approved"
	store.m.Items[0].CurrentVersion = &n
	store.m.Items[0].ReviewVersion = nil
	rev := 3
	_, err := s.Review(context.Background(), "job", ReviewInput{ExpectedRevision: &rev, Actions: []Action{{ID: "thumbnail", Op: "approve"}}})
	if err != nil {
		t.Fatal(err)
	}
	if store.change.InvalidateFrom != domain.CloneBuild {
		t.Fatal("final cover approval did not release initial build review gate")
	}
}

func TestVideoOverrideRequiresConfirmedRedrawCount(t *testing.T) {
	for _, confirmed := range []*int{nil, new(0), new(2)} {
		s, store := testService()
		s.Edits.Media = &stepstest.FakeMedia{Root: t.TempDir()}
		s.Prompts = promptCatalog{}
		s.QuotaLeft = func(context.Context) (int, error) { return 9, nil }
		revision, id := 3, "custom-scene"
		input := OverrideInput{ExpectedRevision: &revision, ScenePromptID: OptionalString{Set: true, Value: &id}}
		raw, _ := json.Marshal(map[string]any{"expectedRevision": revision, "scenePromptId": id, "confirmCount": confirmed})
		if err := json.Unmarshal(raw, &input); err != nil {
			t.Fatal(err)
		}
		_, err := s.ApplyOverrides(context.Background(), "job", input)
		requireCode(t, err, domain.CloneErrConflict)
		if store.commits != 0 {
			t.Fatal("unconfirmed override queued paid redraw")
		}
	}
}
