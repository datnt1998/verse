# Viderse — Brand

Nguồn: board `Brand` ([artboards/Brand.dc.html](artboards/Brand.dc.html)). Tệp trong `brand/` được xuất từ board đó; muốn sửa thì sửa board rồi xuất lại.

## Ý nghĩa

Quả cầu đỏ ở giữa là video, nằm trong một ống kính bằng kính tối. Bốn cung quanh ống kính là bốn trợ lý theo đúng thứ tự làm việc: **Trinh sát** (xanh dương) → **Đạo diễn** (tím) → **Hoạ sĩ** (vàng cam) → **Kiểm định** (xanh bạc hà). Khi trợ lý đang làm, bốn cung xoay một vòng mỗi 9 giây ở logo trên thanh đầu; hết việc thì dừng; máy đặt giảm chuyển động thì luôn đứng yên.

## Tệp

| Tệp | Dùng ở |
|---|---|
| `brand/svg/viderse-mark-dark.svg` | Biểu tượng ống kính trên nền tối: thanh đầu app, tài liệu |
| `brand/svg/viderse-mark-light.svg` | Trên nền sáng (cung đậm hơn một bậc) |
| `brand/svg/viderse-mark-hero.svg` | Bản lớn có quầng: màn chào, trang đăng nhập |
| `brand/svg/viderse-app-icon.svg` · `brand/png/viderse-app-icon-1024.png` | Biểu tượng app mặc định (nền đỏ) |
| `brand/svg/viderse-app-icon-dark.svg` · `brand/png/viderse-app-icon-dark-1024.png` | Biểu tượng app nền tối |
| `brand/svg/viderse-avatar-round.svg` · `brand/png/viderse-avatar-512.png` | Ảnh đại diện tròn |
| `brand/svg/viderse-favicon-{16,24,32,48}.svg` · `brand/png/viderse-favicon-{16,32,48}.png` | Favicon. Dưới 32 px bỏ vành kính, chỉ giữ quả cầu và bốn cung dày hơn |
| `brand/png/viderse-logo-dark@2x.png` · `brand/png/viderse-logo-light@2x.png` | Logo dài (ống kính + chữ “Viderse”): trang đăng nhập, email, tài liệu. Trong app, thanh đầu chỉ dùng biểu tượng |
| `brand/png/viderse-mark-512.png` | Biểu tượng dạng ảnh |

## Luật

- Chữ “Viderse”: Be Vietnam Pro 700, letter-spacing −0,02em.
- Không đặt vào khung chữ nhật kiểu YouTube. Không đổi thứ tự hay màu bốn cung. Không đặt chữ trắng nhỏ trên đỏ `#FF0033` (3,9:1).
- Đỏ của quả cầu (`#FF0033` → `#A3001C`) là màu thương hiệu, khác đỏ nhấn của giao diện (`#FF4D63`). Màu giao diện: [02-stack-verse.md](02-stack-verse.md) §2.
- Trong web app: thay `BrandMark` / `BrandLockup` của kit bằng biểu tượng này; tên “Viderse” đã đặt qua `setKitLabels`.
