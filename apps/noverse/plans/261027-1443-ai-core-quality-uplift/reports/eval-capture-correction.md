# Eval capture correction — report

## Status

Done. Recording option wired, collector rejects unmeasurable context, quote coverage scoped to committed chapters. `go test ./internal/eval ./internal/store` green; `go build`/`go vet` clean on touched packages. Throwaway smoke programs removed.

## Files changed

- `internal/store/session.go`
  - `SessionStore.keepResults map[string]bool` — names of tools whose result text is logged verbatim.
  - `KeepToolResults(names ...string)` — replaces the set; must be called before workers start (documented). `keepToolResult(name)` lock-guarded lookup.
  - `compactMessage` is now a `SessionStore` method; skips `compactText` for registered tool results. Default (nil set) keeps full compaction — production unchanged, no persistent config.
- `internal/host/options.go` — `newOptions.recordToolResults []string`; `WithToolResultRecording(names ...string)` NewOption.
- `internal/host/host.go` — applies `store.Sessions.KeepToolResults(opts.recordToolResults...)` immediately before `agents.BuildWorkers` (before workers start).
- `internal/eval/runner.go` — both `host.New` calls (RunCase + runFixtureCase) pass `host.WithToolResultRecording("novel_context")`.
- `internal/eval/collect.go`
  - `Collected.ContextMissing int` — novel_context results that are errored/compacted/empty (unmeasurable).
  - `sessionScan` + `scanSessionFile`: `commitPending` (tool_call_id → chapter from args) and `committedChapters`; `noteCommitResult` counts a chapter only on a non-error `commit_chapter` result — prefers `chapter`/`committed` from the result JSON (replay-safe), falls back to call args when the result is compacted; `contextResultMissing` counts each unreadable novel_context result.
  - `novelContextResults` now drops error results and `[session_compact:` blocks, so `context_contains` never matches a placeholder or an error string.
  - `QuoteCoverage` computed only over `ChapterFacts` of chapters committed this run — seeded fixture history and editor-only runs yield `Total=0` (N/A).
- `internal/eval/grade.go` — `Metrics.ContextMissing` (`context_missing,omitempty`); `metricsFrom` copies it.
- `internal/eval/report.go` — `SuiteMetrics.ContextMissing`; aggregated per run; printed in run line (`context_missing=N` when >0) and suite metrics block.
- `internal/eval/assert_context.go` — absent-context hard fail now appends the unmeasurable count (`%d kết quả bị compact/lỗi`).
- `internal/store/session_test.go` — `TestKeepToolResultsRecordsVerbatim` (>4KB novel_context verbatim post-registration; unregistered/default still compacted).
- `internal/eval/collect_test.go` — `writeSessionLines` append helper; `TestCollectContextTrimsAndQuoteCoverage` rewritten (successful commit counts, seeded chapter-2 facts and failed commit excluded); new `TestCollectRecordedNovelContextResult` (>4096B payload via real SessionStore → Collect), `TestCollectSkipsCompactedContextResults` (compacted+error → missing=2, results empty, hard fail).

## Smoke recipe (no paid LLM)

Regrade existing artifacts (run, then delete):

```sh
mkdir -p tmp_evalsmoke && cat > tmp_evalsmoke/main.go <<'EOF'
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/datnt1998/noverse/internal/eval"
)

func main() {
	var dirs []string
	for _, root := range os.Args[1:] {
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err == nil && d.IsDir() && d.Name() == "meta" {
				dirs = append(dirs, filepath.Dir(p))
			}
			return nil
		})
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		col := eval.Collect(dir, nil)
		trims := "N/A"
		if len(col.ContextResults) > 0 {
			trims = fmt.Sprintf("%d/%d", col.ContextTrims, len(col.ContextResults))
		}
		fmt.Printf("%s\n  context results=%d trims=%s missing=%d  quote=%d/%d\n",
			dir, len(col.ContextResults), trims, col.ContextMissing,
			col.QuoteCoverage.Hit, col.QuoteCoverage.Total)
	}
}
EOF
go run ./tmp_evalsmoke workspace/evals/p6-tail-ab workspace/evals/p67-ab
rm -rf tmp_evalsmoke
```

