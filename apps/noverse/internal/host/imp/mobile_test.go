package imp

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestMobilePreviewLongChapterThreshold(t *testing.T) {
	p, err := PreviewSource(writePreviewSource(t, "dai.txt", "Chương 1\nmột hai ba bốn năm sáu\nChương 2\nmột\nChương 3\nmột hai\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != 1 || p.Filename != "dai.txt" || p.Size == 0 || p.Words == 0 || p.SplitHow == "" {
		t.Fatalf("missing file metadata: %+v", p)
	}
	if p.Chapters[0].Warning == "" || len(p.Warnings) != 1 {
		t.Fatalf("exactly twice-average chapter must warn: %+v", p)
	}
	if p.Chapters[1].Warning != "" || p.Chapters[2].Warning != "" {
		t.Fatalf("ordinary chapters incorrectly warned: %+v", p.Chapters)
	}
}

func TestMobileGuidedPreviewMatchesConfirmedChunks(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(st.Dir(), "truyen.txt")
	text := "Chương 1\nmột hai\nHồi tiếp\nba bốn\n"
	if err := os.WriteFile(src, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	one := boundariesJSON(boundaryFixture("L1", "", kindChapter, "Chương 1"))
	two := boundariesJSON(boundaryFixture("L1", "", kindChapter, "Chương 1"), boundaryFixture("L3", "", kindChapter, "Hồi tiếp"))
	m := &mockModel{responses: []string{one, two, `{"chapters":[` + factsJSON(1, "Chương 1") + `,` + factsJSON(2, "Hồi tiếp") + `]}`, synthesisFixtureJSON(2, storyOpen)}}
	deps := testDeps(st, m)
	p, err := PreparePreview(context.Background(), deps, Options{SourcePath: src})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Chapters) != 1 {
		t.Fatalf("initial preview=%+v", p)
	}
	guide := "Hồi tiếp là chương riêng, giữ nguyên dấu cách"
	p, err = PreparePreview(context.Background(), deps, Options{Guidance: guide})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Chapters) != 2 || !p.Chapters[0].NewlySplit || !p.Chapters[1].NewlySplit {
		t.Fatalf("guided split markers missing: %+v", p.Chapters)
	}
	state, err := LoadPreviewState(st.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if state.Options.Guide != guide || !reflect.DeepEqual(state.Preview, &p) {
		t.Fatalf("reconnect preview differs: %+v", state)
	}
	again, err := PreparePreview(context.Background(), deps, Options{Guidance: guide})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, p) {
		t.Fatalf("repeated preview lost split markers: %+v", again)
	}
	ws := OpenWorkspace(st.Dir())
	before, err := ws.readBytes(fileSegmentation)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := Run(context.Background(), deps, Options{AcceptSegmentation: true, StoryResolution: storyOpen, ContinueAfter: true})
	if err != nil {
		t.Fatal(err)
	}
	var summary *Summary
	for ev := range ch {
		if ev.Stage == StageError {
			t.Fatal(ev.Err)
		}
		if ev.Stage == StageDone {
			summary = ev.Summary
		}
	}
	if summary == nil || summary.SchemaVersion != 1 || summary.Chapters != 2 || summary.Characters != 1 || summary.Foreshadows != 0 {
		t.Fatalf("completion summary=%+v", summary)
	}
	after, err := ws.readBytes(fileSegmentation)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("confirmation re-split the reviewed segmentation")
	}
	art, err := readArtifact[Segmentation](ws, fileSegmentation)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range art.Payload.Chapters {
		imported, err := st.Drafts.LoadChapterText(c.Number)
		if err != nil {
			t.Fatal(err)
		}
		// The shared commit tool renders a canonical H1; all chunk content must
		// otherwise remain exactly the reviewed source range.
		expected := domain.ApplyChapterHeading(art.Payload.Content([]byte(text), i), c.Title, c.Number)
		if strings.TrimSpace(imported) != strings.TrimSpace(expected) {
			t.Fatalf("imported chapter %d differs from confirmed chunk: %q", c.Number, imported)
		}
	}
}

