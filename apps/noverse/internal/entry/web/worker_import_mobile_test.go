package web

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/noverse/internal/host/imp"
)

// fakeHost embeds a nil *host.Host. Explicit test adapters keep the new Host
// methods from being promoted through that nil pointer in older worker tests.
func (h *fakeHost) ImportPreview(_ context.Context, opts imp.Options) (imp.Preview, error) {
	return imp.PreviewSource(opts.SourcePath)
}

func (h *fakeHost) ImportState() (imp.PreviewState, error) {
	return imp.PreviewState{SchemaVersion: 1, Options: imp.PreviewOptions{SchemaVersion: 1}}, nil
}

type mobileImportHost struct {
	*liveFakeHost
	state          imp.PreviewState
	previewOptions imp.Options
}

func (h *mobileImportHost) ImportPreview(_ context.Context, opts imp.Options) (imp.Preview, error) {
	h.previewOptions = opts
	return *h.state.Preview, nil
}
func (h *mobileImportHost) ImportState() (imp.PreviewState, error) { return h.state, nil }

func newMobileImportHost() *mobileImportHost {
	return &mobileImportHost{liveFakeHost: newLiveFakeHost(), state: imp.PreviewState{
		SchemaVersion: 1, Path: "nguon.txt", Options: imp.PreviewOptions{SchemaVersion: 1, Guide: "tách ở dấu sao", StoryStatus: "open"},
		Preview: &imp.Preview{SchemaVersion: 1, Filename: "nguon.txt", Chapters: []imp.PreviewChapter{{N: 1, Title: "Khởi đầu", NewlySplit: true}}, Warnings: []string{}, Encoding: "utf-8"},
	}}
}

func TestMobileImportHTTPStateAndGuide(t *testing.T) {
	h := newMobileImportHost()
	s := newLiveServer(h)
	s.storyDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(s.storyDir, "nguon.txt"), []byte("nguồn"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := doReq(t, s, "POST", "/api/import/preview", `{"path":"nguon.txt","guide":"tách ở dấu sao"}`)
	if w.Code != 200 {
		t.Fatalf("guided preview=%d %s", w.Code, w.Body.String())
	}
	if h.previewOptions.Guidance != "tách ở dấu sao" || h.previewOptions.SourcePath != filepath.Join(s.storyDir, "nguon.txt") {
		t.Fatalf("Architect preview options=%+v", h.previewOptions)
	}
	w = doReq(t, s, "GET", "/api/import", "")
	body := decodeLiveBody(t, w)
	if body["schemaVersion"] != float64(1) || body["path"] != "nguon.txt" || body["preview"] == nil || body["options"].(map[string]any)["guide"] != "tách ở dấu sao" {
		t.Fatalf("reload state=%v", body)
	}
	w = doReq(t, s, "POST", "/api/import/preview", `{"path":"","guide":"chia lại từ bản chụp"}`)
	if w.Code != 200 || h.previewOptions.SourcePath != "" || h.previewOptions.Guidance != "chia lại từ bản chụp" {
		t.Fatalf("snapshot re-preview=%d %+v", w.Code, h.previewOptions)
	}
}

func TestMobileImportConfirmChoicesAndUnseenGuide(t *testing.T) {
	h := newMobileImportHost()
	s := newLiveServer(h)
	w := doReq(t, s, "POST", "/api/import/confirm", `{"accept":true,"storyStatus":"closed","continueAfter":false,"guide":"chưa xem hướng dẫn này"}`)
	if w.Code != 409 {
		t.Fatalf("unseen split must not be accepted: %d %s", w.Code, w.Body.String())
	}
	w = doReq(t, s, "POST", "/api/import/confirm", `{"accept":true,"storyStatus":"closed","continueAfter":false,"guide":"tách ở dấu sao"}`)
	if w.Code != 200 {
		t.Fatalf("confirm=%d %s", w.Code, w.Body.String())
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.impOpts) != 1 {
		t.Fatalf("unexpected import calls: %+v", h.impOpts)
	}
	opts := h.impOpts[0]
	if !opts.AcceptSegmentation || opts.StoryResolution != "closed" || !opts.ContinueAfterSet || opts.ContinueAfter || opts.Guidance != "" {
		t.Fatalf("confirmed choices=%+v", opts)
	}
}

func TestMobileImportPumpCarriesFinalCounts(t *testing.T) {
	s := newLiveServer(newLiveFakeHost())
	id, ok := s.jobs.begin("import")
	if !ok {
		t.Fatal("begin import")
	}
	ch := make(chan imp.Event, 2)
	ch <- imp.Event{Stage: imp.StageAnalyzing, Current: 13, Total: 24}
	ch <- imp.Event{Stage: imp.StageDone, Current: 24, Total: 24, Continued: true, Summary: &imp.Summary{SchemaVersion: 1, Chapters: 24, Characters: 14, Foreshadows: 9}}
	close(ch)
	s.pumpImport(id, ch)
	got, ok := s.jobs.latest("import")
	if !ok || !got.Done || got.Level != "success" || got.Import == nil {
		t.Fatalf("final import job=%+v", got)
	}
	if got.Import.SchemaVersion != 1 || len(got.Import.Steps) != 5 || !got.Import.Continued || got.Import.Summary.Characters != 14 || got.Import.Summary.Foreshadows != 9 {
		t.Fatalf("final import detail=%+v", got.Import)
	}
	for _, step := range got.Import.Steps {
		if step.Status != "done" {
			t.Fatalf("final step=%+v", step)
		}
	}
}

func TestMobileImportRegistryRetainsWaitingAndDone(t *testing.T) {
	tracker := newJobTracker()
	detail := &imp.Progress{SchemaVersion: 1, Preview: &imp.Preview{SchemaVersion: 1, Filename: "nguon.txt"}}
	tracker.record(jobEvent{Job: "import/1", Kind: "import", Awaiting: "confirm", Import: detail})
	waiting := tracker.entries["import/1"]
	if waiting.Status != JobWaiting || waiting.Result == nil || waiting.Result.Data != detail {
		t.Fatalf("waiting import lost durable preview detail: %+v", waiting.Result)
	}
	detail = &imp.Progress{SchemaVersion: 1, Summary: &imp.Summary{SchemaVersion: 1, Chapters: 24, Characters: 14, Foreshadows: 9}}
	tracker.record(jobEvent{Job: "import/1", Kind: "import", Done: true, Import: detail})
	done := tracker.entries["import/1"]
	if done.Status != JobDoneS || done.Result == nil || done.Result.Data != detail || done.Href != "work?dlg=import" {
		t.Fatalf("completed import lost final result: %+v", done)
	}
}
