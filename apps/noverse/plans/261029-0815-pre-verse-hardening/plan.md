---
title: "Sửa lỗi Postgres hosted trước khi tách repo verse"
description: "Sửa các lỗi High về tính đúng của engine, auth và import/xuất ngược; thêm gate CI cho Postgres, MinIO và web; gắn tag pre-verse."
status: done-local (chờ CI trên GitHub)
priority: P1
branch: "fix/pre-verse-hardening, đã fast-forward vào main (af1c82d)"
tags: [database, backend, auth, ci, hardening]
blockedBy: []
blocks: [verse-monorepo]
created: 2026-10-29
---

# Kết quả cần đạt

Repo `noverse` đạt trạng thái sạch và đáng tin để gắn tag `pre-verse`, rồi mới chuyển code sang monorepo `verse`. Trạng thái đó gồm:

- Không còn lỗi High trong danh sách đã chốt.
- CI chạy thật các suite Postgres, MinIO và web.
- Trên tree không còn thay đổi chưa commit.

Danh sách lỗi và bằng chứng nằm ở [báo cáo review](../reports/review-261029-storage-update-verse-readiness.md). Plan này không lặp lại chi tiết từ đó.

## Quyết định đã chốt — 2026-10-29

1. **Sửa trong repo hiện tại rồi mới tách repo** (anh chọn). Việc dời code sang `verse` không trộn với việc sửa lỗi.
2. **Phạm vi:** H1–H4, H14, H5–H8 và gate CI. H9–H13 (audio hosted, kiểm byte file sự kiện khi import) và các lỗi Medium/Low vẫn mở. Chúng không chặn việc dời code, nhưng phải sửa trước khi chuyển live sang Postgres.
3. **Không chạy `storage import` trên dữ liệu live và không cutover live sang Postgres** cho tới khi xong phase 04.
4. **Phiên khác từng sửa dở** 82 file trên `main`. Phase 01 chỉ đụng file nằm ngoài phần đó. Phần sửa dở đã được commit (`f10299f`, `574d987`); nhánh đã rebase lên `574d987`.

## Các giai đoạn

| Giai đoạn | Kết quả | Chặn bởi | Trạng thái |
|---|---|---|---|
| 01 [Việc không đụng WIP](./phase-01-outside-wip.md) | H4 đã sửa; CI chạy Postgres/MinIO, `sqlc diff` và web | Không | xong |
| 02 [Fence lease và lỗi commit](./phase-02-engine-fences.md) | H1–H3, kèm lỗ hổng gia hạn lease | 01 | xong |
| 03 [Giới hạn đăng nhập](./phase-03-login-limiter.md) | H14, kèm backoff của limiter Postgres | 01 | xong |
| 04 [Import và xuất ngược đủ dữ liệu](./phase-04-import-fidelity.md) | H5–H9, M3, M4 | 01 | xong |
| 05 [Gate cuối và tag](./phase-05-gate-and-tag.md) | Gate đầy đủ qua, smoke architect, tag `pre-verse` | 02–04 | xong local; chờ CI GitHub |

Các phase 02–04 chạm các file khác nhau nên làm song song được sau khi rebase. Ngoại lệ: `queries/*.sql` và `gen/` cần một người giữ, sinh lại code bằng `go tool sqlc generate` một lần cuối.

## Điều kiện hoàn thành

- Mỗi lỗi High trong phạm vi có một test hồi quy chạy với `REQUIRE_POSTGRES_TEST=1`. Test phải fail trước khi sửa và qua sau khi sửa.
- Workflow `CI` qua trên GitHub. Lần đầu phải chạy tay (`workflow_dispatch`), vì workflow không chạy khi push.
- `git status` sạch trên `main`. Tag `pre-verse` trỏ tới commit đã qua gate và có ghi lại kết quả smoke.

## Rủi ro

- **Rebase xung đột** ở `docstore.go`, `narrative.go`, `queries/narrative.sql`. Cách giảm: phase 01 không sửa các file này. Phần `gen/` thì sinh lại, không sửa tay khi resolve xung đột.
- **Phiên kia có thể sửa trùng một lỗi.** Sau khi rebase, kiểm lại từng lỗi bằng test hồi quy rồi mới sửa. Lỗi nào đã được sửa thì chỉ giữ lại test.
- **Test chạy chậm làm lộ flake.** `TestImportPreviewDeadlineCancelsProviderRequest` từng fail một lần khi chạy cả package `host` cùng Postgres, rồi qua 3/3 khi chạy riêng. Nếu lặp lại thì ghi nhận và sửa riêng, không nới assert.
