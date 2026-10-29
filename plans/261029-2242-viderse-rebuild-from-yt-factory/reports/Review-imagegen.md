# Code Review Summary — paid image generation

Stateful review of the pending implementation; not an evergreen specification. Production and test code were not modified. Only this requested report was written.

## Scope

- Reviewed all 17 Go files under `apps/viderse/internal/imagegen/`, migration `00002_image_receipts.sql`, `postgres/queries/imagegen.sql`, `postgres/imagegen.go`, and `postgres/imagegen_pg_test.go`: 21 scoped files, 5,178 lines including tests (2,827 production/schema/query lines).
- Also traced generated `postgres/gen/imagegen.sql.go`, the shared Codex isolation/catalog/argv implementation and process runner, persistence test provisioning, and external callsites.
- Compared the three requested TS sources at **yt-factory commit `2c201e6`**, not its working tree. Read `reports/ImageGen.md`, root `AGENTS.md`, and `plan.md` section “Cấu trúc mã”.
- Scout-first risks: the per-key mutex belongs to one Service instance; SQL uses MAX+1 without a logical-key claim; count and reservation are separate; persistence errors share the provider-failure fallback route; busy/refused errors cross an embedding boundary; the real runner has a different framing bound from the parser.
- Applied the base checklist in two passes: blocking correctness/trust-boundary defects, then informational structure/documentation defects. This scope introduces no HTTP endpoints, so endpoint-specific auth checks are not applicable.

## Overall assessment

**Do not enable unattended paid execution yet.** Three critical admission/finalization defects permit duplicate paid work or spending past the cap. Seven high-priority issues affect ambiguous retries, charge accounting, secret handling, and real-provider output handling. Three medium-priority findings concern midnight reset, dependency boundaries, and inaccurate implementation-report claims.

Source facts below were inspected directly. **[INFERENCE]** marks predicted fault/concurrency behavior derived from that control flow; those scenarios were not fault-injected or run as additional tests. The existing authorized suite passed, but does not establish these failure-path guarantees.

## Critical issues — blocking

### C1. MAX+1 is not a cross-process idempotency claim

**Evidence:** `apps/viderse/internal/imagegen/service.go:143-155,173-177,186-210,413-419`; `apps/viderse/internal/persistence/postgres/queries/imagegen.sql:8-14,24-27`; `apps/viderse/internal/persistence/migrations/00002_image_receipts.sql:23`; `apps/viderse/internal/persistence/postgres/imagegen_pg_test.go:172-184`.

The mutex is stored on one Service, while Latest and Begin are separate operations. The DB primary key protects `(draw_key, attempt)`, not exclusive ownership of the logical draw key. Begin always chooses MAX+1, even when an earlier attempt is running or already successful. The integration test explicitly permits two sequential Begin calls without Finish; that proves bookkeeping, not idempotency.

**[INFERENCE] Trigger/impact:** A and B, in different processes or different Service instances, both settle the same empty key. A commits attempt 1; B then executes Begin, observes MAX=1, creates attempt 2, and both call the paid provider. Atomic output publication stops an overwrite only *after* both requests can have been charged. If their initial INSERTs collide instead, the losing DB error can enter fallback at `service.go:486-506`, creating another paid attempt there. Finish also lacks a prior-state/fence predicate, so a stale owner can overwrite a recovery/abandon decision.

**Fix:** claim the logical key durably before settling and retain/fence that ownership across the complete retry/fallback chain. Recheck the latest state under that claim; permit later attempts only as an owned transition. Fence Finish and check affected rows. Merely adding a unique running-attempt index is insufficient if another caller can enter between a failed primary and its fallback. Add a barrier-controlled two-Service/Postgres test that proves exactly one provider invocation, including the failed-primary/fallback interval.

### C2. Daily cap admission is a read-check-insert race

**Evidence:** `apps/viderse/internal/imagegen/service.go:389-419`; `apps/viderse/internal/persistence/postgres/queries/imagegen.sql:8-14,42-44`; `apps/viderse/internal/imagegen/service_test.go:251-285`.

