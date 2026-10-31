# 09 · Đăng nhập, tài khoản, QR điện thoại & thiết lập lần đầu

Artboard: `Login.dc.html` (A1), `Phone.dc.html` (A2), `Onboard.dc.html` (A3).
Backend tham chiếu: `platform/identity/*`, `internal/entry/web/auth.go`, `cmd/ainovel-cli/serve.go` (đang viết — đối chiếu lại khi merge).

## 1. Mô hình (theo backend thật)

| Khái niệm | Giá trị | Nguồn |
|---|---|---|
| Tệp tài khoản | `~/.ainovel/web-accounts.json` (users, sessions, pairings) | `webauth.DefaultPath` |
| Vai trò | `admin` **Quản trị** — toàn quyền, quản lý tài khoản · `writer` **Người viết** — viết các truyện được giao, tạo truyện mới · `reader` **Người đọc** — chỉ đọc, xuất file | `serveUserUsage` |
| Quyền truyện | `books[]`, `"*"` = tất cả; admin luôn thấy tất cả | `User.CanRead/CanWrite` |
| Tên đăng nhập | `^[a-z0-9._-]{2,32}$` | `usernamePattern` |
| Mật khẩu | ≥ 8 ký tự | `MinPasswordLen` |
| Phiên | `STORE_DRIVER=file`: cookie `ainovel_session`. `STORE_DRIVER=postgres`: cookie `verse_session` dùng chung với Viderse (host-only khi chạy loopback, `Domain=VERSE_COOKIE_DOMAIN` + Secure khi hosted). Cả hai HttpOnly, SameSite=Strict, **30 ngày cố định từ lúc đăng nhập** (`SessionTTL`), `lastSeen` cập nhật mỗi 10 phút | `store.go`, `platform/webguard` |
| Mã QR | mã một lần, **5 phút** (`PairingTTL`), link `<base>/#/pair/<code>`, server trả sẵn SVG | `createPairing` |
| Nhãn thiết bị | `iPhone · Safari`, `Mac · Chrome`… từ User-Agent | `DeviceLabel` |
| Giới hạn thử | Theo từng IP và từng tên đăng nhập: 5 lần sai liên tiếp vẫn trả 401; lần sai thứ 5 bắt đầu khoá 30 giây nên lần thử **thứ 6** (kể cả đúng mật khẩu) trả 429 + `Retry-After`, `retryAfter` (giây): “Thử sai quá nhiều lần, vui lòng đợi rồi thử lại”. Mỗi lần sai tiếp theo nhân đôi thời gian khoá, tối đa 15 phút; đăng nhập đúng xoá bộ đếm; khởi động lại `serve` cũng xoá | `limiter.go`, `tooMany` |
| Tài khoản đầu tiên | tạo **trong terminal** khi `serve` chạy lần đầu, hoặc `ainovel-cli serve user add <tên> --role admin` | `ensureFirstAdmin` |

Web **không** có màn đăng ký. Tài khoản do quản trị viên tạo (web hoặc `serve user add`).

## 2. API dùng cho các màn

| Màn / hành động | API | Lỗi cần xử lý |
|---|---|---|
| Đăng nhập | `POST /api/login {username,password}` → `{user}` + cookie | 401 “sai tên đăng nhập hoặc mật khẩu” · 429 |
| Đăng nhập bằng QR (điện thoại) | `POST /api/pair {code}` → `{user}` + cookie | 401 “mã QR không hợp lệ, đã dùng hoặc đã hết hạn” · 429 |
| Ai đang đăng nhập | `GET /api/me` → `{user, session}` (`session.expiresAt` → “còn N ngày”) | 401 → màn Đăng nhập |
| Đăng xuất | `POST /api/logout {}` | — |
| Đổi mật khẩu | `POST /api/me/password {current,new}` → `{ok:true, reauthenticate:true}`: thu hồi **mọi** phiên kể cả phiên hiện tại, xoá cookie; client báo “Đã đổi mật khẩu. Vui lòng đăng nhập lại.” rồi về đăng nhập | 403 “Mật khẩu hiện tại không đúng” · 400 <8 ký tự · 429 |
| Thiết bị của tôi | `GET /api/me/sessions` → `[{id,device,ip,createdAt,lastSeen,expiresAt,current}]` | — |
| Đăng xuất 1 thiết bị | `POST /api/sessions/:id/revoke {}` (của mình; admin: của bất kỳ ai) | 404 |
| Tạo mã QR cho mình | `POST /api/me/pairings {}` → `{code,username,expiresAt,links:[{label,url,svg}]}` | `links` rỗng → trạng thái “Chưa có địa chỉ cho điện thoại” |
| Quản lý tài khoản (admin) | `GET/POST /api/users` · `POST /api/users/:u` (patch name/role/books/disabled/password) · `POST /api/users/:u/delete` · `GET /api/users/:u/sessions` · `POST /api/users/:u/pairings` | 409 “phải còn ít nhất một quản trị viên đang hoạt động” · 409 “tài khoản đã tồn tại” · 403 |
| Quyền vào app (admin, chỉ postgres) | `PUT /api/users/:u/grants/:app {role:"member", reason?}` · `DELETE /api/users/:u/grants/:app {reason?}` → `User` (có `grants` mới); `app` = `noverse`·`viderse`. Đổi thật thì ghi nhật ký và thu hồi mọi phiên của người đó; không đổi gì thì không ghi, không thu hồi | 400 app lạ hoặc `role` khác `member` · 404 · 403 |

Phát hiện điện thoại đã vào (A1 › qr, `paired`): khi hộp thoại QR mở, gọi `GET /api/me/sessions` (hoặc `/api/users/:u/sessions`) mỗi 3 giây; phiên mới có `createdAt > lúc tạo mã` ⇒ hiện “Đã đăng nhập trên iPhone · Safari” và dừng dò. Không cần API mới.

## 3. User flows

