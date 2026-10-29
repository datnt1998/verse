---
title: "Dựng lại Viderse từ yt-factory theo design v1"
description: "Port logic yt-factory sang Go + Postgres + React trong apps/viderse. Giao diện, luồng và trạng thái làm đúng bộ design v1; lõi chạy một process, đẩy sự kiện qua SSE, phục hồi được khi sập."
status: pending
priority: P1
effort: 460h
branch: "main của datnt1998/verse (Actions đang tắt)"
tags: [feature, backend, frontend, database, api, viderse]
blockedBy: []
blocks: []
created: 2026-10-29
---

# Kết quả cần đạt

Viderse trong `apps/viderse` thay hẳn yt-factory trong việc vận hành nhiều kênh YouTube. Một video đi theo chuỗi 9 bước máy, trình bày thành 4 trợ lý:

- Trinh sát: `download`, `backdrop`, `transcript`.
- Đạo diễn: `metadata`, `scenes`.
- Hoạ sĩ: `images`.
- Kiểm định: `build`, `score`, `export`.

Chuỗi này giữ đúng logic, cổng chất lượng (≥ 90) và cách phục hồi của yt-factory. Giao diện, luồng, trạng thái và câu chữ làm đúng bộ design `apps/viderse/docs/design/v1/`.

- **Nguồn logic** là `/Users/dat/Personal/yt-factory`, commit `2c201e6`. Bản TS `/Users/dat/Projects/viderse` không phải nguồn.
- **Nguồn hành vi và giao diện** là `apps/viderse/docs/design/v1/`. Khi các nguồn lệch nhau thì ưu tiên theo thứ tự: `01-ia-flows.md`, rồi artboard, rồi các tài liệu còn lại (README design › Cách làm việc).
- Chỗ nào code buộc phải làm khác design thì ghi vào `11-hoi-dap.md` trước khi làm. Gói nào xong thì ghi vào `10-gap-closure.md`.

# Quyết định đã chốt (anh, 2026-10-29)

| Câu hỏi | Chốt |
|---|---|
| Dữ liệu yt-factory | **Bắt đầu trắng.** Không viết bộ nhập. Nhóm "Bản cũ" và danh sách job cũ ở Cài đặt › Lưu trữ (design 09 §3) không cần làm; ghi hỏi đáp để design gỡ. |
| Writer hiện có trong verse | **Gỡ bỏ** code, bảng `agent_runs`/`scripts`, `/api/runs`, `/api/scripts` và prompt. Mục "Writer vượt độ dài" đóng theo. |
| D1, D2, D5–D8 | **Theo đúng đề xuất của design.** D1: nghe trên LAN, ghép thiết bị bằng QR. D2: 10 intent, luôn hỏi xác nhận. D5: khoá YouTube Data API tuỳ chọn, không có thì đọc trang công khai. D6: ngưỡng 85; Tự duyệt hết chỉ qua Tuỳ chỉnh và có xác nhận. D7: 50 ảnh trả phí mỗi ngày cho mỗi nhà cung cấp. D8: lưu trên máy, gửi kèm khi Báo lỗi. |
| Cách chạy | **Web + launchd như Noverse**, bỏ Electron. Thông báo hệ thống dùng Notification API cùng Badging API của trình duyệt (cài dạng PWA). |

# Dữ kiện đã kiểm (2026-10-29)

