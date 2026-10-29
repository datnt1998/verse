## Slice: LLM layer, generation runtime robustness, agent

### Mechanisms

Paths beginning `internal/` below refer to `/Users/dat/Projects/ainovel-cli`.
All source evidence refers to `/tmp/xia-ainw`; comparisons are static code inspection, not executed verification.
Quality benefits described below are engineering inferences, not experimentally demonstrated improvements.
No files were edited, and no source code, builds, tests, or installation commands were executed.

#### 1. One parameter policy, but **not** stage-specific sampling temperatures

- The source centralizes request-parameter resolution instead of letting individual workflows choose arbitrary provider fields.
- Step 1: read temperature and operational output limit from the selected model profile.
- Step 2: resolve reasoning from product strategy and generation stage.
- Step 3: constrain requested output by both user limits and verified model limits.
- Step 4: produce one options object consumed by streaming and non-streaming provider paths.
- Important correction: temperature is profile-level, not drafting/planning/review-specific; the inspected OpenAI and Gemini payload builders do not send `top_p`.
- Official Kimi model families receive an omitted temperature field; older official-host Kimi configurations validate temperature within `[0,1]`.
- Evidence: `/tmp/xia-ainw/electron/llm/generation-parameter-policy.ts:24-49,81-113`; `/tmp/xia-ainw/electron/llm/openai-provider.ts:47-103`; `/tmp/xia-ainw/electron/llm/gemini-provider.ts:98-107`.
- Quality implication: fewer rejected requests and fewer accidental differences between visible generation, structured planning, and streaming execution.
- Ours already omits default temperature and clears injected sampling when explicit reasoning requires provider ownership; see `internal/llm/client.go:835-869`.

#### 2. Semantic writing strategies drive reasoning effort by stage

- The source separates creative intent from provider-specific thinking fields.
- `auto` requests low reasoning for drafting/general, medium for planning, and high for review.
- A drafting-oriented strategy requests thinking off for drafting and low elsewhere.
- Consistency-oriented and planning-oriented strategies spend more reasoning on planning/review while keeping drafting relatively light.
- A model-level explicit override takes precedence over the project strategy.
- Step 1: select strategy and stage; step 2: apply override; step 3: locate a verified mapping; step 4: translate into a provider directive.
- Evidence: `/tmp/xia-ainw/src/shared/reasoning-policy.ts:14-19,95-136`.
- Quality implication: reserve expensive deliberation for structural reasoning rather than forcing maximum thinking into every prose-generation call.
- Ours already supports role-level reasoning overrides and capability-aware effective values; the missing increment is a project-level semantic preset spanning roles/stages.
- Local evidence: `internal/bootstrap/config.go:553-559`; `internal/host/host.go:1749-1755`; `internal/agents/build.go:298-306,345-353`.

#### 3. Requested reasoning and effective reasoning are different facts

- The source records requested effort, effective effort, resolution status, and whether the choice came from project strategy or model override.
- It prefers documented aliases, then exact support, then the greatest supported effort not exceeding the requested rank.
- Gemini thinking-level models receive their lowest supported setting when “off” cannot be represented literally.
- Other unsupported combinations remain unsupported rather than inventing request parameters.
- Explicit integration exceptions prevent automatic low/medium requests from being silently escalated into high-only provider modes.
- Evidence: `/tmp/xia-ainw/src/shared/reasoning-policy.ts:66-87,108-136`; `/tmp/xia-ainw/src/shared/reasoning-types.ts:113-118`.
- Quality implication: operators can distinguish author intent from actual provider behavior instead of interpreting an unsupported switch as successful.
- Ours already preserves stored reasoning intent while clamping the effective path; see `internal/host/host.go:1749-1755` and `internal/agents/build.go:75-103`.

#### 4. Provider capability evidence is endpoint-specific

- The source distinguishes provider capacity evidence from user operational limits.
- Step 1: match protocol and exact official endpoint.
- Step 2: match an exact catalog model identifier.
- Step 3: resolve capacity separately from reasoning, structured-output, and usage features.
- Step 4: allow user settings to narrow capacity, never use a large user-entered number as proof of provider support.
- Explicit reasoning mappings are validated and checked for protocol compatibility before use.
- Evidence: `/tmp/xia-ainw/src/shared/provider-presets.ts:563-601,609-677`; `/tmp/xia-ainw/electron/llm/generation-parameter-policy.ts:58-78`.
- Quality implication: prevents optimistic context assumptions and unsupported thinking/JSON parameters when a familiar model name is served by a different gateway.
- Ours has detailed provider capability contracts, but the source’s explicit separation of verified endpoint evidence and operational caps is a useful additional provenance model.
- Local evidence: `internal/llm/capabilities.go:75-88,122-134`; `internal/host/model_config.go:153-157`.

