---
title: "Nâng cấp database, BlobStore và triển khai Dokploy"
description: "Đưa dữ liệu nghiệp vụ vào Postgres, giữ CLI offline và chạy web trên VPS Dokploy với BlobStore và worker TTS tại nhà."
status: in-progress
priority: P1
effort: 280h
branch: main (repo verse)
tags: [database, backend, infra, auth, refactor]
blockedBy: []
blocks: []
created: 2026-10-28
---

## Trạng thái (2026-10-29)

| Phase | Trạng thái |
|---|---|
| 01–06, 08 | Xong. Bằng chứng nằm trong từng phase; riêng 03, 07c, 08 ở [reports/handover-evidence.md](./reports/handover-evidence.md) |
| 07 | Còn mở: chưa nghiệm thu VieNeu thật, chưa có runner `text_export`, export hosted chưa có bìa/nhạc. Xem [phase 07](./phase-07-audio-jobs.md#trạng-thái-hiện-tại-đối-chiếu-code-verse-2026-10-29) |
| 09 | Gate cục bộ qua; gate production còn 7 mục cần VPS, domain, S3 và credentials thật. Xem [phase 09](./phase-09-dokploy-cutover.md#còn-lại-trước-khi-ký-release-gate-production) |

Production chưa chuyển. Live 8790 vẫn chạy file mode.

**Quy ước đường dẫn sau khi chuyển sang monorepo `verse` (2026-10-29):**

- Plan này nằm ở `apps/noverse/plans/`. Đường dẫn trong plan tính từ `apps/noverse/`, tức gốc repo cũ `ainovel-cli`, nên `internal/…`, `cmd/…`, `web/…`, `deploy/…` vẫn đúng.
- Code đã dời ra ngoài app ghi theo gốc repo `verse`:
  - `core/…`: `agentcore`, `llm`, `models`, `llmretry`, `llmcontract`, `auth`;
  - `platform/identity`: trước là `internal/webauth`;
  - `platform/pgtest`;
  - `platform/webguard`: proxy và Origin, trước ở `internal/entry/web/proxy.go`.
- Số dòng `file.go:N` là lúc viết, có thể đã lệch.
- Thư mục `reports/` và `research/` giữ nguyên đường dẫn lúc viết.
- Lệnh `go test ./internal/…` chạy được từ `apps/noverse/`. `go test ./...` phải chạy ở gốc repo mới phủ cả `core/` và `platform/`.

# Kết quả cần đạt

Web chạy sau Traefik của Dokploy, dùng Postgres trong Dokploy làm nguồn dữ liệu chính; không cần bản sao `~/NoverseLive` trên VPS. CLI offline vẫn dùng file. Văn bản, phiên bản chương, quyền, phiên đăng nhập và trạng thái công việc nằm trong database; âm thanh, ảnh, tệp nhập/xuất đi qua `BlobStore`. Không thêm Caddy, SQLite, Redis, OAuth UI hoặc GPU trên VPS.

## Quyết định đã chốt — 2026-10-28

1. **Postgres là database chính**, không dùng SQLite. Đổi DSN/version qua env; các nghiệp vụ phụ thuộc repository, không phụ thuộc driver. Quyết định Dokploy + Postgres đã khóa; câu xác nhận này ghi rõ mặc định vận hành, không mở lại SQLite.
2. **VieNeu ở Mac tại nhà**, worker chủ động kéo việc qua Tailscale. VPS chạy web/Go/ffmpeg nhưng không nạp model TTS. Đổi nơi chạy worker bằng endpoint và worker credentials, không đổi schema/API.
3. **Giữ file mode cho CLI/offline**, hai implementation `file` và `postgres`, không đồng bộ hai chiều tự động. Chi phí: mọi contract và thay đổi nghiệp vụ phải qua hai backend; ước lượng đã gồm chi phí này. Chọn bằng `STORE_DRIVER=file|postgres`, tách với `STORAGE_DRIVER=local|s3` dành cho blob. Bỏ file mode về sau là quyết định sản phẩm riêng, không tự cắt ở kế hoạch này.
4. **DB stack: pgx/v5 + goose/v3 + sqlc.** SQL viết tay trong `internal/persistence/postgres/queries/`, sqlc sinh typed Go (đã commit) từ queries + schema; không dùng ORM. sqlc được pin qua `tool` directive trong `go.mod` (không cần cài global); `go tool sqlc diff`/`vet` là gate chống drift. Chi tiết tại [phase 02](./phase-02-store-contracts.md).

## Kiến trúc và luồng dữ liệu

Browser → Traefik TLS → trusted-proxy middleware → session/CSRF + authorization → repository theo actor/project/story → Postgres transaction → SSE đã lọc quyền. Engine/agent tools đi qua cùng `Store`, không qua thư mục làm database giả. Blob: quyền truy cập → tra `blobs` → presigned GET ngắn hạn hoặc streaming/range từ server. Worker Mac: Tailscale + credential giới hạn TTS → claim job có lease/fence → đọc input snapshot → VieNeu/ffmpeg → upload blob → acknowledge có hash/fence; không được truy cập trực tiếp Postgres. Xem [hợp đồng và schema đầy đủ](./reports/schema-and-contracts.md), [bằng chứng mã nguồn](./reports/codebase-evidence.md), [nghiên cứu giới hạn](./research/platform-storage.md).

`internal/store/store.go:16` hiện là composition facade với 19 concrete substore; giữ facade để tránh đổi toàn bộ hình dạng API, chuyển các trường sang domain contracts, giao compound operation cho backend. File backend giữ format/saga hiện có; postgres backend dùng transaction thực. Đây không phải SQL key/value filesystem và không ghi database rồi ghi lại file. Shared pool/factory là process-scoped; actor là immutable request/run-scoped, tuyệt đối không đặt actor trong `Gateway` hoặc `audioServices` cache (`internal/entry/web/gateway.go:73`, `internal/entry/web/audio.go:94`).

Xem [ERD Auth](./reports/schema-and-contracts.md#7-erd--auth), [ERD dự án/nội dung/jobs/audio/blobs](./reports/schema-and-contracts.md#8-erd--dự-án-nội-dung-jobs-audio-và-blobs) và [ma trận phân quyền cùng đối chiếu code hiện tại](./reports/schema-and-contracts.md#9-ma-trận-phân-quyền-và-đối-chiếu-mã-nguồn). Hai ERD thể hiện PK/FK/cardinality; ma trận phân biệt quyền hiện có với owner-scoped membership/transfer và worker permissions cần bổ sung, không mở rộng quyền admin API cho project owner.

## Các giai đoạn và phụ thuộc

| Giai đoạn | Kết quả | Chặn bởi | Owner | Ước lượng |
|---|---|---|---|---|
| 01 [Khóa inventory, schema và contract](./phase-01-start.md) | Fixture, schema, parity contract | Không | Storage lead | 20h |
| 02 [Store contracts, pgx và goose](./phase-02-store-contracts.md) | Hai backend, transaction/bootstrap | 01 | Repository owner | 32h |
| 03 [BlobStore local/S3 và copy](./phase-03-blobstore.md) | Multipart, GET fallback, hash copy | 02 | Blob owner | 32h |
| 04 [Accounts, sessions, devices, members, audit](./phase-04-accounts.md) | Miền đầu tiên chuyển Postgres; giữ user management | 02,03 | Identity owner | 40h |
| 05 [Projects, stories, chapters và các dữ kiện](./phase-05-story-domains.md) | CRUD/version/search/comments/reviews | 04 | Domain owner | 40h |
| 06 [Engine, tools và direct IO cutover](./phase-06-engine-cutover.md) | Không đọc file nghiệp vụ trên DB mode | 05 | Engine owner | 44h |
| 07 [Audio, jobs và Mac worker](./phase-07-audio-jobs.md) | Queue/lease/export cache bền vững | 03,06 | Audio owner | 36h |
| 08 [Importer và đối soát](./phase-08-import-reconcile.md) | Import lặp an toàn, restore/reverse export | 04–07 | Migration owner | 20h |
| 09 [Dokploy và cutover có phục hồi](./phase-09-dokploy-cutover.md) | Docker/env/health/proxy/backups | 08 | Deployment owner | 16h |

**Thực thi tuần tự 01→09.** Không chạy song song các phase dù có thể có dependency mềm. Mỗi phase giữ quyền sửa các file được liệt kê cho đến handoff; `go.mod`, `go.sum`, `internal/store/store.go`, gateway và docs được chuyển quyền theo thứ tự, không hai owner sửa cùng lúc. Không có phase nào được triển khai production trước importer rehearsal/restore gate của 08 và security/deployment gate của 09. `pending` là trạng thái implementation, không phải báo đã triển khai.

## Tương thích, test và điều kiện hoàn thành

Giữ URL slug, JSON response, global role `admin|member`, project role `owner|collab|view` và flow mật khẩu tạm hiện tại (`platform/identity/store.go:39-41,113-146`). `devices` là tài khoản thiết bị liên kết phiên, không tự phát minh khóa máy. User file hiện dùng username làm identity, không có UUID (`platform/identity/store.go:63-79`): importer cấp UUID ổn định theo source+username, giữ hash Argon2id nguyên byte; session cũ chủ động hết hiệu lực một lần khi cutover. Username 2–32 ký tự lowercase được giữ; unique case-insensitive không cho biến thể chiếm tài khoản. Mật khẩu tạm được thêm TTL/one-time theo yêu cầu, không bỏ bước bắt buộc đổi mật khẩu.

Done đo được: toàn bộ fixture/import manifest được phân loại; lần import thứ hai không thêm row/blob; reconciliation counts + SHA-256 không sai; FK/invariant/concurrent-owner tests đạt; cùng repository/blob conformance qua file+postgres/local+MinIO; worker tắt/mất mạng không mất việc và stale worker không publish; rebuild container không mất dữ liệu; browser login/đổi mật khẩu/quyền/viết-sửa chương/comment/TTS/export hoạt động qua HTTPS. Ma trận và lệnh cụ thể ở từng phase là lệnh **cho implementer**, chưa chạy trong lượt lập kế hoạch.

Rollback trước nhận traffic: giữ nguyên nguồn file, dừng DB writers rồi quay bản release/file mode cũ. Sau nhận traffic: **không** trỏ lại snapshot file cũ; đóng ghi, reverse-export/đối soát dữ liệu phát sinh (08) hoặc chạy release DB-compatible trước đó với schema expand-only. Không chạy `goose down` phá dữ liệu khi còn writer. Backup Postgres và manifest blob cùng checkpoint, restore vào database/bucket riêng trước khi thay production.

## Red Team Review

Phiên 2026-10-28: tự thực hiện bốn lens (Security, Failure, Assumption, Scope) + bốn verification roles; runtime không expose công cụ spawn reviewer/kongming, nên **không phải review độc lập bởi subagent**. 8 findings: 2 Critical, 5 High, 1 Medium; nhận 8, bác 0. Bằng chứng, disposition, mitigation và liên kết phase ở [gate record](./reports/gates.md). Các delta đã đưa thẳng vào bản kế hoạch: account parity, owner invariant, proxy spoofing, engine hidden IO, fencing, GCS checksum/presign fallback, rollback sau ghi và tránh filesystem-mirror abstraction. Không có code/test/build nào được chạy để giả làm bằng chứng.

## Validation Log

Đã tự validate theo Full tier: Fact Checker, Flow Tracer, Scope Auditor, Contract Verifier; đối chiếu symbol/path, luồng guard→handler và lifetime cache qua các read/grep ghi trong [source inventory](./reports/codebase-evidence.md). Không ghi tổng số claim giả định. 9 phase có file thật/symbol+citation, fail-before matrix, lệnh và rollback. DDL/constraints/index/soft-delete/import mapping nằm trong schema report; tài liệu external được đọc trực tiếp. Whole-plan sweep: đã đọc lại index, 9 phase và các report; kiểm tra 8 decision deltas xuyên suốt. Không còn mâu thuẫn kiến trúc biết được. Chưa chạy các lệnh kiểm chứng implementation; không có trạng thái test “pass”.

Gate kỹ thuật của **plan**: PASS với các giả định được ghi công khai; gate phỏng vấn: **đã xác nhận 2026-10-28** — ba giả định trên cùng stack pgx/goose/sqlc và 4 quyết định ma trận quyền (schema report §9) đều được anh chốt. Branch chưa xác minh vì cấm git; activation script tìm thấy nhưng lỗi thiếu `ak-prefs-client.cjs`, nên truyền đường dẫn plan trực tiếp cho implementer; task delegation surface không khả dụng, không tạo TODO tracker.

## Concerns / câu hỏi còn mở

- Ba giả định và DB stack đã chốt 2026-10-28 (mục trên). Mở lại bất kỳ quyết định nào (worker location, storage endpoint, bỏ offline mode, đổi stack) cần quyết định riêng và cập nhật lại estimate.
- Credentials/hostname, quota bucket và Tailscale ACL là input vận hành, không có trong repo. Không cần chúng để lập kế hoạch; cần trước rehearsal/deploy thật. Mặc định đề xuất: Postgres 17, một web replica, bucket private, presign 60 giây, lease 120 giây/heartbeat 30 giây, password tạm 24 giờ; tất cả qua env, không đóng cứng trong nghiệp vụ.
- RLS được cân nhắc và không dùng trong cutover này: mọi query bắt buộc scope qua repository, composite FK và test IDOR; pool không giữ actor session-level. Đây là trade-off có chủ đích, không tuyên bố FK là authorization. Đổi sang RLS cần gate riêng để không làm hỏng worker/import/admin paths.
- Khi lập plan (2026-10-28), đây là kế hoạch, không phải implementation: không sửa mã nguồn, không chạy git, không build/test/lint. Plan khi đó nằm ở repo cũ `ainovel-cli`, nay ở `apps/noverse/plans/261028-1340-db-vps-storage/plan.md` trong repo `verse`.
