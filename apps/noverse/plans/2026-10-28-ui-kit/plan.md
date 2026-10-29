# Kế hoạch: chuẩn hoá UI Kit thành gói `@noverse/ui` tái sử dụng

Ngày: 2026-10-28. Mục tiêu: gom design system (token, primitive, icon, motion, a11y) vào một gói npm workspace `web/packages/ui`, app chỉ tiêu thụ qua tên gói; gói không được import ngược vào app; không đổi một pixel nào của app.

## 1. Hiện trạng (kết quả audit)

- `web/src/ui/`: ~30 component/hook + 6 gallery + `kit.css`/`Toast.css` + barrel `index.ts`.
- Token: `web/src/styles/tokens.stylex.ts` (616 dòng, 25 nhóm `defineVars`) là nguồn duy nhất; CSS cũ đọc `var(--…)` StyleX sinh ra. `--m-*` của mobile là token shell riêng (không phải kit).
- Icon: `web/src/assets/icons/*.svg` (~135 file), `<Icon name>` phân giải qua `import.meta.glob` + mask-image; `Icon.test.tsx` quét mã nguồn chặn tên thiếu.
- Code gắn app trong primitive (phải tách): chuỗi tiếng Việt cứng trong Selects (17), ConfirmDialog (7), QrCode (7), ScrollTopButton, BrandMark, Toast, Dialog, Controls, Inputs, Menus, Overlays, Tabs, feedback, SlowLoadingNotice, StaleBanner, ConflictBar, Icon (chuỗi dev); `ConfirmDialog` dùng `useAction` của `data/`; `useUnsavedGuard` gọi `router` + `confirmPresets`; `ConnectionBanner` dùng session/data + i18n; `PanelRefreshBar` dùng `data/cache`; `ModelContextChoices`, `CharAvatar/KindChip` dùng `api/types` + `lib/`; `agentPalette` chứa tên tiếng Việt.
- Trùng lặp: `BottomSheet/ActionSheet/ConfirmSheet/MobilePage` ở `mobile/primitives.tsx` — shell mobile, gắn `runtime`/`logic` và CSS `.m-*`; giữ trong app (kit cung cấp `Icon`, `useEscapeLayer` cho nó). Không có bản Button/Badge/Segmented thứ hai ngoài kit.
- Màu cứng còn lại nằm chủ yếu trong `mobile.css`/css màn (bảng chốt hardcodedColors bao trọn `src`).

## 2. Kiến trúc đích

`web/packages/ui` — workspace npm của `web/` (thêm `"workspaces": ["packages/ui"]`), `name: "@noverse/ui"`, ESM, `sideEffects: ["*.css"]`.

```
packages/ui/
  package.json            exports "." (js+d.ts), "./tokens", "./gallery", "./kit.css", "./tokens.css", "./tokens.json"
  vite.config.ts          build lib (ESM, external react/@stylexjs/stylex)
  tsconfig.build.json     tsc --emitDeclarationOnly → dist/*.d.ts
  src/
    tokens.stylex.ts      ← chuyển nguyên từ styles/ (nguồn duy nhất, 25 nhóm vars)
    tokens.css            SINH RA từ tokens.stylex.ts (script gen-tokens.mjs) — cho consumer không StyleX
    tokens.json           SINH RA cùng lúc — export dữ liệu
    themes.ts             slot theme: registry { dark: <vars mặc định> }, data-theme hook
    kit.css, Toast.css    ← chuyển nguyên
    icons/*.svg           ← chuyển nguyên từ src/assets/icons
    iconNames.ts          SINH RA: union tên icon (script gen-icons.mjs)
    Icon.tsx              name: IconName (union, hở — string vẫn hợp lệ)
    labels.ts             KitLabels mặc định tiếng Anh + setKitLabels(); app nạp tiếng Việt 1 lần
    primitives: Button, controls, Chip, Tabs, Field, Inputs, Selects, Menus, Overlays,
                Dialog, Toast, feedback, Scroll, Banner, StaleBanner, ConflictBar,
                SlowLoadingNotice, ScrollTopButton, QrCode, ProgressRing, ThinkingOrb,
                AgentOrb, BrandMark, ConfirmDialog (+ confirm()), ReviewIssue
    a11y/hook: useEscapeLayer/pushEscapeLayer, useSaveShortcut, useAsyncAction, useScrollTop
    motion.ts             preset easing/thời lượng (motion token + cubic-bezier dùng chung)
    hardcodedColors.*     ratchet riêng của gói (chỉ đi xuống)
```

