---
phase: 2
title: "Store contracts, pgx, goose và cấu hình backend"
status: completed (2026-10-28; postgres mode dev/migrate-only, not production-ready)
priority: P1
effort: 32h
dependencies: [1]
---

# Phase 02 — Tách Store mà không tạo database giả bằng file

## Context / yêu cầu

Chặn bởi 01; executor tuần tự, phase này độc quyền store facade/bootstrap/go.mod. [Schema](./reports/schema-and-contracts.md) và [research](./research/platform-storage.md) là contract. `internal/store/store.go:16-39` hiện dùng concrete substores; constructor `:53-79` cấp IO riêng/chia sẻ một số khóa; methods `:270-385` chạm trực tiếp `.io`, nên chỉ đổi kiểu field là chưa đủ. `internal/store/io.go:52-74,100-145` atomic file; `docs/architecture.md:38` nói rõ saga. Giữ sự thật này ở file backend.

Requirements: domain contracts nhỏ theo nghiệp vụ, hai backend; context/timeout/cancellation và errors ổn định; postgres pool + migration role khác application role; env parse không fallback; không đổi flow/router/LLM. Chưa bật postgres production trước domain implementations/08–09; backend chưa hỗ trợ domain phải startup fail với capability error, không stub/no-op hoặc lén đọc file.

## Kiến trúc / data flow và lifetime

Env/CLI/local config → validated `StorageConfig` → process-owned factory/pool → story-scoped facade → typed domain repository → transaction/query hoặc file backend. `STORE_DRIVER=file|postgres` và `DATABASE_URL` cho facts; `STORAGE_DRIVER=local|s3` cho blobs. CLI không có env database mặc định file; serve hosted bắt buộc explicit postgres, secret không nằm argv/log.

Giữ `store.NewStore(dir)` là constructor file offline thật, không alias deprecated. `store.Store` facade giữ field shape/compound methods hiện có; đưa concrete fields về domain interfaces, chuyển coordination vào backend operations, IO/private render helpers ở file implementation. `postgres` package implement cùng interface; composition root tạo facade với PG backend. Không stringify path thành SQL keys. Không duy trì backend `Dir()` trả fake directory: hosted callers cần stable identity hoặc explicitly acquired scratch capability ở 06, file-only callers vẫn có file-directory capability.

Pool/client shared process; store/story identity immutable theo request/run; actor tường minh request/transaction, không mutate factory/global/cache. Request pool connection không được giữ qua SSE/LLM/S3; tx rollback on cancellation. Job/engine lease là lifetime run, không HTTP session.

## Files / ownership

`R=/Users/dat/Projects/verse/apps/noverse`.

- Modify `R/go.mod:1-40`, `R/go.sum` (pgx/v5, goose/v3 pinned, `github.com/sqlc-dev/sqlc` pinned qua Go 1.24+ `tool` directive); `R/internal/store/store.go:16-107,270-385`; `R/internal/store/io.go:52-145`; `R/internal/bootstrap/configfile.go:98-165`; `R/cmd/ainovel-cli/serve.go:130-149`; `R/docs/architecture.md:38-58,98-112`; `R/README.md`.
- Create `R/internal/store/contracts.go`, `R/internal/store/backend.go`, `R/internal/store/file_backend.go`; `R/internal/persistence/postgres/pool.go`, `R/internal/persistence/postgres/transaction.go`, `R/internal/persistence/migrate/migrate.go`, `R/internal/persistence/migrations/00001_foundation.sql`; `R/internal/bootstrap/storage.go`; `R/cmd/ainovel-cli/storage.go` (storage migrate command); `R/sqlc.yaml`; `R/internal/persistence/postgres/queries/` (SQL viết tay theo domain); `R/internal/persistence/postgres/gen/` (code sqlc sinh, **commit vào repo**); tests `R/internal/persistence/postgres/transaction_test.go`, `R/internal/persistence/migrate/migrate_test.go`, `R/internal/bootstrap/storage_test.go`.
- File substores remain current files `R/internal/store/{book,progress,outline,drafts,characters,world,checkpoints,session,signals,run_meta,runtime,usage,simulation,decisions,chapter_records,revisions,cast,summaries,user_rules}.go` (field inventory verified `store.go:19-37`). Only touch concrete `.io` coupling needed by extraction; list exact changed substore files in implementation review.
- No destructive Delete. Cross-phase reuse serial: 04 adds account schema; 05 adds narrative; 06 engine wiring; 07 jobs. They do not edit these files concurrently.