| Dữ kiện | Nguồn |
|---|---|
| Phần server của yt-factory có khoảng 41k dòng prod, chưa tính test. Lớn nhất: `server/clone` 20.1k (test 15.0k), `server/tools` 5.0k, `server/editor` 4.8k, `server/channels` 3.9k, `lib` 2.7k, `server/templates` 2.2k | `wc -l` trên `git ls-files` |
| Engine editor `src/editor/engine` 159k dòng prod (Freecut và OpenReel, MIT); `src/editor/ui` 8.1k | như trên, kèm `LICENSE-freecut` và `LICENSE-openreel` |
| UI cũ: `src/components` 14.7k, `src/app` 9.2k. Có khoảng 77 method HTTP dưới `src/app/api`. Tiến độ chỉ cập nhật bằng poll, không có SSE | scout frontend |
| 9 bước theo thứ tự: download, backdrop, transcript, metadata, scenes, images, build, score, export. Cổng xuất: `CLONE_EXPORT_MIN_SCORE = 90`, không có blocker, không có mục `unknown` | `src/lib/clone-steps.ts`, `src/lib/clone-quality.ts:43,156-158` |
| Mỗi job chạy một process Node. Lease và fence nằm trong SQLite. Mặc định `jobConcurrency` 2, `imageConcurrency` 3, `rescanHours` 1 | `runner.ts`, `launch.ts`, `clone-settings.ts` |
| Chữ (metadata) đi qua Codex CLI ở chế độ cô lập. Ảnh dùng `gpt-image-2` qua Codex CLI, dự phòng `grok-imagine-image` | `metadata.ts`, `image-prompt.ts:5`, `tools/grok.ts` |
| Transcript: sidecar Python 3.11 với faster-whisper 1.2.1, quản lý bằng `uv` | `scripts/clone-transcribe.py`, `tools/transcription.ts` |
| Xuất video: Chrome headless mở trang `/render`, chạy engine editor có GPU. Sau đó ffmpeg nén libx264 CRF 28. Độ lớn âm thanh đo bằng PCM gửi lên server rồi chạy ebur128 | `export-launcher.ts`, `export-compress.ts:87-91`, `measurements.ts` |
| Đọc kênh YouTube qua InnerTube (`youtubei.js`), không dùng Data API. Fixtures có ở `src/server/youtube/fixtures` | scout backend |
| Thiết kế V0–V24 chưa có gói nào được làm ở yt-factory | `10-gap-closure.md` trống |
| Lõi của verse đã có: LLM chữ có structured output và retry, OAuth Codex. Chưa có tạo ảnh và TTS | `core/llm`, `core/llmcontract`, `core/auth` |
| Noverse đã có hub SSE: ring 500 sự kiện, nối lại bằng `Last-Event-ID`. Phía web có `data/events.ts` (363 dòng) và `data/cache.ts` (339 dòng) | `apps/noverse/internal/entry/web/events.go` |
| Máy anh đã có sẵn: ffmpeg, ffprobe, yt-dlp, uv, codex, python3.11, psql | `command -v` |

# Kiến trúc đích

```mermaid
flowchart LR
  subgraph web["apps/viderse/web (Vite, React 19, @noverse/ui)"]
    SH["shell: AppHeader · MainNav · JobsPill · Activity"]
    SC["screens theo route design 02 §4"]
    ED["editor + render (chunk lazy)"]
    DL["data: cache theo khoá, vô hiệu qua SSE"]
  end
  subgraph go["viderse serve (một process Go)"]
    GW["web: route /api/* trên platform/webapp"]
    EV["platform/events: SSE + revision"]
    PL["pipeline: 9 bước, lane, slot ảnh, fence"]
    SCN["scan: InnerTube, nhịp quét, khung đăng"]
    TL["tools: yt-dlp · ffmpeg · uv/whisper · codex · chrome"]
    AI["ai: core/llm (chữ) · imagegen (Codex, Grok, API)"]
    RD["render: giám sát Chrome headless"]
  end
  PG[("Postgres viderse_go")]
  FS[("Thư mục dự án (V22)")]
  web <-->|"HTTP + SSE"| GW
  GW --> PL & SCN & AI
  PL --> TL & AI & RD
  RD -->|"mở #/render"| ED
  PL & SCN & GW --> PG
  PL & RD --> FS
  PL & SCN --> EV --> GW
```