### F0. Lần đầu dùng (quản trị)
1. Terminal: `ainovel-cli serve` (hoặc `--lan`) → chưa có tài khoản → **không hỏi gì**, in “Mở trình duyệt để thiết lập: http://127.0.0.1:8686” và mã thiết lập 8 ký tự (hết hạn 30 phút); có LAN/Tailscale thì in thêm địa chỉ điện thoại. *(Đổi 25/10, Gói M · GAP-41.)*
2. Trình duyệt mở `#/` → `GET /api/me` 401 → `GET /api/setup`. Nếu `needed=true` → **A0 · Lần chạy đầu** (`Setup`): nhập mã → tạo tài khoản quản trị → tự đăng nhập → mã khôi phục (xem §11). Nếu đã có tài khoản → **A1 › other** (chưa có tài khoản nhớ trên máy).
3. Đăng nhập (hoặc vừa tạo ở A0; bước Hồ sơ điền sẵn tên hiển thị) → `user.onboardedAt` rỗng (GAP-15) → **A3** luồng quản trị, 7 bước theo `19-gap-closure.md`:
   1. **Hồ sơ**: tên hiển thị (`POST /api/me {name}`), giới thiệu 4 agent trên một hàng.
   2. **Nhà cung cấp AI**: hiện cấu hình đã có → [Kiểm tra kết nối] → [Dùng cấu hình này] hoặc [Đổi]. Cả 12 preset từ `setup.go` luôn hiển thị trong lưới radio 3 cột; giữ tên ngắn và dòng gợi ý đúng `Onboard.dc.html`, không gom vào Combobox hay nhóm API Key. ChatGPT Plus/Pro có biểu tượng OAuth và hai cách đăng nhập. Preset đã chọn mở thẻ đăng nhập hoặc API Key/Base URL ngay dưới lưới.
   3. **Model & agent**: danh sách ngắn dùng hàng radio như artboard; danh sách model dài dùng Combobox (kit). Model chính + mức suy luận; tắt “Dùng model chính cho mọi agent” để đặt riêng Architect/Writer/Editor (`roles`). **Arbiter luôn dùng model chính**.
   4. **Nơi lưu truyện**: người dùng chọn thư mục (đang dùng / gợi ý “Tài liệu › Truyện Noverse” / Chọn thư mục khác…) → `PUT /api/library {root}`; không còn “thư mục gốc chỉ đọc”. Bên dưới: 4 thẻ giải thích thư mục con (đổi tên ở Tuỳ chỉnh › Nâng cao) và công tắc tự xuất bản TXT/EPUB. Mọi chỗ tóm tắt hiện “Nơi › Thư mục”, không hiện tên trần. Xem §10 · GAP-34.
   5. **Dự án đầu tiên** (tùy chọn): hai thẻ cạnh nhau để dùng thư mục có sẵn hoặc tạo mới; tên hiển thị và thư mục trên cùng hàng; mô tả tùy chọn.
   6. **Điện thoại & người cùng viết** (tùy chọn): thẻ QR và lời mời cùng hàng, tự xuống hàng khi thiếu chỗ.
   7. **Hoàn tất**: tóm tắt các giá trị đã lưu (mỗi dòng có “Sửa”), [Viết truyện đầu tiên] và [Vào Workspace]. Không coi tên dự án chưa lưu là dự án đã tạo.
4. Kết thúc → `POST /api/me/onboarded` (GAP-15). Lần sau vào thẳng route cuối cùng.

Bước tùy chọn có [Để sau]. Bước đã xong bấm được trên rail để quay lại; bước chưa tới bị khóa. Mỗi bước ghi ngay khi bấm [Tiếp tục], qua useAction và phiên bản If-Match. Bố cục desktop: rail 272 px, panel phải 304 px, nội dung tối đa 720 px; panel phải giữ bản tóm tắt bằng lời thường theo §9, JSON mặc định thu gọn.

### F0b. Lần đầu của người viết / người đọc
A3 với `role = writer`, 4 bước: **Hồ sơ** (truyện được giao, quyền) → **Mật khẩu riêng** (`POST /api/me/password`; tùy chọn, nhưng nhắc mạnh khi `mustChangePassword`, GAP-15) → **Điện thoại** (QR) → **Hoàn tất** ([Mở truyện được giao]). Người đọc dùng cùng luồng, ẩn nút tạo truyện.

### F1. Đăng nhập lại (A1)
- **login**: danh sách tài khoản đã dùng trên máy này (frontend, `localStorage["ainovel.accounts"] = [{username,name,role,lastLoginAt,device}]` — chỉ thông tin hiển thị, không lưu mật khẩu/cookie). Nút × gỡ khỏi máy. [Đăng nhập tài khoản khác] → **other**.
- **password**: thẻ tài khoản + [Đổi] · mật khẩu (hiện/ẩn) · [Quên mật khẩu?] mở hướng dẫn: nhờ quản trị viên; tài khoản quản trị có thêm **Dùng mã khôi phục** (GAP-33, `20-settings-rules.md` §6) · dòng “Phiên giữ 30 ngày… đến <ngày>” · ô “Máy dùng chung — không ghi nhớ tài khoản”.
- Lỗi: `error=wrong` viền đỏ + “Sai tên đăng nhập hoặc mật khẩu”, giữ nội dung đã gõ, focus lại ô mật khẩu; `error=limited` khóa nút, đếm ngược theo `retryAfter`.
- Thành công → quay lại route đã lưu trong `sessionStorage["ainovel.next"]` (mặc định `#/`).

### F1b. Phiên hết hạn
Bất kỳ API/SSE nào trả 401 khi đang dùng → lưu route hiện tại vào `ainovel.next` → **A1 › expired**: thẻ cam “Phiên đăng nhập đã hết hạn” + khoảng 30 ngày + “Các lượt viết đang chạy trên máy chủ không bị dừng” + nơi sẽ quay lại. Mất kết nối mạng (không phải 401) chỉ hiện banner (Dialogs › offline), không đá ra ngoài.

### F2. Đăng nhập điện thoại bằng QR
Máy tính (đã đăng nhập): ảnh đại diện → **Đăng nhập điện thoại bằng QR** (đúng nhãn `serve` in ra) → **A1 › qr**:
- Quản trị chọn được “Cho tài khoản” (tạo mã cho người khác qua `/api/users/:u/pairings`); người khác chỉ cho mình.
- Tab theo `links[]` (nhãn = host; frontend đoán loại: `100.64.0.0/10` → Tailscale, còn lại Wi-Fi).
- QR từ `svg` server trả (không tự sinh), thanh đếm ngược 5:00, [Tạo mã mới], [Sao chép liên kết]; hết hạn → **qrExpired** (QR mờ + nút tạo lại).
- Danh sách thiết bị của tài khoản đó; thiết bị vừa vào được tô “Vừa đăng nhập”.
- Cảnh báo cố định: mã đăng nhập được tài khoản trong 5 phút; HTTP không mã hoá, ngoài mạng nhà dùng Tailscale.
- `links` rỗng → **qrNoLan**: nút **Cho phép thiết bị cùng Wi-Fi** (quản trị, `PUT /api/system/network {lan:true}` — GAP-33); người khác thấy “Nhờ quản trị bật…”. Cờ `--lan` / `--addr` chỉ còn trong tài liệu dòng lệnh.

Điện thoại (A2): Camera mở `/#/pair/<code>` → **confirm** (máy chủ, nhãn thiết bị, hạn phiên, cảnh báo “chỉ tiếp tục nếu chính bạn vừa quét”) → [Đăng nhập] → `POST /api/pair` → **pairing** → **home** (Workspace mobile + toast “Xin chào Dat · phiên đến …”). Mã hỏng → **invalid** (thông báo server + cách lấy mã mới + [Đăng nhập bằng mật khẩu] → **login**). Không gọi `/api/pair` khi trang vừa tải — chỉ khi người dùng bấm (chống đăng nhập ngầm từ link lạ). Sau khi xong xoá code khỏi URL (`history.replaceState`).

