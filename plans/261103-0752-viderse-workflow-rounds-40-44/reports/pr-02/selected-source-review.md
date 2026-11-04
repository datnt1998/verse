# V36 · Rà soát contract bản ảnh nguồn

## Phạm vi

Quyết định #84A: Sửa/Góp ý dùng đúng bản đang xem; giữ phần prompt, nguồn và refs không sửa; không tự đổi bản dùng trong video. Hai tác vụ review độc lập đọc backend và các producer frontend. Reviewer không chạy test/build hoặc gọi provider. Controller tích hợp và chạy các gate bên dưới. Không làm viewer mobile đợt45.

## Backend — đã xử lý

| Phát hiện | Sửa nguyên nhân | Chứng cứ |
|---|---|---|
| API kiểm limit của chuỗi toàn cục, worker dùng nguồn riêng video | `refLimitFor` nhận override manifest, giữ nguyên provider-card ID; request provider rõ ràng vẫn ưu tiên | Regression card `chatgpt`/`grok`/`gemini-…`; HTTP thật lượt58 trả422 limit1/0, không đổi checkpoint |
| Hướng cũ dính trong Cảnh bị gửi lại cạnh hướng mới | Strip đúng suffix lịch sử khi dựng plan, copy-on-change, không sửa source history | `TestSelectedSourceDropsLegacyDirectionWithoutChangingHistory` |
| Nhận redraw khi ảnh còn chuyến vẽ, draft bị chuyến cũ xoá | API chặn mọi target có `InFlight` trước mutation; worker không coi row failed có flight sống là nguồn mới | Service + worker regressions; HTTP thật409, checkpoint/job không đổi |
| Bỏ hết refs ở normal path nhưng prompt vẫn mô tả refs cũ | So ID/vai trò với baseline hiệu lực trên cả hai path; chỉ dựng lại mô tả khi attachments đổi | Regression remove-all và giữ custom refs instructions cho explicit/omitted source |

Re-review backend cuối xác nhận không còn phát hiện cụ thể. Final focused clone/images + pipeline/images qua; broader web + clone/images + pipeline qua. Không gọi đây là full-backend/performance xanh.

## Frontend — đã tích hợp và kiểm chứng

| Phát hiện | Sửa nguyên nhân |
|---|---|
| Upload ref đã lưu có ID `upload:<mediaId>` nhưng resend dùng cả prefix | Giữ raw media ID từ `source.imageId`; refs không đổi không cần gửi lại |
| Lần huỷ mới nhất trở thành nguồn mặc định không hợp lệ | Bỏ cancelled khỏi lựa chọn mặc định; giữ history readonly có lý do; source hợp lệ vẫn vẽ lại được |
| Grok retry lấy failed đầu tiên thay vì lần lỗi mới nhất | Chọn failed cuối; gửi `baseVersion` thay vì chép lại parts/refs thành sửa tay |
| Submit edit / phím R bỏ qua trạng thái bận hoặc bỏ draft | Chặn drawing/busy và giữ draft; phím tắt không gửi feedback-only khi đang sửa |
| Thêm refs khi chưa có nguồn dẫn tới nút gửi không phản hồi | Cho refs-only ordinary redraw với `baseVersion` bỏ trống khi không có nguồn; giữ chính sách dựng từ lời kể, không chỉnh prompt của bản huỷ |
| Thêm/upload refs trong lúc đang vẽ có thể chốt nguồn drawing rồi gửi nguồn đã huỷ | Chặn entry/callback khi drawing/busy; kiểm trạng thái live của cùng số bản trước gửi; nguồn đổi thì giữ draft và khoá có lý do |
| Rail khi giữ draft lại hiển thị phần không sửa của bản fallback | Rail đọc snapshot `edit.base` lúc edit/raw-dirty; không trộn style/refs của bản đang dùng khác |
| Upload trả về sau đổi job/ảnh/bản làm bẩn draft mới | So job/image/source identity tại completion trước attach; có feedback khi lựa chọn đã đổi |
| Header hiển thị bản0 hoặc gán prompt chung cho bản huỷ | Không gắn số bản khi không có phiên bản dùng được; history huỷ vẫn có nhãn riêng |

