# Leaf-package production-readiness review

## Code Review Summary

### Scope

- Reviewed the complete production source of `apps/viderse/internal/tools/` and `apps/viderse/internal/youtube/`, their relevant tests and fixture-backed behavior (five YouTube fixture files); inspected the existing `internal/imagegen/codex.go` consumer for runner-contract consequences.
- Go scope: 46 files, **13,215 lines** including tests; **8,585 production lines**, 4,630 test lines. Fixtures and upstream TS are additional context, not included in those counts.
- Tools production files: `chrome.go`, `codex.go`, `codex_isolation.go`, `deps.go`, `errors.go`, `ffmpeg.go`, `installer.go`, `lifecycle.go`, `machineload.go`, `managed.go`, `manifest.go`, `process.go`, `sys_unix.go`, `sys_other.go`, `tempdirs.go`, `uv.go`, `ytdlp.go`.
- YouTube production files: `channel.go`, `client.go`, `cursor.go`, `errors.go`, `json.go`, `pacer.go`, `pages.go`, `parse.go`, `uploadtime.go`.
- Behavioral authority: `/Users/dat/Personal/yt-factory`; observed HEAD is exactly `2c201e670739260b7b347b434e03a22a7ce491a9`. Compared the corresponding TS tools/YouTube modules and the original scan request governor.
- Rules/intent: repository `AGENTS.md`, plan “Cấu trúc mã”, `ToolsRunner.md`, `YouTubeClient.md`, and the relevant phase-02/03 requirements. No source or plan state was modified. This report is the only repository write.
- Scout first: the meaningful existing dependent is `internal/imagegen`, whose Codex callback mutates shared audit state. Other risks outside the happy-path fixtures are simultaneous stdout/stderr, cancellation during termination, bootstrap shape drift, a stalled download, queued callers, and archives containing symlinks. No production YouTube caller or registered transcription lifecycle was found outside these packages yet.
- Review used the base critical/informational checklist. HTTP endpoint authorization and SQL/N+1 checks are not applicable to these leaf packages: neither adds a route or database query. Exported dependency interfaces are small; the issues are bypassed seams and lifecycle contracts, not oversized interfaces.

### Overall Assessment

**Do not accept these packages as production-ready yet.** Found **8 high-priority issues and 13 medium-priority issues**. No unconditional critical-severity exploit was established. Passing tests do not cover the failing contracts below, particularly true two-stream concurrency, fail-closed capability inspection, and crash-time ownership of directly spawned Chrome.

Findings below are grounded in static source/control-flow comparison. **Runtime consequences marked [INFERENCE] were not separately reproduced**; the one authorized existing-suite run is reported separately. Paths in findings are relative to `apps/viderse/internal/` unless prefixed with `upstream:`.

## High Priority — blocking

### H1. Codex capability inspection ignores all but the last embedded tool list

**Evidence:** `tools/codex_isolation.go:248-275`, especially assignment at line 272; image/text verdicts at `277-301`. `upstream:src/server/tools/codex-isolation.ts:122,143` concatenates every `additional_tools` list with `flatMap`.

The Go loop overwrites `tools` for each `additional_tools` item. A body with a forbidden shell/MCP tool in the first item and only `image_gen.imagegen` in the last item passes the image verdict. For text mode, a forbidden first list followed by an empty last list passes too. This is a port regression at the actual capability trust boundary, not merely a missing deny flag.

**Impact [INFERENCE]:** preflight can certify a CLI advertisement containing forbidden tools, unlocking image execution against an insufficiently restricted tool surface.

**Fix:** collect and validate the union of all embedded lists, preserving upstream top-level/null fallback behavior. Add image and text cases containing multiple embedded lists, including forbidden tools before an allowed/empty final list.

### H2. `cmd.Wait` closes output pipes before the readers have finished

**Evidence:** pipes/readers in `tools/process.go:132-141,170-185`; `cmd.Wait()` before `copyWG.Wait()` at `214-215`; `splitLines` treats any read error as completion at `373-375`.

With `StdoutPipe`/`StderrPipe`, `Wait` closes the pipes when the direct child exits; callers must finish reading before invoking `Wait`. These independently scheduled readers can still be draining output or executing a callback when the child exits. A close can discard buffered output, and the reader error is ignored. The TS runner instead resolves on the child `close` event after stdio closes (`upstream:src/server/tools/tool-process.ts:208-247`).

