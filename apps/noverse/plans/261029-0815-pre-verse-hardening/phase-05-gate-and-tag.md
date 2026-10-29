---
phase: 5
title: "Gate cuối và tag pre-verse"
status: done (local) — 2026-10-29; còn chờ CI trên GitHub sau khi đẩy `main`
dependencies: [2, 3, 4]
---

# Phase 05 — Gate, smoke, tag

## Các bước

1. Merge `fix/pre-verse-hardening` vào `main` khi `main` đã sạch. Không merge lúc còn thay đổi chưa commit.
2. Chạy gate local:
   - `gofmt -l .`
   - `go vet ./...`
   - `go tool sqlc diff`
   - `go test ./...` có `REQUIRE_POSTGRES_TEST=1` và `REQUIRE_MINIO_TEST=1` (dựng service bằng `deploy/compose.test.yml`)
   - `-race` trên `host`, `store`, `tools`
   - `npm ci`, typecheck và vitest trong `web/`
3. Đẩy `main` lên GitHub (cần anh duyệt), rồi chạy tay workflow `CI`.
4. Chạy smoke architect với `--timeout 60m` ở commit đó. Ghi kết quả vào `plans/reports/`.
5. Gắn tag `pre-verse` lên commit đó và ghi hash vào plan monorepo `verse`.

## Bằng chứng đã có

Lấy từ phase 01, trên worktree sạch (`75c0b2b` cộng các thay đổi của phase 01).

| Gate | Kết quả |
|---|---|
| `actionlint .github/workflows/ci.yml` | Sạch |
| `go tool sqlc diff` | Lệch `ImportItem`/`ImportRun`/`LibrarySetting`. Đúng như dự kiến: phần sửa dở đã sinh lại `gen/` |
| `go test ./internal/revision ./internal/host` có Postgres | Qua. Có một lần flake `TestImportPreviewDeadlineCancelsProviderRequest`, chạy riêng qua 3/3 |
| Bước `Test` của CI (Linux): `go test ./...` có Postgres và MinIO, `REQUIRE_*=1` | Qua, trừ `TestExportSecondRunIsFast`. Test này hết hạn chờ 15 giây của `awaitExport` khi cả suite chạy song song có ffmpeg; chạy riêng qua (4,6 giây), chạy cả package `audio` cũng qua |
| Job `web`: `npm ci`, `typecheck`, `npm test` | Qua: 201 file, 1626 test |

Có hai test hay fail khi máy tải nặng: `TestExportSecondRunIsFast` và `TestImportPreviewDeadlineCancelsProviderRequest`. Nếu CI trên GitHub gặp lại thì sửa nguyên nhân về thời gian của từng test. Không nới assert và không tắt test.

## Kết quả — 2026-10-29

**Merge.** Nhánh được rebase lên `c6006f6` (commit audio của phiên khác) rồi fast-forward vào `main` (`af1c82d`). Bản ghi hash ở `386b0c3`.

**Gate local** chạy trên worktree sạch ở `5543ab8`. Code ở đây giống `af1c82d`, chỉ thiếu commit audio `c6006f6`. Sau khi rebase, em chạy lại riêng `internal/audio`, `store` và `persistence/postgres` có Postgres: qua.

| Gate | Kết quả |
|---|---|
| `gofmt -l .` | 0 file |
| `go vet ./...` | Sạch |
| `go tool sqlc diff` | Sạch |
| `go test ./...` có Postgres 17 + MinIO, `REQUIRE_POSTGRES_TEST=1`, `REQUIRE_MINIO_TEST=1` | Qua, không test nào fail. Hai test hay flake cũng qua |
| `-race` trên `host`, `store`, `tools`, `persistence/postgres`, `storageimport` | Qua |
| Job `web`: `npm ci`, `typecheck`, vitest (ở `c724417`; web không đổi sau đó) | Qua: 201 file, 1626 test |

**Smoke** chạy bằng binary build từ `c724417` (giờ là `7dbafbb`):
- Lệnh: `eval --cases evals/cases/smoke --ci --timeout 60m`.
- Model như cấu hình eval: architect `gpt-6.1-sol`, writer `gpt-6-luna` (openai-codex).
- Run `20261029-101116`, báo cáo thô ở `workspace/evals/pre-verse-smoke-c724417/` (gitignore).

| Case | Kết quả |
|---|---|
| `architect_long` | PASS |
| `architect_short` | PASS |
| `writer_first_chapter` | PASS |

Gate: **PASS**, không hard fail, không warning. Tổng 67 phút, chi phí 1,49 USD, 3 tool error đã tự hồi phục.

Lần này cả hai case architect đều qua. Trong lần A/B trước, cả hai đều chạm guard 30 phút (xem `261027-1443-ai-core-quality-uplift/reports/final-ab.md`).

Code đổi sau commit smoke chỉ gồm `storageimport` (công cụ offline), các helper ánh xạ thuần trong `docstore.go`, và phần lưu export audio (`c6006f6`). Không phần nào nằm trên đường chạy của engine trong eval.

**Tag.** `pre-verse` được gắn local lên commit chứa bản ghi này.

**Còn chờ.**
- Đẩy `main` lên GitHub (cần anh duyệt) và chạy tay workflow `CI`. Đây là lần đầu workflow chạy với Postgres, MinIO và job web.
- Nếu CI fail thì sửa rồi gắn lại tag trước khi chép code sang `verse`.
