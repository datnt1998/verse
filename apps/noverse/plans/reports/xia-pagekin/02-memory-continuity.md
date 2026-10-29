## Slice: Long-book memory, continuity, knowledge & retrieval

Source citations below are relative to `/tmp/xia-ainw`; **Ours** citations are relative to `/Users/dat/Projects/ainovel-cli`.
This was a strictly read-only source study; no code, builds, installations, tests, or model requests were executed.
Quality effects and impact ratings are engineering judgments, not measured Pagekin-versus-ours outcomes.

**Main finding:** Pagekin’s strongest transferable idea is not vector search: it is treating generated memory as a **versioned locator into authoritative prose**, rather than automatically trusting generated summaries as facts.
Evidence: `docs/adr/0017-source-bound-character-state-and-continuity.md:3-9`; `src/services/workflows/chapter-materials.ts:403-423,535-548`.
We already have substantial structured memory, canonical chapter records, deterministic replay, selective recall, and contradiction lint; the largest gaps are exact evidence binding, asynchronous source revalidation, and prose-first historical retrieval.
Ours: `internal/domain/revision.go:21-37,95-108`; `internal/revision/projector.go:41-89`; `internal/tools/novel_context.go:832-888`; `internal/naolint/story.go:13-92`.

### Mechanisms

#### 1. Authority hierarchy: author facts, finalized prose, and derived memory remain different things

- **What:** Author input and confirmed chapter content are authoritative; summaries, character-state caches, continuity projections, and plot-tree snapshots are derived.
- The domain explicitly says generated projections must not overwrite author facts, and a locatable quotation is not proof that an interpretation is semantically correct.
- Evidence: `docs/product-domain.md:9-17`; `docs/adr/0017-source-bound-character-state-and-continuity.md:7-9`.
- **How:** First preserve the original source; then derive summaries/state; then retain the derivation’s provenance; finally re-read prose when downstream writing needs factual evidence.
- Draft generation intentionally leaves the old global-summary, short-summary, and previous-ending prompt slots empty, replacing them with one source-labelled material package.
- Evidence: `src/services/workflows/commands/generate-draft.command.ts:727-755`.
- **Why:** [INFERENCE] This prevents an inaccurate summary from being repeatedly summarized and promoted into increasingly confident canon.
- **Our comparison:** PARTIAL. We have canonical accepted prose plus facts, but normal chapter context directly supplies summaries and state rather than requiring evidence-backed prose retrieval.
- Ours: `internal/domain/revision.go:95-108`; `internal/tools/novel_context_builders.go:446-478,555-594`.

#### 2. Finalization first freezes and commits prose; semantic postprocessing comes afterward

- **What:** A finalization identifies the exact chapter version through draft ID, finalization ID, chapter number, and content hash.
- The editor snapshot freezes content and revision at confirmation; asynchronous completion cannot mark newer editor content as already finalized.
- Evidence: `src/services/finalization-snapshot.ts:11-24,41-67,70-105`; `src/shared/finalized-continuity.ts:11-27`.
- **How:** Hash frozen prose → commit the finalization receipt/outbox → publish the same immutable snapshot → run knowledge import and semantic postprocessing.
- Publication retry reads the committed snapshot, not whatever content happens to be currently visible.
- Evidence: `electron/services/finalization-service.ts:65-90,101-146,149-194`.
- **Why:** [INFERENCE] Prose and extracted memory cannot silently describe different chapter revisions after an interrupted operation.
- **Our comparison:** EXISTS for frozen commit intent and accepted content hashes; PARTIAL for version-bound downstream extraction receipts.
- Ours: `internal/tools/commit_chapter.go:58-64,108-119,219-226`; `internal/domain/revision.go:95-108`; `internal/revision/service.go:209-234`.

#### 3. Finalization postprocessing has three actual steps—not automatic thread mutation

- **What:** The postprocessing builder creates knowledge import, chapter-notes extraction, and character-state updating.
- Knowledge import and chapter notes are marked critical; character-state updating is noncritical.
- Evidence: `src/services/workflows/commands/finalize-chapter.command.ts:112-167,267-305`.
- **How:** Import immutable prose as knowledge chunks → record the returned knowledge-document identity → generate chapter notes → save continuity projection and blueprint notes → propose/apply dynamic character patches.
- Evidence: `src/services/workflows/commands/finalize-chapter.command.ts:129-159,186-238,243-250,272-300`.
- **Important distinction:** Narrative-thread candidates are a separate generation/confirmation flow; they are not a fourth automatic finalization step.
- Evidence: `src/services/workflows/commands/finalize-chapter.command.ts:267-305`; `src/services/narrative-thread-candidate-generator.ts:60-89`; `electron/services/graph-generation.ts:100-125`.
- **Why:** [INFERENCE] Separating persistence from optional semantic interpretation allows recovery without losing confirmed prose.
- **Our comparison:** EXISTS for summary/timeline/thread/state updates at commit; NEW for independently recoverable evidence-bound extraction stages.
- Ours: `internal/tools/commit_chapter.go:276-318`.