**Quét mã trong app (A2b › scan, scanFound, scanWrong, cameraBlocked).** Màn đăng nhập điện thoại có nút lớn “Quét mã QR để đăng nhập” → camera toàn màn hình:
- Khung ngắm 236px bo góc, 4 góc trắng, vệt quét tím chạy dọc; nền ngoài khung tối 58%. Nút đóng (trái), đèn flash (phải, `torch` nếu thiết bị hỗ trợ).
- Tấm dưới: “Hướng camera vào mã QR trên máy tính” · “Giữ điện thoại cách màn hình khoảng 20–30 cm” · gợi ý “Chưa thấy mã trên máy tính? Mở Noverse trên máy tính → bấm ảnh đại diện → Đăng nhập điện thoại bằng QR” · “Đang tìm mã…” · [Đăng nhập bằng mật khẩu].
- Nhận đúng mã (URL cùng máy chủ, dạng `/#/pair/<code>`): góc khung chuyển mint, mã sáng lên, rung nhẹ (`navigator.vibrate(30)` nếu có) → “Đã nhận mã đăng nhập · Đang mở bước xác nhận…” → **confirm** sau ~1 giây.
- Mã QR khác: góc cam + “Đây không phải mã đăng nhập Noverse”, vẫn tiếp tục quét.
- Không mở được camera (bị từ chối quyền, hoặc trình duyệt chặn vì kết nối chưa bảo mật — `window.isSecureContext === false`, thường gặp khi mở qua Wi-Fi nhà bằng địa chỉ http): **cameraBlocked** hướng dẫn dùng ứng dụng Camera của điện thoại (3 bước) + lý do bằng lời thường + [Thử mở camera lại] + [Đăng nhập bằng mật khẩu]. Nếu biết trước là không bảo mật thì đi thẳng tới màn này, không xin quyền camera.
- Giải mã: `BarcodeDetector` (`formats: ['qr_code']`) nếu có, không thì thư viện nhỏ như `jsQR` chạy trên khung hình `<video>` ~8 lần/giây; dừng camera ngay khi nhận mã hoặc rời màn.
- Phía máy tính có artboard riêng **A2a** (= A1 › qr) để xem cặp với điện thoại.

### F3. Menu tài khoản (A1 › menu)
> Bổ sung Gói AO (31/10): nhóm **Ứng dụng** (mở Viderse) và câu “cả Noverse và Viderse” ở Đăng xuất, Chuyển tài khoản. Xem §12. <!-- goi-ao -->

Ảnh đại diện ở header → menu: tên, vai trò, “Phiên trên Mac · Chrome · còn N ngày” + thanh còn lại (đổi sang cam khi ≤ 3 ngày) · Đăng nhập điện thoại bằng QR · Thiết bị đang đăng nhập (số) · Đổi mật khẩu · Quản lý tài khoản (chỉ admin) · Chuyển tài khoản (danh sách nhớ trên máy → đăng xuất rồi tới A1 › password) · Đăng xuất (đỏ).

### F4. Thiết bị & bảo mật (A1 › devices)
Danh sách phiên (thiết bị, IP, hoạt động gần nhất, ngày đăng nhập, hạn; ≤ 2 ngày tô cam) + [Đăng xuất] từng phiên + [Đăng xuất mọi thiết bị khác] (lặp `revoke`). Cột phải: đổi mật khẩu với thanh độ mạnh (≥8 bắt buộc; 12+, chữ+số, ký tự đặc biệt tăng điểm).

### F5. Quản lý tài khoản (A1 › users, chỉ admin)

> **Thay bằng Gói AD (28/10):** hộp thoại A1 › users bỏ; quản lý ở trang **Tuỳ chỉnh › Thành viên** (`Members.dc.html`), xem *F5b* dưới đây. Đoạn cũ giữ lại để tra. <!-- f5-thay -->

Bảng: ảnh đại diện, tên/@, vai trò, truyện, số thiết bị, trạng thái (Hoạt động/Đã khoá). Panel phải: tên hiển thị, vai trò (radio kèm mô tả), cây quyền truyện (Tất cả `*` › dự án › truyện; admin: khóa, luôn tất cả), [Đăng nhập điện thoại cho X], [Đặt lại mật khẩu] (đăng xuất mọi thiết bị), [Khoá/Mở khoá], [Xoá]. Quản trị viên cuối cùng: khoá & xoá bị vô hiệu kèm lý do. [Thêm tài khoản] mở form: tên đăng nhập, tên hiển thị, vai trò, truyện, mật khẩu tạm (sinh sẵn, sao chép được).

Lưu ý id truyện: hiện `books[]` là tên thư mục con của `--root`. Khi có tầng dự án (GAP-1) dùng `project/story` và cho phép `project/*`.

### F5b. Thành viên (Gói AD, chốt 28/10) <!-- f5b-thanh-vien -->

**Chỗ vào:** Tuỳ chỉnh › Hệ thống · Quản trị › **Thành viên** (chỉ quản trị), và menu tài khoản › *Thành viên*. Người khác tự đổi tên, mật khẩu, thiết bị ở menu tài khoản.

**Quyền hai tầng.** Không còn 3 vai trò chung (Quản trị / Người viết / Người đọc) — tên “Người viết” trùng agent.

| Tầng | Giá trị | Ý nghĩa |
|---|---|---|
| Quyền hệ thống | **Quản trị** · **Thành viên** | Quản trị: Chủ dự án ở mọi dự án (kể cả tạo sau) + Thành viên, Kết nối AI, Xuất tự động, Báo động, Nâng cao, Não hệ thống. Thành viên: chỉ dự án được giao. |
| Vai trò theo dự án | **Không vào** · **Chỉ xem** · **Cộng tác** · **Chủ dự án** | Mỗi dự án một vai trò. Dự án tạo sau mặc định *Không vào*. |

| Làm được gì | Chỉ xem | Cộng tác | Chủ dự án |
|---|---|---|---|
| Đọc, nghe, tải bản xuất | ✓ | ✓ | ✓ |
| Viết, duyệt, sửa chương; dặn trợ lý | | ✓ | ✓ |
| Quy tắc riêng, nhân vật & xưng hô của truyện | | ✓ | ✓ |
| Tạo truyện mới trong dự án | | ✓ | ✓ |
| Tạo giọng & xuất audio | | ✓ | ✓ |
| Quy tắc chung, cách viết, ngân sách của dự án | | | ✓ |
| Xoá truyện, đổi tên dự án | | | ✓ |

**Trang** (`Members.dc.html`, `view`: members · log; `who`: linh · dat · minh · ha; `dlg`: none · add · addError · added · reset · resetDone · lock · remove; `edit`: clean · dirty · promote):

