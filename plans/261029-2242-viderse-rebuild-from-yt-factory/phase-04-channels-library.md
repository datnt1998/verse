---
phase: 4
title: "Kênh của bạn, Tạo kênh, Thư viện, Cài đặt template"
status: in-progress
priority: P1
effort: 52h
dependsOn: [3]
---

# Phase 04 · Kênh, Tạo kênh, Thư viện, Cài đặt template

## Mục tiêu

- Tạo được kênh của mình theo một luồng duy nhất, gồm hồ sơ kênh và nhận diện. Mẫu chuẩn chạy nền.
- Mỗi thiết lập của kênh có một nhà riêng.
- Thư viện là chỗ duy nhất giữ template, prompt ảnh, video nền và lớp phủ.
- Template tự lưu, có xem thử và có giá.

Gói design:

- **V4** Kênh (trừ phần mức tự chủ, làm ở phase 08);
- **V9** Lần đầu mở (bước `create|done`) và Tạo kênh;
- **V8** Thư viện và Cài đặt template;
- **V15** Kho prompt ảnh;
- **V23** Video nền (phần thư viện và danh mục).

## Đầu vào (yt-factory)

| Nguồn | Nội dung cần port |
|---|---|
| `src/server/channels/branding.ts`, `components/owner-channels/channel-branding-panel.tsx` | Tạo nhận diện kênh (ảnh đại diện, banner) |
| `src/app/my-channels/*`, `components/owner-channels/{sources-tab,templates-tab,settings-tab}.tsx` | Nguồn và tự động, template luân phiên, hồ sơ kênh |
| `src/server/templates/{template-snapshot,background-clip}.ts` | Snapshot template khi job bắt đầu, cắt clip nền |
| `src/app/templates/*`, `components/templates/{recipe-sections,thumbnail-preview,preview-pane}.tsx` | Các nhóm trường của recipe, xem thử ảnh bìa |
| `src/server/backdrop/{catalogue,listing,pacing,bilibili-api}.ts` + `__fixtures__` | Danh mục video nền Twitch và Bilibili, điều tốc theo nền tảng, ký WBI |
| `src/server/editor/overlay-library.ts`, `scripts/generate-overlay-pack.mts` | Thư viện lớp phủ |
| `src/server/clone/master-prompt-default.ts`, `image-prompt.ts` (phần prompt dựng sẵn) | Prompt ảnh dựng sẵn (`builtin:scene`, `builtin:cover-*`) |

Design:

- 07 §8 (Tạo kênh), §13 (prompt ảnh), §16 (hồ sơ kênh và nội dung đăng), §24 (video nền);
- 08 V4, V8, V9, V15, V23;
- artboard `Channel` (`tab=videos|sources|templates|settings`, `filter`, `select`, `identity`, `link`, `fresh`), `CreateChannel`, `Onboard` (`step=create|done`, `identity`, `rivals`), `Library` (`tab=templates|prompts|backdrops|overlays|projects`, `prompt=`, `nguon=`), `Template` (`section=images|cover|backdrop|output`, `preview=fresh|stale|building`, `fewClips`, `picker`).

## Việc

### Backend

- **Migration `0000N_channels_library.sql`:**
  - `owner_channels` (cột hồ sơ theo §16), `owner_channel_sources`, `owner_channel_templates` (thứ tự luân phiên);
  - `templates` (recipe dạng jsonb, có `revision`), `image_prompts` (§13), `overlays`;
  - `backdrop_sources` (kênh và video lẻ, `segmentFromSec`/`segmentToSec` ≤ 600), `backdrop_videos`, `backdrop_platform_state`.
- **`internal/channels`:**
  - CRUD kênh;
  - bắt buộc ít nhất 1 đối thủ ở UI (§8; CLI vẫn cho bỏ trống);
  - tạo nhận diện chạy nền: chữ qua `core/llm`, ảnh qua `imagegen`;
  - gắn template luân phiên.
