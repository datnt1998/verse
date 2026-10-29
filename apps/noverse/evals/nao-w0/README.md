# W0 — Đo tìm kiếm trên truyện thật

Đo xem embedding chạy trên máy có hơn tìm theo từ trên văn bản truyện tiếng Việt
thật (8 chương, `output/novel/chapters/`) hay không, để quyết định có làm W4.

- `run.py` chỉ dùng thư viện chuẩn Python 3. Chia chương thành ~150 chunk,
  chạy 7 retriever: `bm25`, `bm25_fold` (thêm từ bỏ dấu), `bm25_graph`
  (RRF với danh sách nhân vật từ `characters.json`), `vec_bge_m3`,
  `vec_qwen3` (qwen3-embedding:0.6b, có prefix instruct) và hai hybrid RRF.
- `questions.jsonl`: 48 câu hỏi tiếng Việt, 16 câu mỗi loại lexical /
  paraphrase / entity, kèm chunk gold.
- Chạy: `ollama serve` (đã `ollama pull bge-m3 qwen3-embedding:0.6b`), rồi
  `python3 run.py`. `--no-ollama` chỉ đo 3 retriever theo từ.
- `--dump-chunks` in danh sách chunk; `--show <id>` xem toàn văn chunk.
- Vector đệm trong `/tmp/nao-w0-cache/`, lần chạy sau miễn phí.
- Kết quả: recall@1, recall@5, MRR@10 chung và theo loại câu hỏi → `results.md`.
