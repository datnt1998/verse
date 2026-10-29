# AI core map: `/Users/dat/Projects/ainovel-cli` (Go, module `github.com/datnt1998/noverse`)

## 0. Scope and method

- Read-only, using `grep`, `glob`, and `read`. The `find` tool failed with judge errors, so it was not used.
- Spilled tool outputs are cited as `artifact://N`, which is an investigation artifact rather than a repo file. Where a claim rests on a spilled region, the report says so.
- Line numbers are given where they were captured. A few items cite only a file or function because the captured output had no numbers. Those are marked `[no-line]`.
- Output file: the assignment's `plans/reports/` path could not be written (read-only), so the report is returned here.

## 1. Summary

- **Pipeline**: a Host-level deterministic router (`internal/flow/router.go`, pure `Route(State)`) dispatches four LLM workers (architect_long, architect_short, writer, editor). An LLM **arbiter** (`internal/arbiter`) handles the semantic decisions the router cannot make: plan-start selection, failure triage, and user-intervention routing.
- **Chapter loop** (writer tools): `plan_chapter` → `draft_chapter` (write|append) → `check_consistency` (read-only) → `commit_chapter` (saga). Review is by the editor agent, with a 7-dimension structural review, and results feed a `PendingRewrites` queue.
- **Prompts**: 14 Vietnamese prompt markdowns in `assets/prompts/`, embedded via `go:embed`, plus style packs (`assets/packs`), style files (`assets/styles`), and references (`assets/references`). Assembly is in `assets/load.go` and `assets/pack.go`.
- **Memory**: no embeddings or vector store in the Go code. Memory is JSON files in the store (`summaries/`, arc and volume summaries), assembled per turn by the `novel_context` tool. Token-window management is a separate engine (`internal/context`).
- **QC**: editor review, arbiter, advance gate, mechanical rule checks (`internal/rules`), diagnostics (`internal/diag`), and `commit_chapter` validation.
- **LLM layer**: `internal/llm` on top of an external `litellm` package. Providers are `anthropic`, `bedrock`, `compat`, `gemini`, `ollama`, `openai`, and `openrouter`. Per-role retries are provided by `llmretry`, and structured output by `llmcontract`.
- **Evals**: three small case directories (`smoke`, `editor-goi-a`, `nao-w0`). The eval framework does not yet assert verdicts. `nao-w0` is a standalone Python retrieval experiment.
- **Documentation mismatch**: `docs/context-management.md` references `internal/orchestrator` and `../agentcore`, which were not found in the `internal/` listing. Treat that doc as stale.

## 2. Files and architecture

### 2.1 Router, state, and agent build

- `internal/flow/router.go`
  - Doc comment: "Route 是纯函数" (pure function), with the priority ladder 1–11 listed in the `Route` doc comment `[no-line]`. Order: Complete → planning gap-fill → `PendingRewrites` → Reviewing (dormant) → Steering → aggregate refresh (arc review, arc summary, volume summary, global review) → immediate external-revision feedback → layered arc-end post-processing → non-layered global review (`domain.ShouldReview`) → non-layered outline exhausted → normal writing.
  - Writer rewrite task appends a `plan_chapter` instruction when `RewriteHeadNeedsDirective` is set `[no-line]`.
  - `Instruction` struct (Agent, Task, Reason, Chapter) `[no-line]`.
- `internal/flow/state.go` `[no-line]`: `State` struct carries `Progress`, `LastCompleted`, `ArcBoundary`, `HasArcReview`, `HasArcSummary`, `HasVolumeSummary`, `FoundationMissing`, `PlanningTier`, `HasGlobalReview`, `ImmediateFeedbackCount`, `AggregateRefresh`, `RewriteHeadNeedsDirective`.
- `internal/agents/build.go`
  - `BuildWorkers` `[no-line, ~112–422]`: builds the four workers.
  - `cacheBase := promptCacheBase(store.Dir())` and comment on cache keys: lines 245–248.
  - Per-worker `CacheLastMessage: "ephemeral"` and `PromptCacheKey`: architect_short 269–271, architect_long 288–289, writer 309–310, editor 355–356.
  - Writer stop guard `guard.NewWriterStopGuard`: line 311–312. Editor stop guard `NewEditorStopGuard`: line 357+.
  - Thinking level: `ApplyThinking`, `ParseThinkingLevel`, `ResolveThinkingForModel`, `UsageRecorder` in the 112–422 range `[no-line]`.
