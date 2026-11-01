package editor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/blobstore"
)

type commandTestRepository struct {
	mu     sync.Mutex
	values map[string]Resource
	media  *captionTestMedia
	fail   string
}

func (r *commandTestRepository) Get(_ context.Context, key string) (Resource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.values[key]
	if !ok {
		return Resource{}, blobstore.ErrNotFound
	}
	return v, nil
}
func (r *commandTestRepository) List(_ context.Context, prefix string) ([]Resource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Resource{}
	for key, v := range r.values {
		if strings.HasPrefix(key, prefix) {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *commandTestRepository) CAS(_ context.Context, key, expected string, next Resource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if key == r.fail {
		return fmt.Errorf("Disk write refused")
	}
	current, ok := r.values[key]
	if (!ok && expected != "") || (ok && current.Revision != expected) {
		return ErrConflict
	}
	next.Version = current.Version + 1
	r.values[key] = next
	return nil
}
func (r *commandTestRepository) Delete(_ context.Context, key, expected string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := r.values[key]; ok && v.Revision != expected {
		return ErrConflict
	}
	delete(r.values, key)
	return nil
}
func (r *commandTestRepository) Paths(_ context.Context, prefix string) ([]string, error) {
	r.media.mu.Lock()
	defer r.media.mu.Unlock()
	paths := []string{}
	for path := range r.media.paths {
		if strings.HasPrefix(path, prefix) {
			paths = append(paths, path)
		}
	}
	return paths, nil
}
func seedCommandMedia(t *testing.T, s Service, id, kind string) {
	t.Helper()
	probe := Object{"kind": kind, "mime": "image/png", "width": 128, "height": 128}
	mime := "image/png"
	if kind == "audio" {
		mime = "audio/wav"
		probe = Object{"kind": "audio", "mime": mime, "durationSec": 1, "audioCodec": "pcm_s16le"}
	}
	_, err := s.writeJSON(context.Background(), "media/"+id, "editor/media/"+id+"/metadata.json", Object{"id": id, "fileName": id, "mime": mime, "size": 128, "contentHash": "sha256:" + strings.Repeat("a", 64), "createdAt": 1, "tags": []string{}, "probe": probe}, "")
	if err != nil {
		t.Fatal(err)
	}
}
func commandMediaFixture(t *testing.T) (Service, string) {
	t.Helper()
	s, _, revision := captionTestService(t)
	seedCommandMedia(t, s, "one", "image")
	seedCommandMedia(t, s, "two", "audio")
	p, _, err := s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	p["mediaItems"] = []Object{{"id": "one"}, {"id": "two"}}
	revision, err = s.Store.Write(context.Background(), "project", p, revision)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ReconcileProjectMediaLinks(context.Background(), "project", []string{"one", "two"}); err != nil {
		t.Fatal(err)
	}
	s.Store.Now = func() time.Time { return time.Unix(200, 0) }
	return s, revision
}
func TestRunCommandsCLIMetadataDryRunAddAndReconcile(t *testing.T) {
	s, rev := commandMediaFixture(t)
	seedCommandMedia(t, s, "unlinked", "image")
	unlinked, err := s.GetMediaMetadata(context.Background(), "unlinked")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := commandMediaMetadata(unlinked)
	if err != nil {
		t.Fatal(err)
	}
	commands := []Command{{Type: "media.add", Params: Object{"media": metadata}}, {Type: "media.update", Params: Object{"mediaId": "one", "updates": Object{"tags": []string{"ảnh nền"}, "attribution": Object{"provider": "Fixtures"}}}}}
	if _, err = s.RunCommands(context.Background(), "project", commands, ApplyCommandsOptions{DryRun: true, BaseRevision: rev}); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetMediaMetadata(context.Background(), "one")
	if err != nil || len(before.Tags) != 0 {
		t.Fatal("dry-run mutated metadata", err)
	}
	linked, err := s.ListMedia(context.Background(), "project")
	if err != nil || len(linked) != 2 {
		t.Fatal("dry-run linked media", err)
	}
	result, err := s.RunCommands(context.Background(), "project", commands, ApplyCommandsOptions{BaseRevision: rev})
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	refs := fillObjects(p["mediaItems"])
	if len(refs) != 3 || refs[0]["id"] != "unlinked" {
		t.Fatal(refs)
	}
	updated, err := s.GetMediaMetadata(context.Background(), "one")
	if err != nil || len(updated.Tags) != 1 || updated.Tags[0] != "ảnh nền" || updated.Attribution == nil {
		t.Fatal("metadata not applied", err)
	}
	if err = s.ReconcileProjectMediaLinks(context.Background(), "project", []string{"one", "two"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RunCommands(context.Background(), "project", []Command{{Type: "project.rename", Params: Object{"name": "Reconciled"}}}, ApplyCommandsOptions{BaseRevision: result["revision"].(string)}); err != nil {
		t.Fatal(err)
	}
	linked, err = s.ListMedia(context.Background(), "project")
	if err != nil || len(linked) != 3 {
		t.Fatal("authoritative project refs not reconciled", err)
	}
}
func TestRunCommandsCLIMetadataValidationAndPartialCommit(t *testing.T) {
	s, rev := commandMediaFixture(t)
	for _, updates := range []Object{{"tags": []string{""}}, {"fileName": "forged"}, {"contentHash": "sha256:" + strings.Repeat("b", 64)}} {
		if _, err := s.RunCommands(context.Background(), "project", []Command{{Type: "media.update", Params: Object{"mediaId": "one", "updates": updates}}}, ApplyCommandsOptions{}); err == nil {
			t.Fatal("invalid metadata accepted", updates)
		}
		_, current, err := s.Store.Read(context.Background(), "project")
		if err != nil || current != rev {
			t.Fatal("invalid media committed", err)
		}
	}
	repo := s.Repository.(*commandTestRepository)
	repo.fail = "media/two"
	_, err := s.RunCommands(context.Background(), "project", []Command{{Type: "media.update", Params: Object{"mediaId": "one", "updates": Object{"tags": []string{"updated"}}}}, {Type: "media.update", Params: Object{"mediaId": "two", "updates": Object{"tags": []string{"updated"}}}}}, ApplyCommandsOptions{BaseRevision: rev})
	e, ok := err.(*Error)
	if !ok || e.Code != "metadata_partial" {
		t.Fatalf("wrong partial error: %#v", err)
	}
	_, current, readErr := s.Store.Read(context.Background(), "project")
	if readErr != nil || e.Extra["revision"] != current || current == rev {
		t.Fatal("partial error lost committed revision", readErr)
	}
	applied := e.Extra["appliedMediaIds"].([]string)
	failed := e.Extra["failedMediaIds"].([]string)
	if len(applied) != 1 || applied[0] != "one" || len(failed) != 1 || failed[0] != "two" {
		t.Fatal(e.Extra)
	}
	one, err := s.GetMediaMetadata(context.Background(), "one")
	if err != nil || len(one.Tags) != 1 {
		t.Fatal(one, err)
	}
	two, err := s.GetMediaMetadata(context.Background(), "two")
	if err != nil || len(two.Tags) != 0 {
		t.Fatal(two, err)
	}
}
