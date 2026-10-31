package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
)

func TestHostedMetadataRepositoryCAS(t *testing.T) {
	f := provisionHostedGateway(t, "metadata-cas", "story")
	ctx := t.Context()
	owner := actorOf(f.owner)
	st := library.Story{Project: f.project, Story: f.story, ID: f.project + "/" + f.story}
	for _, target := range []struct {
		name, story string
		write       func(string, string) error
		read        func() (string, error)
	}{
		{"project", "", func(name, token string) error {
			return f.repo.UpdateProjectCAS(ctx, owner, f.project, library.ProjectFile{Name: name}, token)
		}, func() (string, error) {
			file, err := f.repo.ProjectFile(ctx, owner, f.project)
			if err != nil {
				return "", err
			}
			return file.Name, nil
		}},
		{"story", f.story, func(name, token string) error {
			return f.repo.WriteStoryFileCAS(ctx, owner, st, library.StoryFile{Title: name, Status: "idea", TitleRenamed: true}, token)
		}, func() (string, error) {
			file, err := f.repo.LoadStoryFile(ctx, owner, st)
			if err != nil {
				return "", err
			}
			return file.Title, nil
		}},
	} {
		t.Run(target.name, func(t *testing.T) {
			token, err := f.repo.MetadataVersion(ctx, owner, f.project, target.story)
			if err != nil || token == "" {
				t.Fatalf("initial metadata version: %q %v", token, err)
			}
			start := make(chan struct{})
			results := make(chan error, 2)
			for _, name := range []string{"Bản thứ nhất", "Bản thứ hai"} {
				go func(name string) {
					<-start
					results <- target.write(name, token)
				}(name)
			}
			close(start)
			wins, conflicts := 0, 0
			for range 2 {
				if err := <-results; err == nil {
					wins++
				} else if errors.Is(err, errs.ErrStorageConflict) {
					conflicts++
				} else {
					t.Fatalf("concurrent metadata save: %v", err)
				}
			}
			if wins != 1 || conflicts != 1 {
				t.Fatalf("metadata CAS winners=%d conflicts=%d", wins, conflicts)
			}
			next, err := f.repo.MetadataVersion(ctx, owner, f.project, target.story)
			if err != nil || next == token {
				t.Fatalf("row version did not advance: %q -> %q (%v)", token, next, err)
			}
			name, err := target.read()
			if err != nil || name != "Bản thứ nhất" && name != "Bản thứ hai" {
				t.Fatalf("saved metadata: %q %v", name, err)
			}
			// Identical saves still consume a version; this must be a row
			// version, not merely a hash of the visible metadata fields.
			if err := target.write(name, next); err != nil {
				t.Fatal(err)
			}
			if err := target.write("Không được ghi", next); !errors.Is(err, errs.ErrStorageConflict) {
				t.Fatalf("reused version after identical save: %v", err)
			}
			if _, err := f.repo.MetadataVersion(ctx, actorOf(f.outsider), f.project, target.story); !errors.Is(err, errs.ErrStorageNotFound) {
				t.Fatalf("outsider metadata version: %v", err)
			}
		})
	}
	if err := f.repo.UpdateProjectCAS(ctx, actorOf(f.viewer), f.project, library.ProjectFile{Name: "Không được ghi"}, "*"); !errors.Is(err, errs.ErrStorageForbidden) {
		t.Fatalf("viewer project write: %v", err)
	}
	if err := f.repo.WriteStoryFileCAS(ctx, actorOf(f.viewer), st, library.StoryFile{Title: "Không được ghi"}, "*"); !errors.Is(err, errs.ErrStorageForbidden) {
		t.Fatalf("viewer story write: %v", err)
	}
}

type metadataRaceRepository struct {
	*postgres.LibraryRepository
	beforeWrite func()
}

func (r *metadataRaceRepository) UpdateProjectCAS(ctx context.Context, actor *library.Actor, project string, file library.ProjectFile, expected string) error {
	r.beforeWrite()
	return r.LibraryRepository.UpdateProjectCAS(ctx, actor, project, file, expected)
}

func TestHostedMetadataCrossGatewayConflict(t *testing.T) {
	f := provisionHostedGateway(t, "metadata-gateways", "story")
	second := NewGateway("", f.accounts, version.Info{})
	t.Cleanup(second.Close)
	second.SetFactStore(f.g.factStore)
	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	repo := &metadataRaceRepository{LibraryRepository: f.repo, beforeWrite: func() {
		arrived <- struct{}{}
		<-release
	}}
	f.g.SetLibraryRepository(repo)
	second.SetLibraryRepository(repo)
	owner := (&testGateway{t: t, g: f.g}).login("hosted-owner", "mat-khau-12345")
	path := "/api/projects/" + f.project
	initial := doFrom(t, f.g, http.MethodGet, path, owner, "", "127.0.0.1:1234", nil)
	if initial.Code != http.StatusOK || initial.Header().Get("ETag") == "" {
		t.Fatalf("initial metadata: %d %s", initial.Code, initial.Body.String())
	}
	results := make(chan *httptest.ResponseRecorder, 2)
	for _, g := range []*Gateway{f.g, second} {
		go func(g *Gateway) {
			results <- doFrom(t, g, http.MethodPut, path, owner, `{"name":"Tên đã lưu"}`, "127.0.0.1:1234", map[string]string{"If-Match": initial.Header().Get("ETag")})
		}(g)
	}
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for range 2 {
		select {
		case <-arrived:
		case <-timer.C:
			t.Fatal("both gateways must pass their independent feedback checks before writing")
		}
	}
	close(release)
	released = true
	wins, conflicts := 0, 0
	for range 2 {
		w := <-results
		switch w.Code {
		case http.StatusOK:
			wins++
		case http.StatusConflict:
			conflicts++
			var body struct {
				Conflict bool            `json:"conflict"`
				Version  string          `json:"version"`
				Current  json.RawMessage `json:"current"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if !body.Conflict || body.Version == "" || !strings.Contains(string(body.Current), "Tên đã lưu") {
				t.Fatalf("database race must return file-shaped conflict feedback: %s", w.Body.String())
			}
		default:
			t.Fatalf("concurrent metadata HTTP save: %d %s", w.Code, w.Body.String())
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("cross-gateway winners=%d conflicts=%d", wins, conflicts)
	}
}