Focused cuối: Review30/30, mobile11/11, LiveStudio12/12, Workbench5/5, editor25/25; app và engine-test typecheck qua. Full cuối3755qua/0lỗi/4skip; production build qua. Lượt3752 giữ như mốc trước sửa entry/identity cuối. Sửa test precondition LiveStudio (mở viewer trước khi drawing tile bị khoá), function opener do thao tác edit và test field tên sai theo contract; không tăng timeout hoặc giấu failure.

### Conditional simplify

- Gate live:22 file,1874 dòng thay đổi; pass riêng trên bảy file Go đã thực hiện bằng role task.
- Giữ `slices.Contains` cho membership và bỏ import/blank assignment chết.
- Không nhận thay đổi `out.Sorted()` vì helper cấp phát/copy lại slice đã sở hữu; giữ sort stable in-place theo `PartOrder` và thứ tự phần lạ. Source/history không bị mutate.
- Sau simplify và overlay dependency đã commit: focused clone/images0.446s, pipeline/images cached; broader12 package qua,2 không có test, web27.712s. Hồi quy parts-array riêng lỗi trước (`null`), qua sau0.682s.

## Chứng cứ hiện có và giới hạn

- Trước bộ sửa edge-case frontend: app consumer43/43, editor21/21, frontend full3736qua/4skip; app/editor-test typecheck và production+embedded build qua.
- [Native lượt56](runs/56-selected-source-actual-api/selected-base-native-results.json) và [lượt57](runs/57-selected-source-reviewed-api/selected-base-native-results.json): PC/mobile, normal/reduced, HTTP202 thật, draft nguồn2, bản dùng1, refs2; replay khôi phục/source không đổi. Lượt57 đợi finite animations hoàn tất trước ảnh tĩnh.
- [HTTP boundary lượt58](runs/58-selected-source-api-boundaries/rejection-results.json): sáu ca400/404/409/422, không mutation. Dữ liệu sai là domain fixtures khai báo trên replay QA đang pause, không giả outcome provider.
- [Native61 cuối](runs/61-selected-source-final-empty-cancelled-history/selected-base-native-results.json):4/4 context qua, đúng source/current/refs2/1/2, history huỷ3 disabled, không tràn ngang/stacked active modal/overlay ra ngoài màn. Static `ReasonPopover` trong rail cuộn không phải floating overlay; native60 oracle từng phân loại nhầm, source xác nhận `position:static`, không sửa UI để che failure.
- Native59 phát hiện lỗi thật: version huỷ cũ không có prompt serialize `parts:null`, strict rich-client schema rơi về DTO cũ và mất history/refs. Sửa producer trả mảng rỗng đúng contract, không nới schema hoặc làm giả prompt; có hồi quy lỗi-trước/qua-sau và native61 dùng lại ca prompt rỗng.
- [Boundary62 cuối](runs/62-selected-source-final-api-boundaries/rejection-results.json):6/6 ca400/404/409/422 đúng, không persistent mutation/source change; replay phục hồi.
- Giữ file generated-model của người dùng nguyên trạng/unstaged. Go overlay dùng bytes HEAD chỉ cho file đó, nên binary/checks không vô tình xuất bản các trường template/trial ngoài scope.
- `npm run size` cuối: entry142.1/150KiB và CSS34.7/40KiB qua; phone203.1/200KiB FAIL. Không nới ngưỡng, không tuyên bố #83 đã đóng.
- Tổng ngân sách ảnh vẫn2/2 đã tiêu thụ; không có ảnh thứ3. Native source smoke giữ worker pause trước submit; worker request/persistence có regression riêng, không giả có output AI mới.
- Chưa merge/deploy. Hai gate toàn cục #83 vẫn mở, và chưa có chứng cứ Safari/iPhone vật lý.
