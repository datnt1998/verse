# V40 — Bàn dựng, Duyệt và Cài đặt điện thoại

Trạng thái UI: đã kiểm tới `24bf453d`; backend forward-port `776195e6` có Go checks qua, sau đó helper duration ở `bbfab83c` có36focusedtests qua. #91/#92 đã chốt A và triển khai. Gói UI V40 chưa ký đầy đủ/chưa merge/chưa deploy; giữ Safari iPhone thật#93. V39 merge PR#6 (`9ad1effc`). Hotfix production riêng: PR#9 scoped-loopback, PR#10 catalog ảnh, PR#11 sửa semantics duration; source playback backport riêng `9d440603`, không mở gate UI.

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
- Hotfix backend `401d1b45` trước bản vá playback giữ Chrome headless và preflight HTTP loopback; chỉ nhánh token-scoped render/measure nhận exact origin đã bind từ raw loopback peer, không proxy metadata. PUBLIC_URL/SSO/editor policy giữ nguyên. Bản backend-only giữ UI cũ; bản playback tối thiểu sau đó ghi ở dưới.
- [Provenance](source-provenance.json) ghi SHA binary/assets và bằng chứng vnode đang chạy khớp file đã cài. Không dùng file SHA hoặc ready xanh để suy ra process đã đổi image.
- [Trạng thái kiểm](verification-state.json) sở hữu kết quả recovery; không lấy claim/PCM đang chạy làm bằng chứng score/export đã hoàn tất.
- Public-origin workaround đã claim nhưng attempt10 timeout ở426.240.000B. Cùng9.216.000B PCM: public36.327,88ms, loopback53,479ms. Không tăng timeout hoặc chạy lại bước AI. Scoped regression trước sửa403/sau sửa qua; full web/cmd và scoped race qua. Production render suite gặp test15ms sai loại hang khi chạy song song; exact isolated test qua, không nới test.
- Tuyến phục hồi và rollback thuộc [deployment guide](../../../../apps/viderse/deploy/README.md), không mở origin/token toàn app hay bỏ kiểm workspace. Rollout cuối đã xác minh PID38806/inode2809657 trùng binary mới; local/public ready200 và SHA UI cũ khớp.
- Bước đo thật hoàn tất1.298.188.800B trong khoảng20giây. Lỗi75/100 sau đó là thiếu catalog artifact trung gian, không mất ảnh: render.Inputs nay đọc catalog đúngjob, output hiện hành ưu tiên, relative path khớp hoàn toàn, ambiguity lỗi và vẫn hash bytes. Regression Inputs→filesystem→ComputeScore/tamper/isolation/precedence + race qua; full production/integration render/web/cmd qua.
- Nghiệm thu backend ban đầu: score attempt12 đạt100/100, không override (`accepted=false`), reuse measurement hợp lệ; export attempt1 lúc đó đã render2223/101421frames, chưa nhận hoàn tất. [Receipt](production-video-recovery.json). Sau đó MP4 đã hoàn tất; kiểm file và bản vá playback ghi ở mục tiếp theo. Không lặp các bước AI đã thành công.

## Thành phẩm báo mất file dù export thành công
- File production432.805.526B còn nguyên, SHA khớp; ffprobe H264/AAC1920×1080,3380,7s. Export attempt1 đã thành công; không dựng lại hoặc gọi AI. Screenshot lỗi do UI cũ chỉ chọn artifact MP4, trong khi output editor có projectId/exportId và artifacts rỗng.
- Source production `9d440603` chỉ backport resolver và PC/mobile caller lên UI cũ; `removedAt` vẫn chặn bản đã xoá, legacy artifact giữ nguyên. `durationMs` là thời gian xử lý, không dùng làm độ dài media. Production43focusedtests/typecheck/Vite/native build qua; integrated36tests qua.
- [Receipt và giới hạn](playback-hotfix/receipt.json), [PC](playback-hotfix/desktop-real-mp4.webp), [điện thoại](playback-hotfix/mobile-real-mp4.webp): built UI + read-response fixture + đúng MP4 production phát và seek1800s thật; metadata3380,7s, mobile direct-open URL đúng, không tràn ngang. Banner mất kết nối là SSE fixture kết thúc; không nhận đây là authenticated prod hoặc Safari physical proof.
- Rollout PID47165/inode2830537/SHA khớp; public ready200, chunk PC/mobile công khai byte-identical với build đã smoke. Public HTML có Cloudflare chèn script động, không dùng raw HTML hash làm version. Không có chrome-profile/live relay để vào phiên người dùng; HEAD không đăng nhập401 đúng chính sách. Không mint session hoặc mở auth để vượt kiểm.
- Chỉ hotfix phát Thành phẩm được deploy; toàn bộ UI/workflow V40 và gate Safari#93 vẫn giữ.

