package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

// stepMedia resolves recipe-relative paths exclusively through this generation's
// outputs. Intermediate publications share the step's fence and checkpoint tx.
type stepMedia struct {
	base     *media.Service
	jobID    string
	upstream map[domain.CloneStep]domain.CloneStepOutput
	fence    func(context.Context) error
	record   func(context.Context, []domain.CloneArtifact) error
	step     domain.CloneStep
	mu       sync.Mutex
	paths    map[string]string
}

func (m *stepMedia) Acquire(ctx context.Context, path string) (*media.Handle, error) {
	m.mu.Lock()
	logical := m.paths[path]
	m.mu.Unlock()
	if logical != "" {
		return m.base.Acquire(ctx, logical)
	}
	for _, output := range m.upstream {
		for _, artifact := range output.Artifacts {
			if artifact.Path == path || strings.HasSuffix(artifact.Path, "/"+path) {
				if logical != "" && logical != artifact.Path {
					return nil, fmt.Errorf("ambiguous artifact: %s", path)
				}
				logical = artifact.Path
			}
		}
	}
	if logical != "" {
		return m.base.Acquire(ctx, logical)
	}
	// Shared library assets are already logical media paths, not clone-relative.
	if strings.HasPrefix(path, "clone/") {
		if !strings.HasPrefix(path, "clone/"+m.jobID+"/") {
			return nil, ErrNotFound
		}
		return m.base.Acquire(ctx, path)
	}
	if strings.HasPrefix(path, "scenes/") || strings.HasPrefix(path, "images/") || strings.HasPrefix(path, "download/") || strings.HasPrefix(path, "transcript/") || strings.HasPrefix(path, "build/") || strings.HasPrefix(path, "backdrop/") {
		return nil, ErrNotFound
	}
	return m.base.Acquire(ctx, path)
}
func (m *stepMedia) Publish(ctx context.Context, artifacts []media.Artifact, commit func(context.Context) error) error {
	published := make([]media.Artifact, 0, len(artifacts))
	paths := make(map[string]string, len(artifacts))
	entries := make([]domain.CloneArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		if err := media.ValidatePath(artifact.Path); err != nil {
			return err
		}
		file, err := os.Open(artifact.File)
		if err != nil {
			return err
		}
		hash := sha256.New()
		size, err := io.Copy(hash, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		digest := hex.EncodeToString(hash.Sum(nil))
		rel := artifact.Path
		artifact.Path = "clone/" + m.jobID + "/" + digest + "/" + rel
		paths[rel] = artifact.Path
		published = append(published, artifact)
		entries = append(entries, domain.CloneArtifact{ID: InputFingerprint(artifact.Path), Path: artifact.Path, SHA256: digest, Bytes: size, Step: m.step, ContentType: artifact.ContentType, Filename: artifact.Filename})
	}
	err := m.base.Publish(ctx, published, func(txctx context.Context) error {
		if err := m.fence(txctx); err != nil {
			return err
		}
		if err := m.record(txctx, entries); err != nil {
			return err
		}
		if commit != nil {
			return commit(txctx)
		}
		return nil
	})
	if err != nil {
		return err
	}
	m.mu.Lock()
	for rel, path := range paths {
		m.paths[rel] = path
	}
	m.mu.Unlock()
	return nil
}