#### 5. Provider adapters translate reasoning, rather than exposing wire fields to workflows

- OpenAI-compatible adapters support effort strings, thinking budgets, enable switches, and DeepSeek-specific thinking objects.
- These are finite directive variants, not an unrestricted map of model-generated provider options.
- Gemini uses either thinking budgets or thinking levels under `generationConfig.thinkingConfig`.
- Zero budget can disable thinking without sending an invalid numeric provider budget field.
- NovelAI receives a narrower compatibility payload, excluding unsupported reasoning/JSON/stream-usage controls.
- Evidence: `/tmp/xia-ainw/src/shared/reasoning-types.ts:31-66`; `/tmp/xia-ainw/electron/llm/openai-provider.ts:68-100`; `/tmp/xia-ainw/electron/llm/gemini-provider.ts:55-63`.
- Quality implication: product prompts remain provider-independent while the transport expresses the provider’s actual reasoning controls.
- Ours already has a provider-neutral thinking contract and provider-owned sampling handling; see `internal/llm/client.go:848-869`.
- This is primarily an EXISTS item, not a reason to introduce another adapter layer.

#### 6. Reasoning text and durable story text are separate channels

- OpenAI `reasoning_content` is delivered through a reasoning callback instead of becoming visible story text.
- Gemini parts marked `thought: true` take the same separate route.
- Visible content additionally passes through an incremental filter for in-band `<think>` markup.
- The filter holds partial tag prefixes between chunks and tracks nested thought depth.
- An unfinished thought or incomplete tag remains undisclosed instead of leaking a prefix into the candidate.
- Evidence: `/tmp/xia-ainw/electron/llm/openai-provider.ts:300-318`; `/tmp/xia-ainw/electron/llm/gemini-provider.ts:248-265`; `/tmp/xia-ainw/electron/llm/visible-stream.ts:9-41`.
- The main run service forwards reasoning as display-only events, whereas visible text follows the artifact snapshot path.
- Evidence: `/tmp/xia-ainw/electron/services/generation-run-service.ts:157-176,211-255`.
- Ours already separates protocol reasoning and text blocks; the missing defense is equivalent incremental handling when an OpenAI-compatible provider embeds thinking inside ordinary content.
- Local evidence: `internal/llm/client.go:300-327`; a scoped search found no equivalent `<think>` stream filter in `internal/{llm,agent,host}`.

#### 7. Transport completion is not semantic completion

- OpenAI finish reasons normalize into stop, length, content-filter, error, or unknown.
- Non-streaming success is returned only for explicit semantic stop.
- Streaming requires `[DONE]`; EOF without it produces an incomplete-stream failure while retaining the visible prefix.
- The streaming callback still carries normalized finish reason, so `[DONE]` with unknown or length does not by itself prove a complete creative result.
- Gemini normalizes STOP, MAX_TOKENS, and safety-related stops into the same application-level categories.
- Evidence: `/tmp/xia-ainw/electron/llm/openai-provider.ts:29-36,143-166,398-417`; `/tmp/xia-ainw/electron/llm/gemini-provider.ts:66-72`.
- The harness independently treats an omitted finish reason as unknown rather than successful.
- Evidence: `/tmp/xia-ainw/src/services/generation/generation-harness.ts:725-728`.
- Ours already rejects streams closed without a done event and bounds length-recovery calls; see `internal/agent/loop.go:312-334,390-397,823-841`.
- Quality implication: truncated prose cannot silently advance the book’s authoritative state merely because the socket closed normally.

#### 8. Preflight budgeting subtracts input, output, reasoning, and safety separately