func TestMobileImportChoicesPersist(t *testing.T) {
	st := store.NewStore(t.TempDir())
	src := writePreviewSource(t, "nguon.txt", "Chương 1\nnội dung\n")
	ws, _, err := Ingest(st.Dir(), src, Options{ContinueAfter: true, StoryResolution: storyOpen}.intent())
	if err != nil {
		t.Fatal(err)
	}
	r := &runner{deps: Deps{Store: st}, ws: ws, opts: Options{ContinueAfterSet: true, ContinueAfter: false, StoryResolution: storyClosed}}
	if err := r.applyOptions(); err != nil {
		t.Fatal(err)
	}
	in, err := ws.LoadIntent()
	if err != nil {
		t.Fatal(err)
	}
	if in.ContinueAfterImport || in.StoryResolution != storyClosed {
		t.Fatalf("explicit confirm choices were not persisted: %+v", in)
	}
	if err := st.RunMeta.Init("", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.setCompletionHold(); err != nil {
		t.Fatal(err)
	}
	meta, err := st.RunMeta.Load()
	if err != nil {
		t.Fatal(err)
	}
	if meta.AdvanceHold == nil {
		t.Fatal("turning --continue off must restore import completion hold")
	}
}

func TestMobileStoryChoiceOverridesModel(t *testing.T) {
	st := store.NewStore(t.TempDir())
	ws, _, err := Ingest(st.Dir(), writePreviewSource(t, "nguon.txt", "Chương 1\nnội dung\n"), Options{StoryResolution: storyClosed}.intent())
	if err != nil {
		t.Fatal(err)
	}
	if err := writeArtifact(ws, fileSynthesis, "test", BookSynthesis{StoryStatus: storyOpen}); err != nil {
		t.Fatal(err)
	}
	r := &runner{ws: ws, opts: Options{StoryResolution: storyClosed}}
	closed, err := r.resolveStory(&BookSynthesis{StoryStatus: storyOpen})
	if err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Fatal("explicit --story=closed must override model open")
	}
	r.opts.StoryResolution = storyOpen
	open, err := r.resolveStory(&BookSynthesis{StoryStatus: storyClosed})
	if err != nil {
		t.Fatal(err)
	}
	if open {
		t.Fatal("explicit --story=open must override model closed")
	}
}

func TestMobileImportProgressFiveSteps(t *testing.T) {
	p := AdvanceProgress(nil, Event{Stage: StageAnalyzing, Current: 13, Total: 24}, time.Now().Add(-26*time.Second))
	if p.SchemaVersion != 1 || len(p.Steps) != 5 || p.Steps[2].Current != 13 || p.Steps[2].Total != 24 || p.Steps[2].Status != "running" {
		t.Fatalf("five-step progress=%+v", p)
	}
	if p.Steps[0].Status != "done" || p.Steps[1].Current != 24 || p.Steps[3].Status != "pending" || p.EtaSec == nil || *p.EtaSec < 21 {
		t.Fatalf("counts/ETA not measured: %+v", p)
	}
	retry := AdvanceProgress(p, Event{Stage: StageAnalyzing, Level: "warn"}, time.Time{})
	if retry.Steps[2].Current != 13 || p.Steps[2].Status != "running" {
		t.Fatalf("retry lost counts or mutated previous projection: %+v", retry)
	}
	done := AdvanceProgress(retry, Event{Stage: StageDone, Total: 24, Continued: true, Summary: &Summary{SchemaVersion: 1, Chapters: 24, Characters: 14, Foreshadows: 9}}, time.Time{})
	for _, step := range done.Steps {
		if step.Status != "done" || step.Total != step.Current {
			t.Fatalf("incomplete final step=%+v", step)
		}
	}
	if !done.Continued || done.Summary == nil || done.Summary.Characters != 14 || done.Summary.Foreshadows != 9 || done.EtaSec != nil {
		t.Fatalf("final progress=%+v", done)
	}
}
