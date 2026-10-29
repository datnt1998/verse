package web

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/assets"
	"github.com/datnt1998/noverse/internal/bootstrap"
	"github.com/datnt1998/noverse/internal/host"
	"github.com/datnt1998/noverse/internal/host/exp"
	"github.com/datnt1998/noverse/internal/webauth"
)

func TestExportConflictIsPreflightWithoutJobAndPathsAreRelative(t *testing.T) {
	st, s := storyFixture(t, []int{1})
	reader := host.NewStoryReader(st.Dir)
	exports := filepath.Join(s.Dir(), "exports")
	path := filepath.Join(exports, "book.txt")
	if _, err := reader.Export(context.Background(), exp.Options{OutPath: path, OnExists: "replace"}); err != nil {
		t.Fatal(err)
	}
	g := &Gateway{Root: st.Root}
	hub := g.jobsHub()
	defer hub.close()
	subscriber, _, _ := g.events().subscribe(webauth.User{Role: webauth.RoleAdmin}, 0, nil)
	defer g.events().unsubscribe(subscriber)
	_, id, err := g.runGatewayExport(context.Background(), st, reader, exp.Options{OutPath: path, OnExists: "fail"}, exports)
	var conflict *exportConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected structured conflict: %v", err)
	}
	if id != "" || len(hub.list(webauth.User{Role: webauth.RoleAdmin})) != 0 {
		t.Fatalf("preflight conflict registered a failed job: id=%q jobs=%+v", id, hub.list(webauth.User{Role: webauth.RoleAdmin}))
	}
	select {
	case event := <-subscriber.ch:
		t.Fatalf("preflight conflict published job event %s", event.Type)
	default:
	}
	payload, _ := json.Marshal(conflict.payload)
	if strings.Contains(string(payload), st.Root) {
		t.Fatalf("conflict exposed server root: %s", payload)
	}
	expected := "output/novel/exports/book.txt"
	if conflict.payload["existing"].(map[string]any)["path"] != expected || conflict.payload["incoming"].(map[string]any)["path"] != expected {
		t.Fatalf("conflict paths not story-relative: %s", payload)
	}
}

func TestExportFailureDoesNotExposeServerPathsInJobsOrMessages(t *testing.T) {
	st, s := storyFixture(t, []int{1})
	blocker := filepath.Join(s.Dir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "book.txt")
	reader := host.NewStoryReader(st.Dir)
	g := &Gateway{Root: st.Root}
	defer g.jobsHub().close()
	_, _, err := g.runGatewayExport(context.Background(), st, reader, exp.Options{OutPath: path, OnExists: "replace"}, filepath.Dir(path))
	if err == nil || strings.Contains(err.Error(), st.Root) {
		t.Fatalf("unsafe export error: %v", err)
	}
	for _, job := range g.jobsHub().list(webauth.User{Role: webauth.RoleAdmin}) {
		encoded, _ := json.Marshal(job)
		if strings.Contains(string(encoded), st.Root) {
			t.Fatalf("job leaked server path: %s", encoded)
		}
	}
	h, err := host.New(bootstrap.Config{OutputDir: s.Dir()}, assets.Bundle{}, host.WithUnconfiguredReads())
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	worker := newWorkerServer(h, "secret", nil, newBroadcaster())
	worker.storyDir = st.Dir
	if _, _, err := worker.launchExport(exp.Options{OutPath: path, OnExists: "replace"}); err != nil {
		t.Fatal(err)
	}
	event := waitJobDone(t, worker, "export")
	if event.Error == "" {
		t.Fatal("expected real write failure")
	}
	encoded, _ := json.Marshal(event)
	if strings.Contains(string(encoded), st.Root) {
		t.Fatalf("worker result leaked server path: %s", encoded)
	}
	for _, job := range worker.jobs.listJobs() {
		encoded, _ := json.Marshal(job)
		if strings.Contains(string(encoded), st.Root) {
			t.Fatalf("worker job.error/result.sub leaked server path: %s", encoded)
		}
	}
}
