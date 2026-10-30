# Phase 02 review — secrets, AI connections, platform accounts

## Code Review Summary

### Scope and evidence

- Target: uncommitted working tree against `HEAD`, including new/untracked files, branch `main`.
- Reviewed: `apps/viderse/internal/{aiconn,secretbox,accounts,platforms}/**`; `internal/web/{ai,platforms}.go`; `internal/tools/codex_isolation.go`; `internal/imagegen/codex.go`; PostgreSQL AI/platform stores and query sources; migrations `00003`/`00004`; `cmd/viderse/{wire_ai,platforms_wiring}.go`; relevant `serve.go` composition/shutdown. Inspected dependent core/auth, authenticated gateway, web clients/forms/catalog, tests, plan and implementation reports.
- Size: 11,840 lines across the scoped Go/SQL files including tests and existing portions of changed files; this is an inspection surface count, not added LOC.
- Scout first: traced credential creation → persistence → provider transport → DTO; GET/status dependents; concurrent login/refresh/logout; settings/agent referential invariants; catalog pagination; old Codex constructor callers. Those paths exposed the findings below.
- Verification performed: static source/diff/reference inspection only. **No tests, builds, formatters, servers, outbound/network calls or paid calls were run.** Agent-report test/latency claims were read but are not independent verification by this review.
- `MachineSetup.md` explicitly reports unfinished integration, while current `serve.go:99-111` already wires tools/AI/machine services. Treat that report and sibling machine code as in flux; findings here describe the source inspected, not its eventual state.

### Overall assessment

**Do not commit this scope as production-ready yet.** There are concrete first-use credential-loss, unbounded CPU, OAuth completion/deadlock and concurrent account-read defects. The happy-path/fake-auth tests do not cover the relevant real bridge and concurrent execution paths. Several V17 web/runtime paths also cannot fulfill the accepted behavior.

No Critical-severity exploit is established for the stated single-owner local deployment. High issues below are landing blockers. Medium issues are functional/performance/maintainability gaps that should be addressed before phase acceptance; Low items are informational cleanup.

## Critical

None established in the inspected scope. This is not a claim of whole-application security coverage.

## High priority — blocking

### H1 — Concurrent first use can replace the machine key and permanently lose saved API keys

**Evidence:** `apps/viderse/internal/secretbox/secretbox.go:40-69`, especially shared `secret.key.tmp` at lines 58-62; callers seal concurrently in `internal/aiconn/aiconn.go:507-514,604-608`.

`key()` performs read-not-found → generate → write → rename without synchronization or exclusive publication. Two POST/PATCH requests on a fresh installation can both observe no key. One can return key A for encryption while the other publishes key B; sharing the temporary filename also allows one invocation to rename bytes written by the other while returning its own different bytes. Successful ciphertext is then unrecoverable with the final on-disk key. Atomic rename alone does not make create-once initialization atomic.

**Fix:** serialize first creation across processes with a private file lock, re-read under that lock, create a unique 0600 temp file, and publish only if the final key is still absent. Never replace an existing valid machine key. Preserve explicit corruption errors.

**Regression:** synchronize many first `Seal` calls from distinct Box instances, then reconstruct a Box and reveal every successful ciphertext. Add a subprocess case to cover server/CLI contention; assert a single unchanged 32-byte 0600 key.

### H2 — Error sanitization loops forever on the very secrets it is intended to redact

**Evidence:** `internal/aiconn/adapters.go:61-73,77-97`.

`sanitize` repeatedly finds `Bearer ` or `sk-` but preserves that prefix in the replacement at line 94. After the first iteration it finds the same prefix again, inserts another `***`, and never advances. A provider 400/404/429 error body containing `sk-...` or `Bearer ...` pins the handler in an allocating CPU loop after the HTTP response has already completed; request cancellation and the 30-second transport timeout cannot stop it.

**Fix:** use a single bounded regexp replacement or an index-advancing scanner that never rescans replaced text. Reuse a suitable existing redaction implementation rather than adding a third independent sanitizer; omit provider bodies entirely if no useful safe diagnostic is needed.

**Regression:** pure unit cases for multiple Bearer/API-key occurrences, an already-redacted prefix, mixed punctuation, and long error bodies; assert termination, bounded output and absence of all supplied secrets. Exercise `httpError(429, body)` with an injected transport.

### H3 — Real OAuth completion waits for its own timeout; pasted codes can lock all future logins permanently

