# 06 · Dữ liệu, API & SSE

Nguồn sự thật: `web/src/api/types.ts`, `web/src/api/client.ts`, `web/src/api/useBookEvents.ts`, `internal/entry/web/{gateway,worker}.go`, `internal/host/*`, `internal/entry/tui/panels_*.go`.

## Ranh giới hợp đồng chất lượng AI

Hợp đồng engine không có bản sao riêng cho thiết kế web. [Hợp đồng đọc truyện](11-api-contract.md) trỏ tới kiểu nguồn của checklist `goal_checks`, `recheck` / `recheck_results` và sổ phục bút; DTO máy sinh ở [api_dto.go](../../../internal/entry/web/api_dto.go) và [types.gen.ts](../../../web/src/api/types.gen.ts) là nơi đối chiếu payload UI.

- `contract_status` là kết quả suy ra từ checklist, không phải một kết luận độc lập do giao diện tính lại; chủ sở hữu là [domain/goals.go](../../../internal/domain/goals.go), kiểm tra đầu vào ở [save_review.go](../../../internal/tools/save_review.go).
- Hàng chờ kiểm lại `Progress.PendingRechecks` là trạng thái engine, không suy ra từ việc văn bản đã đổi hay tự thêm trường snapshot. Lối đi Router → Editor → `save_review` và điều kiện chờ cả hai hàng đợi xem [cổng đẩy chương](../../chapter-advance-gate.md).
- Quote của timeline/state/foreshadow và `ForeshadowEntry.evidence` là bằng chứng truy hồi, không phải chỉ thị UI hay offset bền vững; quyền dữ liệu, đọc lại chính văn và ngân sách mềm thuộc [quản lý ngữ cảnh](../../context-management.md).


## API hiện có (dùng ngay)

| Hành động UI | API | Ghi chú |
|---|---|---|
| Danh sách truyện | `GET /api/books` → `{root, books: BookSummary[]}` | Hiện = thư mục con của `--root` |
| Tạo truyện | `POST /api/books {name}` | 409 nếu trùng |
| Đóng worker | `POST /api/books/:id/close` | Khi rời truyện lâu / đóng tab |
| Snapshot | `GET /api/books/:id/snapshot` | Gọi trước khi mở SSE (phân biệt 401) |
| Gửi yêu cầu / can thiệp / tiếp tục | `POST /input {text}` → `{action:'start'\|'steer'\|'continue'}` | Một ô chat cho cả 3 |
| Tạm dừng | `POST /abort` | `Esc` |
| Tiếp tục | `POST /resume` → `{label}` | |
| Duyệt chương tiếp | `POST /next` | `/next` |
| Nghiệm thu từng chương | `POST /review {on}` | `/review on\|off` |
| Viết tiếp tập mới | `POST /reopen {direction}` | Hộp thoại Reopen |
| Danh sách chương | `GET /chapters` → `ChapterInfo[]` | Đại cương, Trình đọc |
| Nội dung chương | `GET /chapters/:n` → `{chapter,title,text}` | Trình đọc |
| Tải xuất | `GET /export?format=txt\|epub` | Tải về trình duyệt |
| Luồng sự kiện | `GET /events` (SSE) | Xem dưới |

Mã lỗi: 401 → `#/login` (phiên hết hạn, xem `09-auth-onboard.md`); 403 → “Không có quyền” (vai trò/quyền truyện); 429 → đếm ngược `retryAfter`; 409 `errBookLocked` → LockedDialog; 404 → về màn Dự án; 502 → banner lỗi worker.

## SSE `/events`

| Event | Data | Xử lý |
|---|---|---|
| `reset` | `{}` | Đánh dấu nối lại; giữ lịch sử đang hiển thị đến bản phục hồi nguyên tử |
| `snapshot` | `Snapshot` | Thay snapshot (so sánh nông) |
| `event` | `HostEvent` | Upsert theo `id`; giữ chi tiết đầy đủ của các bước trong luồng |
| `delta` | `{text}` | Nối vào `streamText` (gom theo rAF) |
| `stream` | `{text}` | Thay toàn bộ bản ghi nguyên tử khi nối lại, không xoá rồi nối từng mảnh |
| `clear` | `{}` | Thông báo ranh giới cũ; không xoá lịch sử |
| `done` | `{}` | Tăng `revision` → refetch chapters |

Reconnect: backoff `min(1000·2^n, 15000)` ms → `ConnectionBanner` đếm ngược.

Luồng chung `/api/events` bọc các khung `stream`/`delta` bằng `seq` tăng dần theo truyện. Bản `stream` chỉ chứa các batch đã flush; batch đang gom chưa được công bố vào bản phục hồi. SSE writer giữ watermark riêng cho từng kết nối và truyện: khung có `seq` nhỏ hơn hoặc bằng watermark bị bỏ trước khi ghi ra mạng. Quy tắc áp dụng cả preamble khi nối lại lẫn snapshot do đổi watch, để một batch không vừa nằm trong bản phục hồi vừa được nối lại lần nữa.

## Snapshot → UI

| UI | Trường |
|---|---|
| StatusChip | `runtimeState` (`idle\|running\|pausing\|paused\|completed`), `statusLabel` (`READY\|RUNNING\|REVIEW\|REWRITE\|COMPLETE`) |
| Chip Giai đoạn | `phase`: `init`→Khởi tạo · `premise`→Tiền đề · `outline`→Đại cương · `writing`→Sáng tác · `complete`→Hoàn thành |
| Chip Quy trình | `flow`: `writing`→Sáng tác · `reviewing`→Thẩm duyệt · `rewriting`→Viết lại · `polishing`→Gọt giũa · `steering`→Can thiệp |
| Chip Chế độ | `advanceMode` (`auto`→Tự động, `review`→Nghiệm thu từng chương), `advancePermitChapter` → “Đã duyệt chương N” |
| Chip Đang viết / Đang viết lại / Đang gọt giũa | `inProgressChapter` + `flow` |
| Chip khôi phục | `recoveryLabel` (dịch, xem bảng) |
| GateCard | `advanceMode=review` && !`isRunning` && `advancePermitChapter` < chương kế; `hasAdvanceHold`/`advanceHoldReason` → “Tạm dừng: …” |
| Hiện tại (tab Agent) | `pendingSteer` → “Đang xử lý can thiệp người dùng”; `pendingRewrites` → “Đang xử lý viết lại/sửa đổi” |
| RewriteQueueCard | `pendingRewrites`, `rewriteReason` |
| OutlineRail / VolumeOutline | `outline[] {chapter,title,coreEvent}`, `completedCount`, `inProgressChapter`, `layered`, `currentVolumeArc`, `nextVolumeTitle`, `compassDirection/Scale` |
| AgentCard | `agents[]`: `name`, `state` (`running\|failed\|idle\|working`*), `summary`, `tool`, `context.percent/scope/strategy` |
| CostCard | `totalInputTokens`, `totalOutputTokens`, `totalCostUSD`, `totalSavedUSD`, `budgetLimitUSD`, `cachePerAgent[]`, `cachePerModel[]` |
| CacheCard | `totalCacheReadTokens/WriteTokens`, `overallRecent*`, `totalCacheBreaks`, `missingAssistantUsage`, `overallCacheCapable`, per-agent `cacheRead/input` |
| Tab Truyện | `synopsis`, `premise`, `characters[]`, `supportingCount`, `recentSupporting[]`, `lastCommitSummary`, `lastReviewSummary`, `recentSummaries[]` |
| Model chip | `provider`, `modelName`, `modelContextWindow`, `thinkingLevel` (rỗng → `auto`) |

\* `working` là giá trị observer đang phát nhưng TUI không map — UI coi như `running`.

Context scope: `baseline`→chuẩn · `projected`→dự đoán · `recovered`→khôi phục · `committed`→đã commit · `skipped`→bỏ qua. Strategy: `tool_result_microcompact`→nén nhỏ kết quả tool · `light_trim`→cắt tỉa nhẹ · `full_summary`→tóm tắt toàn diện. Màu ctx: ok <70% · warn 70–85% · err ≥85%. Cache hit: ok ≥50% · warn ≥20% · err.

## HostEvent → ActivityRow

`{id, time, finishedAt, failed, category, agent, summary, kind, level, depth, durationNs, retryAt}`

| `category` | Icon | Chữ |
|---|---|---|
| `DISPATCH` | Spinner khi chưa `finishedAt`; tick ok; ✕ err nếu `failed` | `AGENT` (màu agent) + `（nhiệm vụ）` (fg-4) |
| `TOOL` (`depth=1`) | `├` (fg-5) khi xong; spinner khi chạy; ✕ err khi lỗi | Tên tool mono fg-3, thụt 22px |
| `DECISION` | như DISPATCH | `ARBITER（nhãn phán quyết）` |
| `SYSTEM` | icon setting accent; `level=warn` → chữ warn; `success` → ok | `summary` đã dịch |
| `USER` | icon edit màu `user` | chữ `user-fg` |
| `ERROR` | ✕ err | chữ err |

Thời lượng `durationNs`: `<1s` → `850ms`, `<60s` → `12.3s`, còn lại `2m5s`, nghiêng fg-4. `retryAt` → hậu tố “· Thử lại sau Ns” cập nhật mỗi giây. Upsert theo `id` (dispatch/tool/decision tự cập nhật tại chỗ).

## Bảng dịch (zh → vi) — tương thích dữ liệu cũ

Host phát tiếng Việt trực tiếp (19-gap-closure §B). Bảng regex dưới đây (`web/src/i18n/hostStrings.ts`) chỉ còn dịch sự kiện/snapshot đã lưu từ bản tiếng Trung; đầu ra mỗi mẫu = đúng câu host Việt hiện phát cho cùng sự kiện. Chuỗi Việt mới đi qua nguyên văn.

| Mẫu gốc | Tiếng Việt |
|---|---|
| `写第 N 章` | Viết chương N |
| `重写第 N 章` | Viết lại chương N |
| `打磨第 N 章` | Gọt giũa chương N |
| `启动裁定` · `启动补裁` · `用户干预裁定` · `僵局裁定` · `失败裁定` | Phán định khởi động · Phán định khởi động bổ sung · Phán định can thiệp · Phán định bế tắc · Phán định thất bại |
| `裁定: X` | Phán định: X |
| `已重开全书返工: 第 [a b] 章入队` | Đã mở lại toàn bộ để viết lại: chương [a b] vào hàng đợi |
| `已重开本书为创作状态（用户撤销完结裁定）` | Đã mở lại truyện để sáng tác tiếp (bạn hủy phán định hoàn thành) |
| `重试 (a/b): msg` | Thử lại (a/b): msg |
| `X 上下文 P% (u/w) 策略: s` | X ngữ cảnh P% (u/w) · chiến lược: s(dịch) |
| `开始创作（规划师: A——R）` | Bắt đầu sáng tác (người lập kế hoạch: A — R) |
| `恢复创作: L` · `一致性告警: …` | Tiếp tục sáng tác: L · Cảnh báo nhất quán: … |
| `用户手动暂停当前创作` | Bạn đã tạm dừng sáng tác |
| `引擎停止 (已完成 N 章)` | Engine dừng (đã xong N chương) |
| `《T》创作完成: 共 N 章 M 字` | «T» hoàn thành: N chương, M chữ (M nhóm nghìn kiểu vi-VN) |
| `写作规则已更新并持久化` | Quy tắc viết đã cập nhật và lưu lại |
| `章节推进模式已切换为自动推进/逐章验收` | Đã chuyển cách đẩy chương: Tự động / Nghiệm thu từng chương |
| `已放行第 N 章；…` | Đã duyệt chương N; … |
| `已完成至第 N 章，逐章验收等待放行第 M 章；…` | Đã xong đến chương N, chờ duyệt chương M; dùng /next hoặc nhập ý kiến sửa |
| `预算告警/预算用尽/预算停机/预算盲区` | Cảnh báo ngân sách / Hết ngân sách / Dừng do ngân sách / Không đo được ngân sách |
| `StopGuard: … 已拦截催促（连续第 N 次）` | StopGuard: … đã chặn lệnh thúc giục (lần thứ N liên tiếp) |
| `模型已切换：r → p/m` · `推理强度已切换：r → l` | Đã đổi model: r → p/m · Đã đổi mức suy luận: r → l |
| `[继续] X` · `[用户干预] X` | [Tiếp tục] X · [Can thiệp] X |
| `干预裁定失败：E（未做任何修改）` | Phán định can thiệp thất bại: E (chưa thay đổi gì) |
| `第N章` · `第N章·草稿` · `本弧` · `全局` | Ch.N · Ch.N · bản nháp · cung này · toàn cục |
| statusLabel `就绪/运行中/审阅/返工/完成/暂停/暂停中/错误` | Sẵn sàng / Đang chạy / Thẩm duyệt / Viết lại / Hoàn thành / Tạm dừng / Đang tạm dừng / Lỗi |
| recoveryLabel `重写恢复：N 章待处理` · `打磨恢复：N 章待处理` · `恢复：第 N 章进行中` · `恢复：审阅中断` … | Khôi phục viết lại: N chương chờ xử lý · Khôi phục gọt giũa: N chương chờ xử lý · Khôi phục: chương N đang viết · Khôi phục: thẩm duyệt bị gián đoạn … |

