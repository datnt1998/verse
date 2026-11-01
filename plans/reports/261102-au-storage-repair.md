# AU storage repair — 2026-11-02

Status: **DONE within assigned storage/undo scope**.

Commits:
- `37c1a588` — `fix(noverse): serialize config migration and validate provider undo`.
- `014a5d95` — `fix(noverse): bind config writes to client version snapshots`.

## Fixes

- Finding 2: all file-backed config writes now share the existing `gofrs/flock` pattern on a persistent `<config>.lock` sidecar. Migration holds it across READ → STRIP → atomic replacement. `SaveConfig` takes the same lock; `SaveProviderConfig` holds it across its complete read/patch/write. Internal locked writers avoid recursive lock acquisition. The lock is not attached to the config inode that atomic rename replaces.
- Migration retains unknown raw keys and the existing permission mode. Damaged documents are rejected without changing their bytes or mode. Ordinary typed config saves retain their existing private `0600` policy.
- Added `SaveConfigIfUnchanged`: captured bytes **and file generation** comparison and replacement occur under that same lock, with a separate missing-file state. Web PUT, provider swap and undo bind the caller's `If-Match` to a locked metadata/content snapshot before their typed load and use the atomic snapshot CAS at save time. A cross-process write either before snapshot capture or after it conflicts instead of losing the newer settings. Hosted persistence continues through the existing document CAS.
- Finding 6: undo validates restored primary and fallback provider/model references against the **current** catalog, reuses provider/thinking/fallback validators, and enforces the different-provider fallback rule before saving. Invalid restoration returns HTTP 409 without consuming the undo entry. Undo still stores/restores selection fields only, never provider definitions or old credentials.

## Actual write-path coverage

Source tracing found production config replacement centralized in bootstrap persistence. Setup, project creation, web `saveTargetConfig`, host selection saves, and host model configuration saves reach `SaveConfig`/`SaveProviderConfig`; migration now shares their lock. Web read/modify/write handlers additionally use the locked snapshot CAS. No host, worker, core/llm, UI, legacy HTTP route, embedded dist, or dependency file was edited.

## Verification exercised

All Go commands ran from `/Users/dat/Projects/verse-provider`, with a fresh outer temporary HOME, exported `/tmp/vs3.env`, `GOWORK=off`, `-count=1 -p 1`. Child writer processes receive their own temporary HOME. No protected HOME, production service, provider request, browser, npm install/build, push, or deployment was used.

1. Final formatting: `gofmt -w` on exactly the six changed Go files.
2. Initial combined focused command:
   `go test -count=1 -p 1 ./apps/noverse/internal/bootstrap ./apps/noverse/internal/entry/web -run 'Test(MigrateConfig|ConfigFileCrossProcessWriters|ConfigSnapshot|Swap|ProviderSwap|ConfigWriteSnapshot|LoadConfig|SaveProviderConfig|Config.*Scope|.*Routing)'`
   - Bootstrap **passed**, 2.963s, under HOME `/tmp/au-storage-go-la0lc7sz`.
   - Web initially failed compilation: `config_swap.go:357:30: cannot use 3rd function result (value of type bool) as string value in multiple assignment`.
   - Corrected the assignment to use the existing separate `roles.ResolveReasoningEffort` helper. No assertions were weakened.
3. Repaired final web focused command:
   `go test -count=1 -p 1 ./apps/noverse/internal/entry/web -run 'Test(Swap|ProviderSwap|ConfigWriteSnapshot|Config.*Scope|.*Routing)'`
   **Passed**, 3.960s. Isolated module/build caches from the first temporary HOME were reused; the outer HOME itself was new.
4. Broader config callers and actual file/hosted scope guards:
   `go test -count=1 -p 1 ./apps/noverse/internal/entry/web -run 'TestConfig'`
   **Passed**, 9.794s, with another new outer HOME and the same isolated caches.

### Regression evidence

- Cross-process writer handshake plus independently held flock covers migration, normal save, provider patch and snapshot CAS for global/project/story paths (12 subcases). Writer B commits newer style/budget while holding the same persistence boundary; the blocked migration subsequently removes the model pin and preserves B. No random stress loop is used.
- Stale snapshot CAS rejects the replacement and preserves the current budget/style.
- Unknown nested raw keys and existing `0640` mode survive migration; damaged input bytes/mode stay unchanged; repeat migration does not rewrite clean documents.
- **Real isolated loopback HTTP smoke:** `TestSwapUndoRejectsRemovedCatalogReferencesWithoutConsumingToken` starts an actual `httptest.NewServer(Gateway)` on an ephemeral loopback port with isolated HOME/root. Real HTTP login → A→B swap → PUT removing unused A/provider or old A/model and changing budget → undo 409 leaves disk unchanged and token retained → PUT repairs A with new credentials → same-token undo 200 restores A while retaining budget 77.5 and the new credentials. Both provider-removal and model-removal variants passed.
- Restored fallback catalog/provider/model and same-provider policy rejection are covered, alongside existing successful budget-preserving undo and conflicting selection cases.