#### 4. Chapter notes explicitly preserve modality, corrections, and attribution

- **What:** Notes cover irreversible plot changes, character dynamics, newly established setting facts, and explicitly unresolved threads/hooks.
- Empty sections are allowed; the model is told not to invent information to fill a template.
- Evidence: `src/services/builtin-prompt-templates.ts:1062-1103`.
- **How:** Read the complete chapter → preserve subject/change and any explicitly stated cause/location/knower/source → distinguish happened versus planned, suspected, denied, and unknown → preserve the latest correction.
- The extraction contract rejects deriving causation, ownership, responsibility, or narrative purpose from mere co-occurrence.
- Evidence: `src/services/builtin-prompt-templates.ts:1105-1111`.
- **Why:** [INFERENCE] This targets common long-book errors: intended travel becoming arrival, a suspicion becoming truth, and an early plan surviving its later cancellation.
- **Our comparison:** PARTIAL. Our writer must submit facts from actual prose and not invent events, but the inspected contract lacks these explicit modality/correction/attribution distinctions.
- Ours: `assets/prompts/writer.md:64-64`; `internal/domain/revision.go:21-37`.

#### 5. Continuity facts are compact, categorized evidence locators

- **What:** A fact contains category, entity labels, statement, source chapter, evidence text, and optional proven character-ID references.
- Categories are character state, timeline, open thread, and plot.
- Evidence: `src/shared/finalized-continuity.ts:5-9,40-53`.
- **How:** Split generated notes into statements → identify mentioned known entities → choose a supporting source sentence using lexical bigram overlap → categorize → keep at most 12 facts.
- Source sentences are ranked after removing entity-name contribution; unsupported statements and ambiguous same-display-name character associations are dropped.
- Evidence: `src/shared/finalized-continuity-facts.ts:3-9,12-54,57-87`.
- At save time, the repository checks that every evidence excerpt occurs in the immutable source and that character references match the frozen identity snapshot.
- Evidence: `electron/repositories/summary-repository.ts:569-605`.
- **Why:** [INFERENCE] A small evidence index can locate relevant historical paragraphs without injecting the entire book.
- **Our comparison:** PARTIAL. Our per-chapter facts are richer in several domains but lack exact supporting spans for each timeline/state/thread assertion.
- Ours: `internal/domain/revision.go:21-47`; `internal/domain/tracking.go:3-10`; `internal/domain/review.go:3-25`.

#### 6. Character state is a field-level mixed-provenance cache

- **What:** Dynamic fields are location, power level, physical state, mental state, key items, and recent events.
- Each field can independently originate from author input, finalized-prose derivation, or unknown legacy data.
- Evidence: `src/shared/character-roster.ts:44-69`; `docs/adr/0017-source-bound-character-state-and-continuity.md:3-5`.
- **How:** Freeze identity and field baselines → ask only for changed dynamic fields → bind updates to stable character IDs and evidence → compare field revision/hash/provenance before applying.
- Author and nonempty legacy values become proposals rather than being silently overwritten; derived values may advance only in valid chapter/version order.
- Evidence: `src/shared/character-identity.ts:33-43,65-93`; `electron/repositories/summary-repository.ts:423-468`.
- The prompt specifically requires end-of-chapter recent events and forbids converting an intention to move into a changed location.
- Evidence: `electron/services/finalization-generation-source.ts:47-58`.
- **Why:** [INFERENCE] This separates “the author set this” from “the extractor currently believes this,” while resisting stale concurrent updates.
- **Our comparison:** PARTIAL. We keep structured state transitions and snapshots, but the inspected `StateChange` contract has no field-level author/derived provenance, source hash, or evidence span.
- Ours: `internal/domain/tracking.go:3-10`; `internal/store/characters.go:61-67`; `internal/tools/commit_chapter.go:311-318`.

#### 7. Character identity ambiguity is preserved rather than guessed away

