---
phase: 1
title: "Việc không đụng phần đang sửa dở"
status: done — 2026-10-29
dependencies: []
---

# Phase 01 — H4 và gate CI

## H4: revision đọc chương qua Store

**Nguyên nhân.** Trên backend Postgres, `Dir()` rỗng. Các đoạn code dưới đây vẫn dựng đường dẫn từ `Dir()` nên đọc vào thư mục làm việc của tiến trình:

- `revision.Scan`
- nhánh `resume` của `revisionTexts`
- việc gắn lại góp ý sau khi nhận bản sửa (`comments.AfterWrite(Dir())`)
- `loadLegacyCommits` và `loadLegacyBook`

Truyện hosted vì thế bị chặn ở `requireCleanChapters` ngay sau chương đầu tiên.

**Sửa.**

| File | Thay đổi |
|---|---|
| `internal/revision/scan.go` | Đọc chương qua `st.Drafts.LoadChapterText` |
| `internal/host/web_revision.go` | Đọc chương qua store; chỉ `stat` mtime khi `FileRoot()` là file |
| `internal/revision/service.go` | Gắn lại góp ý qua `st.Comments().AfterWriteAt` |
| `internal/revision/migration.go`, `internal/host/resume.go` | Chỉ đọc lịch sử file cũ khi store là file |

**Bằng chứng.** Test `TestRevisionCheckPostgres` (`internal/host/storage_pg_test.go`):

- Trước khi sửa: fail với `open chapters/01.md: no such file or directory`.
- Sau khi sửa: qua. Test cũng kiểm tra được bản sửa lưu trong store, và thư mục làm việc vẫn trống.

Commit `8300810`.

**Ngoài phạm vi, ghi lại để quyết sau.** Luồng nhập truyện `ImportFrom` (`imp.OpenWorkspace(st.Dir())`, `imp/publish.go`) vẫn dùng `Dir()`. Worker hosted chạy với thư mục làm việc là một scratch riêng (`gateway.go` `launchHostedWorker`), và scratch này bị xoá khi worker thoát. Vì vậy (suy luận, chưa chạy thử) workspace nhập sẽ nằm trong scratch và mất sau mỗi lần chạy, nên nhập dở thì không tiếp tục được. Cần chặn `ImportFrom` trên hosted hoặc chuyển workspace nhập sang store.

## Gate CI

Sửa `.github/workflows/ci.yml`:

- Bước `go tool sqlc diff` (Linux).
- Bước dựng Postgres và MinIO bằng `deploy/compose.test.yml`. Đây cũng là file dùng khi chạy local.
- Bước `go test` trên Linux có `REQUIRE_POSTGRES_TEST=1` và `REQUIRE_MINIO_TEST=1`, nên thiếu service thì fail chứ không skip.
- Job `web` mới: `npm ci`, typecheck, vitest, chạy Node 22 cho khớp `Dockerfile`.

**Kiểm tra.**

- `actionlint` sạch.
- Các bước Go và web đã chạy lại y hệt trên worktree sạch, kết quả ghi ở phase 05.
- `sqlc diff` ở `75c0b2b` báo lệch `ImportItem`. Gate này sẽ fail cho tới khi phần sửa dở (đã sinh lại `gen/`) được commit. Đây là hành vi mong muốn.
