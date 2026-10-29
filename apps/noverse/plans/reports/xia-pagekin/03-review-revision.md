## Slice: Quality control — review, revision, structured repair, and quality programs

- Scope: read-only source inspection; no code, prompts, builds, tests, installs, or model calls were executed.
- Source paths below are absolute under `/tmp/xia-ainw`; our paths are relative to `/Users/dat/Projects/ainovel-cli`.
- Important distinction: implemented mechanisms, documented experimental results, and future qualification requirements are identified separately.
- Main conclusion: the strongest transferable ideas are **per-goal evidence accounting**, **separating issue authorization from factual authority**, **verification of actual repair rather than textual change**, and **independent review-quality calibration**.
- Our targeted editing and deterministic workflow already cover much of the underlying architecture; Pagekin should not be treated as categorically more advanced.
- License boundary: recommendations describe ideas independently; no source code or verbatim prompt text is proposed for reuse.

### Mechanisms

#### 1. Objective review rather than an all-purpose literary score

- The production reviewer concentrates on six areas: plot continuity, causal/motivational breaks, character state, links to legitimate prior material, explicit chapter requirements, and unnecessary complete retellings.
- It explicitly excludes pure prose/style preferences and optional creative suggestions from defect reporting.
- Step 1: supply the full chapter, character states, admitted history, worldbuilding, and optional author review focus.
- Step 2: require evidence for a concrete problem rather than filling a category quota.
- Step 3: permit omission of problem-free dimensions; retain one pass item when nothing specific is wrong.
- Step 4: return severity only after the excerpt and explanation, encouraging an evidence-first classification.
- Quality rationale: reduces cosmetic churn and false-positive repair requests that can damage otherwise acceptable writing.
- Evidence: `/tmp/xia-ainw/src/services/builtin-prompt-templates.ts:924-979`; `/tmp/xia-ainw/src/services/workflows/commands/review-chapter.command.ts:67-89`.
- Ours: broader seven-dimensional literary review already exists, including consistency, character, pacing, continuity, foreshadowing, hook, and aesthetics: `assets/prompts/editor.md:21-74`.
- Recommendation: retain our richer literary reviewer, but distinguish **objective blocking defects** from **taste/craft suggestions**, rather than replacing its rubric.

#### 2. A small, strict review envelope, with a separate goal channel

- The general response is an object containing `summary`, `items`, and optionally `goalReviews`.
- General items have `category`, `description`, severity `error|warning|pass`, and an excerpt required except for pass.
- The parser rejects unexpected root/item fields and requires 1–10 ordinary items.
- The prompt asks for a short summary, short explanations, and short excerpts to prevent sprawling reports.
- Goal results are explicitly outside the ordinary item-count limit, so ten ordinary findings cannot crowd chapter requirements out.
- A later deterministic projection converts normalized goal results into the existing report presentation.
- Evidence: `/tmp/xia-ainw/src/shared/review-generation-report.ts:16-107,109-138`; `/tmp/xia-ainw/src/services/builtin-prompt-templates.ts:970-979`.
- Caveat: current v2 parsing does **not** actually enforce the documented character limits; `reportText` only truncates v1 historical reports.
- That implementation discrepancy is visible at `/tmp/xia-ainw/src/shared/review-generation-report.ts:39-42,93-105`.
- Ours already has a strict tool schema and business validation, but reports aggregate contract status/misses rather than a separately validated per-goal channel: `internal/tools/save_review.go:76-105,257-354`.

#### 3. Software freezes the checklist before asking the reviewer

- The application derives current-chapter goals from blueprint `keyEvents`, splitting only explicit newline/semicolon boundaries.
- It retains each goal’s original wording and assigns a deterministic chapter/source/index identifier.
- It additionally recognizes explicit chapter-specific “must appear” author annotations in world settings and character notes.
- It distinguishes configured goals from unknown/unavailable goal sources and deliberately unconfigured goals.
- The model supplies judgments keyed by these identifiers; it cannot author the authoritative checklist.
- Quality rationale: prevents an apparently successful reviewer from silently omitting, merging, or weakening an inconvenient requirement.
- Evidence: `/tmp/xia-ainw/src/shared/chapter-goal-review.ts:18-39`; `/tmp/xia-ainw/electron/services/review-revision-context.ts:145-152`.
- Ours already exposes a chapter contract and asks the editor to inspect required beats, forbidden moves, continuity checks, emotional targets, payoffs, and hooks: `assets/prompts/editor.md:23-27`.
- Missing equivalent: immutable per-requirement IDs plus deterministic exact judgment coverage; our saved report has only `ContractStatus`, `ContractMisses`, and `ContractNotes`: `internal/domain/review.go:112-126`.