- **Danh sách** (trái, 380 px): ô tìm theo tên / tên đăng nhập; lọc *Tất cả · Quản trị · Chưa giao · Đã khoá* (có số). Mỗi dòng: avatar chữ cái (nền trung tính), tên, “(bạn)”, chip *Quản trị*, tóm tắt quyền (“Cộng tác ở ‘Kênh truyện audio’”, “Chỉ xem ở 2 dự án”, cam “Chưa được vào dự án nào”), trạng thái (Đang hoạt động / Hoạt động / Chưa vào lại / Đã khoá), chip *Mật khẩu tạm* khi chưa đổi.
- **Chi tiết** (phải): đầu thẻ (tên, @tên đăng nhập, ngày tạo, lần hoạt động, nút *Đăng nhập điện thoại cho X* → QR “Cho tài khoản”). Bị khoá → băng đỏ “Đã khoá {ngày} bởi {ai}” + lý do + *Mở khoá*.
  1. **Quyền hệ thống**: 2 thẻ radio. Đổi sang Quản trị → cảnh báo cam (vai trò dự án được giữ để dùng khi bỏ quản trị). Quản trị cuối cùng → khoá cả hai thẻ, câu giải thích.
  2. **Dự án**: mỗi dòng tên + số truyện + ô chọn vai trò (menu 4 mục, có mô tả một dòng, chấm màu). Dòng đã đổi có chip *Đã đổi*. Nút *Mỗi vai trò làm được gì?* mở bảng quyền ở trên. Quản trị: mọi dòng *Chủ dự án · theo quyền quản trị*, khoá.
  3. **Quyền vào ứng dụng** (chỉ `STORE_DRIVER=postgres`): ô *Noverse* · *Viderse*, lưu ngay khi bấm (không qua thanh lưu). Quản trị: cả hai khoá ở trạng thái bật, “Được vào theo quyền quản trị”. Bỏ hết → nhắc “đăng nhập được nhưng sẽ thấy thông báo chưa được cấp quyền”. Cấp hay thu đều đăng xuất người đó khỏi mọi thiết bị. Hộp *Thêm thành viên* có cùng hai ô, mặc định chỉ Noverse.
  4. **Thiết bị đang đăng nhập**: thiết bị, lần hoạt động, còn N ngày, *Đăng xuất* từng cái; *Đăng xuất mọi thiết bị* (của mình: *các thiết bị khác*; thiết bị đang dùng có chip *Thiết bị này*).
  5. **Mật khẩu**: lần đổi cuối / “Đang dùng mật khẩu tạm”; *Đặt lại mật khẩu* (tắt với chính mình — tự đổi ở menu).
  6. **Khoá hoặc xoá**: *Khoá* (đăng xuất mọi thiết bị, giữ vai trò, mở lại được) · *Xoá* (đỏ; truyện, chương, nhật ký giữ, ghi “X (đã xoá)”). Tắt với chính mình và quản trị cuối cùng, kèm lý do.
- **Lưu**: đổi quyền dự án gom vào thanh nổi “N thay đổi cho Linh · có hiệu lực ngay khi lưu” · *Huỷ* · **Lưu**. Quyền dự án có hiệu lực ngay, không bắt đăng nhập lại.
- **Thêm thành viên** (hộp thoại): Tên hiển thị → tự gợi ý tên đăng nhập bỏ dấu (`Ngọc` → `ngoc`, sửa tay thì thôi tự gợi ý); lỗi trùng “Tên đăng nhập ‘linh’ đã có. Thử ‘linh.2’…”; Quyền hệ thống; *Dự án được vào* (mỗi dự án một nhóm nút 4 vai trò, chưa giao gì → nhắc cam); Mật khẩu tạm sinh sẵn (*Sao chép*, *Tạo khác*). Nút *Tạo tài khoản* mờ khi thiếu / sai.
- **Đã tạo** / **Đã đặt lại**: tên đăng nhập, mật khẩu tạm (*Chép*), cách vào; cảnh báo “Mật khẩu tạm chỉ hiện lần này”; *Chép lời nhắn gửi X* (tên đăng nhập + mật khẩu + cách vào), *Đăng nhập điện thoại*, *Xong*.
- **Xác nhận** (Đặt lại · Khoá · Xoá): liệt kê hệ quả bằng lời thường; Khoá/Xoá có ô *Lý do* (ghi nhật ký, không bắt buộc); hộp Xoá có lối *Khoá thay vì xoá*.
- **Nhật ký quản trị** (`view=log`): lọc *Tất cả · Tài khoản · Vai trò · Mật khẩu & thiết bị* và theo người (*Mọi người · Dat · Linh · Hệ thống*); nhóm theo ngày; mỗi dòng giờ, avatar người làm (Hệ thống = khiên đỏ), câu mô tả, chip *trước → sau*, lý do trong ngoặc kép. Chỉ đọc; giữ 1 năm. Ghi cả đăng nhập sai nhiều lần.

## 4. Component mới

| Component | Ghi chú |
|---|---|
| `AuthShell` | Header gọn (brand + chip máy chủ có chấm trạng thái) · cột 400px giữa · footer gợi ý. Không có nav. |
| `OrbHero` | 4 orb xếp cụm + quầng sáng mờ; `breathe` lệch pha; tắt khi reduced-motion |
| `AccountPicker` / `AccountChip` | Avatar chữ cái (nền trung tính, **không** dùng gradient agent), badge vai trò |
| `PasswordField` | Nút hiện/ẩn có `aria-label` đổi theo trạng thái; `aria-invalid` + `aria-describedby` lỗi |
| `RoleBadge` | admin tím + vương miện · writer xanh + bút · reader xám + mắt |
| `AccountMenu` | `role="menu"`, phím ↑↓ Enter Esc, đóng khi click ngoài |
| `PairingDialog` | Tabs địa chỉ, QR SVG (nền trắng, viền 18px — đủ quiet zone), `TtlBar`, `SessionList`, cảnh báo |
| `SessionList` | dùng chung ở QR, Thiết bị, Quản lý tài khoản |
| `StrengthMeter` | 4 vạch + nhãn (Yếu/Tạm được/Mạnh/Rất mạnh); chỉ gợi ý, chặn cứng ở < 8 |
| `UserTable` + `UserEditor` + `BookAccessTree` | A1 › users |
| `OnboardShell` | Header (Bước N/M + thanh) · `StepRail` · nội dung tối đa 680px · `ActionBar` (Quay lại · Enter · Để sau · Tiếp tục) · `WritePreview` 360px |
| `StepRail` | Đã xong (tick mint + giá trị) · đang làm (viền tím, chấm tím) · chưa tới (vòng nét đứt, khóa) · nhãn “tùy chọn” |
| `WritePreview` | Tên tệp/API + JSON tô màu (khóa xanh, chuỗi mint, số cam), dòng thay đổi nền tím nhạt, nhãn “Chưa có API” khi phụ thuộc GAP |
| `ProviderGrid` / `ProviderForm` / `OAuthPanel` | 12 preset, form theo loại (API key bắt buộc/tùy chọn, custom thêm tên + giao thức, OAuth: trình duyệt / mã thiết bị) |
| `MobileShell` | Header 1 dòng, nội dung cuộn, `footer` nút 50px dính đáy, tab đáy 4 mục; font input 16px (tránh iOS zoom) |

## 5. Chuỗi giao diện chính

