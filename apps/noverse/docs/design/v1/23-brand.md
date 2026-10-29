# 23 · Thương hiệu Noverse (26/10)

Sản phẩm đổi tên từ **ainovel** thành **Noverse**. Khẩu hiệu: *Write worlds with AI*. Artboard: `artboards/Brand.dc.html` (B0 trên canvas). Tệp dùng ngay nằm trong `brand/`.

## 1. Biểu tượng

Bốn quả cầu chụm lại là bốn trợ lý cùng viết một thế giới. Thứ tự vẽ từ dưới lên:

| Quả cầu | Trợ lý | Màu giữa | Dải màu (sáng → tối) |
|---|---|---|---|
| Trái | Kiến trúc | `#7896F2` | `#DCE8FF → #A9C2FA → #7896F2 → #5360DC` |
| Trên | Người viết | `#A472EB` | `#F1E2FF → #CDA4F7 → #A472EB → #7C46D4` |
| Phải | Biên tập | `#F29A5A` | `#FFEACF → #FBC48A → #F29A5A → #D9663A` |
| Dưới | Trọng tài | `#5FCFB0` | `#DDFBEE → #A0EDCF → #5FCFB0 → #36A29C` |

- Mỗi quả là `radial-gradient(circle at 34% 28%, …)`, có vành tối dưới phải và ánh sáng trên trái.
- Màu cùng họ với orb của 4 trợ lý trong app (`.o-architect`, `.o-writer`, `.o-editor`, `.o-arbiter`). Orb trong app **giữ nguyên**, không phải đổi.
- Dưới 32 px dùng bản phẳng (`noverse-mark-flat.svg`): không bóng, không ánh sáng. Luật này áp dụng cả logo ở thanh trên cùng (20–22 px); artboard đã vẽ bản phẳng.
- Bản một màu có khe hở trong suốt giữa các quả, đặt trên nền nào cũng đúng.

## 2. Chữ

| Dùng cho | Font | Độ đậm | Ghi chú |
|---|---|---|---|
| Chữ Noverse (chính) | Plus Jakarta Sans | Bold 700 | khoảng chữ −0,012 em |
| Chữ Noverse cỡ nhỏ (≤ 24 px cao) | Plus Jakarta Sans | ExtraBold 800 | bản `-extrabold` |
| Khẩu hiệu | Plus Jakarta Sans | SemiBold 600 | chữ hoa, dài bằng đúng chữ Noverse |
| “Studio” cạnh logo, mọi chữ giao diện | Be Vietnam Pro | như cũ | |

- **Vì sao không dùng Satoshi:** Satoshi (Fontshare) không có trên Google Fonts. Plus Jakarta Sans gần nét nhất (N rộng, o tròn, e cắt ngang), có trên Google Fonts, đủ dấu tiếng Việt, giấy phép OFL.
- Bold 700 là bản chính. Plus Jakarta Sans đậm nhất là 800, nên vai trò “Black” do ExtraBold 800 đảm nhận.
- Trong mọi tệp logo, chữ đã chuyển thành nét vẽ, không cần cài font.
- Chỉ phải tải font khi dựng chữ “Noverse” bằng HTML (thanh trên cùng). Tải đúng các chữ cần dùng:
  `https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@700;800&text=Noverse&display=swap`
  Hoặc tự host tệp `brand/font/PlusJakartaSans[wght].ttf`.

## 3. Tệp trong `brand/`

| Thư mục | Tệp | Dùng ở đâu |
|---|---|---|
| `svg/` | `noverse-mark.svg` · `-glow` · `-flat` · `-mono-black` · `-mono-white` | biểu tượng rời; `-glow` chỉ trên nền tối |
| `svg/` | `noverse-logo-h-{on-dark,on-light,mono-black,mono-white}.svg` | logo dài |
| `svg/` | `noverse-logo-h-tagline-…` | logo dài có khẩu hiệu, khi rộng ≥ 240 px |
| `svg/` | `noverse-logo-v-…` · `noverse-logo-v-tagline-…` | logo đứng: màn chào, ảnh bìa |
| `svg/` | `noverse-logo-h-extrabold-…` · `noverse-wordmark-…` · `noverse-wordmark-extrabold-…` | cỡ nhỏ; chỉ chữ |
| `svg/` | `noverse-appicon-{dark,light}.svg` · `-dark-square` · `-maskable` · `noverse-avatar.svg` · `favicon.svg` | biểu tượng ứng dụng |
| `svg/` | `noverse-og-image.svg` | ảnh chia sẻ 1200×630 |
| `png/icons/` | `favicon-16/32/48/64.png` · `favicon.ico` · `apple-touch-icon.png` (180) · `icon-192.png` · `icon-512.png` · `icon-maskable-512.png` · `noverse-appicon-{dark,light}-1024.png` · `noverse-avatar-512.png` · `noverse-mark*-512.png` | |
| `png/logos/` | mỗi logo hai cỡ (1x và `@2x`) | email, tài liệu, nơi không nhận SVG |
| `png/` | `noverse-og-image.png` | `og:image` |
| `font/` | `PlusJakartaSans[wght].ttf` · `OFL.txt` | |
| `tools/` | `build.sh` + mã sinh | Sinh lại toàn bộ SVG từ mã. Đổi font (vd. sang Satoshi) chỉ cần thay tệp font rồi chạy lại; bố cục giữ nguyên. |