#### 4. Three-valued goal review preserves epistemic uncertainty

- Each goal receives `completed`, `unmet`, or `unknown`; coverage is a separate field.
- The review instructions distinguish actions due now, background constraints, future plans, recaps, and maintenance of an existing state.
- Explicit postponement/refusal/opposite outcomes support unmet; mere silence supports unknown, not proof that an event did not happen.
- Completed requires evidence for **every** due sub-action; a completed majority cannot hide one unresolved component.
- A must-appear instruction needs positive, locatable presentation, not merely lack of contradiction.
- Conflicts between a goal and author-confirmed facts remain unknown rather than allowing either source to be silently rewritten.
- Evidence: `/tmp/xia-ainw/src/shared/chapter-goal-review.ts:42-69`.
- Quality rationale: avoids two common failure modes—declaring absent evidence an error, and counting promises/plans as completed action.
- Ours has aggregate `met|partial|missed` contract status, but no explicit per-goal unknown/evidence model: `internal/tools/save_review.go:100-102`; `internal/domain/review.go:118-120`.
- Port implication: add evidence-aware acceptance items to our existing contract/report types, not another planning database.

#### 5. Mechanical coverage/evidence normalization cannot itself prove semantics

- The normalizer walks the frozen goals, not the model’s response order.
- Missing judgments, duplicate IDs, unknown IDs, unexpected fields, invalid states, empty explanations, or invalid excerpts degrade coverage/judgment to unknown.
- Completed and unmet require at least one excerpt; unknown may legitimately have no excerpt.
- Excerpts are located against the exact frozen manuscript, and offsets are stored with the actual source text.
- Only an extra enclosing quotation pair is tolerated; there is no fuzzy paraphrase, stitched quote, or punctuation normalization.
- Evidence: `/tmp/xia-ainw/src/shared/chapter-goal-review.ts:77-125`.
- Important implementation caveat: directly matching excerpts use the first occurrence; uniqueness is checked only in the enclosing-quotation fallback.
- Thus the prompt’s universal unique-excerpt rule is stronger than this normalizer’s actual enforcement.
- Evidence for the discrepancy: `/tmp/xia-ainw/src/shared/chapter-goal-review.ts:78-90`; `/tmp/xia-ainw/src/services/workflows/commands/review-chapter.command.ts:79-81`.
- Ours checks nonempty issue evidence and some package-rule citation requirements, but those checks do not generally locate every quote in prose: `internal/tools/save_review.go:289-311,395-442`.

#### 6. Review facts are assembled with explicit authority and time labels

- Preparation transactionally loads the selected source draft and verifies its ID/version/status/content hash against the request.
- Historical finalized prose is bound to finalization identity/content, and derived projections are used only when their source identity remains current.
- If the chapter was written from a selected saved predecessor, review uses that predecessor rather than substituting a newer row.
- Character states included as author authority are restricted to author-provenance fields and labeled with their chapter/time.
- Current blueprint instructions and future plans are rendered separately; plans are explicitly not established history.
- AI-selected review notes are rendered separately from author-written guidance.
- Evidence: `/tmp/xia-ainw/electron/services/review-revision-context.ts:53-59,91-150`; `/tmp/xia-ainw/src/shared/review-revision-generation.ts:81-125`.
- Quality rationale: prevents stale summaries, future outlines, and AI suggestions from acquiring the same authority as actual prior prose or author settings.
- Ours already separates working, episodic, and reference memory in editor instructions: `assets/prompts/editor.md:5,23-27`.
- Learn the stronger **same subject/time/conditions** comparison discipline, rather than assuming any apparent state difference is a contradiction.

#### 7. Required review material is not silently dropped to fit context

- Review/revision share one admitted-material selection path.
- Author material is required; the immediate prior chapter is required among historical materials.
- Current finalized continuity facts are accompanied by chapter provenance and evidence.
- Optional history receives an 8,000-character local allowance and relevance terms from title/events/characters.
- If required material cannot fit, the command fails explicitly rather than reviewing a deceptively incomplete packet.
- Evidence: `/tmp/xia-ainw/src/services/workflows/commands/review-revision-materials.ts:6-36`; `/tmp/xia-ainw/src/services/workflows/commands/review-chapter.command.ts:50-65`.
- Quality rationale: a review based on omitted required context should not look equivalent to a complete review.
- Our editor already requires original prose reads and contract/context loading: `assets/prompts/editor.md:23-30`.
- This slice does not establish equivalence of our lower-level context admission; classify the specific all-required-material-or-fail behavior as PARTIAL pending the memory slice.

#### 8. Human confirmation selects work, not replacement facts

