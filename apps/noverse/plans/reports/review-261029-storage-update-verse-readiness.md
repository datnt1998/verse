# Review bản cập nhật lưu trữ Postgres và mức sẵn sàng cho kế hoạch monorepo `verse`

Ngày 2026-10-29.
- Phạm vi: `bc0681e..75c0b2b` (17 commit, 843 file, +64k dòng). Gồm Postgres backend, BlobStore, hàng đợi job và TTS worker từ xa, importer/reconcile/export ngược, deploy Dokploy.
- Thêm: một lượt kiểm tra nhanh trên working tree lúc 07:47. Thời điểm đó có một phiên khác đang sửa dở: 82 file, +2925/−646, thêm 3 migration chưa commit (`00008`–`00010`).

## 1. Gate đã chạy (trên HEAD `75c0b2b`)

| Gate | Kết quả |
|---|---|
| `gofmt -l`, `go vet ./...` | Sạch |
| `go tool sqlc diff` | **Lệch** ở HEAD: `gen/models.go` thiếu `ImportItem`/`ImportRun`/`LibrarySetting`. Working tree hiện đã sạch |
| `go test ./...` có Postgres 17 + MinIO, `REQUIRE_POSTGRES_TEST=1`, `REQUIRE_MINIO_TEST=1` | Qua (exit 0) |
| `go test ./...` chế độ CI (không DB) + `-race` cho `host`, `store`, `tools` | Qua |
| Web: `typecheck`, vitest (202 file / 1628 test), build `@noverse/ui` | Qua, không lệch file sinh |
| `docker build` + chạy bản sao local của `docker-compose.dokploy.yml` + `deploy/smoke.sh` | Image build được, stack lên `healthy`, smoke qua |
| Đăng nhập UI hosted bằng trình duyệt | Vào được. Trang dự án gọi 3 API trả `501`: `config`/`rules` cấp dự án và `validate` (plan đã ghi là chưa làm) |

**Lỗ hổng trong CI** (`.github/workflows/ci.yml` ở `75c0b2b`): không có service Postgres hay MinIO, không có job web. Mọi test Postgres và S3 đều bị skip trên CI. Typecheck và vitest không chạy trên CI. Đã bổ sung trên nhánh `fix/pre-verse-hardening`, xem plan `261029-0815-pre-verse-hardening`.

## 2. Lỗi tìm thấy

Nguồn:
- 3 reviewer độc lập: storage/import, jobs/worker/audio, bảo mật auth/proxy/deploy.
- Dòng ghi "(spot-check)" là em đã tự đọc code xác nhận lại.

Cột "Trạng thái" ghi tình trạng trên nhánh `fix/pre-verse-hardening`. Những dòng còn ghi "Chưa sửa", "Có thay đổi" hay "Chưa xác nhận" là tình trạng của bản sửa dở lúc 07:47. Bản sửa dở đó đã được commit ở `f10299f` và `574d987`; các lỗi ngoài phạm vi plan chưa được kiểm lại.

### High

