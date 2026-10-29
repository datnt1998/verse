---
title: "Phase 04 — An toàn request, identity dùng chung code, khung Viderse"
status: in-progress (4a xong; 4b chờ smoke; 4c đang làm)
---

# Bối cảnh

- Kiểm Origin/`Sec-Fetch-Site` hiện chỉ chạy khi có `PUBLIC_URL` (`entry/web/proxy.go:235-258`). Ở local mode chỉ có `SameSite=Strict`.
- `SameSite` không chặn được request giữa hai cổng trên cùng một host. Ví dụ `127.0.0.1:8789` và `127.0.0.1:8790` là cùng site. Khi Viderse chạy cạnh Noverse, chỉ phép kiểm Origin theo host:port mới chặn được trang Viderse gửi request thay đổi dữ liệu sang Noverse.
- Cookie không tách theo cổng. Nếu hai app dùng chung tên cookie mà mỗi app một kho phiên, chúng sẽ ghi đè phiên của nhau.

Phase gồm 4 bước con. Theo Q6, bước 4d (SSO) không làm trong MVP; phần mô tả được giữ lại để làm sau.

# 4a. An toàn request cho Noverse

## Thay đổi

1. **Kiểm Origin ở mọi chế độ** với POST, PUT, PATCH, DELETE:
   - Danh sách origin hợp lệ:
     - ở local: `http://127.0.0.1:<port>` và `http://localhost:<port>`, cộng các địa chỉ LAN khi bật `-lan` (`serve.go:205-215` đã gom các địa chỉ này);
     - ở hosted: `PUBLIC_URL`.
   - Từ chối khi `Sec-Fetch-Site: cross-site`, hoặc khi có `Origin` mà origin đó không nằm trong danh sách.
   - Request không có `Origin` vẫn được qua, như hiện nay. Lý do: client ngoài trình duyệt không gửi header này, và cookie `SameSite=Strict` không đi kèm request cross-site.
2. **Kiểm Content-Type cho PATCH và DELETE có body**: body phải là `application/json`. DELETE không có body thì cho qua.
3. **`TRUSTED_PROXIES` rỗng**: sửa comment ở `proxy.go:22-30` cho khớp code. Không đổi hành vi, vì code hiện tại chặt hơn comment. Ghi cấu hình đầy đủ vào `apps/noverse/deploy/README.md` cho trường hợp cần cookie `Secure` sau một proxy loopback: `SECURE_COOKIES=true` đi cùng `PUBLIC_URL=https://…`, vì `proxy.go:84-88` từ chối khởi động khi thiếu `PUBLIC_URL` https. Thêm `TRUSTED_PROXIES=127.0.0.1/32` nếu muốn tin `X-Forwarded-*` từ proxy đó. Giữ test hiện có (cấu hình thiếu thì khởi động lỗi), và thêm test nếu chưa có.

## Gate 4a

| Kiểm tra | Điều kiện qua |
|---|---|
| Test bảng | Trong `entry/web`, kết hợp phương thức {POST, PUT, PATCH, DELETE} × chế độ {local, hosted} × `Origin` {đúng, khác cổng cùng host, host khác, không có} × `Sec-Fetch-Site` {same-origin, same-site, cross-site, không có}. Chỉ hai trường hợp được qua: cùng origin, hoặc không có `Origin` và không phải cross-site |
| Content-Type | PATCH có body `text/plain` → 400, giống POST/PUT hiện nay (`gateway.go:614-616`). DELETE không có body → qua |
| Không hỏng giao diện | Chạy server local trên cổng tạm. Dùng trình duyệt thật làm một thao tác ghi bất kỳ (ví dụ đổi tên hiển thị): thành công. Một trang ở `127.0.0.1:<cổng khác>` gửi `fetch` POST sang: bị từ chối |
| Live | Sau `run-live.sh update`, chạy lại gate phase 03 |

## Kết quả 4a (2026-10-29)

Commit "fix(web): check Origin on state changes in every mode" và "fix(web): accept only this deployment's origins for state changes".

- Danh sách origin hợp lệ: hosted chỉ `PUBLIC_URL` (cả scheme). Local: origin phải trùng host:port của chính request và host là IP (127.0.0.1, ::1, địa chỉ LAN/Tailscale mà `-lan` in ra) hoặc `localhost`; tên miền bị chặn để tránh DNS rebinding. Cách này phủ đúng danh sách ở bước 1 mà không phải truyền danh sách địa chỉ LAN vào gateway.
- Test bảng `TestStateChangingRequestsRequireSameOrigin` (4 phương thức × 2 chế độ × 4 Origin × 4 `Sec-Fetch-Site`) thay cho `TestHostedCSRF`; mọi trường hợp của test cũ (same/cross origin, null origin, không Origin, fetch metadata, GET) đều có trong test mới. `TestAllowedOriginHosts` phủ localhost, ::1, IP LAN, DNS rebinding, `PUBLIC_URL` sai scheme. `TestPatchAndDeleteBodiesMustBeJSON`: PATCH/DELETE body `text/plain` → 400, DELETE không body → qua.
- Helper test gửi request (`tg.do`, `doFrom`, `setup_test`, `auth_test`) giờ đặt `Content-Type: application/json` cho mọi request có body, giống client web.