- `internal/agents/context_manager.go`
  - `contextManagerConfig` (ContextWindow, ReserveTokens, Summary, …): lines 12–21.
  - `newContextManager(cfg) *corecontext.ContextEngine` builds the engine: line 23.
  - Rewrite event logged via slog only: 64–67.

### 2.2 Arbiter (`internal/arbiter`)

- `arbiter.go`: `decide` generic over `llmcontract.Execute`; `decideMaxTokens = 8192` (~30–33); `DispatchOp` (~60–64); worker enum `architect_long|architect_short|writer|editor` (~68); dispatch schema (~85–88).
- `plan_start.go`: `PlanStartDecision` (~14–18); `Validate` (~21–24); `DecidePlanStart` (~54).
- `failure.go`: `FailureFacts` (~17–20); `FailureDecision` actions `retry|reroute|abort` (~32–35); `ValidateAgainst` (~38); `DecideFailure` (~72).
- `intervention.go`: `InterventionFacts` (~19–22); `RecentDecision` (~50); `QueueHead` (~57); `CollectInterventionFacts` (~66); `AdvanceHoldOp` (~142); `ReopenOp` (~150); `AskOptionOp` (~156); `AskOp` (~164). Decision order answer → rules → hold → reopen → ask → dispatch, with at most one dispatch (~172). `ValidateAgainst` (~213); `AskAutoPick` (~312); `DecideIntervention` (~347); `maxAskAutoPicks` constant.
- Prompts: `assets/prompts/arbiter-plan-start.md` (planner choice ~3–6), `arbiter-failure.md` (worker_failure, deadlock sections ~7–18), `arbiter-intervention.md` (authorization ~5–7, ask-user routing ~13–16).

### 2.3 Chapter tools (`internal/tools`)

- `plan_chapter.go`: `PlanChapterTool` (~16), `Execute` (~54).
- `draft_chapter.go`: `DraftChapterTool` (~17); mode `write|append` (~27–38); `StrictSchema` (~47); `Execute` (~49).
- `check_consistency.go`: `CheckConsistencyTool` (~16); read-only (~30–32); must run after `draft_chapter` (~26).
- `commit_chapter.go`: `CommitChapterTool` (~24); `NewCommitChapterTool` requires `StyleStatsIndex` (~32–35); saga comment (~73); `Execute` (~85); `ReconcileLayeredCompletion` (~1048). Not read-only, not concurrency-safe. Updates timeline, foreshadow, relationships, state, and progress. Returns next_chapter, review_required, arc_end, volume_end, needs_expansion, book_complete, flow.
- `chapter_guard.go`: `EnsureChapterExpanded` (~13), for layered books inside the expanded outline. This is not a word-count check.

### 2.4 Host (`internal/host`)

- `advance_gate.go`: `ChapterAdvanceGate` (~17–20); `NewChapterAdvanceGate` (~23); `HandleBoundary` (~29; with auto and no hold, reads RunMeta once); `handleHold` (~59); `targetChapterCommitted` (~104; checks PendingCommit).
- `budget.go`: `BudgetSentinel` struct (~29–32); `OnCost` (~70); `HandleEvent` (~96; stops only on `subagent` tool-end boundaries); `HandleBoundary` (~106); `stop` (~114); `Refuse` (~122); `Limit` (~133). States normal → warned → stopPending → stopped. Stop is deferred to a subagent boundary unless `hardStop`. Blind-spot detection: 5 consecutive zero-cost increments trigger a warning.
  - Observation (not verified by build): the import block shown for `budget.go` lists `internal/agent` but not `agentcore`, while the code uses `agentcore.Event`. Needs a `go build` check by the main agent.
- `host.go`: `retryGate *llmretry.Gate` (98–99); gate created at 199 and shared with imports and agent models.
- `imp/call.go`: `retryGate` (56); `RequestRetry` hook shows a progress message (173–177). `imp/runner.go`: `RetryGate` (137–138).
- `import_preview_test.go` / `session_retry_test.go`: `RetryNow` wakes backoff (test 54–58).