**Evidence:** `internal/aiconn/auth.go:86-123`, particularly the uncancellable result wait at line 100 and deferred `<-done` at line 110; caller holds `loginMu` across the call in `internal/aiconn/login.go:94-141`.

The forwarding goroutine exits only when its input closes or `ctx` cancels. Neither happens when `auth.Login` returns success or an early error: CoreAuth waits for the goroutine before returning, while the caller defers cancellation until after CoreAuth returns. Consequently normal completion/early failure remains pending until the 15-minute deadline and keeps the app-wide login lock held.

Worse, after accepting a pasted code the forwarder waits on `<-res` without a cancellation branch. The exposed ChatGPT paste route cannot supply that response: `core/auth/openai_codex.go:72-76,96-125` ignores `LoginOptions.Manual` and waits only on the localhost callback. One paste can therefore leave the forwarding goroutine and login lock stuck even after cancel/timeout. The web explicitly offers that route (`web/src/screens/settings/connect/ProviderCard.tsx:107-119,174-190`).

**Fix:** give the bridge its own cancelable child context, cancel it before joining on every return, and make result receive/send cancellation-aware. Wire ChatGPT manual submissions into the actual callback race with PKCE/state validation, or remove that unsupported interaction through an explicit contract decision; do not keep a route that accepts but never processes codes.

**Regression:** use the actual CoreAuth adapter with an injected provider: immediate success and immediate error must return promptly and permit a second login. Submit a code to a provider that does not consume Manual, cancel, and assert all bridge/flow goroutines exit. Add a valid/invalid state-bound ChatGPT manual-code integration test without contacting OAuth servers. Existing login tests use fake Auth (`aiconn_test.go:221-225,650-700`) and bypass this bridge.

### H4 — Concurrent GETs race on the Bilibili credential cache, including against login/refresh/logout

**Evidence:** unlocked `accounts.Service.Login` → `bilibiliLogin` at `internal/accounts/accounts.go:169-174`; cache writes/reads in `bilibili_cookie.go:65-104`; in-place snapshot mutation at `bilibili_cookie.go:405-408` and `bilibili_qr.go:449-451,495-497`; logout resets at `bilibili_cookie.go:373-381`.

`readLoginState` is not a read-only function: it mutates `loginSnap` and `loginFileMod`. GET `/api/platforms` and GET `/api/status` can execute it concurrently with each other and with locked mutators. Only mutators/CookieHeader take `s.mu`; GET does not. Even simultaneous first GETs can race. The repeated `s.loginSnap` reads also race with clearing it to nil, and snapshots are modified in place while GET builds its view.

**Fix:** publish immutable credential snapshots under a dedicated cache RWMutex (copy before mutation; return copied views), or consistently synchronize all accesses. Do not simply hold the network-operation mutex throughout GET: that would block status for the entire Bilibili refresh chain. Keep disk commit and cache publication ordered.

**Regression:** race-enabled concurrent first GETs and loops of Login/Status against SaveManualCookie, Refresh, Check and ClearLogin, using a fake transport. Assert no races, no panics and internally consistent account/method/renewable views.

### H5 — The web cannot create an OpenAI-compatible provider

**Evidence:** `web/src/screens/settings/connect/AddProvider.tsx:14-25,40-49,79-95`; server requirement `internal/aiconn/aiconn.go:253,487-495`.

The UI classifies `openai_compat` as URL-only, renders only the URL input, and sends only `{kind,baseUrl}`. The Go kind definition requires both API key and URL, so every UI attempt receives `400 missing_key` before any connection is made. This is a concrete Go/client contract mismatch, not a hypothetical validation concern.

**Fix:** represent key and address requirements independently; render and submit both for openai_compat while retaining URL-only Ollama. Keep the server-side key check.

**Regression:** render the real add form, select openai_compat, fill both fields, assert the outgoing JSON contains both and a real handler/service accepts it using a fake provider transport. Assert missing either required field prevents submit and remains rejected server-side.

### H6 — Accepted V17 image sources are discarded and ModelRef selection has no usable execution binding

**Evidence:** `cmd/viderse/wire_ai.go:55-83,156-175,198-230`; `internal/aiconn/agents.go:99-110,173-182`; accepted V17 behavior in `docs/design/v1/08-dev-prompt.md:510-524`.