**Impact [INFERENCE]:** intermittent missing final Codex JSONL/ffprobe output, wrong error classification, or false successful runs with truncated stdout. A leader that exits while descendants retain the pipes can also cause premature success and stop the timeout watcher while descendants remain.

**Fix:** use correctly owned pipes/read completion, or assign writer adapters to `Cmd.Stdout`/`Stderr` so `os/exec` manages copying before `Wait` completes. Keep timeout/group termination active until the intended process/stdio lifecycle is complete. Add a synchronized slow-callback/final-output test, not just an output-length upper-bound assertion.

### H3. Runner callbacks and log writers are invoked concurrently across streams

**Evidence:** `tools/process.go:170-185` starts two goroutines that call the same `Spec.OnLine` and `Spec.Log` without synchronization. `tools/chrome.go:326-364` supplies a shared `strings.Builder` log writer. Existing consumer `imagegen/codex.go:349-360` updates the same `auditTail` and `secretLeaked` from either stream.

The callback contract changed from serial Node event-loop callbacks to concurrent Go calls without adapting the consumers. The Chrome writer also receives both streams despite its unused `stream` field; upstream explicitly filters stdout (`upstream:src/server/tools/chrome.ts:198-202`).

**Impact [INFERENCE]:** races and lost/corrupt logs, damaged probe markers, and unreliable cross-stream secret detection. This is concrete shared mutable state in an existing production consumer, not a hypothetical future caller.

**Fix:** serialize the callback/log delivery contract across both reader goroutines, preserving stream identity; make Chrome consume stdout only. Add a real two-stream callback/log-writer race test. Individual tail buffers can remain per-stream.

### H4. Cancellation during timeout cleanup can prevent SIGKILL entirely

**Evidence:** timeout watcher calls `TerminateGroup(ctxOrphaned(ctx), ...)` in `tools/process.go:200-211`; `ctxOrphaned` returns the still-live original context at `275-280`. `tools/lifecycle.go:211-223` returns on a canceled grace-period wait before reaching SIGKILL.

When the per-process timeout fires first, the original caller context can still be live. If that context expires/cancels during the TERM grace period, termination returns early. The only watcher has already fired and its error is discarded; it does not run a second cleanup pass.

**Impact [INFERENCE]:** a child that ignores TERM can remain alive and `cmd.Wait` can hang indefinitely despite both timeout and caller cancellation.

**Fix:** detach the cleanup context unconditionally once shutdown begins, using an independently bounded cleanup lifetime. Add a test where the process timeout starts TERM and the caller cancels during grace while the child ignores TERM.

### H5. Malformed/truncated session data can panic the whole application

**Evidence:** `youtube/client.go:220-224` checks only the four-byte JSPB prefix then slices at byte five. `parseSWData` accepts device arrays of length 17 at `294-297`, but `bootstrap` indexes `17`, `18`, `79`, `86`, and `87` at `241-252`.

A body consisting of the prefix alone panics on `text[5:]`. A structurally valid bootstrap response containing a 17–87-element device array passes the explicit length check and then panics during client-context construction. This bypasses all `FetchError` classification.

**Impact [INFERENCE]:** an upstream response/layout change or truncated bootstrap body can terminate the single-process server from a worker goroutine rather than yielding a recoverable provider error.

**Fix:** validate the complete prefix length and safely read optional device positions, or require the highest used index before indexing. Add short-prefix and short-device-array cases with an asserted classified error and no panic.

### H6. Chrome CDP probes bypass orphan registration and the injected process seam

**Evidence:** `tools/chrome.go:443-459` constructs/starts a detached `exec.Cmd` directly and obtains its PGID. Cleanup exists only in a defer at `525-530`. There is no registry tracking in this function. `upstream:src/server/tools/chrome.ts:267` explicitly tracks this child with match `--remote-debugging-pipe`.

Even an `ExecRunner` configured with a registry cannot track this child because the injected runner is bypassed. `kill -9` skips the defer, and restart orphan cleanup has no record to find. Direct spawning also contradicts the plan rule that external binaries remain behind injected interfaces.

**Impact [INFERENCE]:** Chrome/GPU helpers and their profile survive a server crash during export-capability probing; subsequent probes add more processes. Existing script-based Chrome tests do not exercise this CDP path.