- **`internal/library`:**
  - template tự lưu theo `revision`; ghi đè lệch phiên bản trả 409;
  - xem thử ảnh bìa và ảnh cảnh chạy nền, kết quả lưu cache theo vân tay recipe;
  - ước giá theo §15 `price.perImage` và số ảnh dự kiến;
  - prompt ảnh dựng sẵn và prompt của bạn (§13): `imagePromptMode` tự suy ra, `coverPromptId`;
  - lớp phủ.
- **`internal/backdrop`:**
  - danh mục Twitch và Bilibili, điều tốc theo nền tảng, ký WBI;
  - dán link nền tảng chưa hỗ trợ thì lưu yêu cầu trên máy, gửi kèm khi Báo lỗi (D8);
  - lấy clip qua yt-dlp (`tools`).
- **API:** theo các mục đã nêu ở 07; đường dẫn `/api/owner-channels…`, `/api/templates…`, `/api/image-prompts…`, `/api/backdrops…`, `/api/overlays…`.
- **CLI:** `viderse channel …`, `viderse template …`, `viderse backdrop …`, `viderse overlay …`, giữ nghĩa của `cli/template.ts`, `backdrop.ts`, `overlay.ts`.

### Web

- `screens/channel`:
  - 4 tab;
  - Kênh › Video: bảng 4 cột cùng `BulkBar` (phần thao tác nối ở phase 07);
  - Nguồn & tự động: đặt **nhịp** (chỉ tính video hàng loạt và video tự lấy);
  - Hồ sơ kênh: ngôn ngữ, giọng văn, đoạn cuối mô tả ≤ 1.000 ký tự, tag luôn thêm ≤ 10.
- `screens/create-channel` và bước `create|done` của `screens/onboard`: nhận diện chạy nền, đối thủ mới thêm ngay trong luồng.
- `screens/library`:
  - các tab template, prompt, video nền, lớp phủ;
  - danh sách dài thì ảo hoá.
- `screens/template`:
  - 4 nhóm trường, tự lưu khi rời ô;
  - xem thử: `fresh` / `stale` / `building`;
  - hiện giá ước tính;
  - bộ chọn clip, cảnh báo khi quá ít clip.

## Kiểm thử

- Go:
  - chuyển ca test từ `template-snapshot`, `background-clip`, `catalogue`, `pacing`, `bilibili-api` (WBI) và test branding;
  - template: ghi đè lệch `revision` trả 409;
  - nhịp: video chọn tay không bị tính.
- Web: test tự lưu (rời ô thì lưu, lỗi thì giữ nguyên chữ đang gõ), test bắt buộc có đối thủ, test trạng thái xem thử.
- Smoke thật:
  - tạo 1 kênh có 2 đối thủ, nhận diện chạy xong;
  - tạo template từ prompt dựng sẵn, xem thử 1 ảnh bìa (đúng 1 ảnh trả phí);
  - thêm 1 kênh video nền Twitch.

## Quyết định triển khai (31/10/2026)

- **Schema** (`00008_channels_library.sql`): ngoài các bảng đã nêu, `owner_channels.identity_failed_step` (`text|images`) cho biết nhận diện hỏng ở bước Đạo diễn hay Hoạ sĩ; máy chủ ghi, web không đoán.
- **Mẫu chuẩn** chỉ là recipe (editor là V13). Kênh tạo không tên thì template tạm tên “Mẫu chuẩn”, đổi sang “Mẫu chuẩn · <tên kênh>” khi kênh có tên; template đã đổi tên tay hoặc kênh khác cũng dùng thì giữ nguyên, `revision` không đổi.
- **Khoá kèm lý do** mọi thứ cần video đã làm, hàng chờ hoặc editor: dựng lại xem thử, vẽ thử bìa, Ảnh mẫu prompt, bảng video và `BulkBar`, số liệu đầu kênh, “Hôm nay x/y”, Dặn trợ lý. Không làm mức tự chủ và ưu tiên.
- **Điện thoại**: cột trái Kênh ẩn dưới 768px như `MChannel` (đổi kênh ở viên kênh trên thanh đầu), tab tên ngắn và cuộn ngang; dưới 1100px cột trái xuống dưới nội dung.
- **Ảnh thu nhỏ Twitch** lấy cỡ 640×360 thay vì 1920×1080 (khoảng 58 KB thay vì 216 KB mỗi ảnh).
- **Lỗi chung tìm được khi smoke** (sửa ở `core/llm`, ảnh hưởng cả Noverse): Responses API cần `image_url` dạng chuỗi kèm `detail` riêng; ảnh đầu vào gửi byte đã giải base64 thay vì base64 hai lần.