- The original AI report remains unchanged; author confirmation is another persisted immutable snapshot pointing to it.
- Each item records origin `ai|author` and decision `apply|ignore|waive`.
- Revision refuses to call the model without a valid saved confirmation tied to the exact source draft.
- The revision brief includes only applied items and explicit author guidance.
- It excludes ignored items, the raw report, and the AI-generated summary.
- Selection of an AI finding authorizes addressing that issue; it does not endorse its factual premise or proposed replacement.
- Evidence: `/tmp/xia-ainw/src/shared/human-confirmed-review.ts:6-44,219-283`; `/tmp/xia-ainw/src/services/workflows/commands/refine-from-review.command.ts:55-76`.
- Database preparation also checks stored/requested snapshots, original-report lineage, and source-draft equality: `/tmp/xia-ainw/electron/services/review-revision-context.ts:75-89`.
- Ours has durable per-item “keep warning” decisions, but not this immutable apply/ignore snapshot: `internal/store/review_decisions.go:7-25`; `internal/entry/web/review_keep.go:16-27,69-103`.
- This is particularly valuable for author-directed workflows; it should not silently replace our intentional autonomous default.

#### 9. Targeted revision is full-chapter generation followed by selective merge

- The refiner receives the frozen source, applied review brief, author facts, and predecessor material.
- It is instructed to verify each selected problem and remedy against source facts before changing prose.
- It should make the smallest justified correction, preserve uncertainty, and leave unrelated voice/pacing/material alone.
- Despite the targeted objective, the model returns the **entire revised chapter**, not a machine patch.
- The contract asks it to preserve every unaffected paragraph/line and forbids summaries, excerpts, placeholders, and collapsing repeated passages.
- It distinguishes actual actions/results from a decision, promise, signature, or a simple negation flip.
- Evidence: `/tmp/xia-ainw/src/services/workflows/commands/refine-from-review.command.ts:17-33,91-105`.
- Our small-repair path is already more output-efficient: original-string replacement in a draft, with final publication reserved for commit.
- Evidence: `internal/tools/edit_chapter.go:16-24,52-68,95-139`; `assets/prompts/writer.md:24-30`.
- Recommendation: keep our patch-first architecture; adopt Pagekin’s factual-authority and preservation contracts, not full rewrites as the default.

#### 10. Paragraph-level merge limits the blast radius of a generated revision

- Blank-line-delimited paragraphs form alignment units.
- A dynamic program aligns unchanged/changed paragraphs, allowing one-to-one matches, insertions/deletions, and limited one-to-many/many-to-one paragraph splits/merges.
- Similarity is based on character-frequency overlap, with cached adjacent-pair/triple frequencies.
- Equal aligned text becomes unchanged segments; unequal text becomes independently selectable hunks.
- Each hunk defaults to the original and allows original, revised, or manually edited text.
- The merged manuscript is assembled from those choices, rather than automatically replacing the source with all model changes.
- Evidence: `/tmp/xia-ainw/src/components/editor/merge-segments.ts:25-41,60-70,85-168,198-249,259-305`.
- User-facing defaults/actions: `/tmp/xia-ainw/src/components/stage/chapter/MergeView.tsx:28-55,69-109`.
- Quality rationale: authors can reject incidental drift and preserve good paragraphs even if the full generated candidate overreaches.
- Ours already has precise machine edits; selective author hunk approval is a distinct additional affordance, not evidence that our mutation granularity is weaker.

#### 11. Repair status is not inferred from “the text changed”

- Review findings carry stable identity, source/excerpt identity, target identity, objective/literary kind, and explicit lifecycle status.
- A cycle records the original source hash, finding-set hash, revision/merge state, merged hash, and at most one targeted recheck.
- A deterministic admission step rejects unchanged/ambiguous finding evidence before spending a model call.
- For objective findings, whitespace/punctuation differences do not count as repairing the old problematic excerpt.
- The recheck receives immutable problem/expected semantics and must return exactly one result per original finding.
- A remedy introducing unsupported specifics cannot count as fixed; changing/removing the old sentence alone is insufficient.
- Evidence: `/tmp/xia-ainw/src/shared/review-cycle.ts:7-16,35-61,85-87,129-148,234-250`; `/tmp/xia-ainw/src/services/workflows/commands/review-chapter.command.ts:90-110`.
- Most importantly, current v2 policy converts a model’s positive resolved judgment into unknown pending author verification, even with a unique anchored excerpt.
- Negative judgments can remain unresolved; mechanical evidence and semantic success remain separate: `/tmp/xia-ainw/src/shared/review-cycle.ts:174-218`.
- Ours queues/executes chapter rewrites and preserves review rounds, but the inspected `ReviewEntry` lacks an equivalent evidence-bound per-finding resolution lifecycle: `internal/domain/review.go:112-126`; `internal/tools/commit_chapter.go:587-612`.

