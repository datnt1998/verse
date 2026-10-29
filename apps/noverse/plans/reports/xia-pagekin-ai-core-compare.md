# Feature Comparison: AI core chất lượng viết truyện — Pagekin vs ainovel-cli

Mode: `--compare` (chỉ nghiên cứu + khuyến nghị, chưa lập plan triển khai).
Ngày: 2026-10-27.

## Source manifest

| Mục | Giá trị |
| --- | --- |
| Repo | https://github.com/EthanYoQ/AI-Novel-Writer (tên mới: Pagekin / 书童) |
| Ref | `master` @ `b049b516672ef642af7756f22eee2a1274ed35a2` (2026-10-27) |
| Stack | TypeScript, Electron + React, SQLite (better-sqlite3), LanceDB (vector, tùy chọn) |
| License | **GPL-3.0**. Repo mình dùng **Apache-2.0**, nên **chỉ học ý tưởng/cơ chế**. Không chép code, không chép nguyên văn prompt; prompt phải tự viết bằng tiếng Việt. |
| Phạm vi đã đọc | prompts, workflows/commands, shared contracts, electron/services, electron/llm, ADR, docs/experiments, docs/research/quality-protocol |
| Bỏ qua | UI, release, packaging, plugin DSH |

Phụ lục, gồm bằng chứng `file:line` cho từng nhận định:

- [00 — bản đồ AI core local](xia-pagekin/00-local-ai-core-map.md)
- [01 — pipeline viết chương + prompt](xia-pagekin/01-prompt-pipeline.md)
- [02 — memory, continuity, retrieval](xia-pagekin/02-memory-continuity.md)
- [03 — review, revision, structured repair, đo chất lượng](xia-pagekin/03-review-revision.md)
- [04 — planning, cast, blueprint, import, style imitation](xia-pagekin/04-planning-import.md)
- [05 — LLM layer, runtime, agent](xia-pagekin/05-llm-runtime.md)

## Kết luận chính

1. Pagekin **không hơn hẳn** mình về kiến trúc. Mình đã mạnh hơn ở các điểm sau:
   - Router tự động theo arc/volume, có arbiter.
   - Sửa chương bằng patch `edit_chapter` thay vì viết lại cả chương.
   - Import đọc toàn văn từng chương, có continuity ledger. Pagekin chỉ lấy mẫu đầu/cuối và cắt mỗi chương ở 6.000 ký tự.
   - Hệ anti-AI-tone gồm voice, fatigue words, rule checker và style stats.
   - Retry/backoff.
2. Giá trị thật của Pagekin nằm ở **kỷ luật về "thẩm quyền của sự thật" (authority) và bằng chứng (evidence)**:
   - Văn bản đã chốt là nguồn chính. Tóm tắt và state chỉ là bản phái sinh, dùng để định vị.
   - Kế hoạch tương lai chỉ là ràng buộc thời điểm. Nó không phải nhiệm vụ của chương này và chưa phải sự kiện đã xảy ra.
   - Reviewer phải chấm theo checklist do phần mềm đóng băng, có 3 trạng thái `completed/unmet/unknown` và trích dẫn nguyên văn mà code định vị được.
   - "Văn bản đã đổi" không đồng nghĩa "lỗi đã sửa". "JSON hợp lệ" hay "trích dẫn có thật" cũng không đồng nghĩa "suy luận đúng".
3. Bài học về phương pháp quan trọng nhất là cách họ **đo reviewer**:
   - Case set đóng băng trước, gold tách khỏi runner.
   - Có positive control: nhân vật nói dối, chuyển giao gián tiếp, sắp xếp tương lai.
   - Có case thiếu bằng chứng, nơi đáp án đúng là `unknown`.
   - Báo cáo tách riêng tính hợp lệ cơ học, tỉ lệ bắt lỗi then chốt, false positive và chi phí.
   - `evals/` của mình hiện **chưa assert được verdict** (`evals/cases/editor-goi-a/dialogue-warning.json`). Đây là điểm nghẽn: chưa đo được thì mọi cải tiến dưới đây chỉ là phỏng đoán.