- The main owner estimates every message’s UTF-8 bytes plus role/framing overhead before dispatch.
- It resolves model/user capacity, root remaining liability, reasoning accounting mode, and safety allowance.
- The planner computes output room after subtracting the entire required input and separately budgeted reasoning.
- Output reservation is the minimum of applicable provider, user, context, and root limits.
- Structured JSON mode is enabled only when the lease’s feature evidence supports it.
- Evidence: `/tmp/xia-ainw/electron/services/main-generation-plan.ts:255-298,310-326`; `/tmp/xia-ainw/src/services/generation/task-budget-planner.ts:274-310`.
- Quality implication: reduces late provider rejections and avoids accidentally allocating a long visible answer into a context already occupied by continuity material.
- Ours already budgets context and includes system/tool prefix overhead, but does not expose this same unified per-request liability decision.
- Local evidence: `internal/agent/context/engine.go:35-44,168-176,509-516`; `internal/host/budget.go:21-35`.

#### 9. Semantic scope splitting is explicit, not hidden input clipping

- The planner can size requests from requested draft units or structured-item counts.
- Its estimates depend on writing language and generation stage.
- It reports requested quantity, selected quantity, remaining quantity, requested output, and physical reservation.
- If a draft is non-segmentable, insufficient capacity becomes a conflict rather than a smaller silently accepted draft.
- A split-required result does not dispatch: the caller must rebuild a smaller semantic request while preserving required input bytes.
- Evidence: `/tmp/xia-ainw/src/services/generation/task-budget-planner.ts:4-40,170-198,335-392`; `/tmp/xia-ainw/electron/services/main-generation-plan.ts:288-290`.
- Important qualification: the current capacity-v2 path selects `physical-capacity`; this can allocate the available output ceiling rather than the semantic estimate.
- Evidence: `/tmp/xia-ainw/electron/services/main-generation-plan.ts:281-285`; `/tmp/xia-ainw/src/services/generation/task-budget-planner.ts:335-364`.
- Ours has input/output-aware import batching, making this PARTIAL rather than entirely NEW.
- Local evidence: `internal/host/imp/analyze.go:69-72,208-221`.

#### 10. Unknown provider consumption remains a liability

- The source models physical attempts as reserved, dispatch-marked, settled, unknown, or cancelled-before-dispatch.
- Only cancellation before dispatch contributes zero liability.
- A settled attempt with trusted actual consumption uses actual tokens; otherwise its reservation remains charged against the root.
- A transaction checks request count, token liability, per-request output, and active elapsed time before admitting another reservation.
- Actual usage is never clipped down to the estimate.
- Evidence: `/tmp/xia-ainw/src/shared/generation-contract.ts:41-57,68-93`.
- Usage settlement validates integer counts and arithmetic, distinguishing reasoning already included in completion from separately billed reasoning.
- Evidence: `/tmp/xia-ainw/electron/services/generation-run-service.ts:75-101`.
- Ours tracks cumulative cost and warns about missing usage, but the inspected paths lack equivalent durable physical-attempt reservations for unknown consumption.
- Local evidence: `internal/host/budget.go:40-46,68-91`; `internal/host/usage.go:61-65,437-481`.

#### 11. Partial output becomes a durable candidate, not a completed chapter

- Before dispatch, the run service reserves an attempt and creates/obtains its artifact.
- Visible output is append-only; conflicting duplicate events or a cumulative output that rewrites its prior prefix abort the stream.
- It flushes candidates every second and after approximately 4 KiB of newly visible text.
- Failures flush what can be saved; storage failure aborts generation and retains a copyable unsaved tail in memory.
- Reasoning never enters this candidate path.
- Evidence: `/tmp/xia-ainw/electron/services/generation-run-service.ts:131-183,201-209,223-256,272-311`.
- Candidate identity includes run/source identity, content hash, failure category, and lifecycle state.
- Evidence: `/tmp/xia-ainw/src/shared/recovery-candidate.ts:19-52`.
- Ours has durable message commits and artifact checkpoints, but partial LLM streams are intentionally rejected from conversation history; a separate user-recoverable visible-candidate store is the missing mechanism.
- Local evidence: `internal/agent/options.go:158-163`; `internal/agent/loop.go:831-841`; `internal/store/checkpoints.go:18-21,51-53`.

#### 12. Candidate promotion requires additional evidence