## Kiến trúc & hiệu năng (đo 31/10/2026, binary thật, DB smoke)

| Chỉ số | Kết quả | Ngưỡng |
|---|---|---|
| GET p95 (300 lượt): `/api/owner-channels`, `/{id}`, `/api/templates`, `/{id}`, `/api/image-prompts`, `/api/overlays`, `/api/backdrop/channels`, `/videos`, trang video kênh nền 24 thẻ | 1,0–3,0 ms | ≤ 100 ms |
| JS tải lần đầu (`npm run size`) | 146,1 kB gzip (phase 03: 129,4) | ≤ 250 kB |
| Chunk lười: Kênh / Tạo kênh / Thư viện / Template | 10,5 / 4,5 / 11,8 + 3,4 CSS / 11,8 + 2,6 CSS kB gzip | — |
| Rảnh 60 giây, có một trang mở | CPU 0,0 %, RSS 17,6 MB, 24 giao dịch DB, 4 dòng ghi | không poll khi rảnh |
| Trang Template mở 20 giây | 0 yêu cầu HTTP | không poll khi rảnh |
| Ảnh thu nhỏ Twitch, trang đầu 24 thẻ | ~1,4 MB thay vì ~5,2 MB | — |

## Tiến độ (31/10/2026)

- [x] Backend: `channels` (CRUD, nhận diện chạy nền, template luân phiên, nhịp), `library` (template theo `revision`, prompt ảnh, lớp phủ, giá ước tính), `backdrop` (Twitch, Bilibili WBI, điều tốc, đoạn ≤ 600 giây), store Postgres, migration 00008; CLI `viderse channel|template|backdrop|overlay`.
- [x] Web: Kênh 4 tab, Tạo kênh `create|done`, Thư viện 4 tab, Cài đặt template 4 nhóm; Khám phá mở điều khiển cần Kênh.
- [x] Review code backend 3 mảng; QA ảnh cặp 1440×900 và 390×844 cho Kênh, Tạo kênh, Thư viện, Template.
- [x] Smoke trên binary thật: kênh 2 đối thủ thật, Đạo diễn đặt tên, nhận diện xong (đúng 2 ảnh trả phí: avatar PNG 800×800, ảnh bìa JPEG 2560×1440); kênh nền Twitch 212 video; template tự lưu, gắn nguồn nền.
- [x] Ghi `10-gap-closure.md` (V4, V8, V9, V15, V23) và hỏi đáp #38–#48.
- [ ] Vẽ thử 1 ảnh bìa trả phí: dời sang smoke phase 05 (cần video đã làm).
- [ ] Mức tự chủ (phase 08); điều khiển cần hàng chờ (phase 05); `MChannel`/`MCreate` (phase 09).

## Nghiệm thu

- Đạt mục Nghiệm thu của **V4** (trừ mức tự chủ), **V8**, **V9**, **V15** và phần thư viện của **V23** (08-dev-prompt).
- Mỗi khoá cài đặt chỉ sửa được ở đúng một màn (bảng "Thiết lập đổi nhà" ở 09 §3, bỏ dòng `legacy`).

## Rủi ro

- **Recipe template có nhiều trường lồng nhau.** Mỗi trường được kiểm bằng một schema JSON dùng chung cho API và CLI. Test chuyển từ `recipe-sections`.
- **Xem thử tốn ảnh.** Lưu kết quả theo vân tay recipe, tính vào giới hạn D7, và hiện giá trước khi bấm.
