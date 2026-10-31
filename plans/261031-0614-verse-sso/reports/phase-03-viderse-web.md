# Viderse web — cổng phiên trên kho tài khoản chung

Phần UI của phase: Viderse dùng cùng kho tài khoản/cookie phiên `verse_session`
với Noverse. Cổng app phân loại chặt phản hồi `/api/me`: chỉ 401 mới dẫn tới
Setup/Login; 403 `app_access_denied` giữ user và mở màn bị chặn;
`mustChangePassword`/`403 password_change_required` mở cổng đổi mật khẩu bắt
buộc; 503/mất mạng mở màn lỗi có Thử lại — không bao giờ Setup/Login giả.

## Đã thay đổi

**`src/api/types.ts`** — `AppId` (`'noverse' | 'viderse'`), `AppGrants`
(`Partial<Record<AppId, Role>>`); `User.grants?: AppGrants | null` và
`User.mustChangePassword?: boolean` (wire của `UserView`). `Session` wire
(`identity.Session`) cho `/api/me/sessions`. `SetupAdminResult.recoveryCode`
đổi thành `string | null` theo contract server: admin đã tạo + phiên đã đặt
nhưng chưa cấp được mã → `null`, không phải lỗi.

**`src/api/sessionSignal.ts`** — từ một tín hiệu thành ba:
`onSessionExpired` (chỉ 401), `onAccessDenied(user)` (403
`app_access_denied`), `onPasswordChangeRequired` (403
`password_change_required`). Listener chỉ đổi trạng thái cổng, không phát
request → không vòng lặp.

**`src/api/request.ts`** — `settle()` đọc payload trước khi quyết định: 401 →
`notifySessionExpired`; 403 + `code:"app_access_denied"` → `notifyAccessDenied`
(kèm safe `user` trong body, có thể null); 403 +
`code:"password_change_required"` → `notifyPasswordChangeRequired`. Mọi 403
nghiệp vụ khác chỉ là `ApiError` cho caller — không coi là mất quyền app hay
hết phiên.

**`src/api/client.ts`** — thêm `changePassword` (`{current,new}` →
`{ok, reauthenticate}`; sai current → 403, kho lỗi → 503), `mySessions`,
`revokeAllMySessions`, `revokeSession` (auth-neutral, chỉ phiên của mình).
Hai parser: `accessDeniedUser(error)` (403 + code + `user` object → trả User)
và `passwordChangeRequired(error)` (403 + code → boolean). Header comment cập
nhật đúng contract mới.

**`src/api/session.tsx`** — viết lại provider:
- `refresh()` trả `'user' | 'denied' | 'password' | 'anon' | 'down'`. 200 →
  user + cờ từ `user.mustChangePassword`; 403 denied → `accessDenied` giữ safe
  user (đồng thời nằm trong `user` để các route auth-neutral vẫn dùng cookie
  này); 403 forced → `mustChangePassword`; status 0 hoặc ≥500 (cả
  `/api/me` lẫn `/api/setup` sau 401) → `serverReachable=false`, outcome
  `'down'`, KHÔNG hỏi/suy `setupNeeded`; 401 → dọn phiên rồi hỏi `/api/setup`.
- `signIn()` — gọi `/api/login` của Viderse; 2xx → `setUser`; 403 denied →
  `setAccessDenied` (không ném lỗi form); lỗi khác ném ra cho form.
- `signOut()` — `logout()` revoke trước; lỗi ≠401 thì ném (không báo giả đã
  đăng xuất); 401 = phiên đã mất → dọn luôn.
- `endSession()` — dọn `user`/`accessDenied`/`mustChangePassword` khi server
  đã thu hồi phiên (đổi mật khẩu xong, revoke-all, 401 từ signal).
- `setAccessDenied(user)` đặt user đó vào cả `user` lẫn `accessDenied`: giữ
  phiên + hiển thị người đăng nhập, Root chặn dữ liệu qua cờ.