- “Chào mừng trở lại” · “Chọn tài khoản để vào thư viện truyện” · “Tài khoản đã dùng trên máy này” · “Đăng nhập tài khoản khác” · “Chào mừng trở lại, {tên}” · “Nhập mật khẩu để tiếp tục”
- “Quên mật khẩu?” → “Nhờ quản trị viên đặt lại mật khẩu cho bạn. Họ sẽ gửi một mật khẩu tạm — lần đầu vào, bạn tự đặt mật khẩu mới.” · quản trị: “Dùng mã khôi phục” · “Đổi mật khẩu sẽ đăng xuất mọi thiết bị của tài khoản này.”
- “Phiên giữ 30 ngày trên thiết bị này — đến {ngày}” · “Máy dùng chung — không ghi nhớ tài khoản trên máy này”
- Lỗi lấy nguyên từ server: “Sai tên đăng nhập hoặc mật khẩu” · “Thử sai quá nhiều lần, vui lòng đợi rồi thử lại” · “Mã QR không hợp lệ, đã dùng hoặc đã hết hạn” · “Mật khẩu hiện tại không đúng” · “Phải còn ít nhất một quản trị viên đang hoạt động” · “Tài khoản đã tồn tại” (viết hoa chữ đầu khi hiển thị).
- “Phiên đăng nhập đã hết hạn” · “Phiên của {tên} trên thiết bị này đã đủ 30 ngày ({từ} → {đến}). Các lượt viết đang chạy trên máy chủ không bị dừng.” · “Sau khi đăng nhập sẽ quay lại {nơi}”
- QR: “Đăng nhập điện thoại bằng QR” · “Cho tài khoản” · tab “Wi-Fi nhà — khi ở nhà” / “Tailscale — khi ra ngoài” (không hiện IP/cổng) · “Dùng một lần · hết hạn sau” · “Tạo mã mới” · “Sao chép liên kết” · “Đang chờ điện thoại quét…” · “Đã đăng nhập trên {thiết bị}” · “Đừng chia sẻ mã này — ai quét được sẽ vào được tài khoản của bạn trong 5 phút.” · “Chưa có địa chỉ cho điện thoại” → “Hiện Noverse chỉ mở được trên máy tính này…”
- Điện thoại: “Đăng nhập trên điện thoại này?” · “Chỉ tiếp tục nếu chính bạn vừa quét mã trên máy tính của mình.” · “Huỷ — không phải tôi” · “Không dùng được mã này” · “Nhanh hơn: quét mã QR”
- Onboard: xem nhãn trong `Onboard.dc.html` (`LABEL`, tiêu đề từng bước).

## 6. Bảo mật & a11y

- Không bao giờ lưu mật khẩu, cookie hay mã QR vào `localStorage`. Danh sách tài khoản nhớ chỉ có tên/vai trò/lần cuối; “Máy dùng chung” ⇒ không ghi.
- `autocomplete`: `username`, `current-password`, `new-password`; `autocapitalize="none"` + `spellcheck=false` cho tên đăng nhập.
- Nút có hành động phá hủy (Đăng xuất thiết bị, Xoá tài khoản) cần xác nhận; Xoá yêu cầu gõ lại tên đăng nhập.
- Đếm ngược QR và 429 dùng `aria-live="polite"`, không đọc từng giây (đọc khi còn 60s, 10s, hết hạn).
- Toàn bộ form submit bằng Enter; focus vào ô đầu tiên khi mở màn/hộp thoại; Esc đóng hộp thoại.

## 7. GAP mới

| ID | Hạng mục | Đề xuất |
|---|---|---|
| GAP-13 | Cấu hình thư viện | `~/Truyen/.ainovel/library.json` `{dirs:{ideas,assets,stories,exports}, slug, autoExport}` · `GET/PUT /api/library` (admin). Dùng chung với Cài đặt › Thư viện |
| GAP-14 | Tự đổi tên hiển thị | `POST /api/me {name}` cho mọi vai trò (hiện chỉ admin sửa qua `/api/users/:u`) |
| GAP-15 | Trạng thái onboard & mật khẩu tạm | `User.onboardedAt`, `User.mustChangePassword` (đặt khi admin tạo/đặt lại mật khẩu, xoá khi `/api/me/password` thành công); trả trong `/api/me`; `POST /api/me/onboarded` |
| GAP-16 | Đăng nhập provider từ web | `POST /api/config/oauth/start {provider, device}` → `{url}` hoặc `{userCode, verifyURL}` · `GET /api/config/oauth/status` · `GET /api/config/models?provider=` (dùng `codex.ListModels`). Chỉ admin |

GAP-5 (đọc/ghi config + thử kết nối) cũng là điều kiện cho bước 3–4 của onboard. Khi chưa có GAP-5/16: bước 3–4 chỉ hiển thị (provider/model đọc từ `Snapshot` của một truyện nếu có), nút đổi thay bằng hướng dẫn dùng `/config` và `/model` trong TUI.

## 8. Đăng nhập nhà cung cấp (OAuth) — trang xác thực & trạng thái thành công

Artboard: `OAuth.dc.html` (A4) — trang trình duyệt thấy sau khi đăng nhập ở nhà cung cấp; `Onboard.dc.html` › step `provider` + tweak `oauth` — thẻ kết quả trong app.

### 8.1 Trang callback (`core/auth/oauth.go` › `callbackPage`)
Phục vụ bởi callback server cục bộ `127.0.0.1:1455/auth/callback` (redirect `http://localhost:1455/auth/callback`). Thay trang `<h2>/<p>` hiện tại bằng thiết kế A4.

| Trường hợp | Điều kiện trong `startCallbackServer` | Tiêu đề | Chi tiết hiển thị |
|---|---|---|---|
| `success` | có `code`, `state` khớp | Đã kết nối ChatGPT | “Noverse đã được phép dùng gói ChatGPT của bạn để viết truyện.” · 3 bước: ✓ Đăng nhập ChatGPT · ✓ Cho phép Noverse sử dụng gói của bạn · → Chọn model để bắt đầu viết · “Điều này có nghĩa là”: không cần mua API riêng, không tốn thêm phí · không phải đăng nhập lại mỗi lần · ngắt kết nối trong Cài đặt › Nhà cung cấp AI |
| `denied` | `error` có giá trị | Chưa kết nối được ChatGPT | Lý do: “Bạn đã huỷ hoặc chưa bấm “Cho phép””. Cách khắc phục: thử lại · kiểm tra tài khoản có gói Plus/Pro |
| `stateMismatch` | `state` khác | Chưa kết nối được ChatGPT | Lý do: “Liên kết đăng nhập này đã cũ” — Noverse chặn để bảo vệ tài khoản. Cách khắc phục: đóng tab · bắt đầu lại |
| `missingCode` | thiếu `code` | Chưa kết nối được ChatGPT | Lý do: “Thông tin đăng nhập chưa đầy đủ”. Cách khắc phục: thử lại · dùng “Đăng nhập bằng mã” |

Mọi trường hợp lỗi có thêm gợi ý “Đăng nhập bằng mã” cho người đang ở máy khác/điện thoại, và mục **Chi tiết kỹ thuật** thu gọn (mặc định đóng) chứa thông điệp lỗi gốc và lệnh `ainovel-cli login` / `login --device` (bản cli). Không hiện địa chỉ trang, cổng, `code`, `state` ở phần chính.

Biến thể theo nơi khởi động (`from`):
- **cli** (`ainovel-cli login`, TUI setup): thẻ “Bạn có thể đóng tab này — quay lại cửa sổ Noverse, danh sách model bạn được dùng sẽ hiện ra ở đó.” (thất bại: “Thử lại trong cửa sổ Noverse”, lệnh nằm trong Chi tiết kỹ thuật).
- **web** (onboard/Cài đặt, GAP-16): nút [Quay lại Noverse] + “Tab này tự đóng sau 3 giây” (`window.close()` chỉ chạy khi tab do app mở bằng `window.open`; không đóng được thì giữ nút).