Charged is queried before Begin, without a shared provider/day transaction or reservation lock. The per-key lock does not serialize different keys. The cap test submits requests sequentially.

**[INFERENCE] Trigger/impact:** with 49 charged attempts and limit 50, two draws on different keys can both read 49, both insert their conservative charged reservation, and both spend. This affects goroutines in the same Service as well as separate processes. The overshoot is bounded only by admitted concurrency, not by the configured cap; limit 1 is not a one-paid-call safety gate under concurrency.

**Fix:** make provider/day admission and charged reservation one atomic store operation, serialized across processes, and return the cap refusal from that operation. Preserve conservative in-flight/unknown reservations. Test concurrent distinct keys with exactly one slot remaining and assert the paid-call count, not just the final receipt count.

### C3. A receipt-write failure after successful generation deletes the good image and spends again

**Evidence:** `apps/viderse/internal/imagegen/service.go:419-431,483-506`; `apps/viderse/internal/imagegen/codex.go:405-420`; `apps/viderse/internal/imagegen/grok.go:356-369`.

A provider can successfully publish the PNG and return a receipt, then Finish can fail. drawWith discards that successful receipt and returns the storage error as though generation failed. drawOnce uses only the output existence snapshot from *before* the draw, removes the now-valid PNG, and invokes the fallback. A sidecar-write error after PNG publication has the same effect. Storage/pre-admission errors are not distinguished from provider failures at all.

**[INFERENCE] Trigger/impact:** inject one transient Finish error after primary success, with secondary ready. The good primary PNG is removed; secondary Begin/Draw can succeed and charge again for the same key. The original attempt may remain running despite its output having been destroyed. This is a new failure boundary introduced by the durable receipt layer, not just the original TS provider-fallback policy.

**Fix:** never route journal/claim failures or post-generation receipt persistence failures through paid fallback. Preserve completed image/provenance and settle or surface persistence failure without regeneration. Delete only an owned, demonstrably partial artifact, not every artifact created since the initial stat. Test failed Finish and failed sidecar persistence after a valid PNG, asserting zero secondary calls and unchanged PNG bytes.

## High priority

### H1. Ambiguous POST failure is automatically treated as safely retryable

**Evidence:** `apps/viderse/internal/imagegen/grok.go:318-334`; `apps/viderse/internal/imagegen/service.go:312-332,428-433,486-506`; `apps/viderse/internal/imagegen/codex.go:458-464`.

Every non-timeout/non-cancel error from Client.Do is converted to the same ProcessFailedError with stderr `connection reset`. drawTransient matches that string and starts a fresh paid attempt; no provider idempotency key is sent. Codex similarly turns text matching transport phrases into an automatic retry. A local receipt cannot prove that the remote generation did not run.

**[INFERENCE] Trigger/impact:** xAI accepts and charges the POST, then the connection closes before the response headers arrive. Client.Do returns an error; automatic retries can issue two more charged generations for the same key. Default fallback can also spend with another provider while the first outcome is unknown.

**Important source distinction:** this risky transport-retry/fallback policy is inherited from the pinned TS source and retry support is explicitly requested. It is not a newly invented port regression. It nevertheless means “one key never spends twice” is not an achievable guarantee with the current provider contract.

**Recommendation:** distinguish definite pre-send failure/provider rejection from an unknown post-send outcome. Unknown outcomes need provider-supported idempotency/reconciliation or a durable uncertain state requiring the explicit owner decision already used for crashes. Do not silently remove the agreed retry feature; the lead must reconcile that policy with the claimed spending guarantee. A deterministic server test can count an accepted request then close the socket without responding.

### H2. Error code alone incorrectly clears charges after the paid boundary

**Evidence:** `apps/viderse/internal/imagegen/service.go:428-448`; `apps/viderse/internal/imagegen/grok.go:181-190,324-350,356-357`; `apps/viderse/internal/imagegen/codex.go:379-391,423-425`; `apps/viderse/internal/imagegen/codex_test.go:242-259`.