#### 12. Revision integrity is checked even when the provider says “stop”

- The shared visible-text gate rejects no-prose output, code fences, leftover reasoning tags, opening metacommentary, explicit truncation markers, and obvious newly duplicated long paragraphs/lines.
- Duplicate detection allows repetition already present in the source, avoiding a blanket ban on authorial repetition.
- Revision length is compared to both source and configured target, using language-aware prose units.
- Minimum accepted size is capped by source length and otherwise uses the larger of 200 units or 60% of the bounded target.
- This is a material-truncation safeguard, not a proof that all content survived.
- Evidence: `/tmp/xia-ainw/src/shared/visible-text-integrity.ts:21-81`; `/tmp/xia-ainw/src/services/workflows/commands/refinement-completeness.ts:5-50`.
- The same acceptance function runs in the renderer consumer, main-process commit, and recovery eligibility.
- Evidence: `/tmp/xia-ainw/src/services/workflows/commands/review-revision-command.ts:147-174`; `/tmp/xia-ainw/electron/services/review-revision-generation.ts:243-252,285-293`.
- Ours rejects empty/no-change rewrites and freezes pending-commit prose, but the inspected rewrite acceptance lacks this source-relative truncation gate: `internal/tools/commit_chapter.go:501-521,590-612`.
- Preserve our legitimate title-only polish behavior; Pagekin’s blanket visible-prose no-op policy is not automatically equivalent to our requirements.

#### 13. Bounded completion separates prose continuation from JSON replacement

- Only output-length termination triggers automatic continuation; safety, cancellation, and unknown/error terminal reasons fail closed.
- Visible prose continues from the original task plus a limited visible tail.
- The continuation must add prose; repeating the tail without progress is rejected.
- Structured output is regenerated as one complete replacement JSON object, never joined by appending a raw suffix.
- Default hard mode bounds are three prose continuations and two structured replacements; callers can request less.
- Evidence: `/tmp/xia-ainw/src/services/workflows/bounded-completion.ts:13-17,166-198,200-220,223-295`.
- The tail is 1,600 characters; overlap detection removes a repeated prefix only when it matches at least 48 nonwhitespace characters.
- Evidence: `/tmp/xia-ainw/src/shared/visible-continuation.ts:1-32`; `/tmp/xia-ainw/src/shared/structured-replacement-prompt.ts:3-24`.
- Partial trustworthy text is exposed for recovery when continuation exhausts/fails, without treating it as a completed chapter.
- Our structured executor instead returns `FailureLength` immediately, while draft tools support agent-managed append: `internal/llmcontract/execute.go:126-138`; `internal/tools/draft_chapter.go:15-16,108-131`.

#### 14. Syntax repair is deliberately weaker than semantic regeneration

- Direct malformed JSON is eligible for one dedicated syntax-only repair path.
- The repair request treats both contract and malformed candidate as untrusted data, not instructions.
- It permits punctuation/container closure/wrapping fixes but forbids changing field names, scalar facts, or scalar order.
- Both immutable contract evidence and malformed-candidate evidence have explicit 32,768-byte limits.
- A lexical checker verifies that repaired output parses, scalar tokens match in sequence, and container structure is unchanged except appended closers.
- Evidence: `/tmp/xia-ainw/src/services/workflows/structured-syntax-repair.ts:5-20,23-101,104-183`.
- Quality rationale: a punctuation repair should not quietly “improve” facts, create missing chapters, or choose different narrative content.
- Ours has schema/decode/business correction messages, but correction returns a regenerated object without this scalar-preservation contract: `internal/llmcontract/execute.go:149-186`.
- Port only as an optional narrow repair layer; lexical preservation still does not prove narrative correctness or complete coverage.

#### 15. Structured batch recovery shrinks the semantic task instead of looping blindly

- The executor validates item shape and exact requested-key coverage before accepting output.
- Output-limited multi-item requests split into smaller batches; single-item requests may use one compact rebuild from bounded original facts.
- The compact fallback does not receive the partial generated output as authority.
- Decode failures also lead to splitting or compact single-item fallback within the surrounding generation budget.
- Duplicate and unexpected items fail; ordinary stop-with-missing-items does not get endless semantic retries.
- After syntax repair, missing items can trigger smaller regeneration because closing JSON cannot invent absent semantic content.
- Evidence: `/tmp/xia-ainw/src/services/workflows/structured-batch-executor.ts:19-39,243-292,398-453,474-524`.
- Accepted artifacts are recorded only after validation; a validated prefix survives later failure: `/tmp/xia-ainw/src/services/workflows/structured-batch-executor.ts:57-100,520-524`.
- Ours already has provider-neutral schema selection and business validation, but the inspected correction loop continues until context termination rather than using an explicit per-action correction count: `internal/llmcontract/execute.go:82-107,172-187`.
- This is a high-value cost/reliability idea for multi-chapter planning, without importing Pagekin’s entire runtime.

