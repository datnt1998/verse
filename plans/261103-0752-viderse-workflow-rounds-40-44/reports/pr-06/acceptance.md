# V40 — Bàn dựng, Duyệt và Cài đặt điện thoại

Trạng thái UI: đã kiểm tới `24bf453d`; forward-port backend ở `776195e6` có Go checks qua, không thay UI assets. #91/#92 đã chốt A và triển khai. Gói UI V40 chưa ký đầy đủ/chưa merge/chưa deploy; giữ Safari iPhone thật#93. V39 merge PR#6 (`9ad1effc`). Hotfix backend riêng: [PR#9](https://github.com/datnt1998/verse/pull/9) sửa scoped-loopback, [PR#10](https://github.com/datnt1998/verse/pull/10) sửa tra cứu artifact ảnh; không mở gate UI.

## Bảng màn → đã sửa / còn lệch

| Màn | Source đã sửa | Bằng chứng / phần chưa chốt |
|---|---|---|
| MVideo | Nội dung đăng/Ảnh, content mặc định; một Info sheet gom Tiến độ/Trợ lý/Nguồn/Template/Nền; readonly template, clipboard phản hồi; không lộ opaque ID khi thiếu tên | Native fixture controller: tabs/default/info/clipboard qua. Các hit-area nhỏ đã sửa và kiểm native cuối đủ44px; selected thumb sau font lệch≤2px, normal/reduced. |
| MReview / Review | Bỏ Why/learned UI; modern prompt4phần, legacy nguyên văn chỉ đọc; giữ version/baseVersion/compare; chip chỉ gỡ câu phân đoạn chính xác, không xoá substring người dùng tự viết | Native51actions/81checks ban đầu; sau sửa normal-motion prompt→edit sang onExited handoff, [focused native cuối](mobile-acceptance/focused-recheck-evidence.json) 10actions/30checks qua,0mutations, không chồng lớp interactive. |
| Kết nối AI PC/mobile | PC inline, phone một Sheet nhập key/address; secret toggle44px; Cancel/Escape/scrim/drag; remove đợi editor unmount mới confirm; old save không đóng draft mới; lỗi thô upstream được map trước confirm | [Settings](settings-acceptance/report.md), [SVG desktop](settings-acceptance/desktop-visual-receipt.md), normal/reduced và deferred/error fixtures qua. Không gọi provider/đăng xuất thật. |
| Logout / UIKit demo | Bỏ fallback hardcode khỏi helper/callers/test/DevKit; nói đúng dùng chung máy, lượt mới, ảnh đã gửi có thể hoàn tất; fallback chỉ khi bật và dùng được | Source + native confirm fixture/cancel; không chứng nhận logout hoặc chuyển provider thật từ fixture. |
| Shell / dock | Giữ5tab và avatar→Cài đặt theo Workflow, không tạo More/Theme phantom; source shared viewport fix giữ nguyên | Chrome scroll/resize/Escape/dock coverage; không chứng nhận Safari thiết bị thật. |
| Công cụ mobile | Kiểm tra lại ngay dưới mô tả theo#91A; dùng cùng probe/readback PC, toast kết quả | [Native PC/mobile2/2](native-tools.json), không install; mobile44px sau sửa flex ép nút từ42,42px. |
| Hoàn tác Bỏ ảnh | Snapshot bền vững, unskip giữ đúng trạng thái/phiên bản; toast toàn app giữ callback khi chuyển route | [Native API4ca](native-unskip-api.json), [UI PC/mobile2/2](native-ui-undo.json); PostgreSQL persistence và race qua. Không duyệt/vẽ lại hoặc thêm quota. |

## Kiểm tra

- Full frontend442file:3677qua/4skip cho source unskip/tools/Toast. Sau đó chỉ sửa `Button lg.minHeight`: shared33test, typecheck/Vite/native embed và native PC/mobile qua; không gán full-run trước sửa minHeight cho source cuối.
- Size entry142,6/150KiB; cold phone179,4/200KiB; CSS33,8/40KiB. Không nâng budget.
- Native settings: desktop/mobile × normal/reduced,44px buttons, one modal, local response-only PATCH/DELETE, six mutation responses fulfilled locally.12/12 dismissal supplement; desktop SVG capture bổ sung. Không actual credential/provider writes.
- Native mobile:51actions/81checks/24captures, response-only; clipboard thật qua readback; legacy desktop/mobile raw và modern4parts. Initial geometry failures và transition overlap được giữ làm failing-before, không xoá hoặc gọi là pass.
- Hai review source cùng họ model; concrete findings đã sửa trong phạm vi. Không tuyên bố independent-family review.
- Có React act() warnings và Vite annotations/chunk warnings; không che.
- Latency serial cuối qua [receipt](latency-final.json):16biến thể/20kvideo, p95 chậm nhất155,83ms/400ms; danh sách50kênh/100kvideo/50kviệc giữ lại p95=25,05ms. Không nâng ngưỡng, không chạy đồng thời build/suite/browser controller.
- Bug hai viền focus người dùng gửi kiểm lại trên native V40 cuối: PC/mobile2/2 qua, một ring bo tròn và giữ Tab/Shift+Tab. Không chỉ dựa vào source StyleX.