attemptCharged makes all cancelled/tool_missing/invalid_request/quota errors free, independent of when they occurred. However Grok can return cancelled after the request was accepted, even after a successful response was decoded. JPEG/WebP processing can return tool_missing for ffmpeg only *after* the image was generated. Codex isolation/forbidden-tool failures likewise can occur after image work; the test currently assumes every forbidden-tool failure was free.

**[INFERENCE] Impact:** charged paid requests disappear from D7 accounting. Repeatedly cancelling after acceptance, or receiving non-PNG images without ffmpeg, can consume paid generations without exhausting the app cap. Cancelling the local transport is not proof of provider cancellation.

**Fix:** carry paid-boundary/charge certainty separately from the UI error code, conservatively retaining charged=true once a request may have been accepted. Only a definite pre-call failure or explicit no-charge provider rejection should release the reservation. Test success-then-cancel and paid-JPEG-then-missing-ffmpeg through Service and Postgres accounting.

### H3. Startup diagnostics bypass secret redaction and are persisted verbatim

**Evidence:** `apps/viderse/internal/imagegen/codex.go:441-447,575-589`; `apps/viderse/internal/imagegen/service.go:453-459`; `apps/viderse/internal/imagegen/imagegen.go:280-303`.

The startup branch removes only Bearer/sk-style strings. It does not run the normal token/cookie/password redactor. errCodeText then trusts a DrawError.Message and writes it directly to error_text, so the receipt boundary does not repair the omission.

**[INFERENCE] Trigger/impact:** a diagnostic such as `Error: invalid refresh_token=SYNTHETIC_VALUE cookie=SYNTHETIC_COOKIE` satisfies startupBadRe and can reach both the returned error and durable receipt with both values intact. These example values are synthetic; no real credential was read or recorded during this review.

**Fix:** sanitize provider-derived text before constructing *every* exposed error, including startup errors, and enforce the same redaction at receipt persistence as defense in depth. Add tests asserting exact synthetic credential absence in returned errors and stored error_text, covering cookies and auth.json-shaped token fields rather than only Bearer strings.

### H4. Long output lines evade the forbidden-sentinel guard

**Evidence:** `apps/viderse/internal/imagegen/codex.go:349-367,379-383,466-469`; `apps/viderse/internal/imagegen/codex_output.go:272-276`; `apps/viderse/internal/tools/process.go:115-117,179-184`.

OnLine receives a whole line, potentially megabytes long, but the code truncates auditTail to its last 16,384 bytes *before* searching for sentinels. Any sentinel earlier in that line is discarded before the check. The parser can still retain its prefix in FinalSummary and later expose it in an error.

**[INFERENCE] Trigger/impact:** emit an agent_message containing a configured sentinel near the beginning followed by over 16 KiB of ordinary text. The guard does not cancel or set secretLeaked; the returned “Không có ảnh” message can contain that sentinel. The current Codex suite has no ForbiddenSentinels scenario.

**Fix:** scan all incoming bytes before truncation, keeping only the bounded overlap needed for sentinel matches across chunks, and prevent detected material from reaching scratch/error fields. Test a sentinel at the beginning/middle/end of a long line and across delivery boundaries. The original TS used raw chunks, whereas this Go adapter receives complete lines; copying its rolling-tail expression does not preserve detection coverage.

### H5. ClassifyImageFailure erases busy/refused error identity

**Evidence:** `apps/viderse/internal/imagegen/imagegen.go:211,232-237`; `apps/viderse/internal/imagegen/codex.go:458-465`; `apps/viderse/internal/imagegen/grok.go:244-252`; `apps/viderse/internal/imagegen/service.go:165-169,439-449,515-534`; `apps/viderse/internal/imagegen/service_test.go:161-179`.

The classifier returns *DrawError and returns the embedded DrawError field from ProviderBusyError/ImageRefusedError. Go embedding is not inheritance: the resulting pointer is not an instance of either wrapper, so IsProviderBusy/IsImageRefused always fail on these results. The IsImageRefused branch following classification is consequently unreachable for classifier-produced errors. The service tests inject wrappers directly and miss this production path.

**[INFERENCE] Impact:** real Codex busy errors do not get the promised four retries; busy/refused attempts are counted as charged; refusal identity is lost during fallback. Grok explicit HTTP 429 remains correct, but text-classified refusals/busy responses do not.