Không khớp mẫu → hiển thị nguyên văn (không ẩn).

## Luồng viết (`streamText`) → StreamRound

Định dạng do `internal/host/stream_extract.go` sinh (header Việt; `TestStreamHeadersMatchWebStreamRounds` giữ hai phía đồng bộ):

1. Tách theo dòng bắt đầu bằng `✻ `. Mỗi đoạn = 1 round. Phần trước header đầu tiên (hoặc round không header) = **văn xuôi** (`draft_chapter.content`) → render Literata, caret khi đang chạy.
2. Ký tự `\x02` bao đoạn **suy nghĩ** → `ThinkingBlock` (chữ fg-4 nghiêng).
3. Thân round là YAML-like: `key: value`, thụt 2 dấu cách cho lồng, `- ` cho phần tử mảng. Parse thành cây `{key, value|children}`.
4. Round cuối là `live` khi `isRunning`; round cũ mặc định thu gọn, trừ `edit_chapter` và `commit_chapter`.
5. Giữ đầy đủ các round. Ranh giới `\\n\\u001e\\n` chỉ tách vòng; không xoá nội dung. Nguồn phục hồi bền vững: [stream_history.go](../../../internal/entry/web/stream_history.go), không phải thư mục tạm của worker hosted.

| Header (hiện tại · cũ) | Nhãn vi | Tool | Render |
|---|---|---|---|
| `✻ Truy vấn ngữ cảnh` · `✻ 查询上下文` | Truy vấn ngữ cảnh | `novel_context` | chip key/value |
| `✻ Đọc chương` · `✻ 读章节` | Đọc chương | `read_chapter` | chip (chapter, source, from, to, max_runes, character) |
| `✻ Kiểm tra nhất quán` · `✻ 一致性检查` | Kiểm tra nhất quán | `check_consistency` | chip |
| `✻ Gọt giũa` · `✻ 打磨` | Gọt giũa | `edit_chapter` | `DiffInline` từ `old_string`/`new_string` + `replace_all` |
| `✻ Nộp chương` · `✻ 章节提交` | Nộp chương | `commit_chapter` | chip `chapter`, `hook_type` (err tint), `dominant_strand`, `feedback`, `cast_intros`; `characters` → chip người; `foreshadow_updates[]` → thẻ `action` (plant info · advance warn · resolve ok) + `id` + `description`; `key_events[]` → trích dẫn viền trái accent |
| `✻ Lập kế hoạch` · `✻ 规划` | Lập kế hoạch | `plan_chapter` | thẻ trường (title, goal, conflict, hook, emotion_arc, notes, contract.*) |
| `✻ Thẩm duyệt` · `✻ 审阅` | Thẩm duyệt | `save_review` | verdict badge + issues |
| `✻ Tóm tắt cung` · `✻ 弧摘要` / `✻ Tóm tắt tập` · `✻ 卷摘要` | Tóm tắt cung · Tóm tắt tập | `save_arc_summary` · `save_volume_summary` | văn bản |
| `✻ Thiết lập` · `✻ 设定` | Thiết lập | `save_foundation` | văn bản / chip |
| `✻ Sửa đại cương` · `✻ 修订大纲` | Sửa đại cương | `revise_outline` | văn bản |
| `✻ <khác>` | nguyên văn | — | khối mono |

Con trỏ khi đang stream: chu kỳ `· ✢ ✳ ✶ ✻ ✽` (màu `round-star`, 200ms/khung).

## GAP — API cần bổ sung ở backend