## 4. Gắn vào web

**`index.html`**

```html
<title>Noverse · Không gian viết</title>
<link rel="icon" href="/icons/favicon.svg" type="image/svg+xml">
<link rel="icon" href="/icons/favicon.ico" sizes="any">
<link rel="apple-touch-icon" href="/icons/apple-touch-icon.png">
<meta property="og:title" content="Noverse">
<meta property="og:description" content="Write worlds with AI">
<meta property="og:image" content="/icons/noverse-og-image.png">
```

**`manifest.webmanifest`**

- `name`: “Noverse · Xưởng viết truyện”, `short_name`: “Noverse”.
- `icons` dùng PNG, không dùng SVG (iOS và nhiều launcher Android bỏ qua SVG):
  - `icon-192.png` và `icon-512.png` với `purpose: "any"`;
  - `icon-maskable-512.png` với `purpose: "maskable"`.

**Logo ở thanh trên cùng** (Login, Setup, Onboard, OAuth, Phone; artboard đã vẽ):

- Biểu tượng bản phẳng, cao **22 px trên máy tính** (mọi màn: Login, Setup, Onboard, OAuth, thanh app) và **20 px trên điện thoại**. Không có cỡ 21 px.
- Chữ “Noverse”: Plus Jakarta Sans 16/700, khoảng chữ −0,012 em.
- Chữ “Studio” (nếu có): Be Vietnam Pro 13 px `#7E7E8A`.
- Biểu tượng cách chữ 10 px. “Noverse” và “Studio” canh theo **đường chân chữ** (`align-items: baseline`), cách nhau 6 px. Không canh giữa theo chiều cao, vì hai font khác nhau nên “Studio” sẽ lệch lên.

**Thanh app** (`_appheader`: Bàn viết, Dự án, Audio…): chỉ biểu tượng 22 px, thay cho hàng 4 orb cũ.

**Component `BrandMark`:**

- Nên dựng bằng SVG inline lấy từ `noverse-mark.svg`, id gradient phải duy nhất theo từng lần dùng.
- Cách khác: 4 `span` tròn đặt tuyệt đối như artboard (`.nv-mark` trong `Login.dc.html`).
- Props: `size` (chiều cao), `variant` (`color` · `flat` · `mono`). `flat` tự bật khi `size < 32`.

## 5. Đổi tên ainovel → Noverse

**Đổi:** mọi chữ người dùng thấy.

- Câu trong giao diện (“Noverse đang đọc quy tắc…”, “Quay lại Noverse”, “Noverse 0.9.3”).
- Tiêu đề trang, manifest, thông báo trình duyệt (“Noverse” muốn gửi thông báo), trang OAuth, email.
- Thư mục gợi ý khi tạo thư viện mới: “Tài liệu › Truyện Noverse”.
- Chữ hiển thị trong cửa sổ dòng lệnh (TUI/CLI): màn chào “N O V E R S E”, tiêu đề, câu mô tả trong phần trợ giúp. Tên lệnh và tên cờ giữ nguyên.
- Tên tệp người dùng tải về hoặc nhận được, vd. `ma-khoi-phuc-noverse.txt` (nội dung tệp cũng ghi Noverse). Tệp đã tải trước đây không cần đổi.
  - Chỉ áp dụng cho thư viện tạo mới.
  - Thư viện cũ “Truyện ainovel” vẫn được nhận ra, vì cách dò dựa vào thư mục `.ainovel/` bên trong, không dựa vào tên.

**Giữ nguyên:** tên kỹ thuật, để không phải đổi code và không phải chuyển dữ liệu cũ.

- Lệnh `ainovel-cli` và repo.
- Thư mục `~/.ainovel/`, `.ainovel/`.
- Biến `AINOVEL_*`.
- Khoá `localStorage` `ainovel.*`.
- Tên gói Go.

Tên kỹ thuật vẫn hiện ở phần “Nâng cao” như hiện nay.

**Viết hoa:** luôn là “Noverse”, kể cả khi đứng đầu câu hay giữa câu. Không viết “NOVERSE”. Riêng khẩu hiệu viết hoa toàn bộ.

**Docs đã đổi:** mọi chữ người dùng thấy trong `docs/design/v1/*.md` và mọi artboard. Đoạn mã giữ nguyên.

## 6. Không làm

- Kéo méo hoặc xoay logo.
- Đổi màu hay thứ tự các quả cầu.
- Gõ lại chữ Noverse bằng font khác trong tệp logo.
- Đặt logo lên nền rối.
- Thêm viền hoặc bóng ngoài bộ tệp.

**Vùng trống quanh logo** bằng nửa chiều cao biểu tượng.

**Cỡ nhỏ nhất:**

- logo dài cao 22 px (dùng bản ExtraBold);
- biểu tượng 16 px (dùng bản phẳng).
