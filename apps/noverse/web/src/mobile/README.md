# Khung điện thoại và máy tính bảng

`App` dùng `useLayoutMode()` với `matchMedia`: ≤640 px là `phone`, 641–1099 là `tablet`, từ 1100 là `desktop`. Điện thoại dùng `MobileApp`; máy tính bảng dùng `TabletApp({parsed})` từ `../tablet`; bản máy tính giữ `AppShell`. Mỗi khung chỉ gắn một lần safe area và provider, không tự thêm khoảng trống cho thanh tab vào màn con.

## Thanh tab và vùng cuộn

Thanh tab là viên nang kính: trái/phải 16 px, đáy 20 px, cao 64 px, bo 32 px, blur 26 px + saturate 180%. `.tind` rộng `(100% - 10px)/4`, trượt theo tab và nảy scale .7→1 trong 550 ms. Tab hiện tại dùng icon bulk tím, chấm cam ở Đọc báo chương cần duyệt. `data-tab-mode` có ba giá trị: `default`, `mini` (cuộn xuống quá 40 px: scale .92, dịch xuống 6 px và ẩn nhãn), `hide` (bàn phím/đọc tập trung/hết phiên/màn con). Phần nổi dùng `bottom:var(--m-float-bottom,92px)`; vùng cuộn có padding cuối để nội dung không bị viên nang che.

Import từ `mobile/runtime`:

- `MobileRuntimeProvider`: dùng chung bàn phím, kết nối, outbox và sheet hết phiên; `MobileApp` đã gắn, `TabletApp` tự gắn cùng `MobileFeedback`.
- `useMobileRuntime()` → `{keyboard,tabMode,down,offline}`. `keyboard` là số px, lấy từ `visualViewport` resize/scroll và đồng bộ `--kb` trên document. Khi bàn phím mở, thanh tab ẩn và `--m-float-bottom` chuyển sang `--kb`.
- `useTabMode(mode?)`: yêu cầu chế độ trong vòng đời màn/sheet; yêu cầu hide thắng mini. Không truyền mode để bỏ yêu cầu. Tự dọn khi unmount.
- `useMobileScroll(ref,{immersive?,onScroll?,disabled?})` → `{down,immersive,reveal}`. Chỉ gắn **một** listener scroll passive; đưa tính tiến độ đọc/lưu vị trí vào `onScroll(element)` thay vì thêm listener thứ hai. Cuộn lên hoặc `reveal()` hiện lại thanh. Nếu màn dùng `MobilePage` nhưng tự quản lý cuộn, truyền `manageScroll={false}`.
- `useStoryConnection({connected,reconnectAt,retryNow,blocked})`: báo trạng thái worker SSE cho băng mất kết nối chung. Singleton dữ liệu và worker đều dùng backoff 1→2→4→5 s trên phone/tablet, không thay chính sách máy tính.

## Các thành phần chung

- `MobilePage({title,backTitle?,onBack?,actions?,children,footer?,manageScroll?})`: thanh trên 52 px, vùng cuộn riêng, nút quay lại 44 px. Footer màn con có padding 16/16/30. Tự dùng `useMobileScroll` nếu không tắt.
- `BottomSheet({open,onClose,title?,height?,children,style?})`: portal, scrim 58%, focus trap/khôi phục focus, Esc/bấm nền đóng, thanh kéo pointer capture. Vào 440 ms, ra 260 ms; kéo xuống **hơn** 30% mới đóng. Không unmount trước animation ra.
- `ActionSheet({open,onClose,title,description?,actions})`: action `{label,onSelect,destructive?}` nhận Promise; khoá thao tác khi chờ, chỉ đóng sau thành công. Lỗi giữ sheet; Huỷ tách riêng.
- `PushPage({title,previousTitle,onBack,children})`: đẩy trang 340 ms.
- `useMobileToast()`: thông báo trên phần nổi, tự đóng 2500 ms.

Class chung: `m-card`, `m-group`, `m-row`, `m-row-copy`, `m-section-label`, `m-muted`, `m-field`, `m-button` (`secondary`,`danger`), `m-chip` (`on`), `m-hscroll`, `m-icon-button`, `m-push`, `m-error`, `m-inline-actions`. Ô nhập 16 px; vùng chạm ≥44 px; nhấn thu .96; tôn trọng reduced motion.

## Phiên, tài khoản và góp ý chờ gửi

`api/client.setUnauthorizedHandler` có thể trả `Promise<boolean>`: `true` sau xác thực sẽ thử lại đúng request/body/version một lần. Mobile giữ nguyên màn và bản nháp, hiện sheet đăng nhập không đóng được; các 401 đồng thời đợi cùng `sessionGate`. Sai mật khẩu/QR và logout không đi vào vòng đăng nhập lại. Phone dùng `screens/auth/mobile/MobileAuth`, tái sử dụng `PairScreen`/`Scanner`: camera thật, phát hiện mã đúng/sai, đèn flash khi phần cứng cho phép, fallback khi camera bị chặn, xác nhận trước POST `/api/pair`, trạng thái đang đăng nhập, mã hết hạn, mật khẩu và trang vào ứng dụng.

`mobile/accounts`:

- `readAccounts()` và `rememberAccount(user)` dùng đúng `localStorage['ainovel.accounts']`: username/tên/vai trò/lần cuối, không mật khẩu/cookie/thông tin thiết bị. Tối đa 8 mục, mới nhất trước.
- `switchAccount({username,password}|{code})`: logout → login/pair → xoá cache → đổi phiên và tải lại dữ liệu; lỗi sau logout thực sự đưa về trạng thái chưa đăng nhập. `beginAccountSwitch()` thu hồi phiên trước khi chuyển sang quét QR. Không tải lại trang HTML để tránh mất bản nháp trong RAM.
- `useAccountDraft(key)` → `[text,setText]`, `getDraft`, `setDraft`: giữ bản nháp trong bộ nhớ riêng từng tài khoản; không đưa vào localStorage.

