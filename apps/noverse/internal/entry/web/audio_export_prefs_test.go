package web

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/datnt1998/noverse/internal/audio"
)

func TestAudioExportPrefsRoundTripAB(t *testing.T) {
	f := newAudioFixture(t)
	base := "/api/projects/proj/stories/01-tra-xanh/audio/export-prefs"
	get := f.tg.do(http.MethodGet, base, f.admin, "")
	if get.Code != http.StatusOK {
		t.Fatalf("GET prefs = %d: %s", get.Code, get.Body)
	}
	var prefs map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &prefs); err != nil {
		t.Fatal(err)
	}
	if prefs["format"] != "mp3" || prefs["gapSec"] != float64(2) || prefs["pack"] != "files" {
		t.Fatalf("defaults: %v", prefs)
	}
	put := f.tg.do(http.MethodPut, base, f.admin, `{"format":"m4b","intro":false,"chapterTitles":true,"loudness":true,"gapSec":3,"chapterMarks":true,"subtitles":false,"pack":"zip"}`)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT prefs = %d: %s", put.Code, put.Body)
	}
	got := f.tg.do(http.MethodGet, base, f.admin, "")
	if got.Body.String() != put.Body.String() {
		t.Fatalf("round trip: %s != %s", got.Body, put.Body)
	}
	disk, err := audio.ReadExportPrefs(f.svc.Layout())
	if err != nil || disk.Format != "m4b" || disk.Subtitles || disk.Pack != "zip" {
		t.Fatalf("persisted prefs: %+v %v", disk, err)
	}
}
