# Dựng Viderse trong verse

Bộ thiết kế này được viết khi Viderse còn là giao diện mới của **yt-factory** (Next.js, SQLite/drizzle, Electron, CLI `ytf`). Từ 29/10/2026, Viderse được dựng lại trong monorepo **verse** ở `apps/viderse`: Go + Postgres, web React trên kit `@noverse/ui` (`platform/ui`). Tài liệu này nói cách đọc bộ thiết kế trong bối cảnh mới.

## 1. Cái gì vẫn là chuẩn, cái gì phải dịch

| Phần | Trong verse |
|---|---|
| Hành vi, luồng, trạng thái, luật chạy ([01-ia-flows.md](01-ia-flows.md)) | **Chuẩn, giữ nguyên.** |
| Giao diện: artboard, [04-components.md](04-components.md), [05-kit.md](05-kit.md), [03-tokens.md](03-tokens.md), [06-copy.md](06-copy.md) | **Chuẩn, giữ nguyên.** Dựng bằng component của `@noverse/ui` theo §3, không chép lớp `.vv-*`. |
| Dữ liệu UI cần ([07-data-contract.md](07-data-contract.md)) | **Chuẩn về hình dạng dữ liệu** (trường, trạng thái, sự kiện). Đường dẫn API giữ dạng `/api/...` nhưng viết trong `apps/viderse/internal` và đăng ký qua `platform/webapp` (`Handle`). |
| Gói việc ([08-dev-prompt.md](08-dev-prompt.md)) | **Việc FE và tiêu chí nghiệm thu dùng được.** Phần “Việc backend” nhắc tới đường dẫn code yt-factory (`src/server/...`, `src/app/...`, drizzle, Electron, `ytf`): đó là nơi logic đang chạy ở bản cũ, dùng làm **bản tham chiếu khi port** sang Go; không phải nơi viết code mới. |
| [09-migration-yt-factory.md](09-migration-yt-factory.md) | Bảng route và thành phần của bản yt-factory. Trong verse dùng bảng route ở §4. |

Chỗ nào code verse làm khác thiết kế: ghi vào [11-hoi-dap.md](11-hoi-dap.md) trước khi làm, không tự đổi.

## 2. Brand

Viderse dùng nền tối của kit Noverse, chỉ đổi sắc thương hiệu (README đợt 19): **tím Noverse → đỏ Viderse**, **đỏ/hồng lỗi Noverse → cam Viderse**. Gắn bằng `defineBrand` ở `apps/viderse/web/src/main.tsx`:

```ts
applyBrand(defineBrand({
  name: 'viderse',
  primary: '#FF4D63',                 // --vv-accent: đang chọn, focus, tick, tab, link nhấn
  status: {
    success: '#A6EDC1',               // --vv-st-ready
    warning: '#F6CB8C',               // --vv-st-review (vàng: tới lượt bạn)
    danger:  '#FF9A5C',               // --vv-danger (cam: lỗi, xoá)
    info:    '#B9D0F7',               // --vv-st-working
  },
  roles: { accentDeep: '#E6002E' },   // --vv-accent-strong: công tắc bật (--kit-switch-on)
}))
```

- Hiện code đang dùng `primary: '#F43F5E'` (bản dựng khung). Đổi sang bộ trên để khớp artboard; ghi ở [11-hoi-dap.md](11-hoi-dap.md) #1.
- **Nút chính là nút sáng** (`Button variant="primary"` của kit, nền `#EDEDEF`). Đỏ không làm nền nút, trừ nút `+` tròn ở thanh dưới điện thoại.
- Nền, mặt, chữ của Viderse trùng giá trị với token kit (`--vv-bg` = `--color-bg`, `--vv-surface-1` = `--color-surface-1`, `--vv-text` = `--color-fg`, `--vv-text-2` = `--color-fg-2`, `--vv-text-3` = `--color-fg-3`): dùng thẳng token kit.
- Logo, biểu tượng app, favicon: [12-brand.md](12-brand.md).

## 3. Component Viderse → `@noverse/ui`

Tên bên trái theo [05-kit.md](05-kit.md); board `Kit` / `MKit` vẽ từng trạng thái.

