package web

// Hosted media + text-export suite: story cover (?size=/?volume= + cache),
// project asset upload/list, text export (fail/keep/replace, file/download)
// and blob delivery — all over HTTP on the PG backend with a real
// project_documents/blobs/export_cache registry.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
)

// pgMediaFixture extends the hosted gateway with audio deps, a local blob
// store and one completed chapter so cover/export paths have real inputs.
type pgMediaFixture struct {
	fx    *hostedFixture
	nar   *postgres.Narrative
	scope postgres.ScopeRef
	st    library.Story
}

func provisionHostedMedia(t *testing.T) *pgMediaFixture {
	t.Helper()
	fx := provisionHostedGateway(t, "media-proj", "truyen-media")
	ctx := context.Background()
	pool := fx.pool
	nar := postgres.NewNarrative(pool)
	audioRepo := postgres.NewAudio(pool)
	cache := postgres.NewExportCache(pool)
	jobs := postgres.NewJobs(pool)
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("local blob store: %v", err)
	}
	fx.g.SetAudioStore(&PGAudioDeps{Jobs: jobs, Audio: audioRepo, Cache: cache, Nar: nar, Blobs: blobs})

	scope, err := nar.Resolve(ctx, fx.project, fx.story, nil, false)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// Book title + one completed chapter: VolumeRanges/exports need both.
	eng, err := fx.g.openFactStore(library.Story{Project: fx.project, Story: fx.story}, nil)
	if err != nil {
		t.Fatalf("open story store: %v", err)
	}
	if err := eng.Book.Save(domain.BookMetadata{Title: "Sách PG", Synopsis: "x"}); err != nil {
		t.Fatalf("save book: %v", err)
	}
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated,
		"Nội dung chương một.", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatalf("accept chapter: %v", err)
	}
	if err := nar.ProgressSave(ctx, scope, &domain.Progress{
		Phase: domain.PhaseWriting, CurrentChapter: 2, CompletedChapters: []int{1},
	}); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := eng.Drafts.SaveFinalChapter(1, "Nội dung chương một."); err != nil {
		t.Fatalf("save chapter text: %v", err)
	}

	st := library.Story{Project: fx.project, Story: fx.story, ID: fx.project + "/" + fx.story}
	return &pgMediaFixture{fx: fx, nar: nar, scope: scope, st: st}
}

// storyCall drives serveStoryRead through the same route-level gate as
// storyReq but reuses the fixture's authenticated user.
func (f *pgMediaFixture) storyCall(t *testing.T, u identity.User, method, rest, query, body string) *httptest.ResponseRecorder {
	t.Helper()
	path := "/api/projects/" + f.st.Project + "/stories/" + f.st.Story + rest
	if query != "" {
		path += "?" + query
	}
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	if !storyAccessAllowed(u, f.st, method, rest) {
		jsonError(rec, http.StatusNotFound, "không tìm thấy truyện")
		return rec
	}
	if !f.fx.g.serveStoryRead(rec, req, u, f.st, rest) {
		t.Fatalf("serveStoryRead false for %s %s", method, rest)
	}
	return rec
}

// audioCall drives serveStoryAudio like the existing PG audio tests.
func (f *pgMediaFixture) audioCall(t *testing.T, u identity.User, method, path, query string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != nil {
		buf, _ := json.Marshal(body)
		r = httptest.NewRequest(method, "http://x"+path+query, bytes.NewReader(buf))
	} else {
		r = httptest.NewRequest(method, "http://x"+path+query, nil)
	}
	w := httptest.NewRecorder()
	if !f.fx.g.serveStoryAudio(w, r, u, f.st, path) {
		t.Fatalf("%s %s not handled", method, path)
	}
	return w
}

// uploadAssetCall posts a multipart asset upload through serveProjects.
func (f *pgMediaFixture) uploadAssetCall(t *testing.T, u identity.User, kind, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if kind != "" {
		_ = mw.WriteField("kind", kind)
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(data)
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/api/projects/"+f.st.Project+"/assets", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	f.fx.g.serveProjects(rec, r, u)
	return rec
}

func (f *pgMediaFixture) projectsCall(t *testing.T, u identity.User, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	f.fx.g.serveProjects(rec, r, u)
	return rec
}

func decodeAny(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %d %q: %v", rec.Code, rec.Body.String(), err)
	}
	return out
}