**Fix:** give the CDP launcher an injectable spawn/pipe-owning seam and register/unregister its process group with a unique command marker. Test crash/recovery ownership and normal child reaping through that seam.

### H7. A stalled managed-tool download has no finite bound and cannot observe UI cancel

**Evidence:** `tools/installer.go:251-269` defaults to `http.DefaultClient` and attaches only the run context; `joinFlight` creates that run from `context.Background()` at `543-546`; reads block in `src.Read` at `287-288`. `RequestCancel` only writes a sentinel (`199-203`), checked through the progress callback at `714-717`, not while a read/header wait is blocked. Upstream had a 30-minute fetch timeout (`upstream:src/server/tools/tool-installer.ts:163`).

Neither the default HTTP client nor the shared run has a deadline. Keeping one waiter alive and receiving no more bytes means the sentinel is never observed. This also affects a server that never responds to the initial request.

**Impact [INFERENCE]:** the install remains in flight forever, staging remains on disk, and later repair/install requests keep joining the stuck run. The claimed cancel observation within about one second is not true in this case.

**Fix:** restore a finite download timeout and connect cancellation of a running install to its run context, so active HTTP operations are interrupted. Preserve the sentinel for durable cancellation semantics rather than relying on progress to notice it. Test stalled headers/body and cancel with an active waiter.

### H8. Registered managed lifecycles lose the shared cancellation/state contract

**Evidence:** `tools/managed.go:77-80` registers only `func(context.Context) error`; `98-104` checks `lc` once, then calls `run(ctx)` with the first callers context. `tools/installer.go:806-865` owns a different shared run context/checker, neither of which reaches that callback.

A mid-lifecycle `Managed.Cancel` writes a sentinel that the registered body cannot observe through this API. Canceling the first waiter instead cancels its captured context even when another waiter still wants the shared install. After successful body completion, the wrapper never writes `PhaseDone`; it returns installed while durable state remains installing.

**Impact [INFERENCE]:** the intended transcription/env lifecycle is non-cancelable from Settings, cancels unexpectedly when its initiating request disconnects, or appears permanently installing. No external registration exists yet, so this is a broken exposed integration contract, not a claim that an already-wired transcription service was exercised.

**Fix:** pass the shared run context and lifecycle check/state capability to the registered body; make the shared lifecycle own terminal success state. Add two-waiter cancellation, mid-body sentinel cancellation, and terminal-state assertions.

## Medium Priority — correctness, fidelity, and maintainability

### M1. Archive traversal protection does not constrain symlink targets or symlink parents

**Evidence:** lexical-only `safeJoin` at `tools/installer.go:390-396`; tar symlink creation at `450-457` followed by ordinary `OpenFile` extraction at `435`; equivalent ZIP paths at `488-516`.

An archive can create `pivot -> ../outside` (or an absolute target), then include `pivot/file`; the later regular-file open follows that symlink outside staging although both entry names pass `safeJoin`. A link replacing a previously extracted file can similarly affect later entries.

**Impact [INFERENCE]:** host-file writes outside the install root. **Threat prerequisite:** the archive must pass the expected checksum, so this is not a claim that a network attacker can bypass pinned hashes. It matters for unsafe trusted release contents and the self-updating yt-dlp archive boundary; no malicious currently pinned payload was established offline.

**Fix:** extract through an OS-enforced rooted filesystem boundary that disallows escaping symlink resolution, and reject outward links while preserving legitimate inward Chrome bundle links. Add symlink-then-file escape tests for ZIP and tar, alongside the existing benign-link test.

### M2. Log-tail limits do not bound line accumulation, even without a line callback

**Evidence:** `tools/process.go:170-181` always uses `splitLines`; `328-338,371` retain every byte of a newline-free stream in `pending`, independently of `LogLimit`.

**Impact [INFERENCE]:** large compact JSON or malformed/no-newline tool output grows process memory without bound despite a small configured log limit. The tail-length test at `tools/process_test.go:100-113` checks only returned size, not retained line memory.

**Fix:** bypass line buffering when `OnLine` is nil. For line consumers, define a bounded maximum record policy that fails explicitly on oversize records rather than silently corrupting JSONL/security inspection. Add a no-callback streaming case and an oversized-record case.

### M3. A caller waiting for session bootstrap cannot honor its own cancellation

**Evidence:** `youtube/client.go:128-131` waits on `sync.Cond` with no cancellation wakeup. Default HTTP client is unbounded (`64-67`); bootstrap uses the initiating callers context (`136`).