## Steps

1. Run fail-before tests 01 then add contract tests for factory selection/transaction cancellation/CAS/typed errors. Preserve `os.IsNotExist` distinction in file adapter; shared contract returns domain not-found/conflict/forbidden, HTTP maps existing status.
2. Extract interfaces from **existing methods actually used**, not invent generic CRUD for 19 domains. Add compound operations for commit/outline/revision+checkpoint. Keep `Store` facade and file implementation so existing constructor callsites retain semantics; no blanket rename through hundreds of tests. Migration of hosted creation sites is enumerated in source inventory §4 and phase 06.
3. Implement PG factory with native `pgxpool`, bounded max conns/idle/lifetime/connect timeout. Factory Open ping validates selected backend and schema compatibility. Use `pgx/v5/stdlib` only for goose Provider `sql.DB`; close migration pool. Transaction helper calls Begin/commit/defer rollback and propagates original errors; no auto-retry side effects. **DB stack đã chốt 2026-10-28: pgx/v5 + goose/v3 + sqlc, không ORM.** SQL viết tay trong `internal/persistence/postgres/queries/` theo domain; `sqlc.yaml` trỏ schema là thư mục migrations goose và sinh typed Go vào `internal/persistence/postgres/gen/`; generated code được commit và regenerate bằng `go tool sqlc generate` (sqlc pin qua `tool` directive trong go.mod, không cần cài global). Repository methods gọi typed gen code trong transaction helper; không viết query string rải rác trong repository.
4. Implement `storage migrate up|status|version` command; embed SQL, goose session locker, PL/pgSQL blocks correctly delimited. One migration owner, explicit schema version; app never runs migration in handler. Foundation creates extensions and migration scaffolding; account/project/domain tables arrive by dependency order in 04/05/07/08.
5. Parse env strictly, report missing DATABASE_URL/bad driver/ambiguous local roots. Env overrides deployment config; story/project inherited creative config still follows current semantics and moves into DB with 05/06, not flattened globally. Never print DSN/password. Separate `MIGRATION_DATABASE_URL` role and `DATABASE_URL` role in deployment.
6. Implement test fixtures provisioning isolated database schema, no production DSN default; integration tests require explicit `TEST_DATABASE_URL`. Without env, unit may skip integration, but acceptance job requires env and fail if suite skipped. Add DB version compatibility bounds to readiness hook for 09.
7. Document maintenance cost of two backends and write authority. Do not expose incomplete postgres mode in public CLI help as production-ready; implementation remains staged until 08.

## Fail-before test matrix

| Level | Case | Expected gate |
|---|---|---|
| Unit | file default, explicit PG, invalid driver/DSN, env precedence | Bad config fails not file fallback |
| Contract | interface parity, nil/empty/corrupt errors, compound idempotence | Same fixtures/semantic outcomes |
| Integration | goose up twice, advisory migration lock two migrators, status, role cannot DDL | Exactly one schema progression; no races |
| Integration | tx forced failure/cancel after first write, CAS with two writers | No partial commit; one conflict |
| Integration | process restart with no disk library | No accidental initializer writes; incomplete capability fails loudly before 05/06 |

## Exact verification commands (implementer only)

```sh
go test ./internal/bootstrap -run TestStorageConfig -count=1
go test ./internal/store -run TestStorageContractFile -count=1
go test ./internal/persistence/migrate -run TestMigrate -count=1
go test ./internal/persistence/postgres -run 'TestTransaction|TestCAS|TestPoolLifecycle' -count=1
go run ./cmd/ainovel-cli storage migrate up
go run ./cmd/ainovel-cli storage migrate status
go tool sqlc vet
go tool sqlc diff
```

`go tool sqlc vet` kiểm queries hợp lệ với schema; `go tool sqlc diff` fail khi generated code lệch queries/schema (gate chống drift — commit gen code rồi verify trong CI/acceptance). Hai lệnh chạy được không cần database nhờ `tool` directive; không yêu cầu cài sqlc global.

## Risk / rollback / done