### 2.5 Store (`internal/store`)

- File list `[no-line]`: session.go, world.go, user_rules.go, usage.go, summaries.go, simulation.go, signals.go, runtime.go, run_meta.go, rewrites.go, revisions.go, progress.go, outline.go, nao_vault.go, nao_relationships.go, nao_identity.go, nao_address.go, drafts.go, comment_review.go, checkpoints.go, characters.go, chapter_records.go, cast.go, book.go, store.go.
- `summaries.go`: `SummaryStore` (12); `SaveSummary` → `summaries/NN.json` (24–25); `LoadRecentSummaries(current, count)` (69); arc summaries `summaries/arc-vNNaNN.json` (85–86, 108–110); volume summaries `summaries/vol-vNN.json` (136–137); `FindCharacterAppearances` (169).

### 2.6 Context and memory

- `internal/agent/context.go`: `ContextManager` interface (85–102): `Project` (prompt view), `RecoverOverflow`, usage reporting. Optional interfaces (129–150): converter, estimator, window provider, window setter.
- `internal/agent/loop.go`: projection stage (611–613); `recoverOverflow` calls `ContextManager.RecoverOverflow` (554–575).
- `internal/context/` (the strategy engine, referenced by `agents/context_manager.go`):
  - `NewLightTrim(KeepRecent, TextThreshold, PreserveHead, PreserveTail)` (`compaction_regression_test.go` 24, 111). Keeps system and first message.
  - `NewFullSummary(FullSummaryConfig{Model, KeepRecentTokens})` (test 37, 74, 141). Commit strategy name `full_summary` (37). Checkpoint invalidates old usage (test 196). Oversized rewritten payload rejected as overflow (test 149–152).
  - `EngineConfig{ContextWindow, ReserveTokens, Strategies, CommitStrategies}` (test 24, 37).
- `internal/tools/novel_context.go` (per-turn memory assembly):
  - Chapter > 0 → `working_memory` (159); planning → `planning_memory` (161).
  - `episodic_memory` carries a usage note: "nhắc lại nguyên văn … là lỗi lặp" (146–147).
  - Containers read: `working_memory`, `episodic_memory`, `planning_memory`, `foundation_memory`, `reference_pack`, `selected_memory` (263–268, 674–680).
  - `memory_policy` (357). `buildUserRules` (164).
- Retention: `internal/tools/retention.go` (RetentionPack 20; `validateRetention` 44; `normalizeStoryRetention` 136; `validateNewHookTypes` 177).
- No embedding or vector-store code found. Grep for `embedding|vector|bm25|rrf` in `internal/store`, `internal/tools`, `internal/host`, `internal/agents`, `internal/domain` returned no matches `[grep-verified]`. The `internal/` and `cmd/` grep returned unrelated matches not inspected individually.

### 2.7 LLM layer

- `internal/llm/` (package `llm`): `client.go` `NewModel(provider, model, opts…)` (123); `NewClientModel` (50); `newProvider(name, cfg)` (962) registers providers by name; comment "registered in litellm (builtin or via litellm.RegisterProvider)" (122). `capabilities.go`: `Capabilities` (25–35), `ThinkingCapabilities` (37), `ToolCapabilities` (55), `StructuredCapabilities` with JSONObject/JSONSchema/Strict (66–69), `StreamingCapabilities` (73), `UsageCapabilities` (82). `contracts.go` `SwappableModel` (205). `history.go`, `message.go`, `model.go` (`ModelPricing` comment at 3).
- Providers under `internal/llm/provider/`:
  - `anthropic/` (`Config` 22; `New` 43; `Chat` 80; `Stream` 122; `stream.go` `newStream` 65 with bufio scanner).
  - `openai/` (`Config` 26; `Chat` 91 branches to Responses API when `APIResponses` at 92–93; `Stream` 135; `url` 241 with Volcengine note 243–244).
  - `compat/` (spec-driven OpenAI-compatible base: `Provider` 18; `Capabilities` 59; `Chat` 73; `Stream` 101; `ListModels` 114).
  - `gemini/` (`Config` 18; `Chat` 66; `Stream` 108).
  - `bedrock/` (`Config` 19; `Chat` 104; `Stream` 147; `ListModels` 177).
  - `ollama/`, `openrouter/` (files exist; contents not read).