// tinyPNG renders a deterministic decodable cover upload.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 30), uint8(y * 30), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// ── cover ───────────────────────────────────────────────────────────────────

func TestPGStoryCoverHTTP(t *testing.T) {
	f := provisionHostedMedia(t)

	// Generated cover: every ?size= variant file mode documents.
	for _, size := range []string{"", "96", "192", "512", "1400"} {
		q := "size=" + size
		if size == "" {
			q = ""
		}
		rec := f.storyCall(t, f.fx.owner, "GET", "/cover", q, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("cover size=%q: %d %s", size, rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
			t.Fatalf("cover size=%q content-type %q", size, ct)
		}
		if _, err := png.Decode(bytes.NewReader(rec.Body.Bytes())); err != nil {
			t.Fatalf("cover size=%q undecodable: %v", size, err)
		}
	}

	// Same request twice: the second hit must serve the cached blob — proven
	// by tampering with the stored bytes between calls.
	rec1 := f.storyCall(t, f.fx.owner, "GET", "/cover", "size=192&volume=1", "")
	if rec1.Code != http.StatusOK {
		t.Fatalf("cover first: %d", rec1.Code)
	}
	ctx := context.Background()
	entries, err := f.fx.g.pgAudio.Cache.List(ctx, f.scope)
	if err != nil || len(entries) == 0 {
		t.Fatalf("cover must register a cache row: %v %d", err, len(entries))
	}
	// Tamper every cached cover blob: whichever row the hit resolves, the
	// bytes served can only come from the cache — never a fresh render.
	fake := []byte("cached-not-rendered")
	count := 0
	for i := range entries {
		if entries[i].Kind != "cover" {
			continue
		}
		count++
		blob, err := f.fx.g.pgAudio.Audio.BlobByID(ctx, f.scope.ProjectID, entries[i].BlobID)
		if err != nil {
			t.Fatalf("cover blob: %v", err)
		}
		if _, err := f.fx.g.pgAudio.Blobs.Put(ctx, blob.ObjectKey, bytes.NewReader(fake), blobstore.PutOptions{ContentType: "image/png"}); err != nil {
			t.Fatalf("tamper blob: %v", err)
		}
	}
	if count == 0 {
		t.Fatal("no cover cache row")
	}
	rec2 := f.storyCall(t, f.fx.owner, "GET", "/cover", "size=192&volume=1", "")
	if rec2.Code != http.StatusOK || !bytes.Equal(rec2.Body.Bytes(), fake) {
		t.Fatalf("second cover must serve the cached blob: %d %d bytes", rec2.Code, len(rec2.Body.Bytes()))
	}
	if rec := f.uploadAssetCall(t, f.fx.owner, "cover", "bia moi.png", tinyPNG(t)); rec.Code != http.StatusCreated {
		t.Fatalf("asset upload for cover: %d %s", rec.Code, rec.Body.String())
	}
	rec3 := f.storyCall(t, f.fx.owner, "GET", "/cover", "size=192&volume=1", "")
	if rec3.Code != http.StatusOK {
		t.Fatalf("cover after upload: %d", rec3.Code)
	}
	if bytes.Equal(rec3.Body.Bytes(), fake) || bytes.Equal(rec3.Body.Bytes(), rec1.Body.Bytes()) {
		t.Fatal("cover must re-render for the new uploaded image")
	}
	if _, err := png.Decode(bytes.NewReader(rec3.Body.Bytes())); err != nil {
		t.Fatalf("rendered cover undecodable: %v", err)
	}

	// Bad size/volume keep file-mode validation.
	for _, q := range []string{"size=abc", "size=0", "size=513", "volume=0", "volume=x"} {
		if rec := f.storyCall(t, f.fx.owner, "GET", "/cover", q, ""); rec.Code != http.StatusBadRequest {
			t.Fatalf("cover %q: %d", q, rec.Code)
		}
	}
}

// ── assets upload + list ────────────────────────────────────────────────────

func TestPGProjectAssetsHTTP(t *testing.T) {
	f := provisionHostedMedia(t)

	w := f.uploadAssetCall(t, f.fx.owner, "cover", "bia-1.png", tinyPNG(t))
	if w.Code != http.StatusCreated {
		t.Fatalf("cover upload: %d %s", w.Code, w.Body.String())
	}
	if got := decodeAny(t, w)["path"]; got != "bia/bia-1.png" {
		t.Fatalf("cover path = %v", got)
	}
	w = f.uploadAssetCall(t, f.fx.owner, "music", "mo-dau.mp3", []byte("mp3-bytes"))
	if w.Code != http.StatusCreated || decodeAny(t, w)["path"] != "nhac/mo-dau.mp3" {
		t.Fatalf("music upload: %d %s", w.Code, w.Body.String())
	}
	// Same name dedupes like file mode's -2 suffix.
	w = f.uploadAssetCall(t, f.fx.owner, "music", "mo-dau.mp3", []byte("mp3-two"))
	if w.Code != http.StatusCreated || decodeAny(t, w)["path"] != "nhac/mo-dau-2.mp3" {
		t.Fatalf("dup upload: %d %s", w.Code, w.Body.String())
	}
	// Traversal names stay inside the prefix.
	w = f.uploadAssetCall(t, f.fx.owner, "cover", "../../x.jpg", tinyPNG(t))
	if w.Code != http.StatusCreated || decodeAny(t, w)["path"] != "bia/x.jpg" {
		t.Fatalf("traversal upload: %d %s", w.Code, w.Body.String())
	}

	// GET /assets lists the docs the uploads registered.
	w = f.projectsCall(t, f.fx.owner, "GET", "/api/projects/"+f.st.Project+"/assets")
	if w.Code != http.StatusOK {
		t.Fatalf("list assets: %d %s", w.Code, w.Body.String())
	}
	body := decodeAny(t, w)
	assets, _ := body["assets"].([]any)
	paths := map[string]bool{}
	for _, a := range assets {
		paths[a.(map[string]any)["path"].(string)] = true
	}
	for _, want := range []string{"bia/bia-1.png", "bia/x.jpg", "nhac/mo-dau.mp3", "nhac/mo-dau-2.mp3"} {
		if !paths[want] {
			t.Fatalf("assets missing %s: %s", want, w.Body.String())
		}
	}

	// Authz: viewer can't upload, outsider can't list.
	if w := f.uploadAssetCall(t, f.fx.viewer, "cover", "v.png", tinyPNG(t)); w.Code == http.StatusCreated {
		t.Fatalf("viewer upload must be rejected")
	}
	if w := f.projectsCall(t, f.fx.outsider, "GET", "/api/projects/"+f.st.Project+"/assets"); w.Code == http.StatusOK {
		t.Fatalf("outsider list must be rejected: %d", w.Code)
	}
	// Bad kind / bad extension stay 400.
	if w := f.uploadAssetCall(t, f.fx.owner, "cover", "a.mp3", []byte("x")); w.Code != http.StatusBadRequest {
		t.Fatalf("mp3 as cover: %d", w.Code)
	}
	if w := f.uploadAssetCall(t, f.fx.owner, "voice", "a.wav", []byte("x")); w.Code != http.StatusBadRequest {
		t.Fatalf("voice kind: %d", w.Code)
	}
}

// ── text exports ────────────────────────────────────────────────────────────

func TestPGTextExportsHTTP(t *testing.T) {
	f := provisionHostedMedia(t)
	owner := f.fx.owner

	// GET /export streams a fresh txt without registering anything.
	rec := f.storyCall(t, owner, "GET", "/export", "format=txt", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /export: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Nội dung chương một") {
		t.Fatalf("export body missing chapter text: %s", rec.Body.String())
	}

	// POST /exports produces the txt, registers it, and returns the blob key.
	rec = f.storyCall(t, owner, "POST", "/exports", "", `{"format":"txt","volume":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /exports: %d %s", rec.Code, rec.Body.String())
	}
	res := decodeAny(t, rec)
	key, _ := res["path"].(string)
	if key == "" || res["chapters"].(float64) != 1 {
		t.Fatalf("export result: %v", res)
	}

	// GET /exports lists the artifact in the matrix + files.
	rec = f.storyCall(t, owner, "GET", "/exports", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /exports: %d %s", rec.Code, rec.Body.String())
	}
	list := decodeAny(t, rec)
	files, _ := list["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("exports files: %s", rec.Body.String())
	}
	f0 := files[0].(map[string]any)
	if f0["path"] != key || f0["format"] != "txt" {
		t.Fatalf("export file row: %v", f0)
	}

	// Same request with default onExists=fail → the file-mode 409 payload.
	rec = f.storyCall(t, owner, "POST", "/exports", "", `{"format":"txt","volume":1}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("export conflict: %d %s", rec.Code, rec.Body.String())
	}
	conf := decodeAny(t, rec)
	if conf["existing"] == nil || conf["incoming"] == nil {
		t.Fatalf("conflict payload: %v", conf)
	}

	// onExists=keep mints a timestamped variant row instead of overwriting.
	rec = f.storyCall(t, owner, "POST", "/exports", "", `{"format":"txt","volume":1,"onExists":"keep"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("keep export: %d %s", rec.Code, rec.Body.String())
	}
	keep := decodeAny(t, rec)
	if keep["path"] == key {
		t.Fatalf("keep must produce a distinct artifact: %v", keep)
	}
	rec = f.storyCall(t, owner, "GET", "/exports", "", "")
	if l := decodeAny(t, rec)["files"].([]any); len(l) != 2 {
		t.Fatalf("keep must add a file: %s", rec.Body.String())
	}

	// onExists=replace overwrites the logical row (files stay at 2).
	rec = f.storyCall(t, owner, "POST", "/exports", "", `{"format":"txt","volume":1,"onExists":"replace"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("replace export: %d %s", rec.Code, rec.Body.String())
	}
	rec = f.storyCall(t, owner, "GET", "/exports", "", "")
	if l := decodeAny(t, rec)["files"].([]any); len(l) != 2 {
		t.Fatalf("replace must not add a file: %s", rec.Body.String())
	}

	// /exports/file serves by the produced object key.
	rec = f.storyCall(t, owner, "GET", "/exports/file", "path="+url.QueryEscape(key), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("exports/file by key: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Nội dung chương một") {
		t.Fatalf("exports/file body: %s", rec.Body.String())
	}

	// /exports/download hands the same bytes under the friendly name.
	rec = f.storyCall(t, owner, "GET", "/exports/download", "file="+url.QueryEscape(key), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("exports/download: %d %s", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("download disposition: %q", cd)
	}

	// Unknown logical path 404s; outsider is cut at the route gate.
	rec = f.storyCall(t, owner, "GET", "/exports/file", "path=tap-99/nope.txt", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing logical file: %d", rec.Code)
	}
	rec = f.storyCall(t, f.fx.outsider, "GET", "/exports/file", "path="+url.QueryEscape(key), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("outsider exports/file: %d", rec.Code)
	}
	rec = f.storyCall(t, f.fx.outsider, "POST", "/exports", "", `{"format":"txt"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("outsider POST exports: %d", rec.Code)
	}

	// epub goes through the same registry path.
	rec = f.storyCall(t, owner, "POST", "/exports", "", `{"format":"epub","volume":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("epub export: %d %s", rec.Code, rec.Body.String())
	}
	epubKey, _ := decodeAny(t, rec)["path"].(string)
	rec = f.storyCall(t, owner, "GET", "/exports/file", "path="+url.QueryEscape(epubKey), "")
	if rec.Code != http.StatusOK || !bytes.HasPrefix(rec.Body.Bytes(), []byte("PK")) {
		got := rec.Body.Bytes()
		if len(got) > 8 {
			got = got[:8]
		}
		t.Fatalf("epub bytes: %d %q", rec.Code, got)
	}
}

// ── blob delivery ───────────────────────────────────────────────────────────

func TestPGBlobDeliveryHTTP(t *testing.T) {
	f := provisionHostedMedia(t)
	owner := f.fx.owner

	// Produce one export blob to stream.
	rec := f.storyCall(t, owner, "POST", "/exports", "", `{"format":"txt","volume":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed export: %d", rec.Code)
	}
	key, _ := decodeAny(t, rec)["path"].(string)

	r := httptest.NewRequest("GET", "/api/blobs/stream/"+key, nil)
	w := httptest.NewRecorder()
	if !f.fx.g.serveBlobDelivery(w, r, owner) {
		t.Fatal("route not handled")
	}
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Nội dung chương một") {
		t.Fatalf("stream: %d %s", w.Code, w.Body.String())
	}

	// Viewer (member, read role) may stream; outsider is denied.
	r = httptest.NewRequest("GET", "/api/blobs/stream/"+key, nil)
	w = httptest.NewRecorder()
	f.fx.g.serveBlobDelivery(w, r, f.fx.viewer)
	if w.Code != http.StatusOK {
		t.Fatalf("viewer stream: %d", w.Code)
	}
	r = httptest.NewRequest("GET", "/api/blobs/stream/"+key, nil)
	w = httptest.NewRecorder()
	f.fx.g.serveBlobDelivery(w, r, f.fx.outsider)
	if w.Code != http.StatusForbidden && w.Code != http.StatusBadRequest {
		t.Fatalf("outsider stream: %d", w.Code)
	}

	// Sign falls back to the stream URL on the local driver.
	r = httptest.NewRequest("GET", "/api/blobs/sign/"+key, nil)
	w = httptest.NewRecorder()
	f.fx.g.serveBlobDelivery(w, r, owner)
	if w.Code != http.StatusOK {
		t.Fatalf("sign: %d %s", w.Code, w.Body.String())
	}
	body := decodeAny(t, w)
	if body["mode"] != "stream" && body["mode"] != "presigned" {
		t.Fatalf("sign mode: %v", body)
	}
}

// ── audio export with assets + peaks ────────────────────────────────────────

func TestPGAudioExportMusicAndPeaks(t *testing.T) {
	if !audio.FFmpegAvailable() {
		t.Skip("ffmpeg required for the hosted export pipeline")
	}
	f := provisionHostedMedia(t)
	ctx := context.Background()
	pool := f.fx.pool
	audioRepo := postgres.NewAudio(pool)
	jobs := postgres.NewJobs(pool)
	blobs := f.fx.g.pgAudio.Blobs

	ref, err := f.nar.ChapterRefByNumber(ctx, f.scope, 1)
	if err != nil {
		t.Fatalf("chapter ref: %v", err)
	}
	script := audio.Script{Chapter: 1, Volume: 1, Status: audio.StatusLocked,
		SourceHash: audio.SourceHash("Nội dung chương một."),
		Segments:   []audio.Segment{{ID: "s0001", Speaker: audio.NarratorID, Text: "Nội dung chương một."}}}
	payload, _ := json.Marshal(script)
	if _, err := audioRepo.SaveScript(ctx, f.scope, postgres.Script{
		ChapterID: ref.ID, RevisionNo: 1, SourceRevision: int32(ref.CurrentRevision),
		SourceSHA256: script.SourceHash, Status: audio.StatusLocked, Payload: payload}); err != nil {
		t.Fatalf("script: %v", err)
	}
	if _, err := audioRepo.SaveSegment(ctx, f.scope.ProjectID, ref.ID, 1, postgres.Segment{
		SegmentID: "s0001", Ordinal: 0, Speaker: audio.NarratorID,
		Text: "Nội dung chương một.", Payload: []byte(`{}`)}); err != nil {
		t.Fatalf("segment: %v", err)
	}

	// A selected render for the segment: real WAV bytes in the blob store.
	wav := testWAV(t, 8000, 0.2)
	wavSHA := sha256.Sum256(wav)
	wavHex := hex.EncodeToString(wavSHA[:])
	wavKey, err := blobkey.Keys{}.AudioSegment(postgres.UUIDString(f.scope.ProjectID), postgres.UUIDString(f.scope.StoryID), wavHex, "wav")
	if err != nil {
		t.Fatalf("wav key: %v", err)
	}
	if _, err := blobs.Put(ctx, wavKey, bytes.NewReader(wav), blobstore.PutOptions{ContentType: "audio/wav"}); err != nil {
		t.Fatalf("put wav: %v", err)
	}
	blobRow, err := audioRepo.PublishBlob(ctx, postgres.PublishInput{ProjectID: f.scope.ProjectID, Key: wavKey, Driver: "local", ContentType: "audio/wav", SHA256: wavHex, Size: int64(len(wav))})
	if err != nil {
		t.Fatalf("mark blob: %v", err)
	}
	synthKey := audio.SynthKey{Text: script.Segments[0].Text, VoiceID: "", Speed: 1,
		Pitch: 0, Quality: "standard", Engine: "remote", Model: "remote"}.Hash()
	if _, err := audioRepo.AddRender(ctx, f.scope.ProjectID, ref.ID, 1, postgres.Render{
		SegmentID: "s0001", SynthKey: synthKey, BlobID: blobRow.ID,
		DurationSec: 0.2, SampleRate: 8000, Selected: true,
	}); err != nil {
		t.Fatalf("add render: %v", err)
	}
	if _, err := audioRepo.SelectRender(ctx, f.scope.ProjectID, ref.ID, 1, "s0001", synthKey); err != nil {
		t.Fatalf("select render: %v", err)
	}

	// Upload a real decodable mp3 as the music bed so pickMusic works.
	musicPath := filepath.Join(t.TempDir(), "mo-dau.mp3")
	if out, err := exec.CommandContext(ctx, "ffmpeg", "-y", "-f", "lavfi", "-i",
		"sine=frequency=440:duration=0.3", "-c:a", "libmp3lame", musicPath).CombinedOutput(); err != nil {
		t.Fatalf("music mp3: %v %s", err, out)
	}
	mp3Data, err := os.ReadFile(musicPath)
	if err != nil {
		t.Fatal(err)
	}
	if rec := f.uploadAssetCall(t, f.fx.owner, "music", "mo-dau.mp3", mp3Data); rec.Code != http.StatusCreated {
		t.Fatalf("music upload: %d %s", rec.Code, rec.Body.String())
	}

	// Export with intro music over HTTP; the durable job materializes the
	// uploaded nhac/ bed into the scratch layout for pickMusic.
	res := f.audioCall(t, f.fx.owner, "POST", "/audio/volumes/1/exports", "",
		map[string]any{"format": "mp3", "intro": true})
	if res.Code != http.StatusAccepted {
		t.Fatalf("export enqueue: %d %s", res.Code, res.Body.String())
	}
	runner, err := audio.NewPGExportRunner(ctx, audio.PGDeps{
		Jobs: jobs, Audio: audioRepo, Cache: postgres.NewExportCache(pool), Nar: f.nar, Blobs: blobs,
	})
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	ran, err := runner.RunOnce(ctx)
	if err != nil || !ran {
		t.Fatalf("runner: %v ran=%v", err, ran)
	}
	rows, err := jobs.List(ctx, f.scope)
	if err != nil || len(rows) == 0 {
		t.Fatalf("jobs: %v", err)
	}
	done, err := jobs.Get(ctx, postgres.JobKey{ProjectID: f.scope.ProjectID, ID: rows[len(rows)-1].ID})
	if err != nil || done.State != postgres.JobCompleted {
		errBody, _ := io.ReadAll(bytes.NewReader(done.Error))
		t.Fatalf("export job = %q error=%s", done.State, errBody)
	}

	res = f.audioCall(t, f.fx.owner, "GET", "/audio/volumes/1/exports", "", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("export metadata: %d", res.Code)
	}
	if duration, _ := decodeAny(t, res)["durationSec"].(float64); duration < 0.45 {
		t.Fatalf("export duration must include the 0.2s chapter and 0.3s intro, got %v", duration)
	}

	// Peaks decodes the produced chapter audio blob.
	res = f.audioCall(t, f.fx.owner, "GET", "/audio/volumes/1/chapters/1/peaks", "", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("peaks: %d %s", res.Code, res.Body.String())
	}
	var peaksBody struct {
		Peaks       []float64 `json:"peaks"`
		DurationSec float64   `json:"durationSec"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &peaksBody); err != nil {
		t.Fatalf("peaks decode: %v", err)
	}
	if len(peaksBody.Peaks) == 0 || peaksBody.DurationSec <= 0 {
		t.Fatalf("peaks payload: %s", res.Body.String())
	}

	// The produced mp3 registers under export-file/ so /exports lists it and
	// /exports/file serves the bytes by object key.
	res = f.storyCall(t, f.fx.owner, "GET", "/exports", "", "")
	if res.Code != http.StatusOK {
		t.Fatalf("exports list: %d", res.Code)
	}
	files, _ := decodeAny(t, res)["files"].([]any)
	found := false
	for _, raw := range files {
		file := raw.(map[string]any)
		if strings.HasSuffix(file["name"].(string), ".mp3") {
			found = true
			rec := f.storyCall(t, f.fx.owner, "GET", "/exports/file",
				"path="+url.QueryEscape(file["path"].(string)), "")
			if rec.Code != http.StatusOK {
				t.Fatalf("export mp3 download: %d", rec.Code)
			}
			_, disposition, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition"))
			name := disposition["filename"]
			words := func(value string) []string {
				return strings.FieldsFunc(value, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
			}
			if err != nil || !slices.Equal(words(name), words(file["name"].(string))) {
				t.Fatalf("export mp3 filename metadata = %q, want %q (parse: %v)", name, file["name"], err)
			}
		}
	}
	if !found {
		t.Fatalf("mp3 must appear in /exports: %s", res.Body.String())
	}
}

var _ = store.NewStore // keep import used on older test builds
