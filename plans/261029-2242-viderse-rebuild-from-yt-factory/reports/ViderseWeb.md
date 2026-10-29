# ViderseWeb — shell, V0/V12, data layer, dev kit

Phạm vi: `apps/viderse/web/**` (không đụng Go, không đụng `platform/ui`).

## Đã làm

**Brand** (`src/brand.ts`): `defineBrand` + `applyBrand` với primary `#FF4D63`, success `#A6EDC1`, warning `#F6CB8C`, danger `#FF9A5C`, info `#B9D0F7`, `roles.accentDeep #E6002E` (02 §2). Logo ống kính `components/LensLogo.tsx` (cung dày khi ≤30px, xoay 9s khi có job, tôn trọng reduced-motion), favicon + app icon từ `docs/design/v1/brand/svg` → `public/`.

**Router** (`src/router.ts`): typed hash router đủ 13 route (`#/`, `#/review`, `#/queue`, `#/channels/<id>`, `#/channels/new`, `#/explore`, `#/new`, `#/library`, `#/templates/<id>`, `#/editor/<projectId>`, `#/settings`, `#/account`, `#/dev/kit` chỉ DEV) + query có kiểu (`tab`, `step`, `section`, `rival`, `video`, `channel`, `board`); enum sai kiểu bị bỏ, hash lạ → Bàn dựng và `Root` chuẩn hoá lại địa chỉ bằng replace. `paths.*`, `navSection()`. Mỗi màn `React.lazy`; shell + Bàn dựng nằm trong chunk đầu.

**Shell**: `AppHeader` (48px, logo, `ChannelSwitcher`, `MainNav`, `JobsPill`, bánh răng + chấm đỏ theo `attention.length`), `MainNav` (5 mục, badge vàng `reviewCount`, cuộn ngang <700px), `MobileShell` (khung: `MobileAppBar` + `MobileTabBar` 5 mục, nút + đỏ tròn giữa, pill đỏ nhạt mục đang mở), `AppShell` (dải offline, nội dung mờ 42%).

**V0** (`src/components/`): `agents.ts` (Trinh sát→architect, Đạo diễn→writer, Hoạ sĩ→editor, Kiểm định→arbiter + màu chữ), `Orb` (bọc `AgentOrb`), `ChannelAvatar`, `StatusLabel` (7 trạng thái, spinner khi Đang làm), `ActionButton` (primary/outline/quiet/danger-soft/danger, `disabledReason`), `Chip/Banner/Toast` dùng kit trực tiếp.

**V12**: `confirm.tsx` phủ đủ 7 tình huống (pause radio + nhớ lựa chọn, overwrite so 2 bản + Giữ cả hai, logout nhà cung cấp, xoá video 2 cột hệ quả, xoá kênh gõ đúng tên + lý do khoá, dừng mọi việc giữ-để-xác-nhận, bỏ thay đổi chưa lưu) + `confirmAutonomy` (Dialogs). `states.tsx`: `SystemLoading`, `SystemNotFound`, `EmptyWorkbench`. Offline: `OfflineStrip` hiện ngay khi luồng SSE không còn `open` (trễ ≤ 1 nhịp lỗi, xa dưới 5 s), đếm ngược thử lại, nút Thử lại ngay + Chi tiết; `useOnline()` cho nút ghi.

**Data**: `@noverse/ui/data` (cache/keys/events) + `useResource` port sang; `data/eventKeys.ts` bảng `status → keys.status()`, `reset → '*'`; `data/events.ts` subscribe + gộp invalidate 250ms + poll `/api/status` 3 s chỉ khi SSE đứt; `data/status.ts` `useStatus()`/`useOnline()`; `api/status.ts` kiểu `Status` theo 07 §5.

**Copy**: `src/i18n/vi.ts` đủ thuật ngữ, mẫu câu, 7 hộp xác nhận, mã lỗi → câu; không chuỗi UI inline (trừ nhãn demo ở trang dev).

**`#/dev/kit`** (DEV): dựng lại Kit (8 nhóm) và MKit (5 nhóm) bằng component thật trong khung 390px; chọn bảng/nhóm bằng `?board=&section=`.

**Kiểm tự động**: `src/checks/design-rules.test.ts` (không hex ngoài token/đồ hoạ, không bán kính cứng, không border-left/right, ≤1 `ActionButton primary` mỗi màn), `scripts/size.mjs` + `npm run size` (ngưỡng 250 kB).

## Kiểm chứng

- `GOTOOLCHAIN` không áp dụng (không sửa Go).
- `npx tsc --noEmit` → sạch.
- `npx vitest run --maxWorkers=2` → **6 tệp / 50 test xanh** (router 26, shell 7, confirm 8, App gate 6, client 3...).
- `npm run build` → xanh; `npm run size` → **112.4 kB gzip** JS tải lần đầu (1 chunk, ngưỡng 250).
- Ảnh so sánh: `/tmp/viderse-kit-compare/` (dev-kit kit-foundation, kit-confirm, kit-inputs, mkit-bars; artboard kit-foundation, mkit-bars; prod-home). Server 5181/5182/8711/8799 đã dừng.

