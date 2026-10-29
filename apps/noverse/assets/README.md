# assets 内容地图

给系统加"一段话 / 一篇资料 / 一条规则"之前，先查下表确定归属，再看接线方式。

| 目录 | 装什么 | 谁消费 | 接线方式 |
|---|---|---|---|
| `prompts/` | Worker system prompt（writer / editor / architect×2）、Arbiter 裁定 prompt 与一次性任务 prompt（import / simulation / revision） | `agents/build.go`、`internal/arbiter`、imp / sim / revision runner | `load.go` Prompts 字段。注意：simulation_guidance 由 `load.go` 加载时注入，md 文件里看不到 |
| `references/` | 题材无关的写作知识材料。不进 system prompt，由 novel_context 按角色 / 章节裁剪后注入 `reference_pack` | writer / editor / architect | **三处接线**：`tools.References` 加字段 + `load.go` loadReferences 读取 + `novel_context.go` writerReferences / architectReferences 注入。放进目录不会自动加载 |
| `references/genres/<style>/` | 题材专属知识（style-references / arc-templates） | 同上，`style != default` 时加载 | `load.go` loadReferences |
| `rules/` | 已废弃的旧内置规则目录；机械基线已迁到代码，用户规则来自 `~/.ainovel/rules/*.md` / `./.ainovel/rules/*.md` 的自然语言快照 | `userrules.Service` 归一化为 `meta/user_rules.json`；`novel_context` 注入；`commit_chapter` 检查 | 内置基线见 `internal/rules/snapshot.go` 的 `SystemDefaults()`；用户 `.md` 零格式、零 YAML，按自然语言归一化 |
| `styles/<style>.md` | 题材写作风格指令 | 拼进 **writer** 的 system prompt（`agents/build.go`） | 文件名即 `config.style` 取值。与 `references/genres/<style>/` 是同一题材概念的两种载体：前者是风格指令，后者是知识材料 |

## 新内容归属判断（五问）

1. 这个流程必须被**保证**？→ 不写 prompt，写代码约束（StopAfterTools / 工具守卫 / Flow Router）
2. 这是裁定判据？→ 查表型流程写 `internal/flow/router.go`；语义判断写 `prompts/arbiter-*.md`
3. 这是某个角色的审美 / 执行标准？→ `prompts/<role>.md`
4. 这是可机械枚举的默认规则（禁词 / 阈值）？→ `internal/rules/snapshot.go` 的 `SystemDefaults()`；用户自定义规则写进 `.ainovel/rules/*.md`，由归一化快照消费（字数/篇幅是语义软约束，走 preferences，不做机械规则）
5. 这是写作知识材料？→ `references/`（记得三处接线）

## 一致性保障

prompt 引用的信封路径（`working_memory.*` 等）必须与 `novel_context` 保持一致。工具参数形状只在工具 Schema 中定义；prompt 只补充 Schema 无法表达的业务语义，不再复制 JSON 参数清单和形状示例。

prompt 可以描述单个 Worker 的执行方法，但全局路由、状态迁移和恢复逻辑只以代码为准。能够从 Store 事实确定的步骤放进 Router/Tool；需要理解小说内容或用户意图的判断才留给模型。

## Phép đo theo gói (P1-1/P1-5)

`goi.json` khai `do_do`: `procedural_max` (số lượt từ thủ tục mỗi chương, **không**
phải lần/1.000 tiếng), `quiet_max`, `sentence_max`, `paragraph_max` (tiếng),
`repeated_min` (số lần xuất hiện của cụm 5 tiếng), `opening_words`,
`opening_overlap_max` (tỷ lệ 0–1), `day_ordinal`, `english_words`. Khoảng thoại
lấy từ `thoai.min/max` (phần trăm). A/B/C giữ các giá trị của
`docs/prd/rules/kiem_tra_truyen.py`, riêng khoảng thoại theo bộ thể loại.
`tieng_moi_chuong.min/max` là số tiếng mỗi chương, phải khớp dòng "Độ dài" của
luật bộ (`TestPackChapterWordsMatchRules` bắt lệch); presets trả nó thành
`chapterWords`, `null` khi gói không khai.

Các số đo này chỉ sinh **warning**, không chặn nộp chương. Không có `do_do`
thì giữ nguyên lint cũ, kể cả lỗi tiếng Anh chức năng. Danh sách tiếng Anh của
gói mở rộng `english_residue` ở mức warning, dùng biên từ Unicode của Python,
không dùng `\b` ASCII của Go. Tiếng của script là mọi token cách trắng
(`str.split()`), khác `domain.WordCount` dùng cho ngưỡng mệt từ. Đoạn của script
là từng dòng không rỗng, không phải khối Markdown; câu tách sau `. ! ? …` ngay
trước khoảng trắng; chỉ dấu “…” tính là thoại. Không đổi các định nghĩa để
làm đẹp kết quả.

`rules.Measure` trả cả số đo thô để so với script. `CheckChapter` đo thoại/thủ
tục/câu/đoạn/ngày/từ Anh trên chương đang nộp; quãng im thoại và cụm lặp dùng
toàn bộ chương trước trong cùng truyện, đúng phép đo toàn truyện của script.
Tiêu đề Markdown đầu chương của store được bỏ khi đo, nhưng vẫn được lint cũ
kiểm tra. Mở đầu so với chương một đã nộp của các truyện anh em cùng dự án
(`story.json`/`project.json`), không so với chương trước của chính truyện này.
Truyện cũ không có metadata dự án/truyện bỏ qua so mở đầu.