#### 16. Contract diagnostics describe the defect without echoing candidate values

- Stable diagnostic categories include invalid envelope/type/value, missing/duplicate/unexpected items, empty/oversized values, and invalid relationships.
- Diagnostics carry schema path and optional character counts, deliberately excluding candidate values and raw parser messages.
- Blueprint validation requires nonempty semantic fields, at least one unique named character, valid relationship endpoints, no self-reference, and exact chapter coverage.
- This verifies the usable narrative-plan shape, not whether the plan is creatively good.
- Evidence: `/tmp/xia-ainw/src/shared/structured-contract-diagnostic.ts:3-43`; `/tmp/xia-ainw/src/shared/blueprint-semantic-contract.ts:18-38,102-160,231-263`.
- Outline acceptance similarly requires one complete chapter entry with a correct heading and nonempty body, not a grouped substitute.
- Each outline chapter gets normal/compact attempts; only a valid stopped artifact advances the cursor.
- Evidence: `/tmp/xia-ainw/src/shared/plot-outline-contract.ts:33-36,158-182`.
- Checkpoints bind confirmed outline body/range and database fingerprints so continuation cannot silently overwrite intervening author edits: `/tmp/xia-ainw/src/shared/plot-outline-checkpoint.ts:5-20,69-129`.
- Ours already has schema fingerprints and typed Arbiter validation; extend that seam rather than creating a competing validator: `internal/arbiter/arbiter.go:28-57`; `internal/llmcontract/contract.go:223-228`.

#### 17. Pre-draft reconciliation tries to resolve plan/history conflicts before prose generation

- A bounded structured reconciliation lists prior final state and each chapter event’s conflict flag plus a compatible realization.
- Results are rendered into a dedicated source-derived instruction block.
- Withdrawn, cancelled, paused, and still-waiting plans must not be narrated as executed.
- Malformed reconciliation returns no block rather than inventing a usable reconciliation result.
- The main process can reconstruct/remove the exact block from the recorded artifact, protecting prompt/source identity.
- Evidence: `/tmp/xia-ainw/src/shared/draft-reconciliation.ts:4-22,31-64,66-115`.
- Production drafting composes recorded reconciliation and short-outline blocks: `/tmp/xia-ainw/src/services/workflows/commands/generate-draft.command.ts:1020-1024`.
- Quality rationale: resolving blueprint-versus-finalized-history tension before drafting can be cheaper than writing a contradictory chapter and repairing it later.
- Ours already plans from chapter context/contracts and checks consistency afterward; this slice did not find an equivalent dedicated reconciliation artifact.
- Treat adding another model call as an experiment, not a prerequisite for every chapter.

#### 18. Quality measurement evaluates the reviewer, not just the writer

- The continuity calibration preregistered ten cases, two arms, identical factual material, no semantic retries, and a development/held-out split.
- Gold labels were kept separate from the runner and prompts.
- Correct controls included indirect transfers, lying characters, future arrangements, and missing-history uncertainty—not only easy injected contradictions.
- Metrics separated structural validity, exact critical-error detection, risk adjudication, false positives, uncertainty, and token cost.
- Evidence: `/tmp/xia-ainw/docs/experiments/continuity-calibration-round01.md:11-34,40-42`.
- Reported result: targeted checking detected one of two definite errors versus zero of two for ordinary review; both missed the other.
- Both arms correctly handled all five positive controls, but neither correctly adjudicated either risk case.
- Targeted checking handled the single missing-evidence uncertainty case better; perfect mechanical validity did not imply semantic validity.
- Evidence: `/tmp/xia-ainw/docs/experiments/continuity-calibration-round01-results.md:17-54`.
- The documented decision was **not** to connect this checker to automatic finalization: `/tmp/xia-ainw/docs/experiments/continuity-calibration-round01-results.md:88-96`.

#### 19. Current qualification distinguishes autonomous discovery from assisted correction

- Current policy accepts imperfect first drafts only when the software’s original review independently discovers actionable key defects and the approved repair produces acceptable final prose.
- Operators cannot add oracle-derived findings, rewrite the report to the answer, or hint known defects through review focus.
- One authorized repair and a normal whole-chapter rereview preserve bounded behavior; a targeted recheck is not a substitute for final review.
- Two independent reviewers assess initial defects, original AI findings, actual revisions, and final manuscript; only disagreements require arbitration.
- Technical success can be automatic, but literary qualification remains pending until independent review.
- Evidence: `/tmp/xia-ainw/docs/research/novel-quality-modernization/quality-protocol.md:25-66`.

