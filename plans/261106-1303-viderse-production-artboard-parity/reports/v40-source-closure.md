# V40 source closure · merged, production chưa deploy

## Đã sửa
- Preview Template có project cũ vẫn hiện overlay stale/building; chặn tương tác, ẩn khỏi accessibility khi unavailable và pause playback thật. Desktop regression: stale/building failed trước vá, passed sau vá; fresh vẫn tương tác.
- Phone Template/Library dùng Sheet cho tên; handoff chờ menu exit, không chạy lại pending action sau reopen/cancel; đang lưu không swipe-dismiss hoặc double-submit. Phone Output không lặp vùng xoá vốn đã nằm trong menu.
- Picker kênh tự quản resource loading/error/retry, desktop Dialog/phone Sheet; phone có nút Đóng≥44px, navigation đợi exit để không chồng lớp.
- Extractor `--production`:517row/29board,37Auth desktop,40probe (12cũ+28geometry/lifecycle), owner theo group/context. Toàn bộ pending/null;#94 chỉ eligible first-run, chưa được đánh verified. Hash HTML/props/contract đầy đủ, receipt cũ được bảo vệ cả alias inode. Không mock hoặc chụp production.

## Verification đã quan sát
- Focused library regression đạt46test. Clean merged-source frontend cuối đạt442file,3696passed/4skipped; typecheck/build/size đạt. Có warning React act và lazy chunk>500kB, không che hoặc nâng ngưỡng.
- `TEST_DATABASE_URL` trỏ database test riêng: `go test ./apps/viderse/...` đạt51package,11package không có test; không dùng production DB/provider.
- Native build workspace ban đầu fail vì gen/models.go thiếu fields; canonical SQLC được tái sinh, không overlay. Clean-source native build cuối đạt sau bổ sung core/ vào archive kiểm chứng và dọn cache Go theo người dùng cho phép. Không đưa model/user index vào commit.
- Máy làm việc hết đĩa; thử builder Docker QA riêng trên Mac mini cũng hết dung lượng VM. Không prune Docker, không chạy script deploy. `go clean -cache` được người dùng chấp thuận giải phóng25GB; build sạch sau đó đạt.
- Size clean build:entry142.6/150KiB;phone179.5/200KiB;CSS33.9/40KiB.
- CLI inventory generate/check đạt. Smoke xác nhận rows/controls evidence vẫn null, group Dialogs/MSystem đúng context, legacy stdout không đổi so snapshot trước cook; symlink/hardlink vào legacy receipt bị từ chối ngay cả `--check`.

## Native smoke trên môi trường thật biệt lập
App DB,identity DB,HOME/browser riêng; không phải production. Không mint session: tạo QA admin qua first-user HTTP/UI, lưu recovery ngoài repo0600, vào phone để không tự cài tools desktop.
- Trước vá đã mở UI thực và chụp dialog tên nằm giữa màn390×844.
- Sau rebuild: rename qua menu→Sheet, một dialog tương tác; sau fonts/motion settle y619+h225=844,w390. Lưu tên và GET `/api/templates/{id}` trả200,name đúng,revision3.
- Output→picker kênh rỗng: Sheet y590+h254=844,w390; nút Đóng w358,h44; đóng được. Empty thực có0kênh, không giả API.
- Tools phone: click Kiểm tra lại chỉ POST `/api/tools/check`;200và GET `/api/tools` trả cùng checkedAt `2026-11-06T14:42:16.803705Z`. Không install/download; đây là proof mới cho#91.
- #92 đã có proof persisted native ở report cũ; source không bị sửa trong lượt này và regression Postgres chạy cùng toàn package. Không lấy preview sample thay backend proof.
- Native smoke cuối từ `/tmp/viderse-v40-clean-native`, không dùng binary workspace: rename Sheet một dialog,x0,y618.703,w390,h225.297,đáy844; không overflow; Huỷ đóng hết dialog. Quan sát và chụp trực tiếp UI390×844; không phải Safari thiết bị thật.
- Checkout candidate ban đầu còn canvasv65 trong Git trong khi workspace nguồn chuẩn làv97. Snapshotv97,01-ia-flows,04-components và08-dev-prompt được đưa nguyên trạng vào private index; không chỉnh artboard. Inventory sạch sau đủ input đạt517row/40probe và khớp byte receipt đã tạo.

## Review / bảo vệ workspace
Hai scout độc lập nghiên cứu V40/manifest; hai reviewer code tìm các lỗi context/hash/probe/alias và overlay/playback, đã xử lý. Final matrix review chỉ yêu cầu regenerate sau đổi provenance null; đã regenerate/check. Simplifier chỉ sửa MTemplateScreen/AttachChannelDialog, không chạy checks mid-flight. Controller tích hợp duy nhất; không tạo worktree hoặc động vào normal index có thay đổi của người dùng.
Final reviewer độc lập không có finding còn lại trong source candidate. Commit `cca32ed1fc11b53b570417942462dee8f50244ce` đã push, PR[#7](https://github.com/datnt1998/verse/pull/7) được chuyển khỏi draft và merge thông thường, không bypass. GitHub không cấu hình statusCheckRollup; không ghi CI xanh. Main merge `60b4766429a852f17c6e305e71b9179bbc0d632a` lúc15:10:12Z; tree `7c248a6008df723538998ee9f25b4a6d04154f95` khớp candidate đã kiểm chứng. Normal index checksum giữ nguyên qua commit.

## Chưa làm / gate
Chưa có physical iPhone Safari proof#93; chưa deploy production;7group production chưa nghiệm thu. Chưa mở maintenance/quiesce/backup hoặc chạy cutover. Không ghi10-gap-closure bằng commit chưa deploy. Preview giao đúng artifact và thời hạn sẽ ghi tại safari-physical.md; report này không phải release receipt. Service native QA do phiên tạo đã dừng sau smoke.
