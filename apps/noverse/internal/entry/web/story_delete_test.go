package web

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/gofrs/flock"
)

const deleteStoryURL = "/api/projects/proj/stories/01-story"
const deleteStoryBody = `{"confirm":"01-story"}`

func TestDeleteStoryPermissionMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, username, password string
		projects                 map[string]identity.ProjectRole
		status                   int
	}{
		{"admin", "admin", "admin-pass", nil, 200},
		{"owner", "writer", "writer-pass", map[string]identity.ProjectRole{"proj": identity.ProjectOwner}, 200},
		{"collab", "reader", "reader-pass", map[string]identity.ProjectRole{"proj": identity.ProjectCollab}, 403},
		{"reader", "reader", "reader-pass", nil, 403},
		{"unassigned member", "reader", "reader-pass", map[string]identity.ProjectRole{"other": identity.ProjectCollab}, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tg := newConfigTestGateway(t)
			if tc.projects != nil {
				if _, err := tg.g.accounts.UpdateUser(tc.username, identity.UserPatch{Projects: &tc.projects}); err != nil {
					t.Fatal(err)
				}
			}
			cookie := tg.login(tc.username, tc.password)
			w := tg.doWith(http.MethodDelete, deleteStoryURL, cookie, deleteStoryBody, func(r *http.Request) { r.Header.Set("Content-Type", "application/json") })
			if w.Code != tc.status {
				t.Fatalf("DELETE %s = %d, want %d: %s", tc.name, w.Code, tc.status, w.Body.String())
			}
			_, err := os.Stat(tg.story)
			if tc.status == 200 && !os.IsNotExist(err) {
				t.Fatalf("deleted story still exists: %v", err)
			}
			if tc.status != 200 && err != nil {
				t.Fatalf("denied delete changed story: %v", err)
			}
		})
	}
}