The settings API accepts image-capable models from Gemini/OpenRouter/OpenAI-compatible providers, but the runtime image map contains only literal IDs `chatgpt` and `grok`. `configure` silently skips every other provider and ignores `ref.Model`. A Gemini primary can become an empty provider list, or silently become Grok if that fallback remains. The required Grok → Gemini fallback cannot exist. The holder is also local to wireAI: `current()` has no caller and the returned service exposes no configured drawing execution binding. The new adapters implement catalog/check operations, not the promised text/vision/image execution path.

**Fix:** supply real operation adapters for the accepted V17 sources and bind by the complete ModelRef; preserve chain order and the requested model. Expose the composed execution dependency to its consumer rather than keeping an unreachable holder. Never silently substitute/drop an accepted source. This finding does not require building the later pipeline, but phase-02 V17 acceptance cannot be claimed with catalog-only adapters.

**Regression:** composition-level tests with three fake providers prove ChatGPT quota → Grok quota → Gemini execution, exact selected model propagation, and failure rather than silent substitution for an unimplemented binding. These tests must call the real configured execution path, not only ResolveArtist.

## Medium priority

### M1 — The Codex refresh guard swallows refresh failures and does not cover the supported call duration

**Evidence:** `internal/aiconn/auth.go:139-173`; `internal/imagegen/codex.go:386-396`.

For a token with 5–15 minutes left, Manager.Credential returns the current token; CLITokens then forces Invalidate but silently falls back to that same token if refresh fails. It reports success despite violating its CLI-safe guarantee. Even a successful acquisition with just over 15 minutes remaining does not cover the supported 20-minute draw plus the reported CLI five-minute proactive window. The synthesized auth has no refresh token, so this becomes a predictable mid-call authentication failure.

**Fix:** propagate forced-refresh failure and validate the resulting expiry against the actual bounded call duration plus CLI refresh skew (including preparation), or explicitly constrain call duration to the guaranteed remaining lifetime. Do not describe 15 minutes as an unconditional no-refresh guarantee.

**Regression:** fake manager/provider cases for 10-minute token + refresh failure, near-boundary fresh token, and a 20-minute call; assert no unsafe CLI auth is issued and no child starts on failure. Test CLITokens itself, not only needsCLIRefresh.

### M2 — AI background work is not bound to serve shutdown

**Evidence:** `internal/aiconn/login.go:102,135-140,154`; `cmd/viderse/wire_ai.go:222-229`; supplied lifetime context at `cmd/viderse/serve.go:87-89,105`.

wireAI receives the server-lifetime context but warm-catalog work derives from Background. StartLogin also derives from Background and has no service shutdown hook, so neither machineStop nor client cancellation ends the flow. Warm Codex child work can outlive an early startup failure/shutdown; OAuth can continue and persist credentials after the HTTP service stops. A finite 90-second/15-minute timeout is not shutdown binding.

**Fix:** inject/own a service-lifetime context, derive warm and detached-request login contexts from it, and cancel/join owned work during shutdown before closing persistence. Preserve login across the initiating HTTP request without detaching it from the server itself.

**Regression:** cancel the injected lifetime during a blocked catalog runner/device flow; assert context cancellation reaches both, no child/goroutine remains, and no post-shutdown credential/settings writes occur.

### M3 — Configuration writes can lose new credentials and bypass the 409 in-use invariant under concurrency

**Evidence:** whole-row read/modify/write `internal/aiconn/aiconn.go:581-660`; check/delete `aiconn.go:756-781`; validate/settings write `agents.go:58-64`; SQL updates all key/URL/selection fields in `internal/persistence/postgres/queries/aiconn.sql:16-20`.

Two tabs are enough: an old selection-only PATCH can read the old EncKey, wait behind a key change/catalog request, then write the old key back. DeleteProvider can observe no assignment while PutAgents validates the provider; deletion and the settings write then both succeed, leaving a dangling ModelRef instead of 409. No transaction/revision/lock spans these invariants; ai.agents is JSON settings, so the model FK does not protect it.

**Fix:** field-specific updates or optimistic revision checks for provider patches, and one consistent transactional/locking boundary across assignment, deselection and deletion. Revalidate under that boundary; keep network fetches outside a long-held DB transaction.

**Regression:** deterministic barriers around provider reads/validation interleave key rotation with selection PATCH, and agent assignment with provider deletion/deselection. Assert the new credential survives and either assignment or deletion is rejected, never both succeeding into an invalid state.