**Impact [INFERENCE]:** caller B can remain blocked after its deadline/cancellation until caller A finishes bootstrap, potentially forever if A has no deadline and its HTTP operation stalls. This violates the Go context-facing API even though ordinary in-flight cancellation tests pass.

**Fix:** represent in-flight bootstrap completion with a channel/result and select on that channel versus each waiting callers context; preserve successful caching and failed-bootstrap eviction. Test a blocked owner plus an independently canceled waiter.

### M4. `Pacer.MaxWait` is not a cumulative queue-wait budget

**Evidence:** `youtube/pacer.go:145-183` repeatedly considers only the next 250 ms wait; `tooFar` resets the budget to `maxWait()` on every iteration at `188-202`.

With no context deadline and default MaxWait=5s, every 250 ms iteration is allowed, regardless of total elapsed queue time. A long request or repeated contention therefore waits without limit; acquiring the semaphore afterward can add another full MaxWait.

**Impact [INFERENCE]:** foreground calls wait much longer than the advertised inline budget instead of returning `DeferredError` for scheduling.

**Fix:** compute one absolute wait cutoff when `Run` begins and use its remaining duration through acquisition and scheduled-start waiting. Test a held semaphore with elapsed time greater than MaxWait and no context deadline.

### M5. Network failures during body reads are misclassified as unusable cursors

**Evidence:** `youtube/client.go:461-463` returns raw body-read errors. `isTransportError` at `478-495` recognizes JSON/session errors, `*url.Error`, EOF and unexpected EOF, but not `net.Error`/`*net.OpError` or reset errno errors from response-body reads. `youtube/errors.go:236-239` classifies every remaining continuation error as `cursor_invalid`.

The HTTP client wraps request failures in `*url.Error`, but response-body read errors need not have that wrapper. Upstream recognizes socket/reset errno causes and leaves unrelated unknown continuation errors as `unexpected_page` (`upstream:src/server/youtube/channel-pages.ts:91-139`).

**Impact [INFERENCE]:** a connection reset while reading a continuation is treated as bad persisted pagination rather than transient transport failure, leading a scanner to discard/restart otherwise valid progress.

**Fix:** classify Go network/body-read failures explicitly, and reserve `cursor_invalid` for proven cursor/provider-layout rejections. Add a response body reader returning a reset `*net.OpError` after headers.

### M6. Stored cursor/remainder parsing is less strict than the upstream schemas

**Evidence:** `youtube/cursor.go:55-80` treats absent `visitorData` like null; `136-147` uses `Decoder.More()` instead of requiring EOF; `239-255` accepts JSON null as a nil slice and returns an empty remainder. Upstream strict schemas require the session key and an actual remainder array (`upstream:src/server/youtube/channel-pages.ts:173-261`).

Concrete accepted bad inputs include `null` for the remainder and an otherwise valid cursor followed by an unmatched closing `}` (the latter makes `More()` false rather than proving EOF). Missing nullable video fields are also accepted as zero/nil values instead of rejected.

**Impact [INFERENCE]:** corrupt stored state can silently become an empty page remainder or a different session context instead of `cursor_invalid`, weakening recovery correctness.

**Fix:** explicitly distinguish required fields from nullable values, require a non-null array, and decode a second value expecting `io.EOF`. Add these cases to the cursor mutation table.

### M7. Pre-start cancel sentinels are deliberately discarded, reversing source behavior

**Evidence:** `tools/installer.go:605-608,822-824` discards the boolean returned by `claimCancelSentinel`. Source checks `cancelBeforeStart` in both install and lifecycle paths (`upstream:src/server/tools/tool-installer.ts:265,283,429-431`). The Go test at `tools/installer_test.go:245-260` explicitly asserts the opposite behavior.

**Impact [INFERENCE]:** cancel arriving after an install is queued but before its goroutine claims the sentinel is lost, and a canceled queued install proceeds to download/activate.

**Fix:** preserve the claimed cancellation and abort the corresponding run, as upstream does. If stale-cancel behavior is intentionally redesigned, use an explicit run identity rather than treating every pre-claim cancellation as old; obtain an accepted behavior decision and document it. Do not call the present test a faithful port.

### M8. Download progress sums cumulative totals as if they were chunk sizes