| # | Quyết định | Lý do |
|---|---|---|
| A1 | **Port theo test, không theo hình code.** Mỗi module Go có các ca test chuyển từ file test tương ứng của yt-factory. Logic thuần dùng test dạng bảng; phần chạm Postgres dùng `platform/pgtest`. | yt-factory có 543 file test và đã kiểm nhiều ca biên. Chuyển ca test là cách nhanh nhất để chứng minh hành vi giống bản cũ. |
| A2 | **Một process, worker là goroutine.** Số lane bằng `jobConcurrency`; slot ảnh là semaphore trong process. Chỉ một `viderse serve` được chạy worker, giữ bằng `pg_try_advisory_lock`. Lease và fence vẫn ghi trong Postgres, nhưng chỉ để phục hồi sau khi sập. | Bỏ được chi phí mỗi job một process Node, bỏ phối hợp nhiều process qua bảng settings, và hết tranh khoá SQLite (`sqlite-busy.ts`). |
| A3 | **Đẩy sự kiện bằng SSE có số revision.** `GET /api/status` và `GET /api/activity` giữ đúng hình dữ liệu của design (§5, §11), dùng cho lần tải đầu và làm đường dự phòng. Client chỉ poll 3 giây khi mất SSE. | JobsPill cập nhật dưới 1 giây, design chỉ đòi ≤ 3 giây. Không còn 6 vòng poll song song như ở yt-factory. Đây là khác về cách truyền so với chữ "poll 3 giây" ở §5, nên ghi hỏi đáp. |
| A4 | **Chữ gọi thẳng `core/llm`** (provider `openai-codex` OAuth, structured output qua `llmcontract`). Ảnh đi qua interface `imagegen.Provider`: bản đầu là Codex CLI `image_gen` và Grok y như yt-factory; V17 thêm Gemini, OpenRouter, Ollama. | Mỗi lần gọi chữ không còn phải khởi động CLI. Ảnh giữ đúng đường đang chạy ổn. |
| A5 | **Transcript giữ sidecar Python** (faster-whisper qua `uv`); Go chỉ giám sát process. | Chất lượng nhận dạng đã kiểm. Viết lại bằng Go không được gì thêm. |
| A6 | **Xuất video giữ engine editor TS.** Engine chuyển vào `apps/viderse/web/src/editor/` thành chunk lazy. Trang `#/render/<requestId>` chạy trong Chrome headless do Go giám sát; ffmpeg nén ở bước cuối. | Xem trước trong editor và bản xuất dùng chung một engine nên không lệch nhau. Viết lại bằng filtergraph ffmpeg vừa lớn vừa dễ sai. |
| A7 | **Port client InnerTube sang Go** (`internal/youtube`), chạy với fixtures của yt-factory. Khoá Data API (D5) là tuỳ chọn, chỉ dùng cho số liệu sau đăng. | Không kéo Node vào lúc chạy. |
| A8 | **Postgres là nguồn sự thật về trạng thái.** Tệp nằm trong thư mục dự án (V22), DB chỉ lưu đường dẫn tương đối. Mọi tệp ghi nguyên tử (ghi tạm rồi đổi tên). | Phục hồi được, và di chuyển được thư mục dự án. |
| A9 | **View-model tính ở server.** `agentView`, `subline`, nhóm trạng thái, `bulkEligibility` và lời giải thích hàng chờ đều viết bằng Go. Web chỉ hiển thị; CLI dùng chung code Go. Go test sinh JSON mẫu (golden) để test web đọc lại. | Mỗi luật chỉ có một bản. Hợp đồng giữa Go và TS được test giữ. |
| A10 | **Hạ tầng dùng chung chuyển lên `platform/`.** Hub SSE thành `platform/events`; client SSE và cache của web thành mã dùng chung. Noverse chuyển sang dùng bản chung trong cùng commit, không để lại bản sao. Mã chỉ Viderse dùng (tools, pipeline) ở lại trong app. | Theo luật chia sẻ của README verse: app thứ hai cần thì mới chuyển. |

# Hiệu năng và ổn định: ngưỡng nghiệm thu

| Chỉ số | Ngưỡng | Đo bằng |
|---|---|---|
| JS tải lần đầu (shell + Bàn dựng), gzip | ≤ 250 kB; editor/render chỉ tải khi mở route của nó | script kiểm kích thước dist, phase 01 |
| API danh sách (`/api/clone?channel=all`, Khám phá), local, với 1.000 job và 20.000 video đối thủ | p95 ≤ 100 ms | benchmark Go có seed dữ liệu |
| Từ lúc job đổi trạng thái tới lúc JobsPill hoặc Bàn dựng cập nhật | ≤ 1 s khi có SSE; ≤ 3 s khi rơi về poll | test e2e đo thời gian |
| Mất máy chủ → hiện dải báo mất kết nối | ≤ 5 s (design V12) | smoke trên trình duyệt |
| Cuộn danh sách 20.000 video | DOM ≤ 200 hàng; không có frame dài trên 50 ms | Chrome performance trace |
| `viderse serve` lúc rảnh | RSS ≤ 150 MB, chưa tính process con | `ps` |
| `kill -9` giữa bất kỳ bước nào | Khởi động lại thì job chạy tiếp từ checkpoint, không vẽ lặp ảnh đã trả tiền, không mồ côi process con | test phục hồi, phase 05 và phase 10 |
| Rút ổ chứa thư mục dự án khi đang chạy | Cần xử lý trong ≤ 10 s, không hỏng video; cắm lại thì tự chạy tiếp (design V22) | smoke, phase 10 |

# Các phase

| Phase | Gói design | Phụ thuộc | Ước lượng |
|---|---|---|---|
| [01 · Nền móng, dọn Writer, shell](phase-01-foundation.md) | V0, V12, hỏi đáp #1–#4 | — | 32h |
| [02 · Công cụ máy, kết nối, Cài đặt](phase-02-tools-settings.md) | V7, V17, V22, phần tài khoản của V23, V24, bước setup của V9 | 01 | 48h |
| [03 · Trinh sát: đối thủ, Khám phá](phase-03-rivals-explore.md) | V5, §12 | 02 | 44h |
| [04 · Kênh và Thư viện](phase-04-channels-library.md) | V4, V8, V9, V15, V23 | 03 | 52h |
| [05 · Lõi pipeline, Làm video, Hàng chờ](phase-05-pipeline-queue.md) | V2, V6, §1–§4 | 04 | 88h |
| [06 · Editor, render, xuất video](phase-06-editor-render.md) | V13, bước `export` | 05 | 60h |
| [07 · Bàn dựng, Duyệt, Việc nền](phase-07-workbench-review.md) | V1, V3, V14, V16, V18 | 05, 06 | 60h |
| [08 · Trợ lý chủ động](phase-08-assistant-autonomy.md) | V11, V19, V20, V21 | 07 | 40h |
| [09 · Điện thoại qua LAN](phase-09-mobile-lan.md) | V10, D1 | 07 | 20h |
| [10 · Nghiệm thu, chạy live, thôi dùng yt-factory](phase-10-acceptance-cutover.md) | toàn bộ | 01–09 | 16h |