- Source candidate preservation is separate from formal draft commit.
- Commit checks the expected composition hash, generation target, and current source binding.
- The last contributing attempt must have semantic stop.
- The composed visible text must meet the configured minimum target-unit range.
- Batch progression also checks expected chapter order and lineage.
- Evidence: `/tmp/xia-ainw/electron/services/generation-draft-effects.ts:125-149,162-173`.
- Quality implication: preservation of useful interrupted text does not weaken the definition of a finished chapter.
- Ours already uses domain tools and explicit terminal-tool boundaries, so this should extend existing draft/commit behavior rather than introduce a second chapter-authority path.
- Local evidence: `internal/agents/build.go:298-313`; `internal/tools/commit_chapter.go:383-396,853-865`.

#### 13. Model execution leases freeze configuration; they are not concurrency semaphores

- A lease stores an immutable model-profile snapshot in the main process.
- The public receipt contains model identity, configuration revision, endpoint fingerprint, capability evidence, and expiration, without the secret profile.
- The revision covers temperature, output limits, reasoning settings, purposes, and embedding options.
- Resolve checks expiration and returns a clone of the frozen profile.
- This prevents live settings changes from mutating the model underneath an existing generation stage.
- Evidence: `/tmp/xia-ainw/electron/services/model-execution-lease.ts:125-166,184-213`.
- Crucial distinction: this registry does not limit concurrent model requests; it provides identity and snapshot ownership.
- Duplicate-call protection instead uses run + invocation nonce + request hash, returning the existing pending or durable receipt.
- Evidence: `/tmp/xia-ainw/electron/services/generation-run-service.ts:112-121`; `/tmp/xia-ainw/electron/services/main-generation-owner.ts:613-620`.
- Ours has engine/exclusive-operation concurrency guards, but no equivalent configuration-revision lease in the inspected runtime paths; see `internal/host/engine.go:97-104` and `internal/host/host.go:527-565`.

#### 14. Resume revalidates the frozen source instead of trusting old UI state

- Resume reconstructs a binding using the current model receipt and current project sources.
- It obtains a new execution lease and checks that its safe receipt matches the intended model receipt.
- Source-current checks rebuild the original binding and compare it with the persisted binding.
- A changed source or model blocks reuse instead of silently merging old generated text with new facts.
- Evidence: `/tmp/xia-ainw/electron/services/main-generation-owner.ts:670-692,710-724`.
- Agent recovery likewise checks source status before advancing to another round.
- Evidence: `/tmp/xia-ainw/electron/services/agent-generation.ts:156-173,180-207`.
- Ours already uses fingerprints for important domain operations; generalizing that proof to recoverable generation candidates remains useful.
- Local evidence: `internal/tools/audit_foundation.go:43-46,76-77`; `internal/store/checkpoints.go:51-53,94-109`.

#### 15. Retry robustness is mainly replay safety, not an elaborate source backoff engine

- The inspected providers perform one `fetch` per invocation; failures surface as normalized incomplete/error outcomes.
- The run service executes one physical dispatch, records its outcome, and deduplicates repeated invocation identities.
- The hosted agent refuses to silently redispatch an existing returned or unknown round, including after restart.
- Evidence: `/tmp/xia-ainw/electron/llm/openai-provider.ts:118-168,186-201`; `/tmp/xia-ainw/electron/services/generation-run-service.ts:112-121,211-270`; `/tmp/xia-ainw/electron/services/agent-generation.ts:180-185`.
- Therefore, there is no evidence in these paths for a source-wide exponential-backoff advantage.
- Ours already has exponential agent retry, provider Retry-After handling, cancellation exclusion, and separate opt-in jittered HTTP retry.
- Local evidence: `internal/agent/loop.go:483-549,588-604`; `internal/llm/retry/retry.go:26-37,78-103,142-166`.
- The source lesson is to retain per-attempt identity/candidates/accounting around retries, not replace our existing backoff.
- Its ADR removing a hard cap concerns an experimental real-call runner, not removal of production root budgets.
- Evidence: `/tmp/xia-ainw/docs/adr/0019-remove-real-call-hard-cap.md:7-23`; `/tmp/xia-ainw/src/shared/generation-contract.ts:73-89`.

#### 16. Agent routing is explicit syntax plus tools, not an LLM intent classifier