- **What:** Display names are not sufficient identity proof; generated updates must identify characters from the frozen roster.
- A missing/unknown ID, legacy identity, or duplicated display label produces an unresolved occurrence with candidate IDs, raw model output, and evidence.
- Evidence: `src/shared/finalized-continuity.ts:148-162,209-222`.
- **How:** Resolve exact scoped candidates → admit only a uniquely proven identity → preserve ambiguity for explicit confirmation → never use similarity to auto-merge identities.
- Even a supplied model ID does not bypass duplicate frozen display-label ambiguity.
- Evidence: `src/shared/character-identity.ts:12-18,115-129`; `src/shared/finalized-continuity.ts:213-222`.
- Exact evidence may omit offsets if the quote occurs once; the program calculates offsets, otherwise rejects missing, repeated, or mismatched evidence.
- Evidence: `src/shared/finalized-continuity.ts:175-199`.
- **Why:** [INFERENCE] This avoids poisoning multiple future chapters through one mistaken identity merge.
- **Our comparison:** PARTIAL. Stable IDs, retained IDs after rename, and exact name/alias lookup already exist; frozen source-scoped alias revisions and evidence-bound unresolved occurrences do not appear in the inspected contracts.
- Ours: `internal/store/nao_identity.go:12-19,52-70`; `internal/store/nao_test.go:35-70`.

#### 8. Narrative threads separate author plans from confirmed occurrences

- **What:** A plan stores title, type, target start/end chapters, and author intent.
- An event stores plan ID, finalized draft ID, event type, evidence, and reason; current status is a projection of confirmed events.
- Evidence: `src/shared/narrative-thread.ts:1-2,18-49`.
- **How:** Generate optional plans from blueprints → author confirms a plan → inspect one finalized chapter against one plan → author confirms an evidence-backed event → fold events ordered by chapter and confirmation ID.
- Event types are `planted`, `progressing`, `resolved`, and `abandoned`; “advanced” in our vocabulary corresponds to their `progressing`.
- Evidence: `src/shared/narrative-thread-generation-pure.ts:103-118,121-175`; `electron/repositories/narrative-thread-repository.ts:80-106,129-148`.
- The model does not choose authoritative plan/draft/chapter IDs in event output; application context supplies them at confirmation.
- Evidence: `src/shared/narrative-thread-generation-pure.ts:146-173`; `electron/services/graph-generation.ts:118-125`.
- **Why:** [INFERENCE] A planned payoff cannot be mistaken for a payoff that has actually appeared in prose.
- **Our comparison:** PARTIAL. We already persist plant/advance/resolve deltas and replay them; source adds independent author intent, target windows, evidence/reason, abandonment, and explicit candidate adoption.
- Ours: `internal/domain/review.go:11-25`; `internal/revision/projector.go:113-140`.

#### 9. Thread relevance includes schedules and entities, with bounded prompt injection

- **What:** Relevant active threads exclude resolved/abandoned threads and include those in the target window or matching current characters/title.
- Thread state also exposes dormant-chapter count and overdue status.
- Evidence: `electron/repositories/narrative-thread-repository.ts:141-169`.
- **How:** Compute last confirmed event and current finalized chapter → derive dormancy/overdue → filter relevant threads → render title/status/target/author intent/latest evidence.
- The drafting context caps this block at six items and approximately 1,200 characters.
- Evidence: `src/services/workflows/commands/generate-draft.command.ts:72-73,2002-2046`.
- **Why:** [INFERENCE] This keeps long-running narrative obligations visible without dumping the full ledger into every chapter.
- **Our comparison:** EXISTS for bounded relevant-thread recall and age-based fallback; PARTIAL for scheduled windows and evidence-rich events.
- Ours: `internal/tools/novel_context.go:832-888,1045-1052`.
- Our age-based refill is worth retaining: it recalls old unrelated unresolved threads that entity-only selection would miss.
- Ours: `internal/tools/novel_context.go:873-906`.

#### 10. Next-chapter memory fetches actual historical prose, not old note statements

- **What:** The continuity index selects recent chapters and older chapters relevant to the current cast; it then re-reads authoritative source prose.
- The recent window is five chapters; candidate source chapters are sorted newest-first and capped at 12.
- Evidence: `src/services/workflows/commands/generate-draft.command.ts:1848-1880`.
- **How:** Collect evidence locators → select current finalized replacement when an index is stale → read/hash-bound prose → locate evidence paragraphs → include one complete neighboring paragraph on either side.
- Missing locators trigger deterministic term-based paragraph recovery; obsolete statements themselves are never inserted as substitute facts.
- Evidence: `src/services/workflows/commands/generate-draft.command.ts:1882-1921`; `src/services/workflows/chapter-materials.ts:345-365,380-423`.
- The immediate predecessor is fetched independently if absent and is admitted as **full required prose**, not just an optional ending snippet.
- Evidence: `src/services/workflows/commands/generate-draft.command.ts:1937-1997`; `src/services/workflows/chapter-materials.ts:460-470,569-571`.
- **Why:** [INFERENCE] Neighboring prose preserves corrections, negations, speaker attribution, and causality that a single extracted sentence can lose.
- **Our comparison:** PARTIAL. We inject an 800-rune previous tail, summaries, timeline, and selected related chapters; we do not equivalently mandate full predecessor prose or expand evidence spans into neighborhoods.
- Ours: `internal/tools/novel_context_builders.go:468-505,584-594`.

