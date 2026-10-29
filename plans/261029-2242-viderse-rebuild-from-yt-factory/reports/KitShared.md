# KitShared — phase 01 (web): @noverse/ui/data + kit additions

## Đã đổi

**Mới: `platform/ui/src/data/`** (subpath `@noverse/ui/data`, entry `data` trong vite lib build, exports `"./data": {"types": "./dist/data/index.d.ts", "default": "./dist/data.js"}`):
- `keys.ts`: `Key`, `KeyPart`, `keyHash`, `matchesPrefix` (copy nguyên bản generic).
- `cache.ts`: `createCache`, `cache`, `invalidate`, `setData`, `getData`, `sameData` + types — chuyển nguyên từ `apps/noverse/web/src/data/cache.ts` (xác nhận: compiled body giống hệt byte).
- `events.ts`: `createEventsClient`, `fetchSource`, `BACKOFF_MS`, `STREAM_STALL_MS`, `STREAM_WAKE_STALL_MS` + types. Khác phần app:
  - `EventsClientOptions.eventTypes: readonly string[]` **required** (client chỉ nghe event được liệt kê — Noverse truyền `SERVER_EVENT_TYPES` từ `eventKeys.ts`).
  - `EventHandler`/`subscribe` là `string` thay `ServerEventType`.
  - `fetchSource(url, { onUnauthorized })` — 401 gọi `onUnauthorized` của app thay vì hardcode `me()`.
- `index.ts`: export list trên (đã gửi ViderseWeb kèm ví dụ).
- Tests: `cache.test.ts` (5), `events.test.ts` (2 — backoff/probe port từ data.test.ts), `events.stall.test.ts` (3 — chuyển nguyên).

**Noverse (`apps/noverse/web/src`)**: `data/cache.ts` xoá, `data/keys.ts` chỉ còn `keys`/`StoryPart`/`ProjectPart` của app, `data/events.ts` chỉ còn singleton `events` + `subscribeEvents` + `startDataEvents` (wiring `SERVER_EVENT_TYPES`, `fetchSource(url,{onUnauthorized: me})`, probe, postWatch). `data/index.ts` re-export lõi kit + phần app (API giữ nguyên cho mọi consumer `../data`). `eventKeys.ts`, `useResource.ts`, `prefetch.ts`, `action.ts`, `modelContext.ts`, `Provider.tsx` + 11 file ngoài data chuyển sang `@noverse/ui/data` cho phần lõi. `data.test.ts` còn 9 test phần app (cache/events client chung → kit). `singleStream.test.ts` expect `[]` (EventSource giờ ở kit). `vite.config.ts` + alias `@noverse/ui/data → platform/ui/src/data/index.ts`. README data cập nhật.

**Kit additions** (additive, Noverse visuals không đổi — test `kitAdditions.test.tsx` 5 case):
- `Button variant="quiet"`: không viền, `--color-fg-3`, hover `--color-fg`.
- `Banner tone="agent"` + `agent?: AgentKind` (mặc định writer): orb 17px làm icon, nền/viền token mới `--banner-agent-<kind>-bg/-bd` (đã gen vào tokens.css/json/stylex.js/tokenDefaults).
- `toast.card({title, text?, action, tone?, key?})`: thẻ 300px, `role="alert"`, đúng 1 nút việc + nút đóng, `expiresAt=Infinity` (ở lại tới khi đóng), key-merge như toast thường. Type `ToastCard` export.
- `ConfirmDialog`/`confirm()`: `agent?: boolean | AgentKind` (true → orb writer như cũ) — theo yêu cầu ViderseWeb.

**boundary.test.ts**: thêm test "subpath data không kéo thư viện ngoài" (data/ chỉ import tương đối, không cả stylex).

## Contract cho consumer
- `eventTypes` required khi `createEventsClient` (phải gồm `'hello'`).
- `fetchSource` không tự `me()` — truyền `onUnauthorized` để chạy handler phiên.
- Event→key map, `keys.*`, `useResource`/`useAction`/`Provider` là của app (không ở kit).