## Khác design (để `11-hoi-dap.md`)

1. **Kit › Nền tảng**: artboard là bảng swatch tài liệu token; trang dev dựng **gallery component thật** (logo, orb, StatusLabel, chip, avatar, icon) vì token đã nằm trong `brand.ts`/`tokens.css`. Các nhóm còn lại bám artboard.
2. **`.vv-badge` không có định nghĩa** trong kit.css/tokens.css/artboard → đặt `--vv-badge-ink: #3D2A08` (mực nâu trên vàng) trong `styles/viderse.css`.
3. **Orb trợ lý** dùng `AgentOrb` của kit (đúng 05-kit) nên hue hơi khác cung vẽ trong Brand artboard; màu *chữ* trợ lý thì theo đúng 03-tokens.
4. **Dev server**: các atom StyleX của kit `Button` không vào CSS dev (nút hiện chữ trơn) — cùng component render **đúng trong bản build** (ảnh `prod-home.png`: nút primary trắng). Chỉ ảnh hưởng trang nội bộ khi chạy `npm run dev`; cần KitShared xem `unstable_moduleResolution` khi ghép Viderse.
5. **MobileShell chỉ là khung** (theo non-goal): có app bar + tab bar + badge, nội dung vẫn là EmptyState của design.

## Chưa có / phụ thuộc

- Các màn nghiệp vụ hiện `EmptyState`/`SystemNotFound` theo đúng giao kèo (nội dung thật ở gói màn hình sau).
- `ChannelSwitcher` nhận `channels` từ ngoài; API kênh chưa có nên hiện "Mọi kênh" + Tạo kênh.
- `/api/status` + `/api/events` do GoCore; UI đã bám hợp đồng chung (ETag không dùng ở client, SSE resume qua `@noverse/ui/data`).

Status: DONE_WITH_CONCERNS
Summary: Shell + typed router + V0/V12 + data layer + dev kit + copy + checks xong; typecheck/vitest(50)/build/size(112.4 kB) xanh.
Concerns/Blockers: Kit `Button` mất style riêng trên dev server (prod đúng) — cần KitShared xem cấu hình StyleX khi tích hợp; trang dev "Nền tảng" là gallery component thay cho bảng swatch.

## Follow-ups (integrator)

1. **Proxy dev** → mặc định `http://127.0.0.1:8789` (`VIDERSE_ADDR`); vẫn giữ `VITE_API_TARGET` để trỏ stub.
2. **Tailwind**: còn dùng — `src/styles/base.css` mở đầu `@import "tailwindcss"` + `@source` (preflight giữ thứ tự layer cho StyleX). Đã **khôi phục plugin** `@tailwindcss/vite`, nên KHÔNG gỡ `tailwindcss`/`@tailwindcss/vite` khỏi package.json/lockfile.
3. **`#/dev/kit` › Nền tảng**: thêm `TokenSheet` (`src/screens/dev/`), đọc giá trị SỐNG bằng `getComputedStyle(:root)`:
   nền & bề mặt, chữ, brand (nhấn / công tắc / đỏ YouTube / nút +), trạng thái, orb + màu chữ trợ lý, bo góc kit,
   khung & vùng chạm (token mới `--vv-h-header/--vv-m-bar/--vv-m-tabbar/--vv-hit` — component dùng lại thay số cứng),
   phông chữ, thang chữ có số đo cạnh mẫu. Mỗi ô in tên token + giá trị; gallery component vẫn ở dưới.
   Ảnh: `/tmp/viderse-kit-compare/kit-tokens.png`.

**Phát hiện khi đối chiếu**: rãnh công tắc của kit là literal `--kit-switch-on: #9A63E3` (brand không ghi đè), nên đã ghim `overrides: { '--kit-switch-on': '#E6002E' }` trong `brand.ts` cho đúng accentDeep của design — giá trị sống giờ là `#E6002E`.

Kết quả chạy lại (dưới heavy lock): `tsc --noEmit` sạch · vitest **6 tệp / 50 test xanh** · `vite build` xanh · size **112.4 kB gzip** (1 chunk, ngưỡng 250). Server dev/stub đã dừng.

## Đợt sửa theo review (R3–R11 + low-priority)