#### 11. Context admission is source-aware and refuses silent loss of required material

- **What:** One selection decision determines rendered context, omissions, consumed sources, and the dependency receipt.
- The receipt records source ID/revision/hash, category, requiredness, units, and omission reasons without exposing prose in diagnostic metadata.
- Evidence: `src/services/workflows/chapter-materials.ts:116-139,141-214`.
- **How:** Build author/future-plan materials and predecessor as required → add historical prose and optional references → deduplicate → select within capacity → fail explicitly if required coverage is impossible.
- Future plans constrain premature execution, not the current chapter’s task list; character state is explicitly time-bound, not permanent.
- Evidence: `src/services/workflows/chapter-materials.ts:93-113,519-548,609-647`.
- **Why:** [INFERENCE] Source-labelled admission prevents a tiny optional reference from crowding out an essential predecessor or author constraint.
- **Our comparison:** PARTIAL. We protect active address terms and trim individual graph-context items; our current overall budget is deliberately soft after a real retry-loop incident.
- Ours: `internal/tools/nao_context.go:72-78,136-195`; `internal/tools/novel_context.go:217-251`.
- **Port caution:** Adopt omission/provenance receipts, not Pagekin’s hard-failure behavior wholesale; preserve our intentional non-looping soft-budget policy.

#### 12. Consistency preflight is narrow, deterministic, evidence-bearing, and overridable

- **What:** The implemented rule detects an explicitly terminal/dead character state conflicting with that character’s inclusion in the new blueprint.
- It considers only current projections, returns warnings, and includes source chapter/evidence; it is not a general semantic contradiction solver.
- Evidence: `src/shared/consistency-preflight.ts:44-55,78-118`.
- **How:** Load projections before each blueprint → ignore active author exemptions → match explicit terminal subject → produce warning → optionally merge it into review findings.
- Stable fact keys derive from category, chapter, normalized entities, and statement; exemptions carry a reason and revocation flag.
- Evidence: `src/services/consistency-preflight.ts:11-24`; `src/shared/consistency-preflight.ts:3-15,58-75,121-134`.
- **Why:** [INFERENCE] Cheap checks catch a known high-impact continuity failure before tokens are spent writing an impossible scene, without banning flashbacks.
- **Our comparison:** PARTIAL, not NEW. Our lint already checks dead-character appearances, foreshadow order, and unexplained relationship/address changes, but against chapter facts rather than this blueprint-stage warning/exemption contract.
- Ours: `internal/naolint/story.go:13-92`; `internal/tools/check_consistency.go:14-26,70-108`.

#### 13. Time continuity is a shared attribution contract, not a normalized story clock

- **What:** Unless author/prose establishes a gap, a new chapter continues within the same day; relative references must match the actual established timing.
- Review/refinement gives explicit manuscript day changes priority, attributes untimed actions to the nearest preceding explicit time, and preserves uncertainty when timing cannot be established.
- Evidence: `src/shared/chapter-time-continuity.ts:3-14`.
- **How:** Share the same drafting rule across consumers → append stricter review attribution → forbid turning an AI-suggested time into source evidence.
- Drafting includes this rule alongside requirements to preserve finalized ownership, knowledge, causality, and unresolved-question boundaries.
- Evidence: `src/services/workflows/commands/generate-draft.command.ts:419-426`.
- **Why:** [INFERENCE] It directly addresses relative-day drift such as a same-evening event becoming “yesterday” in the next chapter.
- **Our comparison:** PARTIAL. We store timeline time text and inject recent events; our inspected writer/editor rules ask for temporal consistency but lack this explicit relative-day attribution algorithm.
- Ours: `internal/domain/review.go:3-8`; `internal/tools/novel_context_builders.go:475-478`; `assets/prompts/editor.md:36-40,72-72`.

#### 14. Embedding retrieval is optional, versioned, and backed by lexical fallback