| Gate | Kết quả |
|---|---|
| Test bảng | Qua (cả package `entry/web`) |
| Content-Type | Qua |
| Không hỏng giao diện | Binary `51708aa` trên `127.0.0.1:8791` (HOME tạm), Chromium thật: tạo admin bằng mã thiết lập, đổi tên hiển thị ở bước hồ sơ → `POST /api/me` 200. Trang ở `127.0.0.1:8792` gửi `fetch` POST `/api/me` sang 8791 → server trả 403; tên hiển thị không đổi |
| Live | Sau `run-live.sh update` lên `51708aa`: gate phase 03 qua (xem phase 03). `curl` POST có `Origin: http://127.0.0.1:8789` tới live → 403 |

# 4b. Code dùng chung cho app thứ hai

Chỉ dời những gì Viderse thực sự import.

1. **`core/auth` nhận tham số app.** Thêm `Options{StorePath, LoginCommand, AppName}` thay cho các chuỗi đang gắn cứng:
   - `oauth.go:161-164,276-277`
   - `store.go:86`
   - `provider.go` (chuỗi lỗi khi refresh)
   - `claudecode.go:21`: biến môi trường. Tên biến cũng thành tham số; Noverse giữ `AINOVEL_CLAUDE_CODE_VERSION`.
   
   Noverse truyền đúng các giá trị hiện tại, nên hành vi không đổi. Mỗi app một kho credential: Viderse dùng `~/.viderse/auth.json`.
2. **`core/roles`.** Tách phần chung của cấu hình role ra khỏi `bootstrap/config.go:194-232,553-563`: `RoleConfig{Provider, Model, Fallbacks []ModelRef, ReasoningEffort}`, `ModelRef{Provider, Model}`, và hàm resolve. Giữ nguyên tag JSON, vì `~/.ainovel/config.json` đang lưu theo đúng dạng này; `bootstrap/models.go:343-354` tra `cfg.Providers[rc.Provider]`. Dời `ParseThinkingLevel` từ `agents` sang `core/llm`, cạnh `ThinkingLevel`. Danh sách role cụ thể (architect, writer, editor…) vẫn thuộc Noverse. Test: đọc một config thật đã bỏ secret rồi ghi lại; giải mã cả hai về `map[string]any` thì phải bằng nhau. Một role có provider khác provider mặc định phải được resolve ra đúng provider và đúng model.
3. **`platform/identity`.**
   - Cắt phụ thuộc `webauth → library`: thêm `Options{ValidProjectID func(string) bool, LegacyProjectID string}`. Noverse truyền `library.ValidProjectID` và `library.LegacyProjectID`.
   - Dời `internal/webauth` sang `platform/identity`, đổi tên package thành `identity` và sửa các chỗ import.
   - Backend Postgres (`persistence/postgres/accounts.go`, `sessions.go`…) ở lại Noverse, vì nó dùng chung migration với Noverse.
4. **`platform/pgtest`.** Đưa tên role, advisory lock và đường dẫn script init thành tham số; Noverse truyền giá trị cũ (`noverse_migrate`, `noverse_app`, `2710,11`).
5. **`platform/logger`.** Chỉ dời nếu Viderse dùng.

## Gate 4b

- Gate phase 01 qua. Thay dòng "Số test không đổi" bằng luật của phase 02: mọi test cũ vẫn có mặt và vẫn pass; test mới được liệt kê riêng.
- Test ranh giới qua: `platform/**` không import `apps/**`.
- Có test mới chứng minh `core/auth` ghi vào `StorePath` được truyền vào, không ghi vào `~/.ainovel`.
- Smoke eval 3/3, chạy bằng binary build từ commit ứng viên và gọi theo đường dẫn tuyệt đối như phase 02. Phải chạy smoke vì `core/roles` và `core/auth` nằm trên đường gọi model.

# 4c. Khung Viderse bằng Go + một Writer agent

## Phạm vi

