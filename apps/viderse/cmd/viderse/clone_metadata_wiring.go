package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	stepmetadata "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/metadata"
)

type metadataMedia struct {
	edits    *jobedit.Service
	snapshot jobedit.Snapshot
}

func (m metadataMedia) Acquire(ctx context.Context, path string) (*media.Handle, error) {
	snap := m.snapshot
	step, _, _ := strings.Cut(path, "/")
	for _, record := range snap.Steps {
		if string(record.Step) == step {
			snap.Step = record
			break
		}
	}
	return m.edits.Acquire(ctx, snap, path)
}
func (m metadataMedia) Publish(context.Context, []media.Artifact, func(context.Context) error) error {
	return errors.New("metadata rewrite cannot publish directly")
}
func metadataRewrite(edits *jobedit.Service, handler func(context.Context, *pipeline.StepContext, []string, string) (pipeline.StepResult, error)) func(context.Context, jobedit.Snapshot, []string, string) (clonemetadata.Content, error) {
	return func(ctx context.Context, snap jobedit.Snapshot, parts []string, direction string) (clonemetadata.Content, error) {
		dir, err := os.MkdirTemp("", "viderse-metadata-rewrite-")
		if err != nil {
			return clonemetadata.Content{}, err
		}
		defer os.RemoveAll(dir)
		step := &pipeline.StepContext{Job: snap.Job, Step: snap.Step, StagingDir: dir, Media: metadataMedia{edits, snap}}
		_, err = handler(ctx, step, parts, direction)
		if err != nil {
			return clonemetadata.Content{}, err
		}
		raw, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
		if err != nil {
			return clonemetadata.Content{}, err
		}
		var out stepmetadata.Output
		if err = json.Unmarshal(raw, &out); err != nil {
			return clonemetadata.Content{}, err
		}
		return clonemetadata.Content{Title: out.Title, Description: out.Description, Tags: out.Tags}, nil
	}
}