**Fix:** preserve the actual error type through classification and extract common DrawError metadata separately, or use one explicit typed classification contract. Add tests through the actual Codex event/diagnostic and Grok body classifiers, asserting retry count, refusal identity, and charged=false for definite rejection. This is a regression from the TS class-returning classifier.

### H6. Valid canonical base64 without padding is rejected

**Evidence:** `apps/viderse/internal/imagegen/codex_output.go:100-118`; `apps/viderse/internal/imagegen/codex_output_test.go:142-159`; `apps/viderse/internal/imagegen/codex_test.go:332-354`.

The padding check unconditionally evaluates `strings.Index(value, "=") < len(value)-2`. For a valid nonempty standard-base64 string with no padding, Index returns -1, making that comparison true. Canonical base64 legitimately has no equals sign whenever the original byte length is divisible by three. Existing success tests use a padded fixture only.

**[INFERENCE] Impact:** otherwise valid provider PNGs in that size class fail capture after paid generation, potentially triggering another paid fallback. The pinned TS regex allows zero padding characters.

**Fix:** apply padding-position checks only when padding exists, or rely on strict decoding plus the existing canonical re-encode comparison. Cover real valid PNGs with byte lengths in all three modulo-three classes, not just malformed inputs.

### H7. The real runner cannot deliver the advertised image JSONL sizes

**Evidence:** `apps/viderse/internal/imagegen/codex_output.go:23-25,146-175`; `apps/viderse/internal/imagegen/codex.go:344-367`; `apps/viderse/internal/tools/process.go:115-117,179-200`; `apps/viderse/internal/imagegen/codex_test.go:117-123,332-354`.

The image parser permits roughly 44 MiB JSONL records for a 32 MiB PNG, but ExecRunner rejects any OnLine record above 4 MiB. CodexProvider necessarily sets OnLine. The fake runner calls the callback directly, bypassing this real framing layer.

**[INFERENCE] Impact:** base64-only PNG events above approximately 3 MiB are rejected upstream even though they are within the documented image limit and dimensional constraints. When no saved-session file is available, paid output is lost and fallback can spend again. Direct parser tests cannot establish this integration contract.

**Fix:** provide bounded stream/framing delivery whose per-image-call bound matches maxImageLine, without raising limits blindly for unrelated tools. Exercise the real runner with a local script emitting a greater-than-4-MiB valid image JSONL record; no paid call is necessary.

## Medium priority — informational

### M1. Adding 24 hours does not mean next local midnight

**Evidence:** `apps/viderse/internal/imagegen/service.go:393-401,359-377`; `apps/viderse/internal/imagegen/service_test.go:280-282`.

**[INFERENCE]** In a DST-observing local time zone, dayStart.Add(24*time.Hour) is 01:00 on the next day at the spring transition, or 23:00 on the same day at the fall transition. skipUntil can therefore keep a provider capped into the next calendar day, and RetryAt is not the promised midnight. The test duplicates the same arithmetic rather than checking a calendar boundary.

**Fix:** construct the next calendar midnight in the same Location, for example dayStart.AddDate(0,0,1), and test a normal local midnight rollover plus both DST transitions. Fixed-offset zones such as Asia/Ho_Chi_Minh do not exhibit this specific bug.

### M2. Concrete tool coupling and hidden composition depart from the accepted structure rules

**Evidence:** `plans/261029-2242-viderse-rebuild-from-yt-factory/plan.md:170-172`; `apps/viderse/internal/imagegen/codex.go:49-64,89-107,155-162`; `apps/viderse/internal/imagegen/grok.go:58-65`.

CodexDeps requires concrete *tools.Codex, then adds StatusFunc/SourceHome/PrepareCatalog overrides to make that concrete dependency replaceable in tests. A small consumer-owned Codex capability interface would express these same operations. Grok constructor also composes a concrete ExecRunner instead of receiving it from cmd, contrary to the explicit composition-root rule.

**Recommendation:** introduce only the narrow capability interface actually consumed here and inject the existing runner from cmd. Remove redundant test-only override seams once migrated; do not build a provider manager or a parallel tool subsystem. Journal already follows the desired consumer-interface pattern.