### M4 — Provider GET is N+1 and materializes every catalog just to count it

**Evidence:** `internal/aiconn/aiconn.go:330-351`; status also calls it at `internal/aiconn/status.go:25-26`; unbounded catalog SQL `internal/persistence/postgres/queries/aiconn.sql:40-43`.

GET providers performs one provider query and one full ListModels query per provider, retaining all model metadata only for len(). The same work recurs when /api/status builds attention, including non-subscription providers it later ignores. Reported timings with four cards do not satisfy the explicit no-N+1 rule or justify this scaling.

**Fix:** fetch provider summaries with aggregate counts in one query; give attention a bounded subscription-only summary/read path. Push model filtering/limits into SQL rather than loading complete catalogs before slicing.

**Regression:** instrument DB calls across 2/20/200 providers: providers/status query counts remain constant and catalog rows are not loaded for counts. Benchmark representative large catalogs before reporting p95 acceptance.

### M5 — Backend truncation hides models; the apparent load-more control also violates the DOM bound

**Evidence:** server cutoff `internal/aiconn/aiconn.go:410-412,427-459`; client has no limit/cursor at `web/src/api/ai.ts:122-128`; view `ModelCatalog.tsx:55-72,147-150`.

The server returns only the first 200 by default with no continuation metadata. The client requests no alternative limit, then treats returned row count as the total, so a real 500-model provider appears to have 200 models and no load-more button. Searching can find some omitted models but does not make the full catalog browsable. Conversely mocked tests returning all 500 exercise a different API contract: pressing load more mounts 400 rows because shown grows and rows.slice(0,shown) keeps previous rows. That exceeds the explicit ≤200 DOM rows requirement.

**Fix:** align a real pagination contract (or explicitly fetch the complete bounded catalog) and render a moving/virtualized window of at most 200 rows. Preserve selected IDs across pages and make the displayed total truthful.

**Regression:** test web through the real Go list contract with 500+ models, browse/select models beyond 200 without search, and assert DOM rows remain ≤200 after repeated navigation/load-more.

### M6 — In-process login contention queues uncancellable requests instead of returning busy

**Evidence:** `internal/accounts/filelock.go:20-25,33-41`; refresh spans multiple 20-second network calls (`bilibili_qr.go:92-119,463-595`); CookieHeader also waits on the mutex (`bilibili_cookie.go:420-423`).

The file lock uses TryLock, but every goroutine first blocks on s.mu.Lock. Thus contention in the normal single server never reaches the documented 409 busy path: overlapping QR polls/logins/refresh requests queue behind slow network work, and canceled requests cannot leave that queue until they acquire the mutex. EnsureFresh may also wait for a long refresh rather than promptly returning the existing cookie. The current busy test holds an external file lock (`bilibili_qr_test.go:633-656`), so it misses normal in-process contention.

**Fix:** use nonblocking operation ownership (TryLock → busy) or a context-aware bounded gate, separate from the immutable cached-read synchronization in H4. Keep EnsureFresh best-effort and able to return the existing credential when another login operation owns the gate.

**Regression:** block one fake transport call, concurrently issue QR/refresh/check with canceled contexts and call EnsureFresh; assert prompt busy/cancellation/fallback, bounded queued work and no additional network calls.

### M7 — Expired pauses remain paused indefinitely on GET

**Evidence:** `internal/platforms/service.go:100-131,139-141`; list query `postgres/queries/platforms.sql:1-5`; only the separate Pause query clears elapsed values (`platforms.sql:16-27`).

Status treats any non-nil paused_until as active without comparing it with Now. GET reads All, which does not clear/filter expiration. With no scanner call after the deadline (including the current phase), the UI continues to say paused in the past and can mask needs_login until some unrelated paced request cleans the row.

**Fix:** normalize expired pauses in the read model using the injected clock, without adding writes or per-platform calls to GET. Omit expired pausedUntil/pauseReason and retain the intended backoff counter separately.

**Regression:** fixed-clock status cases before, exactly at and after the pause deadline; include a refreshError so expiry reveals needs_login immediately.

### M8 — Platform error handlers return raw internal errors

**Evidence:** `internal/web/platforms.go:132-136,299`; file errors include absolute credential paths at `internal/accounts/bilibili_cookie.go:73-81`; DB wrappers expose relation names at `internal/persistence/postgres/platforms.go:29-34`.