- **High (L cao × I cao):** abstraction becomes generic file backend over SQL. Mitigate typed tables/compound operations and forbid persistent file mirror in PG tests.
- **High (L vừa × I cao):** connection/actor reuse leaks tenant or pool exhaustion. Mitigate explicit immutable scopes, no session SET state, transaction deadlines, cleanup test.
- **High (L vừa × I cao):** concurrent DDL/cross-version rollout. Mitigate goose session locker, separate migration job, expand-only migration until cutover, version readiness.
- **Medium:** shared `.io` fields extraction changes lock ordering. File contract/race suite in 06 protects saga; no shared global transaction mutex.

Rollback this phase before PG activation: select file backend/revert code, foundation schema may remain unused. On nonempty DB do not down/drop extensions shared by other schemas. Gate: file parity holds, PG transaction/migration/config tests demonstrate actual operations, no business stub enabled in production; 03 can build BlobStore, 04 first domain can proceed once 03 delivers metadata boundary.

## Implementation evidence (2026-10-28, commit-pending)

- `internal/store`: `contracts.go` (19 domain interfaces extracted from caller-used methods), `backend.go` (`Backend` + `Substores` + `CapabilityError` + `RequiredDomains`), `file_backend.go` (concrete `file*Store` set + compound ops, saga semantics); `store.go` = thin facade, `NewStore(dir)` unchanged public API, `NewFromBackend(b)` = capability gate. Concrete substores renamed `file*Store` via ast codemod; external callers only needed `*store.X` → `store.X` at 2 sites (`internal/host/cocreate.go`, `internal/entry/web/review_items.go`).
- `internal/persistence/postgres`: `pool.go` (pgxpool + ping + `CheckSchema`/`SupportedSchemaFormat` readiness hook), `transaction.go` (`RunInTx`, `ExecCAS`→`errs.ErrStorageConflict`); `internal/persistence/migrate` (goose/v3 Provider + session locker, embedded `migrations/FS`); `migrations/00001_foundation.sql` (citext, pg_trgm, `noverse_meta`, `schema.format=1`).
- `sqlc.yaml` (schema=migrations, queries=`postgres/queries/meta.sql`, gen=`postgres/gen`, pgx/v5); `gen/` committed; `go tool sqlc vet`/`diff`/`generate` all clean.
- `internal/bootstrap/storage.go`: `StorageConfigFromEnv` (STORE_DRIVER/DATABASE_URL/MIGRATION_DATABASE_URL/DB_MAX_CONNS/DB_CONNECT_TIMEOUT, strict, DSN never in errors), `OpenPool`, `OpenStore` (postgres → pool+schema check → `*CapabilityError`; file → real backend). `cmd/ainovel-cli/storage.go`: `storage migrate up|status|version` (MIGRATION_DATABASE_URL → DATABASE_URL fallback). `serve` validates env at startup; `STORE_DRIVER=postgres` fails loud.
- `internal/errs`: `ErrStorageNotFound`/`ErrStorageConflict`/`ErrStorageForbidden`/`ErrStorageUnsupported`. `platform/pgtest`: per-test isolated schema via `search_path`, `REQUIRE_POSTGRES_TEST=1` fail-not-skip.
- Gate evidence (all PASS on disposable `postgres:17` @127.0.0.1:55432, container removed): `TestStorageContractFile|TestStorageAuthority` green; `TestMigrate*` 5/5 (idempotent Up, version, status, concurrent session-locker, app-role DDL refusal 42501); `TestTransaction*` + `TestCAS*` + `TestPool*` 7/7 (commit, rollback-on-error, rollback-on-cancel, CAS single/concurrent winner, lifecycle, schema gate); `TestStorageConfig*` 11/11 (strict env, no DSN leak); `TestBackend*` 4/4 (file defaults, capability gate, constructor parity, idempotent replay); `go tool sqlc vet`/`diff` clean; `go build ./...` clean; `go vet` clean on touched packages; `gofmt -l` clean on all touched files.
- Fail-before evidence: new tests captured red before implementation (undefined `postgres.Open`/`migrate.Up`/`StorageConfigFromEnv`/`Backend`); migration CLI exercised live: `status → chờ`, `up → applied`, `version → 1`, second `up → no pending`.
- Postgres mode is dev/migration-only: no domain repo exists, `serve` with `STORE_DRIVER=postgres` returns `backend postgres chưa khả dụng: … capability` at startup.
