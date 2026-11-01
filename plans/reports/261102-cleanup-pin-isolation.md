# Cleanup pin and integration-storage isolation repair

Status: DONE

## Outcome

Integration fixtures now provision a unique disposable S3 bucket for each candidate configuration instead of pointing independent PostgreSQL repositories at the shared environment bucket. Production cleanup and cache lease behavior are unchanged. The strict pinned-cache and aggregate freed-byte assertions remain in place.

## Root cause and evidence

The observed full-gate failure stopped at `cleanup_integration_test.go:88`, where the old test interpreted any nonzero `RunResult.Freed` as pinned-cache eviction. It had not yet checked whether the cache file existed.

The source exposes a fixture-isolation defect:

- `newFixture` provisions an isolated PostgreSQL schema and cache, but the previous `garageConfig` returned `S3_TEST_BUCKET` unchanged for every fixture.
- `storagegc.Service.Run` lists all `v1/sha256/` objects in the configured bucket. `BlobRepository.Collect` consults only its own repository's roots and leases, so aged objects belonging to another repository can appear collectible.
- `Cleanup.RunCleanup` initializes `Freed` from remote orphan deletion and then adds cache-trim bytes. A nonzero aggregate therefore does not establish cache eviction.
- `Cache.Acquire` retains a shared per-digest file lock until the returned handle releases it. `TrimPreview` requires the exclusive lock, and `media.Service.Acquire` preserves that cache release callback alongside the PostgreSQL read lease. No pin-ownership defect was found in this path.

The old failing preview's individual remote/cache deletion counts were not available, so attribution of its exact nonzero byte count to a particular preexisting shared object remains an inference. The shared-storage isolation defect itself is established by the source and covered by the real-storage regression below.

## Changes

- `apps/viderse/internal/serverstorage/garage_integration_test.go`: reuse `platform/blobstore/s3test.Bucket` to create a uniquely named bucket with access for the fixture key. Register bounded object cleanup against only that new bucket before its existing bucket-deletion cleanup. Never empty or delete `S3_TEST_BUCKET`.
- `apps/viderse/internal/serverstorage/cleanup_integration_test.go`: assert the fresh fixture has no remote orphan candidates; retain member replay's HTTP 403 expectation and strict `Freed == 0` while pinned; read and digest the pinned bytes after actual cleanup; retain exact post-release freed bytes and cache-file deletion checks.
- Add `TestGarageCleanupIsolatesOtherFixtureLiveInput`: two independent real PG/S3 fixtures contain identical content. A future GC clock exercises detach grace and deletes the first fixture's actual orphan while the other fixture's remote object and pinned input remain intact. Assert both fixture buckets differ from each other and from the environment bucket.

No production cleanup, media cache, editor, project-source, storage-provider UX, or deployment files changed.

## Exercised verification

Both runs loaded the local fixture environment without printing its values, required real PostgreSQL and S3, used a temporary HOME, set `GOWORK=off`, and reused the authorized Go module/build caches. Packages ran sequentially with `-p 1`.

1. Focused cleanup integration smoke:
   `go test -p 1 ./apps/viderse/internal/serverstorage -run 'TestGarage(CleanupRoleReplayAndPinnedCache|CleanupIsolatesOtherFixtureLiveInput|GCGraceReaderPinAndRootRecheck)$' -count=1 -v`
   - PASS, package elapsed 3.873s.
   - Actual cleanup with `Acquire` held: `freed=0 removed=0 usableBytes=43`.
   - Actual cleanup after release: `freed=43 removed=1 cacheDeleted=true`.
   - Cross-fixture cleanup: `deletedOwnBytes=60 otherRemoteIntact=true otherPinnedBytes=60`.
   - Reader pin, detach grace, and post-preview root recheck passed.

2. Full serverstorage suite, once after the coherent repair:
   `go test -p 1 ./apps/viderse/internal/serverstorage -count=1 -v`
   - PASS, 17 tests, no skips, package elapsed 10.086s.
   - Includes compilation, member replay protection, pin usability/release deletion, cross-fixture isolation, separate runtime-key bucket authorization, migration, key lifecycle, and materialization coverage.

## Handoff

The parent owns integration and should cherry-pick the repair commit and rerun the full Viderse all-package gate. No browser, protected application ports, remote production mutation, deployment, dependency installation, or shared-bucket deletion was performed.