- Retries:
  - `internal/llmretry/model.go`: `DefaultMaxRetries = 7` (14–17). Retries only pre-stream failures; mid-stream failures go to agentcore's `MaxRetries` (15–16). `RetryingModel` (19–20). `NewRetryingModel` (31–35).
  - `internal/bootstrap/models.go`: `SetRetryGate` (143, 147); wraps models with `NewRetryingModel(m, gate, role, DefaultMaxRetries)` (186).
  - Shared `Gate` lets `Host.RetryNow` wake waiters (host.go 98–99, 199).
- Structured output: `internal/llmcontract/execute.go`
  - `Request[T]` hooks `RequestRetry`, `Correction` (61–62); `RetryGate` (76).
  - `promptCorrection` text (79): Vietnamese message asking the model to fix schema mismatch.
  - `Execute[T any]` (85): loops with correction feedback "until success or context ends" (84 comment). The loop has no fixed attempt cap in this snippet `[no-line on cap]`.
  - Calls `llmretry.Generate` with `Gate` (111–114).
- Streaming: anthropic `stream.go` (65). Tool calls: `llmtypes` (not read).
- Prompt caching:
  - `internal/agent/loop.go`: system floor via `systemPrefixMessages` (645), single cache breakpoint on last non-system message (649–657), `CacheControl` in `LLMPrefix` (717–723), metadata `cache_control` (735, 745).
  - `docs/prompt-cache-design.md` describes OpenAI `prompt_cache_key` routing vs Claude `cache_control` rolling breakpoints. Design text read as `artifact://12` `[no-line]`.

### 2.8 Assets and prompts

- `assets/load.go`: `go:embed` for prompts (15–16), references (18–19), styles (21–22), `voice.md` (24–25). `Load(style, opts)` (81); `BuildWriterPrompt` (107; replaces voice placeholder, appends style); `OverrideVoice` (116); `resolveAppendable` (120; user global style override); `ParseVoice` (158); `AssembleVoice` (180); `loadReferences` (208); `loadPrompts` (247); `WithSimulationGuidance` (272); `OverridePrompt` (277, role-core only).
- `assets/pack.go`: `go:embed packs` (15–16); `Pack.Section` (44); `ReadFile` (63); `loadPack` (109); `parsePack` (147; requires `goi.json`); `applyPackVoice` (188); `RulesSection` (213); `AppendPackRules` (231); `ListPacks` (243). Source order: embedded → `~/.ainovel/style/packs` → `<book>/style/packs`.
- `assets/prompts/` (14 files, `[glob-verified]`): architect-long.md, architect-short.md, arbiter-intervention.md, arbiter-plan-start.md, arbiter-failure.md, editor.md, writer.md, simulation-source.md, simulation-merge.md, revision-analyze.md, import-synthesize.md, import-segment.md, import-range.md, import-analyze.md.
  - architect-long.md: tools (3–9), hard constraints (11–13), 14 required premise headings (32–54), layered outline and story compass (21–105).
  - architect-short.md: scope (21–30), flat outline (58–73), characters (74–86), world rules (87–90).
  - editor.md: tools (3–9), user-intervention boundary (11–20), review method (21–31), 7-dimension structural review (32–75), `user_rules` (76–83), `[LỖI]`/`[CẢNH BÁO]` pack labels (84–87), sub-code `s19-recap` (88–90).
  - writer.md (46–50): `structured` (forbidden_chars, forbidden_phrases, fatigue_words) are mechanically enforced at commit; `preferences` are natural-language; user preference beats project default.
  - import-analyze.md: input (3–11), `hook_type`/`dominant_strand` value constraints (12–18), discipline (19–21).
- `assets/styles/` (`[glob-verified]`): default.md, wuxia.md, suspense.md, romance.md, fantasy.md.
- `assets/references/` (`[glob-verified]`): 14 files including `anti-ai-tone.md`, `quality-checklist.md`, `hook-techniques.md`, `dialogue-writing.md`, `consistency.md`, `chapter-guide.md`, plus `genres/{wuxia,suspense,romance,fantasy}/{style-references,arc-templates}.md`.
- `assets/packs/` contents `[no-line]`: embedded via `pack.go` 15–16; not enumerated.

