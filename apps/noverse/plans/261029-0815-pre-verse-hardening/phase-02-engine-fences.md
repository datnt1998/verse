---
phase: 2
title: "Fence lease phủ mọi lần ghi và lỗi commit không bị nuốt"
status: done — 2026-10-29 (commit 2331818)
dependencies: [1]
---

# Phase 02 — H1, H2, H3

## Yêu cầu

- **H1.** Mọi lần ghi của engine hosted đều phải qua fence của story lease, kể cả `ChapterRecords.Accept` → `Narrative.AcceptRecord`. Hiện chỉ `docStore` có fence (`postgres/backend.go` `WithLeaseFence`).
- **H2.** Fence phải kiểm và ghi trong cùng một transaction, có khoá dòng lease. Hiện `GetStoryLease` không khoá, nên giữa lúc kiểm và lúc ghi thì holder mới có thể chiếm lease mà writer cũ vẫn ghi được.
- **H3.** `WriteJSON` và các lệnh ghi document phải trả về lỗi commit cho chính lời gọi đó. Hiện commit chạy trong `Unlock → EndWrite` sau khi kết quả đã được trả về (`postgres/docstore.go`, `store/io.go`).

## Hướng sửa

1. Đưa bước kiểm fence vào bên trong transaction ghi. Đọc dòng lease bằng `FOR SHARE` (khoá chia sẻ) và so token; lệnh chiếm hoặc gia hạn lease dùng `UPDATE`, nên sẽ phải chờ transaction ghi đang giữ khoá.
   - Mỗi đường ghi (document, chapter record, narrative) đều dùng chung một hàm kiểm.
   - Không thêm fence riêng cho từng substore.
2. Khi dựng backend, truyền fence vào `pgChapterRecords` và vào các writer typed khác của `Narrative`.
3. Đổi ranh giới hoàn tất transaction của document thành hàm trả về `error`. `IO.WriteJSON`/`WriteMarkdown` trả lỗi commit. Không giữ lại `finErr` để báo ở thao tác sau.

## Test hồi quy (phải fail trước khi sửa)

- Lease bị đóng hoặc bị chiếm, sau đó `ChapterRecords.Accept`: phải trả lỗi fence và không tạo thêm version.
- Hai transaction xen kẽ: writer cũ kiểm fence, holder mới chiếm lease, writer cũ commit. Kết quả cuối không được là dữ liệu của writer cũ.
- Commit thất bại (ví dụ huỷ context hoặc đóng kết nối trước khi commit): `WriteJSON` phải trả lỗi.

## Kiểm tra

```bash
REQUIRE_POSTGRES_TEST=1 TEST_DATABASE_URL=… go test -count=1 ./internal/persistence/postgres ./internal/store ./internal/host ./internal/tools
go tool sqlc diff
```

## Rủi ro

`FOR SHARE` làm việc chiếm lease phải chờ transaction ghi đang chạy. Transaction ghi vốn ngắn nên đây là hành vi đúng. Không được giữ khoá trong lúc gọi model.

## Kết quả

**Sửa:**
- `Narrative.WithFence` trả về một bản sao. Mọi transaction của bản sao này đều chạy fence trước. `WithLeaseFence` gắn bản sao đó vào `pgChapterRecords`, `pgWorldStore` (review) và vào doc store. Lease vẫn dùng `Narrative` gốc, không có fence, để tự gia hạn.
- Query mới `GetStoryLeaseForWrite ... FOR SHARE` khoá dòng lease cho tới khi transaction ghi commit. Đã thử trên Postgres 17: một `FOR SHARE` thứ hai không phải xếp hàng sau `UPDATE` đang chờ, nên các vùng ghi lồng nhau (outline + progress) không bị deadlock.
- **Lỗ hổng phát hiện thêm khi sửa:**
  - Vòng gia hạn gọi lại `ClaimStoryLease`, câu này cho phép cùng holder giành lại một lease đã mất khi dòng trống, khiến writer cũ sống lại.
  - Đã tách thành `RenewStoryLease`: chỉ gia hạn khi đúng holder, đúng fencing token và lease còn hạn.
  - `ClaimStoryLease` bỏ điều kiện cùng holder.
  - Đã xoá `Narrative.ClaimLease` và kiểu `Lease` vì không còn ai gọi.
- `IOOps.EndWrite()` giờ trả về `error`. Mọi vùng ghi độc quyền trong `internal/store` (33 chỗ) đều đi qua `IO.lockWrite`, nên lỗi commit được trả cho chính lời gọi đó. `ioMutex` không còn `Lock`/`Unlock`, nên compiler chặn mọi vùng ghi chưa chuyển. Đã bỏ `finErr`. `txErr` giờ chỉ sống trong một vùng ghi, trước đây một lần `Begin` lỗi làm hỏng cả substore.

**Test hồi quy** (`internal/persistence/postgres/story_lease_test.go`). Cả bốn fail trước khi sửa và qua sau khi sửa:

| Test | Lỗi trước khi sửa |
|---|---|
| `TestStoryLeaseFencesTypedWrites` | `accept after takeover = <nil>, want ErrLeaseLost` |
| `TestStoryLeaseTakeoverWaitsForFencedWrite` | `takeover finished while a fenced write was still open` |
| `TestStoryLeaseRenewalNeverReclaims` | `renewal took a lost lease back instead of reporting it lost` |
| `TestDocumentWriteReportsCommitFailure` | `write reported success although its commit was rejected` |

**Phát hiện thêm khi chạy gate:** data race có sẵn từ trước ở `pool.go` `scanTimestampsInUTC`. Hàm này sửa thẳng kiểu `timestamptz` lấy từ map mặc định dùng chung của pgx. Race tái hiện được trên `main` với `-race` (`TestJobsEventsAppendOrdered`). Đã sửa bằng cách đăng ký kiểu riêng cho từng connection (commit `7dbafbb`). Sau khi sửa, `go test -race ./internal/persistence/postgres` qua.