Unlike writeAIError, platform list and the default error branch directly serialize err.Error(). Permission/storage/DB failures can therefore expose the home-directory identity, credential filenames and database/driver details. No direct raw cookie/refresh-token disclosure was found in the normal Bilibili provider-error paths, but this generic API boundary is not safe.

**Fix:** return a neutral 500 envelope and send the internal cause only to the redacted diagnostic logger. Continue returning the curated LoginError messages/reasons.

**Regression:** inject file/DB errors containing a home path, DSN and token-like sentinel; assert none occur in API JSON and the diagnostic path redacts them.

### M9 — Ollama catalog grants text capability to embedding-only models

**Evidence:** `internal/aiconn/adapters.go:477-490` assigns CapText to every /api/tags entry; /api/show capability handling exists only in CheckModel (`adapters.go:522-552`); assignment trusts catalog caps (`agents.go:74-88`).

An embedding-only bge-m3 or qwen3-embedding model can be selected from the ordinary catalog and accepted as Director text. A successful PUT is then not evidence that the chosen model can generate text. The AIConn report itself records embedding-only models advertised as text; the separate manual-check path does not repair the ordinary add/list/select flow.

**Fix:** collect authoritative completion/vision capabilities during explicit catalog refresh/add operations (not per GET), or validate previously unknown capabilities before allowing assignment. Preserve explicit empty capabilities instead of treating them as text.

**Regression:** /api/tags containing bge-m3 and a completion model, with /api/show returning embedding-only vs completion; assert embedding remains browseable but assigning it fails with 422 capability_missing/text. Verify qwen3:14b remains text-only and is rejected for the vision prompt role.

### M10 — Existing paid-smoke caller was not migrated to the required auth dependency

**Evidence:** `internal/imagegen/smoke_test.go:43-67` constructs CodexDeps without Auth; new required Auth path at `internal/imagegen/codex.go:104-117,134-136`.

The opt-in real-image acceptance smoke now always fails Ready with not-logged-in, even on an otherwise configured machine, because it cannot acquire credentials. Normal tests skip this caller by default. This is a source-proven regression; no paid smoke was run in this review.

**Fix:** migrate this caller to the same core/auth-based token source and explicit app identity used in production, not back to the old CLI home. Keep the one-paid-image guard.

**Regression:** extract/reuse the smoke composition and exercise credential acquisition with a fake auth store/provider without drawing. Only the owner should run the opt-in paid smoke after its preflight is fixed.

### M11 — Connection test reports success even when persistence failed

**Evidence:** `internal/aiconn/aiconn.go:805-819`, particularly ignored SetStatus and ReplaceCatalog errors at lines 814-817.

After a successful remote catalog fetch, TestProvider ignores failed status/catalog writes and returns {ok:true}. The UI can say the connection succeeded while the card remains auth_failed and no returned catalog is saved. A canceled request/DB outage is one concrete trigger. A failed mutation is not an optional observation: this endpoint is used to recover the saved provider.

**Fix:** propagate required persistence errors and publish changes only after the committed state is available. If a transport-only test is desired, separate that operation explicitly instead of claiming a state update occurred.

**Regression:** fail SetStatus and ReplaceCatalog independently in the real service fixture; assert error response, no false success, and no success invalidation for state that was not saved.

### M12 — Checking an existing catalog model does not make it manual in PostgreSQL

**Evidence:** `internal/aiconn/aiconn.go:871-875` sets m.Manual=true; `internal/persistence/postgres/queries/aiconn.sql:49-56` leaves manual unchanged on conflict; subsequent DeleteAiCatalogAuto at lines 58-62 removes NOT manual rows.

If a model already came from the automatic catalog, adding/checking it by name is presented as manual but the upsert conflict branch retains manual=false. A later catalog refresh that omits that model deletes it, contrary to the declared manual-survival contract. The in-memory store test updates the flag and the PG test only inserts a brand-new manual ID, so neither covers the actual conflict behavior.

**Fix:** on conflict preserve manual=true once set and promote false to true for a manual upsert (for example `manual = ai_models.manual OR EXCLUDED.manual`). Keep automatic refreshes from demoting explicit manual entries.

**Regression:** real PostgreSQL: insert automatic model, CheckModel/UpsertModel same ID with Manual=true, refresh a catalog omitting it, assert the row and manual=true survive.

## Low priority — architecture and cleanup

