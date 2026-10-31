package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

func mediaTestProject(t *testing.T, s *Service, id string) {
	t.Helper()
	p, e := New(id, "Project", 1920, 1080, 30, s.now())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Store.Create(context.Background(), p); e != nil {
		t.Fatal(e)
	}
}
func mediaFileOptions(t *testing.T, file string) ImportFileOnceOptions {
	t.Helper()
	f := filepath.Join("testdata", "__fixtures__", file)
	b, e := os.ReadFile(f)
	if e != nil {
		t.Fatal(e)
	}
	hash, e := mediaFileHash(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	return ImportFileOnceOptions{File: f, FileName: file, DeclaredMIME: "image/png", ContentHash: hash, MIMEPrefix: "image/"}
}

// Ports every media-import.test.ts case plus media portions of projects.test.ts.
func TestMediaImportDedupPrefixLostIndexAndConcurrent(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	opts := mediaFileOptions(t, "image.png")
	first, reused, e := s.ImportFileOnce(ctx, opts)
	if e != nil || reused {
		t.Fatalf("first %v %v", reused, e)
	}
	again, reused, e := s.ImportFileOnce(ctx, opts)
	if e != nil || !reused || again.ID != first.ID {
		t.Fatalf("reuse %+v %v %v", again, reused, e)
	}
	if e = s.removeResource(ctx, "workspace/media-index", "editor/media-index.json"); e != nil {
		t.Fatal(e)
	}
	found, e := s.FindMediaByContentHash(ctx, first.ContentHash, "image/")
	if e != nil || found == nil || found.ID != first.ID {
		t.Fatalf("lost index %+v %v", found, e)
	}
	found, e = s.FindMediaByContentHash(ctx, first.ContentHash, "video/")
	if e != nil || found != nil {
		t.Fatalf("prefix %+v %v", found, e)
	}
	opts.MIMEPrefix = "video/"
	second, reused, e := s.ImportFileOnce(ctx, opts)
	if e != nil || reused || second.ID == first.ID {
		t.Fatalf("wrong prefix reused %v %v", reused, e)
	}
	s = newMediaTestService(t)
	opts = mediaFileOptions(t, "image.png")
	ids := make(chan string, 8)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); m, _, e := s.ImportFileOnce(ctx, opts); ids <- m.ID; errs <- e }()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	unique := map[string]bool{}
	for id := range ids {
		unique[id] = true
	}
	if len(unique) != 1 {
		t.Fatalf("concurrent ids: %v", unique)
	}
}
func TestMediaImportFileOwnsIndependentBytes(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	opts := mediaFileOptions(t, "image.png")
	b, e := os.ReadFile(opts.File)
	if e != nil {
		t.Fatal(e)
	}
	opts.File = filepath.Join(t.TempDir(), "source.png")
	if e = os.WriteFile(opts.File, b, 0600); e != nil {
		t.Fatal(e)
	}
	m, _, e := s.ImportFileOnce(ctx, opts)
	if e != nil {
		t.Fatal(e)
	}
	h, e := s.AcquireMedia(ctx, m.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer h.Release()
	sourceStat, e := os.Stat(opts.File)
	if e != nil {
		t.Fatal(e)
	}
	mediaStat, e := os.Stat(h.Path)
	if e != nil {
		t.Fatal(e)
	}
	if os.SameFile(sourceStat, mediaStat) {
		t.Fatal("source shares inode")
	}
	if e = os.WriteFile(opts.File, []byte("rewritten"), 0600); e != nil {
		t.Fatal(e)
	}
	actual, e := os.ReadFile(h.Path)
	if e != nil || !bytes.Equal(actual, b) {
		t.Fatalf("source rewrite mutated import: %v", e)
	}
}
func TestMediaImportWorkspaceProjectLinksAndReconcile(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	mediaTestProject(t, s, "one")
	mediaTestProject(t, s, "two")
	opts := mediaFileOptions(t, "image.png")
	opts.ProjectID = "one"
	m, _, e := s.ImportFileOnce(ctx, opts)
	if e != nil {
		t.Fatal(e)
	}
	opts.ProjectID = "two"
	again, reused, e := s.ImportFileOnce(ctx, opts)
	if e != nil || !reused || again.ID != m.ID {
		t.Fatalf("linked reuse %v %v", reused, e)
	}
	for _, pid := range []string{"", "one", "two"} {
		records, e := s.ListMedia(ctx, pid)
		if e != nil || len(records) != 1 || records[0].ID != m.ID {
			t.Fatalf("%s: %+v %v", pid, records, e)
		}
	}
	links, rev, e := s.readMediaLinks(ctx, "one", true)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.ReconcileProjectMediaLinks(ctx, "one", []string{m.ID, m.ID}); e != nil {
		t.Fatal(e)
	}
	unchanged, newRev, e := s.readMediaLinks(ctx, "one", true)
	if e != nil || rev != newRev || !reflect.DeepEqual(links, unchanged) {
		t.Fatal("unchanged links rewritten")
	}
	if e = s.ReconcileProjectMediaLinks(ctx, "one", []string{m.ID, "missing"}); e == nil {
		t.Fatal("missing record accepted")
	}
	if _, e = s.UnlinkMedia(ctx, "one", m.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.GetMediaMetadata(ctx, m.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.LinkMedia(ctx, "two", m.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.DeleteMedia(ctx, m.ID); e != nil {
		t.Fatal(e)
	}
	records, e := s.ListMedia(ctx, "two")
	if e != nil || len(records) != 0 {
		t.Fatalf("delete retained links: %+v %v", records, e)
	}
}
func TestMediaLinksAreConcurrentAndPreservePhase05Canonical(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	mediaTestProject(t, s, "one")
	a := mediaTestImport(t, s, "image.png")
	b := mediaTestImport(t, s, "image.jpg")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{a.ID, b.ID} {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.LinkMedia(ctx, "one", id); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	records, e := s.ListMedia(ctx, "one")
	if e != nil || len(records) != 2 {
		t.Fatalf("concurrent links %v %v", records, e)
	}
	links, _, e := s.readMediaLinks(ctx, "one", true)
	if e != nil || len(links.MediaIDs) != 2 {
		t.Fatal(e)
	}
	if mediaLinksPath("one") != "editor/projects/one/links.json" {
		t.Fatal("phase05 path changed")
	}
}
func TestMediaDeleteUnreadableLinksKeepsSourceAndUnlinksOtherProjects(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	for _, id := range []string{"good", "bad"} {
		mediaTestProject(t, s, id)
	}
	m := mediaTestImport(t, s, "image.png")
	for _, id := range []string{"good", "bad"} {
		if _, e := s.LinkMedia(ctx, id, m.ID); e != nil {
			t.Fatal(e)
		}
	}
	r := s.Repository.(*mediaTestRepository)
	resource, e := r.Get(ctx, "projects/bad/links")
	if e != nil {
		t.Fatal(e)
	}
	h, e := s.acquire(ctx, resource.Path)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(h.Path, []byte("{"), 0600); e != nil {
		t.Fatal(e)
	}
	h.Release()
	if e = s.DeleteMedia(ctx, m.ID); e == nil {
		t.Fatal("corrupt links silently ignored")
	}
	if _, e = s.AcquireMedia(ctx, m.ID); e != nil {
		t.Fatal(e)
	}
	links, _, e := s.readMediaLinks(ctx, "good", true)
	if e != nil || len(links.MediaIDs) != 0 {
		t.Fatalf("good project not unlinked: %+v %v", links, e)
	}
}
func TestMediaMetadataSurvivesReplacementAndValidatesPatch(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	m := mediaTestImport(t, s, "image.png")
	updated, e := s.UpdateMediaMetadata(ctx, m.ID, Object{"tags": []any{"  snow  ", "night"}, "attribution": Object{"provider": "Pexels", "author": "Artist", "license": "CC0"}, "overlay": true, "fileName": "must not change"})
	if e != nil || !reflect.DeepEqual(updated.Tags, []string{"snow", "night"}) || updated.Attribution == nil || updated.Attribution.Provider != "Pexels" || updated.FileName != m.FileName {
		t.Fatalf("patch %+v %v", updated, e)
	}
	overlay := true
	withBG, e := s.SaveMediaBackground(ctx, m.ID, MediaBackgroundDetection{Kind: "green", KeyColor: "#00ff00"}, m.ContentHash, &overlay)
	if e != nil || withBG.Background == nil {
		t.Fatal(e)
	}
	h, e := s.AcquireMedia(ctx, m.ID)
	if e != nil {
		t.Fatal(e)
	}
	oldPath := h.Path
	defer h.Release()
	b, e := os.ReadFile(filepath.Join("testdata", "__fixtures__", "image.jpg"))
	if e != nil {
		t.Fatal(e)
	}
	replaced, e := s.ReplaceMedia(ctx, m.ID, ImportMediaOptions{FileName: m.FileName, DeclaredMIME: "image/jpeg", Body: bytes.NewReader(b)})
	if e != nil || replaced.ContentHash == m.ContentHash || replaced.ID != m.ID || replaced.CreatedAt != m.CreatedAt || replaced.Background != nil || !reflect.DeepEqual(replaced.Tags, updated.Tags) || replaced.Attribution == nil || !replaced.Overlay {
		t.Fatalf("replace %+v %v", replaced, e)
	}
	newHandle, e := s.AcquireMedia(ctx, m.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer newHandle.Release()
	if oldPath == newHandle.Path {
		t.Fatal("same-id replacement overwrote immutable source path")
	}
	if _, e = s.SaveMediaBackground(ctx, m.ID, MediaBackgroundDetection{Kind: "black"}, m.ContentHash, nil); e == nil {
		t.Fatal("stale classification saved")
	}
	cleared, e := s.UpdateMediaMetadata(ctx, m.ID, Object{"tags": []any{}, "attribution": nil, "overlay": false})
	if e != nil || len(cleared.Tags) != 0 || cleared.Attribution != nil || cleared.Overlay {
		t.Fatalf("clear %+v %v", cleared, e)
	}
	for _, p := range []Object{nil, {"tags": []any{" "}}, {"tags": []any{1}}, {"tags": []any{strings.Repeat("x", 65)}}, {"tags": make([]string, 33)}, {"attribution": Object{}}, {"attribution": Object{"provider": "ok", "author": 1}}, {"overlay": "yes"}} {
		if _, e = ValidateMediaMetadataPatch(p); e == nil {
			t.Fatalf("invalid patch accepted: %#v", p)
		}
	}
}

type mediaBrokenReader struct{}

func (mediaBrokenReader) Read([]byte) (int, error) { return 0, errors.New("stream failed") }
func TestMediaImportRollsBackFailuresAndEnforcesLimits(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	for _, body := range []io.Reader{strings.NewReader("not a movie"), mediaBrokenReader{}} {
		if _, e := s.ImportMedia(ctx, ImportMediaOptions{FileName: "bad.mp4", Body: body}); e == nil {
			t.Fatal("bad import succeeded")
		}
		records, e := s.ListMedia(ctx, "")
		if e != nil || len(records) != 0 {
			t.Fatalf("failed import retained: %v %v", records, e)
		}
	}
	if _, e := s.ImportMedia(ctx, ImportMediaOptions{ProjectID: "missing", FileName: "bad.mp4", Body: mediaBrokenReader{}}); e == nil {
		t.Fatal("missing project accepted")
	}
	entries, e := os.ReadDir(s.StagingDir)
	if e != nil || len(entries) != 0 {
		t.Fatalf("failed import staged bytes: %v %v", entries, e)
	}
	t.Setenv("YTF_EDITOR_MAX_IMAGE_BYTES", "1")
	b, e := os.ReadFile(filepath.Join("testdata", "__fixtures__", "image.png"))
	if e != nil {
		t.Fatal(e)
	}
	for _, mime := range []string{"image/png", "video/mp4"} {
		_, e = s.ImportMedia(ctx, ImportMediaOptions{FileName: "image.png", DeclaredMIME: mime, Body: bytes.NewReader(b)})
		mediaErrorCode(t, e, "payload_too_large")
	}
	t.Setenv("YTF_EDITOR_MAX_IMAGE_BYTES", "")
	opts := mediaFileOptions(t, "image.png")
	opts.ContentHash = "sha256:" + strings.Repeat("0", 64)
	_, _, e = s.ImportFileOnce(ctx, opts)
	mediaErrorCode(t, e, "invalid_request")
}

func TestMediaDeleteLinkRewriteFailureKeepsMediaAndUnlinksOthers(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	for _, pid := range []string{"writable", "blocked"} {
		mediaTestProject(t, s, pid)
	}
	m := mediaTestImport(t, s, "image.png")
	for _, pid := range []string{"writable", "blocked"} {
		if _, e := s.LinkMedia(ctx, pid, m.ID); e != nil {
			t.Fatal(e)
		}
	}
	r := s.Repository.(*mediaTestRepository)
	r.FailCAS = map[string]error{"projects/blocked/links": errors.New("write denied")}
	if e := s.DeleteMedia(ctx, m.ID); e == nil {
		t.Fatal("link rewrite failure ignored")
	}
	if _, e := s.GetMediaMetadata(ctx, m.ID); e != nil {
		t.Fatal(e)
	}
	good, _, e := s.readMediaLinks(ctx, "writable", true)
	if e != nil || len(good.MediaIDs) != 0 {
		t.Fatalf("writable links: %v %v", good, e)
	}
	bad, _, e := s.readMediaLinks(ctx, "blocked", true)
	if e != nil || len(bad.MediaIDs) != 1 {
		t.Fatalf("blocked links: %v %v", bad, e)
	}
	r.FailCAS = nil
	if e = s.DeleteMedia(ctx, m.ID); e != nil {
		t.Fatal(e)
	}
}
func TestMediaWireShapeHidesInternalFieldsAndPreservesEmptyMetadata(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	m := mediaTestImport(t, s, "image.png")
	m, e := s.UpdateMediaMetadata(ctx, m.ID, Object{"tags": []any{}, "attribution": Object{"provider": "", "author": ""}})
	if e != nil {
		t.Fatal(e)
	}
	b, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	var o Object
	if e = json.Unmarshal(b, &o); e != nil {
		t.Fatal(e)
	}
	if tags, ok := o["tags"].([]any); !ok || len(tags) != 0 {
		t.Fatalf("empty tags omitted: %s", b)
	}
	attribution, ok := o["attribution"].(map[string]any)
	if !ok || attribution["provider"] != "" || attribution["author"] != "" {
		t.Fatalf("empty attribution omitted: %s", b)
	}
	for _, key := range []string{"sourceFile", "updatedAt", "storageType", "lastUsedAt"} {
		if _, ok := o[key]; ok {
			t.Fatalf("storage field %s leaked", key)
		}
	}
	h, e := s.AcquireMedia(ctx, m.ID)
	if e != nil {
		t.Fatal(e)
	}
	h.Release()
}

func TestMediaImportPublishesBinaryRoot(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	s.Prober = mediaProberFunc(func(context.Context, string) (ProbeResult, error) {
		return ProbeResult{Kind: "image", MIME: "image/png"}, nil
	})
	m, err := s.ImportMedia(ctx, ImportMediaOptions{FileName: "source.png", DeclaredMIME: "image/png", Body: strings.NewReader("source bytes")})
	if err != nil {
		t.Fatal(err)
	}
	root, err := s.Repository.Get(ctx, "media/"+m.ID+"/source")
	if err != nil {
		t.Fatalf("imported source has no durable binary root: %v", err)
	}
	stored, _, err := s.readMedia(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if root.Path != "editor/media/"+m.ID+"/"+stored.SourceFile || root.Revision != m.ContentHash {
		t.Fatalf("binary root does not match metadata: %+v, %+v", root, stored)
	}
	if _, err = s.UpdateMediaMetadata(ctx, m.ID, Object{"tags": []any{}}); err != nil {
		t.Fatal(err)
	}
	after, err := s.Repository.Get(ctx, root.Key)
	if err != nil || after != root {
		t.Fatalf("metadata-only update changed binary root: %+v, %v", after, err)
	}
	ids, err := s.mediaIDs(ctx)
	if err != nil || !reflect.DeepEqual(ids, []string{m.ID}) {
		t.Fatalf("binary row leaked into media discovery: %v, %v", ids, err)
	}
}

func newBinaryMediaTestService(t *testing.T) (*Service, *exportTestMedia, *exportTestRepository) {
	t.Helper()
	backend := &exportTestMedia{root: t.TempDir(), paths: map[string]string{}}
	repo := &exportTestRepository{rows: map[string]Resource{}, media: backend}
	backend.repo = repo
	s := newMediaTestService(t)
	s.Media, s.Repository = backend, repo
	s.Store.Media, s.Store.Repository = backend, repo
	s.Prober = mediaProberFunc(func(context.Context, string) (ProbeResult, error) {
		return ProbeResult{Kind: "image", MIME: "image/png"}, nil
	})
	return s, backend, repo
}

func TestMediaImportBinaryAndMetadataPublishTogether(t *testing.T) {
	for _, fromFile := range []bool{false, true} {
		name := "stream"
		if fromFile {
			name = "file"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s, backend, repo := newBinaryMediaTestService(t)
			content := []byte("independent source bytes")
			var m MediaRecord
			var err error
			if fromFile {
				file := filepath.Join(t.TempDir(), "source.png")
				if err = os.WriteFile(file, content, 0600); err != nil {
					t.Fatal(err)
				}
				m, err = s.ImportMediaFile(ctx, ImportMediaFileOptions{File: file, FileName: "source.png", DeclaredMIME: "image/png"})
			} else {
				m, err = s.ImportMedia(ctx, ImportMediaOptions{FileName: "source.png", DeclaredMIME: "image/png", Body: bytes.NewReader(content)})
			}
			if err != nil {
				t.Fatal(err)
			}
			binary, err := repo.Get(ctx, "media/"+m.ID+"/source")
			if err != nil || binary.Revision != m.ContentHash {
				t.Fatalf("missing matching source root: %+v, %v", binary, err)
			}
			head, err := repo.Get(ctx, "media/"+m.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(backend.published) == 0 || len(backend.published[0]) != 2 {
				t.Fatalf("binary and metadata were not published together: %+v", backend.published)
			}
			batch := backend.published[0]
			if batch[0].Path != binary.Path || batch[1].Path != head.Path {
				t.Fatalf("publication does not match committed heads: %+v", batch)
			}
			h, err := s.AcquireMedia(ctx, m.ID)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(h.Path)
			h.Release()
			if err != nil || !bytes.Equal(actual, content) {
				t.Fatalf("imported bytes differ: %q, %v", actual, err)
			}
		})
	}
}

func TestMediaReplaceBinaryRollbackAndRelease(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "binary-head"
		if legacy {
			name = "phase05-source-fallback"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s, backend, repo := newBinaryMediaTestService(t)
			oldBytes := []byte("old source bytes")
			m, err := s.ImportMedia(ctx, ImportMediaOptions{FileName: "source.png", DeclaredMIME: "image/png", Body: bytes.NewReader(oldBytes)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.UpdateMediaMetadata(ctx, m.ID, Object{"tags": []any{}}); err != nil {
				t.Fatal(err)
			}
			binaryKey, metadataKey := "media/"+m.ID+"/source", "media/"+m.ID
			binary, err := repo.Get(ctx, binaryKey)
			if err != nil {
				t.Fatal(err)
			}
			oldPath := binary.Path
			if legacy {
				delete(repo.rows, binaryKey)
				oldPath = "editor/media/" + m.ID + "/" + m.FileName
				backend.paths[oldPath] = backend.paths[binary.Path]
				delete(backend.paths, binary.Path)
				stored, rev, err := s.readMedia(ctx, m.ID)
				if err != nil {
					t.Fatal(err)
				}
				stored.SourceFile = ""
				if err = s.saveMedia(ctx, stored, rev); err != nil {
					t.Fatal(err)
				}
			}
			oldHead, err := repo.Get(ctx, metadataKey)
			if err != nil {
				t.Fatal(err)
			}
			oldFile := backend.paths[oldPath]
			sawMetadataCAS := false
			repo.beforeCAS = func(key string, next Resource) {
				if key != metadataKey {
					return
				}
				sawMetadataCAS = true
				if repo.rows[metadataKey] != oldHead {
					t.Error("metadata head changed before its CAS")
				}
				contents, readErr := os.ReadFile(oldFile)
				if readErr != nil || !bytes.Equal(contents, oldBytes) {
					t.Errorf("old metadata points at unavailable bytes before commit: %q, %v", contents, readErr)
				}
			}
			repo.failKey = metadataKey
			replacement := []byte("replacement source bytes")
			_, err = s.ReplaceMedia(ctx, m.ID, ImportMediaOptions{Body: bytes.NewReader(replacement), DeclaredMIME: "image/png"})
			if err == nil {
				t.Fatal("metadata CAS failure was ignored")
			}
			if head, getErr := repo.Get(ctx, metadataKey); getErr != nil || head != oldHead {
				t.Fatalf("failed replacement changed metadata: %+v, %v", head, getErr)
			}
			if legacy {
				if _, getErr := repo.Get(ctx, binaryKey); !mediaMissing(getErr) {
					t.Fatalf("failed legacy replacement retained an adopted binary head: %v", getErr)
				}
			} else if root, getErr := repo.Get(ctx, binaryKey); getErr != nil || root != binary {
				t.Fatalf("failed replacement changed binary head: %+v, %v", root, getErr)
			}
			h, err := s.AcquireMedia(ctx, m.ID)
			if err != nil {
				t.Fatal(err)
			}
			contents, err := os.ReadFile(h.Path)
			h.Release()
			if err != nil || !bytes.Equal(contents, oldBytes) || backend.paths[oldPath] != oldFile {
				t.Fatalf("failed replacement released old source: %q, %v", contents, err)
			}
			if len(backend.paths) != 3 {
				t.Fatalf("failed replacement leaked bindings: %+v", backend.paths)
			}
			repo.failKey = ""
			updated, err := s.ReplaceMedia(ctx, m.ID, ImportMediaOptions{Body: bytes.NewReader(replacement), DeclaredMIME: "image/png"})
			if err != nil {
				t.Fatal(err)
			}
			if !sawMetadataCAS || updated.ContentHash == m.ContentHash || updated.Tags == nil || len(updated.Tags) != 0 {
				t.Fatalf("replacement lost metadata or skipped CAS: %+v", updated)
			}
			if _, exists := backend.paths[oldPath]; exists {
				t.Fatal("successful replacement retained old unowned source binding")
			}
			root, err := repo.Get(ctx, binaryKey)
			if err != nil || root.Revision != updated.ContentHash || root.Path == oldPath {
				t.Fatalf("replacement binary root disagrees with metadata: %+v, %v", root, err)
			}
		})
	}
}

func TestMediaDeleteReleasesBinaryRootBeforeRawUnbind(t *testing.T) {
	ctx := context.Background()
	s, backend, repo := newBinaryMediaTestService(t)
	m, err := s.ImportMedia(ctx, ImportMediaOptions{FileName: "source.png", DeclaredMIME: "image/png", Body: strings.NewReader("source bytes")})
	if err != nil {
		t.Fatal(err)
	}
	root, err := repo.Get(ctx, "media/"+m.ID+"/source")
	if err != nil {
		t.Fatal(err)
	}
	// A non-pointer legacy artifact still requires raw Unbind during deletion.
	artifact, _, _, err := s.stageFile("editor/media/"+m.ID+"/thumbnail.jpg", strings.NewReader("thumbnail"), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(artifact.File)
	if err = backend.Publish(ctx, []media.Artifact{artifact}, nil); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteMedia(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"media/" + m.ID, "media/" + m.ID + "/source"} {
		if _, err = repo.Get(ctx, key); !mediaMissing(err) {
			t.Fatalf("deleted media retained head %s: %v", key, err)
		}
	}
	for _, p := range []string{root.Path, artifact.Path} {
		if _, exists := backend.paths[p]; exists {
			t.Fatalf("deleted media retained binding %s", p)
		}
	}
}

func TestMediaImportBinaryPublicationRollback(t *testing.T) {
	ctx := context.Background()
	s, backend, repo := newBinaryMediaTestService(t)
	const id = "failed-import"
	repo.failKey = "media/" + id
	_, err := s.importMedia(ctx, id, ImportMediaOptions{FileName: "source.png", DeclaredMIME: "image/png", Body: strings.NewReader("source bytes")}, nil, "")
	if err == nil {
		t.Fatal("metadata CAS failure was ignored")
	}
	if len(repo.rows) != 0 || len(backend.paths) != 0 {
		t.Fatalf("failed first import retained heads or source bindings: %+v, %+v", repo.rows, backend.paths)
	}
	entries, err := os.ReadDir(s.StagingDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed first import retained staged files: %v, %v", entries, err)
	}
}