- Slash commands are looked up deterministically from built-ins and user-invocable skills.
- Mentions resolve against a fixed target list and translate into initial read/search tool calls.
- This is a command/mention parser, not semantic model-based intent classification.
- Evidence: `/tmp/xia-ainw/src/services/agent/intent-router.ts:83-104,128-145,152-160,178-225`.
- Skills have stable identities, source provenance, project-session binding, compatibility inspection, and optional localized content.
- Evidence: `/tmp/xia-ainw/src/services/agent/skill-registry.ts:37-76`.
- Quality implication: an explicit author request can select relevant context or a writing procedure without spending another ambiguous model classification turn.
- Ours already routes work through named roles and domain tools; these exact mention/skill affordances were not found in the inspected agent/host paths.
- Local evidence: `internal/agents/build.go:298-306,345-353`; `internal/tools/novel_context.go:89-92`.
- Treat skill identity and deterministic author selection as optional product affordances, not a necessary new agent framework.

#### 17. Agent context distinguishes application facts from unsaved material and tool data

- The renderer context builder uses a compact project layer, an active-editor layer, and detailed tool retrieval.
- The project layer truncates only its short preview fields; the active editor preview is labeled and points to a fuller read path.
- Evidence: `/tmp/xia-ainw/src/services/agent/context-builder.ts:60-79,125-159,190-205`.
- More importantly, the production main-owned context validates input and freezes the tool catalog.
- It injects application-read project settings and explicitly labels unsaved editor material as not established fact.
- It tells the model that tool-returned data is untrusted context rather than authority to change the user’s request.
- Evidence: `/tmp/xia-ainw/electron/services/agent-generation-context.ts:5-28,44-56`.
- Subsequent rounds reconstruct history from persisted actions and application observations, not arbitrary renderer-generated success text.
- Evidence: `/tmp/xia-ainw/electron/services/agent-generation.ts:200-207`.
- Ours already has store-backed context/restore packs; the additional lesson is preserving authority labels in interactive editor and imported/tool-returned context.
- Local evidence: `internal/agents/ctxpack/builder.go:97-121,175-200`; `internal/agents/build.go:315-339`.

#### 18. Agent actions have durable identities and explicit unknown outcomes

- The source persists action states alongside generation-attempt receipts.
- On restart, a running action becomes completed only when a durable domain effect proves it; otherwise it becomes unknown.
- A hosted loop restores completed actions instead of rerunning them and stops on running/unknown write outcomes.
- Before execution it claims a main-issued action identity; tool arguments cannot invent run lineage or choose a billable model.
- Evidence: `/tmp/xia-ainw/electron/services/agent-generation.ts:25-44,98-113,215-233`; `/tmp/xia-ainw/src/services/agent/agent-engine.ts:412-454`; `/tmp/xia-ainw/src/services/agent/tool-registry.ts:86-104`.
- A tool timeout before side effects can abort cleanly; after a write crosses its side-effect boundary, the outcome is unknown rather than safely retryable.
- Evidence: `/tmp/xia-ainw/src/services/agent/agent-engine.ts:467-480,524-538`.
- Domain writes and workflow launches must use real application seams and return actual receipts, not merely create similarly named Markdown files.
- Evidence: `/tmp/xia-ainw/docs/adr/0008-agent-actions-use-domain-fact-and-workflow-seams.md:1-3`; `/tmp/xia-ainw/src/services/agent/tools/start-workflow.tool.ts:79-105`.
- Ours already validates, previews, gates, and durably commits assistant messages before tool execution; universal durable action claim/unknown-write reconciliation is still a meaningful increment.
- Local evidence: `internal/agent/loop.go:483-488,913-975`; `internal/agent/options.go:158-163`.

### Learnable techniques table

Impact measures likely AI-output reliability/quality benefit; effort is relative implementation scope, not a time estimate.
`NEW` means no equivalent was found in the inspected local paths; the cited local mechanism is the nearest existing integration point.