### L1 — Wire/domain separation is partial, and secret-free views still carry ciphertext

**Evidence:** `internal/aiconn/aiconn.go:30-41,163-171,320-325`; JSON-tagged service inputs at lines 481-485 and 571-575; `internal/web/ai.go:228-245` directly serializes/deserializes domain.AgentModels; `internal/aiconn/login.go:25-34` calls its session wire-level.

Most provider/platform output DTOs are explicitly mapped, which is good. However the business layer chooses HTTP statuses and owns some JSON request/result shapes, and ProviderView embeds domain.Provider including EncKey despite its secret-free comment. Current providerToWire correctly excludes it, so this is not a demonstrated ciphertext/API leak; it is avoidable exposure/coupling that makes a future direct encoding dangerous. The unused Tasks interface also adds an activity dependency with no caller.

**Fix:** keep HTTP mapping in web, use an explicit secret-free provider summary, and remove unused interfaces rather than adding speculative adapters. Keep the actual consumer interfaces grouped by cohesive use, not split merely for interface-count aesthetics.

**Regression:** DTO contract tests reject credential/ciphertext fields for every create/patch/list/error path; an architecture test checks that web uses the summary rather than embedding the persistence provider representation.

### L2 — Duplicate helpers/tool composition and unreachable compatibility code add maintenance paths

**Evidence:** identical key hints at `internal/aiconn/aiconn.go:918-924` and `internal/secretbox/secretbox.go:110-117`; independent redactors at `aiconn/adapters.go:77-97`, `aiconn/auth.go:178-192` and `internal/logs/logs.go:59-72`; additional tools.New without the shared registry at `cmd/viderse/platforms_wiring.go:41-46` versus `serve.go:95-104`; unused holder access at `wire_ai.go:80-84` and unused SetSelected interface method at `aiconn/aiconn.go:61`.

The duplicated redaction already contains a real defect (H2). Separate tools instances mean profile verification does not use the same configured runner/registry as machine tools. No current process-stop failure is established from that duplication alone, so do not treat it as a second security finding.

**Fix:** use the existing injected tool kit for the YouTube probe, consolidate key hints/redaction where their policies actually match, remove unused SanitizeLoginError/Tasks/SetSelected or give them a real consumer, and remove the unreachable holder through the H6 execution cutover.

**Regression:** one composition test proves the profile probe uses the supplied shared runner/registry; one table of secret-redaction cases is applied to the diagnostic boundaries. No need for an abstraction wider than these real consumers.

## Explicit security and correctness conclusions

### Secret handling that looks correct in the inspected paths

- API keys are sealed before DB storage, and ai_providers stores bytea ciphertext plus the limited hint (`aiconn.go:507-526`; migration `00003_ai_providers.sql:15-16`). AES-256-GCM uses random nonces and authentication failure is explicit (`secretbox.go:74-106`). **H1 concerns initialization, not the cipher choice.**
- Provider HTTP DTOs are allowlists and omit EncKey/full API keys (`web/ai.go:46-58,111-128`). Bilibili views omit cookie/CSRF/refresh token and mask SESSDATA (`accounts/bilibili_cookie.go:202-211,235-263`; `web/platforms.go:64-75`). Key hints of at most the final four characters are consistent with §15.
- Core OAuth access/refresh credentials remain local plaintext in the explicitly private app store: `core/auth/store.go:166-219` uses a file lock, unique 0600 temp file and atomic rename. For this single-user app, that is the accepted private-file boundary; it is not equivalent to encrypting API keys in Postgres and should not be described as such.
- Bilibili snapshot/mirror/QR files are newly written via unique 0600 temps and rename (`accounts/bilibili_cookie.go:112-145`); Netscape jars are restricted to bilibili.com/subdomains and removed after use (`bilibili_cookie.go:438-470`). Raw manual-cookie input is validated without echoing its content (`158-197`). Existing directory/file permissions are not globally repaired simply by MkdirAll; the code guarantees modes on its new secret writes, not arbitrary pre-existing files.
- The synthesized Codex home receives only app access token/account ID, a deliberately empty refresh token and a fresh last_refresh; auth.json is 0600 and call home 0700 (`tools/codex_isolation.go:503-575`). Image work is deferred-cleaned (`imagegen/codex.go:292-306`), and its model sandbox verifies auth.json is unreadable (`241-255`). No app refresh token is copied into CLI auth. The access-token-as-id_token format is supported by the author-reported pinned CLI check; this review did not execute or independently inspect external Codex source. The token lifetime defect is separately M1.
- Authenticated routes are registered through Gateway.Handle, and platform/webapp enforces session plus same-origin/JSON boundaries before business dispatch (`platform/webapp/server.go:105-164`). In the explicitly single-owner model, not adding per-provider ownership/RBAC is reasonable; this must not silently become a multi-tenant API.
- Normal Bilibili transport failures use curated messages, not response bodies containing tokens; redirects are disabled (`accounts/accounts.go:146-150`; `bilibili_qr.go:92-140`). No direct credential-in-log/API-response leak was found in those normal paths. Generic internal errors are the M8 exception. Export-bundle redaction outside this scope is not certified here.

