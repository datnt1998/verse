package editor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/transcript"
	"github.com/datnt1998/verse/platform/blobstore"
)

type captionTestMedia struct {
	mu     sync.Mutex
	dir    string
	paths  map[string]string
	writes int
}

func (m *captionTestMedia) Acquire(_ context.Context, path string) (*media.Handle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	file, ok := m.paths[path]
	if !ok {
		return nil, blobstore.ErrNotFound
	}
	return &media.Handle{Path: file}, nil
}
func (m *captionTestMedia) Publish(ctx context.Context, artifacts []media.Artifact, commit func(context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if commit != nil {
		if err := commit(ctx); err != nil {
			return err
		}
	}
	for _, a := range artifacts {
		b, err := os.ReadFile(a.File)
		if err != nil {
			return err
		}
		f := filepath.Join(m.dir, ID())
		if err = os.WriteFile(f, b, 0600); err != nil {
			return err
		}
		m.paths[a.Path] = f
		m.writes++
	}
	return nil
}
func (m *captionTestMedia) Unbind(ctx context.Context, paths []string, commit func(context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if commit != nil {
		if err := commit(ctx); err != nil {
			return err
		}
	}
	for _, p := range paths {
		delete(m.paths, p)
	}
	return nil
}
func captionTestService(t *testing.T) (Service, *captionTestMedia, string) {
	t.Helper()
	m := &captionTestMedia{dir: t.TempDir(), paths: map[string]string{}}
	repo := &commandTestRepository{values: map[string]Resource{}, media: m}
	s := Service{Media: m, Repository: repo, StagingDir: t.TempDir(), Now: func() time.Time { return time.Unix(100, 0) }}
	s.Store = Store{Media: m, Repository: repo, StagingDir: s.StagingDir, Now: s.Now}
	p, err := New("project", "Captions", 1920, 1080, 30, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	p["timeline"] = Object{
		"tracks": []Object{{"id": "subs", "kind": "video"}},
		"items": []Object{{
			"id": "subtitle", "type": "subtitle", "trackId": "subs", "label": "Captions",
			"from": 0, "durationInFrames": 150, "wordsEstimated": true,
			"cues":  []Object{{"id": "cue", "text": "Việt 2026 works", "startSeconds": 1, "endSeconds": 4}},
			"words": []Object{},
		}},
	}
	rev, err := s.Store.Create(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	s.Anchors = func(context.Context, string) (Object, error) {
		return Object{
			"version": 1, "sourceAudioSha256": strings.Repeat("a", 64),
			"spans": []Object{{"startMs": 0, "endMs": 5000}},
			"words": []Object{
				{"text": "viet", "startMs": 1200, "endMs": 1400},
				{"text": "2026", "startMs": 1500, "endMs": 1700},
				{"text": "works", "startMs": 1800, "endMs": 2000},
			},
			"model":       Object{"name": "whisper", "version": "1", "size": "small", "computeType": "int8", "device": "cpu"},
			"generatedAt": "now",
		}, nil
	}
	return s, m, rev
}
func TestCaptionAlignPersistsExactWordsInOneRevision(t *testing.T) {
	s, m, rev := captionTestService(t)
	before := m.writes
	result, err := s.AlignCaptions(context.Background(), "project", AlignCaptionsInput{CueIDs: []string{"cue"}, Revision: rev})
	if err != nil {
		t.Fatal(err)
	}
	if m.writes != before+1 {
		t.Fatalf("want one undo-worthy write, got %d", m.writes-before)
	}
	if result["revision"] == rev {
		t.Fatal("revision did not change")
	}
	cues := fillObjects(result["cues"])
	if len(cues) != 1 || fillNum(cues[0], "offsetSec", 0) != 0.12 {
		t.Fatalf("wrong alignment: %#v", result)
	}
	words := fillObjects(cues[0]["words"])
	if len(words) != 3 || fillNum(words[0], "startSec", 0) != 1.2 {
		t.Fatal(words)
	}
	p, stored, err := s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	if stored != result["revision"] {
		t.Fatal("returned revision is not persisted")
	}
	item := fillObjects(p.Timeline()["items"])[0]
	w := fillObjects(item["words"])
	if len(w) != 3 || fillNum(w[0], "startFrame", 0) != 36 || fillBool(w[0], "estimated") {
		t.Fatal(w)
	}
}
func TestCaptionAlignValidatesSelectionAndRevision(t *testing.T) {
	s, m, rev := captionTestService(t)
	for _, selection := range []any{nil, "all", []any{1}, []string{"missing"}} {
		before := m.writes
		if _, err := s.AlignCaptions(context.Background(), "project", AlignCaptionsInput{CueIDs: selection, Revision: rev}); err == nil {
			t.Fatalf("accepted invalid selection %#v", selection)
		}
		if m.writes != before {
			t.Fatal("invalid selection wrote project")
		}
	}
	_, err := s.AlignCaptions(context.Background(), "project", AlignCaptionsInput{CueIDs: "all-drifting", Revision: "sha256:stale"})
	if err == nil {
		t.Fatal("stale revision accepted")
	}
}
func TestCaptionAlignKeepsMeasuredAnchorsAndRejectsUnavailableEvidence(t *testing.T) {
	s, m, rev := captionTestService(t)
	p, _, err := s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	item := fillObjects(p.Timeline()["items"])[0]
	item["wordsEstimated"] = false
	item["words"] = []Object{{"text": "Việt 2026 works", "startFrame": 30, "endFrame": 120, "estimated": false}}
	rev, err = s.Store.Write(context.Background(), "project", p, rev)
	if err != nil {
		t.Fatal(err)
	}
	before := m.writes
	if _, err = s.AlignCaptions(context.Background(), "project", AlignCaptionsInput{CueIDs: "all-drifting", Revision: rev}); err != nil {
		t.Fatal(err)
	}
	if m.writes != before {
		t.Fatal("measured anchors were rewritten")
	}
	s.Anchors = func(context.Context, string) (Object, error) { return nil, errors.New("ASR unavailable") }
	if _, err = s.AlignCaptions(context.Background(), "project", AlignCaptionsInput{CueIDs: []string{"cue"}}); err == nil {
		t.Fatal("missing evidence was silently interpolated")
	}
}
func TestCaptionAlignRejectsMalformedAnchorBoundary(t *testing.T) {
	s, _, _ := captionTestService(t)
	s.Anchors = func(context.Context, string) (Object, error) {
		return Object{"words": []Object{{"text": "viet", "startMs": -1, "endMs": 2}}}, nil
	}
	if _, err := s.AlignCaptions(context.Background(), "project", AlignCaptionsInput{CueIDs: "all-drifting"}); err == nil {
		t.Fatal("invalid anchors accepted")
	}
}
func captionTestJSON(t *testing.T, v any) Object {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var o Object
	if err = json.Unmarshal(b, &o); err != nil {
		t.Fatal(err)
	}
	return o
}

// Ported caption-align.test.ts cases exercise the editor's current timeline,
// rather than measuring the transcript's pre-editor timing.
func TestCaptionAlignUpstreamMatchingCases(t *testing.T) {
	anchors := func(text string, start float64) []Object {
		out := []Object{}
		for i, word := range strings.Fields(text) {
			onset := start + float64(i)*300
			out = append(out, Object{"text": word, "startMs": onset, "endMs": onset + 200})
		}
		return out
	}
	for _, tc := range []struct {
		name, text string
		start, end float64
		anchors    []Object
		index      int
		onset      float64
		estimated  bool
		none       bool
	}{
		{name: "accent numeric punctuation", text: "Việt! 2026, works", start: 1, end: 4, anchors: anchors("viet 2026 works", 1200), index: 0, onset: 1200},
		{name: "inserted dropped tokens", text: "one missing two three", start: 1, end: 4, anchors: anchors("one extra two three", 1200), index: 2, onset: 1800},
		{name: "unmatched stays estimated", text: "one missing two three", start: 1, end: 4, anchors: anchors("one extra two three", 1200), index: 1, estimated: true},
		{name: "near substitution threshold", text: "words", start: 1, end: 4, anchors: anchors("word", 1200), onset: 1200},
		{name: "UTF16 non-BMP substitution threshold", text: "𐐀abc", start: 1, end: 4, anchors: anchors("𐐀abd", 1200), onset: 1200},
		{name: "unrelated substitution", text: "cat", start: 1, end: 4, anchors: anchors("dog", 1200), none: true},
		{name: "repeated nearest phrase", text: "hello there", start: 2, end: 2.7, anchors: append(anchors("hello there", 800), anchors("hello there", 2100)...), onset: 2100},
		{name: "empty anchors", text: "nothing measured", start: 1, end: 4, anchors: []Object{}, none: true},
		{name: "outside candidate band", text: "one two", start: 1, end: 4, anchors: anchors("one two", 8000), none: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, rev := captionTestService(t)
			p, _, err := s.Store.Read(context.Background(), "project")
			if err != nil {
				t.Fatal(err)
			}
			item := fillObjects(p.Timeline()["items"])[0]
			cue := fillObjects(item["cues"])[0]
			cue["text"], cue["startSeconds"], cue["endSeconds"] = tc.text, tc.start, tc.end
			rev, err = s.Store.Write(context.Background(), "project", p, rev)
			if err != nil {
				t.Fatal(err)
			}
			original := s.Anchors
			s.Anchors = func(ctx context.Context, id string) (Object, error) {
				value, err := original(ctx, id)
				if err != nil {
					return nil, err
				}
				value["words"] = tc.anchors
				return value, nil
			}
			result, err := s.AlignCaptions(context.Background(), "project", AlignCaptionsInput{CueIDs: "all-drifting", Revision: rev})
			if err != nil {
				t.Fatal(err)
			}
			if tc.none {
				if len(fillObjects(result["cues"])) != 0 || result["revision"] != rev {
					t.Fatal("unmeasured timing was fabricated", result)
				}
				return
			}
			p, _, err = s.Store.Read(context.Background(), "project")
			if err != nil {
				t.Fatal(err)
			}
			words := fillObjects(fillObjects(p.Timeline()["items"])[0]["words"])
			if len(words) <= tc.index {
				t.Fatal(words)
			}
			word := words[tc.index]
			if fillBool(word, "estimated") != tc.estimated {
				t.Fatal(word)
			}
			if !tc.estimated && fillNum(word, "startFrame", -1) != fillRound(tc.onset*0.03) {
				t.Fatal(word)
			}
		})
	}
}

func TestCaptionAlignOnlySelectedCueAndKeepsNextBoundary(t *testing.T) {
	s, _, rev := captionTestService(t)
	p, _, err := s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	item := fillObjects(p.Timeline()["items"])[0]
	item["cues"] = []Object{{"id": "cue", "text": "Việt 2026 works", "startSeconds": 1, "endSeconds": 4}, {"id": "next", "text": "next cue", "startSeconds": 1.75, "endSeconds": 4}}
	rev, err = s.Store.Write(context.Background(), "project", p, rev)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.AlignCaptions(context.Background(), "project", AlignCaptionsInput{CueIDs: []string{"cue"}, Revision: rev})
	if err != nil {
		t.Fatal(err)
	}
	if len(fillObjects(result["cues"])) != 1 {
		t.Fatal(result)
	}
	p, _, err = s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	cues := fillObjects(fillObjects(p.Timeline()["items"])[0]["cues"])
	if fillNum(cues[0], "endSeconds", 0) != 1.75 || fillNum(cues[1], "startSeconds", 0) != 1.75 || cues[1]["text"] != "next cue" {
		t.Fatal(cues)
	}
}

func TestCaptionAlignLoadsRealPhase05ArtifactNamespace(t *testing.T) {
	s, m, _ := captionTestService(t)
	anchors, err := s.Anchors(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	put := func(path string, value Object) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(m.dir, ID())
		if err = os.WriteFile(file, raw, 0600); err != nil {
			t.Fatal(err)
		}
		m.paths[path] = file
	}
	pid := "clone-job-uuid-g2"
	prefix := "clone/job-uuid/" + strings.Repeat("a", 64)
	put(prefix+"/build/checkpoint.json", Object{"projectId": pid, "generation": 2})
	put(prefix+"/transcript/asr-anchors.json", anchors)
	loaded, err := s.LoadCaptionAnchors(context.Background(), pid)
	if err != nil {
		t.Fatal(err)
	}
	if !commandJSONEqual(loaded, captionTestJSON(t, anchors)) {
		t.Fatal(loaded)
	}
	if _, err = s.LoadCaptionAnchors(context.Background(), "clone-job-uuid-g1"); err == nil {
		t.Fatal("different build generation accepted")
	}
	put("clone/job-uuid/"+strings.Repeat("b", 64)+"/transcript/asr-anchors.json", anchors)
	if _, err = s.LoadCaptionAnchors(context.Background(), pid); err == nil {
		t.Fatal("ambiguous ASR generation silently selected")
	}
}

func TestCaptionAlignRebasesEarlyAnchorsWithoutMovingMeasuredCue(t *testing.T) {
	s, _, revision := captionTestService(t)
	project, _, err := s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	item := fillObjects(project.Timeline()["items"])[0]
	item["from"] = 90
	item["cues"] = []Object{
		{"id": "cue", "text": "Việt 2026 works", "startSeconds": 0, "endSeconds": 1},
		{"id": "measured", "text": "independent", "startSeconds": 1.2, "endSeconds": 2},
	}
	item["words"] = []Object{{"text": "independent", "startFrame": 36, "endFrame": 45, "estimated": false}}
	revision, err = s.Store.Write(context.Background(), "project", project, revision)
	if err != nil {
		t.Fatal(err)
	}
	original := s.Anchors
	s.Anchors = func(ctx context.Context, id string) (Object, error) {
		value, err := original(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, word := range fillObjects(value["words"]) {
			word["startMs"] = fillNum(word, "startMs", 0) + 1300
			word["endMs"] = fillNum(word, "endMs", 0) + 1300
		}
		return value, nil
	}
	if _, err = s.AlignCaptions(context.Background(), "project", AlignCaptionsInput{CueIDs: []string{"cue"}, Revision: revision}); err != nil {
		t.Fatal(err)
	}
	project, _, err = s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	item = fillObjects(project.Timeline()["items"])[0]
	origin := fillNum(item, "from", 0)
	if origin >= 90 {
		t.Fatal("early onset was clamped instead of rebased")
	}
	found := false
	for _, word := range fillObjects(item["words"]) {
		switch word["text"] {
		case "Việt":
			if origin+fillNum(word, "startFrame", 0) != 75 {
				t.Fatal("source onset was changed", word)
			}
		case "independent":
			found = true
			if origin+fillNum(word, "startFrame", 0) != 126 || fillBool(word, "estimated") {
				t.Fatal("measured word moved", word)
			}
		}
	}
	if !found {
		t.Fatal("unselected measured cue was removed")
	}
}

func TestCaptionAlignUnmatchedWordNeverEnablesKaraoke(t *testing.T) {
	s, _, revision := captionTestService(t)
	project, _, err := s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	item := fillObjects(project.Timeline()["items"])[0]
	fillObjects(item["cues"])[0]["text"] = "one two nonsense three four"
	revision, err = s.Store.Write(context.Background(), "project", project, revision)
	if err != nil {
		t.Fatal(err)
	}
	original := s.Anchors
	var evidence Object
	s.Anchors = func(ctx context.Context, id string) (Object, error) {
		value, err := original(ctx, id)
		if err != nil {
			return nil, err
		}
		words := []Object{}
		for i, text := range []string{"one", "two", "three", "four"} {
			start := 1200 + i*300
			words = append(words, Object{"text": text, "startMs": start, "endMs": start + 200})
		}
		value["words"] = words
		evidence = value
		return value, nil
	}
	if _, err = s.AlignCaptions(context.Background(), "project", AlignCaptionsInput{CueIDs: "all-drifting", Revision: revision}); err != nil {
		t.Fatal(err)
	}
	project, _, err = s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	item = fillObjects(project.Timeline()["items"])[0]
	storedCue := fillObjects(item["cues"])[0]
	cue := FillCue{ID: "cue", Text: fillString(storedCue, "text"), StartSec: fillNum(storedCue, "startSeconds", 0), EndSec: fillNum(storedCue, "endSeconds", 0)}
	for _, word := range fillObjects(item["words"]) {
		cue.Words = append(cue.Words, FillCueWord{Text: fillString(word, "text"), StartMs: fillNum(word, "startFrame", 0) * 1000 / 30, EndMs: fillNum(word, "endFrame", 0) * 1000 / 30, Estimated: fillBool(word, "estimated")})
	}
	prepared := PrepareCaptionData([]FillCue{cue}, 30, 150)
	pages := []transcript.DisplayedCaptionPage{}
	for _, page := range prepared.Pages {
		if EffectiveCaptionPreset("karaoke", false, page.Estimated) != "fade" {
			t.Fatal("estimated page enabled karaoke")
		}
		displayed := transcript.DisplayedCaptionPage{ItemID: "subtitle", Estimated: page.Estimated}
		for _, word := range page.Words {
			displayed.Words = append(displayed.Words, transcript.PageWord{Text: word.Text, StartMs: float64(word.StartFrame) * 1000 / 30, EndMs: float64(word.EndFrame) * 1000 / 30, Frame: int64(word.StartFrame)})
		}
		pages = append(pages, displayed)
	}
	anchors, err := parseCaptionAnchors(evidence)
	if err != nil {
		t.Fatal(err)
	}
	metric := transcript.MeasureCaptionAlignment(pages, anchors)
	if metric.MatchedRatio < 0.6 || metric.KaraokeRatio != 0 {
		t.Fatal(metric)
	}
}

func TestCaptionAlignGlobalDegradationDoesNotOverwriteMeasuredWords(t *testing.T) {
	s, m, revision := captionTestService(t)
	project, _, err := s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	item := fillObjects(project.Timeline()["items"])[0]
	// Phase05 omits estimated:false on measured words, while the item-level
	// flag can still be true because a different source piece was dropped.
	item["wordsEstimated"] = true
	item["words"] = []Object{{"text": "Việt 2026 works", "startFrame": 30, "endFrame": 120}}
	revision, err = s.Store.Write(context.Background(), "project", project, revision)
	if err != nil {
		t.Fatal(err)
	}
	before := m.writes
	result, err := s.AlignCaptions(context.Background(), "project", AlignCaptionsInput{CueIDs: "all-drifting", Revision: revision})
	if err != nil {
		t.Fatal(err)
	}
	if m.writes != before || result["revision"] != revision || len(fillObjects(result["cues"])) != 0 {
		t.Fatal("measured source timing was overwritten", result)
	}
}