## Release còn mở

#91/#92 đã chốt và có native readback; không còn câu hỏi sản phẩm cho hai phần này. Gate Safari iPhone thật#93 vẫn giữ theo quyết định người dùng: [URL QA, phạm vi và checklist](physical-safari-handoff.md). LAN preview dùng UI đã build với dữ liệu mẫu riêng, không có credential/API production; Chromium3flow qua chỉ chứng minh route dùng được, không thay bằng proof Safari. Gate latency/cold-size#83 đã qua. Không merge/deploy chỉ vì unit tests xanh. Không sửa artboard, không thêm viewer45, không tạo ảnh mới hoặc đổi provider production. Git dùng private index, normal index của người dùng để nguyên; models.go/auth giữ SHA, build dùng overlay SQLC model.

## Kiểm bổ sung #91/#92 và vùng chạm
- Backend3package ảnh/model/web qua; real PostgreSQL publication/readback và race skip/unskip qua. Legacy skipped thiếu snapshot không thể suy ra trạng thái cũ; stale/busy trả409, không ép ghi đè.
- Hook/mobileReview/tools37test qua; shared UI33test qua. Task-role review giới hạn phần unskip/tools/touch không có blocker cụ thể; không nhận là review độc lập họ model.
- Native Undo: desktop chuyển route trước Hoàn tác vẫn khôi phục; mobile action44px. [Ảnh PC](native-undo-1440.png), [ảnh mobile](native-undo-390.png).
- Native Tools: POST probe và GET readback thật trên PC/mobile, toast kết quả; không cài/tải công cụ. [Ảnh PC](native-tools-1440.png), [ảnh mobile](native-tools-390.png).
- Ledger QA cuối vẫn3job/27step/2receipt; không gọi provider hoặc sinh ảnh mới.

## Hotfix production độc lập với V40
- Bản chạy production `401d1b45` giữ Chrome headless và preflight HTTP loopback; chỉ nhánh token-scoped render/measure nhận exact origin đã bind từ raw loopback peer, không proxy metadata. PUBLIC_URL/SSO/editor policy giữ nguyên. UI production byte-identical; không migrate hoặc triển khai UI V40.
- [Provenance](source-provenance.json) ghi SHA binary/assets và bằng chứng vnode đang chạy khớp file đã cài. Không dùng file SHA hoặc ready xanh để suy ra process đã đổi image.
- [Trạng thái kiểm](verification-state.json) sở hữu kết quả recovery; không lấy claim/PCM đang chạy làm bằng chứng score/export đã hoàn tất.
- Public-origin workaround đã claim nhưng attempt10 timeout ở426.240.000B. Cùng9.216.000B PCM: public36.327,88ms, loopback53,479ms. Không tăng timeout hoặc chạy lại bước AI. Scoped regression trước sửa403/sau sửa qua; full web/cmd và scoped race qua. Production render suite gặp test15ms sai loại hang khi chạy song song; exact isolated test qua, không nới test.
- Tuyến phục hồi và rollback thuộc [deployment guide](../../../../apps/viderse/deploy/README.md), không mở origin/token toàn app hay bỏ kiểm workspace. Rollout cuối đã xác minh PID38806/inode2809657 trùng binary mới; local/public ready200 và SHA UI cũ khớp.
- Bước đo thật hoàn tất1.298.188.800B trong khoảng20giây. Lỗi75/100 sau đó là thiếu catalog artifact trung gian, không mất ảnh: render.Inputs nay đọc catalog đúngjob, output hiện hành ưu tiên, relative path khớp hoàn toàn, ambiguity lỗi và vẫn hash bytes. Regression Inputs→filesystem→ComputeScore/tamper/isolation/precedence + race qua; full production/integration render/web/cmd qua.
- Recovery cuối score attempt12 đạt100/100, không override (`accepted=false`), reuse measurement hợp lệ; export attempt1 đã claim/render thật2223/101421frames. [Receipt](production-video-recovery.json). Đây là bằng chứng gỡ kẹt, **không phải file MP4 đã xuất xong**. Không lặp các bước AI đã thành công.