Real ContextTool + real recording path on the seeded wr-thread-overdue fixture:

```sh
mkdir -p tmp_evalsmoke && cat > tmp_evalsmoke/main.go <<'EOF'
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/datnt1998/noverse/internal/eval"
	llmtypes "github.com/datnt1998/noverse/internal/llm"
	"github.com/datnt1998/noverse/internal/store"
	"github.com/datnt1998/noverse/internal/tools"
)

func main() {
	dir := os.Args[1]
	if err := eval.SeedFixture("evals/fixtures", "wr-thread-overdue", dir, "warning"); err != nil {
		panic(err)
	}
	s := store.NewStore(dir)
	s.Sessions.KeepToolResults("novel_context") // what host.WithToolResultRecording sets
	tool := tools.NewContextTool(s, tools.References{}, "default", tools.NewStyleStatsIndex(s))
	raw, err := tool.Execute(context.Background(), json.RawMessage(`{"chapter":3}`))
	if err != nil {
		panic(err)
	}
	s.Sessions.SubAgentLogger(nil)("writer", "Viết chương 3", llmtypes.Message{
		Role:     llmtypes.RoleTool,
		Content:  []llmtypes.ContentBlock{llmtypes.TextBlock(string(raw))},
		Metadata: map[string]any{"tool_name": "novel_context", "tool_call_id": "c1"},
	})
	col := eval.Collect(dir, nil)
	fmt.Printf("payload=%dB loi_hoi_1979=%v quá hạn thu=%v  results=%d missing=%d\n",
		len(raw), strings.Contains(string(raw), "loi_hoi_1979"),
		strings.Contains(string(raw), "quá hạn thu"),
		len(col.ContextResults), col.ContextMissing)
}
EOF
go run ./tmp_evalsmoke /tmp/wr-overdue-smoke
rm -rf tmp_evalsmoke
```

Observed (2026-10-28): payload ≈25.6KB, both `context_contains` needles present; `results=1 missing=0`; session line stored verbatim (no `[session_compact:`). Selected `story_threads` = 5 items — this is the selection budget, not a bug: `loi_hoi_1979` is included with reason "quá hạn thu từ chương 2" (correct overdue selection). No tools bug to report.

## Regrade results on existing artifacts

- All artifact runs have no recoverable context results; each contains one or more compacted `novel_context` results. Context assertions and trims are unmeasurable (N/A), not zero.
- Writer quote coverage recovered from successful commit messages: full-tail experiment baseline 67/67, variant 69/69; memory-prompt experiment baseline 73/73, variant 62/62. Only accepted writer commits from the run count, not seeded history.
- Editor runs (`ed-*`): `quote=0/0` (N/A) — correctly excluded.

## Saved metrics now invalid

- `context_trims` in all p6-tail-ab / p67-ab (and any pre-fix) artifacts: values were computed over compacted placeholders — all-zero, unmeasurable. Treat as N/A; new runs record verbatim.
- `quote_coverage` in all existing artifacts: previously counted seeded fixture chapters (and editor-only runs could inherit facts) — inflated totals. Regrade via the recipe above gives corrected numerator/denominator where commit tool messages were preserved (all wr-* runs).
- `context_contains` assertions on old artifacts: any pass computed against compacted placeholders is suspect (they all fail as "no measurable result" now — accurate, not reconstructable).

## Concerns

- `context_missing` counts errored novel_context results too (not just compacted) — an errored call is equally unmeasurable for assertions; reported separately from `context_trims`.
- Old artifacts' `context_missing` reflects whatever session lines exist; a run with zero novel_context calls reports `missing=0` and empty `ContextResults` — the hard fail still fires on the absent-results branch.
- Quote coverage depends on `commit_chapter` tool results being logged. The artifacts preserve them (sub-4KB results, uncompacted); if a future run produces a >4KB commit result it compacts — `noteCommitResult` then falls back to the call-args chapter via pending correlation.
- Parent owns: prompt-selection/p67 follow-up, final evaluation. Suggest keeping `host.WithToolResultRecording` eval-only (runner.go) — no production call sites exist.
