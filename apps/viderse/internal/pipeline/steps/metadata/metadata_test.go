package metadata

import (
	"reflect"
	"strings"
	"testing"
)

// Port metadata.test.ts: source language/keywords and injection boundary.
func TestMetadataPromptPreservesSourceAndBoundsText(t *testing.T) {
	p := BuildMetadataPrompt(PromptInput{Title: "A proven title", Description: "a description", Tags: []string{"keyword", "search"}, Transcript: strings.Repeat("x", TranscriptLimit+100), ChannelName: "Owner", StyleNote: "cinematic"})
	for _, s := range []string{"Title: A proven title", "Description: a description", "Tags: keyword, search", "Owner channel: Owner", "Channel style note: cinematic", "UNTRUSTED", "Transcript excerpt:", "--- END COMPETITOR VIDEO ---"} {
		if !strings.Contains(p, s) {
			t.Errorf("missing %q", s)
		}
	}
	if strings.Contains(p, strings.Repeat("x", TranscriptLimit+1)) {
		t.Fatal("unbounded transcript")
	}
}
func TestSanitizeTagsDropsEveryCloneSubstring(t *testing.T) {
	got := SanitizeTags([]string{"  Good ", "CLONE", "MyCloneTag", "clonefish", "good", "Other", ""})
	if !reflect.DeepEqual(got, []string{"Good", "Other"}) {
		t.Fatal(got)
	}
}
func TestMetadataLimitsMatchYouTube(t *testing.T) {
	for _, o := range []Output{{Title: "", Description: "d"}, {Title: "t", Description: ""}, {Title: strings.Repeat("t", 101), Description: "d"}, {Title: "t", Description: strings.Repeat("d", 5001)}, {Title: "t", Description: "d", Tags: []string{strings.Repeat("x", 500), "x"}}} {
		if validate(&o) == nil {
			t.Fatalf("accepted invalid metadata: %+v", o)
		}
	}
	o := Output{Title: "t", Description: "d", Tags: []string{strings.Repeat("x", 500)}}
	if err := validate(&o); err != nil {
		t.Fatal(err)
	}
}