| ID | Nhu cầu | Đề xuất |
|---|---|---|
| GAP-1 | Thư viện → Dự án → Truyện | `GET /api/projects` · `POST /api/projects {slug,name,language,description,copyGlobalConfig}` · `GET /api/projects/:p` · `GET/POST /api/projects/:p/stories` · story id = `:p/:story`. Worker chạy với cwd = thư mục truyện |
| GAP-2 | `project.json` | `{ "name", "description", "language": "vi", "notes", "createdAt" }` |
| GAP-3 | `story.json` | `{ "title", "status": "writing\|done\|published\|paused", "idea", "tags": [], "publishedAt", "createdAt" }` (đồng bộ `title` từ `meta/book.json`) |
| GAP-4 | Xuất tự động | Cấu hình `export.auto {onVolume, onBook, onChapter, formats[], pattern, onExists: replace\|keep}`; ghi vào `<story>/xuat-ban/`; sự kiện SYSTEM khi xuất xong; `GET /export` thêm `from`, `to`, `overwrite` |
| GAP-5 | Đọc/ghi cấu hình | `GET/PUT /api/config?scope=global\|project` (che `api_key`); `POST /api/config/test` (Thử kết nối) |
| GAP-6 | Đồng sáng tác | `POST /api/books/:id/cocreate {message}` → `{reply, draft, ready, suggestions[]}` |
| GAP-7 | Chi tiết truyện | `GET /reviews/latest` (ReviewEntry đầy đủ), `GET /chapters/:n/plan`, `GET /chapters/:n/facts`, `GET /foreshadow`, `GET /characters` (đủ vai trò + mô tả), `GET /chapters/:n?source=draft` |
| GAP-8 | Lệnh TUI-only | `/sync`, `/diag`, `/import` (stream giai đoạn), `/simulate`, `/importsim`, `/start <tệp>`, `/model` (đổi model theo vai trò lúc chạy) |
| GAP-9 | Đổi tên thư mục truyện | `POST /api/projects/:p/stories/:s/rename {slug}` (chỉ khi rảnh) |
| GAP-10 | Kế thừa cấu hình/quy tắc | Engine đọc `.ainovel/config.json` + `rules/` theo chuỗi toàn cục → dự án → truyện |
| GAP-11 | Ý tưởng & tài sản | `GET/POST /api/projects/:p/ideas` (tệp .md), `GET /api/projects/:p/assets` |
| GAP-12 | Kiểm tra cấu trúc | `GET /api/projects/:p/validate` → danh sách thiếu/sai tên |
| GAP-13 | Cấu hình thư viện | `library.json` + `GET/PUT /api/library` — chi tiết `09-auth-onboard.md` §7 |
| GAP-14 | Tự đổi tên hiển thị | `POST /api/me {name}` |
| GAP-15 | Onboard & mật khẩu tạm | `User.onboardedAt`, `User.mustChangePassword`, `POST /api/me/onboarded` |
| GAP-17…21 | Audio (bản đọc, giọng, tạo, ghép, bộ đọc trên máy) | xem `10-audio.md` §4 |
| GAP-16 | Đăng nhập provider từ web | `POST /api/config/oauth/start`, `GET /api/config/oauth/status`, `GET /api/config/models` |
| GAP-22 | Lệnh: tham số có khoảng trắng | `strings.Fields` tách args — hỗ trợ ngoặc kép hoặc chặn ở web, `13-commands.md` §5 |
| GAP-23 | Danh sách lệnh cho web | `GET /api/commands` sinh từ registry + trường `Params`, `13-commands.md` §1.4 |
| GAP-24 | Việt hoá thông báo host | `SetAdvanceMode`, `Reopen`, `requireCleanChapters` còn tiếng Trung, `13-commands.md` §5 |
| GAP-25 | Nút web cho `/next`, `/review` | gọi cùng API với lệnh, `13-commands.md` §5 |
| GAP-26 | Việc chạy nền | `GET /api/jobs`, SSE `job.*`, pause/resume/cancel/retry — `14-jobs-dock.md` §1.7 |
| GAP-27 | Xuất: xử lý tệp trùng | đưa `OnExists` (`replace` / `keep`) ra API |
| GAP-28 | Chẩn đoán dạng JSON | `GET /api/diag` từ `Host.Diagnose()` + gửi Action |
| GAP-29 | Kết quả `/sync` chi tiết | `revision.Result` + số chữ thêm/bớt cho `--check` |
| GAP-30 | Thông báo đẩy & hộp thư | Web Push (VAPID): `POST/DELETE /api/push/subscriptions {endpoint, keys, device}`; `GET/PUT /api/notify/prefs {types{gate,error,job,done,security}, digest: each\|five\|day, quiet{from,to}}`; `GET /api/notifications?before=` + `POST /api/notifications/read {ids\|all}`; payload `{type, story, chapter?, job?, title, body, url, actions[]}` — `18-mobile.md` §M7 |
| GAP-31 | Nhật ký: truy vấn & lời gọi công cụ | Lịch sử sự kiện phân trang có lọc `?chapter=&agent=&level=error&q=&before=`; mỗi sự kiện công cụ kèm `{tool, args (rút gọn), result (rút gọn), durationMs, status, retries[]}` — `18-mobile.md` §M6 |
| GAP-32 | Quy tắc viết áp dụng giữa chừng | `POST /api/rules/understand`, `GET S/rules/effective`, `POST S/rules/apply {recheck}` (giữ luật runtime), `POST S/rules/runtime/:id/keep` · `DELETE …`, lưu `violations` theo chương + `GET S/rules/violations`; đề xuất trộn hợp danh sách cấm — `20-settings-rules.md` §3.4 |
| GAP-33 | Không cần dòng lệnh khi đăng nhập | Mã khôi phục quản trị (`POST /api/me/recovery`, `/rotate`, `POST /api/recover`) + bật/tắt truy cập Wi-Fi nhà từ UI (`PUT /api/system/network {lan}`) — `20-settings-rules.md` §6 |
| GAP-34 | Duyệt & kiểm tra thư mục cho bộ chọn nơi lưu truyện | Admin. `GET /api/fs/places` → `[{id, label, path, icon}]` (Thư mục nhà, Tài liệu, Màn hình, iCloud Drive, Google Drive/OneDrive, ổ ngoài). `GET /api/fs/list?path=` → `[{name, path, kind: dir\|library\|readonly, items, writable}]`: chỉ thư mục, ẩn thư mục chấm và thư mục hệ thống, giới hạn trong các places. `POST /api/fs/mkdir {parent, name}`. `GET /api/library/inspect?path=` → `{kind: found\|empty\|other\|readonly\|missing, projects, stories, display}`. Dùng khi không có `pick-folder` (từ xa / `501`) — `09-auth-onboard.md` §10 |
| GAP-35 | Văn mẫu cho `/simulate` trên web | Admin hoặc người có quyền viết. `GET/POST/DELETE /api/books/:id/style-samples` (dán hoặc tải `.txt`, ≤ 20 mẫu); UI ở Tuỳ chỉnh › Cách viết › Văn phong của bạn. Không lộ thư mục `simulate/` — `22-brief-2510.md` câu hỏi treo |
| GAP-36 | Thẩm duyệt có số mục luật | Mỗi mục: `{level: "error"\|"warning", ref: "Master §7"\|"Bộ A §4.3"\|"Xưng hô"\|"Quy tắc của bạn", refName, text, quote, fix}`; kèm `verdict`, `round`, `pack`. Có `error` ⇒ chương vào hàng chờ sửa, `/review` approve trả 409 — D4 |
| GAP-37 | Khung truyện & bản đồ giữ chân | `GET/PUT /api/books/:id/frame {audience: 0..4, difference, sequelSeed}`; `GET /api/books/:id/retention` → mỗi chương `{questions[], promise, payoff{tag,text}, hook{type: info\|threat\|decision\|line, waiting}}` + `checks[{ref, ok, text}]` (§2, §3, §7, §8 tính ở server) — D5 |
| GAP-38 | Xưng hô & quan hệ sửa trên web | `address_terms.json` `{from, to, call, self, from_chapter, until_chapter, reason}` + `relationship_log.jsonl`. `GET/PUT /api/books/:id/address-terms`, `POST …/relations`. Engine đang viết ⇒ 423 Locked; `from_chapter` ≤ chương đã viết ⇒ 409 `{affected: [2,3]}`, gửi lại với `apply: "forward"\|"mark"` — D7, D8 |
| GAP-39 | Não truyện | `GET /api/books/:id/brain` → `entities[{id, type: char\|place\|obj\|ev\|fs\|rule, name, role, desc, fromChapter, appears[]}]`, `edges[{a, b, phases[{fromChapter, type: enemy\|ally\|work\|own\|secret\|rel, label}]}]`. Web tự tính trạng thái tại chương N — D10 |
| GAP-40 | Xuất bản đọc TTS qua API | `POST /api/books/:id/export {format: txt\|epub\|narration, from, to, readNumbers, onExists: fail\|replace\|keep}`; tự xuất nhận `formats[]` có `narration` — D3 |
| GAP-41 | Lần chạy đầu trên web (A0) | Khi chưa có tài khoản: sinh `setupCode` 8 ký tự (bỏ 0/O/1/I/L), chỉ giữ bản băm trong bộ nhớ, sống 30 phút, in ra stdout/log. `GET /api/setup` → `{needed, expiresAt, attemptsLeft, lockedUntil?}`. `POST /api/setup/verify {code}` → `{setupToken, expiresAt}` · 400 `{attemptsLeft}` · 423 `{lockedUntil}` (sai 5 lần → khoá 10 phút) · 410 hết hạn · 409 đã có quản trị. `POST /api/setup/admin {setupToken, name, username, password}` → `{user, recoveryCode}` + cookie · 400 (tên đăng nhập sai mẫu, mật khẩu < 8) · 409 `{at}` đã có quản trị. Kiểm “chưa có tài khoản” trong cùng giao dịch ghi; tạo xong thì huỷ mã |
| GAP-42 | Ma trận chất lượng (D13) | `GET /api/books/:id/quality` → `{pack, sections[{ref, refName}], chapters[{n, title, status: rewritten\|rewriting\|previous}], cells[{ref, ch, error, warning, fixed, kept}], issues[ReviewIssue + {id, ch, state: open\|fixed\|kept, keptReason?}]}`. Tổng theo hàng và cột tính ở client |
| GAP-43 | Sổ cái dự án & luật khoảng cách (D14) | `GET /api/projects/:id/ledger` → `{axes[10]{n, name}, stories[{id, title, values[10], day7, opening}], channelMedianDay7}`. `POST /api/projects/:id/ledger/check {brief}` → `{checks[{level: error\|warning\|ok, rule, text, axes[], against?}], cells[{story, axis, mark: dup\|overused}]}`. `POST /api/projects/:id/ledger/suggest {brief}` → `{values[10], changed[], brief}` (Kiến trúc sư đề xuất và viết lại yêu cầu). Gọi `check` ở màn Tạo truyện mới trước khi bắt đầu viết; có `level=error` thì server từ chối bắt đầu (409 `{checks}`). Chỉ trả truyện người gọi được xem |
| GAP-44 | Vũ trụ chung (D14) | `GET /api/projects/:id/universe` → `{name, counts{stories, chars, places}, entities[{id, type, name, stories[], lastRole}], checks[{ref: "Master §19", story, ch, text, quoteId}]}` |
| GAP-45 | Tìm trong dự án (D14) | `GET /api/projects/:id/search?q=` → `{entities[{id, type, name, stories[]}], passages[{story, ch, quote, hl[[start, end]]}]}`, lọc theo quyền xem truyện |
| GAP-46 | Não hệ thống (D15, chỉ admin) | `GET /api/system/lessons?state=pending\|kept\|dropped` → `[{id, origin: review\|user, refs[], text, evidence{count, kind, stories, pack, chips[]}, scope: pack\|all, packId, decidedAt?}]`. `POST /api/system/lessons/:id {action: keep\|drop\|edit\|undo, text?, scope?}`; `keep` ghi quy tắc vào lớp “Mọi truyện” hoặc lớp gói. `GET /api/system/violations?pack=` → `{rows[{ref, refName, stories, total, level, alert}], preamble}`. Không lưu tên nhân vật hay tình tiết trong `text`. Người không phải admin: 403 |
| GAP-47 | Quyết định trên mục thẩm duyệt (D4) | `POST /api/books/:id/review/items/:itemId/keep {reason}` → `{state:"kept", reason, by, at}`; `DELETE …/keep` để bỏ giữ. Lưu ở `reviews/decisions.json`, khoá `itemId` ổn định (băm `ref + chương + trích`). Trả trong `items[].decision`. Biên tập viên nhận danh sách mục đã giữ ở lần chấm sau. Thêm `round.at` (RFC3339) cho phụ đề “· 21:52”. |
| GAP-48 | Mô tả gói (D2) | `goi.json` thêm `ten_ngan`, `mo_ta`; `/api/config/presets` › `packs[]` trả `short`, `desc`. `audience` của khung truyện đổi thành `number[]` (gói C `[2, 1]`). |
| GAP-49 | Địa điểm & vật (D10) | chapterfacts thêm `places[]`, `objects[]` dạng `{name, desc, firstChapter, with[]}`; `/brain` gộp vào `entities` với `type: place\|obj`. |
| Bổ sung | GAP-32 · 38 · 41 · 37 · 39 | GAP-32: mỗi quy tắc có `uncertain`, `uncertainNote`. GAP-38: lỗi lint Xưng hô mức error vào `items[]` của vòng chấm với `blocking:true`. GAP-41: 400 `{format:true}` không tính lượt; `GET /api/setup` có `expired:true`. GAP-37: `hook.type` thêm `seed` (chỉ chương cuối). GAP-39: trạng thái chương thêm `writing`. |
| Bổ sung §H | GAP-36 · 39 · 42 · 43 · 44 · 46 | `checks[].short` (câu ngắn theo mẫu từng phép kiểm). `project.universeName` (tuỳ chọn, mặc định tên dự án). Mục thẩm duyệt thêm `rule` (mã con ổn định, vd. `s19-recap`). `chapters[].rewrites[{at, cause: review\|user}]`. Bài học nhóm theo (gói, mục) không tách mức; mục không thuộc gói nhóm qua mọi gói. Không có `dropReason`. Ngày 7 để `null` thì ẩn cột. <!-- bo-sung-h --> |
| Bổ sung §I | GAP-43 · 44 · 46 · 48 | `goi.json` thêm `tieng_moi_chuong {min, max}` (cả 3 gói 1400–1600). `checks[].fix` (gợi ý ngắn theo mẫu, `null` khi đạt). `/universe.checks` bỏ mục đã giữ nguyên, thêm `keptCount`. Chuông của mục dùng chung tính trên mọi gói. `lessons.json` chuyển một lần sang id nhóm (`schema: 2`), giữ quyết định theo thứ tự kept > dropped > pending. |
| Bổ sung §J | GAP-44 · 46 · 47 | `/universe.kept[]` thêm `reason`, `decidedBy`, `decidedAt` (lấy từ quyết định GAP-47) để thẻ đã giữ hiện lý do. `Violations`: `alertStories` dùng cho tooltip chuông và nhãn “· mọi gói” (chỉ khi `alertScope:"all"` và chuông bật). |
| Bổ sung §M | GAP-46 | Bài học: `origin` quyết định chip loại (“Từ góp ý của bạn” / “Từ lần chấm”), không đổi khi đếm lại. Câu nguồn sinh từ bằng chứng theo loại: `userEdits`, `errors`, `warnings` → “Rút từ N lần bạn sửa … và N lần Bắt sửa và N Cảnh báo ở M truyện · K gói”, bỏ phần bằng 0. Đuôi: mục `Bộ …` ghi “· gói {tên}” (`packLabel`), mục dùng chung ghi “· K gói” (`packsCount`, kể cả 1). D9 không còn mức Cảnh báo “Chưa rõ ai nói”. |
| Bổ sung §S | GAP-7 · 39 | `characters[]` (mọi nơi trả nhân vật: tab Truyện, `GET /characters`, `S/brain` `entities` loại `char`, Não dự án, người nói Audio) thêm `kind`: `lead` · `antagonist` · `ally` · `support` và `revealAt`: số chương lộ vai thật hoặc `null` (không có bí mật). Kiến trúc sư đặt `kind` khi lưu nhân vật; người dùng sửa ở Dự án › Nhân vật. Dữ liệu cũ chưa có `kind`: backend tính một lần từ `role` (chính → `lead`; phản diện, đồng phạm → `antagonist`; đồng minh, bạn → `ally`; còn lại → `support`) rồi lưu lại. Thiếu `kind` thì FE coi là `support`. |
| Bổ sung §V | GAP-19 | Hàng đợi tạo giọng: mỗi chương trả `state` (`done` · `rendering` · `queued` · `unlocked`), `sec` (thời lượng thật, `null` khi chưa tạo), `estSec` (ước tính = số chữ ÷ tốc độ đọc của giọng, luôn > 0), `pct` khi đang tạo. Tổng: `madeSec`, `estTotalSec` (thật + ước tính). FE vẽ bề rộng theo `sec ?? estSec`. |
| Bổ sung §W | GAP-16 | Danh mục model (dùng cho `/model` và Tuỳ chỉnh › Trợ lý AI) trả mỗi model `contextWindow` (giá trị đang dùng), `contextSource` (`model_config` · `config` · `registry` · `default`) và `registryWindow` (giá trị danh mục, để hiện “Tự nhận · N”). Ghi: quản trị sửa `providers.<p>.models[].context_window` (số, bỏ trống = theo danh mục); giá trị 8 000–2 000 000. FE hiển thị nguồn: `registry` → “tự nhận”, `model_config`/`config` → “bạn đặt”, `default` → “dự phòng” (cảnh báo). |
| Bổ sung §AA | GAP-20 | Xuất lẻ: `POST S/audio/volumes/:v/exports { chapters: [n…], split: true, pack: "files"\|"zip", format, subtitles }`. `split` thì mỗi chương một tệp, không nhạc mở/kết, không khoảng lặng giữa chương. Trả `files[{chapter, name, sizeBytes, durationSec, url}]` (+ `zipUrl` khi `pack:"zip"`). Tên tệp máy tính: `Tập 1 · Chương 02 · {tên}.{đuôi}`, điện thoại tải về: `tap-1-chuong-02.{đuôi}`. Xuất cả tập vẫn trả danh sách `chapterFiles[]` để mở “Xem 8 tệp”. |
| Bổ sung §AB | GAP-20 | Tuỳ chọn xuất dùng chung: `GET/PUT S/audio/export-prefs` → `{ format: "mp3"\|"m4b"\|"wav", intro, chapterTitles, loudness, gapSec, chapterMarks, subtitles, pack: "files"\|"zip" }`. Lưu theo truyện, máy tính và điện thoại đọc cùng một bản; mặc định giao diện: `mp3`, các cờ `true`, `gapSec: 2`, `pack: "files"` (khác mặc định `false` của API khi gọi thẳng). `chapterMarks` chỉ có tác dụng với `m4b` và xuất cả tập; xuất lẻ bỏ qua `intro`, `gapSec`, `chapterMarks`. <!-- ab-prefs --> |
| Bổ sung §AD | GAP-14 · 15 · mới | Thành viên: `User.role: "admin"\|"member"` (thay admin/writer/reader) + `User.projects: {[projectId]: "owner"\|"collab"\|"view"}` (thiếu = Không vào; bỏ `books[]`). `POST /api/users {username, name, role, projects, password}`; `POST /api/users/:u {name?, role?, projects?, disabled?, password?, reason?}`; `POST /api/users/:u/delete {reason?}`; `POST /api/users/:u/sessions/revoke-all`. 409 `{error, field:"username", suggest:"linh.2"}` khi trùng; 409 quản trị cuối. Quyền kiểm theo từng request (đổi có hiệu lực ngay, phát `account.changed`); 403 nói bằng lời: “Bạn chỉ có quyền Chỉ xem ở dự án này”. Nhật ký: `GET /api/admin/audit?type=acct\|perm\|sec&actor=&before=&limit=` → `{items:[{id, at, actor:{username,name}\|null, type, action, target, project?, from?, to?, reason?}], next}` — chỉ ghi thêm, giữ 1 năm, ghi cả đổi quyền, tạo/khoá/xoá, đặt lại mật khẩu, đăng xuất thiết bị, đăng nhập sai ≥ 5 lần. **Di trú:** admin → admin; writer → member + `collab`, reader → member + `view` ở mọi dự án chứa truyện trong `books[]` (`"*"` → mọi dự án hiện có); truyện lẻ trong dự án → cả dự án, ghi một dòng nhật ký “Di trú quyền”. <!-- ad-members --> |
| Thiết bị dùng chung | SSO | `GET /api/me/sessions` và `GET /api/users/:u/sessions` → `{ sessions: [{ id, device, browser, lastActiveAt, expiresAt, current }] }`. Thu hồi một phiên hoặc mọi phiên khác áp dụng cho cả Noverse và Viderse. |
| Bổ sung §AE | GAP-20 · 26 | Ghép chạy nền: `POST S/audio/volumes/:v/exports` trả ngay `{jobId}` (202), không giữ request. Việc `Job.kind="export"`, `title:"Ghép Tập 1"`, `unit:{done,total,label:"Chương"}`, `etaSec`; xong `result:{files:[{name,sizeBytes,url}], zipUrl?}`; lỗi `error:{message (lời người dùng), chapter, retryable, cause:"intro_missing"\|…}`. `POST /api/jobs/:id/retry` chạy tiếp từ chương lỗi; `retry {intro:false}` cho *Ghép không nhạc*. Không cho chạy hai việc ghép cùng tập: lần 2 trả 409 `{jobId}` của việc đang chạy. Tự tải là việc của client: khi nhận `job.done` của việc do chính tab này tạo và công tắc bật → tải `files[0].url` (hoặc `zipUrl`); trình duyệt chặn → hiện toast “chưa tải được”. Công tắc lưu `localStorage` theo thiết bị (`exportAutoDownload`). <!-- ae-async --> |
| Bổ sung §AF | GAP-20 · 21 | Metadata audio: trình phát web đặt `navigator.mediaSession.metadata = new MediaMetadata({title, artist:"Noverse Audio", album, artwork:[{src, sizes:"96x96…512x512", type:"image/png"}]})` (+ `chapterInfo` khi trình duyệt hỗ trợ), gọi `setPositionState` mỗi khi đổi chương/tua, đăng ký `seekbackward/seekforward` (10 s), `previoustrack/nexttrack` (chương), `play/pause`. Bìa: `GET S/cover?size=96\|192\|512\|1400` → bìa đã tải hoặc bìa tự tạo (PNG, máy chủ dựng, có cache). Truyện thêm `shortTitle?` (≤ 32 ký tự). Tệp xuất gắn thẻ: MP3/WAV ID3v2.4 `TIT2 TPE1 TALB TPE2 TRCK TPOS TCON TDRC APIC COMM TLAN=vie` (+ `CHAP/CTOC` cho tệp cả tập; WAV thêm khối `LIST/INFO`), M4B `©nam ©ART ©alb aART trkn disk ©gen ©day covr` + chương + `stik=2`. Tải về: `Content-Disposition: attachment; filename="<ascii-khong-dau>.mp3"; filename*=UTF-8''<tên tiếng Việt mã hoá>` — tên theo quy tắc ở `10-audio.md` › A7. Không bao giờ dùng trạng thái giọng (“Chưa chọn giọng”) làm artist. <!-- af-meta --> |
| Bổ sung §AG | — | Điều hướng màn 02: không đổi API. Cần sẵn: `GET /api/projects` có `storyCount`, `writingCount`; mỗi truyện có `status` + `currentChapter` (để hiện “Đang viết lại · chương 4” và nút *Tới chương đang viết*); mục lục `volumes[]{n, chapterCount, state: live\|planned\|unplanned, locked}` + `chapters[]{n, title, state}`. Truyện > 200 chương: tải chương theo tập khi mở tập. Tìm chương lọc ở máy khách (bỏ dấu, khớp số chương). Nhớ tập đang mở theo truyện (`localStorage`). <!-- ag-data --> |
| Bổ sung §AH | GAP-16 | Danh sách model của nhà cung cấp dùng khoá API: `POST /api/config/models/list` `{provider, config}` (config là bản nháp như `/api/config/test`, để dùng được khoá chưa lưu) → `{models:[{name, displayName, description?, contextWindow, contextSource, price?:{input, output} /* USD mỗi 1 triệu token */, recommended}], total, fetchedAt}`. Gọi API danh sách của chính nhà cung cấp: kiểu OpenAI `GET {base}/models` (OpenAI, DeepSeek, Qwen, xAI), OpenRouter `GET /api/v1/models` (`context_length`, `pricing`), Anthropic `GET /v1/models`, Gemini `GET /v1beta/models` (`inputTokenLimit`), Ollama `GET /api/tags`. Cache 10 phút theo nhà cung cấp + khoá; *Tải lại* bỏ cache. Lỗi trả `{error: key_rejected\|unreachable, message}` bằng lời; nhà cung cấp không có API danh sách → `{models:[], unsupported:true}` (UI chỉ còn *Thêm theo tên*). `recommended` lấy từ danh mục model của Noverse. Lưu: `providers.<id>.models[]` (`context_window` lấy từ nhà cung cấp khi danh mục không biết). Từ chối bỏ model đang được `roles.*` hoặc `fallbacks` dùng (409, kèm tên trợ lý). <!-- ah-data --> |
| Bổ sung §AI | — | Vòng đời góp ý theo đoạn. `ParagraphComment` thêm `status: pending\|sent\|fixed\|lost\|unsure\|done`, `sentAt?`, `fixedAt?`, `resolvedAt?`, `fixedBy?: agent\|quote_gone`, `unfixed?: bool`, `moved?: bool`, `relinked?: bool`, `whole?: bool`, `newQuote?` (câu sau khi sửa, để hiện Trước/Sau), `candidates?: [{paragraph, snippet}]` (tối đa 3); `paragraph` + `quote` giữ nguyên làm vị trí gốc, chỉ dùng nội bộ, không hiện số đoạn. **Gắn lại tự động** sau mỗi lần ghi chương: khớp nguyên văn → giữ; khớp gần đúng duy nhất → `moved`; nhiều đoạn gần đúng → `unsure`; không có → `lost`. Với góp ý `sent`: Người viết báo id đã sửa (thêm `fixed_comments: [id]` vào công cụ lưu chương; `Compose` ghi kèm id) → `fixed`; câu trích không còn và tìm được đoạn thay → `fixed` (`quote_gone`); câu còn nguyên → `pending` + `unfixed`. Thao tác: `PATCH …/chapters/:n/comments/:id` `{action: resolve\|reopen\|relink, paragraph?, quote?}` · `{action: resend_whole}`. Tự `fixed→done` khi duyệt lần kế tiếp của truyện. Sự kiện `comments.reanchored {chapter, fixed, unfixed, need}` cho dải báo/toast. <!-- ai-data --> |
| Bổ sung §AK | — | Tạo giọng một ngăn. `AudioChapter` đã có `chapterChanged`, `scriptChanged`, `changedSegments` — dùng để hiện “n đoạn đổi”; thêm `updateEtaSec?` (ước tính chỉ đọc lại đoạn đổi). `POST …/audio/queue` thêm `redo?: number[]` (chương đọc lại từ đầu); chương cần cập nhật gửi như thường → soạn lại phần đổi nếu chương đổi, rồi **chỉ** đọc đoạn chưa có giọng (giữ đoạn không đổi). Job thêm `kind: create\|update\|redo` (nhãn “Đang tạo” / “Đang cập nhật”) và `position` trong hàng đợi. Lỗi trả bằng lời. <!-- ak-data --> |
| Bổ sung §AL | — | Cách viết chuyển vào Dự án & truyện: **không đổi API**. Văn phong vẫn là `config.style` 3 cấp (ghi cấp truyện/dự án như hiện có, lưu ngay thay vì qua bản nháp chung); văn mẫu vẫn là `…/style-samples` theo truyện. Trang dự án cần biết truyện nào chọn văn phong riêng: dùng `effective`/`own` của từng truyện (đã có trong `GET …/config?scope=story`), hoặc thêm `styleBy: [{story, style, own}]` vào tổng quan dự án nếu gọi từng truyện quá chậm. <!-- al-data --> |
| Bổ sung §AM | — | **Không đổi API.** Chọn nhanh ở “Đang áp dụng” ghi đúng các khoá đang có (`config.style`, model người viết, `review` …) ở cấp truyện/dự án, lưu ngay. **Xoá truyện** dùng `DELETE …/stories/{s}` với `{confirm}` như hiện có: ô gõ trên giao diện so với **tên ngắn của truyện** ở phía FE, rồi FE gửi `confirm` = mã truyện (tên thư mục) — người dùng không phải thấy hay gõ mã này. Quyền: chủ dự án (quản trị tính là chủ). Truyện đang chạy → 409 “Truyện đang chạy — dừng trước khi xoá”: FE hiện dải vàng + *Tạm dừng*, khoá nút xoá. Muốn số liệu trong hộp (“58.699 chữ”, “2 giờ 24 phút”, “2 bản xuất”) lấy từ tổng quan truyện sẵn có; thiếu số nào thì bỏ phần đó, không gọi thêm API. <!-- am-data --> |
| Bổ sung §AM · màn Tạo truyện | — | **Không cần API mới.** `POST …/stories` chỉ nhận `name`, `idea`; nên khi bấm *Bắt đầu viết* với văn phong / người viết khác dự án, FE làm: `createStory` → `putConfig('story', {style?, roles?}, project, story)` (chỉ các khoá đã đổi, ghi đúng khoá mà thẻ đang đọc: `style`, `roles.default.model`) → bắt đầu như cũ. Lỗi ở bước ghi cấu hình thì vẫn mở truyện, toast “Chưa lưu được văn phong riêng, đang dùng theo dự án”. Nếu backend muốn gọn một lần gọi: thêm `config?` vào thân `POST …/stories` (không bắt buộc). <!-- am-new-data --> |
| Bổ sung §AN | `GET/PUT /api/system/storage`, `POST …/test`, `POST …/migration{,/pause,/resume,/cancel}` | Lưu trữ tệp — **đã có trong bản thật**, chỉ thêm GAP-AN1…AN8 (`20-settings-rules.md` §8.7). Ánh xạ: `mode` (file/postgres) → `filemode`; `locked` → `envlocked`, `active.source[*]` → nhãn “do máy chủ đặt”; `encryption.available=false` → `nokey`; `active.{driver, provider, endpointHost, presignEndpoint, region, bucket, pathStyle, allowHTTP, delivery, env, accessKeyLast4}` → thẻ trạng thái; `usage{audio,export,cover,music,voiceSample,import}` + `total` → Dung lượng; `cache{usedBytes,maxBytes}` → Bộ nhớ đệm; `operations[{op,outcome,count,bytes,durationSeconds,maxDurationSeconds}]` gộp theo `op` (lỗi = `outcome≠ok`; trung bình = tổng thời gian/số lần) → Hoạt động; `migration{state,copiedCount,copiedBytes,totalCount,totalBytes,bytesPerSecond,errorCount,errors[{kind,story,code}],startedAt}` → thẻ chuyển; `health{lastCheck,error}` → `down`. Kiểm tra: `StorageTestResult{ok, steps[{step,ok,ms,code,reason}], corsProbeUrl, corsProbeExpiresAt}`; bước trình duyệt do FE tự tải `corsProbeUrl`. Ghi: `StorageCandidate{version, driver, provider, endpoint, presignEndpoint, region, bucket, pathStyle, allowHTTP, delivery, accessKeyId?, secretAccessKey?}` (khoá bỏ trống = giữ khoá đã lưu); kết quả `{version, needsMigration, count, bytes}`. <!-- an-data --> |
| Bổ sung §AP | GAP-AP1…AP7 | Quy tắc chỉ hai cấp: **chuyển quy tắc global vào từng dự án rồi ngừng đọc `SourceGlobal`, từ chối `scope=global`** (AP1, bắt buộc cùng FE). `effective` trả `fatigueWords[{word, per1000, perChapter, source}]` + `forbiddenDefaults[]` (AP2). Tiền đề theo mục `GET S/premise` (AP4), `GET S/world-rules` (AP5), `openThreads` trong snapshot (AP6). Đã có, dùng luôn: `GET/PUT S/meta` (title, shortTitle, status, tags, idea, `builds`), `GET/PUT S/frame`, `GET /ledger` (bỏ `day7` khỏi UI). Chi tiết `20-settings-rules.md` §9.7 <!-- ap-gap --> |
| Bổ sung §AQ | GAP-AQ1, AQ2 | Dự án mới: `shareGlobal` chỉ model/ngân sách (đã có); thêm `copyRulesFrom` để chép quy tắc + văn phong từ dự án khác (AQ1). Luồng viết: mỗi thao tác cần `op`, `args`, `result`, `role`, `round`, `startedAt`, `durationMs`, `status`, `error`; lịch sử phân trang theo vòng (AQ2). **Không** cần token/chi phí theo thao tác. Bảng tên thẻ ở `20-settings-rules.md` §10 <!-- aq-gap --> |

