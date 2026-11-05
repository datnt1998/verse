# V38 backend final review

Status: DONE

## Summary

No verified consumer-visible blocker found in the assigned snapshot/admission, image-QA, retained-image, and historical-version backend scope. This is a source review, not whole-V38 acceptance. Read phase05, backend-contracts, `reports/pr-04/{qa-instruction-review,integration-state}.md`, the assigned source/test files, and directly relevant snapshot/locking/approval helpers. No edits to product/index, tests, builds, vet, formatters, servers, browser, or provider calls were performed.

References below are relative to `apps/viderse/` unless identified as plan reports.

### Snapshot and default-template race

- `internal/persistence/postgres/clone.go:449–458,544–578` reads default and instructions from the locked channel row, rejects a stale implicit template, takes the template revision fence, and inserts the captured instructions with the job in the same transaction. `queries/channels.sql:73–82,98–120` provides `FOR UPDATE` on the channel, `FOR SHARE` on the template, and atomic per-recipient instruction merge/delete; channel PATCH therefore participates in the same row serialization.
- `cmd/viderse/clone_wiring.go:128–158` returns the selected template's snapshot and revision together. `internal/library/snapshot.go:74–102,119–142` derives recipe fields and revision from the retrieved template. `queries/library.sql:38–47` increments revision when changing recipe. `internal/pipeline/service.go:104–122` re-resolves once only on the admission-fence error; explicit override remains explicit.
- The race assertions are behavior-bearing: `channel_admission_pg_test.go:41–101` observes an actual PostgreSQL blocker, commits B, demands stale-A rejection and absence of a persisted stale job, then verifies B. Lines 103–130 check unchanged explicit queue data, override A, old-job immutability, and recipe-revision rejection. Lines 261–333 exercise real service/store retry after B commits before admission; the test resolver is synthetic, so this is not proof of every production snapshot dependency.
- `qa_instruction_snapshot_test.go:27–63` admits old/new QA notes, deletes the channel's QA instruction key, then uses fresh repositories to verify both persisted job maps. This tests actual DB readback rather than echoing a returned map. It deletes the instruction, not the channel itself; no channel-deletion claim is inferred from the test name.

### QA applicability, score masking, manual review

- `internal/autonomy/scorer.go:183–231` uses the instruction-specific schema, rejects missing applicability, returns an `InstructionIssue` on contract failure, and suppresses a numeric score for false applicability or an outstanding issue. Image-only bounds are explicit at lines 121–126, consistent with accepted design #88; no audio implementation is required or claimed.
- `internal/pipeline/steps/images/handler.go:329–332,610–629` consumes the job's snapshotted QA instruction, not current channel state. Scoring failure/no scorer leaves no passing score and supplies a manual-review issue when a note exists.
- `internal/clone/images/autoreview.go:23–33` masks even a persisted 100 score when `ScoreIssue` is present. `internal/autonomy/autonomy.go:66–87` treats missing scores as below threshold; `run.go:41–66` stops before approval. `internal/clone/images/service.go:478–518` retains the explicit owner approval path without requiring a QA score.
- `internal/autonomy/qa_instruction_test.go:55–109` uses fixed, non-request-echoing remote responses to exercise real schema/parser/applicability/weighted-score behavior, including malformed/missing fields and false-with-100. `internal/clone/images/qa_instruction_test.go:48–143` invokes the real manifest reader, dispatcher, projections and owner review: it asserts the failing item's score is masked, no automatic commit/draft occurs, the other item's score survives, and manual approval preserves version history. Only persistence/remote boundaries are replaced; these are not assertions over an echoed mocked verdict.

### Paid image retention and historical consumers

- `internal/pipeline/steps/images/handler.go:579–660` builds a ready paid version before scoring, retains receipt/media metadata on scoring failure, then publishes bytes and checkpoint together. `pipeline/steps/images/qa_instruction_test.go:68–107` reads the serialized checkpoint, checks the previous version is unchanged, verifies the new version's ready state/receipt/cost and acquired stored bytes, and checks owner-review readiness. The drawer/bytes are synthetic (`"webp bytes"`), not real-provider image proof.
- Both backend representations now carry per-version score and issue: `internal/clone/images/types.go:128–130,215–216`; `service.go:230–264` and `versions.go:117–126` independently mask an issue-bearing version's score. Item/video summaries deliberately use `model.Displayed` (candidate first, otherwise current: `pipeline/steps/images/model.go:291–309`), not a UI history selection (`service.go:277–311`). Historical prompt/sent text, receipt-derived provider, URL and issue remain attached to their own version (`versions.go:70–126`).
- `versions.go:338–390` allows an explicit owner selection of a ready historical version; it does not turn a temporary history view into an automatic approval decision. No backend path inspected borrows the current version's passing score for a historical DTO.

## Concerns / Blockers and evidence limits

- **No verified new blocker in this scope.** Prior execution is recorded in plan reports `reports/pr-04/integration-state.md:11–14` and `qa-instruction-review.md:7–13`; the assignment reports real-PG fullbackend and vet already passed. I did not rerun or independently certify those executions.
- Applicability remains a model judgment. The fixed-response regressions prove fail-closed handling of the returned contract, not that a real model always recognizes an unsupported/ambiguous natural-language instruction. No real model/image-scoring or audio validation was performed; this is the documented limit, not a verified semantic failure.
- The service race regression substitutes its template resolver; production resolver wiring was inspected, not executed here. The instruction DB regression is sequential around admissions, while shared channel-row locking is source-backed; it is not a separate concurrent QA-note barrier proof.
- The historical PC/mobile fail-before/pass-after evidence and warning screenshots are reported by the parent integration reports, not re-observed here. Warning response overrides and synthetic media are not native backend/provider proof. Native real-image/built-in prompt sample coverage was not exercised; #90's genuine artwork-or-neutral-placeholder decision is unchanged.
- Unavailable LSP was not retried; scoped literal searches and source ranges were used. This report does not approve unrelated frontend, production migration/maintenance, media completeness, or release gates.