## Head-to-head

| Khía cạnh | Pagekin | ainovel-cli | Khuyến nghị |
| --- | --- | --- | --- |
| Điều phối | Workflow tuyến tính do người dùng bấm; batch tối đa 10 chương; auto-next chỉ điền sẵn form | Router thuần `flow.Route` + arbiter LLM + advance gate (`internal/flow/router.go`, `internal/arbiter`) | **Giữ của mình** |
| Lập kế hoạch chương | Short outline ≤6 bullet, tách "tiền chương đã hoàn thành gì" khỏi "chương này làm gì", không được bịa mục tiêu (`src/shared/draft-short-outline.ts:3-17`) | `plan_chapter` bắt buộc, có contract (`internal/domain/writing.go:5-31`) | **Bổ sung** trường/hướng dẫn "đã hoàn thành ở chương trước" vào `plan_chapter`, không thêm lượt LLM |
| Phân cấp thẩm quyền trong prompt | Rõ ràng: chính văn đã chốt > câu chữ blueprint; ý định cũ ≠ lệnh cấm; hành động bắt buộc ≠ ràng buộc duy trì (`generate-draft.command.ts:396-429`) | Có ưu tiên user > mặc định (`assets/prompts/writer.md:32-49`), chưa có phân cấp sự thật | **Học**: viết lại bằng tiếng Việt |
| Kế hoạch tương lai | 5 blueprint kế tiếp được inject như *ranh giới thời điểm*, cấm thực hiện sớm (`chapter-materials.ts:517-524`) | Có outline window (`novel_context_builders.go:510-534`), không có ngữ nghĩa "không thực hiện sớm" | **Học** |
| Chương trước | **Toàn văn** chương trước là bắt buộc (`chapter-materials.ts:460-470,569-572`) | Chỉ **800 rune cuối** (`internal/tools/novel_context_builders.go:497-503`) | **Thử nghiệm A/B**: chi phí token cao, cần đo |
| Truy hồi lịch sử | Fact chỉ là locator; đọc lại đoạn văn gốc kèm 1 đoạn lân cận mỗi bên, gộp chỗ chồng lấn (`chapter-materials.ts:345-423`) | Đưa thẳng summary, state, timeline (`novel_context_builders.go:446-594`) | **Học**: thêm "đoạn văn bằng chứng" vào recall |
| Thời gian | Hợp đồng chung: mặc định cùng ngày, quy đổi "hoàng hôn → tối nay / hôm qua" (`src/shared/chapter-time-continuity.ts`) | Có time text và yêu cầu nhất quán chung chung (`assets/prompts/editor.md:36-40`) | **Học** (effort S) |
| Trích xuất fact khi commit | Phân biệt đã xảy ra / dự định / nghi ngờ / phủ nhận / hủy; giữ bản đính chính mới nhất; cấm suy nhân quả từ việc cùng xuất hiện (`builtin-prompt-templates.ts:1062-1111`) | "Chỉ fact từ chính văn" (`assets/prompts/writer.md:64`) | **Học** (effort S, impact H) |
| Bằng chứng cho fact/state | Mỗi fact có excerpt, code kiểm tra excerpt nằm trong văn bản bất biến (`summary-repository.ts:569-605`) | Fact/state không có span (`internal/domain/revision.go:21-47`, `tracking.go:3-10`) | **Học** (M) |
| Provenance của state nhân vật | Từng field gắn author / derived / legacy, compare-and-swap (`character-roster.ts:44-69`) | State delta và snapshot, không có provenance | Học có chọn lọc |
| Định danh nhân vật | Mơ hồ thì giữ "unresolved", không fuzzy-merge (`finalized-continuity.ts:209-222`) | Có ID ổn định + alias (`internal/store/nao_identity.go`) | Bổ sung trạng thái unresolved |
| Tuyến truyện / phục bút | Plan (ý đồ tác giả, cửa sổ chương mục tiêu) tách khỏi Event (planted/progressing/resolved/**abandoned** + evidence); tính dormant/overdue | plant/advance/resolve + nhắc lại sau 30 chương (`internal/tools/novel_context.go:832-906`) | **Mở rộng** ledger: target window, overdue, abandoned |
| Review | 6 vùng *khách quan*, loại trừ gu văn; evidence trước severity; 1–10 item; kênh `goalReviews` riêng (`builtin-prompt-templates.ts:924-979`) | 7 chiều 0–100 trộn khách quan và thẩm mỹ (`assets/prompts/editor.md:32-75`) | **Giữ 7 chiều**, nhưng tách *lỗi chặn khách quan* khỏi *gợi ý tay nghề* |
| Checklist mục tiêu chương | Phần mềm đóng băng ID và nguyên văn; model chỉ trả status theo ID; thiếu/trùng/sai thì hạ về `unknown`; quote phải định vị được (`src/shared/chapter-goal-review.ts`) | `ContractStatus met/partial/missed` gộp chung (`internal/domain/review.go:118-120`) | **Học**: ưu tiên cao |
| Vòng sửa lỗi | Finding có vòng đời, gắn source hash; "văn bản đổi ≠ đã sửa"; tối đa 1 recheck nhắm đích (`src/shared/review-cycle.ts`) | Có PendingRewrites; chỉ chặn trường hợp rewrite không đổi gì (`commit_chapter.go:501-521`) | **Học** (M) |
| Cách sửa | Viết lại cả chương, merge theo đoạn | Patch `edit_chapter` | **Giữ của mình** |
| Gate toàn vẹn văn bản | Chặn meta-commentary, code fence, mảnh thẻ `<think>`, dấu hiệu bị cắt, đoạn dài trùng lặp mới, rewrite ngắn bất thường (<60%) (`visible-text-integrity.ts`, `refinement-completeness.ts`) | Chỉ check rỗng / không đổi | **Học** (S–M), tự viết regex cho tiếng Việt |
| Viết tiếp khi bị cắt | Bỏ phần chồng lấn ≥48 ký tự với tail 1.600; đo tiến độ bằng văn bản *mới thật*; tối đa 1 lần no-progress (`visible-continuation.ts`, `generate-draft.command.ts:1540-1580`) | `draft_chapter append`, không xử lý chồng lấn | **Học** |
| Lặp lại đoạn kết chương trước | So n-gram đầu chương mới với cuối chương trước, chặn nếu trùng liên tục ≥80 ký tự (`generate-draft.command.ts:275-315`) | Chỉ hướng dẫn chống recap trong voice | **Học** |
| Structured output | Length → chia batch nhỏ hoặc compact rebuild; repair chỉ sửa cú pháp, giữ nguyên scalar; số lần sửa có giới hạn | `llmcontract.Execute` **sửa vô hạn** tới khi ctx hết; length thì fail ngay (`internal/llmcontract/execute.go:107-129`) | **Học**: đặt giới hạn số lần sửa (S), chia batch (M) |
| Reasoning | Strategy cấp dự án → effort theo stage (draft thấp, plan vừa, review cao); ghi requested và effective | Override theo role; đã clamp effective | Thêm preset (S) |
| Thinking lẫn trong content | Bộ lọc `<think>` theo stream, giữ prefix của thẻ bị cắt giữa các chunk (`electron/llm/visible-stream.ts`) | Không tìm thấy bộ lọc tương đương (`<think>` không có trong `internal/`) | **Học** (M); rủi ro thật với Ollama/Qwen/DeepSeek-distill |
| Ứng viên khôi phục | Stream bị lỗi thì phần text nhìn thấy được lưu làm candidate, không tự thành draft | Bỏ partial stream | Tùy chọn |
| Cast | Manifest định danh (ID/tên/vai/quan hệ) được đóng băng trước, sinh chi tiết theo batch sau (`prompt-language.ts:98-128`) | Lưu thẳng mảng nhân vật (`save_foundation.go:274-289`) | **Học** (M) |
| Planning outline | Mỗi request 1 chương; giữ prefix đã validate; 1 lần compact rebuild | Architect nộp theo arc | Chỉ khi đo thấy lỗi bị cắt |
| Mật độ sự kiện | Dedupe sự kiện lặp, không bịa sự kiện độn chữ, không bỏ sự kiện bắt buộc, gặp xung đột thì gắn cờ (`directory.command.ts:141-150`) | Một phần (`architect-long.md:92-97`) | **Học** (S) |
| Style imitation | 3–6 gợi ý tùy chọn; cấm biến tần suất trong mẫu thành quota; style không được tạo sự kiện | Profile tối đa 12 item mỗi nhóm (`internal/domain/simulation.go:9-12`) | **Học**: thêm view "drafting" gọn |
| Retrieval | LanceDB vector + fallback SQL `LIKE` (**không phải** BM25/hybrid như comment ghi) | Chưa có; W4 chờ eval `nao-w0` | **Giữ cổng eval**, đừng chạy theo |
| Đo chất lượng | Calibration đóng băng, dev/held-out, positive control, case unknown, gold tách riêng | Eval harness có, nhưng editor eval chưa assert verdict | **Học trước tiên** |

## Bảng ưu tiên

Impact là ước lượng kỹ thuật ([INFERENCE]), chưa đo.

### P0 — Đo trước khi sửa

| # | Việc | Impact | Effort | Ghi chú |
| --- | --- | --- | --- | --- |
| 0.1 | Editor/reviewer eval assert được verdict và issue; thêm case positive control (nhân vật nói dối, chuyển giao gián tiếp, kế hoạch tương lai, flashback) và case thiếu lịch sử (đáp án đúng là `unknown`) | H | M | Gold tách khỏi runner. Báo cáo riêng: hợp lệ cơ học, hit lỗi then chốt, FP, unknown, token. Tham khảo `docs/experiments/continuity-calibration-round01*.md` |
| 0.2 | Bộ case cho writer: mở đầu lặp lại chương trước, sai thời gian tương đối, thực hiện sớm kế hoạch tương lai, biến "dự định đi" thành "đã đến" | H | M | Dùng làm A/B cho P1 |

### P1 — Prompt/rule, rẻ, kỳ vọng cao (effort S)

| # | Việc | Nơi sửa |
| --- | --- | --- |
| 1.1 | Phân cấp thẩm quyền cho writer: chính văn đã chốt > câu chữ outline/contract; ý định cũ của nhân vật không phải lệnh cấm; ràng buộc duy trì không đòi phải dựng cảnh xác nhận | `assets/prompts/writer.md`, `assets/voice.md` |
| 1.2 | Outline window = ranh giới thời điểm: không thực hiện sớm, không tiết lộ sớm hiểu biết/chuyển giao/kết quả của chương sau; foreshadow vẫn được phép | `novel_context_builders.go` (nhãn), `writer.md` |
| 1.3 | Hợp đồng thời gian: mặc định nối tiếp trong cùng ngày; quy đổi thời điểm tương đối theo chính văn; editor quy hành động không ghi giờ về mốc thời gian rõ ràng gần nhất phía trước | `writer.md`, `editor.md` |
| 1.4 | Trích xuất fact khi commit: phân biệt đã xảy ra / dự định / nghi ngờ / phủ nhận / hủy; giữ bản đính chính mới nhất; ghi rõ ai biết, ai sở hữu; cấm suy nhân quả từ việc cùng xuất hiện | `writer.md` (phần commit), mô tả schema `commit_chapter` |
| 1.5 | Editor: tách `lỗi chặn khách quan` khỏi `gợi ý tay nghề`; bắt viết evidence trước khi chọn severity; được phép bỏ trống chiều không có vấn đề | `assets/prompts/editor.md` |
| 1.6 | Style chỉ chi phối cách diễn đạt; không được thêm sự kiện hay fact, không biến tần suất trong mẫu thành quota. Thứ tự: fact tác giả > nhân quả chương > độ dài > style | `assets/load.go` (khối chung), `simulation-*.md` |
| 1.7 | Planner: dedupe sự kiện được mô tả nhiều lần; không độn sự kiện cho đủ chữ; không bỏ sự kiện bắt buộc; quá tải thì gắn cờ | `architect-long.md`, `architect-short.md` |
| 1.8 | `plan_chapter`: thêm phần "chương trước đã hoàn thành gì / trạng thái kết thúc" để chương mới không kể lại kết quả cũ | `internal/domain/writing.go`, `writer.md` |
| 1.9 | Giới hạn số lần sửa trong `llmcontract.Execute`; hiện vòng lặp chỉ dừng khi ctx kết thúc | `internal/llmcontract/execute.go:107` |

### P2 — Code, impact cao (effort M)

| # | Việc | Ghi chú port sang Go |
| --- | --- | --- |
| 2.1 | **Goal checklist đóng băng**: tách `required_beats`/contract thành item có ID do code sinh; editor trả `{id, status: completed/unmet/unknown, evidence[]}`; code kiểm tra phủ đủ ID, không trùng, không lạ, quote định vị được duy nhất trong chính văn; thiếu thì hạ về `unknown` | Mở rộng `ReviewEntry` và `save_review`, không tạo DB mới. Dùng byte offset UTF-8. `unknown` không tính là pass |
| 2.2 | **Gate toàn vẹn văn bản** dùng chung cho draft, append, rewrite, commit: chặn meta-commentary tiếng Việt, code fence, thẻ thinking, dấu bị cắt, đoạn dài mới trùng lặp; rewrite toàn chương ngắn bất thường so với bản gốc (trừ polish tiêu đề hoặc yêu cầu rút gọn) | Tự xây regex từ lỗi thật đã quan sát; lặp lại đã có trong nguồn thì cho qua |
| 2.3 | **Append thông minh**: bỏ phần chồng lấn giữa tail và đầu đoạn mới; đo tiến độ bằng chữ mới thật; không có tiến độ thì dừng | Seam `draft_chapter append` |
| 2.4 | **Chặn replay**: đầu chương N trùng liên tục với cuối chương N-1 vượt ngưỡng thì từ chối hoặc cảnh báo | Chuẩn hóa dấu câu tiếng Việt; dùng n-gram âm tiết |
| 2.5 | **Lọc `<think>` trong stream** cho provider compat/ollama | Bộ lọc tăng dần, giữ prefix của thẻ bị cắt; thinking dở dang thì không lộ ra ngoài |
| 2.6 | **Đoạn văn bằng chứng**: recall trả về đoạn văn gốc kèm 1 đoạn lân cận mỗi bên, gộp chỗ chồng lấn, ghi rõ chương nguồn | Mở rộng recall trong `novel_context`; không thay store JSON |
| 2.7 | **A/B chương trước**: toàn văn vs tail 800 rune + đoạn bằng chứng | Chạy trên harness 0.2; quyết định theo dữ liệu |
| 2.8 | **Vòng đời finding**: issue có ID, gắn hash của văn bản nguồn; sau rewrite thì recheck nhắm đích từng finding; "đoạn trích cũ biến mất" không được tính là đã sửa | Gắn vào `PendingRewrites`/`ReviewEntry` |
| 2.9 | **Structured length → chia nhỏ**: với caller nhiều item (outline, import batch) thì chia batch hoặc compact rebuild từ fact gốc, không dùng output hỏng làm bằng chứng | Chỉ ở caller miền nghiệp vụ, không làm splitter JSON tổng quát |
| 2.10 | **Evidence span cho fact/state**: `{chapter, content_hash, byte_start, byte_end}` tùy chọn; code tự định vị quote duy nhất | Nền cho 2.6 và cho việc đánh dấu stale khi sửa chương trước |
| 2.11 | **Cast identity-first**: manifest (ID/tên/vai/quan hệ) đóng băng trước, sinh chi tiết sau; chi tiết phải echo đúng ID và tên | `save_foundation` |
| 2.12 | **Thread ledger mở rộng**: ý đồ tác giả, cửa sổ chương mục tiêu, `abandoned`, dormant/overdue, evidence | Giữ cơ chế nhắc lại theo tuổi hiện có |

### P3 — Tùy chọn hoặc để sau

- Preset reasoning theo dự án: draft thấp, plan vừa, review cao. Ghi lại cả requested và effective.
- Lưu ứng viên khôi phục cho stream bị lỗi, không đưa vào history hội thoại.
- Biên nhận context (included/omitted) để chẩn đoán. Giữ chính sách soft budget hiện tại; **không** chuyển sang hard fail.
- Cast proposal cần tác giả duyệt, và cơ chế khai báo nhân vật mới trong outline. Chỉ áp dụng cho chế độ review/import, không chặn chế độ auto.
- Reconciliation trước khi viết giữa kế hoạch và lịch sử: chỉ thử nghiệm trên chương có xung đột.
- Repair JSON chỉ sửa cú pháp, có kiểm tra bảo toàn scalar.
- Đánh dấu stale cho phần semantic phía sau khi sửa chương trước, theo generation counter.
- Writing skill theo stage, đóng băng khi khởi chạy.
- Embedding: chỉ làm sau khi eval `nao-w0` đạt ngưỡng W4.

## Decision matrix

| # | Quyết định | Cách của Pagekin | Cách của mình | Hybrid | Risk | Chọn |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Điều phối | Workflow do người dùng bấm | Router + arbiter tự động | — | low | **Local** |
| 2 | Sửa chương | Viết lại toàn chương + merge theo đoạn | Patch `edit_chapter` | Thêm gate rút ngắn bất thường cho lần ghi đè toàn chương | low | **Local + gate** |
| 3 | Review | 6 vùng khách quan + goal channel | 7 chiều 0–100 | Giữ 7 chiều, thêm goal channel và tách lỗi chặn khỏi gợi ý | medium | **Hybrid** |
| 4 | Context chương trước | Toàn văn bắt buộc | Tail 800 rune | Quyết định qua A/B | medium | **Đo trước** |
| 5 | Ngân sách context | Hard fail khi thiếu material bắt buộc | Soft overflow (sau sự cố retry loop thật) | Thêm biên nhận và chẩn đoán | medium | **Local + receipts** |
| 6 | Memory | Fact = locator, đọc lại văn bản gốc | Đưa summary/state trực tiếp | Giữ JSON, thêm span và đoạn bằng chứng | medium | **Hybrid** |
| 7 | Retrieval | Vector + LIKE | Lexical/entity, W4 chờ eval | — | low | **Local, giữ cổng eval** |
| 8 | Import | Lấy mẫu + cắt 6.000 ký tự | Phân tích toàn văn + ledger | — | low | **Local** |
| 9 | Structured output | Giới hạn số lần, chia batch | Sửa vô hạn | Giới hạn + chia batch ở caller | low | **Học** |
| 10 | Duyệt thủ công từng finding | Bắt buộc | Auto/review mode | Chỉ áp dụng cho review mode | low | **Local** |

Risk score: **Low**, với 2 giả định cần kiểm chứng:

- (a) Chi phí token của việc đưa toàn văn chương trước có xứng đáng hay không. Mục 2.7 sẽ trả lời.
- (b) Đổi schema `save_review`/`ReviewEntry` ảnh hưởng tới web UI, `diag` và prompt cache.

Không có giả định nào dẫn tới mất dữ liệu.

## Không nên học

- Đừng tin comment hay tên tính năng thay cho hành vi thật:
  - Comment ghi "BM25/hybrid" nhưng code thực chạy vector + `LIKE` (`electron/vector-store.ts:1436-1511`).
  - Comment ghi "stable-prefix cache" nhưng material động lại đặt trước template (`generate-draft.command.ts:654-657,838-841`).
  - Prompt ghi "excerpt phải duy nhất" nhưng code không kiểm tra điều đó ở mọi nhánh (`chapter-goal-review.ts:78-90`).
- Đừng coi "quote định vị được", "JSON hợp lệ", "finish=stop" hay "model nói đã sửa" là đúng về ngữ nghĩa. Chính tài liệu của Pagekin thừa nhận điều này.
- Đừng mang các mặc định thương mại kiểu web novel Trung Quốc (mở đầu bùng nổ, chương nào cũng phải có hook, cứ 3–5 chương một tiểu cao trào) vào làm luật chung. Chúng chỉ nên là preset.
- Đừng áp các hằng số cứng của họ như luật chất lượng (12 fact, 12 nguồn, 6 thread, 300 đơn vị tiến độ, 60% sàn rewrite). Chỉ coi là điểm khởi đầu để hiệu chỉnh.
- Đừng mang bộ máy hành chính chất lượng của họ (feature-union 153 assertion, program v1/v2/v3). Chỉ lấy tinh thần: input đóng băng, bằng chứng trung thực, chấm độc lập.
- Đừng mang parser tool-call dạng XML hay các đường hiển thị kép. Pipeline native content-block của mình sạch hơn.
- Đừng thêm đa ngôn ngữ chỉ để ngang bằng họ. Sản phẩm của mình chủ đích viết tiếng Việt (`internal/bootstrap/config.go:322-325`).

## Đính chính sau khi lập plan

Phạm vi: phần so sánh và đề xuất bên trên là ảnh chụp nghiên cứu trước triển khai, không phải hợp đồng runtime hiện hành. Các quyết định đã chốt nằm trong [plan triển khai](../261027-1443-ai-core-quality-uplift/plan.md); hành vi hiện tại phải đối chiếu code.

- **Ngân sách context là mềm, không hard fail.** `finalizeContextPayload` giữ payload đã cắt và thêm cảnh báo, không trả lỗi chỉ vì vượt ngân sách: [`internal/tools/novel_context.go:236–264`](../../internal/tools/novel_context.go#L236-L264). Đây là chủ ý tránh vòng lặp gọi lại khi agent không nhận được ngữ cảnh; không chuyển sang hard fail như Pagekin.
- **Bỏ mục 2.9 và 2.11.** Quyết định D1 của plan bỏ chia batch khi length vì import đã có salvage/halve, các caller structured còn lại nhỏ và có giới hạn. D2 bỏ cast identity-first vì cast được lưu một lần, ID được giữ theo tên/biệt danh; không có đường sinh chi tiết qua nhiều batch cần cơ chế đó. Không coi hai đề xuất nghiên cứu này là phần còn thiếu của sản phẩm.
- **Span lưu bằng quote, không bằng offset.** `EvidenceRef` chỉ giữ chương, action và quote; [LocateQuote](../../internal/domain/quote.go) định vị lại khi dùng. Các đề xuất byte offset trong mục 2.1/2.10 không phải hợp đồng được chọn; kiểu hiện tại ở [domain/review.go](../../internal/domain/review.go).
- **Recheck dùng `save_review`, không thêm tool.** Quyết định D8 của plan đã chọn đường này; [SaveReviewTool](../../internal/tools/save_review.go) tiếp nhận `recheck` / `recheck_results`, [Route](../../internal/flow/router.go) đọc `PendingRechecks` để giao Editor. `unfixed` đưa chương lại vào hàng đợi sửa theo phép gộp của [ApplyReviewOutcome](../../internal/store/progress.go), không xem “đã đổi văn bản” là “đã sửa lỗi”.


## Bước tiếp theo đề xuất (tại thời điểm nghiên cứu)

Dựng P0 (eval assert verdict + bộ case calibration) trước, sau đó làm P1 như một gói thay đổi prompt và A/B trên harness đó. Muốn triển khai thì chạy `/ak:plan` với báo cáo này làm đầu vào (chế độ `--port`), rồi `/ak:cook`.