### M3. The implementation report misstates the pinned original recovery behavior

**Evidence:** `plans/261029-2242-viderse-rebuild-from-yt-factory/reports/ImageGen.md:16,117-121`; yt-factory **`2c201e6:src/server/clone/images.ts:59-62,80-95`**.

The report claims the original removes work before findOwnedSessionImage, making every restart unrecoverable. The pinned source does the lookup/capture inside try and removes work in the following finally. Its journal also lives under cache/image-invocations, not the draws/<key>.journal.json location named in the mapping table.

**Recommendation:** correct these claims in the implementation report; preserve the real distinction, namely durable Postgres receipt/admission bookkeeping versus the original invocation-file journal. Do not justify changes with a source defect that is absent at the agreed commit. No source or plan file was edited in this review.

## Areas that look correct within the inspected boundaries

- **Crash fail-closed baseline:** Begin precedes the provider call (`service.go:413-419`), running rows start charged (`queries/imagegen.sql:8-12`), and unrecoverable running attempts become failed_unknown rather than automatically drawing (`service.go:235-290`). A stored ok receipt is reused (`service.go:199-204`). These are correct single-owner flows, subject to C1/C3.
- **Explicit abandon keeps cost evidence:** `queries/imagegen.sql:38-44` changes state without clearing charged. Failed-unknown keys require Abandon before another attempt; the unit and PostgreSQL recovery tests cover this sequential behavior (`service_test.go:445-485`, `imagegen_pg_test.go:190-252`).
- **Cap filter and zero:** the SQL count filters both provider and charged and includes conservative in-flight/unknown rows (`queries/imagegen.sql:42-44`); `(provider, started_at)` has an index (`migrations/00002_image_receipts.sql:26-27`). Limit 0 rejects paid admissions before Begin/Draw (`service.go:389-419`, `service_test.go:289-307`). Nonpaid providers are explicitly exempt. These do not repair concurrent admission or H2.
- **Codex isolation setup:** readiness requires ChatGPT login, verified checks, and an image-tool-allowlist check (`codex.go:118-145`). Each invocation gets a private work directory and ownership marker, auth-only home creation, restricted catalog, copied references, exact environment, and mechanical readable-ref/unreadable-auth/unreadable-secret probes (`codex.go:151-241,264-301`; `tools/codex_isolation.go:139-173,344-374,390-475`). No inherited user config/session/skill copy was found. The image path reuses the existing tools argv builder rather than implementing a second sandbox convention.
- **Credential persistence baseline:** scratch stores invocation pointers/ownership/session/provenance, and strips image Base64 (`codex.go:29-38,307-324`); normal receipt fields contain provenance, not auth.json or OAuth token values (`imagegen.go:64-74`). Runner Log is not supplied for image generation (`codex.go:344-370`). Private auth/home permissions are explicit. These are not a blanket no-leak guarantee; H3/H4 remain blockers.
- **Recovery attribution:** target, app-owned temporary path, home containment, 0700 mode, ownership token, and live PID are checked before reclaiming work (`codex.go:481-529`). A mismatched saved PNG is rejected by byte hash, and work cleanup occurs after recovery (`codex.go:529-558`). No paid provider call occurs inside Reconcile. These checks do not provide a DB fence against another process.
- **Atomic publication/path containment:** PNG bytes are written to a same-directory temporary file and linked exclusively; existing final targets are not overwritten by link (`atomic.go:19-52`). JSON sidecars use temp+rename (`atomic.go:71-104`). CaptureImage checks exact session/call paths, realpath, regular-file status and PNG shape before copying (`codex_output.go:445-487`); Unix PNG validation opens with O_NOFOLLOW (`open_nofollow_unix.go:11-17`). No shell-interpolated prompt/path invocation was found. The destructive fallback deletion in C3 is outside, and defeats, these otherwise sound publication primitives. Atomic visibility was inspected; power-loss durability was not established.
- **SQL/error/performance baseline:** receipt queries are parameterized, Latest is LIMIT 1, and this path contains no loop of per-image database reads beyond bounded retries. No new endpoint or cross-user resource lookup is introduced in the scoped files. HTTP authorization for eventual pipeline/web integration is therefore outside this review, not established by these package tests.