[Showing lines 1-250 of 329. Use :251 to continue]
- Samples/configuration/round identities are frozen; failed, unknown, aborted, and not-run slots remain in the record instead of being replaced by favorable reruns.
- Evidence: `/tmp/xia-ainw/docs/research/novel-quality-modernization/quality-protocol.md:130-146,172-180`.
- The current document explicitly says implemented wiring has **not** earned formal qualification: `/tmp/xia-ainw/docs/research/novel-quality-modernization/quality-protocol.md:9-23`.
- Ours already has production-path A/B/repeat/diagnostic/style evaluation, but independent semantic review is still optional/not implemented in that harness: `docs/evaluation-system.md:61-85,117`; `internal/eval/case.go:1-8`.

### Learnable techniques table

| # | Technique | Source evidence | Ours (EXISTS/PARTIAL/NEW + citation) | Impact (H/M/L) | Effort (S/M/L) | Notes for Go port |
|---|---|---|---|---|---|---|
| 1 | Separate objective defects from subjective craft advice | `/tmp/xia-ainw/src/services/builtin-prompt-templates.ts:950-978` | PARTIAL — seven-dimensional editor mixes structural and aesthetic review; `assets/prompts/editor.md:32-74` | H | S | Preserve literary review; distinguish blocking facts from optional craft recommendations. |
| 2 | Evidence-first finding fields/classification | `/tmp/xia-ainw/src/services/builtin-prompt-templates.ts:974-979` | PARTIAL — evidence/comments required, but no prescribed evidence-before-verdict discipline; `internal/tools/save_review.go:77-94,289-301` | M | S | Independent wording; prompt-only pilot before changing persistence. |
| 3 | Frozen per-goal IDs and exact review coverage | `/tmp/xia-ainw/src/shared/chapter-goal-review.ts:18-39,94-125` | PARTIAL — chapter contracts exist, report is aggregate; `assets/prompts/editor.md:26-27`; `internal/domain/review.go:118-120` | H | M | Extend existing contract/report; software owns IDs and original requirement text. |
| 4 | Goal completed/unmet/unknown distinction | `/tmp/xia-ainw/src/shared/chapter-goal-review.ts:42-69` | PARTIAL — aggregate met/partial/missed only; `internal/tools/save_review.go:100-102` | H | M | Explicit unknown avoids false certainty; do not equate missing prose with a contradictory event. |
| 5 | Evidence spans bound to frozen prose | `/tmp/xia-ainw/src/shared/chapter-goal-review.ts:77-125` | PARTIAL — evidence required, quote presence checked for some package errors; `internal/tools/save_review.go:289-311,437-442` | H | M | Use UTF-8 byte offsets or explicit rune units; validate uniqueness on every path. |
| 6 | Time-scoped author facts and plan/history labels | `/tmp/xia-ainw/electron/services/review-revision-context.ts:113-150`; `/tmp/xia-ainw/src/shared/review-revision-generation.ts:107-121` | PARTIAL — working/episodic/reference split; `assets/prompts/editor.md:5,23-27` | H | M | Add provenance/temporal labels at existing context assembly points. |
| 7 | Immutable apply/ignore confirmation snapshot | `/tmp/xia-ainw/src/shared/human-confirmed-review.ts:6-44,219-283` | PARTIAL — durable keep-warning decision, not applied-work snapshot; `internal/store/review_decisions.go:7-25`; `internal/entry/web/review_keep.go:16-27` | M | M | Optional review-mode feature; preserve autonomous mode’s existing contract. |
| 8 | AI suggestion approval never grants factual authority | `/tmp/xia-ainw/src/services/workflows/commands/refine-from-review.command.ts:17-33` | PARTIAL — scoped user intervention already limits repair; `assets/prompts/editor.md:11-19`; `internal/tools/novel_context_builders.go:287-332` | H | S | Add explicit remedy validation against facts; selected issue is not a trusted answer. |
| 9 | Minimal targeted edits preserve unrelated prose | `/tmp/xia-ainw/src/services/workflows/commands/refine-from-review.command.ts:17-33` | EXISTS — patch-first edit tool and structural-only full overwrite guidance; `internal/tools/edit_chapter.go:52-68`; `assets/prompts/writer.md:24-30` | H | S | Keep ours; do not regress to mandatory full-chapter generation. |
| 10 | Author-selectable paragraph merge | `/tmp/xia-ainw/src/components/editor/merge-segments.ts:242-305` | PARTIAL — precise draft edits exist, but not equivalent hunk approval; `internal/tools/edit_chapter.go:19-24,120-139` | M | L | Optional CLI patch preview/selection or server merge view; simpler diff first. |
| 11 | Changed text is not proof of repaired finding | `/tmp/xia-ainw/src/shared/review-cycle.ts:129-148,234-250` | PARTIAL — rewrite no-op gate exists, not per-finding repair accounting; `internal/tools/commit_chapter.go:501-521`; `internal/domain/review.go:112-126` | H | M | Tie findings to source hash and targeted before/after evidence. |
| 12 | Positive model recheck remains pending author verification | `/tmp/xia-ainw/src/shared/review-cycle.ts:174-218` | NEW — existing reviewed state has no equivalent field/lifecycle; `internal/domain/review.go:112-126` | M | M | Useful in explicit author-review mode; do not universally block autonomous writing. |
| 13 | Source-relative truncated-rewrite rejection | `/tmp/xia-ainw/src/services/workflows/commands/refinement-completeness.ts:5-50` | PARTIAL — empty/no-change guards, no inspected relative-length gate; `internal/tools/commit_chapter.go:501-521,590-612` | H | S | Reject suspiciously incomplete full replacements; exempt intentional shortening/title-only work. |
| 14 | Stop-independent visible prose integrity | `/tmp/xia-ainw/src/shared/visible-text-integrity.ts:21-81` | PARTIAL — draft/commit checks exist, not this common gate; `internal/tools/commit_chapter.go:501-521` | M | S | Share one acceptance function across generation, recovery, commit. |
| 15 | Syntax-only JSON repair with scalar conservation | `/tmp/xia-ainw/src/services/workflows/structured-syntax-repair.ts:23-101,166-183` | NEW — generic regeneration correction exists; `internal/llmcontract/execute.go:149-186` | M | M | Narrow optional path; retain schema/business validation afterward. |
| 16 | Bounded correction attempts and provenance-aware recovery | `/tmp/xia-ainw/src/services/workflows/commands/review-revision-command.ts:147-174`; `/tmp/xia-ainw/src/services/workflows/review-revision-recovery-workflow.ts:10-39` | PARTIAL — context-bounded structured correction and frozen pending commits; `internal/llmcontract/execute.go:107-187`; `internal/tools/commit_chapter.go:590-595` | H | M | Keep attempt allowance in existing action/session state across restarts. |
| 17 | Structured length failure splits semantic batches | `/tmp/xia-ainw/src/services/workflows/structured-batch-executor.ts:243-292,398-453` | PARTIAL — structured executor returns length failure; `internal/llmcontract/execute.go:126-129` | H | M | Add only at multi-item domain callers; avoid generic “split arbitrary JSON” abstraction. |
| 18 | Safe code/path diagnostics instead of raw candidate echo | `/tmp/xia-ainw/src/shared/structured-contract-diagnostic.ts:22-43` | PARTIAL — typed failure/layer and schema fingerprint exist; `internal/llmcontract/execute.go:149-185`; `internal/arbiter/arbiter.go:41-50` | M | S | Typed Go diagnostic with code/path; preserve detailed local logs separately. |
| 19 | Pre-draft plan/history reconciliation artifact | `/tmp/xia-ainw/src/shared/draft-reconciliation.ts:4-22,66-101` | PARTIAL — contracts/context and post-edit consistency; `assets/prompts/editor.md:23-30`; `assets/prompts/writer.md:24-29` | M | M | Experiment only on chapters with conflicting plans/history, not unconditional extra calls. |
| 20 | Reviewer calibration with positive controls and unknown cases | `/tmp/xia-ainw/docs/experiments/continuity-calibration-round01.md:16-34` | PARTIAL — editor cases exist, some expected verdict assertions not wired; `evals/cases/editor-goi-a/safe-ending.json:5-12` | H | M | Add original-review detection/false-positive assertions and held-out controls. |
| 21 | Independent end-to-end discovery/repair qualification | `/tmp/xia-ainw/docs/research/novel-quality-modernization/quality-protocol.md:42-66` | PARTIAL — real harness A/B/repeat exists; independent judge/human layer remains optional; `docs/evaluation-system.md:61-85,117` | H | M | Reuse our runner; add frozen review packets and independent adjudication, not another platform. |
| 22 | Distinguish propagation permission from semantic acceptance | `/tmp/xia-ainw/src/shared/chapter-goal-review.ts:11-16`; `/tmp/xia-ainw/src/shared/review-cycle.ts:205-218` | EXISTS architecturally — gate controls permission, Arbiter semantics remain separate; `docs/chapter-advance-gate.md:18-30,63-77` | H | S | Preserve this separation; chapter permit or successful commit must not become a quality-pass claim. |