- **R3**: `fetchSource` giờ chạy với `stallMs: 3500, stallCheckMs: 500, connectTimeoutMs: 3500` (server ping 2 s ⇒ down trong ≤5 s cả khi `read()` treo lẫn headers không về). `startDataEvents` nghe `offline`/`online` của trình duyệt → `retryNow()` ngay; `useOnline()` còn cộng `navigator.onLine` qua `useSyncExternalStore` nên dải + nút ghi khoá tức thì khi mất mạng. Test fake-timer: hello-rồi-im (read treo), headers-không-về, recovery sau backoff.
- **R4**: `api/sessionSignal.ts` mới — `api/request.ts` báo `notifySessionExpired()` trên mọi 401 (kể cả probe `/api/me` của luồng); `fetchSource.onUnauthorized` cũng đi thẳng vào đây. `SessionProvider` đăng ký listener `setUser(null)` → Root unmount shell → teardown của `startDataEvents` dừng poll/SSE + `cache.clear()`; màn Đăng nhập hiện, không reload. Test: 401 request thường, 401 lúc mở SSE, teardown giữa request pending (không ghi cache sau clear).
- **R5**: bỏ `fetchStatus()` thô trong `onReconnect` — chỉ `invalidate()`; cache tự fetch/huỷ/xếp thứ tự. Test spy `cache.setData` qua vòng rớt→nối lại: không ghi thô.
- **R6**: `text()` trả thẳng giá trị `URLSearchParams.get()` (đã decode một lần). Test: `%`, `%zz`, `%25`, `%2525` (encode đôi), Unicode; `routePath` encode lại đúng.
- **R7**: `confirmDeleteChannel` truyền `disabledReason = runningHint` khi `runningCount > 0` (prop mới của kit). Test: gõ đúng tên vẫn `disabled`, click + Enter đều không gọi `onConfirm`.
- **R9**: `onPick` bắt buộc; route mới `pickChannelRoute(route, id)` trong router.ts — Bàn dựng đổi `?channel` (giữ `video`), trang Kênh đổi `<id>`, "Mọi kênh" từ trang Kênh → `#/`. Cả AppHeader + MobileShell điều hướng bằng typed route; test hash sau khi chọn ở cả hai ngữ cảnh.
- **R10**: `VIcon`/registry/`src/icons/` xoá; 36 SVG còn thiếu chép sang `platform/ui/src/icons/` + `gen-icons.mjs` chạy lại (187 tên). Mọi call site dùng kit `Icon`; dev kit giữ gallery icon dùng kit `Icon`.
- **R11**: poll chỉ khi `document.visibilityState === 'visible'` VÀ SSE down; listener `visibilitychange` dừng khi ẩn và `cache.refresh` ngay khi tab trở lại. Test với spy visibilityState.
- **Low-priority**: guard `design-rules.test.ts` giờ bắt CSS `border-radius`/spacing literals + numeric JSX values (bỏ qua `0`); mọi literal trong `components.css` + JSX chuyển sang token (`--vv-s-*`, `--vv-r-*`, `--vv-shadow-*`, `--vv-err-*`, kit `--radius-*`); `ScreenEmpty` → `components/`; `#/account` lazy (test chờ macrotask); xoá `AccountMenu`, thay bằng avatar chữ cái đầu link `#/account` ở cuối AppHeader (đúng artboard Main: vòng 30px cạnh bánh răng).

**Gates**: `tsc --noEmit` sạch · vitest **7 tệp / 63 test xanh** · `vite build` xanh · size **111.2 kB gzip** (ngưỡng 250). Noverse/không đụng: chỉ thêm file vào `platform/ui/src/icons/` + regen `iconNames.ts` theo phân quyền R10.

## Đợt sửa smoke (browser findings + breakpoint/motion)

1. **Brand auth**: `src/kitLabels.ts` mới — `applyViderseLabels()` = `setKitLabels({ ...KIT_LABELS_VI, brand: t.brand })`; `main.tsx` và cả ba test dùng chung nó (trước main đặt KIT_LABELS_VI nguyên xi → "Noverse Studio" trên Setup/Login). `parts.tsx` bỏ `BrandLockup` của kit → lockup Viderse = `LensLogo` 24px + wordmark Be Vietnam Pro 700 −0.02em; `LensLogo` thêm `halo` (quầng đỏ, bản hero) — Login/Setup render `LensLogo size=64 halo` phía trên tiêu đề (Onboard dùng mark-hero). Test mới: auth render "Viderse", không "Noverse", wordmark + svg trong header.
2. **Khoá ghi khi offline**: vùng nội dung cả hai shell nhận `inert` (pointer + bàn phím) thay chỉ mờ 42%; strip + header ngoài vùng inert nên Thử lại vẫn bấm/gõ được. `.vv-offline-dim` đã xoá. Test: offline → `[inert]` có + strip hiện + retry không bị khoá; hello đến → hết inert.
3. **Breakpoint**: `useMediaQuery('(max-width: 767px)')` theo V10; comment sửa cả AppShell/MobileShell. Test boundary: matchMedia stub chỉ khớp đúng query đó → dưới 768 ra `.vv-tabbar`, từ 768 ra `.vv-header`. Rule cuộn ngang của nav (04 §32) giữ nguyên trong CSS.
4. **Reduced motion**: `base.css` thêm `@media (prefers-reduced-motion: reduce)` phủ toàn cục (`animation-duration/iteration-count`, `transition-duration`, `scroll-behavior` với `!important`) — đứng vòng cung logo, `ai-breathe` của AgentOrb, chấm đỏ `.vv-rec`, spinner, transition của cả kit. `design-rules.test.ts` thêm test stylesheet: block phải tồn tại, phải có animation + `!important`, không animation `!important` nào né được.

**Gates**: `tsc --noEmit` sạch · vitest **7 tệp / 67 test xanh** · `vite build` xanh (dist đã build lại vào `internal/web/dist`) · size **110.2 kB gzip**.