Mỗi phase là một lát cắt dọc gồm backend, web và test. Phase chỉ xong khi đủ cả 4 việc:

- smoke trên `viderse serve` thật, có trình duyệt;
- ghi gói vào `10-gap-closure.md`;
- commit;
- kiểm Actions vẫn tắt, push, và xác nhận số run CI là 0.

# Mục sẽ ghi vào `11-hoi-dap.md` (phase 01)

- **#1–#4:** dev trả lời.
  - #1: đổi `defineBrand` theo 02 §2.
  - #2: thêm `variant="quiet"` vào `Button` của kit.
  - #3: thêm `tone="agent"` cho `Banner`, nhận màu từ `AgentOrb`.
  - #4: `toast()` nhận thêm nội dung dạng thẻ 300px.
  - Cả bốn đều chỉ thêm, không đổi gì của Noverse.
- **#5 (D8):** đã chốt theo đề xuất.
- **#8 Truyền sự kiện:** SSE thay poll 3 giây. Hình dữ liệu §5 và §11 giữ nguyên (A3).
- **#9 Không nhập dữ liệu yt-factory:** bỏ nhóm "Bản cũ" và danh sách job cũ ở Lưu trữ (09 §3).
- **#10 Không còn Electron:**
  - V14: `Notification` và `dock.setBadge` đổi thành Notification API và `navigator.setAppBadge` (PWA).
  - V22: chọn thư mục bằng hộp duyệt thư mục phía server, vì trình duyệt không trả được đường dẫn thật.
- **#11 D1 đã chốt:** §10 chọn cách (a).

#6 và #7 (Bilibili trên điện thoại, `loginUrl`) làm ở phase 02: API trả `loginUrl` (§24.1); thử trên điện thoại ở phase 09.

# Rủi ro chính

| Rủi ro | Cách giảm |
|---|---|
| Port state machine của clone (`service.ts` 78 KB, `runner.ts`, `repository.ts`) lệch hành vi | A1: chuyển ca test trước rồi mới viết code. Có test phục hồi `kill -9` cho từng bước. |
| InnerTube đổi định dạng | Fixtures, kèm lỗi `unexpected_page` báo to như bản cũ. Khi lỗi thì ghi vào Cần xử lý, không âm thầm bỏ qua. |
| Engine editor 159k dòng gắn chặt với Next | Chỉ 9 tệp trong `src/editor` import `next/`. Thay bằng router hash và API client; engine giữ nguyên, ghi rõ commit gốc trong `UPSTREAM.md`. |
| Tốn tiền vẽ ảnh khi nghiệm thu | D7 giới hạn 50 ảnh/ngày. Lượt chạy thật ở phase 05 và phase 10 phải hỏi anh trước, kèm số ảnh dự kiến. |
| Mở LAN (D1) làm lộ app | Mọi `/api/*` và tệp media đều cần phiên. Thiết bị mới phải ghép bằng QR. webguard chặn yêu cầu khác origin. Có giới hạn số lần đăng nhập (`platform/identity`). |
| Tách `platform/events` làm hỏng Noverse | Chuyển trong cùng commit, chạy đủ test Noverse. Live Noverse chỉ cập nhật khi chủ động chạy `run-live.sh update`. |

# Quy ước khi làm

Theo AGENTS.md của repo và các quy ước tài nguyên hiện có:

- **Việc nặng:** chạy từng việc một, bọc trong `lockf -k /tmp/verse-heavy.lock` và `nice -n 10`.
- **Go:** `GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4`, `go test -p 2`, `-race -p 1`.
- **Vitest:** `--maxWorkers=2`.
- **Commit:** conventional, không mã plan hay phase trong code, test hoặc commit; quét secret trước mỗi commit.
- **Đồ thứ ba:** engine editor giữ `LICENSE-freecut` và `LICENSE-openreel`, ghi thêm vào `third_party/NOTICE` của verse. ffmpeg chỉ gọi binary ngoài, không nhúng.
