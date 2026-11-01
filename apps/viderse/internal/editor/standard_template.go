package editor

import (
	_ "embed"
	"fmt"
	"time"
)

// standardLayout is the schema-v15 result of yt-factory 2c201e6's
// buildStandardTemplateProject and its command engine. The bundled source
// assets are immutable, including the voice envelope; generation is recorded
// by testdata/generate-standard.mjs (output dir: assets). It lives outside
// testdata because release builds exclude testdata. Keeping the engine-authored
// payload avoids a second implementation of Subscribe's keyframes or track
// defaults.
//
//go:embed assets/standard-project.json
var standardLayout []byte

// BuildStandardTemplateProject creates a fresh Mẫu chuẩn with the exact
// canonical tracks, transitions, Subscribe animation, captions, voice waveform
// and looping snow. Media ids are content-stable shared bundled assets.
func BuildStandardTemplateProject(id, name string, now time.Time) (Project, error) {
	base, err := New(id, name, 1920, 1080, 30, now)
	if err != nil {
		return nil, err
	}
	project, err := Parse(standardLayout)
	if err != nil {
		return nil, fmt.Errorf("decode bundled standard layout: %w", err)
	}
	project["id"] = base["id"]
	project["name"] = base["name"]
	project["createdAt"] = base["createdAt"]
	project["updatedAt"] = base["updatedAt"]
	return project, nil
}