## Triển khai (25/10 · đợt 1)

Đường dẫn thật thay cho `/api/books/:id` trong bảng GAP: `/api/projects/{p}/stories/{s}/…` (dưới đây viết tắt `S`). JSON camelCase; lỗi `{"error":"<tiếng Việt>"}`.

- **GAP-35.** `GET/POST S/style-samples`, `DELETE S/style-samples/{id}`. POST nhận JSON `{title?, text}` hoặc multipart `file` (`.txt/.md/.markdown`); 400 text rỗng, 409 khi đã 20 mẫu, **404 với người không có quyền viết (cả GET — ẩn tồn tại)**, 404 id không an toàn. `id` = đường dẫn tương đối trong `<truyện>/simulate/`, `title` = tên tệp bỏ đuôi, `chars` = số từ (`domain.WordCount`). Không trả tên thư mục; SSE `story.changed` part `style`.
- **GAP-36.** Review entry thêm `items[{id,level,ref,refName,text,quote,fix}]`, `round`, `pack{id,ten}|null`, `blocking`, và **`previous:[{round,verdict,items[]}]`** (vòng cũ lưu ở `reviews/rounds/NN-r<K>.json`, cũ→mới). `round` = số vòng lưu được, fallback đếm checkpoint `step=review`. `POST S/next` → 409 `{error, items:[…]}` khi vòng mới nhất còn `error`; lỗi lint `Xưng hô` một mình không đặt `blocking`. `GET S/events` thêm `review:{round,items}` cho dòng Biên tập viên (chỉ lịch sử).
- **GAP-37.** `GET/PUT S/frame` → `{audience, difference, sequelSeed, source, audiences[{n,label,wants}]}` (map `nguoi_nghe/diem_khac/hat_giong_phan_hai`); `GET S/retention` → chương + `promiseAt/twistAt` (0..1) + `checks[]`. `audience_groups` là mục mới trong `assets/packs/*/goi.json`. PUT ghi `meta/frame_source.json` (`{"source":"user"}`) — artifact chỉ backend đọc.
- **GAP-38.** `GET/PUT S/address-terms`, `POST S/relations`. `call` ← trường lưu `calls`; `423 {error, writing:{chapter, role}}` (hợp đồng ghi `{chapter}`, thêm `role`); `409 {error, affected:[…]}`; `forward` ghi `effectiveFrom = max(fromChapter, latestWritten+1)`. `address_terms.json` thêm `source:"người dùng"`; `relationship_log.jsonl` thêm `type` (enum `enemy|ally|work|own|secret|rel`, thiếu/cũ → `rel`) và `reason` (tuỳ chọn). `watch.go` thêm nhóm `address`/`brain`; `story.changed` parts `["address","brain","snapshot","chapters"]`.
- **GAP-39.** `GET S/brain`. `entities` hiện có `char|fs|ev|rule`; `place|obj` chưa có nguồn dữ liệu nên không bao giờ xuất hiện. `appears[]` từ `meta/chapter_records/*.json` `facts.characters` (fallback `summaries/NN.json`), tên quần chúng lấy từ `meta/cast_ledger.json`. `edges[].phases[].type` lấy từ `relationship_log.jsonl` (mới), dòng cũ → `rel`.
- **GAP-40.** `POST S/exports {format: txt|epub|narration, from?, to?, volume?, readNumbers?, onExists: fail|replace|keep}` — `overwrite` bị bỏ (gửi lên → 400). Tệp bản đọc: `-narration.txt`; `GET S/exports` `matrix[].narration` là cột riêng; autoexport `formats[]` nhận `narration`.
- **GAP-41.** `GET /api/setup` → `{needed, version?, expiresAt?, attemptsLeft, lockedUntil?}` (`version` chỉ khi `needed`, cùng giá trị `/api/system`); `POST /api/setup/verify {code}` → 200 `{setupToken, expiresAt}` · 400 `{error, attemptsLeft}` · 423 `{error, lockedUntil}` · 410 · 409 `{error, at}`; `POST /api/setup/admin {setupToken, name, username, password}` → 200 `{user, recoveryCode}` + cookie phiên · 400 `{error, field}` · 410 · 409. `GET /api/me/recovery` thêm `mask` (chỉ khi mã chưa dùng).
- **D1.** `GET S/meta` thêm `builds[] = {from, to, version, commit, at}` (hợp đồng ghi `{from,to,version}`): `version` bỏ tiền tố `v` và hậu tố `+commit`; `commit` 7 ký tự (rỗng khi `dev`/thiếu); `at` = `AcceptedAt` chương đầu vùng (rỗng khi thiếu).
- **D6.** `GET S/metrics?chapters=last3|all` → `{chapters:[{n, metrics:[{key,name,value,unit,limit,ok}]}]}`; gói không có `do_do` → `{"chapters":[]}`.
- **D2/D1 phụ trợ.** `GET /api/config/presets` › `packs[]` thêm `desc` (hiện `""`), `hasRules`, `available`, `layers[3]`, `sections[23]`, `metrics[9]`, `overrides{count,keys}`, `voice`; `GET /api/system` thêm `releases:[{version,date,notes}]` (notes = markdown của GitHub, rỗng khi offline).
- **GAP-42…46.** Chưa làm (đợt 3).