### 2.9 Rules and diagnostics

- `internal/rules/types.go`: `Structured` fields (`ForbiddenChars` 129, `ForbiddenPhrases` 130, `FatigueWords` 133, `Measurements` 135); severity mapping comment (149–151): forbidden_chars → Error, forbidden_phrases → Error, fatigue_words → Warning; `Violation` struct (164–171).
- `internal/rules/checker.go`: `appendForbiddenPhrases` (54–68, `Rule: "forbidden_phrases"`, Error); `appendFatigueWords` (74–96, density-based: occurrences > `per_1000 × syllables/1000`, minimum 1 allowed; not cross-chapter).
- `internal/rules/snapshot.go`: merge rule comments (72–78: genre high-priority overrides; forbidden lists union; fatigue per-word, nearest layer wins; preferences concatenated); `mergeStructured` (152–154); `SystemDefaults` (222–248) with four built-in forbidden phrases, sourced from `assets/references/anti-ai-tone.md` (244–246).
- `snapshot_test.go` asserts defaults: 4 forbidden phrases (96) and 16 fatigue words (98).
- `internal/domain/chapter.go:35`: `WordCount` (whitespace tokens with at least one letter or digit; punctuation-only excluded). Used as the "syllable" count.
- `internal/diag/rules_quality.go`: `ChronicLowDimension` (11), `ContractMissPattern` (52), `HookWeakChain` (87), `PayoffMissPattern` (133), `ExcessiveRewrites` (180), `WordCountAnomaly` (213; requires ≥3 chapter counts).

### 2.10 Import and user rules (docs)

- `docs/import-pipeline.md` (status line 3: implemented v1 in `internal/host/imp`). Problems identified (2.1–2.4, lines 42–66): chapter-title regex is open-ended; a single LLM foundation call grows with chapter count; partial writes before failure; hardcoded semantics. Target and non-goals (68–90). Responsibilities table (94). Workspace manifest (162). Stage derived from artifacts, not persisted enum (190–206). Source decoding and normalization (234–256).
- `docs/user-rules-runtime.md` (status line 21–23: implemented 2026-07-19). Single source of truth: one `meta/user_rules.json` snapshot injected via `novel_context` (3–5, 52). Inputs: four sources (94). Rules file is natural-language markdown, not YAML (103–110). Normalization is a separate schema-constrained LLM call per source (131–133). Conservative promotion to `structured` (165). Failure degrades, never blocks writing (184–186). System defaults are built-in (198). Merge order (208). Two entry points (229). Echo to user (253). Interventions split into three classes (287).

### 2.11 Voice layer (docs)

- `docs/voice-layer.md` (read as `artifact://14` region 4): voice is an embedded `voice.md` (`assets/load.go` 24–25) split into keyed sections via `ParseVoice` (158) and reassembled with overrides via `AssembleVoice` (180); `BuildWriterPrompt` (107) injects the placeholder. Packs can override voice keys (`applyPackVoice`, `pack.go` 188).

### 2.12 Advance gate and boundaries

- Advance gate pauses at chapter boundaries per AdvanceHold modes. Not a route branch; `HandleBoundary` runs before dispatch. Spec: `docs/chapter-advance-gate.md` (read in part; sections not line-mapped).

### 2.13 Evals

- `evals/cases/smoke/`: `writer_first_chapter.json`, `architect_short.json`, `architect_long.json` (`[glob-verified]`).
- `evals/cases/editor-goi-a/`: `dialogue-warning.json`, `safe-ending.json` (`[glob-verified]`).
  - `dialogue-warning.json` (`role: editor`, `gate.max_severity: warning`, `expect: {}`). Its description states the framework cannot yet assert verdict/issue or load a finished chapter for the editor. The expected outcome is accept with a warning, `requires_change=false`, rule `Master §13` (dialogue share 40–55%).