### SSRF/baseUrl decision for this app

**Accept loopback, private LAN hosts and arbitrary owner-selected HTTP(S) Ollama/OpenAI-compatible endpoints in this single-owner local app.** Blocking RFC1918/localhost would break the product. With the session and origin guard above, a deliberate authenticated request to contact the endpoint the owner supplied is not, by itself, a release-blocking SSRF vulnerability.

The current API only trims BaseURL (`aiconn.go:487-500,612-619`) and accepts overrides even for fixed-provider kinds. That is an unnecessarily broad configuration boundary, not proof of a remote unauthenticated exploit. Recommended hardening: parse and require absolute http/https URLs, reject URL userinfo/fragments/query-carried credentials, allow address overrides only for the intended custom/local kinds, and make custom endpoint/key trust explicit. Preserve legitimate HTTP localhost/LAN. For key-bearing transports, reject credential-forwarding cross-origin redirects (including x-api-key/x-goog-api-key) and HTTPS downgrades rather than relying on Go only stripping standard Authorization on some redirects. Add fake-transport redirect/URL-policy tests. Reassess the threat model before untrusted paired users or public hosting gain settings access.

### Capability/status semantics that look correct

- When stored capability metadata is correct, assignment validates Director text, prompt vision, draw/fallback image and returns 422 capability_missing with field/need/model (`agents.go:74-110`). Ordinary qwen3:14b is not granted vision by the Ollama family list. Metadata correctness is not guaranteed for embedding-only models (M9).
- Invalid create credentials keep a repairable auth_failed card and return 422 with upstream status plus id/hint (`aiconn.go:533-567`; `web/ai.go:214-229`). In-use deletion/deselection returns 409 with agents in the sequential case (`aiconn.go:684-687,772-775`). M3 is the concurrent-invariant gap.
- `POST .../test` returning 200 `{ok:false,...}` for an upstream failure is compatible with §15; §15 reserves its explicit 422 auth_failed example for provider creation. Do not change this to 422 merely for stylistic consistency. Persistence failure must still propagate (M11).
- The platform error mapping consistently uses 400 invalid/unsupported, 409 busy, 422 rejected and 502 unavailable (`web/platforms.go:266-295`); normal same-process busy behavior is the M6 implementation gap.

## Bilibili fidelity and provenance

The Go QR/refresh ordering matches the **available working-tree reference** inspected:

- best-effort buvid3/buvid4; HTTPS Bilibili QR URL validation; 180-second local expiration;
- poll codes 86101 waiting, 86090 scanned, 86038 expired, 0 success, others rejected;
- URL cookies merged with Set-Cookie, required SESSDATA+bili_jct, verify-nav failure distinction;
- 24-hour lastCheckedAt gate; cookie/info `-101` fallback timestamp; RSA-OAEP-SHA256 correspond path;
- refresh with old csrf/token, confirmation with new csrf/cookie and old refresh token, nav, then one authoritative snapshot containing the new cookie/token;
- failed refresh keeps old credentials and marks refreshError; scan can continue with existing cookie; successful QR/manual/refresh triggers backoff clearing through platforms (`platforms/login.go:45-47,104-107,126-129,176-183`). The clearing errors are deliberately ignored, so immediate ready is only guaranteed while DB mutation succeeds.

**Source provenance limitation:** `git show 2c201e6:src/server/backdrop/bilibili-qr-login.ts` fails because that path is absent in the pinned commit. `git ls-tree 2c201e6:src/server/backdrop` contains catalogue/links/listing/pacing/pick, not the QR/login modules. Both `src/server/backdrop/bilibili-{login,qr-login}.ts` are untracked in the reference repository; its HEAD is `2c201e6`. Therefore a claim that QR fidelity was verified against that commit would be false. The readable untracked files were used for the comparison above. The lead should record/pin the actual QR behavior source before acceptance; no reference repository files were changed.

