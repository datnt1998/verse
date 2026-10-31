package sim

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

type LLMChat interface {
	Generate(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error)
}

type Deps struct {
	Store   *store.Store
	LLM     LLMChat
	Prompts Prompts
}

type Prompts struct {
	Source string
	Merge  string
}

type Options struct {
	SourceDir   string
	LoadSources func(context.Context) ([]Source, error)
}

// Source is one backend-neutral sample; bytes retain their original encoding
// so fingerprints and decoding match the filesystem scanner.
type Source struct {
	Name      string
	Data      []byte
	UpdatedAt time.Time
}

type Stage string

const (
	StageScan    Stage = "scan"
	StageAnalyze Stage = "analyze"
	StageMerge   Stage = "merge"
	StageImport  Stage = "import"
	StageDone    Stage = "done"
	StageError   Stage = "error"
)

type Event struct {
	Time    time.Time
	Stage   Stage
	Current int
	Total   int
	Message string
	Err     error
}

type ImportResult struct {
	ImportedSources int
	SkippedSources  int
	ProfilePath     string
}
