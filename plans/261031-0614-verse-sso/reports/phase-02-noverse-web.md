# Noverse web — màn chặn cấp quyền, cấp quyền app, đổi mật khẩu đăng nhập lại

Phần UI của phase: đăng nhập chia sẻ qua kho tài khoản chung, cổng cấp quyền app
(403 `app_access_denied`), đổi mật khẩu thu hồi mọi phiên (`reauthenticate`), và
lọc đường quay lại sau đăng nhập.

## Đã thay đổi

**`src/api/types.ts`** — thêm `AppId` (`'noverse' | 'viderse'`), `AppGrants`
(`Partial<Record<AppId, Role>>`); `User.grants?: AppGrants | null`. Quy ước từ
máy chủ: object (kể cả `{}`) = kho chung Postgres; `null` = backend file độc
lập, không có khái niệm cấp quyền liên app.

**`src/api/client.ts`**
- `changePassword` trả `{ ok: boolean; reauthenticate?: boolean }` — đổi xong
  máy chủ đã thu hồi mọi phiên + xoá cookie.
- Mới `grantAppAccess` / `revokeAppAccess` → `PUT`/`DELETE`
  `/api/users/{username}/grants/{app}` (body `{role:'member'}`; DELETE gửi body
  rỗng/lý do vì máy chủ đọc `reason` trong body).
- Mới `accessDeniedUser(error)` — nhận diện 403 `code:"app_access_denied"`, trả
  user view an toàn đi kèm payload. 403 này KHÔNG đi qua handler 401.
- `OFFLINE_MSG` phục hồi (đã lỡ xoá trong lúc sửa).

**`src/api/session.tsx`** — viết lại provider:
- `deniedUser` trong context: phiên hợp lệ nhưng chưa được cấp quyền app.
  `refresh()` trả `'user' | 'denied' | 'anon'`; bắt `accessDeniedUser` trên
  `/api/me` → `deniedUser` set, `user` vẫn `null` (⇒ DataProvider/SSE, library,
  screen không mount). `setUser` luôn xoá `deniedUser`.
- `signOut()` — gọi `logout()` (revoke phiên máy chủ) rồi dọn UI về `#/login`;
  kể cả khi revoke lỗi vẫn dọn (không kẹt ở màn chặn).
- `endSession()` — dọn UI về `#/login` khi máy chủ ĐÃ thu hồi phiên (đổi mật
  khẩu, revoke-all): không gọi logout, không refresh-me.

**`src/api/storage.ts`** — `safeHashPath(value)`: chỉ đường nội bộ bắt đầu bằng
một `/` (ký tự 2 không phải `/`/`\\`), giải mã URL một lần rồi kiểm lại, chặn
`%2f`/`%5c`/`%00` (mở ra `//`, `\\`, null-byte khi router decode). `takeNextRoute`
đọc qua nó. Không bao giờ gán `location.href` — đích đến chỉ vào `location.hash`.

**`src/App.tsx`** — `deniedUser` → `<AccessDeniedScreen />` trước mọi screen
(setup/login/missing đều bị che); `user` null nên redirect-effect chỉ lưu
`ainovel.next` và điều hướng `#/login`, không mount dữ liệu.

**`src/screens/auth/AccessDenied.tsx`** (mới) — màn chặn: hiển thị user đang
đăng nhập (avatar + vai trò), đúng câu "Tài khoản của bạn chưa được cấp quyền
dùng Noverse. Vui lòng liên hệ quản trị viên.", nút **Thử lại** (`refresh()` →
granted: vào `authDestination`; denied: ở lại; anon: `endSession`), **Đăng xuất**
(`signOut` — revoke phiên máy chủ), và link "Đăng nhập tài khoản khác"
(`endSession` — giữ cookie). Không render qua route nên không bao giờ chạm login
loop.

**Các điểm vào denied** — `AuthScreen`, `PairScreen` (pair + đăng nhập dự phòng),
`screens/auth/mobile` (CompactAuth), `RecoveryForm`, `setup/index`: lỗi 403
`app_access_denied` → `setDeniedUser` thay vì báo lỗi form. `publicApi.ts` giờ
gắn `payload` vào `ApiError` (trước bỏ rơi) nên body denied đến được UI.

**`src/screens/settings/members.tsx` + `i18n/members.ts`**
- `sso = all.length > 0 && all.every(u => u.grants != null)` → chỉ kho chung.
- Section "Quyền vào ứng dụng" giữa Vai trò và Dự án: hai Checkbox Noverse/
  Viderse (icon `book-1`/`video-play`), lưu NGAY từng request qua PUT/DELETE,
  khoá trong lúc chạy, lỗi hiện `role="alert"` cạnh đúng checkbox — không qua
  savebar, không bao giờ hiển thị "đã lưu" khi request lỗi. Admin hệ thống:
  checkbox tắt + dòng "Được vào theo quyền quản trị" (kế thừa, không row grant
  giả). Độc lập hoàn toàn với quyền dự án.