The important Go-specific deviations needing correction are cache synchronization (H4), in-process busy/cancellation (M6), and elapsed pause presentation (M7), not the refresh-chain field ordering.

## GET-path and architecture conclusions

- GET providers/models/agents use local persistence/auth state, not provider HTTP or binary execution (`aiconn.go:267-286,330-351,417-463`; `agents.go:19-29`). Exceptions by design: GET QR polling is an explicit active login action that contacts Bilibili; it is not the ordinary read/status path. Provider GET still violates no-N+1 (M4).
- GET platforms batches platform state and usage rather than per-platform SQL (`platforms/service.go:100-131`); no tool spawn, provider HTTP or recursive filesystem walk occurs in its normal status path. Login cache races remain H4. Browser profile enumeration is explicit/cached rather than a status-side walk; an empty result uses a nil slice and therefore misses the intended cache hit (`accounts/youtube.go:188-201`), a small additional performance limitation rather than a tree walk on every status GET.
- Business code does not import app persistence/web/bootstrap/cmd in the reviewed packages; composition stays in cmd and PostgreSQL adapters in persistence. The import-layer test would not catch wire-type mixing or oversized cohesive facades (L1). Parameterized sqlc queries and atomic platform slot/scanner UPSERTs look appropriate; the platform SQL primary key supports its per-platform operations. No SQL-injection issue was found.
- No independent p95/RSS/idle benchmark was performed. The author reports measured four-provider latency, but those measurements do not disprove N+1, shutdown detachment or catalog truncation. No recurring idle timer was added inside these account/AI packages; warm work is one-off but must be server-bound (M2).

## Recommended actions and plan status

1. Fix H1–H4 first: credential retention, terminating sanitization, real OAuth bridge completion, immutable synchronized account reads.
2. Repair the actual V17 end-to-end paths (H5/H6) and concurrency/lifetime boundaries (M1–M3/M6). Do not mark V17 complete based on catalog/fixture-only tests.
3. Align real backend/frontend list contracts and no-N+1/DOM bounds (M4/M5), fix state/error/capability/persistence issues (M7–M12), then consolidate the identified dead/duplicated paths.
4. Lead verification after edits land: focused unit/integration/contract suites under the prescribed heavy lock, race coverage for the new interleavings, and web tests against real response shapes. Add the regression scenarios above before relying on a green suite. Do not run the paid smoke during this read-only review; its caller currently needs migration.
5. Plan tasks with evidence of implementation: encrypted API-key storage/explicit DTO mapping; AI provider/agent CRUD semantics for sequential happy paths; private Bilibili credential files; QR/refresh protocol and platform status/pacing persistence. **Not acceptance-complete:** actual OAuth completion/manual fallback; safe concurrent credential lifecycle; no-N+1/catalog/UI contracts; V17 extra-source execution/fallback; usable paid-smoke composition. V24 real phone-confirmed QR/refresh was not exercised here. V7/V9/V22 work outside this scope is not approved by this report.
6. No plan checkboxes/statuses were changed. Reconcile MachineSetup/report claims with the final landed source and pin the missing QR reference provenance.

### Metrics

- Type coverage: not measured; no percentage claimed.
- Test coverage: not measured; no percentage claimed.
- Lint/build issues: not measured; no clean result claimed.
- Runtime/network/paid checks: none executed.

### Unresolved questions / bounded limitations

- Exact versioned source for the untracked yt-factory QR/login modules is missing from commit 2c201e6; the working-tree comparison is stated explicitly above.
- Existing report claims about the external Codex 0.162.1 parser were not independently revalidated (network/CLI checks prohibited here). The local synthesized shape and refresh-token exclusion were inspected.
- Final machine/setup/log integration may change after this snapshot. No conclusion about another reviewer/agent area should be inferred from this scoped report.

Status: DONE_WITH_CONCERNS
Summary: Completed the read-only secrets/AI/platform review and wrote this report. Six High landing blockers plus twelve Medium and two Low findings are backed by source locations, concrete fixes and regression-test ideas; normal private-file storage, explicit secret DTOs, authenticated boundaries and Bilibili protocol ordering look sound within the stated limitations.