| Viderse | `@noverse/ui` | Ghi chú |
|---|---|---|
| `Button` (`primary` · `outline` · `quiet` · `danger-soft` · `danger`) | `Button` (`variant`: `primary` · `ghost` · `icon` · `dsoft` · `danger`; `size`: `sm` · `md`) | `outline` → `ghost`; `danger-soft` → `dsoft` (cam nhạt); `danger` → `danger` (cam đặc, chỉ trong hộp xác nhận xoá vĩnh viễn). `quiet` chưa có biến thể riêng: dùng `ghost` không viền — xem hoi-dap #2. |
| `ConfirmDialog` (7 tình huống) | `ConfirmDialog` / `confirm()` (`tone`: `normal` · `warn` · `dsoft` · `danger`; gõ để xác nhận, giữ để xác nhận `HOLD_DURATION`) | Danh sách hệ quả, so hai bản: phần thân `body`. |
| `AccountMenu`, `RowMenu` | `ActionMenu`, `ContextMenu` | Menu con lật trái như Noverse. |
| `Select` · `RichSelect` · `Combobox` · `MultiSelect` | `KitSelect` · `RichSelect` · `Combobox` · `MultiCombobox` | |
| `Input` · `Field` · `Textarea` | `TextField`, `Field`, `FieldLabel`, `FieldError`, `PasswordField`, `TextArea` | |
| `Stepper` · `Slider` | `NumberStepper` · `Slider` | |
| `Checkbox` · `Radio` · `ChoiceRow` · `Switch` | `Checkbox` · `Radio` / `RadioDot` · `RadioCard` · `Switch` | Thẻ lựa chọn đang chọn: viền đỏ .4 + nền đỏ .06 (theo brand). |
| `Segmented` · tab | `Segmented` · `Tabs` | |
| `Tooltip` · `ExplainPopover` (Vì sao?) | `Tooltip` · `InfoPopover` | |
| `Badge` · `Chip` · `Kbd` · `Card` | `Badge` · `Chip` (`Tone`) · `Kbd` · `Card`, `CardTitle` | Màu trạng thái video: [03-tokens.md](03-tokens.md) › Trạng thái. |
| `Banner` | `Banner` (`tone`: `info` · `success` · `warning` · `error`) | `--issue` → `error` (cam theo brand); `--review` → `warning`; `--ready` → `success`; `--director` (tím Đạo diễn) chưa có — hoi-dap #3. |
| `Toast` | `toast()` · `ToastHost` | Thẻ 300px cho “Cần bạn / Cần xử lý” chưa có trong kit — hoi-dap #4. |
| `Skeleton` · `Spinner` · `Shimmer` · `EmptyState` | `Skeleton` · `Spinner` · `Shimmer` · `EmptyState` | |
| `Sheet` (điện thoại) | `Sheet` | |
| `Dialog` (hộp thường) | `Dialog` | |
| Orb trợ lý | `AgentOrb` (`agent`: `architect` · `writer` · `editor` · `arbiter`) | Bốn trợ lý Viderse trùng sắc bốn agent Noverse: **Trinh sát → `architect`** (xanh), **Đạo diễn → `writer`** (tím), **Hoạ sĩ → `editor`** (vàng cam), **Kiểm định → `arbiter`** (mint). Tên hiển thị luôn là tên Viderse. |
| Icon | `Icon name` (Iconsax, cùng bộ) | Bộ SVG ở [icons/](icons/). |
| Thành phần riêng Viderse (`VideoThumb`, `AgentCard`, `ProgressLog`, `LiveStudio`, `JobsPill`, `PlatformCard`, `QrLoginDialog`…) | dựng trong `apps/viderse/web` | [05-kit.md](05-kit.md) › Thành phần riêng Viderse; chỉ chuyển vào `platform/ui` khi app thứ hai cần (README verse › Sharing). |

## 4. Route (hash router của `apps/viderse/web`)

| Màn | Hash | Tham số |
|---|---|---|
| Bàn dựng | `#/` | `?channel=<id>&video=<jobId>` |
| Duyệt | `#/review` | `?channel=` |
| Hàng chờ | `#/queue` | — |
| Kênh | `#/channels/<id>` | `?tab=videos|sources|templates|settings` |
| Tạo kênh | `#/channels/new` | `?step=create|done` |
| Khám phá | `#/explore` | `?rival=<id>&video=<id>` |
| Video mới | `#/new` | — |
| Thư viện | `#/library` | `?tab=templates|prompts|backdrops|overlays` |
| Cài đặt template | `#/templates/<id>` | `?section=images|cover|backdrop|output` |
| Editor | `#/editor/<projectId>` | — |
| Cài đặt | `#/settings` | `?section=tools|connect|agents|accounts|storage|logs` |
| Tài khoản | `#/account` | (đã có) |
| Lần đầu mở | màn `Setup` của `platform/webapp` | (đã có) |

Việc nền & thông báo là popover từ viên trên thanh đầu, không có route riêng.

## 5. Xem artboard

Từ gốc repo verse: `npx serve apps/viderse/docs/design/v1` rồi mở `/artboards/Main.dc.html`. Tweak (trạng thái) nằm trong `data-props` cuối mỗi tệp; bảng mọi giá trị ở [artboards/README.md](artboards/README.md). Canvas bấm được: link ở [README.md](README.md).