| # | Technique | Source evidence | Ours (EXISTS/PARTIAL/NEW + citation) | Impact (H/M/L) | Effort (S/M/L) | Notes for Go port |
|---|---|---|---|---|---|---|---|
| 1 | Single sampling/parameter resolution seam | `/tmp/xia-ainw/electron/llm/generation-parameter-policy.ts:81-113` | EXISTS — `internal/llm/client.go:835-869` | M | S | Keep provider-owned omission; do not infer stage-temperature tuning from this source. |
| 2 | Project creative strategy → stage reasoning | `/tmp/xia-ainw/src/shared/reasoning-policy.ts:14-19,95-111` | PARTIAL — role overrides exist in `internal/bootstrap/config.go:553-559`; project strategy presets missing | H | S | Map semantic presets onto architect/writer/editor effective settings while preserving explicit overrides. |
| 3 | Requested/effective reasoning receipt | `/tmp/xia-ainw/src/shared/reasoning-policy.ts:126-136` | PARTIAL — clamping exists in `internal/host/host.go:1749-1755`; richer resolution status is the increment | M | S | Expose mapped/capped/forced/unsupported without rewriting stored intent. |
| 4 | Endpoint-scoped capability provenance | `/tmp/xia-ainw/src/shared/provider-presets.ts:574-601,609-677` | PARTIAL — `internal/llm/capabilities.go:75-88,122-134`; capacity provenance is less explicitly separated | H | M | Keep verified provider facts distinct from author operational ceilings. |

[Showing lines 1-250 of 345. Use :251 to continue]
| 5 | Provider-neutral reasoning directives | `/tmp/xia-ainw/src/shared/reasoning-types.ts:35-66` | EXISTS — `internal/llm/client.go:848-869` | M | S | Extend existing thinking contracts only for genuinely unsupported mappings. |
| 6 | Protocol reasoning/text separation | `/tmp/xia-ainw/electron/llm/openai-provider.ts:300-318` | EXISTS — `internal/llm/client.go:300-327` | H | S | Preserve typed content blocks rather than stringify thinking into prose. |
| 7 | Split-chunk in-band thinking quarantine | `/tmp/xia-ainw/electron/llm/visible-stream.ts:9-41` | NEW — nearest `internal/llm/client.go:317-327` appends content deltas as text | H | M | Add a provider-compatible incremental filter before story/artifact consumers; define literal-tag behavior carefully. |
| 8 | Explicit semantic completion evidence | `/tmp/xia-ainw/electron/llm/openai-provider.ts:143-166,398-411` | EXISTS — `internal/agent/loop.go:823-841,312-334` | H | S | Keep missing terminal evidence distinct from completed output. |
| 9 | Unified input/output/reasoning preflight | `/tmp/xia-ainw/electron/services/main-generation-plan.ts:255-298` | PARTIAL — `internal/agent/context/engine.go:168-176,509-516`; root liability integration missing | H | M | Return a numeric decision receipt before physical dispatch. |
| 10 | Semantic demand and explicit splitting | `/tmp/xia-ainw/src/services/generation/task-budget-planner.ts:335-392` | PARTIAL — import batching in `internal/host/imp/analyze.go:208-221`; not a shared generation contract | H | M | Extend existing planners; do not silently shorten required chapter scope. |
| 11 | Durable per-attempt liability reservations | `/tmp/xia-ainw/src/shared/generation-contract.ts:68-93` | NEW — nearest cost sentinel `internal/host/budget.go:68-91` operates on observed cumulative cost | H | L | Reserve before dispatch; retain liability for unknown consumption; reconcile trusted actuals. |
| 12 | Visible recovery candidates independent of history | `/tmp/xia-ainw/electron/services/generation-run-service.ts:157-183,253-277` | PARTIAL — durable messages/checkpoints exist in `internal/agent/options.go:158-163`, `internal/store/checkpoints.go:51-53`; incomplete text lacks this separate path | H | M | Persist non-authoritative candidates without replaying malformed assistant/tool messages. |
| 13 | Unsaved-tail salvage on storage failure | `/tmp/xia-ainw/electron/services/generation-run-service.ts:165-175,307-311` | NEW — nearest durable commit stops execution in `internal/agent/options.go:158-163` | M | S | Abort further spending and expose a copyable in-memory tail separately from durable text. |
| 14 | Candidate promotion guards | `/tmp/xia-ainw/electron/services/generation-draft-effects.ts:125-149` | PARTIAL — domain commit/terminal boundaries in `internal/agents/build.go:298-313`, `internal/tools/commit_chapter.go:383-396` | H | M | Require matching source/composition hash and complete generation evidence before promotion. |
| 15 | Immutable model/config execution receipt | `/tmp/xia-ainw/electron/services/model-execution-lease.ts:125-166,184-208` | NEW — nearest model hot-application path `internal/host/model_config.go:269-285` | M | M | Snapshot per stage/run; record non-secret revision and endpoint identity. |
| 16 | Idempotent invocation identity | `/tmp/xia-ainw/electron/services/generation-run-service.ts:112-121` | PARTIAL — host exclusivity exists in `internal/host/host.go:527-565`; durable per-call nonce replay is different | H | M | Deduplicate run/nonce/hash before allocating another paid attempt. |
| 17 | Source-bound recovery checks | `/tmp/xia-ainw/electron/services/main-generation-owner.ts:670-692,710-724` | PARTIAL — domain fingerprints in `internal/tools/audit_foundation.go:76-77`, artifact hashes in `internal/store/checkpoints.go:94-109` | H | M | Bind candidates to exact source revisions; block stale continuation/promotion. |
| 18 | Backoff and Retry-After | `/tmp/xia-ainw/electron/services/generation-run-service.ts:211-270` | EXISTS, stronger locally — `internal/agent/loop.go:483-549,588-604`, `internal/llm/retry/retry.go:142-166` | M | S | No source backoff advantage found; wrap our retries with attempt accounting instead. |
| 19 | Explicit mention/skill routing | `/tmp/xia-ainw/src/services/agent/intent-router.ts:128-145,178-225` | NEW for this exact affordance — nearest named-role/domain routing `internal/agents/build.go:298-306,345-353` | M | M | Optional deterministic user syntax; do not introduce a model classifier unnecessarily. |
| 20 | Facts versus unsaved/untrusted context labels | `/tmp/xia-ainw/electron/services/agent-generation-context.ts:44-56` | PARTIAL — store context/restore exists in `internal/agents/ctxpack/builder.go:175-200`, `internal/agents/build.go:315-339` | H | S | Label author draft/editor context and retrieved observations without promoting them to canon. |
| 21 | Durable action claim and unknown-write state | `/tmp/xia-ainw/electron/services/agent-generation.ts:25-44,215-233`; `/tmp/xia-ainw/src/services/agent/agent-engine.ts:467-480` | PARTIAL — validation/preview/gates exist in `internal/agent/loop.go:913-975`; universal durable action reconciliation missing | H | L | Use persistent action IDs and commit receipts, not blanket tool retries. |
| 22 | Domain tools report actual workflow receipts | `/tmp/xia-ainw/src/services/agent/tools/start-workflow.tool.ts:79-105` | EXISTS — authoritative commit and terminal boundaries in `internal/tools/commit_chapter.go:383-396`, `internal/agents/build.go:307-313` | H | S | Preserve our existing domain seams; never equate creating a file with committing story facts. |
| 23 | Safe phase-level transport diagnostics | `/tmp/xia-ainw/electron/llm/transport-diagnostics.ts:12-27,49-71,92-95` | PARTIAL — missing-usage diagnostics exist in `internal/host/usage.go:61-65`; transport phase receipt is the increment | M | S | Record response/output timing, safe error code, and reasoning/visible activity without prompt/body leakage. |