### Pitfalls / over-engineering in source we should NOT copy

- **Do not copy its entire administrative quality-program machinery.** Feature-union ledgers contain 153 action assertions and many publication/UI obligations unrelated to our narrow AI-quality problem.
  - Evidence: `/tmp/xia-ainw/docs/plans/novel-quality-program-v3-2026-09-13/09-FEATURE-UNION.md:26-38`.
  - Borrow frozen inputs, honest evidence levels, and independent judgment—not the organizational surface area.

- **Do not confuse historical plans with current product policy or qualification.** The v3 master still describes an 80-call cap and 18-chapter two-arm program; the current protocol explicitly supersedes those assumptions.
  - Evidence: `/tmp/xia-ainw/docs/plans/novel-quality-program-v3-2026-09-13/01-MASTER-PLAN.md:23,33-35`; `/tmp/xia-ainw/docs/research/novel-quality-modernization/quality-protocol.md:13-21,63-66,172-180`.
  - The useful lesson is to keep one concise current protocol rather than accumulate policy layers that readers must reconcile.

- **Do not advertise mandatory excerpt uniqueness or character caps without implementing them.** Direct goal excerpts are not universally uniqueness-checked; current report parsing does not enforce the prompt’s length bounds.
  - Evidence: `/tmp/xia-ainw/src/shared/chapter-goal-review.ts:78-90`; `/tmp/xia-ainw/src/shared/review-generation-report.ts:39-42,93-105`.
  - Our port should make mechanical promises explicit and testable.