### Triển khai (25/10 · đợt 2)

- **GAP-39 mở rộng.** `GET S/brain?upTo=N`: 400 `{"error":"upTo không hợp lệ"}` khi không phải số hoặc `N < 1`; cắt `entities` có `fromChapter > N`, `appears[]`, `timeline[]`, phase có `fromChapter > N`, và bỏ cạnh còn 0 phase hoặc có đầu mút bị ẩn; **không** cắt `chapters[]` (mock web thì cắt — chỉ dùng cho MBrain, đừng dùng `getBrainUpTo` cho thanh chương). Không có `upTo` → giữ nguyên hành vi đợt 1. `entities[]` thêm `aliases: string[]` (luôn mảng) và `timeline: [{chapter, text}]`; nguồn timeline = phase `relationship_log.jsonl` (`"<label> · <tên kia>"`, thêm “Đổi thành ” từ phase thứ hai) + thay đổi `address_terms.json` (`fromChapter > 1`, `"<A> gọi <B> là “<call>”"`); không gộp `summaries`/`timeline.json`.
- **D12 reveal.** `POST /api/reveal {target: "nao-story"|"nao-project"}` → `{ok:true}` (không `path`), chỉ admin, POST chỉ loopback; kho truyện `<output/novel>/nao/`, kho dự án `<dự án>/nao/`; không tự tạo thư mục; 404 `{error:"Chưa có kho ghi chú của truyện"|"…của dự án"}`; mở `obsidian://open?path=<urlencoded>` (`%20` cho khoảng trắng). `GET /api/reveal?target=nao-*` → `{canReveal}` (không path). Chưa có số ghi chú.
- **D11.** `GET /api/config/presets` › `packs[].audio = {che_do: "single"|"cast"|"", giong: "nu"|"nam"|""}` đọc thẳng `goi.json` `audio`; `voice` giữ nguyên, chỉ để hiển thị.
- **D5 lệch còn lại.** PUT `/frame` chỉ phát `story.changed` part `snapshot` — chưa có part `frame`; tab khác chỉ làm mới khi có sự kiện `outline`. GAP-48 sẽ đổi `audience` thành `number[]` và thêm `short`/`desc` (chưa làm).
- **D6 lệch còn lại.** `GET S/metrics` chưa có `severity`/`ref`; mỗi chương 9 dòng (không phải 8).
- **Đợt 3.** GAP-42…46, 47…49; GAP-38 bổ sung (`Xưng hô` mức error ⇒ `items[]` `blocking:true`); `hook.type:"seed"`; trạng thái chương `writing`; `uncertain`/`uncertainNote`; `GET /api/setup` `expired:true` + 400 `{format:true}`; `round.at`; `items[].decision`.

### Triển khai bổ sung (25/10 · câu 20–23)

- **GAP-38 phía giao diện (báo cáo FeCast4).** Sửa/thêm xưng hô và quan hệ gửi `apply:"next"|"mark"` (chọn sẵn `mark`); thay cho tên `forward` trong triển khai đợt 1. Chỉ 409 có `affected` mới mở hộp xác nhận áp dụng. Lý do bắt buộc cho thao tác sửa/thêm; dòng engine thiếu lý do cần người dùng nhập khi sửa. Không suy ra backend đã chuyển hợp đồng chỉ từ báo cáo FE này.
- **Quan hệ mới từ web.** Form không có trường chọn `type`, nên thao tác tạo dùng mặc định `rel`; không suy loại từ nhãn tự nhập. Dòng engine giữ `type` được phục vụ. Chip: `enemy` đỏ, `ally` xanh lá, loại khác/`rel` xanh lam.
- **GAP-39 phía giao diện.** `writing` dùng chấm tím và tooltip “đang viết”; banner khoá đọc `writing.state`. Loại thực thể không có trong payload bị ẩn; mock có `place`/`obj` và renderer hỗ trợ cả hai, nhưng nguồn chapterfacts GAP-49 vẫn cần bằng chứng backend riêng.

### Triển khai (25/10 · đợt 2)

**Chốt 2 + 2b: các shape dưới đây thắng ghi chú đợt 1/đầu đợt 2 ở trên khi khác nhau.** `S` = `/api/projects/{p}/stories/{s}` (không phải `/api/books/:id`). BE dưới `internal/`; FE dưới `web/src/`. GAP-47…49 và bổ sung 32/37/38/41 đã triển khai, không còn chờ đợt 3; GAP-42…46 vẫn ngoài đợt này. Nghiệm thu/câu hỏi còn mở: `19-gap-closure.md` §G.