- **What:** LanceDB stores canonical text chunks plus separate versioned embedding generations; embedding identity includes model fingerprint, dimensions, distance semantics, and generation.
- Chunking is paragraph-first, then sentence-aware, with default 500-character chunks and 50-character overlap; oversized sentences are forcibly bounded.
- Evidence: `electron/vector-store.ts:78-113`; `electron/embedding.ts:346-414`.
- **How:** Optionally embed the query → require matching active embedding space → vector search → if unavailable/mismatched/empty, use canonical lexical search.
- Endpoint/model identity is fingerprinted without API secrets; embedding failures preserve the no-embedding path.
- Evidence: `electron/knowledge-base.ts:75-118`; `electron/services/generation-knowledge-source.ts:117-147`.
- **Critical correction:** Despite comments advertising BM25/hybrid, inspected execution is **vector-first plus SQL `LIKE` fallback**, not fused BM25/vector ranking.
- Evidence: `electron/knowledge-base.ts:122-125`; `electron/vector-store.ts:1436-1511`; `electron/services/generation-knowledge-source.ts:117-147`.
- **Our comparison:** NEW for production embedding-space infrastructure, intentionally deferred here; our documented W0 showed no decisive benefit and requires a larger evaluation before W4.
- Ours: `docs/prd/nao-truyen.md:217-234`; `docs/adr/0001-nao-truyen-du-lieu-goc-json.md:9-18`.

#### 15. Retrieved knowledge is frozen and verified by exact IDs before consequential actions

- **What:** A knowledge snapshot records chunk/document IDs, chunk index, text hash, canonical/table revisions, document chunk count, and a hash of its ordered chunk set.
- Knowledge tagged `reference` is excluded from the writing knowledge snapshot; edited-but-not-reindexed document copies are omitted.
- Evidence: `src/shared/generation-knowledge.ts:1-14`; `electron/services/generation-knowledge-source.ts:86-115,131-150`.
- **How:** Pin/read a stable source view → verify document completeness → verify vector hits match canonical text → capture selected identities → later re-read those exact IDs without rerunning search or embedding.
- Revalidation occurs around preparation/begin/dispatch/commit/resume boundaries; a short gate prevents concurrent canonical KB changes during the check/write boundary.
- Evidence: `electron/services/generation-knowledge-source.ts:155-169`; `electron/controllers/generation-controller.ts:159-168,193-200,256-269,281-286`; `electron/services/knowledge-source-gate.ts:13-30`.
- **Why:** [INFERENCE] A generated chapter cannot be saved as if it used a newly edited reference when it actually used an obsolete chunk.
- **Our comparison:** PARTIAL. Prepared revision application checks prose hash/revision, but our inspected context assembly lacks a comparable frozen manifest for all consumed memory sources.
- Ours: `internal/revision/service.go:209-234`; `internal/tools/novel_context_builders.go:446-508`.

#### 16. Chapter changes invalidate semantic suffixes without destroying recoverable originals

- **What:** Replacing an earlier finalized chapter or deleting it increments a project continuity generation and records the earliest stale chapter.
- Projections in the affected suffix from older generations become stale even when their own prose remains hash-valid.
- Evidence: `electron/repositories/finalization-repository.ts:271-272`; `electron/repositories/chapter-deletion-repository.ts:100-102`; `electron/repositories/summary-repository.ts:381-396,767-820`.
- **How:** Invalidate suffix → reject in-flight extractions frozen under an older generation → retain original prose → use stale projections only as locators → rebuild semantic projections explicitly when needed.
- Evidence commits recheck receipt identity, exact evidence inclusion, character associations, and generation inside the save transaction.
- Evidence: `electron/repositories/summary-repository.ts:584-612`; `docs/adr/0017-source-bound-character-state-and-continuity.md:7-9`.
- Candidate drafts also retain dependency hashes; candidate dependency chains are checked transitively, but author-finalized chapters seal their historical lineage.
- Evidence: `src/shared/draft-source-dependency.ts:1-17`; `electron/repositories/draft-repository.ts:174-201,203-268`.
- **Why:** [INFERENCE] Changing Chapter 3 cannot leave Chapter 20’s old semantic memory silently “current,” while confirmed Chapter 20 prose still remains available as authority.
- **Our comparison:** PARTIAL. We replay chapter projections and invalidate affected arc/volume summaries, snapshots, style rules, and reviews; no equivalent per-fact generation/source-span contract was found.
- Ours: `internal/revision/service.go:145-175`; `internal/store/revisions.go:39-89`; `internal/revision/projector.go:41-89`.

#### 17. Derived plot graphs and transfer metadata do not become a second story canon

