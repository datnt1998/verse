package render

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

func TestRenderInputsScoresApprovedImagesFromArtifactCatalog(t *testing.T) {
	ctx := context.Background()
	f := newScorerFixture(t)
	files := &stepstest.FakeMedia{Root: f.root}
	jobID := f.input.Job.ID
	catalog := []domain.CloneArtifact{}
	for i := range f.images.Items {
		item := &f.images.Items[i]
		version := &item.Versions[0]
		version.Path = "images/versions/" + item.ID + "/1.webp"
		path := "clone/" + jobID + "/" + version.SHA256 + "/" + version.Path
		files.Put(path, []byte("approved-image-"+item.ID))
		catalog = append(catalog, domain.CloneArtifact{Path: path, SHA256: version.SHA256, Step: domain.CloneImages})
	}
	f.sync()
	manifest, err := os.ReadFile(f.media.files["images/manifest.json"])
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := "clone/" + jobID + "/" + scorerTestHash(manifest) + "/images/manifest.json"
	files.Put(manifestPath, manifest)
	manifestArtifact := domain.CloneArtifact{Path: manifestPath, SHA256: scorerTestHash(manifest), Step: domain.CloneImages}
	catalog = append(catalog, manifestArtifact)
	// The approved versions live only in the durable catalog. The images step
	// output publishes its manifest, not the image artifacts it references.
	s := &Service{Media: files, Jobs: renderJobRepo{
		job:       f.input.Job,
		artifacts: catalog,
		records: []domain.CloneStepRecord{
			{Step: domain.CloneBuild, Output: &domain.CloneStepOutput{Data: map[string]any{"projectId": "project"}}},
			{Step: domain.CloneImages, Output: &domain.CloneStepOutput{Artifacts: []domain.CloneArtifact{manifestArtifact}}},
		},
	}}
	job, upstream, projectID, resolved, err := s.Inputs(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if projectID != "project" {
		t.Fatalf("wrong project: %q", projectID)
	}
	for i, item := range f.images.Items {
		version := item.Versions[0]
		if handle, err := files.Acquire(ctx, version.Path); err == nil {
			handle.Release()
			t.Fatalf("fixture unexpectedly exposes relative image %s", version.Path)
		}
		handle, err := resolved.Acquire(ctx, version.Path)
		if err != nil {
			t.Fatalf("resolve catalog image %s: %v", version.Path, err)
		}
		hash, hashErr := steps.SHA256File(handle.Path)
		wantPath := filepath.Join(files.Root, filepath.FromSlash(catalog[i].Path))
		gotPath := handle.Path
		handle.Release()
		if hashErr != nil || hash != version.SHA256 || gotPath != wantPath {
			t.Fatalf("wrong catalog image %s: path=%q hash=%q err=%v", version.Path, gotPath, hash, hashErr)
		}
	}
	f.input.Job, f.input.Upstream, f.input.Media = job, upstream, resolved
	score, err := ComputeScore(ctx, f.input)
	if err != nil {
		t.Fatal(err)
	}
	if !score.Passed || score.Total != 100 || len(score.Blockers) != 0 {
		t.Fatalf("catalog-backed approved images blocked scoring: %+v", score)
	}
	for _, id := range []string{"images", "thumbnail"} {
		if item := scorerItem(t, score, id); item.Status != "pass" {
			t.Fatalf("catalog-backed %s did not pass: %+v", id, item)
		}
	}

	// Catalog hashes are not evidence by themselves: the scorer must still
	// hash the resolved bytes for both scene images and the thumbnail.
	for _, artifact := range catalog[:len(f.images.Items)] {
		files.Put(artifact.Path, []byte("tampered image"))
	}
	score, err = ComputeScore(ctx, f.input)
	if err != nil {
		t.Fatal(err)
	}
	if !scorerBlocked(score, "images") || !scorerBlocked(score, "thumbnail") {
		t.Fatalf("tampered catalog bytes were accepted: %+v", score)
	}
}

func TestRenderInputsArtifactCatalogIsolationAndPrecedence(t *testing.T) {
	const logical = "images/versions/a/1.webp"
	const current = "clone/job/current/" + logical
	const stale = "clone/job/stale/" + logical
	const foreign = "clone/other/current/" + logical
	for _, test := range []struct {
		name      string
		catalog   []string
		upstream  string
		requests  []string
		want      string
		ambiguous bool
	}{
		{name: "cross-job relative path", catalog: []string{foreign}, requests: []string{logical}},
		{name: "cross-job full path", catalog: []string{foreign}, requests: []string{foreign}},
		{name: "job prefix is not ownership", catalog: []string{"clone/job-other/current/" + logical}, requests: []string{logical}},
		{name: "relative path must match exactly", catalog: []string{"clone/job/current/unrelated/" + logical}, requests: []string{logical}},
		{name: "ambiguous own-job versions", catalog: []string{current, stale}, requests: []string{logical}, ambiguous: true},
		{name: "upstream wins over stale catalog", catalog: []string{stale, "clone/job/older/" + logical, foreign}, upstream: current, requests: []string{logical, current}, want: current},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			files := &stepstest.FakeMedia{Root: t.TempDir()}
			repo := renderJobRepo{
				job: stepstest.Job("job"),
				records: []domain.CloneStepRecord{{Step: domain.CloneBuild, Output: &domain.CloneStepOutput{
					Data: map[string]any{"projectId": "project"},
				}}},
			}
			for _, path := range test.catalog {
				files.Put(path, []byte(path))
				repo.artifacts = append(repo.artifacts, domain.CloneArtifact{Path: path, Step: domain.CloneImages})
			}
			if test.upstream != "" {
				files.Put(test.upstream, []byte(test.upstream))
				repo.records = append(repo.records, domain.CloneStepRecord{Step: domain.CloneImages, Output: &domain.CloneStepOutput{
					Artifacts: []domain.CloneArtifact{{Path: test.upstream}},
				}})
			}
			if test.ambiguous {
				// Ambiguity must fail closed rather than falling through to a
				// stale unscoped alias in the underlying media store.
				files.Put(logical, []byte("unscoped stale image"))
			}
			s := &Service{Jobs: repo, Media: files}
			_, _, _, resolved, err := s.Inputs(ctx, "job")
			if err != nil {
				t.Fatal(err)
			}
			for _, request := range test.requests {
				handle, err := resolved.Acquire(ctx, request)
				if test.want == "" {
					if err == nil {
						handle.Release()
						t.Fatalf("accepted rejected catalog path %q", request)
					}
					if test.ambiguous {
						if !strings.Contains(err.Error(), "ambiguous") {
							t.Fatalf("ambiguity was not reported: %v", err)
						}
					} else if !scoreMissing(err) {
						t.Fatalf("expected missing/foreign artifact, got %v", err)
					}
					continue
				}
				if err != nil {
					t.Fatalf("resolve upstream %s: %v", request, err)
				}
				hash, hashErr := steps.SHA256File(handle.Path)
				wantPath := filepath.Join(files.Root, filepath.FromSlash(test.want))
				gotPath := handle.Path
				handle.Release()
				if hashErr != nil || hash != scorerTestHash([]byte(test.want)) || gotPath != wantPath {
					t.Fatalf("catalog redirected upstream %s: path=%q hash=%q err=%v", request, gotPath, hash, hashErr)
				}
			}
		})
	}
}