### Pitfalls / over-engineering in source we should NOT copy

- **Do not claim stage-specific temperature optimization.**
  The source fixes temperature at model-profile scope and varies reasoning by stage instead.
  Evidence: `/tmp/xia-ainw/electron/llm/generation-parameter-policy.ts:81-113`; `/tmp/xia-ainw/src/shared/reasoning-policy.ts:14-19`.

- **Do not treat semantic token coefficients as measured quality laws.**
  The planner uses language/stage coefficients and explicitly describes them as sizing, not mathematical tokenizer bounds.
  Keep our actual-token feedback and use independently calibrated coefficients if we adopt demand planning.
  Evidence: `/tmp/xia-ainw/src/services/generation/task-budget-planner.ts:4-24,170-198`.

- **Do not transplant the source’s XML tool-call compatibility parser.**
  It accepts tagged JSON, raw envelopes, legacy tagged forms, and embedded JSON.
  This increases ambiguity and requires special interrupted-output quarantine; our native tool-call/content-block path is preferable.
  Evidence: `/tmp/xia-ainw/src/shared/agent-response-protocol.ts:16-67,134-160`; local `internal/llm/contracts.go:47-50` and `internal/agent/loop.go:815-819`.

- **Do not mistake its one-call prompt instruction for enforced one-call execution.**
  The tool prompt requests at most one tool block, but the run service accepts parsed arrays up to 64 calls.
  If we want a real action limit, enforce it as a contract rather than relying on prompt wording.
  Evidence: `/tmp/xia-ainw/src/services/agent/tool-registry.ts:239-245`; `/tmp/xia-ainw/electron/services/generation-run-service.ts:262-266`.

