---
phase: 4
title: "Import và xuất ngược giữ đủ dữ liệu"
status: done — 2026-10-29 (commit f67a2fc)
dependencies: [1]
---

# Phase 04 — H5, H6, H7, H8

Plan `261028-1340-db-vps-storage` ghi phase 08 là xong. Review cho thấy chưa đạt, nên phase này mở lại các yêu cầu của phase 08 đó. Chưa xong phase này thì cấm import dữ liệu live và cấm cutover.

## Yêu cầu

- **H5.** Import giữ nguyên `facts`, `style_delta`, `content_sha256`, `accepted_at` của bản ghi chương. Reconcile phải so cả các trường này, không chỉ so số lượng và hash thô.
- **H6.** Import ghi dữ liệu dưới đúng khoá mà Store production đọc:
  - plan: `chapter_plan` + `NN`
  - chương: artifact `chapter_text`
  - nhân vật: `story_state(characters)`
  
  Dùng chung bảng ánh xạ của `docstore.resolveDoc`, không viết một ánh xạ thứ hai.
- **H7.** Xuất ngược ghi ra `domain.ChapterRecord` đầy đủ, file backend phải mở được (không còn lỗi `content digest mismatch`).
- **H8.** Xuất ngược phải đủ để dùng làm rollback:
  - tài khoản, quyền
  - checkpoint, sự kiện, lịch sử version
  - dự án và blob tạo sau cutover (liệt kê từ bảng hiện tại, không dựa vào sổ import)
  
  Thiếu blob thì phải fail, không được báo thành công.

## Test hồi quy

Các test đều chạy trên fixture và Postgres thật.

- Import → đọc lại bằng **Store production** (`NewFromBackend`): plan, chương, nhân vật và bản ghi chương phải khớp nguồn từng trường.
- Import một fixture đã bị sửa mất `facts` sau khi import: reconcile phải báo lệch.
- Xuất ngược → mở bằng **file Store** với thư mục tài khoản rỗng (không dùng lại `fixtureAccounts`): tài khoản, checkpoint và bản ghi chương đều đọc được.
- Dự án tạo qua `LibraryRepository.CreateProject` sau khi import phải có trong bản xuất. Xoá một blob khỏi BlobStore thì export phải fail.

`TestExportRoundTrip` hiện dùng lại tài khoản gốc nên che mất lỗi. Sửa test này theo cách trên, không giữ assert cũ.

## Kiểm tra

```bash
REQUIRE_POSTGRES_TEST=1 REQUIRE_MINIO_TEST=1 … go test -count=1 ./internal/storageimport ./internal/persistence/postgres
go run ./cmd/ainovel-cli storage import --dry-run …   # trên bản sao fixture, không trên dữ liệu live
```

## Ngoài phạm vi

H9 (không kiểm byte file sự kiện khi import) và M3–M7 thuộc cùng khu vực. Sửa luôn nếu đụng đúng hàm, nếu không thì ghi lại ở báo cáo.

## Kết quả

H5–H9, M3 và M4 đã sửa. Chi tiết từng lỗi (nguyên nhân, cách sửa, test, dòng lỗi trước khi sửa) và các giới hạn còn lại nằm trong [import-export-fidelity-261029.md](../reports/import-export-fidelity-261029.md).

**Review độc lập 2 vòng:**
- Vòng 1 tìm thêm 5 lỗi trong bản sửa đầu: xuất ngược làm mất bản nháp, import tiếp tục ghi đè bản nháp, mất `signals`/`comment_events`, sống lại chương đã xoá, digest prose cũ lệch.
- Vòng 2 tìm thêm 2 chỗ chặn quá tay: file không phần mở rộng và review global không gắn chương.

Cả 7 lỗi đều có test fail trước, qua sau.

**Còn mở:**
- **M5:** chuyển role tài khoản v1/v2.
- **Không xuất ngược:** phiên đăng nhập, ghép thiết bị, transcript agent, job.
- **Định dạng lưu:**
  - Lịch sử version xuất ra theo layout v3, mà file backend hiện tại chưa đọc.
  - File cạnh `story.json` được xuất về dưới `output/novel/`.
  - Thư mục tập của script audio bị gộp về `tap-00`.

Không mục nào làm mất bản ghi đã chấp nhận.

**Kiểm tra:**
- Các package `storageimport`, `persistence/...` và `store` qua với `REQUIRE_POSTGRES_TEST=1` và `REQUIRE_MINIO_TEST=1`.
- `go vet` sạch, `sqlc diff` sạch.
- Chạy thử CLI `storage import --dry-run` trên bản sao fixture: sạch.