- **GAP-41 · Câu 1–2.** `GET /api/setup` giữ `expiresAt` cũ, có `expired:true` khi hết hạn; POST `/api/setup/verify` xét 410 trước 423. Mã sai định dạng trả 400 `{error,format:true}` **không tính vào 5 lượt**; mã sai hợp lệ mới mất lượt. FE chặn mã chưa đủ 8 ký tự, dùng expired thay kiểm expiresAt, giữ số lượt khi format; khoá hiện HH:MM/sau 10 phút; taken không footer phiên bản. `webauth/setup.go`, `entry/web/setup.go`, `screens/auth/setup/**`.
- **GAP-48 · Câu 5–7, 12.** `assets/pack.go` + 3 goi.json có `ten_ngan`, `mo_ta`; `GET /api/config/presets` › `packs[]` có `short`, `desc`, `meta`, `params:[{key,text}]` (6 dòng do_do), `layers[].desc`, `overrides:{count,keys,rows:[{key,value}]}`. Master 22 mục/43 error/50 warning đếm từ nguồn, không hardcode; overrides hiện chỉ khi count>0 (3 gói đang 0); cả 3 available/hasRules. `metrics` vẫn còn cho nơi khác, không còn render làm thông số gói. `packLabel` chung: short → thiếu thì bỏ “Gói ”/cắt phần sau dấu phẩy.
- **GAP-37/48 · Câu 17–18.** `GET/PUT S/frame` → `{audience:number[],difference,sequelSeed,source,audiences:[{n,label,wants}]}`. **Trường lưu `nguoi_nghe` cũng `number[]`**, kể cả save_foundation/Kiến trúc sư; gói C mặc định `[2,1]`, một biến thể → một phần tử. Nhãn cũ được chuyển lúc đọc, lần ghi tiếp theo lưu mảng. `meta/frame_source.json` tiếp tục giữ source; FE nhiều nhóm, lưu `[2,1]` đã đo. PUT hiện chỉ có SSE part snapshot, chưa frame; tab hiện tại tự setData.
- **GAP-37 · Câu 15, 19.** `GET S/retention` có `empty:true,checks:[]` khi đại cương chưa khai giữ chân; FE hiện “Chưa lập bản đồ giữ chân”, CTA điền dock `Lập bản đồ giữ chân: `, không tự gửi. Hook outline mới: info/threat/decision/line + seed (cuối); đọc dữ liệu cũ map crisis→threat, mystery→info, choice/desire→decision, emotion→line. **Seed sai vị trí bị bỏ trên đường đọc và giải thích ở kiểm §7**, không trả 400 từ GET. `promiseAt/twistAt` so vùng chưa làm tròn, mẫu số len(outline). Commit chapterfacts giữ taxonomy móc riêng.
- **D6 · Câu 16.** `GET S/metrics?chapters=last3|all` → `{chapters:[{n,metrics:[{key,name,value,unit,limit,ok,detail?,severity?,ref?}],tiredWord?:{word,per1000,limit,ok}}]}`. Chín metric (có ngay_thu), số câu/đoạn/quãng là số lượng vượt chuẩn; `value:null` khi mở đầu không áp dụng ngoài chương 1. Severity/ref từ dòng luật có nhãn, Bộ C §14 override error; tiredWord tách riêng, thêm limit/ok so với mô tả đầu đợt 2. FE rail chỉ lệch, Settings 9 + 1 dòng từ mệt/null “—”; Activity ẩn null, detail ưu tiên thay chuẩn, badge theo severity. `rules/metrics.go`, `entry/web/{metrics_api,metric_refs}.go`, `lib/metrics.ts`.
- **GAP-38 · Câu 20–21.** `GET/PUT S/address-terms`, `POST S/relations`: `reason` bắt buộc cho ghi web → thiếu/rỗng 400 `{error,field:"reason"}` không ghi; engine reason rỗng vẫn đọc được/FE ẩn dòng. `apply` nhận `""|"next"|"mark"` (rỗng mở xác nhận 409 affected khi chạm chương đã viết); **không còn forward, gửi forward→400**. Next giữ chương đã viết, hiệu lực từ chương tiếp theo; mark đưa chương vào hàng chờ sửa; FE mặc định mark. PUT id dòng đã hết hiệu lực → 409, không phải 400; không có affected thì FE hiện lỗi thay mở xác nhận. `relations[]` có type, dòng thiếu/cũ → rel; tạo web không trường chọn type nên rel. Khoá 423 trả `writing:{chapter,role,state:"writing"|"rewriting"}`.
- **GAP-38 bổ sung · Câu 13.** Thay hẳn mô tả đợt 1 “lint Xưng hô một mình không blocking”: nay error đi `commit_chapter` lint/lưu vi phạm → `tools/save_review.go` gộp `RequiresChange` vào review → `store/progress.go` `PendingRewrites`; accept chuyển rewrite; DTO `items[]` `blocking:true`; `POST S/next` 409. Warning không chặn; ưu tiên global/arc giữ nguyên. Tests `tools/save_review_address_test.go` (error queue/warning), `entry/web/review_items_test.go` (`TestReviewAddressLintAloneBlocksApprove`, dedupe, gate/review sạch), FE `workspace/gate.test.ts`; không chỉ chặn nút ở FE.
- **GAP-47 · Câu 14.** `POST S/review/items/{id}/keep {reason}` → 200 `{state:"kept",reason,by,at}`; `DELETE S/review/items/{id}/keep` → **204**. 400 reason thiếu, **404 khi không có quyền ghi (ẩn)**, 404 id lạ, 409 mục Bắt sửa; ID băm ref + chương + trích. `store/review_decisions.go` lưu `reviews/decisions.json`; review trả `items[].decision`; `by` tên hiển thị (fallback username); danh sách mục giữ/lý do vào `tools/novel_context.go` cho chấm sau. **Lệch GAP-47 đề xuất:** giữ `round:number`, timestamp là **flat `entry.at?:RFC3339`**, không round.at; checkpoint đúng artifact làm nguồn, thiếu bỏ trường. FE request hỗ trợ 204; đã bấm lưu/tải lại/bỏ giữ/tải lại.
- **GAP-49 · Câu 22.** ChapterFacts thêm `places[]`/`objects[]`, mỗi mục `{name,desc,firstChapter,with[]}` qua `domain/revision.go`, `chapterfacts/facts.go`: model strict **nullable-required** cả hai khoá, nhận null hoặc []; firstChapter null = chương hiện tại. Validate tên không rỗng/chương không âm/with không rỗng trước pending commit. Store nhận nguyên facts; đọc cũ/facts endpoint chuẩn hoá [] thay null. Tests schema/validate/store/API ở tools/chapterfacts/domain/web; fixtures `host/engine_test.go:235`, `revision/revision_test.go:205` đã khai null, không bỏ ràng buộc schema.
- **GAP-39/49 · nguồn địa điểm/vật.** `GET S/brain` đọc records một lần, tạo place `dd:<slug>`/obj `vat:<slug>`; fromChapter=min(firstChapter, lần xuất hiện đầu), appears gộp, desc đầu tiên không rỗng. With[] nối cạnh rel/“Liên quan”, phase theo chương; gộp cặp với log, tên tra index nhân vật/cast/địa điểm, bỏ tên không biết, không tạo nút giả. Không facts thì không hai loại này, FE ẩn bộ lọc/nhóm rỗng.
- **GAP-39 · Câu 23/M8.** `GET S/brain.chapters[].status` = written/rewriting/writing/unwritten; rewriting thắng khi vào hàng chờ, còn in_progress → writing (kể cả ngoài outline). Thanh chương desktop/mobile chấm tím, tooltip phân biệt đang viết/viết lại; khoá đọc writing.state. Entity aliases[] luôn mảng, timeline[{chapter,text}] từ phase quan hệ + thay đổi xưng hô, không summaries/timeline.json.
- **GAP-39 · giới hạn xem nhanh.** `GET S/brain?upTo=N`: N không nguyên/<1 → 400 `{error:"upTo không hợp lệ"}`; cắt entities/appears/timeline/phases tương lai và cạnh rỗng/mất đầu mút; **chapters[] giữ nguyên** (mock cuối cũng giữ, thay ghi chú mock cắt ở trên), **desc tĩnh không lọc nội dung theo chương**. MReader dùng endpoint này, có sheet vai/quan hệ/xưng hô/“Chỉ hiện…”/“Mở Não truyện” giữ chương đọc; desc còn có thể spoil ch.4 khi đọc ch.2, GAP-39 chỉ Một phần, cần design quyết định ẩn hay nguồn desc theo chương.
- **D12.** `POST /api/reveal {target:"nao-story"|"nao-project"}` → `{ok:true}`, GET `?target=nao-*` → `{canReveal}`, cả hai không path. Chỉ admin, POST loopback; không tự tạo kho; 404 “Chưa có kho ghi chú của truyện/dự án”; Obsidian URI encode `%20`. `entry/web/reveal.go`, FE `settings/advanced/NaoRows.tsx`; không có notes count.
- **D11.** `GET /api/config/presets.packs[].audio:{che_do:"single"|"cast"|"",giong:"nu"|"nam"|""}` lấy đúng goi.json. `voice` chỉ hiển thị; FE `packVoice.ts` không parse câu voice/không đoán giới tính từ nhóm nghe. Gói single khoá phân vai ở Transcript/AudioExport/MAudio; giong rỗng dùng “một giọng kể”. Chưa có id giọng gói/ép mode của cast cũ ở BE được chứng minh.
- **GAP-32 · Câu 26.** Items API: `items[].uncertain:boolean`, `uncertainNote:string` (không phải mảng). Model có `is_uncertain`/`uncertain_note` nullable-required, cờ/note đi qua cache/store. `/api/rules/understand` và runtime[] giữ `uncertain:string[]` lý do cũ, thêm uncertainNote; FE quy về boolean/note cho khung cam. Chưa cờ giữ ready; mẫu văn phong mới nhất đứng đầu. AudioExport chưa có khối mốc chương/timestamp audio trong contract, chưa thể coi 26b đã xong chỉ vì font đã rõ.
- **Câu 3–4, 8–12, 24–25 phụ trợ.** Onboard formats dùng narration; UI bỏ main; exports giữ -narration.txt ở đĩa nhưng hiển thị “… · Tập N · bản đọc.txt”/badge TTS; command spec/tests có format= và doc-so; bỏ loại “Chưa rõ ai nói”; Esc giữ nháp/gõ thêm mở lại; recovery thiếu mask che toàn bộ. Câu 12 tên ngắn đã đi qua cả review/gate/audio/applied.
- **Gate 2b do integrator báo, 25/10 ~23:10.** tsc sạch; vitest 113 tệp/1028 test; npm build, go build, go vet đạt; gofmt chỉ `panels_outline.go`; full `go test -p 4 -count=1 ./...` **exit 0 lúc ~23:40**. Giữ ghi nhận hai lỗi tải nặng của lượt trước ở `19-gap-closure.md` §G; fixture `revision/revision_test.go:205` places/objects do integrator sửa. Không chạy kiểm tra/code mới trong lượt cập nhật docs.

### Triển khai (25/10 · đợt 3)

**GAP-42…46 đã triển khai; shape dưới đây thắng đề xuất trong bảng GAP khi khác.** `S` = `/api/projects/{p}/stories/{s}`, `P` = `/api/projects/{p}`. Đường dẫn đã đối chiếu `internal/entry/web/{quality_api,ledger_api,universe_api,search_api,lessons_api,projects,storyread,cocreate_project,auth}.go`. Nghiệm thu/câu hỏi: `19-gap-closure.md` §H.

- **GAP-42.** `GET S/quality` (gateway đọc thẳng, không cần worker) → `{pack, sections[{ref,refName}], chapters[{n,title,status}], cells[{ref,ch,error,warning,fixed,kept}], issues[ReviewItem + {ch,state:"open"|"fixed"|"kept",keptReason?}]}`; mảng rỗng là `[]`, không null. **`status` = `approved|rewritten|rewriting|previous`** (thêm `approved`): đang viết lại → `rewriting`; nằm hàng chờ chưa viết → `previous`; xong + vòng trước có Bắt sửa → `rewritten`, còn lại `approved`; chương chỉ có trong đại cương bị bỏ. `sections` theo thứ tự gói (Master trước, rồi mục gói; Xưng hô/Quy tắc của bạn/ngoài gói theo lần gặp). Mục đã giữ không tính open; gộp trùng chương/global. Truyện không đọc được → **404** (giấu sự tồn tại, không 403). `StorySummary.openErrors` (danh sách truyện) = số mục Bắt sửa mở duy nhất.
- **GAP-43.** `GET P/ledger` → `{axes[10]{n,name}, stories[{id,title,values[10],day7,opening}], channelMedianDay7:null, candidate:{story,title,brief,values,checks,cells}|null}`; chỉ truyện `user.CanRead`; `day7` luôn `null` (không có nguồn số người nghe). `POST P/ledger/check {brief, story?}` → `{axes,values,checks[{level:"error"|"warning"|"ok",rule,text,axes[],against?}],cells[{story,axis,mark:"dup"|"overused"}]}`, **luôn đủ 4 phép kiểm** (`combination` 1+3+4+6, `distance` ≥ 4 trục với truyện liền trước, `overuse` chỉ trục 3/4/7 — `he-thong-san-xuat-lien-tuc.md` dòng 33, `opening` trục 9), kể cả `level:"ok"`; ô của ứng viên dùng `story:"new"`. `POST P/ledger/suggest {brief, story?}` → `{values,changed[],brief}`. Cả hai cần quyền quản lý dự án (403), `brief` rỗng 400, `story` không đọc được 404, chưa cấu hình model 409 `{error,needsConfig:true}`, Kiến trúc sư lỗi 502. **Cửa bắt đầu:** `POST S/input` lần đầu (truyện chưa có phase/tên) và `POST P/cocreate/commit {name?, brief?}` → 409 `{error,checks,axes,values,cells}` khi có Bắt sửa; chỉ Cảnh báo thì đi tiếp; chưa có truyện cũ đủ 10 trục thì không kiểm. `brief` ở commit (bản đã viết lại) thay bản nháp đồng sáng tác. Kết quả lưu ở `meta/ledger.json` của truyện.
- **GAP-44.** `GET P/universe` → `{name, counts{stories,chars,places}, entities[{id,type:"char"|"place",name,stories[{id,title}],lastRole}], checks:null}`. `name` = tên dự án (`""` với dự án ẩn danh); chỉ thực thể có ở ≥ 2 truyện xem được; gộp theo tên/biệt danh (thường hoá, NFC, gộp khoảng trắng, **giữ dấu**); `stories[]` là object, mới trước; `counts.stories` = số truyện có ít nhất một thực thể chung. **`checks` = `null`** tới khi Master §19 có nguồn chấm (đến 26/10; nay xem “Triển khai (26/10 · §H)”); `[]` để dành cho lần chấm thật không lỗi.
- **GAP-45.** `GET P/search?q=` → `{entities[cùng shape GAP-44, type char|place|obj], passages[{story,storyTitle,ch,quote,hl[[s,e]]}], total}`. So khớp bỏ dấu/hoa thường/khoảng trắng thừa trên văn bản chương đã xong, theo đoạn (bỏ tiêu đề, dòng ngắt cảnh); `quote` ≤ 240 ký tự quanh chỗ khớp đầu, cắt có “…”. **`hl` là offset UTF-16 trong `quote`** (dùng thẳng `String.slice`). Xếp: khớp đúng dấu → nhiều chỗ khớp → truyện mới → thứ tự đọc. Tối đa 50 đoạn (`total` = số trước khi cắt) và 50 thực thể. `q` rỗng/chỉ khoảng trắng hoặc > 200 ký tự → 400; dự án không có/không đọc được → 404. Chỉ truyện `user.CanRead`.
- **GAP-46 (chỉ admin; non-admin 403 cho mọi route).**
  - `GET /api/system/lessons?state=pending|kept|dropped` (mặc định `pending`, sai → 400) → `{lessons[{id,origin,refs[],text,evidence{count,kind,stories,pack,packsCount,chips[]},scope,packId,packName,state,createdAt,decidedAt?}], counts{pending,kept,dropped}, stats{projects,stories,rounds}, packs[{id,name,stories}]}` (object, không phải mảng như đề xuất).
  - `POST /api/system/lessons/{id} {action:"keep"|"drop"|"edit"|"undo", text?, scope?:"pack"|"all"}` → bài học sau khi đổi; 404 id lạ, 400 thao tác sai trạng thái/`edit` chứa tên nhân vật, biệt danh hay đoạn truyện. `keep` **bỏ qua `text`** (sửa chữ = `edit` rồi `keep`). Phương thức khác → 405.
  - `GET /api/system/violations?pack=` (nhận id gói hoặc `A|B|C`; sai → 400) → `{rows[{ref,refName,stories,total,level,alert}], preamble, packName, stories}`.
  - Đề xuất khi cùng gói + mục luật + mức có ở **≥ 2 truyện**; `alert` khi Bắt sửa ở **≥ 3 truyện**; gộp vòng hiện tại + vòng cũ; chữ bài học sinh từ mục luật/tiêu đề, không lấy trích. **`origin:"user"` chưa sinh** (không có mẫu sửa có cấu trúc). Catalog `~/.ainovel/rules/lessons.json` (bài đã giữ là quy tắc lớp “Mọi truyện” hoặc lớp gói); truyện đã bắt đầu đóng băng ở `meta/lesson_rules.json`; Kiến trúc sư nhận bài học + cùng `preamble`.
  - `GET /api/me` thêm `pendingLessons` (số bài chờ duyệt) **chỉ khi là admin**.