**`src/App.tsx`** — Root gating: `full = user && !accessDenied &&
!mustChangePassword` điều khiển cả `startDataEvents` lẫn hash-normalize. Thứ
tự cổng: chưa kiểm → spinner; `accessDenied` → `AccessDeniedScreen`;
`mustChangePassword` → `ChangePasswordScreen`; `!serverReachable && !user` →
`ServerDownScreen`; `setupNeeded` → Setup; `!user` → Login; còn lại shell.
Hash route không vượt được hai cổng (render thay shell, không nút bỏ qua).

**`src/screens/auth/AccessDeniedScreen.tsx`** (mới) — avatar + vai trò của
user bị chặn, đúng câu "Tài khoản của bạn chưa được cấp quyền dùng Viderse.
Vui lòng liên hệ quản trị viên.", **Thử lại** (`refresh()`; `'anon'` →
`endSession`), **Đăng xuất** (`signOut` revoke thật — lỗi thì báo lỗi, giữ
màn), `HintLine` ghi rõ thoát cả hai app. Không có nút đổi tài khoản giấu phiên.

**`src/screens/auth/ChangePasswordScreen.tsx`** (mới) — `ChangePasswordForm`
(`{forced}`): 3 ô PasswordField (hiện tại/mật khẩu tạm — mới — nhập lại), tối
thiểu 8 ký tự, mismatch chặn submit phía client. `changePassword` thành công →
`toast.ok('Đã đổi mật khẩu. Vui lòng đăng nhập lại.')` + `endSession()` —
không gọi lại `me()` (server đã thu hồi mọi phiên + xoá cookie; refresh sẽ
trông như đổi thất bại). 403 → `wrongCurrent`; lỗi khác (503/0) → message
server. `ChangePasswordScreen` bọc `AuthShell` cho cổng bắt buộc, không nút bỏ qua.

**`src/screens/auth/ServerDownScreen.tsx`** (mới) — `EmptyState` +
`AuthShell` + nút Thử lại (`refresh()`): đứng ở cổng thay Login/Setup khi
không kiểm được phiên (503/5xx/mất mạng), không suy ra store trống.

**`src/screens/auth/LoginScreen.tsx`** — submit qua `signIn()` của session:
2xx → vào app, 403 denied → màn bị chặn (không hiện lỗi form), 401/429 giữ
nguyên message server. Không redirect sang app khác.