**Evidence:** `tools/installer.go:285-301` passes cumulative `done` to `tick`, while `714-716` adds each value to `bytesDone`. Upstream passes `chunk.length` (`upstream:src/server/tools/tool-installer.ts:156`).

For two equal reads the caller records one chunk plus two chunks: three chunks done although only two arrived. The error grows with every read and spans all assets.

**Impact [INFERENCE]:** install progress exceeds its total by orders of magnitude on Chrome-sized downloads, producing incorrect completion/progress displays.

**Fix:** pass deltas, or retain the prior cumulative value per asset and add only the difference. Assert exact final BytesDone for a deliberately multi-read download.

### M9. Linux x64 Codex manifest selects the ARM64 archive member

**Evidence:** `tools/manifest.go:221` downloads `codex-x86_64-unknown-linux-musl.tar.gz` but sets member `codex-aarch64-unknown-linux-musl`. `unpackAsset` renames the requested member at `tools/installer.go:338-350`. Source member is x86_64 (`upstream:src/server/tools/tool-manifest.ts:347`).

**Impact [INFERENCE]:** on the advertised linux-x64 manifest path, the correctly downloaded/verified archive cannot yield the requested member and install fails. This platform-specific issue was not cross-built or run on this Darwin review host.

**Fix:** use `codex-x86_64-unknown-linux-musl`; assert platform URL/member agreement in manifest tests.

### M10. Default pacing has no jitter

**Evidence:** `youtube/pacer.go:43-47` constructs an empty Pacer; `74-80` uses random value zero whenever Rand is nil. Upstream defaults to `Math.random` (`upstream:src/server/channels/scan-repository.ts:263`).

**Impact [INFERENCE]:** normal clients always use the minimum one-second gap, not the documented 1–2-second jittered schedule. Test injection works, but production never injects randomness through NewClient defaults.

**Fix:** supply a real default random source while retaining an injected deterministic Rand for tests. Assert non-nil production default configuration separately from fixed-gap tests.

### M11. Concurrent Codex login starts are not serialized, and old exits clear newer state

**Evidence:** `tools/codex.go:408-425` performs an unlocked read/probe/spawn/write sequence; `427-429` unconditionally removes the login state when that child exits. `CancelLogin` removes the state before waiting for termination at `437-443`.

**Impact [INFERENCE]:** two Go callers can both observe no login and spawn separate detached flows, while only one PID is retained. Alternatively, start a replacement after cancel clears the file; the old childs wait goroutine can then erase the replacements state. Settings loses the ability to track/cancel the still-running flow.

**Fix:** serialize session start/cancel transitions and conditionally clear the state only if it still identifies the exiting child. Test simultaneous starts and old-child exit after replacement starts.

### M12. Mutable package-wide policy violates the explicit constructor/no-globals rule

**Evidence:** `tools/lifecycle.go:109,188` exposes/uses mutable `SystemClock`; `tools/manifest.go:108,145,167,199` exposes map-containing pin globals; `tools/codex_isolation.go:304-324` exposes mutable security-feature slices. Source plan rule: `plans/261029-2242-viderse-rebuild-from-yt-factory/plan.md:170-171` (constructor dependencies, no globals). `ToolsRunner.md` claims no globals.

These are writable shared policy, not just immutable regex compilation or an error sentinel. `PinnedAssets` also returns the underlying asset slice (`tools/manifest.go:257-268`). Feature mutation can diverge from already-cached isolation results; changing CodexDisabledFeatures does not rebuild the separately snapshotted text-disabled list.

**Impact [INFERENCE]:** instances/tests cannot isolate policy, and incidental mutation can change install or isolation behavior process-wide. No actual consumer mutation was found, so this is a structure/maintenance finding, not an assertion of a demonstrated active bypass.

**Fix:** keep defaults private and expose fresh/deep-owned values or inject instance-owned policy at construction; make the default clock a value selected inside the function. Update the intent report claim. Do not replace immutable regex values merely for stylistic consistency.

### M13. Loopback capability capture shares its verdict without synchronization

**Evidence:** `tools/codex_isolation.go:523-530` lets HTTP handler goroutines write `captured`; `578-586` reads/dereferences it after Runner.Run without a mutex/channel. Deferred `server.Close` runs only after the read.

Subprocess completion/HTTP traffic is not a Go memory synchronization operation. Multiple handlers can also write concurrently, including requests still being handled when the child exits.

