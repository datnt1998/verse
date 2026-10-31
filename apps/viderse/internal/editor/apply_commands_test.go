package editor

import (
	"context"
	"errors"
	"testing"
)

func cliCommandBatch() []Command {
	return []Command{{Type: "track.add", Params: Object{"id": "v", "name": "Video", "kind": "video"}}, {Type: "track.add", Params: Object{"id": "t", "name": "Chữ", "kind": "video"}}, {Type: "item.add", Params: Object{"item": Object{"id": "image", "type": "image", "trackId": "v", "from": 0, "durationInFrames": 90, "label": "Image"}}}, {Type: "item.add", Params: Object{"item": Object{"id": "caption", "type": "text", "trackId": "t", "from": 0, "durationInFrames": 90, "label": "Caption", "text": "Xin chào", "color": "#ffffff"}}}, {Type: "keyframe.add", Params: Object{"itemId": "image", "property": "opacity", "frame": 10, "value": 0.5, "easing": "linear"}}, {Type: "caption.setWords", Params: Object{"itemId": "caption", "words": []Object{{"text": "Xin", "startFrame": 0, "endFrame": 30}, {"text": "chào", "startFrame": 30, "endFrame": 60}}, "estimated": false}}, {Type: "caption.setPreset", Params: Object{"itemId": "caption", "preset": "word-highlight"}}}
}
func TestRunCommandsCLIAtomicCaptionsKeyframesAndDryRun(t *testing.T) {
	s, m, rev := captionTestService(t)
	commands := cliCommandBatch()
	before := m.writes
	dry, err := s.RunCommands(context.Background(), "project", commands, ApplyCommandsOptions{DryRun: true, BaseRevision: rev})
	if err != nil {
		t.Fatal(err)
	}
	if dry["revision"] != rev || dry["baseRevision"] != rev || m.writes != before {
		t.Fatal("dry run committed")
	}
	result, err := s.RunCommands(context.Background(), "project", commands, ApplyCommandsOptions{BaseRevision: rev})
	if err != nil {
		t.Fatal(err)
	}
	summary := fillObject(result["summary"])
	if fillNum(summary, "commandCount", 0) != float64(len(commands)) {
		t.Fatal(summary)
	}
	p, current, err := s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	if current == rev || result["revision"] != current {
		t.Fatal("revision not committed")
	}
	items := fillObjects(p.Timeline()["items"])
	var caption Object
	for _, i := range items {
		if i["id"] == "caption" {
			caption = i
		}
	}
	if len(fillObjects(caption["words"])) != 2 || caption["captionPreset"] != "word-highlight" || len(fillObjects(p.Timeline()["keyframes"])) != 1 {
		t.Fatal(p)
	}
	if len(result["results"].([]any)) != len(commands) {
		t.Fatal("missing command results")
	}
}
func TestRunCommandsCLIValidationAndUnsupportedSchema(t *testing.T) {
	s, m, rev := captionTestService(t)
	for _, tc := range []struct {
		commands []Command
		index    int
	}{{[]Command{{Type: "project.rename", Params: Object{"name": "Changed"}}, {Type: "missing.command", Params: Object{}}}, 1}, {[]Command{{Type: "caption.setPreset", Params: Object{"itemId": "subtitle", "preset": "bogus"}}}, 0}, {[]Command{{Type: "item.add", Params: nil}}, 0}} {
		before := m.writes
		_, err := s.RunCommands(context.Background(), "project", tc.commands, ApplyCommandsOptions{})
		var e *Error
		if !errors.As(err, &e) || e.Code != "invalid_command" || fillNum(e.Extra, "commandIndex", -1) != float64(tc.index) {
			t.Fatalf("wrong indexed error: %#v", err)
		}
		if m.writes != before {
			t.Fatal("invalid batch persisted")
		}
	}
	if _, err := s.RunCommands(context.Background(), "project", nil, ApplyCommandsOptions{BaseRevision: "stale"}); err == nil {
		t.Fatal("stale revision accepted")
	}
	before := m.writes
	empty, err := s.RunCommands(context.Background(), "project", []Command{}, ApplyCommandsOptions{})
	if err != nil || empty["revision"] != rev || m.writes != before {
		t.Fatal("empty batch must not save", err)
	}
	p, _, err := s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	p["schemaVersion"] = 999
	_, err = s.Store.Write(context.Background(), "project", p, rev)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.RunCommands(context.Background(), "project", nil, ApplyCommandsOptions{})
	var e *Error
	if !errors.As(err, &e) || e.Code != "unsupported_format" {
		t.Fatal(err)
	}
}
func TestApplyCommandsCLIRejectsInvalidWordsAndSourceMetadata(t *testing.T) {
	s, _, _ := captionTestService(t)
	p, _, err := s.Store.Read(context.Background(), "project")
	if err != nil {
		t.Fatal(err)
	}
	for _, words := range [][]Object{{{"text": "bad", "startFrame": 4, "endFrame": 4}}, {{"text": "bad", "startFrame": 0, "endFrame": 151}}, {{"text": "", "startFrame": 0, "endFrame": 1}}} {
		if _, err := ApplyCommands(p, []Command{{Type: "caption.setWords", Params: Object{"itemId": "subtitle", "words": words}}}); err == nil {
			t.Fatalf("accepted bad caption words %#v", words)
		}
	}
}
func TestRunCommandsConcurrentSaveHasOneWinner(t *testing.T) {
	s, _, rev := captionTestService(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, name := range []string{"one", "two"} {
		go func(name string) {
			<-start
			_, err := s.RunCommands(context.Background(), "project", []Command{{Type: "project.rename", Params: Object{"name": name}}}, ApplyCommandsOptions{BaseRevision: rev})
			results <- err
		}(name)
	}
	close(start)
	a, b := <-results, <-results
	if (a == nil) == (b == nil) {
		t.Fatalf("want one revision conflict: %v, %v", a, b)
	}
}

func TestParseCommandsCLIJSONBoundary(t *testing.T) {
	for _, value := range []any{nil, Object{}, 1, []any{Object{"params": Object{}}}, []any{Object{"type": "project.rename"}}, []any{Object{"type": "project.rename", "params": "name"}}} {
		if _, err := ParseCommands(value); err == nil {
			t.Fatalf("malformed CLI input accepted: %#v", value)
		}
	}
	commands, err := ParseCommands([]any{Object{"type": "project.rename", "params": Object{"name": "Valid"}}})
	if err != nil || len(commands) != 1 || commands[0].Type != "project.rename" {
		t.Fatal(commands, err)
	}
}

func TestRunCommandsRejectsMalformedCaptionArrayBeforeSaving(t *testing.T) {
	s, _, revision := captionTestService(t)
	for _, words := range []any{Object{}, []any{1}, nil} {
		if _, err := s.RunCommands(context.Background(), "project", []Command{{Type: "caption.setWords", Params: Object{"itemId": "subtitle", "words": words}}}, ApplyCommandsOptions{}); err == nil {
			t.Fatal("malformed caption array accepted", words)
		}
	}
	_, current, err := s.Store.Read(context.Background(), "project")
	if err != nil || current != revision {
		t.Fatal("invalid captions committed", err)
	}
}