- **What:** Plot trees are read-only visualization/navigation snapshots; they explicitly do not enter prose prompts or write back to plans/facts.
- Each event must reference an existing blueprint, finalized chapter, or confirmed narrative-thread source; planned/occurred status must agree with source type.
- Evidence: `docs/adr/0014-plot-tree-is-a-derived-snapshot.md:3-7`; `src/shared/plot-tree.ts:225-269`.
- **How:** Derive/validate a full snapshot → replace only on success → retain the prior successful snapshot on failure → mark refresh needed when sources differ, without automatic model work.
- Transfer adoption changes only proven continuity-epoch prefixes; values, chapter order, and source identities remain unchanged.
- Evidence: `docs/adr/0014-plot-tree-is-a-derived-snapshot.md:5-7`; `electron/services/transferred-character-continuity.ts:30-65,70-79`.
- **Why:** [INFERENCE] Visualization summaries and backup lineage cannot accidentally outrank the actual manuscript.
- **Our comparison:** EXISTS for canonical JSON plus one-way readable projections; PARTIAL for generation-bound transferred state.
- Ours: `docs/adr/0001-nao-truyen-du-lieu-goc-json.md:9-11`; `internal/store/nao_vault.go:40-64`.

### Learnable techniques table

| # | Technique | Source evidence | Ours (EXISTS/PARTIAL/NEW + citation) | Impact (H/M/L) | Effort (S/M/L) | Notes for Go port |
|---|---|---|---|---|---|---|
| 1 | Keep original prose authoritative over generated memory | `docs/adr/0017-source-bound-character-state-and-continuity.md:7-9` | PARTIAL — canonical records exist, but context directly injects summaries: `internal/domain/revision.go:95-108`; `internal/tools/novel_context_builders.go:468-478` | H | M | Add evidence-backed historical passages without replacing canonical JSON stores. |
| 2 | Freeze exact content and fact intent during commit recovery | `electron/services/finalization-service.ts:69-90,101-117` | EXISTS — frozen payload plus prose: `internal/tools/commit_chapter.go:58-64,108-119,219-226` | H | S | Reuse the existing saga; do not create another finalization protocol. |
| 3 | Extract latest corrections, modality, ownership and knower attribution | `src/services/builtin-prompt-templates.ts:1105-1111` | PARTIAL — actual-prose-only instruction: `assets/prompts/writer.md:64`; structured facts: `internal/domain/revision.go:21-37` | H | S | Write original Vietnamese instructions and add cancellation/suspicion/ownership regression cases. |
| 4 | Store exact evidence for each continuity assertion | `src/shared/finalized-continuity.ts:40-53`; `electron/repositories/summary-repository.ts:589-605` | NEW — current fact/state shapes lack spans: `internal/domain/revision.go:21-47`; `internal/domain/tracking.go:3-10` | H | M | Optional evidence reference: chapter revision/hash plus byte span and quote hash. |
| 5 | Let software locate unique quotes instead of asking models to count offsets | `src/shared/finalized-continuity.ts:175-199` | NEW — inspected state/event contracts contain no such locator: `internal/domain/tracking.go:3-10`; `internal/domain/review.go:3-25` | H | S | Use UTF-8 byte offsets in Go; reject ambiguous quotes or require more surrounding text. |
| 6 | Field-level author/derived/legacy state provenance | `src/shared/character-roster.ts:44-69`; `src/shared/character-identity.ts:65-93` | PARTIAL — state deltas/snapshots exist: `internal/domain/tracking.go:3-10`; `internal/store/characters.go:61-67` | H | M | Add provenance only where author-edited dynamic fields and extracted state can conflict. |
| 7 | Compare-and-swap state updates against captured field baseline | `electron/repositories/summary-repository.ts:428-465` | PARTIAL — chapter-level prepared hash checks: `internal/revision/service.go:209-234` | H | M | Extend existing revision checks, not a generic policy engine. |
| 8 | Preserve ambiguous character occurrences for confirmation | `src/shared/finalized-continuity.ts:209-222` | PARTIAL — stable IDs and exact aliases exist: `internal/store/nao_identity.go:52-70`; `internal/store/nao_test.go:65-70` | H | M | Stable ID first; unresolved identity stays unresolved, never fuzzy-merged. |
| 9 | Distinguish thread plan from evidence-backed occurrences | `src/shared/narrative-thread.ts:18-49`; `electron/repositories/narrative-thread-repository.ts:80-106` | PARTIAL — plant/advance/resolve facts exist: `internal/domain/review.go:11-25`; `internal/revision/projector.go:113-140` | H | M | Extend the ledger with author intent, evidence, target range, optional abandoned state. |
| 10 | Derive dormancy and overdue flags from event history | `electron/repositories/narrative-thread-repository.ts:141-148` | PARTIAL — 30-chapter aging refill: `internal/tools/novel_context.go:873-906,1048-1052` | M | S | Preserve aging fallback; add target-end and last-advanced timing. |
| 11 | Bound relevant active thread context | `src/services/workflows/commands/generate-draft.command.ts:72-73,2029-2046` | EXISTS — five-item recall: `internal/tools/novel_context.go:840-888` | M | S | Improve evidence richness, not another recall subsystem. |
| 12 | Include source paragraph plus neighbors | `src/services/workflows/chapter-materials.ts:345-365` | PARTIAL — related-chapter recall exists: `internal/tools/novel_context_builders.go:584-594` | H | M | Return paragraph windows from canonical chapter text, merging overlapping spans. |