- **Do not equate quote anchoring, valid JSON, a stopped response, a changed hunk, merge completion, or model pass with semantic correctness.**
  - Evidence: `/tmp/xia-ainw/src/shared/review-cycle.ts:174-218`; `/tmp/xia-ainw/docs/experiments/continuity-calibration-round01-results.md:43-54`.
  - This is the source’s most important negative lesson.

- **Do not replace our efficient exact-edit path with compulsory full-chapter rewrites.** Pagekin targets issues semantically but regenerates all prose, then needs length/integrity gates and paragraph merge to contain damage.
  - Evidence: `/tmp/xia-ainw/src/services/workflows/commands/refine-from-review.command.ts:17-33`; `/tmp/xia-ainw/src/components/editor/merge-segments.ts:242-305`.
  - Our `edit_chapter` already minimizes the rewritten surface; retain it.

- **Do not copy the paragraph alignment algorithm before proving simpler diff is insufficient.** It uses full paragraph-pair dynamic-programming matrices and character-frequency similarity, including split/merge alternatives.
  - Evidence: `/tmp/xia-ainw/src/components/editor/merge-segments.ts:85-168`.
  - It helps presentation; it is not an evidence-preserving semantic mapping and should not define finding resolution.

- **Do not treat size ratios as completeness proofs.** A replacement can exceed the 60% floor while losing essential scenes, and an intentional compression may legitimately become much shorter.
  - Evidence: `/tmp/xia-ainw/src/services/workflows/commands/refinement-completeness.ts:35-50`.
  - Use it only as a suspicious-truncation guard, conditioned on the requested operation.

- **Do not turn source-specific regexes into universal literary truth.** Visible-integrity rules contain Chinese/English metacommentary/truncation patterns and a fixed long-duplicate threshold.
  - Evidence: `/tmp/xia-ainw/src/shared/visible-text-integrity.ts:4-7,43-60,63-79`.
  - Vietnamese output and deliberate stylistic repetition need our own observed failure cases.

- **Do not make manual per-finding approval mandatory for autonomous mode.** Pagekin intentionally keeps positive rechecks pending author verification.
  - Evidence: `/tmp/xia-ainw/src/shared/review-cycle.ts:205-211`.
  - Our `auto` versus `review` policy is explicit and should remain independent: `docs/chapter-advance-gate.md:11-16,63-77`.

- **Do not claim quality improvements from the small continuity experiment.** Its own result says only one additional definite-error detection, persistent risk failures, and no demonstrated long-book or literary gains.
  - Evidence: `/tmp/xia-ainw/docs/experiments/continuity-calibration-round01-results.md:36-43,88-96`.
  - The experiment design is more transferable than any assertion that the targeted reviewer “works.”

- **Do not declare the latest quality program passed.** Current documentation says wiring is implemented but formal qualification remains outstanding; historical failures are retained.
  - Evidence: `/tmp/xia-ainw/docs/research/novel-quality-modernization/quality-protocol.md:9-23,54-66`.
  - We should match that distinction between capability presence, technical validation, and demonstrated writing-quality benefit.

Status: DONE_WITH_CONCERNS — All assigned mechanisms and requested comparisons were inspected read-only, including the corrected `.command.ts` filenames. Strong ideas are identifiable, but the source contains concrete prompt/enforcement discrepancies and explicitly unqualified quality claims; none of its reported experiments were rerun or independently reproduced.
