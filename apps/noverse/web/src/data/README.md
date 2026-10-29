# Dữ liệu và phản hồi thao tác

Một `DataProvider` trong `App` giữ một luồng `/api/events` cho phiên đăng nhập. SSE dùng `fetch` để gửi header `Last-Event-ID`; mất luồng nối lại sau 1 → 2 → 4 → 5 giây (giữ 5 giây cho các lần sau). Chỉ luồng SSE điều khiển backoff; `/api/me` kiểm tra phiên một lần sau `hello` nối lại, không probe trong outage. Nối lại/reset làm mới mọi tài nguyên đang hiển thị. Focus/hiện tab giới hạn 15 giây mỗi khoá khi luồng đang mở; có mạng lại nối SSE ngay rồi làm mới cache khi nhận `hello`. Đăng xuất xoá cache.
Bootstrap `/api/me` có thể trả 401 khi trình duyệt sạch: App xoá phiên nhưng giữ nguyên các route công khai (`login`, `pair`, gallery phát triển), không chuyển trang ghép QR sang đăng nhập. Route cần đăng nhập vẫn chuyển về login.
`session.revoked` khớp cả username lẫn session ID hiện tại thì giữ trang đích, dừng SSE, xoá phiên và về `/login` kèm thông báo bị thu hồi; không đi qua 401 hoặc dùng lời báo hết hạn 30 ngày. Thu hồi thiết bị khác chỉ làm mới danh sách phiên.

## Đọc và tải lại

```tsx
const chapter = useResource(
  keys.story(project, story, 'chapter', number, source),
  ({ signal }) => getChapter(project, story, number, source, signal),
  { enabled: number !== null },
)
```

- Lần đầu dùng Skeleton đúng bố cục; lỗi lần đầu dùng trạng thái lỗi + `refresh`.
- Có dữ liệu rồi: giữ nội dung khi `refreshing` hoặc lỗi. Gắn `PanelRefreshBar` (vạch 2 px, không chữ) và `StaleBanner`; `updatedAt` chỉ vào tooltip “Cập nhật lúc hh:mm”.
- Fetcher nhận `AbortSignal`; truyền tới API để huỷ khi không còn người đọc. Các nơi dùng cùng khoá chia sẻ yêu cầu đang bay. Khoá là mảng JSON gồm string/number/boolean/null, không dùng object.
- Dùng `invalidate(keys.story(p,s,'chapters'))` sau ghi; `setData(key,result)` khi server trả tài nguyên mới. Prefix `[]` nghĩa là tất cả. Không mở SSE riêng cho việc nền: dùng `subscribeEvents(['job.created','job.progress','job.done'], handler)`; `events.onReconnect` cho kho ngoài cache.
- Một sự kiện chỉ tạo một đợt tải: prefix trùng/lồng nhau được gộp trước invalidate. `chapters` bao gồm cả nội dung, kế hoạch, dữ kiện, thẩm duyệt, sửa đổi và phục bút; revision chỉ invalidate phụ khi đang dùng polling, không lặp lại SSE.
- Mọi ghi quy tắc (HTTP hoặc ngoài đĩa) phát `config.changed` với `section:"rules"` và `scope` toàn cục/dự án/truyện; không phát thêm `story.changed` cho cùng ghi quy tắc. Khoá config/rules theo scope cũng tham gia danh sách watch.
- Font được Vite đóng gói từ `@fontsource` (Be Vietnam Pro 400/500/600, JetBrains Mono 400/500) và Literata variable `opsz` 7–72 (normal 400/500, italic 400). Không gọi Google Fonts; service worker lưu các tài nguyên `/assets/*.woff2` cùng nguồn theo bộ lọc MIME hiện có.

## Ghi, lạc quan, hoàn tác

```tsx
const action = useAction(save, {
  success: 'Đã lưu',
  invalidate: [keys.project(project)],
  optimistic: (input) => {
    const previous = cache.setData(key, input)
    return () => cache.restore(key, previous)
  },
  undo: async (result) => remove(result.id),
})
<Button pending={action.pending} success={action.success}
  disabled={action.busy || !connection.writable}
  onClick={() => action.run(input)}>Lưu</Button>
```

`busy` khoá ngay; `pending` sau 150 ms, giữ tối thiểu 400 ms. `run` trả kết quả hoặc `undefined` khi thất bại/bị chặn; lỗi được giữ trong `error` và phát toast tiếng Việt kèm Thử lại. Không làm rỗng nội dung khi tải lại. `optimistic` trả hàm rollback; `undo` thực hiện ghi đảo ngược, không chỉ đổi UI. Toast hoàn tác đếm ngược 5 giây, lỗi hoàn tác có Thử lại. Hành động hoàn tác chỉ thành công một lần.

Biểu mẫu giữ riêng bản server làm mốc và bản nháp. Không đổi mốc khi nhận SSE lúc đang sửa. PUT truyền `version` của mốc qua tham số cuối (`putConfig`, `updateProject`, `putStoryMeta`); `ConflictError.current` là tài nguyên GET hiện tại và `.version` là phiên bản mới. `ConflictBar` cho xem bản mới hoặc ghi lại với phiên bản mới; PUT thay cả tài liệu (config) thì “Ghi đè” chỉ áp các trường đã sửa lên `current` (`rebaseConfig`). Dùng `useUnsavedGuard(dirty)` và `useSaveShortcut(save, enabled)`; SaveBar ghi phím ⌘S/Ctrl+S. Rời trang trong ứng dụng (liên kết, `navigate()`, back/forward) hỏi bằng `confirmUnsavedLeave` của kit qua `addNavigationBlocker` trong router; đóng tab vẫn dùng `beforeunload` native.
Riêng Thông báo (GAP-30) là tuỳ chọn tài khoản lưu ngay: không dùng phạm vi config toàn cục/dự án/truyện, SaveBar cấu hình hoặc phím tắt ghi config.