Quy tắc dựng:
- **Trang không biết email/gói** — lúc callback chỉ mới có `code`, token đổi sau (`exchangeCode`). Không hiển thị tài khoản ở trang này; tài khoản hiện ở terminal/app sau khi đổi xong. Nếu đổi token lỗi, terminal/app báo “Đăng nhập thất bại: …” (thẻ `failed` trong app).
- Dùng `html/template` (không `fmt.Fprintf` — CSS có `%` sẽ vỡ), escape thông điệp lỗi, viết hoa chữ đầu khi hiển thị.
- Tự chứa: CSS + SVG inline, không tải font/ảnh ngoài (font hệ thống, fallback cho Be Vietnam Pro), không JS ở bản cli; bản web chỉ một đoạn đóng tab.
- Header: `Cache-Control: no-store`, `Referrer-Policy: no-referrer` (URL chứa `code`), `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'`. Gọi `history.replaceState` để xoá `code`/`state` khỏi thanh địa chỉ (bản web).
- Hoạt ảnh một lần: vòng toả 1.6s + nét tick vẽ 0.45s; tắt khi `prefers-reduced-motion`.
- Dòng cuối: “Trang này do Noverse mở trên chính máy tính của bạn và sẽ tự tắt sau khi xong. Noverse không nhìn thấy mật khẩu ChatGPT của bạn.”

### 8.2 Thẻ kết quả trong app (bước Nhà cung cấp, Cài đặt › Nhà cung cấp)
`oauth`: `idle` (2 lựa chọn trình duyệt / mã thiết bị) → `browser` (đang chờ tab mới, hạn 10 phút) | `device` (mã `XXXX-XXXX` + `auth.openai.com/codex/device`, đang chờ) → `done` | `failed`.
- **done**: thẻ viền mint, tick có quầng sáng, “Đã đăng nhập ChatGPT Plus/Pro” · “qua trình duyệt / qua mã thiết bị · vừa xong” · chip gói (`cred.Plan`). Bảng: Tài khoản (`cred.Label()` — email), Giữ kết nối “Tự động gia hạn — không phải đăng nhập lại”, Model dùng được (từ `codex.ListModels`) → chọn ở bước sau. Chân thẻ: “Ngắt kết nối bất cứ lúc nào trong Cài đặt › Nhà cung cấp AI” + [Dùng tài khoản khác]. Lưới nhà cung cấp thu lại để thẻ nằm ngay trong tầm nhìn.
- **failed**: thẻ viền đỏ, lý do bằng lời thường (vd “Liên kết đăng nhập đã cũ nên Noverse chặn lại để bảo vệ tài khoản của bạn.”), [Thử lại] + [Dùng mã thiết bị]; thông điệp gốc chỉ trong Chi tiết kỹ thuật. Gợi ý mã thiết bị khi lỗi là “không mở được cổng callback 127.0.0.1:1455 (đang bị chiếm? thử `--device`)”.
- Luồng trình duyệt chỉ hợp lệ khi trình duyệt chạy **cùng máy** với `serve` (callback về `localhost`). Mở app từ điện thoại/máy khác → ẩn lựa chọn trình duyệt, chỉ còn mã thiết bị.
- Chung cho mọi provider OAuth sau này: tham số hoá `provider.Name()`, đường dẫn credential, nguồn model; không dùng logo/thương hiệu nhà cung cấp, chỉ tên.

## 9. Ngôn ngữ dễ hiểu

Áp dụng cho mọi màn A1–A4 (xem quy tắc chung ở `07-copy.md` › Giọng văn).
- Không hiện IP, cổng, `localhost`, đường dẫn tệp cấu hình, tên lệnh ở nội dung chính. Danh sách thiết bị chỉ hiện tên thiết bị + thời gian hoạt động.
- Panel phải của onboard là **“Thiết lập sẽ lưu”**: các dòng tên → giá trị bằng lời thường. Tệp/API/JSON và ghi chú GAP nằm trong **Chi tiết kỹ thuật** thu gọn (mặc định đóng) — dành cho người dựng, không phải người viết truyện.
- Cài đặt › Nhà cung cấp giữ cùng cấu trúc artboard: provider chính mở, các provider khác là hàng thu gọn. Thẻ mở có thông tin đăng nhập/khoá, Base URL, các model và thời gian stream; model có chip vai trò riêng. Đổi provider đang mở không làm mất bản nháp của SaveBar dùng chung.
- **Không hiện lệnh dòng lệnh trên UI** (trừ trang OAuth mở từ dòng lệnh). Quên mật khẩu quản trị → mã khôi phục; bật Wi-Fi nhà → nút trong UI (GAP-33, `20-settings-rules.md` §6).

## 10. Chọn nơi lưu truyện (bước 5 · Tuỳ chỉnh › Nâng cao) — bổ sung 25/10

**Vì sao:** bản cũ lấy luôn `serve --root` (mặc định `~/Truyen`) rồi hiện `filepath.Base(root)` (ra chữ “lib”, “Truyen”). Người dùng chưa bao giờ được chọn, và tên trần không cho biết thư mục nằm ở đâu.

**Artboard:** `Onboard.dc.html` step `library`, tweak `lib`: `existing` · `fresh` · `picking` · `browse` · `picked` · `readonly`. `Settings.dc.html` section `adv`, nhóm “Vị trí lưu trên máy chủ”, dòng **Nơi lưu truyện › Đổi thư mục…**

### 10.1 Bố cục bước
- Tiêu đề **“Lưu truyện ở đâu?”**. Câu dẫn: “Mọi dự án, truyện và bản xuất nằm trong một thư mục trên máy chủ. Đổi sau được.”
- Danh sách lựa chọn (radio card, chọn 1):
  1. **Thư mục đang dùng.** Hiện khi `GET /api/library` có dự án. Ví dụ: *Truyen* · “Thư mục nhà › Truyen” · nhãn **Đang dùng** · “1 dự án · 1 truyện”.
  2. **Gợi ý thư mục mới.** *Truyện Noverse* · “Tài liệu › Truyện Noverse” · nhãn **Gợi ý** · “Thư mục mới”. Tạo khi bấm Tiếp tục.
  3. **Thư mục người dùng chọn.** Chỉ hiện sau khi đã chọn. Nhãn **Bạn chọn** · trạng thái “Thư mục trống” hoặc “Không ghi được”.
- Nút viền gạch **“Chọn thư mục khác…”**, chữ phụ “iCloud, Google Drive, ổ ngoài…”. Khi đang mở hộp chọn của máy: “Đang mở cửa sổ chọn thư mục trên máy…”, chữ phụ “chọn xong quay lại đây”.
- Hộp trạng thái dưới danh sách (màu theo loại):

  | Loại | Màu | Chữ |
  |---|---|---|
  | `found` | xanh | Đã có N dự án Noverse (“{tên}”) với M truyện — Noverse dùng luôn, không đổi gì. |
  | `new` | xanh dương | Noverse sẽ tạo thư mục “{tên}” trong {nơi}. Truyện cũ ở “{tên cũ}” vẫn giữ nguyên, nhập lại được bất cứ lúc nào. |
  | `empty` | xanh | Thư mục trống, ghi được. Nếu là iCloud/Drive, nhắc bật “Luôn giữ trên máy”. |
  | `readonly` | đỏ | Noverse không ghi được vào thư mục này (chỉ đọc)… → **khoá nút Tiếp tục** |

- Ô tóm tắt ở bước Hoàn tất: **Lưu ở:** “iCloud Drive › Truyện của Dat”.