## Hotfix Editor và sidebar Cài đặt · 2026-11-06

- Production `9d1e4b6f`: backend và frontend dùng `X-Editor-Revision`, `If-Match` có dấu nháy kép, chỉ `revision_conflict` mở dialog. Không bỏ CAS; bỏ lời gán “Kiểm định” và giờ ghi giả.
- Native HTTP + PostgreSQL thật: transport cũ gặp ETag yếu → lưu409; transport mới cùng revision → lưu được. UI mở project schema15 và normalization PUT200; xung đột thật vẫn409, giữ snapshot; “Tiếp tục ở đây” tắt tự lưu. Chưa quan sát request có xác thực của ảnh lỗi production nên chưa kết luận nguyên nhân chính xác của ảnh đó.
- Sidebar desktop:9 hàng chung cột icon20px/nhãn47px, không wrap/overflow;7 nút đổi đúng route, link Hàng chờ hoạt động. Mobile390×844 không tràn; link Thành viên giữ đích/role. Không phải nghiệm thu toàn bộ Settings1:1.
- Isolated production:452 file,3740 test qua/4 bỏ qua; typecheck/Vite/native build qua. PostgreSQL bắt buộc3/3 qua, không skip. Primary focused59 qua.
- Đã cài rồi kickstart: PID79761/inode2856607 đúng binary; loopback/public ready200, chunk Settings/Editor public trùng build. [Receipt và ảnh](editor-settings-hotfix/receipt.json).
- [Team audit Template đầu ra](template-output-audit.md): báo cáo source/artboard, chưa sửa UI Template, chưa có browser production đăng nhập hoặc Safari thật. Giữ V40 chưa phát hành và gate #93 KEEP.

## Hotfix sửa mẫu Joker và phản hồi kênh · 2026-11-06

- Production `520d4ba3`: chỉ ghi trường của chế độ ảnh đang hoạt động vào track; form giữ số khi đổi chế độ. Regression ba mode thất bại trước và qua sau vá. Full suite library qua trên production source và main integration; primary typecheck/51 test qua, production typecheck/build/24 test qua.
- Project Joker cũ có `none` kèm `count:5`, làm GET Editor400. Đã sao lưu0600, khoá hàng và so sánh revision3 + toàn bộ project trước sửa; chỉ bỏ count không hoạt động và cập nhật giờ/revision4. Readback so sánh toàn bộ phần còn lại và recipe không đổi. Read-only business Get/ReadEditorProject trên production cùng qua.
- Native PostgreSQL/compiled UI: mở thẻ → settings → đổi đủ ba mode → mở Biên tập; thao tác ẩn track được tự lưu, đọc lại revision7 và visible=false. Media/quality không được cấu hình đầy đủ trong fixture; không phải nghiệm thu phát/xuất video.
- Nút Làm cho… có nhãn co/ellipsis và icon cố định; thẻ200px không đè copy, copy đúng link. Avatar dùng nguồn của đúng kênh, tròn và không bị thumbnail cắt; normal motion có kit-shim2.4s, reduced motion tắt, paused không shimmer. Kiểm390×844 không tràn; chưa có browser production đăng nhập hoặc Safari thật.
- Đã cài trước kickstart, binary mapped đúng inode; public/loopback ready200, các chunk public so trùng build. Source hiện tại bổ sung file settings.stylex.ts bị thiếu trong commit hotfix trước dù binary cũ đã chứa file đó.
- [Receipt và ảnh](template-channel-hotfix/receipt.json), [audit Bàn dựng đúng kênh](channel-workbench-audit.md). Còn Bản tin/Đề xuất/ô Nhắn trợ lý/inspector/template luân phiên theo UI cũ: không tuyên bố chuẩn toàn bộ artboard hoặc fullV40 đã deploy; gate Safari#93 KEEP.