Hai kênh giữ riêng: snapshot người dùng không lưu ngưỡng của gói. Lúc kiểm tra
ghép `SystemDefaults < gói đang nạp < user_structured` (ý định ghi đè tường minh).
Đổi gói rồi mở lại công cụ/kiểm tra lại không cần dựng lại snapshot. Snapshot
cũ chưa có `user_structured` chỉ suy được ghi đè khác mặc định; ý định cũ ghi
đúng bằng mặc định không thể phục hồi từ dữ liệu đã trộn. Snapshot mới lưu
provenance này để không bị mất ý định đó.

Khoá `fatigue_words` nếu có là `word → per_1000`, được chuyển qua
`SystemDefaults(overrides)` trước khi ghép ý định người dùng. Ba gói hiện không
khai khoá này: B/C không có số giới hạn cụ thể; A §6 có giới hạn hai lần
**cả truyện**, không phải mật độ mỗi chương. Không đổi sai đơn vị sang per_1000;
giới hạn cả truyện cần bộ đếm xuyên chương riêng, ngoài lát P1-5.

Golden tại `internal/rules/testdata/tra-xanh/`: tám chương thật (83.345 byte,
chép nguyên byte từ mẫu PO; đuôi `.txt`), `oracle.json` cho cả bản editor và
`by-chapter.json` cho từng chương. Test ráp lại đúng bản editor xuất gốc:
tiêu đề sách, tiêu đề `第 N 章`, ba dấu xuống dòng giữa chương. Golden sinh
từ chính `main` của script, không từ Go. Lệnh oracle:

```sh
python3 docs/prd/rules/kiem_tra_truyen.py /tmp/ainovel-metrics-editor.txt --so-voi /tmp/ainovel-metrics-editor.txt
go test ./internal/rules -run 'TestMeasurementsOracle|TestMeasurementDefinitions|TestPackWarningsAndPackOff|TestLivePackOverridesWithoutSnapshotRebuild|TestSiblingOpeningsProjectScope' -v
```

Tự so cùng bản chỉ kiểm phép tính mở đầu (100%), không phải phán đoán tính
nguyên bản. Mốc PRD tái hiện được: chương 7 có 475/1.673 tiếng thoại (28%);
quãng 725–742 dài 332 tiếng **đi qua ranh giới chương 5–6**. Kiểm riêng từng
chương không thấy quãng này, nên không dùng kết quả riêng chương thay cho
golden toàn truyện. Web còn chờ D6 để hiển thị nhãn/số đo mới; không thay
frontend trong lát này.

### Đối chiếu mẫu PO

Mọi ô dưới đây là **Python/Go**; dòng của phép đo riêng chương tính từ một.
Số cụm lặp là số cụm khác nhau thỏa bộ lọc STOP của script, không phải số lượt.

| Chương | Tiếng | Tiếng thoại | % thoại | Thủ tục | Từ Anh (số lượt) | Dòng ngày thứ | Câu >40 | Dòng đoạn >90 | Cụm lặp | Quãng >250 | Trùng mở đầu tự so |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 1611/1611 | 540/540 | 34/34 | 15/15 | 0/0 | ∅/∅ | 0/0 | 3/3 | 0/0 | 0/0 | 100%/100% |
| 2 | 1700/1700 | 590/590 | 35/35 | 5/5 | 0/0 | ∅/∅ | 0/0 | ∅/∅ | 0/0 | 0/0 | 100%/100% |
| 3 | 1594/1594 | 692/692 | 43/43 | 3/3 | live 1/1 | ∅/∅ | 0/0 | ∅/∅ | 0/0 | 0/0 | 100%/100% |
| 4 | 1610/1610 | 655/655 | 41/41 | 15/15 | live 2/2 | ∅/∅ | 0/0 | ∅/∅ | 1/1 | 0/0 | 100%/100% |
| 5 | 1655/1655 | 699/699 | 42/42 | 18/18 | casting 4/4, live 1/1, scandal 1/1 | 129/129 | 0/0 | ∅/∅ | 0/0 | 0/0 | 100%/100% |
| 6 | 1639/1639 | 569/569 | 35/35 | 19/19 | 0/0 | 7/7 | 0/0 | ∅/∅ | 0/0 | 0/0 | 100%/100% |
| 7 | 1673/1673 | 475/475 | 28/28 | 20/20 | credit 3/3 | ∅/∅ | 0/0 | ∅/∅ | 0/0 | 0/0 | 100%/100% |
| 8 | 1899/1899 | 579/579 | 30/30 | 17/17 | credit 3/3 | ∅/∅ | 0/0 | 91,127 / 91,127 | 1/1 | 0/0 | 100%/100% |

| Phép đo trên cả bản editor | Python | Go |
| --- | --- | --- |
| Từ Anh | casting 4, credit 6, live 4, scandal 1 | casting 4, credit 6, live 4, scandal 1 |
| Dòng ngày thứ | 725, 742 | 725, 742 |
| Câu >40 tiếng | 0 | 0 |
| Dòng đoạn >90 tiếng | 5, 1086, 1122 | 5, 1086, 1122 |
| Cụm 5 tiếng lặp, qua lọc STOP | 9 | 9 |
| Quãng >250 tiếng | 725–742: 332 tiếng | 725–742: 332 tiếng |
| Mở đầu (tự so cùng bản) | 100% | 100% |

Không có chênh lệch số đo. Câu dài không xuất hiện trong mẫu thật nên có
fixture tổng hợp kiểm cả ranh giới dấu câu/khoảng trắng và warning vượt ngưỡng.