**`src/screens/auth/SetupScreen.tsx`** — `recoveryCode: null` → bước recovery
hiện `recoveryUnavailable` ("đã tạo tài khoản, đăng nhập xong phát mã qua
Noverse") thay cảnh báo mã duy nhất — không bắt owner setup lại.

**`src/screens/account/AccountScreen.tsx`** — section "Đổi mật khẩu" dùng
`ChangePasswordForm` (tự nguyện — cùng hành vi reauth); Đăng xuất qua
`signOut()` + dòng `sharedLogoutNote` nói rõ thoát cả hai app.

**`src/i18n/vi.ts`** — `auth.serverDownTitle/serverDownBody`,
`auth.accessDeniedTitle/accessDenied`, nhóm `auth.passwordForm` (forceTitle,
forceBody, current, currentTemp, next, confirm, hint, mismatch, wrongCurrent,
failed, submit, changing, changed, reauthNote), `setup.recoveryUnavailable`,
`account.passwordTitle`, `account.sharedLogoutNote`. (Nhóm đặt tên
`passwordForm` vì `auth.password` đã là nhãn trường nhập.)

## Quyết định / deviation

- `accessDenied` giữ safe user (và `user` cũng trỏ tới đó): Thử lại/Đăng xuất/
  `me/sessions` đều là route auth-neutral hợp lệ với cookie phiên này — khác
  cách Noverse để `user=null` + `deniedUser` riêng nhưng tương đương về mặt
  chặn dữ liệu (Root gate trên `full`, không chỉ `user`).
- Cờ `mustChangePassword` nằm trong SessionState (không chỉ trên `user`) để
  tín hiệu 403 `password_change_required` từ request nghiệp vụ bất kỳ cũng mở
  được cổng kể cả khi user đã mount với flag cũ.
- `/api/me` 403 `password_change_required` là defensive path (contract coi
  `/api/me` auth-neutral): vẫn xử lý đúng nếu server trả mã này.
- `signOut` giữ hành vi "lỗi revoke → báo lỗi, giữ phiên" của Viderse
  (trung thực với contract revoke-trước-clear), thay vì dọn UI khi lỗi như
  Noverse; riêng 401 (phiên đã mất) vẫn dọn luôn.
- `mySessions`/`revokeAllMySessions`/`revokeSession` thêm để đủ client contract
  auth-neutral; UI liệt kê thiết bị không nằm trong scope màn này.

## Gates

- `npx tsc -b --noEmit`: PASS, 0 lỗi (tsconfig.tsbuildinfo đã revert, không
  nằm trong diff).
- `npx vitest run --maxWorkers=2` (trong `lockf -k /tmp/verse-heavy.lock nice
  -n 10`): **30 files, 318 tests PASS** (273 cũ + 45 mới/đổi, 0 fail).
- `npm run size`: PASS — `index-*.js` 146.1 kB gzip (ngưỡng 250 kB; đo trên
  manifest build hiện có, integrator build lại sau).
- Không chạy `npm run build` theo chỉ đạo (bundle tracked do integrator dựng).

## Tests mới / đổi

- `src/api/session.test.tsx` (mới, 20): phân loại boot 200/403-denied/403-
  forced/401/503/offline/setup-503; tín hiệu 403 hai mã từ route nghiệp vụ vs
  403 thường vs 401; refresh denied→granted/denied→denied/anon; signIn 2xx
  forced + 403 denied + 401 ném lỗi; signOut ok / lỗi 500 giữ phiên.
- `src/screens/auth/AccessDeniedScreen.test.tsx` (mới, 5): đúng câu thông báo
  + user + gợi ý hai app + không nút đổi tài khoản; Thử lại granted→vào app /
  401→về login; Đăng xuất revoke→về login; lỗi revoke giữ màn.
- `src/screens/auth/ChangePasswordScreen.test.tsx` (mới, 5): success → body
  `{current,new}` + toast "Đã đổi mật khẩu. Vui lòng đăng nhập lại." + phiên
  UI dọn + `/api/me` chỉ gọi lúc boot; 403 wrong-current riêng; 503 message
  khác; mismatch chặn submit; không nút bỏ qua.
- `src/App.test.tsx` (viết lại, 14): boot→Setup/Login giữ nguyên; offline &
  503 `/api/me` & setup-503 → màn lỗi + Thử lại (không `#lg-user`/`#setup-code`,
  không gọi `/api/setup` sau 503 me); denied → đúng câu + không `#lg-user` +
  không `/api/events`/`/api/status`; Thử lại→shell; Đăng xuất→logout POST→login;
  `mustChangePassword` + hash `#/settings` vẫn ra cổng đổi mật khẩu + không nav
  + không SSE; đổi xong → login + toast; login 403 → màn bị chặn không lỗi form;
  login user-forced → cổng đổi mật khẩu; shell/account + login 2xx → shell.
- `src/api/client.test.ts` (+4): `accessDeniedUser`/`passwordChangeRequired`
  đúng mã/trạng thái (401 cùng code không nhận); `changePassword` body +
  `reauthenticate`; `mySessions`/`revokeSession` theo contract.

## Thay đổi hành vi so với trước

- Trước: mọi lỗi `me` (kể cả 503/mất mạng) → `user=null` + hỏi `/api/setup` →
  có thể hiện Setup/Login giả. Sau: chỉ 401 mới đi đường đó; lỗi khác → màn
  lỗi Thử lại.
- Trước: `request` coi mọi 403 như lỗi thường và không phân loại. Sau: hai mã
  cổng được phát tín hiệu tập trung.
- Trước: logout chỉ `setUser(null)` sau `logout()` — lỗi revoke báo chung.
  Sau: `signOut` ném lỗi revoke (trừ 401) để UI hiện đúng trạng thái.
