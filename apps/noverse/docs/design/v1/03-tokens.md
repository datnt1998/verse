# 03 · Design tokens

Nguồn chuẩn trong app: `web/src/styles/tokens.stylex.ts` (StyleX `defineVars`, giữ nguyên tên biến `--color-*`, `--line-*`, `--radius-*`… nên CSS cũ vẫn dùng `var(--…)`). `tokens.css` ở thư mục này là bản thiết kế gốc để đối chiếu. Component mới viết bằng `stylex.create` với các biến này; mã màu viết cứng bị `web/src/styles/hardcodedColors.test.ts` chặn (bảng chốt theo tệp, chỉ được giảm).

## Màu

| Token | Giá trị | Dùng cho |
|---|---|---|
| `bg` | `#0B0B0D` | Nền ứng dụng |
| `surface-0` | `#0F0F11` | Ô nhập, khối lồng (chip lệnh, đường dẫn, round luồng viết) |
| `surface-1` | `#131315` | Nền tablist/segmented, đáy card |
| `surface-2` | `#171719` | Đỉnh card |
| `surface-3` | `#18181A` | Ô chat, hộp thoại |
| `surface-4` | `#1A1A1D` | Hàng đang chọn |
| `surface-5` | `#1E1E21` | Rãnh progress |
| `surface-6` | `#222225` | Tab/segment đang chọn |
| `surface-7` | `#2A2A2E` | Segment chọn mạnh, switch tắt |
| `fg` → `fg-4` | `#EDEDEF` `#C9C9D1` `#A0A0AB` `#7E7E8A` | Chữ chính → phụ thấp nhất |
| `accent` | `#C9A6F5` | Focus, icon tab chọn, caret |
| `ok` `warn` `err` `info` | `#A6EDC1` `#F6CB8C` `#FF9AA2` `#B9D0F7` | Trạng thái; nền mờ dùng `--tint-*` (10%). **Chip trạng thái** (Đang chạy, Đang chờ bạn, Hoàn thành, Khôi phục…): nền `--tint-*` 10%, viền cùng màu 30%; không dùng 6% hay 8% (chốt 27/10) |

**Agent** (chữ / orb):

| Agent | Chữ | Orb (radial gradient) |
|---|---|---|
| ARCHITECT (`architect_long`, `architect_short`) | `#B9D0F7` | `#EEF4FF → #B9D0F7 → #7C93F3 → #5A57E3` |
| WRITER | `#C9A6F5` | `#F3E8FF → #C9A6F5 → #9A63E3 → #C9457F` |
| EDITOR | `#F6CB8C` | `#FFF1DC → #F6CB8C → #EC9548 → #E25E36` |
| ARBITER | `#A6EDC1` | `#E9FBEF → #A6EDC1 → #5DC1B0 → #3E73D6` |

Quy tắc: gradient **chỉ** ở orb, thanh tiến độ WRITER (`--grad-writer-progress`), viền chạy ô chat (`--grad-run-ring`), vệt icon suy nghĩ (`--grad-thinking`), thanh chi phí theo vai trò (dùng orb gradient).

## Chữ

| Vai trò | Font | Cỡ / độ đậm |
|---|---|---|
| Tiêu đề trang | Be Vietnam Pro | 24–30 / 600, `letter-spacing: -0.02em` |
| Tiêu đề màn trong shell | Be Vietnam Pro | 17–20 / 600 |
| Tiêu đề card (`h3`) | Be Vietnam Pro | 13 / 500, màu `fg-3`, kèm icon 16px |
| Nhãn nhóm (`lbl`) | Be Vietnam Pro | 11 / 600, uppercase, `letter-spacing: .08em`, `fg-4` |
| Thân UI | Be Vietnam Pro | 13–14 / 400 |
| Lệnh, tool, đường dẫn, token | JetBrains Mono | 11–13 |
| Văn bản truyện | Literata | 16–18 (người dùng chỉnh 14–24), `line-height: 1.8`, `max-width: 66ch` |

Số luôn `tabular-nums`. Định dạng: token `3.6M`/`76.1k`, chi phí `$2.61`, thời lượng `137ms` · `15.6s` · `2m31s` (theo TUI).

## Bo góc · khoảng cách · kích thước

- Bo: chip 6 · control 8 · button 9 · row 10 · tile 12 · card 16 · dock/dialog 18 · pill 999.
- Khoảng cách: bậc 4/6/8/10/12/14/16/20/24.
- Header 48px. Panel trái 236–272px. Panel phải 340–360px. Ô chat cao ~52px (control 38px).
- Breakpoint: `≥1100px` desktop 3 cột; `<1100px` xếp chồng; `<760px` mobile (tab dưới, ô chat sticky).

## Card “liquid dark”

```css
background: var(--card-bg);           /* #171719 → #131315 */
border: 1px solid var(--line-2);
box-shadow: var(--card-shadow);       /* vệt sáng 1px mép trên */
border-radius: var(--radius-card);
```

## Chuyển động

| Tên | Thời lượng | Dùng |
|---|---|---|
| `ai-breathe` | 2.8s ease-in-out ∞ | Orb agent đang chạy |
| `ai-spin` | .9s linear ∞ | Spinner bước/tool đang chạy |
| `ai-shimmer` | 2.2s linear ∞ | Chữ bước đang chạy, “Đang suy nghĩ” |
| `ai-run` | 4.5s linear ∞ | Viền gradient chạy quanh ô chat (+ bản blur 16px, opacity .45) |
| `ai-infinity` | 1.8s linear ∞ | Vệt gradient chạy trên icon ∞ |
| `ai-twinkle` | 1.6s ∞, lệch .8s | ✦ ✧ cạnh tiêu đề luồng viết |
| `ai-blink` | 1s steps(1) | Caret cuối văn bản đang stream |

Tất cả tắt khi `prefers-reduced-motion: reduce`. Chỉ chạy khi `snapshot.isRunning`.