- `evals/nao-w0/` (`[glob-verified]`: README.md, run.py, questions.jsonl, results.md). README: measures whether local embeddings beat lexical search on 8 real Vietnamese chapters; about 150 chunks; 7 retrievers (`bm25`, `bm25_fold`, `bm25_graph`, `vec_bge_m3`, `vec_qwen3`, two hybrids); 48 questions in 3 types of 16 each; metrics recall@1, recall@5, MRR@10; uses Ollama. Status: experiment, decision pending on "W4" (not implemented in Go code).

## 3. Pipeline (premise → chapter → commit)

1. **Start**: `Arbiter.DecidePlanStart` chooses `architect_long` or `architect_short` (`arbiter/plan_start.go` 54). The planner calls `save_book`, `save_foundation`, and emits foundation files (`architect-long.md`, `architect-short.md`).
2. **Foundation gap-fill**: `Route` rule 2 re-dispatches the same planner with `FoundationMissing` until complete (`router.go`). Empty `PlanningTier` falls back to Engine `planStartFallback` (per router comment).
3. **Outline**: layered (volume → arc → chapter) or flat. `revise_outline`, `resolve_outline_feedback`, `save_foundation` types (`append_volume`, `expand_arc`, `complete_book`) per router rules 8, 12.
4. **Chapter**: writer runs `plan_chapter` (contract) → `draft_chapter` (write|append) → `check_consistency` → `commit_chapter`. Writer stop guard enforces completion (`build.go` 311).
5. **Review**: editor runs `novel_context` + `read_chapter`, then `save_review` with scope `arc|global|chapter`. Arc-end triggers arc review → arc summary → volume summary → expand or new volume (router rule 8).
6. **Rewrites**: `PendingRewrites` queue; writer rewrites or polishes head chapter, with `plan_chapter` required when no contract exists (router rule 3).
7. **Gates**: advance gate at boundaries; budget sentinel at subagent boundaries; arbiter for failures and interventions.

## 4. Prompt system

- Core prompts: `assets/prompts/*.md` (14 files, §2.8).
- Assembly: `assets/load.go` `Load` → `BuildWriterPrompt` (voice placeholder + style), `loadPrompts`, `AppendPackRules` (pack `[LỖI]/[CẢNH BÁO]` rules).
- Overrides: `OverridePrompt` (role-core only), `OverrideVoice` (A/B tests), user global style at `~/.ainovel/style` and per-book `<book>/style` (`pack.go` source order).
- Style packs: `packs/<name>/goi.json` with sections `fatigue_words`, `thoai`, `do_do`, `audio`, `outline`, `voice`, `luat` (`pack.go` 44, 147, 188, 213).
- Caching: §2.7.

## 5. Memory and context

- **Store**: JSON files under the book directory, per §2.5.
- **Per-turn**: `novel_context` builds containers (§2.6). Writer sees working memory and user rules; episodic memory is facts already written, flagged against repetition.
- **Window management**: `internal/context` engine with LightTrim and FullSummary strategies; overflow recovery via `ContextManager.RecoverOverflow`.
- **Retrieval**: keyword and entity-based (summaries, `FindCharacterAppearances`). No embeddings in the Go code; embedding comparison exists only as `evals/nao-w0` experiment.

## 6. Quality control

- **Editor**: 7 dimensions scored 0–100, pass/warning/fail derived from score (`editor.md` 32–75). Genre-pack labels `[LỖI]`/`[CẢNH BÁO]` (84–87).
- **Mechanical checks**: `internal/rules/checker.go` (forbidden phrases error; fatigue density warning) applied at commit.
- **Consistency**: `check_consistency` tool (read-only), runs before commit.
- **Word count**: `domain.WordCount` (token-based); anomaly diag `WordCountAnomaly` (`rules_quality.go` 213). No hard minimum length gate found in the code read; target length not verified `[UNVERIFIED]`.
- **Anti-AI tone**: `assets/references/anti-ai-tone.md`; built-in forbidden phrases (`snapshot.go` 244–246); fatigue-word table (16 defaults per test); genre-pack overrides.
- **Diagnostics**: §2.9 rules; `ExcessiveRewrites` (180) for rewrite count.
- **Arbiter**: failure triage with retry/reroute/abort (§2.2).
- **Budget**: `BudgetSentinel` (§2.4).
- **Evals**: §2.13; editor eval cannot yet assert verdicts.