## Exact limits / integration notes

- The HTTP smoke uses the real gateway and filesystem over TCP, not a separately built CLI `serve` executable, live worker, or upstream provider. Parent owns final combined runtime/serve verification after runtime/UI/legacy-route repairs land.
- No Go LSP tool/server was exposed; references used the available source-search fallback. A fail-before negative control used Go's isolated overlay mechanism, without mutating shared source; details below.
- No full repository suite, standalone `go vet`, race detector, UI check, or deployment is claimed. `go test` used its ordinary vet/build behavior. Bootstrap results were reused after the web-only compile correction; no unchanged gate was rerun just to reconfirm it.
- No product design document was edited under the shared rule prohibiting docs changes. This report supplies the integrator's durable documentation note: shared config sidecar locking, atomic file snapshot CAS and current-catalog undo validation.
- Commit used a private index initialized from current HEAD and six explicitly named owned files; no unrelated staged files were included. Parent's serialized HEAD slot was released immediately after the hash was obtained.

## Follow-up: original client If-Match race closed

Reviewer found a separate public concurrency gap after the original repair: process-local middleware could accept client version A, another process could save B, and the handler would capture B as its CAS baseline while still applying stale body A.

`014a5d95` fixes both sides of this boundary:
- `ReadConfigFileSnapshot` captures document bytes and metadata under the shared flock.
- `version.go` factors out the existing metadata/content hash formula so the handler checks the exact captured snapshot against the original caller token, not a later independent read.
- PUT, swap and undo return 409 if that snapshot is not the caller's version. The final locked CAS additionally compares file identity, size, modification time and bytes, rejecting later replacements, including identical-content atomic saves.
- All snapshot callsites migrated to `ConfigFileSnapshot`; the previous byte-only signature is removed. Added bootstrap coverage for identical-content replacement invalidating a snapshot.

### Deterministic fail-before / pass-after evidence

The new `TestConfigIfMatchBindsMiddlewareToHandlerSnapshot` uses a real ephemeral TCP server with an explicit dispatch seam between the actual `beginFeedback` and `putConfig` methods:
1. Send stale body A with actual HTTP `If-Match: version-A`.
2. Wait for `beginFeedback` to accept A.
3. Run the actual cooperating `SaveConfig` writer B in a separate test-binary process with its own HOME.
4. Release the handler to capture its snapshot.
5. Require 409/conflict and byte-for-byte preservation of B.

This runs for global, project and story scopes. No scheduling sleeps, production hook or mock storage is used. The separate authenticated `httptest.NewServer(Gateway)` swap/removal/repair/undo smoke now also supplies real `If-Match` headers on every mutation.

- **Negative control:** copied only `config.go` into a temporary directory, removed the new caller-token/snapshot binding in that copy, and ran the exact regression using `go test -overlay=<temporary overlay.json> -count=1 -p 1 ./apps/noverse/internal/entry/web -run '^TestConfigIfMatchBindsMiddlewareToHandlerSnapshot$'`.
  All three scope subcases failed at `config_swap_test.go:550` with **`stale A after middleware passed = 200`**, proving the stale overwrite remains observable without the fix. Expected-negative result: package failed in 1.965s. The first negative attempt also exposed a test dispatch setup mistake (not forwarding the query scope to `resourceFor`); it was corrected before the conclusive three-scope run, without weakening assertions. Shared source was never mutated by this control. Temporary overlay/source files were removed afterward.
- **Final positive gate:** HOME `/tmp/au-ifmatch-final-ihxktikn`, exported `/tmp/vs3.env`, `GOWORK=off`, isolated module/build caches; `go test -count=1 -p 1 ./apps/noverse/internal/entry/web -run 'Test(Config|Swap|ProviderSwap|ResourceVersion|Audio.*Version)'` **passed in 17.022s**.
- AURuntimeRepair supplied final full-bootstrap gate evidence on shared source `014a5d95`: **PASS 21.101s**, including the new snapshot/identical-replacement tests. This is peer-executed evidence, not a duplicate run by storage. Runtime's host/race/vet/build/serve gates remain its responsibility.
- Follow-up commit again used a fresh private index from latest HEAD and exactly six explicit files, including parent-authorized `version.go`; HEAD slot released immediately after hash delivery.
