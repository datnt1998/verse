package postgres

import (
	"strings"
	"testing"
)

// Two jobs building the same content-addressed media (identical audio bytes,
// so the same media-<sha> id) write metadata.json with a fresh createdAt, so
// the generations differ. Resolving the alias must still work: before this
// rule the lookup failed with "ambiguous current editor artifact" and every
// job after the first could not be scored or exported.
func TestPickArtifactPrefersNewestMediaGeneration(t *testing.T) {
	alias := "editor/media/media-5c11796ac20f96dfb8c1705db37e0bdd/metadata.json"
	artifacts := []legacyEditorArtifact{
		{Path: "clone/job-new/aaa/editor/media/media-5c11796ac20f96dfb8c1705db37e0bdd/metadata.json", Digest: "bbbb"},
		{Path: "clone/job-old/ccc/editor/media/media-5c11796ac20f96dfb8c1705db37e0bdd/metadata.json", Digest: "aaaa"},
	}
	got, err := pickArtifact(alias, artifacts)
	if err != nil {
		t.Fatalf("content-addressed media alias must resolve, got %v", err)
	}
	if got.Digest != "bbbb" {
		t.Fatalf("newest generation wins, got %s", got.Digest)
	}
}

func TestPickArtifactKeepsStrictConflictForProjectFiles(t *testing.T) {
	alias := "editor/projects/clone-job-g1/project.json"
	artifacts := []legacyEditorArtifact{
		{Path: "clone/job/aaa/editor/projects/clone-job-g1/project.json", Digest: "bbbb"},
		{Path: "clone/job/ccc/editor/projects/clone-job-g1/project.json", Digest: "aaaa"},
	}
	_, err := pickArtifact(alias, artifacts)
	if err == nil || !strings.Contains(err.Error(), "ambiguous current editor artifact") {
		t.Fatalf("project files must keep the conflict check, got %v", err)
	}
}