Paths in this section without the common prefix are relative to `apps/viderse/internal/`.

## Edge cases found by scout / missing regression coverage

1. Two Service instances sharing one PostgreSQL store and one key, with a controlled gap between Latest and Begin, and with a primary/fallback handoff (C1).
2. Two distinct keys contending for the last provider/day slot, including limit 1 (C2).
3. Successful primary PNG followed by failed Finish/sidecar persistence; verify unchanged bytes and zero fallback spend (C3).
4. Accepted POST with lost response, post-accept cancellation, and JPEG processing without ffmpeg; assert request count and durable charge state (H1/H2).
5. Actual Codex busy/refusal classification instead of directly injected wrapper errors; synthetic secrets in startup diagnostics and long sentinel-bearing lines (H3-H5).
6. Valid PNGs in every base64 padding class and real runner delivery of a multi-megabyte base64-only record (H6/H7).
7. Midnight rollover and DST transitions with the configured clock/location rather than copying reset arithmetic into the expected result (M1).

## Verification and metrics

Executed the one authorized unpaid command once, with the required lock/priority/toolchain/concurrency and TEST_DATABASE_URL settings:

`lockf -k /tmp/verse-heavy.lock nice -n 10 env GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4 TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test go test -p 2 ./apps/viderse/internal/imagegen/... ./apps/viderse/internal/persistence/...`

Observed results: imagegen **ok (0.731s)**; persistence/postgres **ok (1.878s)**; migrations and generated package reported no test files. Command wall time was 3.92 seconds. The supplied DB configuration reaches the isolated role/schema provisioning used by these integration tests (`postgres/settings_pg_test.go:31-49`). No paid smoke was enabled and no live provider call was made by the reviewer. The opt-in smoke gate is at `imagegen/smoke_test.go:44-47`.

- Type coverage: not a meaningful percentage for this Go-only scope; compilation was exercised by go test.
- Test coverage percentage: not measured; no coverage run was authorized/performed.
- Standalone lint/race/sqlc diff: not run in this review. Claims about those checks in ImageGen.md were not re-exercised or adopted as evidence.
- Fault-injection/concurrency reproducers above: recommended checks, not claimed test results. Existing green tests do not cover the listed schedules.

## Plan status recommendations and actions

The requested files and both real provider implementations exist; no placeholder provider was found. Sequential receipt persistence/recovery, explicit abandon, basic cap boundaries, ordinary PNG publication, and major isolation setup are implemented. Do not mark paid idempotency, concurrent D7 enforcement, full source-classifier parity, or complete secret confinement as accepted while C1-C3/H2-H7 remain. Existing ImageGen.md is a completion report, not evidence that these edge cases were exercised.

Prioritize durable key ownership plus atomic cap admission, then prohibit paid fallback on ledger/postprocessing persistence failures. Correct charge certainty and secret guards next, then fix classifier/base64/runner integration. Keep the inherited retry/fallback product policy explicit (H1); do not change it silently under a refactor. Finally resolve DST reset, narrow dependency seams, and correct the source-comparison report. The lead owns all implementation, plan mutation, and subsequent verification; this review did not change plan task state.

## Unresolved questions / boundaries

- The pinned TS source and the accepted report intentionally permit retry/fallback, while strict at-most-one-remote-charge semantics require a stronger provider contract. No remote idempotency guarantee is established by the inspected code. H1 identifies that policy conflict rather than assuming it away.
- Real sandbox enforcement was inspected through its existing setup/probe code and fake-driven tests, not re-probed against a live Codex installation during this review. No sandbox escape is claimed.
- The common environment and source tree were not frozen; the conclusions apply to the inspected pending implementation, not future edits.

Status: DONE_WITH_CONCERNS
Summary: Review complete; the authorized unpaid unit/Postgres suite passed. Three critical, seven high-priority, and three medium-priority findings remain; the paid path is not ready for unattended production use.