## Trình đọc

Các panel, nội dung, đại cương, thẩm duyệt, xuất và nhận xét dùng cùng cache. Poll revision 5 giây chỉ bật khi SSE không `open`, chỉ chạy lúc tab hiện và có mạng. Nhận xét lưu theo chỉ số đoạn `<p>` từ 0, loại trừ tiêu đề; chọn nhiều đoạn giữ trích dẫn nhưng không đoán chỉ số. `comments.ts` dùng chung cho desktop/mobile, rollback khi ghi lỗi.

## Kiểm chứng

`npx vitest run src/data/data.test.ts` gồm 11 kiểm tra. Đã chạy mutation thật trong bản sao `/tmp/FData3-web` (không sửa nguồn dùng chung) và quan sát 9 thất bại trước khi khôi phục; hai regression bổ sung cũng được chạy trên mã trước khi sửa:

| Trường hợp | Mutation | Fail-before quan sát |
|---|---|---|
| Gộp yêu cầu | Bỏ chặn in-flight | `expected "spy" to be called 1 times, but got 2 times` |
| SWR | Xoá data lúc lỗi | `expected undefined to be 'bản cũ'` |
| Prefix | Bỏ lọc prefix | `expected "spy" to be called 1 times, but got 2 times` |
| Focus | Bỏ giới hạn thời gian | `expected "spy" to be called 1 times, but got 2 times` |
| Ánh xạ sự kiện | Bỏ chapter khỏi chapters | `to deep equally contain [ 'story', 'p', 's', 'chapter' ]` |
| Nối lại | Delay = 0 | `expected 1792872217341 to be 1792872218341` |
| Pending | Delay = 1 ms | `expected true to be false` |
| Rollback | Bỏ gọi rollback | `expected 'mới' to be 'cũ'` |
| Hoàn tác | undoMs = 0 | `expected +0 to be 5000` |
| Huỷ nội bộ | Fetcher trả AbortError nhưng signal cache chưa huỷ | `expected 'loading' to be 'error'` |
| Nguồn hoàn tác | Đổi callback sau khi đổi chương | `expected "spy" to be called 1 times, but got 0 times` |

Mock: `AINOVEL_MOCK_EXTERNAL_EDIT=1 npm run dev:mock -- --port 5199 --strictPort` đổi phong cách toàn cục mỗi 12 giây và phát sự kiện. Mở hai tab Cài đặt → Sáng tác; lưu ở tab A cập nhật tab B. Để bản nháp B chưa lưu, ghi A rồi lưu B để hiện ConflictBar.
Mock `/api/config/presets` dùng đúng 12 provider, thứ tự, nhãn, protocol và model mặc định từ `bootstrap.setupProviders` và `web.servePresets`; cấu hình mẫu và OAuth dùng ID `openai-codex`, không thêm radio alias `codex`.

## Workspace và Nhật ký tiến độ

`useStoryEvents` đọc snapshot bằng `useResource`; snapshot và danh sách sự kiện worker đều đặt vào cache dùng chung. Giữ luồng worker `/stories/:s/events/live` cho delta ghi theo `requestAnimationFrame`, không mở thêm luồng gateway: `/api/events` vẫn là singleton của `DataProvider`. Đại cương, đồng sáng tác, góp ý và thông tin hệ thống dùng cùng tầng cache; sự kiện `done` làm mới các khoá của truyện.

Lịch sử lưu trên đĩa dùng chung `useActivityHistory` cho Nhật ký máy tính, điện thoại và panel Bàn viết: `GET /stories/:s/events` (GAP-31), không suy ra lịch sử từ luồng live. Hook giữ cursor theo truy vấn, huỷ trang đang tải khi đổi truyện/bộ lọc, gộp trang cũ → đầu trang đã làm mới → live theo ID. Nhật ký máy tính lấy 500 dòng đầu và tải trang cũ tự động khi cuối danh sách gần vùng nhìn (sentinel ẩn, không thêm nút ngoài artboard); Bàn viết chỉ cần 40 dòng gần nhất như panel `Main.dc.html`. Refresh/lỗi lịch sử đi qua thanh phản hồi hiện có.

Duyệt chương gửi `applyComments:true` với số chương hoàn thành lớn nhất trong đại cương, đếm riêng góp ý chưa có `appliedAt`. “Cần sửa” gửi ý kiến trong Composer qua `/input`; không có endpoint từ chối riêng. Preset tạm dừng mặc định “Tạm dừng sau bước này” (`afterStep:true`), có “Tạm dừng ngay”; toast Hoàn tác gọi `/resume` thật. Composer giữ bản nháp khi lỗi hoặc khi người dùng tiếp tục gõ lúc chờ; ⌘S/Ctrl+S gửi, rời trang có bản nháp được bảo vệ.

Khi worker lỗi, Workspace chỉ hiện một thông báo với nguyên văn lỗi máy chủ và một nút Thử lại; `needsConfig:true` thêm liên kết Nhà cung cấp cho quản trị viên. Tiêu đề vẫn lấy từ danh sách truyện, tổng chữ và bản nộp mới nhất lấy từ danh sách chương khi chưa có snapshot. Vai trò Người đọc thấy thông báo Chỉ đọc thay Composer và gợi ý lệnh; không có nút duyệt, can thiệp, tạm dừng/tiếp tục hoặc xác nhận nhập. Nhật ký vẫn xem/lọc được nhưng không có thao tác ghi.