### 10.2 Hiển thị đường dẫn
- **Không bao giờ hiện tên thư mục trần** (`lib`, `Truyen`) hay đường dẫn tuyệt đối. Luôn hiện dạng **Nơi › … › Thư mục**.
- Nơi là tên thân thiện tính trên máy chủ:

  | Nơi | macOS | Windows / Linux |
  |---|---|---|
  | Thư mục nhà | `~` | `~` |
  | Tài liệu | `~/Documents` | Documents / `XDG_DOCUMENTS_DIR` |
  | Màn hình | `~/Desktop` | Desktop |
  | iCloud Drive | `~/Library/Mobile Documents/com~apple~CloudDocs` | — |
  | Google Drive, OneDrive | `~/Library/CloudStorage/*` | thư mục đồng bộ |
  | Ổ ngoài | `/Volumes/*` | ổ đĩa / `/media/$USER/*` |

- Đường dẫn đầy đủ chỉ dành cho quản trị: tooltip, hoặc Nâng cao › “Mở trong Finder”.

### 10.3 Hành vi “Chọn thư mục khác…”
1. Truy cập trực tiếp từ máy chủ (`isDirectLoopback`): gọi `POST /api/system/pick-folder {start}` để mở hộp chọn của hệ điều hành. Kết quả:
   - `{path}` → kiểm tra (10.4) rồi thêm lựa chọn 3.
   - `{cancelled:true}` → không làm gì.
   - `501` (máy không có hộp chọn, ví dụ Linux không có zenity/kdialog) → chuyển sang bước 2.
2. Từ điện thoại, máy khác, hoặc khi bị `501` / `403`: mở **hộp duyệt thư mục trong app** (GAP-34).
   - Cột trái: các Nơi (10.2).
   - Danh sách: chỉ thư mục, ẩn thư mục chấm và thư mục hệ thống. Mỗi dòng có huy hiệu `ainovel` (thư viện có sẵn) hoặc `chỉ đọc`, và số mục.
   - Có nút **Thư mục mới**.
   - Chân hộp: “{Nơi › Thư mục} · {kết luận kiểm tra}” + nút **Dùng thư mục này**. Nút bị khoá khi thư mục chỉ đọc.
   - Nền hộp thoại đặc (`#18181A`), không trong suốt.
3. Bấm **Tiếp tục** gọi `PUT /api/library {root}`:
   - Lựa chọn “Thư mục mới” → `POST /api/fs/mkdir` trước, rồi mới gọi `PUT /api/library {root}`.
   - Thành công → lưu vào `~/.ainovel/serve.json` (đã có, `SaveServeRoot`).
   - `409` (có truyện đang chạy) → “Đang có truyện chạy — hãy đóng tất cả trước khi đổi thư viện”.

### 10.4 Kiểm tra thư mục (`GET /api/library/inspect?path=`)
Trả `{kind: found|empty|other|readonly|missing, projects, stories, items, place, display}`. `display` là chuỗi “Nơi › … › Thư mục” đã dịch sẵn. `readonly` dùng cùng cách thử của `library.CheckRoot` (tạo rồi xoá thư mục tạm).
- `other`: “Có tệp khác không phải truyện — Noverse không đụng tới, nhưng nên chọn thư mục riêng” (vàng, vẫn cho dùng).

### 10.5 Đổi sau khi đã thiết lập
Tuỳ chỉnh › Nâng cao › “Vị trí lưu trên máy chủ”:
- **Nơi lưu truyện** “Tài liệu › Truyện Noverse · 3 dự án” → **Đổi thư mục…** mở đúng bộ chọn trên.
- Hộp xác nhận ghi rõ: “Truyện cũ không bị di chuyển. Thư mục cũ vẫn mở lại được.”
- **Mở nơi lưu truyện** → “Mở trong Finder” (chỉ khi truy cập từ máy chủ).

### 10.6 Triển khai
- **Go**: `internal/library/places.go` (Nơi + `Display` → “Nơi › … › Thư mục”), `internal/library/inspect.go` (`Inspect`, `ListFolders`), `internal/entry/web/fs.go` (`GET /api/fs/places`, `GET /api/fs/list?path=`, `POST /api/fs/mkdir` — chỉ admin, giới hạn trong các Nơi), `internal/entry/web/library_inspect.go` (`GET /api/library/inspect?path=`, `rootDisplay`), `internal/entry/web/reveal.go` (`POST /api/system/pick-folder` — chỉ loopback, 501 khi không có osascript/zenity/kdialog, 403 khi truy cập từ xa), `internal/entry/web/projects.go` (`PUT /api/library {root}` → `switchLibraryRoot`: 409 khi có truyện chạy, ghi `~/.ainovel/serve.json` chỉ sau khi thành công), `internal/library/serveconfig.go` (`ResolveServeRoot`: cờ `--root` → `serve.json` → `~/Truyen` chỉ khi đã tồn tại; cài mới → `""`).
- **Web**: `web/src/screens/onboard/libraryChoice.ts` + `OnboardSteps.tsx › LibraryStep` (radio card Đang dùng / Gợi ý / Bạn chọn, hộp trạng thái, `commit()` = `fs/mkdir` khi cần rồi `PUT /api/library {root}`), `web/src/screens/dialogs/FolderBrowserDialog.tsx` (`useFolderPicker` + bộ duyệt trong app), `web/src/screens/dialogs/libraryInspect.ts` (chữ/màu chung), `web/src/screens/settings/sections/LibraryRootRow.tsx` + `advanced/LocationRows.tsx` (Nâng cao).
- **Quy tắc rơi về** (`useFolderPicker.pick`): `canReveal` (loopback) → `pick-folder`; `canReveal=false`, `501` hoặc `403` → bộ duyệt trong app trên `fs/places` + `fs/list` + `fs/mkdir` + `library/inspect`. Mọi chỗ hiển thị đều qua `rootDisplay`/`display` — không bao giờ tên trần hay đường dẫn tuyệt đối.

## 11. Lần chạy đầu trên web (A0 · `Setup.dc.html`) — bổ sung 25/10 <!-- goi-m -->

- **Màn:** `22-brief-2510.md` › A0. **API:** GAP-41 (`06-data-contract.md`). **Việc dev:** `21-dev-prompt.md` › Gói M mục 1.
- **Bảo mật:**
  - Mã chỉ in ra ở máy chủ, không bao giờ trả qua API.
  - So sánh mã theo hằng thời gian.
  - Giới hạn 5 lần sai tính chung cho mã (không theo IP, vì chỉ có một mã), khoá 10 phút.
  - `setupToken` dùng một lần và sống không quá thời hạn còn lại của mã.
  - Chỉ nhận khi chưa có tài khoản nào. Kiểm lại trong cùng giao dịch ghi tài khoản để hai người không cùng tạo được.
- **Qua LAN:** được phép nhập mã từ máy khác trong mạng (người cài đọc mã trên màn hình máy chủ). Có mã nghĩa là có quyền vào máy.
- **Hết hạn giữa form** (`Setup.state=tokenExpired`): `POST /api/setup/admin` trả 410 → giữ dữ liệu form, hiện ô nhập mã mới ngay trong form, verify lại rồi tạo.
- **Mã khôi phục:** hiện ở bước Hoàn tất của A3 như hiện có, không chen giữa A0 và A3.
- **Thanh bước:** A0 có 3 bước (bước 3 “Thiết lập thư viện · 7 bước”); A3 thêm khối “Lần chạy đầu” ✓✓ ở đầu rail.
- **a11y:**
  - 8 ô là một input duy nhất, `aria-label="Mã thiết lập, 8 ký tự"`.
  - Lỗi và số lần thử còn lại đọc qua `aria-live="polite"`.
  - Khi bị khoá, giờ mở lại ghi bằng chữ.

