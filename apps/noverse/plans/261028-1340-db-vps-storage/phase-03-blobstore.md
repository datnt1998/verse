---
phase: 3
title: "BlobStore local/S3, multipart, streaming và copy đối soát"
status: done
priority: P1
effort: 32h
dependencies: [2]
implemented: 2026-10-28 — evidence in /tmp/lm-reports-bug2710.md ##DbPhase03
---

# Phase 03 — Một BlobStore với local và S3-compatible

## Context / requirements

Sau 02, trước data references ở 04–07. [Research provider](./research/platform-storage.md) và [blob protocol](./reports/schema-and-contracts.md#5-transaction-boundaries-và-blob-protocol). Hiện binaries/cache dựa đường dẫn: `ReadSegmentAudio` (`internal/audio/cache.go:181-198`), `serveAudioFile` (`internal/entry/web/audio.go:1348-1365`), story export download (`internal/entry/web/storyread.go:912-917`), ZIP file reads (`internal/audio/export_files.go:49-56`). Phase này tạo driver/copy + authenticated delivery seam; 07 mới chuyển audio callers.

Bắt buộc: local driver; **một** aws-sdk-go-v2 driver với knobs dùng cho AWS/R2/GCS/MinIO/Wasabi/B2/Spaces; presigned GET + fallback streaming/range; multipart upload; một conformance suite chạy local và MinIO; store-to-store copy verify download SHA-256. Không giả đã test mọi provider chỉ vì MinIO pass.

## Architecture / data flow

Proposed `BlobStore` methods: `Put(ctx,key,reader,PutOptions) (BlobInfo,error)`, `Open(ctx,key,ByteRange) (ReadCloser,BlobInfo,error)`, `Stat`, `Delete`, `List(ctx,prefix,cursor,limit)`, `PresignGet(ctx,key,ttl,DownloadOptions)`. `ErrPresignUnsupported` là capability bình thường của local, không fake URL. Multipart là nội bộ `Put`; caller không phải biết SDK. `BlobInfo` có key/size/type/SHA256 metadata; không đồng nhất ETag với digest.

Upload streaming → temp/SDK multipart → compute canonical SHA-256 bytes → verify → metadata attach ở domain tx. Object key immutable theo tenant UUID, không theo title/slug/user filename. Local confined root+symlink defense; reader size/timeout cap. HTTP authorization lấy tenant từ DB blob record trước presign/open. Private signed URL là bearer capability ngắn hạn (default 60s) nên revoke quyền có cửa sổ tối đa TTL; endpoints nhạy cảm dùng streaming reauth thay signed redirect. Public base URL chỉ cho public assets đã được đánh visibility public.

`STORAGE_DRIVER`, `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_FORCE_PATH_STYLE`, `S3_PUBLIC_BASE_URL` đầy đủ; local `BLOB_LOCAL_ROOT`. BaseEndpoint/UsePathStyle và WHEN_REQUIRED checksum policy theo research; GCS HMAC endpoint `storage.googleapis.com`, region `us-east-1`, R2 region `auto`, MinIO path-style. GCS không có multipart precondition: DB CAS+unique immutable key làm authority.

## Files / ownership

`R=/Users/dat/Projects/ainovel-cli`.

- Modify `R/go.mod`, `R/go.sum` (SDK v2 config/credentials/s3/manager cùng version family), `R/internal/bootstrap/storage.go` (new 02), `R/cmd/ainovel-cli/storage.go` (new 02), `R/README.md`.
- Create `R/internal/blobstore/blobstore.go`, `local.go`, `s3.go`, `copy.go`, `conformance_test.go`, `local_test.go`, `s3_test.go`, `copy_test.go`; `R/internal/entry/web/blob.go`, `blob_test.go`; `R/deploy/compose.test.yml` (Postgres+MinIO test-only, không proxy thay Dokploy). Blob metadata DDL nằm trong migration04 sau projects, không tạo standalone migration ở03.
- Read/caller integration later: `R/internal/audio/cache.go:181-212`, `R/internal/audio/export_result_cache.go:34-50`, `R/internal/entry/web/audio.go:1348`, `R/internal/entry/web/storyread.go:912-917`. Không chuyển nửa audio persistence ở phase này.
- File ownership tuần tự, 04 nhận migrations/gateway; không Delete existing local/offline exports.

## Steps

1. Giữ thứ tự FK: physical driver/conformance/copy độc lập database trong03; metadata bảng `blobs` tạo trong `00002_accounts_projects_blobs.sql` do04 sở hữu sau projects. Delivery seam không được public-wire trước authorizer04. Không tạo migration tham chiếu project chưa tồn tại.
2. Viết conformance suite và fail-before cases rồi implement local atomic file driver. Root phải absolute; reject `..`, absolute/empty keys, backslash traversal, symlink parent escape; temp+fsync+rename và dir fsync. Streaming/pagination không load cả bucket; Delete not-found idempotent theo contract, other failures propagate.
3. Implement one S3 client with explicit static credentials, optional BaseEndpoint, required region/bucket, UsePathStyle bool strict. SDK manager uploader concurrency/part size bounded; choose content length/seekable input where possible, streaming unknown length has upload size/part-count cap. Abort multipart on cancellation/error. No provider-specific driver duplicates.
4. Implement signed GET and authenticated streaming fallback with HEAD/range (single ranges including suffix, 206/416, Content-Range/Length), safe Content-Disposition filename, CSP/cache headers for private content. Do not accept arbitrary remote URL from request; SDK endpoint env admin-only. Presign errors return fallback route; browser signed GET expiry/network failure can retry fallback once. At 03 delivery seam tests use injected authorizer; real account/project repo authorization wired 04/05, not fake public handler.
5. Implement `storage copy --from-config <path> --to-config <path> --manifest <path> --resume` (proposed). Config files mode0600 outside repo contain no hardcoded secrets in argv; support local↔s3 and s3↔s3. List paginated, stream source/upload, download target and compare SHA-256+size; ledger per immutable key. Same target hash skip, different target hash fails conflict; no overwrite/delete-source default. Exit nonzero on any mismatch and do not switch live pointer.
6. Document all seven provider recipes, CORS/private bucket rules, optional public URL and GCS HMAC specifics. Conformance mandatory local+MinIO; run provider smoke when credentials obtained, do not promote a row to “tested” by documentation.

## Fail-before matrix

| Level | Cases | Gate |
|---|---|---|
| Unit | local traversal/symlink/concurrent put, read/cancel/hash/empty key | No root escape/partial visibility |
| Unit | SDK option parse/invalid bool/endpoint/region, public private separation | Correct BaseEndpoint/path-style, no URL host rewrite after signing |
| Conformance local+MinIO | put/head/get/list/pagination/delete, Unicode keys, empty/>5MiB object, multipart abort, corrupt transport | Same suite, real SDK/network for MinIO |
| HTTP integration | auth denial, private header, Range/suffix/HEAD/416, presign unsupported/expired | Streaming preserves byte ranges and checks permission |
| Integration copy | local→MinIO→new local, resume after killed copy, same-size corruption | Source/target download SHA-256 match, zero changed pointer on mismatch |
| Provider acceptance | production chosen S3 endpoint, GCS checksum/presign/multipart when GCS chosen | Report measured behavior; fallback actually serves bytes |

## Verification commands (not executed here)

```sh
docker compose -f deploy/compose.test.yml up -d postgres minio
go test ./internal/blobstore -run TestConformanceLocal -count=1
go test ./internal/blobstore -run TestConformanceMinIO -count=1
go test ./internal/blobstore -run 'TestLocalConfinement|TestMultipartAbort|TestCopyResumeHash' -count=1
go test ./internal/entry/web -run TestBlobDelivery -count=1
go run ./cmd/ainovel-cli storage copy --from-config "$COPY_FROM_CONFIG" --to-config "$COPY_TO_CONFIG" --manifest "$COPY_MANIFEST" --resume
```

Test MinIO phải fail nếu required config absent trong CI acceptance (`REQUIRE_MINIO_TEST=1`); không pass bằng skip. Paths/flags mới là deliverables. Copy manifest phải có mismatch=0 và copied+matched=source_objects.

## Risks / rollback / done

- **High (L cao × I cao):** checksum/presign GCS incompatibility. Mitigate SDK pin + WHEN_REQUIRED policy + independent hash, live smoke/fallback; not downgrade to public bucket.
- **High (L vừa × I cao):** local escape/SSRF/private URL exposure. Confine roots/keys, endpoint env only, authorize before lookup, private TTL/cache headers.
- **High (L vừa × I cao):** orphan multipart/copy half-cutover. Abort/lifecycle cleanup; immutable staging, checkpoint/hash copy; switch only after full reconcile.
- **Medium:** audio blob RAM amplification. Stream large objects, bounded uploader concurrency and scratch quota; existing []byte TTS segment kept bounded per segment, not whole volume.


## Implementation evidence (2026-10-28)

- `internal/blobstore/`: `blobstore.go` (interface, `ByteRange`, `BlobInfo`, sentinels `ErrNotFound/ErrInvalidKey/ErrInvalidRange/ErrPresignUnsupported/ErrTooLarge/ErrDigestMismatch`, `ValidateKey`), `local.go` (os.Root confinement, temp+fsync+rename+dir-fsync, `.tmp-` invisible, 8 GiB cap), `s3.go` (one aws-sdk-go-v2 driver: BaseEndpoint/UsePathStyle/strict bool/WHEN_REQUIRED checksum, manager uploader part≥5MiB concurrency≤4 parts≤10 000, abort on error, presign TTL≤15min default 60s, `sha256` user-metadata), `copy.go` (paginated list→stream→Put→re-download SHA-256+size verify, JSONL ledger mode0600, --resume re-Stat, conflict never overwritten).
- `internal/bootstrap/storage.go`: `BlobConfig`/`BlobConfigFromEnv`/`BlobConfigFromFile`(0600)/`OpenBlobStore` với env contract đầy đủ + `S3_ALLOW_INSECURE_ENDPOINT` opt-in cho http endpoint.
- `internal/entry/web/blob.go`: `BlobDelivery` với injected `BlobAuthorizer` — `GET|HEAD /api/blobs/stream/{key}` (206/416/404/403, `Content-Range`, `X-Content-SHA256`, `Cache-Control: private`, CSP sandbox) và `GET /api/blobs/sign/{key}` → `mode=presigned` hoặc fallback `mode=stream`. Chưa gắn route production; live giữ file mode.
- `cmd/ainovel-cli storage copy --from-config --to-config --manifest [--resume] [--concurrency]`: exit non-zero khi conflicts/mismatches/errors >0.
- `deploy/compose.test.yml`: postgres:17 @55432 + minio @59000/59001 (tmpfs, test-only).
- Fail-before đã quan sát: bỏ `ValidateKey` trong `Stat` → `TestConformanceLocal/invalid_keys` FAIL; bỏ range bounds → `range_read` FAIL (`out-of-range err = <nil>`); `LeavePartsOnError=true` → `TestMultipartAbort` FAIL (`orphaned multipart uploads`); bypass authorize → `auth_denial`/`nil_authorizer` FAIL (200 thay vì 403/500).
- Xanh: `TestConformanceLocal` + `TestConformanceMinIO` (10 subtest mỗi bên, gồm 9MiB multipart, unicode, pagination, presign), `TestLocalConfinement` (symlink escape/`..`/backslash), `TestMultipartAbort`, `TestCorruptTransportDetection`, `TestCopyResumeHash`, `TestCopyConflictAndCorruption`, `TestCopyLocalToMinIO` (local→MinIO→local chain), `TestBlobDelivery` (11 subtest), `TestS3ConfigValidation`, `TestBlobConfig` (8 subtest). `go build ./...`, `go vet`, `gofmt` sạch; `internal/persistence` + `internal/bootstrap` xanh.
- CLI live run: local→MinIO 3 objects (gồm 7MB) → `copied=3`, verify SHA-256 download; rerun --resume → ledger skip; same-size different-content → `conflict`, exit 1, target không bị ghi đè.
- Chưa wire production: audio callers (`cache.go`, `audio.go:1348`, `storyread.go:912`) giữ file mode; authorizer thật chờ 04/05; chỉ local+MinIO đã conformance — AWS/R2/GCS/Wasabi/B2/Spaces là recipe cấu hình, cần smoke khi có credentials.
Rollback: leave old driver/bucket authoritative, stop writes during pointer switch; never delete source. If switched, freeze, copy delta back with hash and update metadata/driver atomically after validation; no writes to both stores. Done: suite same code passes local+MinIO, >5MiB upload proved multipart, canceled multipart absent, copy corruption detected, authenticated fallback delivers exact range. Production auth wiring remains blocked on 04 by design, not advertised complete until integrated.