[Showing lines 1-250 of 319. Use :251 to continue]
| 13 | Require full direct predecessor prose when capacity permits | `src/services/workflows/chapter-materials.ts:460-470,569-571` | PARTIAL — 800-rune tail only: `internal/tools/novel_context_builders.go:497-505` | H | M | Evaluate full predecessor versus tail plus targeted passages; do not copy fixed character budgets. |
| 14 | Emit included/omitted source receipts | `src/services/workflows/chapter-materials.ts:141-214` | PARTIAL — loading summary and item-level trimming: `internal/tools/novel_context.go:217-245,261-290`; `internal/tools/nao_context.go:136-195` | M | M | Keep current soft budget; add diagnostic evidence of what the model actually received. |
| 15 | Blueprint-stage dead-character warning with justified exemption | `src/shared/consistency-preflight.ts:78-118` | PARTIAL — post-fact deterministic dead-character lint exists: `internal/naolint/story.go:64-90` | M | S | Reuse lint logic against planned cast; warn, do not prohibit flashbacks. |
| 16 | Explicit same-day/relative-time attribution contract | `src/shared/chapter-time-continuity.ts:3-14` | PARTIAL — time strings and timeline checks exist: `internal/domain/review.go:3-8`; `assets/prompts/editor.md:36-40` | H | S | Add shared writer/editor rule; distinguish explicit gap from default continuation. |
| 17 | Optional paragraph/sentence-aware chunking with overlap | `electron/embedding.ts:346-414` | NEW for production semantic retrieval; W4 deferred: `docs/prd/nao-truyen.md:217-234` | M | M | Build only after retrieval evaluation justifies it; chunk authoritative text, retain offsets. |
| 18 | Version embedding spaces and validate response shape | `docs/adr/0004-versioned-project-embedding-space.md:3`; `electron/embedding.ts:52-70,81-128` | NEW — vector work intentionally deferred: `docs/prd/nao-truyen.md:218-234` | H if vectors enabled | M | Fingerprint model/config/dimension/metric; never mix incompatible spaces. |
| 19 | Preserve usable lexical retrieval when embeddings fail | `electron/knowledge-base.ts:98-118`; `electron/vector-store.ts:1469-1511` | NEW for this KB runtime; existing deterministic recall remains available: `internal/tools/novel_context.go:832-888` | M | M | Do not advertise SQL substring search as BM25 or hybrid fusion. |
| 20 | Freeze retrieval results by chunk/document identity and hash | `electron/services/generation-knowledge-source.ts:86-102,155-169` | PARTIAL — prose revision checks exist: `internal/revision/service.go:209-234` | H | M | Start with canonical file/record revision manifests before adopting a vector DB. |
| 21 | Invalidate downstream semantic suffix after earlier chapter change | `electron/repositories/summary-repository.ts:381-396,778-820` | PARTIAL — aggregates invalidated and facts replayed: `internal/store/revisions.go:39-89`; `internal/revision/service.go:145-155` | H | M | Add generation/staleness metadata where semantic dependence crosses chapter boundaries. |
| 22 | Verify candidate dependency chains, seal author-finalized authority | `electron/repositories/draft-repository.ts:174-201,203-268` | PARTIAL — local prepared revision CAS only: `internal/revision/service.go:209-234` | M | M | Useful if we add multi-chapter unsaved candidate batches; avoid unnecessary DAG machinery now. |
| 23 | Keep visual graphs derived and out of prose authority | `docs/adr/0014-plot-tree-is-a-derived-snapshot.md:3-7` | EXISTS — one-way JSON-to-vault projection: `docs/adr/0001-nao-truyen-du-lieu-goc-json.md:9-11`; `internal/store/nao_vault.go:40-64` | H | S | Preserve our existing canonical-data boundary. |

### Pitfalls / over-engineering in source we should NOT copy

1. **Do not mistake lexical evidence matching for semantic verification.**
   The fact builder uses bigram overlap and category regexes, and persistence only checks source inclusion.
   That proves a quote is present, not that it entails the generated statement; Pagekin’s own ADR makes this limitation explicit.
   Evidence: `src/shared/finalized-continuity-facts.ts:5-54`; `electron/repositories/summary-repository.ts:589-605`; `docs/adr/0017-source-bound-character-state-and-continuity.md:7`.