`mobile/outbox` lưu IndexedDB `ainovel.mobile/outbox` theo username. `enqueueComment({project,story,chapter,input})`, `submitQueuedComment(...)` dùng `ParagraphCommentInput`; `useCommentOutbox(project,story,chapter)` trả danh sách chờ. M1 dùng `enqueueInput({project,story,text})` và `useInputOutbox(project,story)`. Chỉ xoá khi POST thành công; lỗi giữ nguyên, không gửi bản nháp của tài khoản khác. Provider gửi lại khi `online`, khi SSE mở lại hoặc sau đăng nhập; bình luận thành công làm mới cache comments. UI ghi “Chờ gửi · sẽ tự gửi khi có mạng”.

## Màn và liên kết

- M1 `MobileWorkspace({project,story,tablet?})`: đủ chạy/chờ duyệt/rảnh/tạm dừng/hoàn tất/lỗi/trống/mất mạng/hết phiên/gõ/lệnh/tham số/việc nền/xác nhận tạm dừng; registry chung, không hiển thị config/start/simulate/importsim trên điện thoại.
  Quyền M1 dùng chung cho điện thoại và cột trái T1: `canWriteStory()` quyết định có hiện lệnh nhanh, duyệt, tạm dừng, chạy tiếp, đổi model và mở lại truyện hay không. Người đọc chỉ thấy lệnh xuất (sheet chỉ có `/export`), vẫn dùng được Xuất EPUB và Nghe; xuất dựa trên `canReadStory()`, không yêu cầu quyền viết.
- M2 `MobileReader({project,story,chapter})`: đọc tập trung, góp ý/chờ gửi, chương/menu/hiển thị; dùng hook cuộn chung để không nhân listener.
- M3 `MobileAudio({project,story,volume,mode})`: thu nhỏ ở 92 px; `paths.audio(p,s,v,'listen') + '?chapter=N&play=1'` chọn/mở/tự phát chương N đã sẵn sàng. Chưa có audio thì hiện trạng thái chưa sẵn sàng, không phát chương khác thay thế.
- M4 `MobileMore({project,story,initialPage?,initialCommand?,onStoryChange?})`: tài khoản/truyện/model/thiết bị/mật khẩu/hướng dẫn. `?page=guide&cmd=export` mở hướng dẫn; `?command=<dòng lệnh>` ở Viết điền sẵn. `MobileFirstPassword({onDone})` là màn không có back/tab cho mật khẩu tạm, bắt buộc đổi (không có “Để sau”: máy chủ chặn mọi API tới khi đổi).
  Người đọc không gọi tài nguyên cấu hình/model cần quyền viết: chỉ hiện model đã biết ở dạng chỉ đọc kèm ghi chú quyền; chưa có tóm tắt thì ẩn hàng model. Lỗi tải model/thiết bị kết thúc bằng thông báo và nút thử lại, không giữ nhãn đang tải.
- M5 `CommandDeepLink({parsed})` lấy `GET /api/jobs/:id` để tìm truyện/kết quả rồi dựng `MobileCommandResults`; trang con không có tab. `/cmd/new?project=P&story=S&command=<dòng lệnh>` mở luồng mới; callback `onJob` thay bằng id thật để reload không chạy lại lệnh.
- M6 `MobileActivity({project,story})` dùng route activity: lịch sử GAP-31 phân trang và trực tiếp, gom theo chương, tìm/tô sáng, chip, chi tiết công cụ/Chép, viên sự kiện chưa đọc; không có tab.
- M7 `/notifications?view=inbox|prompt|system|denied|settings|lock|lockActions`: `NotifyPage` và Settings máy tính dùng chung tuỳ chọn GAP-30, lưu If-Match, cache `['notifications']` được làm mới bởi `notify.changed` và `ainovel.notification`. Xin quyền sau màn giải thích, dùng requestPermission/PushManager/VAPID thật; HTTPS/localhost; iOS 16.4+ phải thêm vào Màn hình chính. System/lock ghi rõ minh hoạ, không giả cấp quyền.
- T1: thanh trên 72 px với 4×94 px tab, cột Viết 372 px tái dùng MobileWorkspace chế độ tablet + cột đọc tối đa 560 px; readFocus thu cột trái 450 ms. Nghe/Thêm dùng màn mobile căn giữa 560 px.

Liên kết sâu `/read/:story/:ch?gate=1`, `/write/:story`, `/listen/:story`, `/more/devices`, `/cmd/:job` được đưa vào hash router. Truyện được tra từ thư viện thật; trùng tên cần project hint, không đoán dự án. Mã job có `/` phải URL-encode. Thông báo/SW gửi `{type:'ainovel.navigate',url,expired?}`; khi hết phiên lưu `ainovel.next`, đăng nhập xong trở lại đích. URL khác origin bị từ chối.

## PWA

Manifest standalone (Noverse · Xưởng viết truyện), theme/background `#0B0B0D`; biểu tượng PNG `any` 192/512 + `maskable` 512 + apple-touch-icon 180 (SVG bị iOS và nhiều launcher bỏ qua). SW chỉ cache manifest/icon và tài nguyên build công khai có MIME hợp lệ; không cache HTML, `/api`, phiên hoặc dữ liệu truyện. Cache `ainovel-mobile-static-v3` dọn bản cũ khi activate. Push do mô-đun thông báo quản lý: gọi action thật rồi focus/postMessage đích, hoặc mở cửa sổ `/#...`; không tạo luồng SSE riêng.