**Impact [INFERENCE]:** a race in the security preflight can yield stale/inconsistent capture or race-detector failures when the real loopback path is exercised. The current pure allowlist unit tests do not establish absence of this race.

**Fix:** synchronize verdict publication and snapshot it only after handler completion is coordinated; preserve fail-closed semantics across captured advertisements. Add a fake runner that makes actual local HTTP advertisement requests and joins them deterministically.

## Low Priority

No style-only findings. Verified dead weight worth removing as part of the owning fixes: `InstallBusyError` is defined but never constructed (`tools/installer.go:62-68`); `managedEntry.lifecycle` is assigned but never read (`tools/managed.go:43-46,70`); `domOutWriter.stream` is initialized but never consulted (`tools/chrome.go:338,354-358`). These are source facts, not separate release blockers.

## Areas That Look Correct Within the Inspected Scope

- **Hash before use:** the managed installer finishes SHA-256 comparison before unpack/Prepare/SelfCheck (`tools/installer.go:304-322,714-726`); yt-dlp also verifies before extraction/self-check (`tools/ytdlp.go:369-399`). Hash mismatch/partial-copy failure cleanup is present. The manifest URL/version/hash values inspected agree with the source; the Linux member typo is called out separately. No release bytes were downloaded or independently rehashed.
- **Normal cross-version cutover:** staging is separate, self-check precedes activation, and `current` is replaced using a temporary marker plus rename (`tools/installer.go:739-767`). This does not prove every failure is rolled back: installed.json/finalize occur after activation, as in upstream, and yt-dlp replacement removes its old directory before rename (`tools/ytdlp.go:400-405`). Do not broaden this observation into a blanket transactional/crash-proof guarantee.
- **Normal UNIX cancellation and reaping:** Setpgid is used (`tools/sys_unix.go:34-36`), group signaling uses negative PGID (`17-19`), the runner waits its direct child (`tools/process.go:214`), and detached login has a wait goroutine (`tools/codex.go:427-430`). Identity is rechecked before orphan SIGKILL (`tools/lifecycle.go:216-224`); the caveats H4/H6 still block lifecycle acceptance.
- **No shell interpolation in ordinary tool calls:** the runner takes executable plus argv. Isolation probes intentionally run fixed local shell snippets, not arbitrary user shell commands.
- **Codex per-call auth isolation:** auth-only home copy, 0700 home and 0600 auth file, and exact-environment process options are preserved (`tools/codex_isolation.go:442-468`, `tools/process.go:296-317`). Catalog reduction keeps image input while stripping unrelated tools, and text mode removes image modality/features (`tools/codex_isolation.go:87-131,323-325`). This is code-level evidence, not a real-binary sandbox attestation; H1 prevents accepting the full preflight proof.
- **YouTube tested happy paths:** home → Videos → optional Latest endpoint, explicit Latest checks, same visitor/version continuation replay, deduplication, nullable renderer fields, header-banner selection, and publishDate-before-uploadDate match the corresponding TS logic (`youtube/pages.go:37-125,311-343,447-491`, `youtube/channel.go:115-254,270-346`, `youtube/uploadtime.go:35-60`).
- **Pacing scope:** all actual InnerTube HTTP calls route through `Client.do`/the shared per-client Pacer, no automatic retries are added, and HTTP 429 moves the per-instance schedule (`youtube/client.go:393-419`, `youtube/pacer.go:106-135`). This is compatible with the single-process plan if composition owns one shared client; no global singleton is required inside the package. Budget and jitter caveats remain M4/M10.
- **Classified HTTP errors:** 401/403/challenges, 429 Retry-After, 5xx/408, continuation 400/404, and channel 404 have explicit classifications (`youtube/errors.go:175-200`). The HTTP failure messages do not echo response bodies/tokens. Error-body snippets stay internal to HTTPStatusError. Body-read transport handling remains M5.
- **Secrets assessment:** no new direct printing of auth.json contents or supplied CookieFile contents was found. This is not a universal redaction guarantee: Spec.Log is explicitly raw, ProcessFailedError carries raw child stdout/stderr (`tools/process.go:49-56,79-80,176-180`), yt-dlp classification retains raw stderr (`tools/ytdlp.go:53-57,72-120`), and ChannelNotFoundError echoes raw input (`youtube/channel.go:84-91`, `youtube/errors.go:219-221`), as upstream does. Phase-02 public logs/diagnostics must redact at their owning boundary; those integrations and a real secret-canary run were outside this review.