2. **Do not claim implemented BM25 or hybrid rank fusion from comments.**
   An FTS index is attempted, but the inspected search paths return vector hits directly and otherwise perform weighted substring matching.
   There is no observed BM25/vector blend, reciprocal-rank fusion, or cross-encoder reranker in these paths.
   Evidence: `electron/vector-store.ts:595-599,1436-1511`; `electron/services/generation-knowledge-source.ts:117-147`.

3. **Do not automatically port a vector database before proving retrieval quality.**
   Pagekin’s optional vectors are a capability, not demonstrated evidence that they outperform our selective memory for Vietnamese fiction.
   Our existing evaluation decision explicitly defers W4 until a larger corpus shows at least a five-point R@5 gain.
   Evidence for source optionality: `electron/knowledge-base.ts:98-118`; ours: `docs/prd/nao-truyen.md:230-234`.

4. **Do not copy hardcoded 12-fact, 12-source, six-thread caps as universal quality settings.**
   These are implementation bounds, not source-backed optimal values for our chapter lengths or language.
   A head-truncated notes list can exclude late corrections; a fixed thread cap can hide an overdue but lower-ranked thread.
   Evidence: `src/shared/finalized-continuity-facts.ts:3,87`; `src/services/workflows/commands/generate-draft.command.ts:72-73,1880,2031-2039`.
   The exclusion risk is [INFERENCE]; evaluate coverage rather than adopting these constants.

5. **Do not duplicate our store with a second character-memory authority.**
   Pagekin deliberately reuses `characters` and `summary_snapshots`; its plot tree and Markdown are projections.
   The transferable idea is provenance and source binding, not migrating our Go canonical store into Electron-shaped repositories.
   Evidence: `docs/adr/0007-structured-character-roster-fact-source.md:3`; `docs/adr/0017-source-bound-character-state-and-continuity.md:7`.

6. **Do not copy the prompt-overlay contradiction pattern.**
   The final character contract explicitly overrides an earlier full-field/new-character JSON example.
   That is avoidable ambiguity: our Go port should emit one coherent output contract with no obsolete example to override.
   Evidence: `electron/services/finalization-generation-source.ts:47-58`; `src/services/workflows/commands/finalize-chapter.command.ts:289-295`.

7. **Do not blindly import hard context-capacity failures.**
   Pagekin refuses generation when required material does not fit; our repository deliberately changed to soft overflow after repeated identical context requests stalled a real run.
   Preserve our current recovery behavior while adding source receipts, clear omission diagnostics, and targeted separate reads.
   Evidence: `src/services/workflows/chapter-materials.ts:93-113`; ours: `internal/tools/novel_context.go:217-251`.

8. **Do not treat dormancy as forgetting or force advancement every few chapters.**
   Pagekin computes dormant/overdue metadata; its rule does not prove the author wants every inactive thread advanced immediately.
   Our existing aging fallback appropriately supplies a reminder rather than changing canon.
   Evidence: `electron/repositories/narrative-thread-repository.ts:141-148`; ours: `internal/tools/novel_context.go:873-906,1048-1052`.

9. **Do not start automatic full-book semantic re-extraction after every edit.**
   Source design explicitly preserves hash-valid originals and falls back to them instead of automatically invoking models across the entire suffix.
   Our deterministic projection replay already provides a strong base; add targeted stale semantic markers, not an expensive parallel memory pipeline.
   Evidence: `docs/adr/0017-source-bound-character-state-and-continuity.md:7-9`; ours: `internal/revision/projector.go:41-89`.

10. **Do not apply permanent constraints to time-bound manual character state.**
    Author-entered location, injury, or possession describes a known point in the story, not an eternal prohibition on subsequent change.
    The state timestamp and explicit new prose should arbitrate evolution.
    Evidence: `docs/adr/0017-source-bound-character-state-and-continuity.md:5`; `src/services/workflows/chapter-materials.ts:519-523`.

**Recommended order:** original Vietnamese modality/time extraction rules → exact evidence references → historical prose neighborhoods/full-predecessor experiment → source manifests and suffix staleness → richer thread plans/events.
This ordering builds on our current records, replay, lint, and recall rather than introducing another memory service; embedding infrastructure should remain behind the existing evaluation gate.

Status: DONE_WITH_CONCERNS — All requested memory/continuity mechanisms were traced and compared with our implemented stores/context paths. The principal concern is that source documentation’s “BM25/hybrid” terminology overstates the inspected retrieval implementation; recommendations above distinguish actual behavior from design claims and inferred quality benefits.