- App giữ lại ở `web/src/ui/`: `confirmPresets`, `useUnsavedGuard` (gọi `router` + preset vi), `PanelRefreshBar`, `ConnectionBanner`, `ModelContextChoices`, `CharAvatar/KindChip/KindLegend`, `agentPalette`, và toàn bộ `*Gallery.tsx` + `KitGallery` (route `#/dev/kit` — playground dev-only; chọn gallery sẵn có thay Storybook vì Storybook thêm stack, config và hai hệ render; gallery đã là trình diễn đầy đủ).
- Chuỗi tiếng Việt của kit dồn vào `web/src/ui/kitLabels.vi.ts`, nạp bằng `setKitLabels()` ở `main.tsx`. Component đọc `getKitLabels()` tại render (không phải context) — mặc định gói là tiếng Anh.
- `ConfirmDialog` thay `useAction` bằng `useAsyncAction` nội bộ gói (cùng hành vi pending/busy); `useUnsavedGuard` ở app dùng `useNavigationGuard` mới của gói (hook thuần, nhận hàm prompt).

## 3. Pha thực hiện

- **Pha 0 — Kiểm thử đỏ trước:** test ranh giới (gói cấm import `src/*` của app; app không import `packages/ui/src` trực tiếp hay đường `./ui/…` cũ), test token parity (`tokens.stylex.ts` ↔ `tokens.css` ↔ `tokens.json` — khóa và giá trị khớp tuyệt đối), consumer smoke `scripts/smoke-consumer.mjs` (pack tarball → app `/tmp` Vite cài và render Button/Sheet/Icon + theme). Cả ba phải fail trước khi di cư.
- **Pha 1 — Khung gói:** package.json/vite/tsconfig/scripts gen; workspace + alias `resolve.alias` (`@noverse/ui` → `packages/ui/src`, `@noverse/ui/tokens` → `tokens.stylex.ts`, `@noverse/ui/gallery` → `gallery/KitGallery`) trong `vite.config.ts` + `tsconfig.paths`.
- **Pha 2 — Token + CSS + icon:** chuyển `tokens.stylex.ts`, `kit.css`, `Toast.css`, `icons/`; gen `tokens.css/tokens.json/iconNames.ts`; sửa import `styles/tokens.stylex` của app → `@noverse/ui/tokens`; `styles/tokens.css` chỉ còn font/base (đổi tên vai trò trong nhận xét); `motion.test.ts`/`hardcodedColors` cập nhật phạm vi quét.
- **Pha 3 — Primitive thuần:** chuyển Icon, Button, Chip, controls, Tabs, Field, Inputs, feedback, Scroll, ProgressRing, ThinkingOrb, AgentOrb; sửa import nội bộ `../styles/tokens.stylex` → `./tokens.stylex`.
- **Pha 4 — Tách khớp:** labels cho Selects/Toast/Dialog/controls/Inputs/Menus/Overlays/Tabs/feedback/banner/hai-bar/ScrollTop/QrCode/BrandMark/ConfirmDialog/Icon-dev; `kitLabels.vi.ts` + `setKitLabels` ở `main.tsx`; `useAsyncAction`; `useNavigationGuard`; `useEscapeLayer`/`useScrollTop`/`useSaveShortcut` → nhóm a11y; `OverlaysGallery` bỏ `paths` (điểm vào gallery nhận callback).
- **Pha 5 — Cắt dây hoàn toàn:** viết lại toàn bộ import `../ui/<x>`/`./ui/<x>` của app → `@noverse/ui` (cho phần đã chuyển) bằng codemod AST; `ui/index.ts` chỉ còn export hàng app; sửa `#/dev/kit` render `KitGallery` từ `@noverse/ui/gallery` đúng ý dev-only; cập nhật `Icon.test`, `escapeLayers.test`, các test colocated chuyển theo file.
- **Pha 6 — Đóng gói + tài liệu:** `npm install` (workspace link), `vite build` + `tsc --emitDeclarationOnly`, `npm pack`; README.md hướng dẫn dùng (cài, import token/component/theme, labels); version 0.1.0.
- **Pha 7 — Xác minh:** vitest đủ (gồm 3 test mới xanh), `tsc -b --noEmit`, smoke consumer chạy thật, đo lại màn hình 1440/390 so khớp baseline `/tmp/uikit-baseline-*.json`.

## 4. Ràng buộc

- Không shim/re-export: mọi `import` cũ được viết lại; file nguồn cũ xoá hẳn.
- `hardcodedColors` chỉ đi xuống (app) / khởi đầu đúng trạng thái (gói).
- Không chuỗi tiếng Việt cứng trong `packages/ui/src` (trừ chú thích + gallery? — chuẩn: literal không; chú thích giữ tiếng Việt hiện hữu).
- StyleX: consumer cuối không cần plugin StyleX — gói xuất JS đã biên dịch + CSS; app dev đọc source qua alias (HMR giữ nguyên).
