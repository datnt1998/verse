package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
)

func TestHostedStorySummaryStorageState(t *testing.T) {
	f := provisionHostedMedia(t)
	// A gateway cwd can contain an unrelated file-backend engine. Its flock
	// must never mark a hosted story as locked or select its config chain.
	t.Chdir(t.TempDir())
	output := mkdirAll(t, "output", "novel")
	lock := flock.New(filepath.Join(output, ".ainovel.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		t.Fatalf("hold unrelated file lock: %v, %v", locked, err)
	}
	t.Cleanup(func() {
		if err := lock.Close(); err != nil {
			t.Errorf("close unrelated file lock: %v", err)
		}
	})
	tg := &testGateway{t: t, g: f.fx.g}
	owner := tg.login("hosted-owner", "mat-khau-12345")
	storyURL := "/api/projects/" + f.st.Project + "/stories/" + f.st.Story
	w := tg.do(http.MethodPost, storyURL+"/exports", owner, `{"format":"txt","volume":1}`)
	if w.Code != http.StatusOK {
		t.Fatalf("create hosted export: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/projects/" + f.st.Project, "/api/projects/" + f.st.Project + "/stories"} {
		w := tg.do(http.MethodGet, path, owner, "")
		if w.Code != http.StatusOK {
			t.Fatalf("hosted summary: %d %s", w.Code, w.Body.String())
		}
		var response struct {
			Stories []storySummary `json:"stories"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Stories) != 1 || response.Stories[0].ID != f.st.ID {
			t.Fatalf("unexpected hosted summaries: %s", w.Body.String())
		}
		if response.Stories[0].Locked {
			t.Errorf("hosted summary used gateway cwd file lock: %s", w.Body.String())
		}
		if response.Stories[0].ExportsCount != 1 {
			t.Errorf("hosted summary must count registry exports: %s", w.Body.String())
		}
	}
}