func TestDeleteStoryWrongConfirmation(t *testing.T) {
	tg := newConfigTestGateway(t)
	cookie := tg.login("admin", "admin-pass")
	w := tg.doWith(http.MethodDelete, deleteStoryURL, cookie, `{"confirm":"wrong"}`, func(r *http.Request) { r.Header.Set("Content-Type", "application/json") })
	if w.Code != 400 {
		t.Fatalf("wrong confirmation = %d, want400: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(tg.story); err != nil {
		t.Fatalf("wrong confirmation removed story: %v", err)
	}
}

func TestDeleteStoryRunningOrLeasedLeavesFilesUntouched(t *testing.T) {
	for _, mode := range []string{"worker", "lease"} {
		t.Run(mode, func(t *testing.T) {
			tg := newConfigTestGateway(t)
			cookie := tg.login("admin", "admin-pass")
			output := mkdirAll(t, tg.story, "output", "novel")
			chapter := filepath.Join(mkdirAll(t, output, "chapters"), "01.md")
			if err := os.WriteFile(chapter, []byte("Không được xoá"), 0644); err != nil {
				t.Fatal(err)
			}
			var workerDone chan struct{}
			if mode == "lease" {
				lease := flock.New(filepath.Join(output, ".ainovel.lock"))
				locked, err := lease.TryLock()
				if err != nil || !locked {
					t.Fatalf("lease setup: %v", err)
				}
				defer lease.Close()
			} else {
				workerDone = make(chan struct{})
				var once sync.Once
				slot := tg.g.slot("proj/01-story")
				slot.worker = fakeWorker(workerDone, &once)
				tg.g.setTransport(transportFunc(func(r *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"isRunning":true}`))}, nil
				}))
				defer tg.g.closeBook("proj/01-story")
			}
			w := tg.doWith(http.MethodDelete, deleteStoryURL, cookie, deleteStoryBody, func(r *http.Request) { r.Header.Set("Content-Type", "application/json") })
			if w.Code != 409 {
				t.Fatalf("%s delete = %d, want409: %s", mode, w.Code, w.Body.String())
			}
			if got := decodeResp(t, w)["error"]; got != "Truyện đang chạy — dừng trước khi xoá" {
				t.Fatalf("error: %v", got)
			}
			if data, err := os.ReadFile(chapter); err != nil || string(data) != "Không được xoá" {
				t.Fatalf("busy deletion changed chapter: %q %v", data, err)
			}
			if workerDone != nil {
				select {
				case <-workerDone:
					t.Fatal("busy worker was stopped")
				default:
				}
			}
		})
	}
}

func TestDeleteStoryClosesIdleWorkerAndCleansState(t *testing.T) {
	tg := newConfigTestGateway(t)
	cookie := tg.login("admin", "admin-pass")
	for _, rel := range []string{"audio/ban-doc/01.json", "output/novel/meta/comments/01.json", ".ainovel/cocreate/draft.json"} {
		path := filepath.Join(tg.story, rel)
		mkdirAll(t, filepath.Dir(path))
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	projects := map[string]identity.ProjectRole{"proj": identity.ProjectOwner}
	if _, err := tg.g.accounts.UpdateUser("writer", identity.UserPatch{Projects: &projects}); err != nil {
		t.Fatal(err)
	}
	admin := identity.User{Username: "admin", Role: identity.RoleAdmin}
	sub, _, _ := tg.g.events().subscribe(admin, 0, []string{"proj/01-story", "proj"})
	defer tg.g.events().unsubscribe(sub)
	hub := tg.g.jobsHub()
	hub.apply(JobChange{Job: Job{ID: "proj/01-story/export/1", Project: "proj", Story: "01-story", Kind: "export", Status: JobDoneS}})
	hub.apply(JobChange{Job: Job{ID: "proj/other/export/1", Project: "proj", Story: "other", Kind: "export", Status: JobDoneS}})
	done := make(chan struct{})
	var once sync.Once
	slot := tg.g.slot("proj/01-story")
	slot.worker = fakeWorker(done, &once)
	tg.g.setTransport(transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"isRunning":false}`))}, nil
	}))
	w := tg.doWith(http.MethodDelete, deleteStoryURL, cookie, deleteStoryBody, func(r *http.Request) { r.Header.Set("Content-Type", "application/json") })
	if w.Code != 200 {
		t.Fatalf("idle delete = %d: %s", w.Code, w.Body.String())
	}
	select {
	case <-done:
	default:
		t.Fatal("idle worker was not closed")
	}
	if _, err := os.Stat(tg.story); !os.IsNotExist(err) {
		t.Fatalf("story audio/comments/drafts retained: %v", err)
	}
	if watched := tg.g.events().Watched(); watched["proj/01-story"] || !watched["proj"] {
		t.Fatalf("watch cleanup: %v", watched)
	}
	jobs := hub.list(admin)
	if len(jobs) != 1 || jobs[0].Story != "other" {
		t.Fatalf("job cleanup: %v", jobs)
	}
	hub.apply(JobChange{Job: Job{ID: "proj/01-story/export/late", Project: "proj", Story: "01-story", Kind: "export", Status: JobDoneS}})
	if got := hub.list(admin); len(got) != 1 {
		t.Fatalf("deleted story job returned after late update: %v", got)
	}
	accounts, err := tg.g.accounts.Users()
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range accounts {
		if account.Username == "writer" {
			if account.Projects["proj"] != identity.ProjectOwner {
				t.Fatal("deleting a story must preserve project access")
			}
		}
	}
	libraryChanged, projectChanged := false, false
	for {
		select {
		case ev := <-sub.C:
			libraryChanged = libraryChanged || ev.Type == "library.changed"
			projectChanged = projectChanged || ev.Type == "project.changed"
		default:
			if !libraryChanged || !projectChanged {
				t.Fatalf("missing deletion invalidation: library=%t project=%t", libraryChanged, projectChanged)
			}
			return
		}
	}
}
