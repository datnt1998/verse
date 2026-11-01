# Editor reference lifetime and history repair

Status: DONE_WITH_CONCERNS

## Outcome

Uploaded references remain pinned through their worker-owned copy and are released on both successful and failed copies. Newly uploaded and explicit rival-frame references receive content-addressed artifact paths and hashes from the exact staged bytes. Their artifacts are published atomically with the drawing checkpoint before provider submission. Subsequent staging, resumed flights, and refused-image retries use the frozen artifact instead of resolving the source again. Per-scene artifact-copy directories prevent concurrent scenes from sharing writable `.part` files.

The existing version DTO resolves these persisted paths/hashes to authorized artifact URLs; no empty-URL fallback was added. The client rejects missing/empty reference URLs. A `quota: null` response is accepted and normalized to the hook's existing optional quota state without discarding historical images.

## Changed contracts and callers

- `images.Deps.UploadPath` is replaced by `AcquireUpload(ctx, id) (path, release, error)`. The production callback in `cmd/viderse/clone_wiring.go` returns `AcquireMedia`'s path and release method. `Handler.refFile` owns release after copying.
- `Handler.stageRefs` additionally returns new reference artifacts. Its callers are `Handler.plan` and `Handler.runQueue`; `DrawPlan.RefArtifacts` carries planned publication into the initial fenced checkpoint transaction.
- `Handler.copyArtifact` additionally takes the drawing item ID to isolate concurrent staging copies.
- `jobImagesSchema` accepts null or omitted quota and normalizes both to `undefined`; `getJobImages` and its hook consumers retain their optional quota contract. `imageRefSchema.url` remains required and now rejects an empty string.
- `internal/clone/images/versions.go` needs no production change: its existing exact path/hash lookup now receives persisted reference metadata. New DTO regression coverage exercises it.

No rebind, undo, review-consumer, or other worktree files were edited. No merge, browser session, package installation, production fixture, protected port, or real user configuration was used.

## Permanent regression coverage

- Repeated upload planning releases ownership after copying, and the staged bytes survive removal of the source at release.
- Copy failure releases the upload; reference publication failure prevents provider submission and an exposed flight.
- Upload and explicit rival-frame draws publish their exact reference bytes before drawing and resolve their original source only once.
- Two concurrent scenes sharing a frozen upload receive distinct staging paths and exact copies.
- The version DTO exposes upload/frame URLs selected by exact hash while artist quota resolution fails.
- `getJobImages` preserves history with null quota and both reference source kinds; missing/empty reference URLs remain errors.

## Exercised final checks

All Go commands used an isolated temporary HOME with the existing module/build caches, `GOWORK=off`, and `-p 1`.

1. `go test -p 1 ./apps/viderse/internal/pipeline/steps/images ./apps/viderse/internal/clone/images ./apps/viderse/cmd/viderse -skip TestReferenceIntegrationSmoke -count=1` — all three packages passed; the production callback compiled.
2. `npm exec -- vitest run src/api/images.test.ts --maxWorkers=1` — 11 tests passed.
3. `npm run typecheck` — editor project-reference build and application no-emit typecheck passed.
4. Actual throwaway integration: `go test -p 1 ./apps/viderse/internal/clone/images -run '^TestReferenceIntegrationSmoke$' -count=1 -v` — passed after the harness-specific adjustments below. The throwaway Go/TypeScript files were removed afterward.

### Integration evidence and boundaries

The integration exercised six redraws, each with an uploaded reference and an explicitly requested video frame. It used the real media service, filesystem blob store, cache, read leases, FFmpeg frame extraction, standalone cwebp encoding, image handler, version DTO, HTTP media delivery, and TypeScript `getJobImages`. Only repository persistence and AI-provider/analysis calls were test boundaries. An isolated HTTP test server served the real read DTO and resolved media service; this was not a full authenticated production-gateway/browser run.

Observed output:

```text
Client live-HTTP smoke: 1 test passed.
draws=6 uploaded acquisitions=6 final leases=0
all 12 reference GETs exact
real cache trims=6
real FFmpeg extraction and cwebp encoding
```

Each provider-boundary call compared staged bytes with the already-published artifact and its recorded hash. All twelve Go-side reference GET checks returned 200, `image/png`, and exact matching bytes. The real client fetched the live Go DTO, parsed six historical versions with both reference kinds, fetched their URLs successfully, and retained history with unavailable artist quota. Cache entries were actually trimmed after every redraw, proving that no cache pin remained; active repository read leases returned to zero each time.

Independent source review by `EditorContractReview` cleared upload ownership, publication/fences, cancellation, frozen reuse, quota normalization, and the per-scene staging-race repair. The reviewer ran no checks.

## Concerns and verification limits

- The first smoke attempt found that the workstation's installed `/opt/homebrew/bin/ffmpeg` lacks `libwebp`. The smoke used the installed real `cwebp` through the existing encoder dependency boundary; no production fallback or tool configuration was changed. Deployment must supply the encoder required by the existing production image encoder.
- The HTTP client smoke initially encountered happy-dom cross-origin preflight behavior against the ephemeral test server. Running that throwaway consumer in Vitest's Node environment exercised real HTTP without the unrelated simulated browser-origin restriction.
- No live AI provider was called or claimed. No full gateway/authenticated browser proof was performed for this assignment. The user-reported baseline failures were not rerun merely to reconfirm them.
