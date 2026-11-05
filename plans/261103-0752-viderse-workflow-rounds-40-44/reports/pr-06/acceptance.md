# V40 — Bàn dựng, Duyệt và Cài đặt điện thoại

Trạng thái: phần source đã kiểm tới `4da829e8`; chưa ký đầy đủ, chưa merge hoặc deploy. V39 đã merge PR#6 (`9ad1effc`).

## Bảng màn → đã sửa / còn lệch

| Màn | Source đã sửa | Bằng chứng / phần chưa chốt |
|---|---|---|
| MVideo | Nội dung đăng/Ảnh, content mặc định; một Info sheet gom Tiến độ/Trợ lý/Nguồn/Template/Nền; readonly template, clipboard phản hồi; không lộ opaque ID khi thiếu tên | Native fixture controller: tabs/default/info/clipboard qua. Các hit-area nhỏ đã sửa và kiểm native cuối đủ44px; selected thumb sau font lệch≤2px, normal/reduced. |
| MReview / Review | Bỏ Why/learned UI; modern prompt4phần, legacy nguyên văn chỉ đọc; giữ version/baseVersion/compare; chip chỉ gỡ câu phân đoạn chính xác, không xoá substring người dùng tự viết | Native51actions/81checks ban đầu; sau sửa normal-motion prompt→edit sang onExited handoff, [focused native cuối](mobile-acceptance/focused-recheck-evidence.json) 10actions/30checks qua,0mutations, không chồng lớp interactive. |
| Kết nối AI PC/mobile | PC inline, phone một Sheet nhập key/address; secret toggle44px; Cancel/Escape/scrim/drag; remove đợi editor unmount mới confirm; old save không đóng draft mới; lỗi thô upstream được map trước confirm | [Settings](settings-acceptance/report.md), [SVG desktop](settings-acceptance/desktop-visual-receipt.md), normal/reduced và deferred/error fixtures qua. Không gọi provider/đăng xuất thật. |
| Logout / UIKit demo | Bỏ fallback hardcode khỏi helper/callers/test/DevKit; nói đúng dùng chung máy, lượt mới, ảnh đã gửi có thể hoàn tất; fallback chỉ khi bật và dùng được | Source + native confirm fixture/cancel; không chứng nhận logout hoặc chuyển provider thật từ fixture. |
| Shell / dock | Giữ5tab và avatar→Cài đặt theo Workflow, không tạo More/Theme phantom; source shared viewport fix giữ nguyên | Chrome scroll/resize/Escape/dock coverage; không chứng nhận Safari thiết bị thật. |
| Công cụ mobile | Chưa thêm layout khác artboard | Câu hỏi#91: vị trí Kiểm tra lại. |
| Hoàn tác Bỏ ảnh | Không dùng approve hoặc restore giả làm undo | Câu hỏi#92: API unskip giữ đúng trạng thái/bản, không duyệt hoặc vẽ lại. |

## Kiểm tra

- Full frontend441file:3667qua/4skip trước các sửa cuối geometry/handoff. Source cuối: focused28qua, typecheck/Vite/native build qua và native focused30/30; không gán full-run cũ cho source cuối.
- Size entry142,5/150KiB; cold phone179,4/200KiB; CSS33,7/40KiB. Không nâng budget.
- Native settings: desktop/mobile × normal/reduced,44px buttons, one modal, local response-only PATCH/DELETE, six mutation responses fulfilled locally.12/12 dismissal supplement; desktop SVG capture bổ sung. Không actual credential/provider writes.
- Native mobile:51actions/81checks/24captures, response-only; clipboard thật qua readback; legacy desktop/mobile raw và modern4parts. Initial geometry failures và transition overlap được giữ làm failing-before, không xoá hoặc gọi là pass.
- Hai review source cùng họ model; concrete findings đã sửa trong phạm vi. Không tuyên bố independent-family review.
- Có React act() warnings và Vite annotations/chunk warnings; không che.
- Latency serial cuối qua [receipt](latency-final.json):16biến thể/20kvideo, p95 chậm nhất155,83ms/400ms; danh sách50kênh/100kvideo/50kviệc giữ lại p95=25,05ms. Không nâng ngưỡng, không chạy đồng thời build/suite/browser controller.
- Bug hai viền focus người dùng gửi kiểm lại trên native V40 cuối: PC/mobile2/2 qua, một ring bo tròn và giữ Tab/Shift+Tab. Không chỉ dựa vào source StyleX.

## Release còn mở

Cần chốt#91/#92 và thực hiện đủ acceptance sau đó. Safari thiết bị thật chưa có proof: cả workstation và Mac mini không có route `xcrun devicectl` dùng được; không suy ra rằng không có thiết bị vật lý. Gate latency/cold-size#83 đã qua bằng receipt cuối. Không merge hoặc deploy chỉ vì unit tests xanh. Không sửa artboard, không thêm viewer45, không tạo ảnh mới hoặc đổi provider production. Git dùng private index; normal index thay đổi ngoài các lệnh private-index trong lúc làm và được để nguyên, không reset. models.go/auth giữ SHA; build dùng overlay SQLC model.