## 12. Đăng nhập chung với Viderse: chuyển app và câu chữ (Gói AO, 31/10) <!-- goi-ao -->

Chỉ khi `STORE_DRIVER=postgres` (có `grants`). Noverse (`nov.aarverse.studio`) và Viderse (`vid.aarverse.studio`) dùng chung kho tài khoản và một phiên. Phía Viderse là Gói V26 (`apps/viderse/docs/design/v1/08-dev-prompt.md`); artboard vẽ cả hai app nằm ở Viderse: `Auth.dc.html` (`screen=menu`, `app=noverse` là menu của Noverse) và `MAuth.dc.html` (`screen=account`, `app=noverse` là tab Thêm). Canvas Noverse (bản 130) đã vẽ lại các màn này <!-- ao-canvas -->:
- `Login` · `kicked`: none · self · other · password · admin · unknown (khối lý do trên form ở `state` login · password · other; `expired` giữ màn cũ) · `viderse`: yes · no (nhóm Ứng dụng trong `state=menu`, nút *Mở Viderse* ở `state=denied`) · `state=denied` (chưa được cấp quyền vào Noverse) · `state=devices` (câu phiên chung, đổi mật khẩu kể cả máy này) · dòng phụ “Một tài khoản dùng cho Noverse và Viderse” ở mọi màn đăng nhập.
- `Members` · `app`: clean · changed · viderseOff · none (mục 3 Quyền vào ứng dụng, chọn sẵn Linh); `who=dat` cho trạng thái quản trị (khoá cả hai ô); `dlg` reset · lock có câu “ở cả Noverse và Viderse”.
- `MMore` · `viderse`: yes · no (nhóm Ứng dụng trên nhóm Tài khoản); dòng phụ dưới *Đăng xuất*; trang Thiết bị, Đổi mật khẩu, sheet chuyển tài khoản dùng câu phiên chung.
- `Phone` · `kicked` (dùng với `state=login`): dòng phụ + khối lý do như máy tính.
- Logo Viderse (ống kính đỏ trong vòng bốn màu) và logo Noverse nằm trong vòng tròn nền `#18181B`; dòng app đang mở dùng nền `rgba(255,255,255,.08)`, không viền màu một bên.

### 12.1 Chuyển app (menu tài khoản, F3)
- Thêm nhóm **Ứng dụng** sau các mục tài khoản, trước *Chuyển tài khoản*: dòng Noverse (app đang mở, nhãn *Đang mở*) và dòng **Viderse** (logo ống kính đỏ trong vòng tròn, tên, địa chỉ `vid.aarverse.studio` chữ mono, biểu tượng mở ở thẻ mới). Bấm → mở Viderse ở thẻ mới, đã đăng nhập sẵn.
- Chỉ hiện khi người dùng có quyền Viderse (`grants.viderse`). Không có quyền thì bỏ cả nhóm, không vẽ dòng khoá. Quản trị viên luôn có.
- Không thêm nút chuyển app ở đầu trang: menu tài khoản là chỗ duy nhất, giống Viderse.
- **Điện thoại** (tab Thêm): thêm nhóm **Ứng dụng** trên nhóm Tài khoản, cùng hai dòng.
- Địa chỉ app lấy từ `apps: AppLink[]` của `GET /api/me` (GAP: máy chủ biết nhưng chưa trả, xem Viderse hỏi đáp #50).

### 12.2 Câu chữ ảnh hưởng cả hai app
- **Đăng xuất** (menu, tab Thêm): dòng phụ “Khỏi cả Noverse và Viderse trên trình duyệt này”.
- **Chuyển tài khoản**: dòng phụ của từng tài khoản “Đăng xuất cả hai app rồi nhập mật khẩu của Linh”.
- **Đổi mật khẩu** (hộp thoại, Thiết bị & bảo mật): “Đổi xong, mọi thiết bị phải đăng nhập lại ở cả Noverse và Viderse, kể cả máy này.”
- **Thiết bị đang đăng nhập**: “Mỗi thiết bị một phiên chung cho cả hai app.” · *Đăng xuất mọi thiết bị khác*: “Thiết bị bị đăng xuất phải đăng nhập lại ở cả Noverse và Viderse.”
- **Thành viên › Quyền vào ứng dụng** (F5b mục 3): bật hoặc tắt một ô → câu ngay dưới hai ô “Linh sẽ bị đăng xuất khỏi mọi thiết bị ở cả Noverse và Viderse và phải đăng nhập lại.” Tắt ô Viderse: “Linh vẫn vào Noverse được; mở Viderse sẽ thấy thông báo chưa được cấp quyền.” Tắt cả hai: giữ câu đã có.
- **Thành viên › Đặt lại mật khẩu, Khoá**: hệ quả trong hộp xác nhận ghi “đăng xuất khỏi mọi thiết bị ở cả Noverse và Viderse”.

### 12.3 Màn đăng nhập: vì sao phải đăng nhập lại
Một khối ngắn trên form (A1 › login, password), theo lý do máy chủ trả trong 401 (`reason`, `app`):

| Lý do | Câu |
|---|---|
| tự đăng xuất | **Đã đăng xuất** · Đã đăng xuất khỏi cả Noverse và Viderse trên trình duyệt này. |
| đăng xuất ở Viderse | **Bạn vừa đăng xuất ở Viderse** · Hai app dùng chung một lần đăng nhập, nên Noverse cũng cần đăng nhập lại. |
| đổi mật khẩu | **Mật khẩu vừa được đổi** · Mọi thiết bị phải đăng nhập lại ở cả Noverse và Viderse. Dùng mật khẩu mới. |
| quản trị viên đổi | **Quản trị viên vừa thay đổi tài khoản này** · Có thể là quyền vào ứng dụng hoặc mật khẩu đã được đặt lại. Đăng nhập lại để tiếp tục. |
| hết hạn | giữ A1 › expired như cũ |
| không rõ | **Phiên đăng nhập đã kết thúc** · Có thể bạn vừa đăng xuất hoặc đổi mật khẩu ở Noverse hay Viderse. Đăng nhập lại để tiếp tục. |

Dòng phụ dưới tiêu đề đăng nhập thêm “Một tài khoản dùng cho Noverse và Viderse”.

### 12.4 Chưa được cấp quyền vào Noverse
Đối xứng với Viderse: thành viên chỉ có quyền Viderse đăng nhập Noverse → màn “Chưa được cấp quyền vào Noverse” (giữ phiên), thẻ tài khoản, *Mở Viderse* (khi có quyền Viderse), *Thử lại*, *Chép lời nhờ quản trị viên cấp quyền*, *Đăng xuất* + “Đăng xuất khỏi cả Noverse và Viderse trên trình duyệt này.” Bố cục như Viderse `Auth` · `screen=denied`, đổi logo và màu theo Noverse.