- **Do not replicate dual visibility paths.**
  The OpenAI adapter still has a path that wraps protocol reasoning into `<think>` text and a separate visible-only path.
  That broadens the number of representations every consumer must handle; preserve one typed internal representation locally.
  Evidence: `/tmp/xia-ainw/electron/llm/openai-provider.ts:314-340,404-411`.

- **Do not assume all source provider paths have equivalent safeguards.**
  Gemini’s non-streaming path selects only the first text part; streaming joins non-thought parts and filters in-band thinking.
  `[INFERENCE]` Multipart non-streaming responses could lose later text or diverge from streaming privacy behavior.
  Evidence: `/tmp/xia-ainw/electron/llm/gemini-provider.ts:132-147,248-265`.

- **Do not duplicate source authority checks across independent budgeting frameworks.**
  The source has renderer harness planning plus main-owned physical planning with overlapping estimates and limits.
  Retain the idea of an authoritative preflight seam, not every compatibility/fallback layer.
  Evidence: `/tmp/xia-ainw/src/services/generation/generation-harness.ts:598-658`; `/tmp/xia-ainw/electron/services/main-generation-plan.ts:255-326`.

- **Do not confuse configuration leases with resource scheduling.**
  The model lease registry stores snapshots and expiration; it contains no capacity semaphore or request queue.
  Our existing engine/exclusive-job guards solve a different problem and should remain distinct.
  Evidence: `/tmp/xia-ainw/electron/services/model-execution-lease.ts:169-213`; local `internal/host/host.go:527-565`.

- **Do not globally truncate every tool observation at an arbitrary character count.**
  The hosted engine truncates successful observations to 3,000 characters and suggests file rereading.
  `[INFERENCE]` Important facts near the end can disappear, especially when the result is not actually a rereadable file.
  Evidence: `/tmp/xia-ainw/src/services/agent/agent-engine.ts:452-459,549-557`.

- **Do not cargo-cult eight tool rounds or thirty-second tool timeouts.**
  These are source product limits, not evidence-backed optimums for novel writing.
  The valuable distinction is before-commit cancellation versus after-commit unknown outcome.
  Evidence: `/tmp/xia-ainw/src/services/agent/agent-engine.ts:29-36,524-538`.

- **Do not interpret ADR 0019 as permission to remove production cost controls.**
  It removes an experimental runner’s global request cap while explicitly preserving physical-call accounting.
  Production generation still enforces finite root request/token/time budgets.
  Evidence: `/tmp/xia-ainw/docs/adr/0019-remove-real-call-hard-cap.md:14-23,34-37`; `/tmp/xia-ainw/src/shared/generation-contract.ts:73-89`.

- **Do not copy GPL implementation or verbatim prompt prose into our Apache-2.0 project.**
  The actionable deliverable is independently implemented mechanisms: stage reasoning intent, visible-candidate recovery, source identity, and durable action outcomes.

### Recommended extraction priorities

1. **First: visible-only interrupted candidates plus source/composition identity.**
   This most directly preserves expensive useful writing without weakening our complete-message or chapter-commit contracts.
2. **Second: incremental in-band thinking quarantine.**
   Our typed protocol separation already exists; close the ordinary-content escape hatch rather than rewriting the provider layer.
3. **Third: semantic reasoning presets over our existing role overrides.**
   This is a small product-level addition; keep requested and effective settings distinct.
4. **Fourth: unified preflight and durable physical-attempt accounting.**
   Reuse our context engine, import batching, usage tracker, and budget sentinel; add reservations rather than a parallel cost subsystem.
5. **Fifth: persistent action claims and unknown-write reconciliation.**
   Retain domain tool authority; extend replay safety around lost receipts and restarts.
6. **Keep our existing backoff, typed tool calls, context compaction, and role-based agents.**
   These already cover much of the source’s basic runtime capability and should not be replaced merely for architectural similarity.

Status: DONE_WITH_CONCERNS — Completed the read-only runtime/LLM/agent comparison. The strongest learnings are recoverable visible candidates, explicit source-bound replay safety, reasoning strategy receipts, and conservative physical-attempt accounting; source layering and several provider/protocol inconsistencies should not be transplanted.