### Triển khai (26/10 · §H)

**Trả lời 20 câu §H (`22-brief-2510.md` › *Trả lời 20 câu*).** Đối chiếu `internal/entry/web/{ledger_api,universe_api,projects,review_items,quality_api,lessons_api}.go`, `internal/ledger/ledger.go`, `internal/lessons/lessons.go`, `internal/store/rewrites.go`, `internal/library/library.go`. `S`/`P` như trên; mục này thắng các dòng đợt 3 khi khác.

- **`checks[].short` (câu 1–2).** `ledger.Check` thêm trường `Short` (JSON `short`) — câu một dòng theo mẫu từng phép kiểm, luôn có ở cả `level:"ok"`; overuse gộp mọi trục dùng quá vào **một** hàng `axes:[…]`. Có ở mọi payload: `GET P/ledger.candidate.checks`, `POST P/ledger/check`, 409 của `POST S/input` lần đầu, 409 của `POST P/cocreate/commit`, và kết quả cửa tool `save_foundation`. Bản ghi `meta/ledger.json` cũ trả `short:""` → FE mới cắt “…” + `title`; bản ghi mới luôn có câu. Vẫn đúng 4 phép kiểm theo thứ tự `combination, distance, overuse, opening`; “5 truyện gần nhất ≥ 2 trục” không còn.
- **`project.universeName` (câu 3).** `project.json` thêm `universeName` (tuỳ chọn, `omitempty`, `library.ProjectFile.UniverseName`). `PUT P {name?, universeName?, …}` nhận chuỗi (trim; rỗng = xoá, quay về tên dự án) — cùng ngữ nghĩa thay-toàn-phần như `name`. Trả `universeName: string` (luôn có, `""` khi chưa đặt) trong `GET P` (`.project`), phản hồi PUT, và tóm tắt ở `GET /api/projects` / `GET /api/library` (`projects[]`). `GET P/universe.name` = `universeName` nếu có, không thì tên dự án; dự án ẩn danh (`_`) hoặc `project.json` không có tên → `""`.
- **Review `rule` + `rule_aware` (câu 4).** Mục thẩm duyệt thêm `rule?: string` (`omitempty`) — mã con ổn định, hiện chỉ có `s19-recap` (Master §19: nhắc chuyện cũ quá 2 câu hoặc không lồng vào hành động). Vòng chấm sinh bởi schema `save_review` mới mang `rule_aware: boolean` trên entry **và** từng `previous[]` (DTO luôn có, không `omitempty`). Thiếu `rule` ở vòng `rule_aware` = đã chấm và sạch; vòng không `rule_aware` = chưa chấm được. Quality `issues[]` thừa hưởng `rule` qua `ReviewItem`.
- **`meta/rewrites.jsonl` + `chapters[].rewrites` (câu 9).** Log chỉ thêm, mỗi dòng `{chapter, at, cause:"review"|"user"}`, ghi khi chương **mới** bước vào hàng viết lại (so hàng chờ trước/sau, không trùng; vào lại sau khi viết xong thì ghi thêm). `cause=review` từ kết quả thẩm duyệt; `cause=user` từ góp ý đã duyệt, xưng hô/quan hệ `apply:"mark"`, và `reopen_book`. `GET S/quality` thêm `chapters[].rewrites[{at,cause}]` — **luôn mảng**, cũ→mới, `[]` khi không có nguồn; `status:"rewritten"` chỉ khi log khác rỗng (không còn suy từ vòng chấm cũ). Brain không thêm trường này.
- **Bài học: nhóm + `evidence.pack`/`packsCount` (câu 6, 15).** Nhóm theo `(packID, ref)` khi ref khớp `^Bộ\s+\S+\s*§`; mọi ref khác (`Master §…`, `Định dạng §…`, `Xưng hô`, `Quy tắc của bạn`) nhóm theo `ref` qua mọi gói. Mức không tách nhóm: `evidence.count` đếm cả Bắt sửa + Cảnh báo, `evidence.kind` = mức cao nhất; đề xuất ≥ 2 truyện, `alert` (chuông) = Bắt sửa ở ≥ 3 truyện. `evidence.packsCount` = số gói trong nhóm, `evidence.pack` = tên gói khi `packsCount == 1`, `""` khi ≥ 2 (mục `Bộ …` luôn có tên gói; đổi 26/10: mục dùng chung **luôn** `pack:""`, xem “Triển khai (26/10 · §I)”); `scope` mặc định `all` cho mục dùng chung. ID = `lesson-` + 12 byte `sha256(groupKey)`; **không có migration** từ id tách mức cũ — id lạ nằm im trong `lessons.json` (đổi 26/10: đã có migration `schema:2`, xem “Triển khai (26/10 · §I)”). Không có `dropReason`; ref `Xưng hô` lấy tên `naolint.IssueName`. `/api/system/violations` dẫn từ cùng cách nhóm nhưng số/level/chuông vẫn **theo gói**.
- **`checks` của vũ trụ (câu 4).** `GET P/universe.checks` = `[{ref:"Master §19", story, ch, text, quoteId}]` gom từ mọi truyện dùng chung ít nhất một thực thể, đọc cả bài hiện tại (`reviews/NN[-global].json`) lẫn `reviews/rounds/`, chỉ mục có `rule:"s19-recap"`, khử trùng theo `(truyện, item.id)`; `quoteId` là mã mục GAP-47 (Reader `?quote=`). `null` khi chưa truyện dùng chung nào có vòng `rule_aware`; `[]` khi đã chấm mà sạch. Truyện không đọc được không góp gì. Mục đã “Giữ nguyên” (GAP-47) hiện **vẫn** nằm trong `checks` (đổi 26/10: đã lọc sang `kept` + `keptCount` — xem “Triển khai (26/10 · §I)”).

### Triển khai (26/10 · §I)

**Trả lời 11 câu §I (`22-brief-2510.md` › *Trả lời 11 câu*).** Đối chiếu `internal/{ledger/ledger.go,entry/web/{config.go,packdetail.go,universe_api.go,lessons_api.go},lessons/{lessons.go,store.go,migrate.go},assets/pack.go}`. Mục này thắng các dòng §H khi khác; `S`/`P` như trên.

- **Độ dài chương (câu 1).** `assets/packs/*/goi.json` thêm `"tieng_moi_chuong": {"min":1400,"max":1600}` (đơn vị tiếng/chương, tách khỏi `thoai` là %). Loader `assets.PackTiengMoiChuong` + `Pack.TiengMoiChuong` (`*`, khai báo theo manifest `tieng_moi_chuong`). `GET /api/config/presets` → `packs[].chapterWords:{min:number,max:number}|null` — **key luôn có**, `null` khi gói không khai (gói người dùng); `params[]` và `meta` không đổi. Drift test `TestPackChapterWordsMatchRules` đọc dòng “| Độ dài |” trong `luat/` của chính gói.
- **`checks[].fix` (câu 3).** `ledger.Check.Fix *string` (`json:"fix"`) — luôn phát, `null` khi phép kiểm đạt hoặc artboard không vẽ gợi ý: `combination` mức error → `"Đổi ít nhất 1 trong 4 trục"`, `overuse` mức warning → `"Cân nhắc đổi"`, `distance`/`opening` và mọi hàng đạt → `null`. Đi cùng `short` trên mọi payload (`GET P/ledger.candidate.checks`, `POST P/ledger/check`, hai cửa 409, kết quả tool `save_foundation`). FE vẽ căn phải cuối hàng, chỉ khi khác `null`.
- **Vũ trụ: mục đã giữ (câu 9).** `GET P/universe` thêm `keptCount:number` và `kept:[{ref,story,ch,text,quoteId}]` (**luôn mảng**, `[]` khi không có). Mục có `decision.state == "kept"` (GAP-47, đọc qua bộ chuyển mục thẩm duyệt) **không** vào `checks` mà sang `kept`; `checks` vẫn `null` (chưa chấm) / `[]` (đã chấm sạch), khử trùng `(truyện, item.id)`, truyện không đọc được không góp mục nào.
- **Chuông liên gói (câu 10).** Trong `GET /api/system/violations?pack=`, hàng **mục dùng chung** (`Master §…`, `Định dạng §…`, `Xưng hô`, `Quy tắc của bạn`) có `alertScope:"all"` và `alertStories:number` — số **truyện có Bắt sửa trên mọi gói**, luôn phát kể cả `0`; `alert` = `alertStories >= 3`. Hàng `Bộ …` không có hai khoá này và giữ chuông theo gói. `stories`/`total`/`level` vẫn theo gói. `preamble` đọc `alert` của hàng nên mỗi gói dùng chung bộ chuông liên gói với số của gói mình.
- **Nguồn bài học (câu 4, đổi §H).** `evidence.pack` **chỉ** mang tên gói cho mục `Bộ … §…`; mọi mục dùng chung luôn `pack:""` với `evidence.packsCount` thật (kể cả `1`) → dòng nguồn luôn “· N gói”. `scope` mặc định `all` cho mục dùng chung, `pack` cho mục `Bộ`.
- **`lessons.json` `schema: 2` (câu 11).** `internal/lessons/migrate.go`: `schemaVersion = 2`, `legacyID` (chỉ trong tệp này) = `lesson-` + 12 byte `sha256(packID\x00ref\x00level)`. Mỗi mục cũ được đưa về `ID(GroupKey(packID, ref))`; trùng id thì gộp `kept > dropped > pending` (giữ `createdAt` sớm nhất, `decidedAt` của bên thắng); id không khớp cả khoá mới lẫn khoá cũ bị bỏ; mục dùng chung còn `pending` được đưa về `scope:"all"` (đã quyết giữ phạm vi), `Bộ …` không chạm; ghi lại kèm `"schema": 2`. Chạy trong `transaction()` dưới khoá `lessons.lock`, và `read()` migrate xong mới trả dữ liệu; tệp đã `schema >= 2` không bị đọc-ghi lại (lần hai byte-identical). `evidence` không bị viết lại lúc migrate — làm mới ở `Sync` kế tiếp.

