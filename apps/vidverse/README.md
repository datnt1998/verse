# Vidverse

App mới trong monorepo `verse`, hiện ở bước khởi tạo. Giao diện web có cùng
kiến trúc với Noverse: React + Vite, StyleX + Tailwind, kit `@noverse/ui`, hash
router, `SessionProvider`, shell desktop/mobile. Bản build nhúng vào binary
`vidverse`. Phía HTTP dùng `platform/webapp`, gồm thiết lập lần đầu, đăng nhập
và phiên.

Hiện có:

- thiết lập quản trị viên bằng mã in ra lúc khởi động, kèm mã khôi phục;
- đăng nhập/đăng xuất;
- app shell với trang chủ trống và trang tài khoản.

Chưa có nghiệp vụ video, Postgres hay agent.

Dữ liệu tách hẳn khỏi các app khác:

| | |
|---|---|
| Thư mục dữ liệu | `~/.vidverse` (`web-accounts.json`) |
| Cookie phiên | `vidverse_session` |
| Cổng mặc định | `127.0.0.1:8788` |

## Chạy

```sh
# ở gốc repo
npm ci                                  # một lần: workspace gồm platform/ui + các web app
npm run build -w apps/vidverse/web      # build ra apps/vidverse/internal/web/dist (được nhúng)
go run ./apps/vidverse/cmd/vidverse serve
```

Lần đầu, lệnh `serve` in địa chỉ và **mã thiết lập** (hết hạn sau 30 phút). Mở
địa chỉ đó, nhập mã, tạo tài khoản quản trị và cất mã khôi phục. Mã thiết lập
chỉ in một lần: lỡ mất thì khởi động lại `serve` khi chưa có tài khoản nào.

## Biến môi trường

| Biến | Ý nghĩa |
| --- | --- |
| `VIDVERSE_ADDR` | Địa chỉ lắng nghe, mặc định `127.0.0.1:8788` (`-addr` ghi đè) |
| `TRUSTED_PROXIES` / `PUBLIC_URL` / `SECURE_COOKIES` | Ranh giới trusted-proxy (xem `platform/webguard`) |

## Phát triển web

```sh
go run ./apps/vidverse/cmd/vidverse serve   # backend ở :8788
npm run dev -w apps/vidverse/web            # Vite, proxy /api → 127.0.0.1:8788
npm run typecheck -w apps/vidverse/web
npm test -w apps/vidverse/web
```

Kit dùng chung ở `platform/ui`. App đọc thẳng mã nguồn kit qua alias trong
`vite.config.ts` và `tsconfig.json`. Brand Vidverse (màu chính `#F43F5E`) và nhãn
tiếng Việt được nạp ở `web/src/main.tsx`. Bản build `internal/web/dist` được
commit vì `go build` nhúng nó; sửa web xong phải build lại.

## Bố cục

```
cmd/vidverse/        CLI: serve, version
internal/bootstrap/  thư mục dữ liệu ~/.vidverse
internal/web/        gateway: SPA nhúng + platform/webapp
web/                 SPA (kiến trúc như apps/noverse/web)
```