## Edge Cases Found by Scout / Regression Recommendations

The missing cases are captured directly in H1–H8 and M1–M13. Highest-value additions are:

1. Multiple embedded capability lists with a forbidden earlier tool; real loopback capture with synchronization.
2. Simultaneous stdout/stderr callbacks, slow callback plus final output, and no-newline output with a tiny capture limit.
3. Timeout starts TERM, then caller cancellation occurs during grace; child ignores TERM.
4. CDP child survives owner death and is reaped from the durable registry on restart.
5. Minimal JSPB prefix and short device array; one blocked bootstrap owner with an independently canceled waiter.
6. Stalled install headers/body with UI cancel; pre-start cancel; multi-chunk exact byte accounting.
7. Two lifecycle waiters and initiating-caller disconnect; successful lifecycle terminal state.
8. MaxWait exceeded while a request owns the slot, reset errors while reading continuation bodies, null/trailing-junk persisted state.

## Plan / Intent Follow-ups

- The leaf capabilities and fixture-based unit tests exist; the reports are not merely API scaffolds.
- **Do not mark the tools leaf accepted** until H1–H4/H6–H8 are fixed and focused regressions prove the corrected contracts. The reported no-globals/all-dependencies-injected claims need correction (H6/M12).
- **Do not mark YouTube acceptance complete** until bootstrap malformed-input safety and the queue/context/error-classification contracts are corrected (H5/M3–M6/M10).
- The intentional system-before-managed resolution order and removal of cross-process install leases match the accepted Go plan; they are not flagged as fidelity defects merely because old TS used managed-before-system/SQLite leases.
- The changed pre-start-cancel semantics are not such a documented accepted deviation (M7).
- The full phase-02/03 acceptance criteria also require integrated Settings/background tasks, account/cookie handling, scanner fencing/recovery, and designated live smoke. This review neither implements nor signs off those broader phases; no plan checkboxes/status were edited.

## Verification / Metrics

Executed exactly once, using the authorized resource wrapper:

```text
lockf -k /tmp/verse-heavy.lock nice -n 10 env GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4 go test -p 2 ./apps/viderse/internal/tools/... ./apps/viderse/internal/youtube/...

ok  github.com/datnt1998/verse/apps/viderse/internal/tools    25.326s
ok  github.com/datnt1998/verse/apps/viderse/internal/youtube   1.616s
Wall time: 26.63 seconds
```

- Existing package tests: PASS. No extra reproducer, race run, linter, cross-build, formatter, or real-binary/live-network smoke was run. No external network lookup/download was requested during review; the unit suite uses local fixtures/test transports and local subprocesses.
- Type coverage: not measured; Go compilation was exercised by the package test run. A TypeScript-style percentage is not applicable.
- Test coverage: not measured; no coverage percentage claimed.
- Linting issues: not measured; no standalone lint/vet count claimed.
- Earlier race/build/smoke results in the intent reports are author-provided historical evidence, not review verification.

## Recommended Actions

1. Fix the fail-open capability inspector and the shared runner output/termination contracts first; then adapt the existing imagegen and Chrome consumers.
2. Restore durable process ownership for CDP and coherent managed-install/lifecycle cancellation.
3. Harden bootstrap, queue budgets, body-error classification and persisted-state parsing.
4. Correct the platform member/progress/jitter regressions and remove mutable global policy/dead fields.
5. Main agent should run the focused new regressions and the permitted package/race checks after all fixes land; this review did not rerun verification or alter sources.

## Unresolved Questions / Limits

- Release artifact integrity beyond copying the source pins cannot be established without downloading bytes; prohibited here. No checksum is alleged wrong.
- Real Codex sandbox enforcement, actual Chrome CDP crash cleanup, and live YouTube response compatibility were not exercised. The positive observations above deliberately stop at inspected code and the authorized fixture suite.
- No new authorization/data-query layer exists in this scope, so no endpoint permission or DB efficiency finding is inferred.

Status: DONE_WITH_CONCERNS
Summary: Reviewed both leaf packages against the pinned TS source and structure rules, wrote this report, and ran the authorized package tests once (both pass). Acceptance is blocked by eight high-priority lifecycle/security/input-safety defects; thirteen medium-priority fidelity, concurrency, extraction-safety and structure issues are documented with file/line evidence.
