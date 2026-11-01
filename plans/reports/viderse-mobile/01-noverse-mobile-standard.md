# 01 · Chuẩn mobile Noverse (as-built) → chuẩn mobile dùng chung

Phạm vi: mô tả **đúng như đang chạy** chuẩn mobile của Noverse (mã + spec), rồi so từng điểm với cách Viderse mobile đang dựng, kết thúc bằng **catalogue component** và **checklist chuẩn mobile dùng chung** (đã chia sẻ / Noverse-only cần đưa lên / Viderse-specific).

Không sửa mã sản phẩm trong báo cáo này.

---

## 0. Nguồn & cách đo

| Loại | Nguồn |
|---|---|
| Mã Noverse | `apps/noverse/web/src/mobile/*` (`index.tsx`, `runtime.tsx`, `tabs.tsx`, `primitives.tsx`, `mobile.css`, `logic.ts`, `toastBottom.ts`, `outbox.ts`, `pwa.ts`), `shell/KeepAlive.tsx`, `screens/**/mobile/**` |
| Spec Noverse | `apps/noverse/docs/design/v1/18-mobile.md` (bản đặc tả mobile), `artboards/M*.dc.html` |
| Kit dùng chung | `platform/ui/src` (`Sheet.tsx`, `Toast.tsx`, `Toast.css`, `kit.css`, `feedback.tsx`, `motion.ts`, `data/*`) |
| Mã Viderse | `apps/viderse/web/src/shell/{MobileShell,AppShell,OfflineStrip,toastAnchor}.tsx`, `App.tsx`, `data/events.ts`, `screens/**` (nhánh `mobile`), `styles/viderse.css` |
| Spec Viderse | `apps/viderse/docs/design/v1/03-tokens.md`, `04-components.md`, `05-kit.md`, `11-hoi-dap.md` (#57), `artboards/M*.dc.html`, `artboards/kit.css` |
| Đo thật | Chromium 390×844 (DPR 1, `env(safe-area-inset-*) = 0`), production `https://nov.aarverse.studio` và `https://vid.aarverse.studio`, tài khoản `qaaudit`. Ảnh: `/tmp/vmob-MStd-noverse-more-390.png`, `/tmp/vmob-MStd-viderse-settings-390.png` |

Quy ước: mọi con số px dưới đây là **đo được** hoặc đọc thẳng từ CSS; chỗ nào là suy luận ghi `[INFERENCE]`. Trên Chromium `env()` = 0 nên "px khi env=0" ≠ "px trên iPhone"; cột giá trị luôn ghi rõ công thức.

---

## 1. Kiến trúc mobile Noverse

### 1.1 Nhận diện bố cục — 3 mốc, một nguồn

- `MOBILE_QUERY = '(max-width: 640px)'`, `TABLET_QUERY = '(min-width: 641px) and (max-width: 1099px)'` — `apps/noverse/web/src/mobile/logic.ts:5-6`.
- `useLayoutMode()` trả `'phone' | 'tablet' | 'desktop'` từ hai `matchMedia` — `mobile/index.tsx:36-40`; `useMobileLayout()` cùng file `:35`.
- `App.tsx` chọn **cây khác nhau**, không phải nhánh trong màn: `layout === 'phone' → <MobileApp>` (`apps/noverse/web/src/App.tsx:129`), `tablet → <TabletApp>` (`:130`), còn lại `AppShell` (`:132-142`).
- Không có màn "trên máy tính" trên điện thoại: route chỉ tồn tại ở bản desktop được `phoneRedirect()` đổi URL ngay (`mobile/tabs.tsx:61-72`), gọi trong `mobile/index.tsx:103-106`.

### 1.2 Cây điện thoại riêng

Mỗi màn có thư mục `screens/<màn>/mobile/` riêng (33 file test trong các thư mục `mobile`), ví dụ `screens/reader/mobile/MobileReader.tsx`, `screens/workspace/mobile/index.tsx`, `screens/audio/mobile/index.tsx`. `MobileApp` chỉ là khung + bảng route → pane (`mobile/index.tsx:158-174`). Spec chốt: "**Không** dùng chung component lưới. Mỗi màn có `features/<màn>/mobile/`" — `18-mobile.md:473`.

### 1.3 Tab, pane, giữ trạng thái

| Việc | Cơ chế | Nguồn |
|---|---|---|
| Route → tab | `paneKeyOf(parsed)` trả `'work' \| 'read' \| 'audio' \| 'more:*'`, `null` = màn đẩy | `mobile/tabs.tsx:28-57` |
| Tab bar | 4 mục, `button` + `navigateNow()` (không `<a href="#…">` vì iOS bắn chạm trùng) | `tabs.tsx:104-155` |
| Chỉ báo lạc quan | đặt `--tab-index` ngay khi chạm, nhả về tab thật sau 800 ms nếu điều hướng bị chặn | `tabs.tsx:104-127` (`RELEASE_MS = 800`) |
| Pane giữ-alive | `KeepAlive` = `<Activity mode={visible?'visible':'hidden'}>` | `shell/KeepAlive.tsx:10-12` |
| Ảnh chụp pane | `keepPane` + `paneSignature` (JSON của route+search+project+story) để `memo` không bị phá mỗi `hashchange` | `tabs.tsx:80-93`, `index.tsx:98-100` |
| Render pane | `PaneView` là `memo` | `index.tsx:49-66` |
| Ẩn/hiện pane | `display:none` + `aria-hidden` + `MobilePaneScope active` (hook trong pane ẩn không đẩy trạng thái lên runtime) | `tabs.tsx:157-163`, `runtime.tsx:141-146` |
| Tab mode | `'default' \| 'mini' \| 'hide'`; `hide` khi bàn phím mở / màn đẩy / phiên hết hạn / đọc tập trung; `mini` khi cuộn xuống | `logic.ts:11-13`, `mobile/index.tsx:93`, `runtime.tsx:118-120` |

Đo thật (Noverse 390×844): sau khi rời `#/settings/mobile` sang `#/p/dit-dang-audio`, pane cũ **vẫn nằm trong DOM** `{"k":"more:mobile:","on":false,"display":"none","aria-hidden":"true"}` → quay lại là hiện lại nguyên trạng (state + vị trí cuộn + dữ liệu SWR).

### 1.4 Màn đẩy (subpage)

`route.name === 'activity' | 'projbrain' | 'notifications' | 'newstory' | settings/notify` → `useTabMode('hide')` và render `screen` **phủ lên trên** các pane (`index.tsx:92-93`, `:145-157`). Màn đẩy dùng `PushPage` (trượt 340 ms) — `primitives.tsx:65-67`.

---

## 2. Primitives & component mobile Noverse

Tất cả nằm ở `apps/noverse/web/src/mobile/primitives.tsx` + `runtime.tsx` (app-local, không ở kit):

| Primitive | Hành vi chốt | Nguồn |
|---|---|---|
| `MobileFeedback` + `useMobileToast()` | toast toàn cục, `role="status"`, tự tắt 2.500 ms, nằm trên thanh tab; mỗi màn đăng ký độ cao đáy qua `useToastBottom(px, priority)` → `--m-toast-bottom` trên `<html>` (ưu tiên cao thắng, cùng ưu tiên thì đăng ký sau thắng) | `primitives.tsx:8-18`, `toastBottom.ts:5-30`, `logic.ts:28-41` |
| `MobilePage({title, backTitle, onBack, actions, children, footer, manageScroll})` | topbar 52 px, vùng cuộn riêng, nút ‹ 44 px, footer 16/16/30 + gradient | `primitives.tsx:20-26`, `mobile.css:16`, `README.md:39` |
| `BottomSheet` | wrapper của kit `Sheet` (portal, scrim 58%, focus trap, Esc, kéo >30% đóng, vào 440/ra 260 ms) | `primitives.tsx:28-32`, `platform/ui/src/Sheet.tsx:32-113`, `kit.css:142-166` |
| `ActionSheet` | nhóm iOS: tiêu đề + mô tả + hành động (Promise, khoá khi chờ, lỗi giữ sheet), nút **Huỷ** tách riêng | `primitives.tsx:34-41`, `mobile.css:22` |
| `ConfirmSheet` | `role="alertdialog"`, nền trong suốt, không thanh kéo, một nhóm + một nhóm Huỷ | `primitives.tsx:43-63`, `mobile.css:29` |
| `PushPage` | đẩy trang 340 ms | `primitives.tsx:65-67`, `mobile.css:22` |
| `useMobileScroll(ref, {immersive, onScroll, disabled})` | **một** listener `scroll` passive/màn; `down = y > last && y > 40`; bỏ qua cú "cuộn lên" do thanh co lại clamp `scrollTop`; trả `{down, immersive, reveal}` | `runtime.tsx:148-183` (`:168-172`) |
| `useTabMode(mode?)` | yêu cầu chế độ trong vòng đời màn/sheet; `hide` thắng `mini`; tự dọn khi unmount | `runtime.tsx:130-139`, `logic.ts:11-13` |
| `useMobileRuntime()` | `{keyboard, tabMode, down, offline, seconds, retryNow}` | `runtime.tsx:22-26`, `:116-125` |
| `useStoryConnection()` | báo trạng thái worker SSE cho băng mất kết nối chung | `runtime.tsx:150-153` |

Trạng thái (empty/loading/error/offline/expired) là **hợp đồng**, không phải tuỳ màn:

- Loading: `Skeleton` của kit, **đúng bố cục** màn (ví dụ reader 5 dòng 18 px — `screens/reader/mobile/MobileReader.tsx:274`; brain 4 khối — `screens/brain/mobile/MobileBrain.tsx:100`), kèm `slow` → "Đang chậm hơn bình thường…" (`screens/activity/mobile/index.tsx:170`).
- Empty: `EmptyState` kit hoặc dòng `m-empty` (`screens/more/mobile/index.tsx:70,238`; `screens/more/mobile/more.css:45`).
- Offline: băng nổi + đếm ngược + "Thử ngay" + nội dung mờ 55%/giảm bão hoà — `runtime.tsx:126`, `mobile.css:25`.
- Expired: sheet đăng nhập **không đóng được**, `inert` toàn `.m-app`, trap Tab, giữ nháp đang gõ — `runtime.tsx:51-69`, `:127`.

---

## 3. Cách tạo kiểu

- **Lớp phẳng toàn cục**: `mobile/mobile.css` (31 dòng) giữ khung, thanh tab, lớp phổ quát (`.m-card`, `.m-group`, `.m-row`, `.m-section-label`, `.m-muted`, `.m-field`, `.m-button`, `.m-chip`, `.m-hscroll`, `.m-icon-button`, `.m-error`, `.m-inline-actions`, `.m-push`, `.m-toast`) — `README.md:47-49`.
- **CSS riêng từng màn** khi màn có nhịp riêng: `screens/reader/mobile/reader.css`, `screens/workspace/mobile/workspace.css`, `screens/audio/mobile/mobile.css`, `screens/more/mobile/more.css`, `screens/newstory/mobile/styles.ts` (StyleX).
- **StyleX** cho phần mới/động (`screens/newstory/mobile/styles.ts`, `screens/more/mobile/Rules.tsx`), token từ `@noverse/ui/tokens.stylex`.
- **Kit** cho control phổ quát: `Sheet`, `ToastHost`, `Skeleton`, `EmptyState`, `Spinner`, `Icon`, `Segmented`, `Switch`, `Field`… (`platform/ui/src/index.ts:1-37`).
- Luật ngầm quan trọng: *một khung duy nhất gắn safe-area + provider*; màn con **không** tự thêm khoảng trống cho thanh tab (`README.md:5`).

---

## 4. VÙNG AN TOÀN (safe area) — chuẩn tham chiếu của hệ

Đây là phần được chủ dự án ưu tiên; ghi đủ công thức + px + nơi thi hành.

### 4.1 Một nguồn khai báo duy nhất

```css
/* apps/noverse/web/src/mobile/mobile.css:6 */
:root{--m-safe-top:env(safe-area-inset-top,0px);
      --m-safe-bottom:env(safe-area-inset-bottom,0px);
      --m-tabbar-h:64px;
      --m-tabbar-bottom:max(20px,var(--m-safe-bottom))}
/* :7-12 — dải mờ iOS 26 */
:root{--m-edge:0px}
@supports (-webkit-touch-callout:none){
  @media (display-mode:standalone) and (orientation:portrait){:root{--m-edge:40px}}
  @media (display-mode:standalone) and (min-width:641px) and (min-height:641px){:root{--m-edge:40px}}
}
html[data-standalone]{--m-edge:40px}
/* :13 */
:root{--m-top:max(8px,calc(var(--m-safe-top) + var(--m-edge)))}
```

- `env(safe-area-inset-top)` được đọc **đúng một lần** trong toàn bộ shell (test cấm đọc lại: `mobile/safeArea.test.ts:41-62`).
- `data-standalone` được đặt **trước paint** bằng script inline: `apps/noverse/web/index.html:2` (script inline `if (navigator.standalone === true) documentElement.setAttribute('data-standalone','')`), dự phòng cho khi `display-mode` không khớp (`safeArea.test.ts:67-71`).
- `viewport-fit=cover` + `apple-mobile-web-app-status-bar-style: black-translucent` — `apps/noverse/web/index.html:2`.

### 4.2 Bảng neo — công thức và px đo được

| Neo | Công thức / nguồn | env=0 (đo) | iPhone có notch `[INFERENCE]` |
|---|---|---|---|
| Mép trên khung | `.m-app{padding-top:var(--m-top)}` — `mobile.css:14` | **8 px** | `max(8, 59+0)=59`; iOS 26 đã cài: `59+40=99` |
| Thanh trên | `.m-topbar{height:52px}` — `mobile.css:16` | 52 | 52 |
| Thanh tab đáy | `.m-tabs{bottom:var(--m-tabbar-bottom)}` — `mobile.css:17` | **20 px** | `max(20,34)=34` |
| Cao thanh tab | `--m-tabbar-h:64px` — `mobile.css:6` | 64 | 64 |
| Đệm cuối vùng cuộn | `.m-scroll{padding-bottom:calc(92px + var(--m-tabbar-bottom))}` — `mobile.css:16` | **112 px** | 126 |
| Nút nổi trên thanh tab | `--m-float-bottom:calc(var(--m-tabbar-bottom) + 72px)` — `mobile.css:14` | **92 px** | 106 |
| Footer / thanh hành động | `.m-footer{padding:16px 16px 30px}`; chế độ `hide`: `padding-bottom:max(30px,var(--m-safe-bottom))` — `mobile.css:20` | 30 | 34 |
| Toast | `.m-toast{bottom:var(--m-toast-bottom,calc(var(--kb,0px) + 92px))}` — `mobile.css:21`; `--m-toast-bottom` đặt theo artboard: MWork 214 · MReader 100 · MAudio 168 · MMore 100 · MCmd 112 · MLog/MNotify 30 (`logic.ts:33-41`, áp ở `index.tsx:94`) | 92 (không đặt) | 92 + kb |
| Băng offline | `.m-offline{top:calc(var(--m-top) + 52px)}` — `mobile.css:25` | 60 | 111 / 151 |
| Sheet (kit) | `.m-sheet{padding-bottom:max(26px,var(--m-safe-bottom));max-height:calc(100dvh - var(--m-top) - 12px)}` — `kit.css:144`; bản `md`: `max(8px,var(--m-safe-bottom))` — `kit.css:157` | 26 / 8 | 34 |
| Composer Workspace | `.mw-composer-wrap{bottom:calc(var(--m-tabbar-bottom) + var(--m-tabbar-h))}`; khi gõ `bottom:var(--kb,0px)` — `workspace.css:90-91` | 84 | 98 |
| Footer trình đọc | `.mr-root .m-footer{bottom:calc(var(--m-tabbar-bottom) + var(--m-tabbar-h))}` — `reader.css:39` | 84 | 98 |
| Toast trong trình đọc | `.mfb-toast{top:calc(var(--m-top) + 52px)}` — `reader.css:167` | 60 | 111 |
| Trình phát thu nhỏ | `.ma-mini{bottom:var(--m-float-bottom,92px)}` — `screens/audio/mobile/mobile.css:76` | 92 | 106 |
| Thanh hành động MCmd | `.mc-actions{padding:16px 16px max(30px,env(safe-area-inset-bottom,0px))}` — `screens/cmdresults/mobile/mobile.css:21` | 30 | 34 |
| Nút dính đáy M11 | `paddingBottom:max(26px,env(safe-area-inset-bottom,0px))` — `screens/newstory/mobile/styles.ts:30,100` | 26 | 34 |
| Sticky Cài đặt | `padding:8px 12px calc(8px + env(safe-area-inset-bottom,0px))` — `screens/settings/components.tsx:316` | 8 | 42 |
| Máy tính bảng | `.tablet-safe{height:max(24px,var(--m-top))}` — `tablet/tablet.css:8` | 24 | 59/99 |
| Đăng nhập | `.phone-auth{padding-top:var(--m-top)}`; sheet đăng nhập `bottom:var(--kb,0px)`, `max-height:calc(100dvh - var(--kb,0px) - 52px)` — `screens/auth/mobile/phone.css:1-2` | 8 | 59/99 |
| Bàn phím | `--kb` từ `visualViewport` — `runtime.tsx:80-88` | 0 | 0…~340 |

### 4.3 Luật rút ra (bắt buộc cho cả hai app)

1. **R-SA1** — Chỉ **một** file khai báo `env(safe-area-inset-*)`; mọi nơi khác dùng `var(--m-safe-top/--m-safe-bottom)`. Test phải cấm đọc lại `env(safe-area-inset-top)` ngoài khai báo (`safeArea.test.ts:42-57`).
2. **R-SA2** — Mép trên = `--m-top = max(8px, safe-top + edge)`; **không** cộng dồn, **không** sàn 40 px cho mọi máy (sàn 40 px làm thanh trên thấp ~40 pt trên iPhone — `mobile.css:1-5`).
3. **R-SA3** — `edge = 40px` **chỉ** khi app đã cài trên iOS/iPadOS (standalone, dọc; iPad cả khi ngang) hoặc `navigator.standalone`; đặt `data-standalone` trước paint (`index.html:2` — script inline trong `<head>`, `mobile.css:7-12`).
4. **R-SA4** — Thanh tab: `bottom = max(<gap artboard>, safe-bottom)`, `height = 64`. Noverse `<gap> = 20`; Viderse artboard/spec là **30** (`04-components.md:224`), code đang 4 (`MobileShell.tsx:147`).
5. **R-SA5** — Mọi thứ nổi phía trên thanh tab dùng **cùng gốc**: `calc(var(--m-tabbar-bottom) + var(--m-tabbar-h))` (+ phần riêng). Cấm viết cứng 84/110 px (`safeArea.test.ts:85-89`).
6. **R-SA6** — Vùng cuộn luôn có `padding-bottom` ≥ `tabbar-bottom + tabbar-h + phần nổi` để nội dung cuối không bị viên kính che (`mobile.css:16`).
7. **R-SA7** — Sheet/toast/băng offline neo theo `--m-top` / `--m-safe-bottom`; sheet **không** được cao hơn `100dvh - --m-top - 12px`.
8. **R-SA8** — Không vẽ thanh trạng thái / home indicator giả; artboard vẽ 40 px/26 px là minh hoạ (Viderse #57: `11-hoi-dap.md:436`).
9. **R-SA9** — Khi bàn phím mở, `--kb` thay cho safe-bottom ở mọi thanh dính đáy (`workspace.css:91`, `phone.css:2`).

---

## 5. Bàn phím & visualViewport

- `visualViewport` `resize` + `scroll` → `delta = innerHeight - vv.height - vv.offsetTop`; `delta > 80` mới tính là bàn phím; ghi `--kb` lên `document.documentElement` và state `keyboard` — `runtime.tsx:80-89`.
- `--kb` được dùng bởi: tab mode `hide` (`logic.ts:11`), composer (`workspace.css:91`), toast (`mobile.css:21`), sheet đăng nhập (`phone.css:2`), thanh hành động MCmd (`cmdresults/mobile/mobile.css:21,130`), layout M11 (`screens/newstory/mobile/layout.ts:21-30` — `barOffset` theo `keyboard`).
- Ngưỡng 80 px để không nhầm với thanh địa chỉ co giãn của Safari.

## 6. Vùng chạm & cỡ chữ

| Mục | Noverse | Nguồn |
|---|---|---|
| Nút biểu tượng | 44×44 | `mobile.css:19` |
| Nút chính | cao 50 (min 44), bo 14, chữ 16/600 | `mobile.css:19` |
| Chip | cao 34 **nhưng** `::before{inset:-5px 0}` → vùng chạm 44 | `mobile.css:31` |
| Hàng danh sách | `min-height:54px` | `mobile.css:20` |
| Ô nhập | `min-height:46`, chữ **16** (iOS không tự phóng) | `mobile.css:20`, `mobile.css:24` |
| Nút quay lại | 44 | `mobile.css:16`, `README.md:39` |
| Thân chữ | 15/1.5 (`--m-app`), tiêu đề 20/700, phụ 12.5–13 | `mobile.css:14,16,20` |
| Nhấn | `transform:scale(.96)` cho button/a (trừ switch/radio/checkbox) | `mobile.css:15` |

Spec nền: `18-mobile.md:42` ("Vùng chạm ≥ 44 px; nút chính cao 50, bo 14; chip cao 34; thân 15–16; ô nhập 16").

## 7. Chuyển động

- Nhịp: sheet vào 440 ms `cubic-bezier(.2,.9,.25,1.03)`, ra 260 ms; push 340 ms; toast vào 420 ms; thanh tab 420 ms `transform`; chỉ báo tab 180 ms trượt + keyframe `tind` nảy (`mobile.css:17,21,26`, `kit.css:144-145,164-165`, `platform/ui/src/motion.ts:30-40`).
- `prefers-reduced-motion: reduce` tắt **toàn bộ** animation+transition trong `.m-app`/`.m-overlay` — `mobile.css:26`.
- Không animate `height`/`top`; dùng `transform`/`opacity`; `will-change: transform` chỉ trên thanh tab và chỉ báo (`mobile.css:17`).

## 8. PWA / service worker / manifest

| Hạng mục | Noverse | Nguồn |
|---|---|---|
| Manifest | `standalone`, `#0B0B0D`, icon 192/512 `any` + maskable 512, `lang: vi` | `public/manifest.webmanifest:1-33` |
| apple-touch-icon | 180 px PNG | `index.html:2` |
| SW | chỉ cache manifest + icon + `/assets/*` có MIME hợp lệ, cùng origin, không `no-store/private`, **không** cache HTML/`/api`/phiên | `public/sw.js:1-69` |
| Cache version | `ainovel-mobile-static-v3`, dọn bản cũ khi activate | `sw.js:2,49-55` |
| Push | `job.done` → `showNotification` khi `document.hidden`; đăng ký SW lười, chỉ khi người dùng bật | `mobile/pwa.ts:60-85,131-156`; gọi một lần ở `main.tsx:12` |
| Quyền | màn giải thích trước hộp thoại hệ thống; không xin quyền lúc mở trang | `pwa.ts:13-34`; `18-mobile.md:378` |
| Đăng ký | `registerMobilePWA()` một lần lúc khởi động, có `isSecureContext` | `pwa.ts:131-135` |

## 9. Dữ liệu trên điện thoại

- **Một** luồng SSE `/api/events` cho cả phiên, backoff 1→2→4→5 s (`logic.ts:9-10`), nối lại thì làm mới cache; poll chỉ khi tab hiện và luồng không `open` (`data/README.md:1-3`).
- Cache SWR theo khoá (`@noverse/ui/data`), `useResourceValue` với `select` để chỉ re-render khi giá trị dẫn xuất đổi (`mobile/index.tsx:86-88`).
- **Prefetch sau khung hình đầu**: `requestIdleCallback(warm, {timeout:1500})` nạp trước chapters/chapter đang đọc/snapshot/outline/audio/library/config để mở tab lần đầu là có dữ liệu thật, không skeleton — `mobile/index.tsx:110-146`.
- **Outbox ghi khi offline**: IndexedDB `ainovel.mobile/outbox`, tách theo `account`, chỉ xoá khi POST thành công, tự gửi lại khi `online`/SSE mở lại/đăng nhập — `mobile/outbox.ts:13-31,51-85`; nhãn UI "Chờ gửi · sẽ tự gửi khi có mạng" (`18-mobile.md:90`).
- Nháp đang gõ giữ theo tài khoản trong RAM (`accounts.ts:23`), không vào localStorage.
- Lỗi 401 → `sessionGate` (một cửa duy nhất), request được thử lại **một lần** sau khi đăng nhập (`README.md:53-55`).

## 10. Kỹ thuật hiệu năng (đang dùng)

| Kỹ thuật | Chi tiết | Nguồn |
|---|---|---|
| Giữ màn khi đổi tab | `<Activity>`: state+DOM+cùng vị trí cuộn, effect (SSE/poll/timer) tự dừng | `shell/KeepAlive.tsx:3-12` |
| Tránh render lại pane ẩn | `memo(PaneView)` + `paneSignature` ổn định | `index.tsx:49-66`, `tabs.tsx:80-93` |
| Một listener cuộn | `useMobileScroll` passive, tính trong callback, không listener thứ hai | `runtime.tsx:148-183` |
| Cách ly layout | `.m-scroll{contain: layout style; overscroll-behavior: contain}` | `mobile.css:16` |
| Prefetch | idle callback, dedupe với request đang bay | `index.tsx:110-146` |
| Debounce báo mất mạng | `useSustained(flap, 3000)` để iOS nền→hiện không nháy băng | `runtime.tsx:28-38,99` |
| Skeleton đúng bố cục | không nhảy layout khi dữ liệu về | `MobileReader.tsx:274`, `MobileBrain.tsx:100` |
| Ảnh | avatar/icon dùng `width`/`height` số; thumbnail `loading="lazy"` | `screens/settings/members.tsx:213` |
| Toast không gây layout | `position:fixed` + biến `--m-toast-bottom`, registry theo màn | `toastBottom.ts:9-30` |

## 11. Quy ước test mobile

- File test **cạnh mã**, tên `*.test.ts(x)`; 33 file trong các thư mục `mobile` (trên tổng 214).
- Không dùng testing-library: `createRoot` + `act` thủ công, `// @vitest-environment happy-dom` đầu file — `mobile/tabs.test.tsx:1-30`; `fake-indexeddb` cho outbox (`package.json:30`).
- **Test hợp đồng CSS**: đọc thẳng file CSS/TSX và khẳng định chuỗi — `mobile/safeArea.test.ts:1-95` (một nguồn inset, `--m-top`, thanh tab 64, composer dùng chung gốc, `viewport-fit=cover`).
- Test hành vi thuần tách sang `logic.ts` (`tabs.test.tsx`, `logic.test.tsx`, `runtime.test.ts`, `outbox` qua `sw.test.ts`, `safeArea.test.ts`).
- Luật hiển thị chốt bằng test: `settledText.test.ts`, `landing.test.ts`, `expired.test.tsx`.

---

## 12. So sánh từng điểm với Viderse mobile

Nguồn Viderse: `shell/AppShell.tsx:35,97-105`, `shell/MobileShell.tsx:31-161`, 56 file có `max-width: 767px`, spec `03-tokens.md:64`, `04-components.md:223-233`, `11-hoi-dap.md:436`.

| # | Điểm | Noverse (chuẩn) | Viderse hiện tại | Lệch? |
|---|---|---|---|---|
| 1 | Mốc bố cục | 640 phone / 641–1099 tablet (`logic.ts:5-6`) | 767 phone, không tablet (`AppShell.tsx:35`; test chốt một mốc `design-rules.test.ts:224-239`) | **Khác có chủ đích**, nhưng kit Toast dùng 759 (`Toast.tsx:123,201,418`) → 3 mốc trong hệ |
| 2 | Cấu trúc | cây điện thoại riêng (`screens/**/mobile`) | nhánh `mobile ?` trong chính màn desktop (56 file) | Có — nguồn gốc của "không giống artboard" |
| 3 | Route→tab | `paneKeyOf` + `phoneRedirect` | `navSection` + tab href cứng (`MobileShell.tsx:22-28`) | Không có redirect route desktop→mobile |
| 4 | Giữ màn khi đổi tab | `<Activity>` + pane snapshot (`KeepAlive.tsx:10`) | **không có**; `Screen` unmount (`App.tsx:44-73`) | **P1** |
| 5 | Vị trí cuộn khi quay lại tab | giữ (pane alive) | **mất** — đo: `scrollTop 500 → 0` khi Khám phá → Duyệt → Khám phá | **P1** |
| 6 | Nạp lại dữ liệu khi đổi tab | prefetch + cache, 0 request mới | **gọi lại API**: +14 resource, có `/api/rivals/all/videos`, `/api/rivals/…/scan`, `/api/rivals/…/posting-slots` (đo trên production) | **P1** |
| 7 | Prefetch sau khung hình đầu | có (`index.tsx:110-146`) | không | P1 |
| 8 | Tab mode | `default/mini/hide` (`logic.ts:11-13`) | **không có** — spec yêu cầu (`04-components.md:228`, `artboards/kit.css:442-443`) nhưng code không có `.is-mini`/`data-tab-mode` | **P1** |
| 9 | Ẩn tab khi bàn phím | `visualViewport` → `--kb` → `hide` | **không có** `visualViewport` trong toàn app | **P1** |
| 10 | Thanh tab: vị trí đáy | `max(20px, safe-bottom)` = 20 (env 0) | `calc(4px + safe-bottom)` = **4** (đo); artboard/spec **30** | **P1** (off-artboard) |
| 11 | Thanh tab: cao/bo/kính | 64 / 32 / blur 26+saturate 180 | 64 / 32 / giống (`MobileShell.tsx:147`) | Khớp |
| 12 | Đệm cuối vùng cuộn | `calc(92px + tabbar-bottom)` = 112 (`mobile.css:16`) | `--vv-tabbar-pad: 110px` cứng (`viderse.css:247`); artboard/spec đòi **118px** (`04-components.md:229`) | P2 (không suy ra từ safe-area) |
| 13 | Mép trên | `--m-top = max(8, safe+edge)` | `paddingTop: env(safe-area-inset-top)` (`MobileShell.tsx:134`) | P2 — thiếu sàn 8 px & `--m-edge` (dải mờ iOS 26) |
| 14 | Nguồn inset | 1 khai báo + test cấm đọc lại | `env()` rải rác: `MobileShell.tsx:134,147`, `queueMobile.stylex.ts:70`, `NewScreen.tsx:195`, `settings.stylex.ts:644` | **P1** |
| 15 | Sheet & safe-area | `padding-bottom:max(26px,--m-safe-bottom)`; `max-height:calc(100dvh - --m-top - 12px)` (`kit.css:144,157`) | **đo thật: `padding-bottom: 0px`**, `--m-safe-bottom`/`--m-top` **không tồn tại** trong Viderse → khai báo invalid | **P0** (nội dung sheet chạm home indicator; sheet `lg` mất trần chiều cao) |
| 16 | Toast khi có thanh tab | `--m-toast-bottom` theo route (registry, `toastBottom.ts:9-30`) | `ToastHost` dùng `--m-toast-bottom` **không được định nghĩa** → fallback 96 px (`Toast.tsx:427`) ⇒ trên thanh tab 64+4 px, viên toast cách đỉnh thanh tab 28 px (đo: đỉnh thanh tab 776, đáy toast 748); spec đòi nằm trên thanh đáy **12 px** (`04-components.md:247`). Toast việc nền (`ActivityToastHost`) lại có neo riêng `--vv-toast-bottom` do `ToastAnchor` đo DOM (`toastAnchor.ts:9-24`, `Toast.tsx:404`) | P2 — hai cơ chế neo song song, không có luật neo theo thanh tab |
| 17 | Băng mất kết nối | băng nổi + đếm ngược + "Thử ngay", debounce 3 s, nội dung mờ 55% | `OfflineStrip` in-flow (đẩy nội dung), không đếm ngược, mờ 42% (`AppShell.tsx:63,102`; `OfflineStrip.tsx:13-78`) | P1 |
| 18 | Vùng chạm | 0 phần tử < 44 px ở màn Thêm (đo) | 4 phần tử < 44 px (đo màn Cài đặt): logo `<a>` 36 (`MobileShell.tsx:110-113`, **không** có halo), `JobsPill` 36 (có `roundTouch` 44 — `JobsPill.tsx:142`), avatar 36 (có `touchHalo` — `MobileShell.tsx:119`), link “Thành viên · ở N” 39 | P2 — 2 chỗ thiếu halo (logo, link chữ) |
| 19 | Halo chạm cho avatar 36 | không áp dụng (avatar 44) | có `touchHalo` 44 chỉ khi `(pointer: coarse)` (`MobileShell.tsx:132,146`) | Đạt — nên đưa vào chuẩn |
| 20 | Cỡ chữ thân | 15/22.5 (đo) | 14/21.7 (đo) | P2 |
| 21 | Nhấn `.96` | toàn cục (`mobile.css:15`) | chỉ nút FAB (`MobileShell.tsx:158`) | P2 |
| 22 | Reduced motion | tắt toàn bộ trong `.m-app/.m-overlay` (`mobile.css:26`) | tắt toàn cục (`viderse.css:401-411`) + hằng `NO_MOTION` | Khớp |
| 23 | PWA cache | SW cache asset có kiểm MIME (`sw.js:1-69`) | SW chỉ có `notificationclick`, không cache (`public/sw.js:1-12`); đăng ký không kiểm `isSecureContext` (`main.tsx:14`) | P2 |
| 24 | Offline write | outbox IndexedDB | không có | P1 (nếu Viderse có thao tác ghi trên mobile) |
| 25 | Mã chia nhỏ | màn hình thường, prefetch bù | `React.lazy` từng route (`App.tsx:20-42`) | **Đạt — Viderse tốt hơn** |
| 26 | Test mobile | CSS contract + DOM + logic (33 file) | `matchMedia` mock + vài test màn (`shell.test.tsx:27-35`, `MobileQueue.test.tsx`) | P2 — thiếu test hợp đồng CSS |
| 27 | Điều hướng tab | `button` + `navigateNow` (tránh iOS nháy) | `<a href="#…">` (`MobileShell.tsx:36-53`) | P2 |
| 28 | Safe-area test | `safeArea.test.ts` chốt 1 nguồn | chỉ test mốc 767 (`design-rules.test.ts:224`) | P2 |

### 12.1 Danh sách luật Viderse **thiếu** (xếp theo mức)

- **P0** — Hợp đồng biến safe-area của kit (`--m-safe-bottom`, `--m-top`) không được định nghĩa ⇒ sheet mất padding đáy, mất `max-height`.
- **P1** — Không giữ màn (unmount + refetch mỗi lần đổi tab); mất vị trí cuộn.
- **P1** — Không có `mini`/`hide` cho thanh tab (spec có, code không).
- **P1** — Không có `visualViewport`/`--kb` ⇒ bàn phím che composer/thanh dính đáy.
- **P1** — `env()` đọc ở 5 nơi, không có biến trung gian; thanh tab 4 px thay vì 30 px.
- **P2** — Toast chưa neo theo thanh tab trên mobile (fallback 96 px, lệch 16 px so với 12 px của spec).
- **P1** — Băng offline kiểu desktop (in-flow), không đếm ngược/không debounce.
- **P2** — Hai vùng chạm < 44 px không có halo: logo `<a>` 36 px ở thanh trên và link chữ 39 px trong Cài đặt.
- **P2** — Cỡ chữ thân 14 thay vì 15–16; nhấn `.96` chỉ ở FAB; SW không cache; `<a>` thay `button` cho tab.

---

## 13. Catalogue component dùng chung (kit vs app-local)

### 13.1 Đã ở `platform/ui` (dùng được ngay cho cả hai)

| Component | File | Ghi chú mobile |
|---|---|---|
| `Sheet` | `platform/ui/src/Sheet.tsx:32` | đã có `size='md'` cho artboard Viderse, `homeIndicator`, `subtitle`; **phụ thuộc `--m-safe-bottom`/`--m-top`** (kit.css:144,157) |
| `ToastHost` / `toast` | `Toast.tsx:415,427` | đọc `--m-toast-bottom` (đã sẵn cho Noverse), nhánh `activity` cho Viderse, mốc 759 px |
| `Dialog`, `ConfirmDialog` | `Dialog.tsx`, `ConfirmDialog.tsx` | Viderse dùng `confirm()`; Noverse dùng `ConfirmSheet` |
| `Skeleton`, `EmptyState`, `Spinner`, `Shimmer`, `ProgressBar` | `feedback.tsx:1-113` | cả hai dùng |
| `Chip`, `Badge`, `Card`, `CardTitle` | `Chip.tsx:1-130` | Noverse bọc thêm `.m-chip` (34 px + halo 44) |
| `Tabs`, `Segmented` | `Tabs.tsx` | dùng ở cả hai |
| `Field`, `TextField`, `Selects`, `Switch`, `Radio` | `Field.tsx`, `Inputs.tsx`, `Selects.tsx`, `controls.tsx` | ô nhập mobile cần ép 16 px |
| `Banner`, `StaleBanner`, `ConflictBar`, `SlowLoadingNotice` | `Banner.tsx`… | trạng thái |
| `Icon`, `AgentOrb`, `BrandMark` | `Icon.tsx`… | — |
| `MOTION` | `motion.ts:24-40` | nhịp sheet/toast/tab dùng chung |
| `cache`/`createEventsClient` | `data/*` | **đã chia sẻ thật** — Viderse `data/useResource.ts:3` và `data/events.ts:8` import từ `@noverse/ui/data` |

### 13.2 Chỉ có ở Noverse (ứng viên đưa lên kit)

| Component / hook | Nguồn | Viderse có? | Đích đề xuất | Slots / API |
|---|---|---|---|---|
| `KeepAlive` (`<Activity>`) | `shell/KeepAlive.tsx:10` | không | `@noverse/ui/mobile` → `KeepAlive` | `{active, children}` |
| `MobilePanes` | `mobile/tabs.tsx:157` | không | `MobilePanes` | `{panes: [id, node][], active}` |
| `MobileTabBar` | `mobile/tabs.tsx:110` | có bản riêng | `MobileTabBar` | `{items[{key,label,icon,iconActive,href,badge,dot,disabled}], index, mode, onSelect, centerAction?}` |
| `paneKeyOf`/`phoneRedirect` | `mobile/tabs.tsx:28,61` | không | `usePhonePanes(routeMap)` | map do app truyền |
| `MobilePage` (topbar 52 + scroll + footer) | `mobile/primitives.tsx:20` | không (mỗi màn tự dựng) | `MobilePage` | `{title, backTitle, onBack, actions, footer, manageScroll, headerSlot}` |
| `ActionSheet` / `ConfirmSheet` | `primitives.tsx:34,43` | không | `ActionSheet`, `ConfirmSheet` | `{title, description, actions[{label,tone,onSelect}]}` |
| `PushPage` | `primitives.tsx:65` | không | `PushPage` | `{title, previousTitle, onBack}` |
| `useMobileToast` + `useToastBottom` + `toastBottomFor` | `primitives.tsx:8`, `toastBottom.ts:14` | kit fallback | gộp vào `ToastHost` (đã đọc `--m-toast-bottom`) | `useToastBottom(px, priority)` |
| `useMobileScroll` | `runtime.tsx:148` | không | `useMobileScroll` | `{down, immersive, reveal}` |
| `useTabMode` + `tabModeFor` | `runtime.tsx:130`, `logic.ts:11` | không | `useTabMode` | `TabMode` |
| `visualViewport` → `--kb` | `runtime.tsx:80-89` | không | `useVisualViewportKeyboard()` | trả `{keyboard}` + ghi `--kb` |
| Bộ biến safe-area (`mobile.css:6-13`) + `safeArea.test.ts` | `mobile/mobile.css`, `mobile/safeArea.test.ts` | không | `@noverse/ui/mobile.css` (hoặc `kit.css`) | biến `--m-*` + test mẫu |
| Băng offline có đếm ngược | `runtime.tsx:99,126` | có bản desktop | `OfflineBanner` | `{seconds, onRetry, dim}` |
| `useSustained` | `runtime.tsx:28` | không | `useSustained(value, ms)` | — |
| Outbox IndexedDB | `mobile/outbox.ts` | không | `createOutbox({dbName, store})` trong `@noverse/ui/data` | `enqueue/drain/useOutbox` |
| `registerMobilePWA` | `mobile/pwa.ts:131` | có SW riêng | `registerPWA({swUrl, cache})` | — |

### 13.3 Chỉ có ở Viderse (giữ app-local, nhưng phải theo chuẩn chung)

| Component | Nguồn | Ghi chú |
|---|---|---|
| `MobileShell` (bar 52 + tabbar 5 mục + FAB đỏ) | `shell/MobileShell.tsx:71-130` | bản sắc Viderse; chỉ nên tách phần **khung** ra kit |
| `MobileTabBar` Viderse (FAB giữa, badge đỏ) | `shell/MobileShell.tsx:31-68` | trùng chức năng với Noverse → một component + `centerAction` slot |
| `ChannelSwitcher` (mobileBar) | `shell/ChannelSwitcher.tsx` | Viderse-specific (khái niệm kênh) |
| `JobsPill` (mobileBar) | `shell/JobsPill.tsx` | Viderse-specific (khái niệm việc nền) |
| `ActivityPopover`, `ActivityBridge`, `notificationList` | `shell/*` | Viderse-specific |
| `ToastAnchor` | `shell/toastAnchor.ts:9` | nên gộp vào `ToastHost` như Noverse (`useToastBottom`) |
| `queueMobile.stylex.ts` | `screens/queue/queueMobile.stylex.ts` | Viderse-specific |
| `OfflineStrip` (desktop) | `shell/OfflineStrip.tsx:13-78` | giữ cho desktop; mobile dùng `OfflineBanner` chung |

### 13.4 Trùng lặp cần gộp (cùng một thứ, hai bản)

| Thứ | Noverse | Viderse | Kết luận |
|---|---|---|---|
| Thanh tab dưới | `.m-tabs` + `MobileTabBar` | `.vv-tabbar` + `MobileTabBar` | **1 component kit** + token màu theo app |
| Thanh trên | `.m-topbar` (52, tiêu đề 20) | `.vv-mbar` (52, logo+switcher+pill+avatar) | **1 khung** `MobileAppBar` với `leading`/`center`/`trailing` slots |
| Sheet | `BottomSheet` (wrapper kit) | `Sheet size="md"` trực tiếp | giữ `Sheet` kit; thêm `ActionSheet`/`ConfirmSheet` |
| Toast | `MobileFeedback` (tự viết) | `ToastHost` kit + `ToastAnchor` | **chỉ dùng `ToastHost`** + `--m-toast-bottom` |
| Chips | `.m-chip` (34 + halo) | `.vv-chip`/StyleX | `Chip` kit + `density="mobile"` |
| Hàng/card | `.m-row`/`.m-card`/`.m-group` | `vq-*`, settings rows | `ListRow`/`Card`/`Group` kit |
| Băng offline | `.m-offline` | `OfflineStrip` | `OfflineBanner` (mobile) + `OfflineStrip` (desktop) |
| Nút nhấn | `.96` toàn cục | chỉ FAB | luật toàn cục trong kit CSS mobile |
| Mốc bề ngang | 640 (+tablet 641–1099) | 767 | **một hằng số** dùng chung (`--m-bp-phone` + hằng TS) — hoặc ghi rõ hai mốc hợp lệ |

---

## 14. Checklist chuẩn mobile dùng chung

### A. Đã chia sẻ (giữ nguyên, không làm lại)

- [x] `platform/ui` kit: `Sheet`, `ToastHost`, `Dialog`, `Skeleton/EmptyState/Spinner`, `Chip/Card/Badge`, `Tabs/Segmented`, `Field/Inputs/Selects/controls`, `Banner/StaleBanner/ConflictBar`, `Icon/AgentOrb/BrandMark`, `MOTION`.
- [x] `@noverse/ui/data`: cache SWR theo khoá, `createEventsClient` (SSE + backoff 1→2→4→5 + `Last-Event-ID`), `useResource` (Viderse `data/useResource.ts:3`).
- [x] Token StyleX (`tokens.stylex`) + brand riêng mỗi app.
- [x] `prefers-reduced-motion` toàn cục ở cả hai app.

### B. Noverse-only — phải đưa lên kit rồi Viderse dùng lại

1. **Hợp đồng biến safe-area** (`--m-safe-top/-bottom`, `--m-tabbar-h/-bottom`, `--m-top`, `--m-edge`) + test cấm đọc lại `env()` (đây là điều kiện để kit `Sheet`/`Toast` hoạt động đúng ở Viderse).
2. `KeepAlive` (`<Activity>`) + `MobilePanes` + `paneKeyOf/keepPane` → hết unmount/refetch khi đổi tab.
3. `MobileTabBar` (3 mode, `--tab-index` lạc quan, dot, `button`+navigate) + `useTabMode`.
4. `MobilePage` (topbar 52 / scroll / footer 30+safe).
5. `ActionSheet` + `ConfirmSheet` (nhóm iOS, Promise, khoá khi chờ).
6. `useToastBottom` + `toastBottomFor` (neo toast theo màn, ưu tiên) — nối vào `ToastHost` đã có.
7. `useVisualViewportKeyboard` (`--kb`, ngưỡng 80 px).
8. `useMobileScroll` (một listener, `down>40`, `immersive`, `reveal`).
9. `OfflineBanner` (đếm ngược + Thử ngay + `useSustained(3000)` + mờ 55%).
10. `createOutbox` (IndexedDB theo account, chỉ xoá khi thành công).
11. `registerPWA` (SW cache asset có kiểm MIME, `isSecureContext`, đăng ký lười).
12. Quy ước test: 1 file `safeArea`-style contract test + `*.test.tsx` cạnh mã + `happy-dom`.

### C. Viderse-specific (giữ, nhưng phải khớp artboard + chuẩn)

- 5 mục tab với FAB đỏ giữa (`--vv-red-button-fill`), badge Duyệt vàng, icon `-b` khi active — chỉ thay *khung* bằng kit.
- `ChannelSwitcher`, `JobsPill`, `ActivityPopover`/`ActivityBridge`, `QueueSummarySheet`, `queueMobile.stylex.ts`.
- Mốc 767 px (nếu giữ) phải là **hằng số duy nhất** và kit không được dùng 759.
- Bảng màu/typography Viderse (`--vv-fs-*`, `--vv-m-*`) — nhưng thân chữ mobile nên 15–16 như Noverse.

### D. Luật bắt buộc cho mọi màn mobile (áp cho cả hai app)

1. Khung: `height:100dvh`, `padding-top: var(--m-top)`, `overflow:hidden`, nội dung cuộn trong **một** vùng.
2. Thanh tab: cao 64, `bottom: max(<gap>, var(--m-safe-bottom))`, 3 chế độ; ẩn khi bàn phím mở.
3. Vùng cuộn: `padding-bottom ≥ tabbar-bottom + 64 + phần nổi`.
4. Sheet: dùng kit `Sheet`; `padding-bottom: max(26px, var(--m-safe-bottom))`; `max-height: calc(100dvh - var(--m-top) - 12px)`; kéo >30% đóng; Esc/backdrop đóng.
5. Toast: `position:fixed`, đọc `--m-toast-bottom` (giá trị theo artboard 30–214 px, fallback 92 / `--kb + 92`), tự tắt 2,5 s.
6. Vùng chạm ≥ 44 px (chip 34 + halo; avatar 36 + halo khi `pointer: coarse`).
7. Ô nhập 16 px; thân 15–16/1.5; tiêu đề màn 20/700 (Noverse) hoặc theo artboard.
8. Nhấn `.96`; chuyển động chỉ `transform`/`opacity`; `prefers-reduced-motion` tắt hết.
9. Không vẽ thanh trạng thái/home indicator giả.
10. Loading = skeleton đúng bố cục; empty = `EmptyState` + đúng một lối đi; lỗi = câu thật + Thử lại; offline = băng + đếm ngược + nội dung mờ, thao tác ghi xếp outbox.
11. Đổi tab: pane giữ-alive, 0 request mới nếu cache còn; prefetch idle cho tab kế tiếp.
12. Mỗi màn có test hợp đồng CSS/khung (một nguồn safe-area, mốc bề ngang, vùng chạm).

---

## 15. Chưa quan sát được / cần chốt

- `env(safe-area-inset-*)` trên máy thật (iPhone 14/15) chưa đo được trong Chromium: mọi px "có notch" ở §4.2 là `[INFERENCE]` từ công thức.
- Bàn phím thật trên iOS (chiều cao `--kb`, hành vi `visualViewport`) — chưa quan sát; chỉ đọc mã.
- Viderse: trạng thái "đang gửi khi offline" không tồn tại nên không quan sát được.
- Dữ liệu production của `qaaudit` nghèo (0 kênh ở Viderse, truyện 0 chương ở Noverse) → các màn nhiều dữ liệu (Hàng chờ, Duyệt, Đọc chương) chưa đo được bằng mắt; kết luận dựa trên mã + đo khung.
- Mốc bề ngang hợp nhất (640 vs 767) là **quyết định thiết kế**, cần chủ dự án chốt; báo cáo chỉ nêu sự kiện 3 mốc đang tồn tại.

---

Status: DONE

Summary: Đã dựng chuẩn mobile Noverse as-built (cây phone riêng, tab/pane `<Activity>`, primitives, safe-area một nguồn, `--kb`, outbox, prefetch, PWA cache, test hợp đồng CSS) với file:line; so 28 điểm với Viderse (3 P0/P1 nổi bật: sheet mất safe-area do thiếu `--m-safe-bottom`/`--m-top`, mất keep-alive + refetch mỗi lần đổi tab, thiếu mini/hide + `--kb`), kèm catalogue component (kit vs app-local vs trùng lặp, tên đích + slots) và checklist 4 nhóm (đã chia sẻ / Noverse-only cần đưa lên / Viderse-specific / luật bắt buộc).

Concerns: px "có notch" là suy luận (Chromium env=0); dữ liệu production của `qaaudit` nghèo nên các màn nhiều dữ liệu chưa đo bằng mắt; mốc 640 vs 767 cần chủ dự án chốt.