- Dialog tạo thành viên: member thấy 2 checkbox (Noverse mặc định bật, Viderse
  tắt — trạng thái cập nhật functional, không mất khi bật nhanh hai ô); admin
  thấy dòng kế thừa. `onCreate` sau khi tạo chỉnh mọi app khác lựa chọn (máy chủ
  mặc định cấp Noverse): PUT grant / DELETE thu grant; mật khẩu tạm hiện trước
  nên lỗi chỉnh grant không mất tài khoản.

**Đổi mật khẩu → đăng nhập lại (3 caller)** — `DevicesDialog`, mobile
`Password`, onboard: thấy `reauthenticate` → toast "Đã đổi mật khẩu. Vui lòng
đăng nhập lại." + `endSession()` (xoá phiên UI, về `#/login`), KHÔNG
`refresh()`/`me()`/`onDone()` sau đó — tránh refresh-then-401 trông như đổi
thất bại. Ghi chú thiết bị cũ ("máy này vẫn đăng nhập") đổi thành mọi thiết bị
phải đăng nhập lại (i18n `t.account.passwordReauthNote` cho desktop; mobile
`Password.tsx` vốn hardcode tiếng Việt nên sửa tại chỗ theo convention file).

**`src/screens/auth/parts.tsx`** — `authDestination`/`nextRouteLabel` đọc qua
`safeHashPath` (`takeNextRoute` đã lọc; `getLastRoute` lọc lại).

## i18n

`vi.ts`: `t.auth.accessDeniedTitle/accessDenied/retry/differentAccount/
passwordChanged`, `t.account.passwordReauthNote`. `members.ts`: `apps`,
`appsHint`, `appsNote`, `appNames`, `appAccess`, `appInherited`, `appNoneHint`.
Không chuỗi nào tham chiếu phase/threat trong code hay test names.

## Quyết định / deviation

- Backend file: user JSON mang `grants:null` (phía máy chủ strip) ⇒ UI chỉ cần
  `grants != null` là phân biệt được; `{}` = member chưa được cấp app (vẫn là
  kho chung, UI hiện checkbox tắt).
- `POST /api/me/sessions/revoke-all` không có caller UI hiện tại (revoke mọi
  phiên của chính mình chỉ đi qua đổi mật khẩu / nút "Đăng xuất mọi thiết bị
  khác" — khác phiên hiện tại) nên client chưa thêm hàm thừa.
- Grant role cố định `'member'` trong UI (admin kế thừa tự động, không có nhu
  cầu grant 'admin').

## Gates

- `npx tsc -b --noEmit` (web): PASS, 0 lỗi.
- `npx vitest run --maxWorkers=2`: 191 files, **1557 tests PASS** (1514 cũ +
  43 mới, 0 fail).
- `npm run build` (`tsc --noEmit && vite build`): PASS — `index-*.js` 2.33 MB (683 kB gzip); cảnh báo chunk >500 kB có sẵn từ trước, package không có size-check script riêng.

## Tests mới

- `api/storage.test.ts` (mở rộng): `safeHashPath` 22 case loại (`//evil`,
  `#//evil`, `javascript:`, URL tuyệt đối, `%2f%2f`, `%5c%5c`, `/p%2f%2f`,
  mã hoá hỏng, null/empty/relative) + giữ đường nội bộ/`%20`/`%2E` +
  `takeNextRoute`/`getLastRoute` lọc tại điểm đọc.
- `api/session.test.tsx` (mới): 403 `/api/me` → deniedUser + `user:null` +
  `startDataEvents` KHÔNG gọi (SSE không mount); 401 → anon gate (`getSetup`
  gọi, deniedUser null); session đủ quyền → user + SSE mount; `refresh()`
  sau cấp quyền → 'user' + denied xoá; refresh vẫn denied → 'denied'; signOut
  → logout gọi + denied/user xoá.
- `screens/settings/members.test.tsx` (mở rộng, +6): file mode (grants null)
  không render điều khiển; toggle Noverse → DELETE + refresh; toggle Viderse →
  PUT không động `updateUser`; request lỗi → alert cạnh checkbox + trạng thái
  không đổi (không batch-success giả); admin kế thừa → checkbox disabled, không
  gọi API; tạo member (Noverse mặc định, tick Viderse, bỏ Noverse) →
  `revokeApp('ngoc','noverse')` + `grant('ngoc','viderse')`.
- `screens/account/DevicesDialog.test.tsx` (mới, 2): reauthenticate → toast đúng
  câu + `endSession` + `onClose` + `refresh` KHÔNG gọi; đổi lỗi → hiện lỗi,
  không endSession.
- `screens/more/mobile/Password.reauth.test.tsx` (mới): reauthenticate → toast +
  `endSession` + `onDone`/`invalidate` KHÔNG gọi.
- `screens/onboard/reauth.test.tsx` (mới): bước password của member →
  `changePassword` + toast + `endSession` + `refresh` KHÔNG gọi.