| # | Lỗi | Vị trí | Bằng chứng | Trạng thái |
|---|---|---|---|---|
| H1 | Fence của story lease không phủ việc chấp nhận chương. Engine đã mất lease vẫn ghi được version chương mới | `postgres/backend.go:54-58,85`; `chapter_records.go`; `narrative.go` `AcceptRecord` | Tái hiện: sau `lease.Close()` vẫn trả `err=<nil> record=true` (spot-check) | **Đã sửa** (`2331818`) |
| H2 | Fence kiểu kiểm tra rồi mới ghi: `GetStoryLease` không có `FOR UPDATE`. Writer cũ ghi đè được dữ liệu của holder mới | `story_lease.go:123-146`; `queries/narrative.sql` `GetStoryLease` | Tái hiện: giá trị cuối là `{"owner":"stale"}` (spot-check) | **Đã sửa** (`2331818`), kèm sửa vòng gia hạn lease giành lại lease đã mất |
| H3 | Lỗi commit của document bị nuốt. `WriteJSON` trả thành công rồi commit mới chạy trong `Unlock → EndWrite`, lỗi chỉ hiện ở thao tác sau | `postgres/docstore.go:315-328`; `store/io.go:114-118` | Tái hiện bằng `TestDocumentWriteReportsCommitFailure` | **Đã sửa** (`2331818`) |
| H4 | `revision.Scan` vẫn `os.ReadFile(st.Dir()/chapters/..)`. Trên Postgres `Dir()==""` nên đọc vào CWD và chặn viết tiếp sau chương đầu | `revision/scan.go:41-44`; `host.go` `requireCleanChapters` | Tái hiện bằng `TestRevisionCheckPostgres` | **Đã sửa** (`8300810`) |
| H5 | Import làm mất `facts`, `style_delta`, `accepted_at` của bản ghi chương, nhưng reconcile vẫn báo "0 lệch" | `storageimport/decode.go:651-681`; `rows.go:285-291`; `reconcile.go` | Tái hiện bằng fixture | **Đã sửa** (`f67a2fc`) |
| H6 | Import lưu plan, chương và nhân vật dưới khoá mà Store production không đọc (`chapter_plan/drafts/..` thay vì `02`, thiếu `chapter_text`, thiếu `story_state(characters)`) | `storageimport/rows.go:111-127,298-349`; `docstore.go:95-112` | Tái hiện | **Đã sửa** (`f67a2fc`) |
| H7 | Bản ghi chương xuất ngược bị file backend từ chối (`content digest mismatch`) | `storageimport/export.go:406-424` | Tái hiện | **Đã sửa** (`f67a2fc`) |
| H8 | Xuất ngược bỏ sót tài khoản, quyền, checkpoint, sự kiện, lịch sử version. Ngoài ra bỏ sót dự án và blob tạo sau cutover, và blob thiếu thì vẫn báo thành công. Kết quả là xuất ngược **không dùng làm rollback được** | `storageimport/export.go:91-149,617-645` | Tái hiện. `TestExportRoundTrip` che lỗi vì dùng lại tài khoản gốc | **Đã sửa** (`f67a2fc`); giới hạn còn lại ghi ở `import-export-fidelity-261029.md` |
| H9 | Không kiểm byte file sự kiện (fact file) giữa lúc scan và lúc import | `storageimport/importer.go:432-463` | Lần theo code | **Đã sửa** (`f67a2fc`) |
| H10 | Worker ACK audio cũ sau khi script đã đổi, và audio cũ vẫn được chọn | `entry/web/worker_api_pg.go:158-231`; `audio/pg_views.go` | Tái hiện | Có sửa `pg_views.go` (thêm `persistScript`/`SaveScript`), chưa xác nhận đã chặn được |
| H11 | Export audio chạy trên dữ liệu hiện tại thay vì snapshot ứng với cache key, nên cache bị đầu độc | `audio/pg_export.go:191-206,365-397` | Tái hiện có ffmpeg thật | Có thay đổi, chưa xác nhận |
| H12 | Lease export 120 giây không được gia hạn. Hết hạn thì runner dừng hẳn cho đến khi khởi động lại server | `audio/pg_export.go:310-347`; `serve.go` | Lần theo code | Có thay đổi, không thấy heartbeat |
| H13 | Phát audio cả chương trên hosted không có nguồn ghi `chapter-audio:` | `audio/pg_play.go:168-186` | Tái hiện. Grep chỉ thấy phía đọc (spot-check) | Chưa sửa |
| H14 | Username viết hoa né được giới hạn đăng nhập theo tài khoản (cột `citext` so khớp không phân biệt hoa thường, còn khoá limiter chỉ được thêm khi `ValidUsername` qua) | `entry/web/auth.go:115-119`; migration `00002:12` | Tái hiện bằng `TestGatewayLoginRateLimitIgnoresUsernameCase` | **Đã sửa** (`104388d`) |

