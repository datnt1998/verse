package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

type sourceRivalReader interface {
	SourceRival(context.Context, string) (*domain.CloneSourceSnapshot, error)
}

// enrichSource reads only the selected job's published early-step evidence.
func (s *Service) enrichSource(ctx context.Context, job *domain.CloneJob, steps []domain.CloneStepRecord) error {
	if reader, ok := s.Store.(sourceRivalReader); ok {
		rival, err := reader.SourceRival(ctx, job.Source.URL)
		if err != nil {
			return err
		}
		job.Source.Outlier, job.Source.ViewCount, job.Source.PublishedAt = nil, nil, nil
		if rival != nil {
			job.Source.Outlier, job.Source.ViewCount, job.Source.PublishedAt = rival.Outlier, rival.ViewCount, rival.PublishedAt
		}
	}
	for _, step := range steps {
		if step.Status != domain.CloneSucceeded || step.Output == nil {
			continue
		}
		if step.Step == domain.CloneDownload {
			if caption, ok := step.Output.Data["caption"]; ok {
				present := caption != nil
				job.Source.HasCaptions = &present
				if details, ok := caption.(map[string]any); ok {
					if language, ok := details["language"].(string); ok && language != "" {
						job.Source.CaptionLanguage = &language
					}
				}
			}
		}
		if s.Media == nil {
			continue
		}
		for _, artifact := range step.Output.Artifacts {
			name := ""
			if step.Step == domain.CloneDownload && (artifact.Path == "download/source.info.json" || strings.HasSuffix(artifact.Path, "/download/source.info.json")) {
				name = "info"
			}
			if step.Step == domain.CloneTranscript && (artifact.Path == "transcript/captions.json" || strings.HasSuffix(artifact.Path, "/transcript/captions.json")) {
				name = "captions"
			}
			if name == "" {
				continue
			}
			handle, err := s.Media.Acquire(ctx, artifact.Path)
			if err != nil {
				return err
			}
			file, err := os.Open(handle.Path)
			if err != nil {
				handle.Release()
				return err
			}
			var raw map[string]json.RawMessage
			err = json.NewDecoder(file).Decode(&raw)
			closeErr := file.Close()
			handle.Release()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			if name == "info" {
				if chapters, ok := raw["chapters"]; ok && string(chapters) != "null" {
					var entries []json.RawMessage
					if err := json.Unmarshal(chapters, &entries); err != nil {
						return err
					}
					n := len(entries)
					job.Source.ChapterCount = &n
				}
			} else {
				var cues []struct { Text string `json:"text"` }
				if err := json.Unmarshal(raw["cues"], &cues); err != nil {
					return err
				}
				var parts []string
				for _, cue := range cues {
					if text := strings.TrimSpace(cue.Text); text != "" {
						parts = append(parts, text)
						if len([]rune(strings.Join(parts, " "))) >= 320 {
							break
						}
					}
				}
				text := strings.Join(parts, " ")
				if chars := []rune(text); len(chars) > 320 {
					text = string(chars[:320])
				}
				if text != "" {
					job.Source.TranscriptExcerpt = &text
				}
			}
		}
	}
	return nil
}
