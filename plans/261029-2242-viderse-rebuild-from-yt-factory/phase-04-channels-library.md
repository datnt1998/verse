---
phase: 4
title: "Kênh của bạn, Tạo kênh, Thư viện, Cài đặt template"
status: pending
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

## Nghiệm thu

- Đạt mục Nghiệm thu của **V4** (trừ mức tự chủ), **V8**, **V9**, **V15** và phần thư viện của **V23** (08-dev-prompt).
- Mỗi khoá cài đặt chỉ sửa được ở đúng một màn (bảng "Thiết lập đổi nhà" ở 09 §3, bỏ dòng `legacy`).

## Rủi ro

- **Recipe template có nhiều trường lồng nhau.** Mỗi trường được kiểm bằng một schema JSON dùng chung cho API và CLI. Test chuyển từ `recipe-sections`.
- **Xem thử tốn ảnh.** Lưu kết quả theo vân tay recipe, tính vào giới hạn D7, và hiện giá trước khi bấm.
