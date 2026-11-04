package images

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

func TestUploadedReferencePlanningReleasesAfterCopy(t *testing.T) {
	s, _, manifest, _ := sceneFixture(t, "sc-0")
	item := manifest.Items[0]
	refs := []ImageRef{{ID: "upload:media-1", Role: RefLayout, Source: RefSource{Type: "upload", ImageID: "media-1"}}}
	item.Draft = &PromptDraft{Refs: &refs}
	active, acquired := 0, 0
	file := filepath.Join(t.TempDir(), "upload.png")
	h := handler(&recorderDrawer{}, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 }
		d.AcquireUpload = func(context.Context, string) (string, func(), error) {
			if err := os.WriteFile(file, []byte("exact uploaded bytes"), 0600); err != nil {
				return "", nil, err
			}
			active++
			acquired++
			return file, func() {
				active--
				if err := os.Remove(file); err != nil {
					t.Error(err)
				}
			}, nil
		}
	})
	for range 8 {
		plan, err := h.plan(context.Background(), s, &manifest, item)
		if err != nil {
			t.Fatal(err)
		}
		if active != 0 || len(plan.RefFiles) != 1 || len(plan.RefArtifacts) != 1 {
			t.Fatalf("active=%d plan=%+v", active, plan)
		}
		data, err := os.ReadFile(plan.RefFiles[0])
		if err != nil || string(data) != "exact uploaded bytes" {
			t.Fatalf("copy did not survive release: %q %v", data, err)
		}
		hash, err := steps.SHA256File(plan.RefFiles[0])
		if err != nil || plan.Refs[0].SHA256 != hash || !strings.HasPrefix(plan.Refs[0].Path, "images/references/"+hash) {
			t.Fatalf("reference not frozen: %+v %v", plan.Refs, err)
		}
	}
	if acquired != 8 {
		t.Fatalf("acquisitions=%d", acquired)
	}
}

func TestUploadedReferenceReleasesOnCopyFailure(t *testing.T) {
	s, _, manifest, _ := sceneFixture(t, "sc-0")
	released := false
	h := Handler{Deps: Deps{AcquireUpload: func(context.Context, string) (string, func(), error) {
		return t.TempDir(), func() { released = true }, nil
	}}}
	_, _, _, err := h.stageRefs(context.Background(), s, &manifest, manifest.Items[0], []ImageRef{{Source: RefSource{Type: "upload", ImageID: "media-1"}}}, 4)
	if err == nil || !released {
		t.Fatalf("copy error=%v released=%v", err, released)
	}
}

type referenceCheckingDrawer struct {
	check func(imagegen.Request)
	recorderDrawer
}

func (d *referenceCheckingDrawer) Draw(ctx context.Context, request imagegen.Request) (imagegen.Receipt, error) {
	d.check(request)
	return d.recorderDrawer.Draw(ctx, request)
}

func TestDrawPublishesExactUploadedAndExplicitFrameReferences(t *testing.T) {
	for _, source := range []string{"upload", "rival_frame"} {
		t.Run(source, func(t *testing.T) {
			s, workspace, manifest, _ := sceneFixture(t, "sc-0")
			at := 1.25
			refs := []ImageRef{{ID: source + ":one", Role: RefLayout, Source: RefSource{Type: source, ImageID: "media-1", VideoID: "source-video", AtSec: &at}}}
			manifest.Items[0].Draft = &PromptDraft{Refs: &refs}
			if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
				t.Fatal(err)
			}
			payload := []byte("exact " + source + " reference bytes")
			file := filepath.Join(t.TempDir(), "upload.png")
			active, resolutions := 0, 0
			workspace.Put("download/source.mp4", []byte("video"))
			s.ProcessRunner = func(_ context.Context, _ string, args []string, _ tools.Spec) (tools.Result, error) {
				resolutions++
				return tools.Result{}, os.WriteFile(args[len(args)-1], payload, 0600)
			}
			drawer := &referenceCheckingDrawer{check: func(request imagegen.Request) {
				if active != 0 || len(request.References) != 1 {
					t.Fatalf("active=%d references=%v", active, request.References)
				}
				version := loadManifest(t, s).Items[0].Versions[0]
				if len(version.Refs) != 1 || version.Refs[0].Path == "" || version.Refs[0].SHA256 == "" {
					t.Fatalf("drawing checkpoint has no durable reference: %+v", version.Refs)
				}
				for _, path := range []string{request.References[0], filepath.Join(workspace.Root, version.Refs[0].Path)} {
					data, err := os.ReadFile(path)
					if err != nil || string(data) != string(payload) {
						t.Fatalf("reference bytes=%q err=%v", data, err)
					}
				}
			}}
			h := handler(drawer, func(d *Deps) {
				d.Ffmpeg = "ffmpeg-boundary"
				d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 }
				d.AcquireUpload = func(context.Context, string) (string, func(), error) {
					if err := os.WriteFile(file, payload, 0600); err != nil {
						return "", nil, err
					}
					active++
					resolutions++
					return file, func() { active-- }, nil
				}
			})
			if _, err := h.Run(context.Background(), s); err != nil {
				t.Fatal(err)
			}
			if active != 0 || resolutions != 1 {
				t.Fatalf("active=%d resolutions=%d; actual draw must reuse frozen input", active, resolutions)
			}
			version := loadManifest(t, s).Items[0].Versions[0]
			if version.State() != VersionReady || version.Refs[0].Path == "" {
				t.Fatalf("version=%+v", version)
			}
		})
	}
}

func TestReferencePublicationFailureDoesNotStartDraw(t *testing.T) {
	s, workspace, manifest, _ := sceneFixture(t, "sc-0")
	refs := []ImageRef{{ID: "upload:one", Role: RefLayout, Source: RefSource{Type: "upload", ImageID: "one"}}}
	manifest.Items[0].Draft = &PromptDraft{Refs: &refs}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "upload.png")
	if err := os.WriteFile(file, []byte("reference"), 0600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("publication failed")
	workspace.FailPublish = want
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 }
		d.AcquireUpload = func(context.Context, string) (string, func(), error) { return file, func() {}, nil }
	})
	if _, err := h.Run(context.Background(), s); !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	if len(drawer.sent()) != 0 || loadManifest(t, s).Items[0].InFlight != nil {
		t.Fatal("publication failure exposed a flight or started paid work")
	}
}

func TestConcurrentScenesUsePrivateFrozenReferenceCopies(t *testing.T) {
	s, workspace, manifest, _ := sceneFixture(t, "sc-0", "sc-1")
	const path = "images/references/shared.png"
	workspace.Put(path, []byte("shared frozen reference"))
	ref := ImageRef{Path: path, Source: RefSource{Type: "upload", ImageID: "one"}}
	type result struct {
		path string
		err  error
	}
	results := make(chan result, 2)
	for _, item := range manifest.Items[:2] {
		go func() {
			path, err := (Handler{}).refFile(context.Background(), s, &manifest, item, ref)
			results <- result{path: path, err: err}
		}()
	}
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.path == second.path {
		t.Fatalf("concurrent copies share mutable staging: first=%+v second=%+v", first, second)
	}
	for _, copied := range []string{first.path, second.path} {
		data, err := os.ReadFile(copied)
		if err != nil || string(data) != "shared frozen reference" {
			t.Fatalf("reference copy=%q err=%v", data, err)
		}
	}
}
