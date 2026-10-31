package scenes

import (
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"strings"
)

type Edit struct {
	Op      string
	SceneID string
	Frame   int
	Text    string
	RightID string
}

func ApplyEdits(m Manifest, edits []Edit) (Manifest, error) {
	// Copy the scene slice so a rejected batch never changes its caller.
	m.Scenes = append([]Scene(nil), m.Scenes...)
	for _, e := range edits {
		idx := -1
		for i, s := range m.Scenes {
			if s.SceneID == e.SceneID {
				idx = i
				break
			}
		}
		if idx < 0 {
			return m, steps.CloneError(domain.CloneErrNotFound, "Không tìm thấy cảnh", "")
		}
		scene := m.Scenes[idx]
		switch e.Op {
		case "text":
			m.Scenes[idx].Text = strings.TrimSpace(e.Text)
			m.Scenes[idx].TextSource = "edited"
		case "boundary":
			if idx+1 >= len(m.Scenes) || e.Frame <= scene.StartFrame || e.Frame >= m.Scenes[idx+1].EndFrame {
				return m, fmt.Errorf("invalid boundary")
			}
			m.Scenes[idx].EndFrame = e.Frame
			m.Scenes[idx+1].StartFrame = e.Frame
			for _, i := range []int{idx, idx + 1} {
				s := &m.Scenes[i]
				s.KeyframeStale = s.KeyframeFrame < s.StartFrame || s.KeyframeFrame >= s.EndFrame
			}
		case "split":
			if e.Frame <= scene.StartFrame || e.Frame >= scene.EndFrame || e.RightID == "" {
				return m, fmt.Errorf("invalid split")
			}
			for _, s := range m.Scenes {
				if s.SceneID == e.RightID {
					return m, fmt.Errorf("duplicate scene id")
				}
			}
			left, right := scene, scene
			left.EndFrame = e.Frame
			right.StartFrame = e.Frame
			right.SceneID = e.RightID
			left.KeyframeStale = left.KeyframeFrame < left.StartFrame || left.KeyframeFrame >= left.EndFrame
			right.KeyframeStale = right.KeyframeFrame < right.StartFrame || right.KeyframeFrame >= right.EndFrame
			m.Scenes = append(m.Scenes[:idx], append([]Scene{left, right}, m.Scenes[idx+1:]...)...)
		case "merge":
			if idx+1 >= len(m.Scenes) {
				return m, fmt.Errorf("no right neighbour")
			}
			right := m.Scenes[idx+1]
			scene.EndFrame = right.EndFrame
			scene.Text = strings.TrimSpace(scene.Text + " " + right.Text)
			scene.TextSource = "edited"
			scene.KeyframeStale = scene.KeyframeFrame < scene.StartFrame || scene.KeyframeFrame >= scene.EndFrame
			m.Scenes = append(m.Scenes[:idx], append([]Scene{scene}, m.Scenes[idx+2:]...)...)
		default:
			return m, fmt.Errorf("unknown scene edit %q", e.Op)
		}
	}
	m.Revision++
	m.Pending = nil
	m.Approval = nil
	return m, nil
}