### Medium

| # | Lỗi | Vị trí | Trạng thái |
|---|---|---|---|
| M1 | Limiter Postgres không tăng dần thời gian khoá, cố định 2 giây | `postgres/auth_limiter.go:86-97` (spot-check) | **Đã sửa** (`104388d`) |
| M2 | Link tải export audio trên hosted luôn `404` vì không có route `/api/blobs/*` | `audio/pg_export.go`; `entry/web/blob.go` (spot-check) | Chưa sửa |
| M3 | Xuất ngược ghi sai đường dẫn (`du-an/du-an/.ainovel/..`, `chapter_plan/` thừa) | `storageimport/export.go` | **Đã sửa** (`f67a2fc`) |
| M4 | Xuất ngược làm sống lại nội dung đã xoá mềm | `storageimport/export.go` | **Đã sửa** (`f67a2fc`) |
| M5 | Chưa chuyển được tài khoản v1/v2 (role `writer`/`reader`) | `storageimport/decode.go:474-499` | Chưa sửa |
| M6 | Chương cũ chỉ có prose: comment/audio được import trước chương, vi phạm FK | `storageimport/importer.go` | Chưa xác nhận |
| M7 | Xoá truyện tạm số lớn nhất thì không tạo được truyện tạm mới (đụng unique) | `postgres/library.go:351-390` | Chưa xác nhận |
| M8 | Cache key của export bỏ qua tên sách và tên chương | `audio/pg_export.go` | Chưa xác nhận |
| M9 | Worker từ xa bỏ qua pitch nhưng vẫn ghi là đã áp | `ttsworker/worker.go:282-323` | Chưa sửa |
| M10 | Một job lỗi làm dừng cả worker | `ttsworker/worker.go:160-168` | Chưa sửa |
| M11 | Thay đổi hàng đợi trên hosted không tới được giao diện đang mở (live update) | `entry/web/audio.go`; `worker_api_pg.go` | Chưa xác nhận |
| M12 | Upload bị huỷ hoặc lỗi để lại blob mồ côi; không có cơ chế dọn | `entry/web/worker_api.go:358-388` | Chưa sửa |

### Low

- Bảng `login_rate_limits` không bao giờ được dọn. `PurgeExpiredLimitBuckets` không có ai gọi (spot-check).
- Khi `TRUSTED_PROXIES` để trống thì mất quy tắc loopback, nên cookie sau `tailscale serve` thiếu `Secure`. WIP chỉ bỏ alias, chưa sửa quy tắc.
- Một lần lookup audio lỗi thì truyện đó hỏng đến khi khởi động lại (`audio/pg.go` cache cả kết quả lỗi).
- Nghi vấn: cách mở cổng worker trong `deploy/README.md` §3.4 có thể không làm được trên Dokploy (Swarm), khiến API worker lộ ra internet. Vẫn cần token 384-bit.

### Phần đã kiểm và ổn

- Claim job loại trừ nhau (`SKIP LOCKED`); fence của worker, thu hồi job hết hạn, huỷ khi đang upload.
- Xác thực worker: hash token, thu hồi được, có kiểm capability. Token worker không gọi được API người dùng.
- Phân quyền member/owner/admin khi chạy đồng thời; phiên đọc lại user mỗi request; mật khẩu tạm dùng một lần.
- Chống CSRF ở hosted (`Sec-Fetch-Site` + `Origin`); proxy tin cậy chỉ đọc header từ CIDR được khai.
- Role DB: app không có quyền `CREATE`, migrate role sở hữu schema. Docker chạy non-root. `backup.sh` kiểm checksum.
- Migration `Up` 1–7 chạy được trên schema cô lập. Trigger bất biến cho version chương hoạt động.

## 3. Ảnh hưởng tới kế hoạch monorepo `verse`