| Thành phần | Quyết định |
|---|---|
| Vị trí | `apps/viderse/{cmd/viderse,internal,assets,web}`. Không import `apps/noverse/**` (test ranh giới chặn) |
| Cổng | `127.0.0.1:8789`. Bản TS dùng `8787`/`5173`; Noverse live dùng `8790` |
| Dữ liệu | Postgres trên cùng cụm với bản TS, database riêng (đề xuất `viderse_go`, Q8), role `viderse_app`/`viderse_migrate`. Dùng goose migration embed, sqlc với `apps/viderse/sqlc.yaml` |
| Đăng nhập | `platform/identity`, backend file `~/.viderse/web-accounts.json`, cookie `viderse_session` (Q6, Q7) |
| Model | `core/roles`, file `~/.viderse/config.json`. Role `writer` mặc định là `gpt-6-luna` qua `openai-codex` |
| Writer agent | Đầu vào là brief: chủ đề, khán giả, độ dài mục tiêu. Agent kết thúc bằng tool `save_script`, có JSON schema, dùng `StopAfterToolResult`. Lưu kết quả vào bảng `scripts`. Mỗi lần chạy có một dòng `agent_runs`: trạng thái, model, token, chi phí, thời điểm bắt đầu/kết thúc, lỗi. Đọc contract của module `writer` bên Viderse TS để lấy cấu trúc kịch bản, không chép code |
| Web | React + Vite, gói npm riêng ở `apps/viderse/web`, không dùng `@noverse/ui`. Có màn đăng nhập, form brief, danh sách lần chạy, xem kịch bản. Trạng thái chạy cập nhật bằng polling |
| Không làm | Không dùng chung dữ liệu với Noverse. Không thay bản TS. Không làm Remotion, các module agent khác hay MCP |

## Gate 4c

| Kiểm tra | Điều kiện qua |
|---|---|
| Build và test | `go build ./...`; `go test ./...` qua, gồm test Postgres của Viderse với `REQUIRE_POSTGRES_TEST=1`; `apps/viderse/web` chạy được `typecheck` và `test`. Hai job này có trong CI |
| sqlc | `(cd apps/viderse && go tool sqlc diff)` sạch |
| Chạy thật | Trên DB trống: `viderse migrate up` → `viderse serve` → tạo admin → đăng nhập bằng trình duyệt → gửi brief. Writer chạy với model thật tới lúc xong. Bảng `scripts` có 1 dòng; `agent_runs` có 1 dòng `status=succeeded` với token > 0; giao diện hiển thị kịch bản |
| Không giẫm Noverse | Đăng nhập Viderse trên cùng trình duyệt, rồi mở live Noverse 8790: phiên Noverse vẫn còn hiệu lực. Trang Viderse gửi POST sang `127.0.0.1:8790`: bị từ chối (nhờ 4a) |
| Ranh giới | Test ranh giới qua. Thêm tạm một import `apps/noverse/...` vào Viderse thì test fail |
| Dọn | Dừng mọi process đã khởi động. DB thử nghiệm được xoá, hoặc ghi rõ là giữ lại |

# 4d. SSO — không làm trong MVP (Q6), giữ lại để làm sau

Nếu làm:

1. Một kho identity dùng chung. Backend file `~/.verse/identity.json` hay Postgres là quyết định riêng, phải chốt trước.
2. Cookie `verse_session`. Ở local là host-only, nên các cổng trên `127.0.0.1` tự dùng chung. Ở hosted, `Domain` là domain cha (cần chốt tên).
3. `apps.json` chứa tên và URL của các app. Mỗi app có `grants.json` với vai trò riêng. Admin nền tảng ngầm là admin ở mọi app. Không có phiên → 401. Có phiên mà không có grant → 403.
4. Migrate tài khoản Noverse từ file v4 sang kho chung. Backup `.v4.bak` theo đúng cách các bản backup v2/v3 đang làm. Mọi thiết bị phải đăng nhập lại.
5. Thống nhất luật giữa hai backend: hoa/thường username và ngưỡng limiter.

Gate 4d:

- Đăng nhập một lần ở Noverse thì vào được Viderse không cần đăng nhập lại.
- User không có grant Viderse nhận 403.
- Thu hồi phiên thì cả hai app trả 401 ngay ở request kế tiếp.
- Test migrate từ fixture v4 giữ đủ user, vai trò và quyền dự án.
- Rollback: khôi phục `web-accounts.json` từ `.v4.bak` và chạy lại binary trước SSO.

# Rollback chung

Mỗi bước con nằm trong commit riêng. 4a và 4b revert được độc lập. 4c chỉ thêm `apps/viderse`; bỏ đi không ảnh hưởng Noverse.

# Rủi ro

- **Quyết định 0016 của Viderse** đi ngược hướng nhúng core Go và dùng OAuth subscription. Không làm 4c khi anh chưa xác nhận Q7.
- **Ước lượng Writer bị thấp.** Bản TS có module `writer` cùng các module mà nó phụ thuộc (`pipeline`, `knowledge`…). Khung Go chỉ làm Writer độc lập, đầu vào là brief nhập tay.
- **Đụng file core cùng lúc với phase 05.** Hai phase cùng sửa `core/agentcore` thì chỉ một người được giữ file đó tại mỗi thời điểm.
