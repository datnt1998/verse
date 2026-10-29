# W0 — Kết quả đo tìm kiếm (25/10/2026)

Truyện `01-tra-xanh-cuop-kich-ban`, 8 chương, 150 chunk (~500 ký tự), 48 câu hỏi
(16 lexical / 16 paraphrase / 16 entity). Embedding qua Ollama trên máy (arm64, 16 GB).

## Tổng quan (n=48 câu, 150 chunk)

| retriever | R@1 | R@5 | MRR@10 |
|---|---|---|---|
| bm25 | 0.604 | 0.729 | 0.655 |
| bm25_fold | 0.562 | 0.708 | 0.635 |
| bm25_graph | 0.479 | 0.583 | 0.510 |
| vec_bge_m3 | 0.500 | 0.729 | 0.604 |
| vec_qwen3 | 0.458 | 0.750 | 0.580 |
| hybrid_bge | 0.458 | 0.583 | 0.540 |
| hybrid_qwen3 | 0.500 | 0.625 | 0.564 |

## R@5 theo loại câu hỏi

| retriever | lexical | paraphrase | entity |
|---|---|---|---|
| bm25 | 0.938 | 0.562 | 0.688 |
| bm25_fold | 0.938 | 0.500 | 0.688 |
| bm25_graph | 0.625 | 0.562 | 0.562 |
| vec_bge_m3 | 0.938 | 0.625 | 0.625 |
| vec_qwen3 | 0.938 | 0.562 | 0.750 |
| hybrid_bge | 0.625 | 0.625 | 0.500 |
| hybrid_qwen3 | 0.625 | 0.625 | 0.625 |

## Thời gian & kích thước

- Số chunk: 150 — Số câu hỏi: 48
- Embedding 150 chunk bằng bge-m3: 7.7 s — kích thước trên đĩa: 1.2 GB
- Embedding 150 chunk bằng qwen3-embedding:0.6b: 13.1 s — kích thước trên đĩa: 639 MB

## Ví dụ câu paraphrase phân kỳ

- "Cô từ chối tạo ra tấm ảnh thân thiện trên thảm đỏ để đối phương không biến nó
  thành thông báo gì?" — bm25_graph trượt, bge-m3 trúng.
- "Người cố vấn khuyên cô không được suy đoán điều gì về trang tin bôi nhọ và số
  điện thoại lạ?" — cả hai bm25 trượt, cả hai vector trúng.
- "Trước khi bấm đồng ý đi chương trình, cô chỉ hỏi một câu duy nhất về điều
  gì?" — bm25 trúng, cả hai vector đều trượt.

## Kết luận (chốt 25/10)

Mốc so sánh đúng là `bm25` thuần, vì đây là cấu hình mạnh nhất không cần model. Đo theo mốc này:

- **Vector không hơn rõ.**
  - `vec_bge_m3` hoà `bm25` (R@5 72,9%).
  - `vec_qwen3` hơn 2,1 điểm. Với 48 câu, mức đó là **một câu**.
  - Ở nhóm paraphrase, vector hơn tối đa một câu trên 16 (62,5% so với 56,2%).
  - R@1 và MRR của mọi vector đều **thấp hơn** `bm25`.
- **Gộp thực thể bằng RRF ở mức đoạn làm kết quả kém đi:** lexical giảm từ 93,8% xuống 62,5%, entity từ 68,8% xuống 56,2%. Đồ thị nên dùng để trả dữ kiện có cấu trúc (xưng hô, quan hệ) hoặc để lọc, không trộn thứ hạng với từng đoạn văn.
- **Bỏ dấu làm kết quả kém đi** (paraphrase giảm từ 56,2% xuống 50,0%). Giữ dấu là mặc định. Chỉ thử dạng bỏ dấu khi truy vấn có dấu không ra kết quả nào.
- **Giới hạn của lần đo:** chỉ một truyện, 150 đoạn. Tìm kiếm cấp dự án (khoảng 300 truyện mỗi năm) chưa đo được.

**Quyết định:**

- Không làm W4 lúc này.
- Đo lại bằng chính `run.py` khi dự án có từ 10 truyện trở lên, thêm câu hỏi xuyên truyện.
- Nếu khi đó vector hơn `bm25` từ 5 điểm R@5 trở lên thì làm W4 với `qwen3-embedding:0.6b` (639 MB, nhỏ hơn bge-m3 1,2 GB).