**Lớp core gần như không đổi.**
- `agent`, `llm`, `models`, `llmretry`, `llmcontract`, `auth`, `codex`, `anthropicoauth`, `xai` chỉ thay đổi 1 file (+21 dòng).
- Phụ thuộc giữa các package vẫn gọn: `agent→llm`, `llm→models`, adapter OAuth → `auth`, `llm`.
- 134 file ngoài core import các package này. Tổng cộng 737 file Go import module path.

**Những gì đổi trong kế hoạch:**

1. **M0 chưa bắt đầu được.**
   - Working tree đang có WIP của phiên khác (3 migration chưa commit). Tag `pre-verse` phải đặt trên tree sạch, đã commit và qua gate.
   - Nếu chuyển repo lúc này thì phần WIP sẽ bị tách đôi giữa hai repo.
2. **Không được chạy `storage import` dữ liệu live hay cutover live sang Postgres** cho tới khi sửa xong H1–H8.
   - Lý do: import làm mất dữ liệu nhưng reconcile vẫn báo khớp, và xuất ngược không dùng làm rollback được.
   - Live hiện chạy chế độ file (`75c0b2b`, pid 17766, không có `STORE_DRIVER`), nên chưa bị ảnh hưởng.
3. **Quyết định về DB trong bản tư vấn trước đã lỗi thời.** Noverse giờ có 2 backend: file khi chạy local, Postgres khi hosted (cả tài khoản lẫn dữ liệu truyện).
   - Ở M4 phải quyết: tổ chức Postgres cho nhiều app thế nào (một cluster, schema riêng mỗi app + schema `identity` chung; hay mỗi app một database) và `platform/identity` có 2 backend file/Postgres.
4. **SSO khi hosted.** Các app ở subdomain khác nhau nên cookie chỉ theo host là không đủ, cần cookie gắn domain cha.
   - `crossSiteRequest` hiện chỉ chặn `cross-site`. Request từ app anh em cùng site (subdomain khác) chỉ bị chặn nhờ kiểm `Origin` khác host. Phải giữ nguyên kiểm tra này khi chuyển sang cookie domain cha.
   - Kiểm `Origin` mới chỉ bật khi hosted (`PublicURL != nil`), chế độ local chưa có.
5. **Phân lớp các package mới:**
   - Lên `platform/` được: `blobstore` (không phụ thuộc gì), `persistence/migrate` và `persistence/pgtest` (dùng chung), `notify`, `logger`.
   - Phải ở `apps/noverse`: `persistence/postgres` (phụ thuộc `audio`, `comments`, `library`, `rules`, `store`, `webauth`), `storageimport`, `ttsworker`, các migration.
   - `webauth` vẫn phụ thuộc `library` (`ValidProjectID`).
6. **Danh sách đường dẫn M1 phải sửa dài thêm:**
   - `Dockerfile` (`web/`, `internal/entry/web/dist`, `./cmd/ainovel-cli`), `sqlc.yaml` (3 đường dẫn), `docker-compose.dokploy.yml` (build context), `.goreleaser.yml`, `.github/workflows/{ci,docker,release}.yml`, `deploy/*.sh`.
   - Nguồn repo trên Dokploy, `run-live.sh`.
   - `@noverse/ui` nằm trong `web/packages/ui`, giữ nguyên chỗ trong `apps/noverse/web` ở M1.
7. **Gate CI phải có trước khi chuyển repo:** service Postgres + MinIO với `REQUIRE_*=1`, job web (typecheck + vitest), và `sqlc diff`.

## 4. Thứ tự đề xuất trước M1

1. Phiên đang sửa commit xong WIP; tree sạch.
2. Sửa H1–H4 và H14 (tính đúng của engine và auth). Sửa H5–H8 trước bất kỳ lần import hay cutover nào.
3. Thêm gate CI ở mục 3.7; chạy toàn bộ gate qua.
4. Chạy smoke architect ở HEAD với `--timeout 60m`; gắn tag `pre-verse`.
5. Bắt đầu M1: dời code nguyên trạng vào `apps/noverse`.