### Triển khai (26/10 · §J)

**Trả lời 4 câu §J (`22-brief-2510.md` › *Trả lời 4 câu của dev*) + dòng “Bổ sung §J” ở trên.** Đối chiếu `internal/entry/web/{universe_api.go,review_keep.go,storyread.go}`, `internal/store/review_decisions.go`, `internal/lessons/lessons.go`. `S` = `/api/projects/{p}/stories/{s}`, `P` = `/api/projects/{p}`; mục này thắng các dòng §I khi khác.

- **`kept[]` mang quyết định giữ (câu 1).** `GET P/universe` → `kept:[{ref,story,ch,text,quoteId,reason,decidedBy,decidedAt}]`. `universeKeptCheck` nhúng `universeCheck` (JSON phẳng) nên `checks[]` giữ nguyên hình dạng §H/§I, chỉ `kept[]` thêm 3 khoá; **cả 3 luôn phát, không `omitempty`**, `""` với bản ghi cũ thiếu trường — không suy ra, không lấy tên người gọi hiện tại. Nguồn: `reviews/decisions.json` → `store.ReviewDecision{state,reason,by,at,ref,refName,text,quote}`, chỉ `serveReviewKeep` ghi; `by` = tên hiển thị người giữ, `at` = RFC3339. GAP-47 đã bắt buộc `reason` (400 `Cần ghi lý do giữ nguyên`) nên chỉ bản ghi trước §J mới rỗng. Không đổi store/handler.
- **Bỏ giữ (câu 1).** `DELETE S/review/items/{itemId}/keep` → **204** không thân; xoá quyết định khỏi `reviews/decisions.json` rồi phát `review_changed`; lần `GET P/universe` sau mục trở lại `checks` và `keptCount` giảm. Lỗi: **404 khi không có quyền ghi truyện (ẩn)**, 404 khi `itemId` sai dạng (không khớp `^ch(\d+)-`) hoặc không tìm thấy mục; method khác trả `false` để caller chuyển tiếp (không 405). `{itemId}` **chính là `quoteId`** của DTO; `{s}` là mã truyện trần (không phải `proj/…`). Không có thân, không trả mục nào.
- **`Preamble` (câu 4).** `internal/lessons.Preamble(rows, packName)`: với mỗi hàng `alert`, mặc định `<stories>/<total> truyện`; khi `alertScope == "all"` **và** `*alertStories != stories` thì `<alertStories> truyện ở mọi gói`. Không đổi hình dạng câu (`Truyện <gói> trước đây hay vấp <mục> (<tên mục>, <số>), …`), hàng `Bộ …` luôn dạng n/tổng. Mock `web/mock/lessons.ts` phản chiếu đúng luật này.

## Gói AP — quy tắc hai cấp

- **AP1.** `/api/rules`, `/api/rules/file`, `/api/rules/file/delete`, `/api/rules/understand` và `/api/rules/effective` từ chối `scope=global` bằng **400** `{"error":"Quy tắc viết chỉ áp dụng cho dự án hoặc truyện, không còn cấp toàn cục"}`. Chuỗi `chain` và các `items[].scope` chỉ còn `project` → `story`; `quy-tac.md` vẫn cộng dồn hai cấp. `GET /api/rules/effective?scope=story&project={p}&story={s}` cùng phản hồi với `GET S/rules/effective`; cấp dự án không có `runtime`/`timing`.
- Lần đầu khởi động sau nâng cấp, tệp global thật trên máy chủ `~/.ainovel/rules/*.md` được chuyển vào **từng dự án**: file mode ghi `<dự án>/.ainovel/rules/`; hosted ghi `project_documents(kind=rules)`. Tệp thường cùng tên giữ bản dự án (quy tắc gần thắng); `quy-tac.md` thêm các dòng global chưa có vào trước các dòng dự án. Truyện cũ thuộc dự án ẩn danh được nhận quy tắc tại cấp truyện do không có cấp dự án riêng.
- Marker `rules.project-only.v1`: file mode `<thư viện>/.ainovel/rules.project-only.v1.done`; hosted `noverse_meta[key=rules.project-only.v1]`. Ghi nhật ký quản trị `type=perm`, `actor=null`, `action="Chuyển quy tắc toàn cục vào từng dự án"`; chạy lại không thêm quy tắc hay audit. Hosted chép/audit/marker trong cùng giao dịch. Nguồn không đọc được hoặc audit không ghi được thì không đánh dấu xong và không phục vụ thư viện trước khi chuyển thành công.
- Chỉ **quy tắc và văn phong** bỏ cấp global: `/api/config?scope=global` không trả `style` trong `target.config`, `chain[].config`, `effective`, `sources`; ghi `style` ở cấp global trả 400. Chuỗi cấu hình dự án/truyện không thừa hưởng `style` global; chưa chọn dùng gói “Chung” tích hợp của Noverse. Provider, model, khoá truy cập và các cài đặt hệ thống vẫn giữ cấp global như cũ.

- Bài học **Não hệ thống** đã duyệt với `scope=all` không phải quy tắc global do người dùng viết: vẫn được chốt cho truyện mới như mặc định Noverse, nguồn **`default:lessons`** (nguồn cũ `global:lessons` được đổi nhãn khi đọc, không đổi nội dung). Không sinh mục/nhóm “Mọi truyện” trong phản hồi hiệu lực. Bài học riêng gói giữ nguồn `pack:*`; truyện cũ không tự nhận các bài học duyệt sau khi bắt đầu.
- Dự án **tạo mới sau nâng cấp** bắt đầu với quy tắc trống, không chép lại các tệp global đã nghỉ dùng. `shareGlobal` chỉ còn chọn kế thừa/chốt cấu hình trợ lý AI; không kéo theo quy tắc hoặc văn phong global.

### AP2 — ngưỡng chống giọng văn máy

`GET S/rules/effective` và `GET /api/rules/effective?scope=project|story&project={p}&story={s}` thêm:

```ts
fatigueWords: Array<{
  word: string;
  per1000: number;       // nguồn hiệu lực, tính theo 1.000 tiếng
  perChapter: number;    // round(per1000 * 1.5), chương tham chiếu 1.500 tiếng
  source?: "default" | "pack" | "project" | "story" | "learned";
  basePerChapter?: number; // ngưỡng ↺ nếu bỏ ghi đè ở cấp đang sửa (kể cả 0)
  baseSource?: "default" | "pack" | "project";
}>;
forbiddenDefaults: string[]; // bốn cụm cấm mặc định Noverse, không sửa/xoá
```

Mảng luôn chứa đủ 16 từ/cụm nhạy mặc định, cộng từ riêng hiệu lực nếu có; ngưỡng **0** là cấm xuất hiện, không phải “chưa đặt”. Lưu/apply vẫn qua quy tắc chữ tự do và bộ chuẩn hoá hiện có, không có kho cài đặt riêng. Xoá ghi đè trả về ngưỡng gói/mặc định. Thứ tự ưu tiên số liệu cùng checker: mặc định → gói → dự án → truyện → điều chỉnh đã học. Nguồn được lưu theo từng từ tại `snapshot.fatigue_sources`, kể cả ghi đè có giá trị bằng mặc định.

**Tương thích dữ liệu cũ:** snapshot trước AP không lưu nguồn thắng theo từng từ. Chỉ suy nguồn nếu các nguồn đã chốt chứng minh được; nếu mơ hồ thì **không có `source`**, UI không hiện nhãn nguồn. Số liệu vẫn đúng; lần Apply kế tiếp sẽ ghi đủ provenance. Không đoán từ các tệp hiện tại vì chúng có thể đã sửa sau lần Apply cũ.

Mốc ↺ cấp dự án gồm mặc định + gói; cấp truyện gồm mặc định + gói + quy tắc dự án, bỏ qua lời dặn/ghi đè truyện và điều chỉnh đã học. UI dùng `basePerChapter`, không đoán mốc gói từ mặc định Noverse khi từ đang có ghi đè riêng.

### AP4/AP5/AP6 — nền truyện và tuyến còn mở

| API / khóa | Kiểu và nguồn |
|---|---|
| `GET S/premise` | `{sections: [{heading: string, text: string}], missing: string[], updatedAt: string \| null}`. Tách bằng cùng `parsePremiseSections` của engine, thiếu mục tính bằng `requiredPremiseHeadings(planningTier)`. `text` giữ toàn bộ nội dung đã lưu, không tóm tắt hay sinh mục trống. Giữ cả mục `##` riêng; mục chuẩn theo thứ tự mẫu, mục riêng theo tên. `updatedAt` là RFC3339 của mtime `premise.md` ở file mode hoặc `outline_documents.updated_at` ở PG; chưa có tệp là `null`. |
| `GET S/world-rules` | `Array<{category: string, rule: string, boundary: string}>` từ `world_rules.json`, giữ đủ trường. Category gốc: `magic`, `technology`, `geography`, `society`, `other`; UI dùng bản dịch tương ứng. Chưa có dữ liệu trả `[]`. |
| `snapshot.openThreads` | `string[]` từ compass `open_threads`, mặc định `[]`, không suy từ số chương. |
| `snapshot.planningTier` | `"short" \| "mid" \| "long" \| ""` từ `run.json.planning_tier`; dữ liệu cũ chưa có giữ `""`, không đoán bậc theo số chương. |
| `GET S/brain` → rule entity `category` | Trường phân loại quy tắc thế giới; entity khác không thay đổi. |

Hosted: hai API nền truyện đọc kho fact PG theo quyền dự án trước nhánh proxy; snapshot đi qua gateway → worker như cũ và có đủ hai khóa mới. File mode dùng cùng DTO và parser; lỗi đọc/giải mã là lỗi API, không giả thành dữ liệu rỗng. Hai API đọc bổ sung không thay đổi công cụ ghi nền truyện hiện có.


## Sao chép quy tắc và lịch sử luồng viết

- `POST /api/projects` nhận `copyRulesFrom?: string` là slug dự án có quyền đọc. Sao chép một lần các tệp quy tắc dự án và `config.style`, không liên kết hai dự án. `shareGlobal` vẫn chỉ điều khiển cấu hình trợ lý. `GET /api/library` và `GET /api/projects` trả mỗi dự án thêm `ruleCount: number` và `style: string` (rỗng là mặc định). `ruleCount` đếm từng dòng quy tắc trong `quy-tac.md`, cộng một cho mỗi tệp quy tắc khác có nội dung; tệp trống không tính.
- `GET S/live-history?beforeRound=&limit=20` trả `{rounds, nextBeforeRound}`; `beforeRound` là mốc loại trừ, mặc định tải 20 vòng mới nhất, từng trang sắp theo vòng tăng dần, `nextBeforeRound: number | null`.
- Mỗi vòng: `{round: number, roundId: string, role: string, title?: string, startedAt: string, operations: []}`; `roundId` là id sự kiện điều phối của vòng (live SSE gửi kèm). `title` chỉ có khi dữ kiện điều phối có cấu trúc đủ xác định; vòng cũ không có thì bỏ qua.
- Mỗi thao tác: `{id, role, round, roundId?, startedAt, durationMs, status, error, key, params, result}`; `status` là `done | running | error`, `error` là câu tiếng Việt an toàn được dựng từ loại lỗi và ngữ cảnh số chương/cung/tập, không chép lỗi gốc hay khoá, bí mật, đường dẫn. Lỗi không phân loại dùng câu chung; `params` và `result` giữ dữ liệu thô trong chi tiết kỹ thuật. Lịch sử dùng hàng đợi runtime bền sẵn có, không thêm kho thứ hai.
- SSE `event` loại DISPATCH/TOOL trong Luồng viết mang `id` (id thao tác; DISPATCH trùng `roundId`), `agent`, `role`, `roundId`, `roundStartedAt`, `roundTitle?` để gộp live vào đúng vòng/thẻ; TOOL lỗi có thêm `error` an toàn và lưu bền cùng sự kiện. Các sự kiện khác không bắt buộc có các trường này.