## Verify (đã chạy)
- `platform/ui`: `npx tsc --noEmit` ✓ · `npx vitest run` 132/132 ✓ (gồm hardcodedColors + tokenParity) · `npm run build` ✓ (`dist/data.js` + `dist/data/index.d.ts` sinh đúng, import ESM kiểm trực tiếp ok).
- `apps/noverse/web`: `npx tsc --noEmit` ✓ · `npx vitest run` 1514/1514 ✓ · `npm run build` ✓ (dist mới: `index-dxIMO1nZ.js` + `index-CelPP53i.css`).
- So dist before/after: build `--minify false` của worktree HEAD vs sau đổi → `createCache`/`sameData` compiled giống hệt; `createEventsClient` chỉ khác `SERVER_EVENT_TYPES → options.eventTypes` + options signature; `fetchSource` chỉ khác `me() → options.onUnauthorized`. CSS +0.3 kB (quiet/agent-banner/toast-card styles). Các chênh khác: rename `push→push2` (helper `push` mới trong Toast), dòng token/stylex mới — đúng phần additive.

## Open issues
- `apps/noverse/internal/entry/web/dist/` được build lại thay baseline cũ — integrator quyết commit hay revert.
- `apps/viderse/web` tsconfig đã có `@noverse/ui/*` path và vite alias `@noverse/ui/data` (sẵn từ scaffold).
- `Icon.test.tsx` của kit quét icon names mọi app dùng kit: code viderse mới cần `workbench`, `channel`, `radar`, `video-add`, `gallery` (+bản `-b`). Đã copy 9 SVG từ `apps/viderse/docs/design/v1/icons/` vào `platform/ui/src/icons` và chạy `gen-icons` (151 tên) — test xanh trở lại; thuộc ranh giới mình nên xử lý luôn thay vì chờ.
- `ToastHost` vẫn đặt cuối-giữa như Noverse; Viderse design muốn góc phải dưới — chỉnh bằng CSS `--m-toast-bottom`/`style` hoặc override trong app Viderse khi mount host.

## Follow-up: kit StyleX atoms mất ở `vite dev` của Viderse (đã sửa)

**Triệu chứng** (ViderseWeb báo): dev 5181 nút kit hiện như chữ trơn (mất nền/đệm/bo góc); build production đúng.

**Tái hiện + so sánh**: dựng dev Noverse (5183) và Viderse (5184), đọc module đã transform của `platform/ui/src/Button.tsx` — hai app giống hệt nhau (atom class có trong JS), nên lỗi không ở transform. Đo `document.styleSheets` + computed style bằng Chromium headless:
- Noverse dev: sheet[0] = `<style>@layer theme, base, …, priority1…priority8</style>` (khai báo tĩnh trong `index.html`), sheet[1] = `__stylex_virtual__`.
- Viderse dev: sheet[0] = `__stylex_virtual__` (bắt đầu `@layer priority1;`), khai báo thứ tự chỉ nằm trong `src/styles/base.css` (do JS nạp sau).
- Nút mắt của `Field` (kit `Button` icon): Viderse trước khi sửa `border-radius: 0px`, `border: 0px solid …` (reset thắng atom) — sau khi sửa `border-radius: 8px`, `color: fg-3`, cùng giá trị với Noverse.

**Nguyên nhân gốc**: StyleX chèn `virtual:stylex.css` bằng một `<style>` tạo lúc chạy → luôn là stylesheet ĐẦU TIÊN. File này mở đầu bằng `@layer priority1; …`, nên nó chốt thứ tự layer trước app. Khai báo `@layer theme, base, …, priority1…` trong `base.css` (nạp sau) bị bỏ qua → `base/components/utilities` rơi xuống SAU mọi `priority*` → reset Tailwind preflight/`@layer base` đè atom của kit. Noverse không dính vì `index.html` có khai báo tĩnh cùng thứ tự ngay trong `<head>`.