## 7. LLM layer

- Providers: anthropic, openai (chat + Responses), compat (OpenAI-compatible spec), gemini, bedrock, ollama, openrouter. Routing by provider name string in `newProvider` (`llm/client.go` 962).
- Per-role model assignment: `bootstrap/models.go` `ModelSet` (`Default` + per-role); `SwappableModel` for hot-swap (`llm/contracts.go` 205).
- Retries: pre-stream, 7 attempts, wakeable gate (`llmretry/model.go` 14–35). Contract correction loop (`llmcontract/execute.go` 79–111).
- Structured output: JSON Schema per contract, `StrictSchema` for tools (`tools/draft_chapter.go` 47), capability flags (`llm/capabilities.go`).
- Tool calls and streaming: per provider (`anthropic/stream.go` 65).
- Prompt caching: §2.7.

## 8. Import, user rules, voice (one paragraph each)

- **Import**: `internal/host/imp` implements the semantic compile pipeline described in `docs/import-pipeline.md`. Source is decoded and normalized into stable coordinates (234–256), then analyzed in phases with artifact-derived state (190–206). Each phase is an LLM call with schema constraints (`assets/prompts/import-*.md`). Per the doc, the old one-shot approach is replaced because of format fragility and partial-write failures (42–66). Retry progress is shown via `RequestRetry` (`imp/call.go` 174–177).
- **User rules**: a single `meta/user_rules.json` snapshot built from four sources (start prompt, rules file, interventions, system defaults). Each source is normalized by a separate schema-constrained LLM call (`docs/user-rules-runtime.md` 131–133). Merge is priority-layered (208; code `rules/snapshot.go` 72–78). Failure degrades, never blocks (184–186). Enforced mechanically via `rules/checker.go`; preferences go to the writer prompt (`writer.md` 47–49).
- **Voice**: an embedded `voice.md` split into keyed sections (`assets/load.go` 158, 180). Packs may override sections. `BuildWriterPrompt` (107) inserts the voice into the writer prompt; `docs/voice-layer.md` describes the design.

## 9. Gaps and TODOs

- `docs/design/v1/` gap tables (mostly Vietnamese, UI and web backend):
  - `01-review.md` 19–20: `/import`, `/cocreate`, `/sync`, `/diag`, `/simulate` API backlog; auto-export gap.
  - `06-data-contract.md` ~139–160: GAP table.
  - `08-plan.md` ~59: post-v1 backlog (cocreate, import, sync, diag, simulate, runtime model switch, light theme).
  - `09-auth-onboard.md` 177–186: GAP-13…16.
- `evals/cases/editor-goi-a/dialogue-warning.json` description: framework lacks verdict/issue assertions and cannot load a finished chapter for editor evals.
- `evals/nao-w0`: embedding/hybrid retrieval decision (W4) pending.
- `internal/flow/router.go` rule 4: Reviewing branch is dormant (no producer sets `FlowReviewing`).
- `docs/context-management.md`: references `internal/orchestrator` and `../agentcore`, not found in `internal/` listing `[glob-verified]`. Doc likely stale.
- `internal/host/budget.go`: possible missing `agentcore` import `[UNVERIFIED — needs go build]`.
- No explicit engine TODO/FIXME list found in `docs/` grep (artifact://14 region 13).

## 10. Not verified / limits

- `litellm` package location and provider registration internals (outside `internal/llm`).
- `internal/llm/provider/ollama` and `openrouter` contents.
- `internal/context` strategy source beyond tests.
- `docs/chapter-advance-gate.md` section content, `docs/prompt-cache-design.md` details, `docs/evaluation-system.md` (not found in `docs/` `[glob-verified]`).
- `internal/llmcontract/execute.go` retry cap for correction loop.
- Word-count target or minimum length enforcement.
- Eval results (`nao-w0/results.md` not read).
- No builds or tests were run, per instructions.

Status: partial. The core map (pipeline, prompts, memory, QC, LLM layer, import/rules/voice, gaps) is complete with the citations above. Items in §10 are not verified and are marked as such.