**Sửa** (theo đúng pattern Noverse, không thêm cơ chế thứ hai): thêm khai báo tĩnh `<style>@layer theme, base, components, utilities, properties, priority, priority1…priority8;</style>` vào `<head>` của `apps/viderse/web/index.html` (trước `<title>`, cùng danh sách layer với `apps/noverse/web/index.html`, kèm comment giải thích race). `src/styles/base.css` vẫn giữ khai báo trùng — y như Noverse. Không cần plugin trong `vite.config.ts` (đã bỏ thử nghiệm `cssLayerOrder()`).

**Verify**: dev 5184 sau sửa — sheet[0] là khai báo layer, sheet[1] stylex; computed style nút kit khớp Noverse (bo góc, màu fg-3, nền/đệm của primary `bg fg / color bg / radius 10 / padding 0 14 / border 1`); dev 5183 không đổi. Dev 5183 sau khi chuyển sang index.html: sheet[0] = khai báo layer, sheet[1] = stylex; computed style nút kit y hệt lần sửa trước (nút mắt radius 8px/color fg-3; primary `bg fg / color bg / radius 10 / padding 0 14 / border 1`). `npx vitest run` 50/50 ✓, `npm run build` ✓ — `internal/web/dist/index.html` có `<style>@layer …</style>` trước `<link rel=stylesheet>` và module script.

## Review fixes (R3 client, R7 kit, R8)

- **R8 · toast card layout** (`platform/ui/src/Toast.tsx`): `Notification` giờ chọn `sx.toastCard` khi `item.card`, pill `sx.toast` cho toast thường — tone border giữ nguyên (`--line-4` neutral, `c.bd` cho tone màu). Test mới trong `kitAdditions.test.tsx` mount `ToastHost` thật và so atom layout (width 300 · column · radius 14 trên khung; atom pill radius 999/min-height 40 vắng mặt) bằng hợp đồng `stylex.create` ngữ nghĩa như `Selects.test.tsx`. Verify thêm trong Chromium thật trên dev Viderse (5183, đã tắt): computed `width: 300px`, `flex-direction: column`, `border-radius: 14px`.
- **R7 · `disabledReason`** (`platform/ui/src/ConfirmDialog.tsx`): prop mới `disabledReason?: string` (cũng trên `ConfirmOptions`/`confirm()`). Khi set: nút phá hoại `disabled`, `run()` return sớm (kể cả gọi qua onClick/secondary), `startHold()` không khởi động hold timer, Enter trong dialog bị chặn; lý do render thành dòng `sx.note` (icon warning-2 + text). Tests `ConfirmDialog.test.tsx` (happy-dom + createRoot): click path, hold path (pointerdown + Space, fake timers), keyboard path (Enter trên input + dialog), và idle path không-disabled vẫn gọi `onConfirm` (helper `typePhrase` dùng prototype setter — `input.value=` trực tiếp không sinh onChange trong happy-dom). Additive, Noverse không đổi.
- **R3 client · stall watchdog cấu hình được** (`platform/ui/src/data/events.ts`): `FetchSourceOptions` thêm `stallMs` (mặc định `STREAM_STALL_MS` 45s), `stallCheckMs` (mặc định 5000), `connectTimeoutMs` (mặc định `STREAM_STALL_MS` — headers response không về sau khoảng này ⇒ down; trước đây chỉ có watchdog 45s). `responded` flag clear timer khi headers về; `release()` clear cả hai timer. Wake check dùng `min(STREAM_WAKE_STALL_MS, stallMs)`. Tests `events.stall.test.ts` +2: hello-then-silence báo lỗi trong ~4s với cấu hình 3500/500/3500 (fake timers); headers-never-arrive báo lỗi đúng 3500ms. Defaults giữ hành vi Noverse (3 test cũ xanh). Viderse truyền `fetchSource(url, { stallMs: 3500, stallCheckMs: 500, connectTimeoutMs: 3500 })` — đã nhắn ViderseWeb.

**Verify**: `platform/ui` `npx tsc --noEmit` ✓ · vitest 139/139 ✓ · `npm run build` ✓ (dist/data.js 13 exports) · `apps/noverse/web` `tsc --noEmit` ✓ · vitest 1514/1514 ✓.
