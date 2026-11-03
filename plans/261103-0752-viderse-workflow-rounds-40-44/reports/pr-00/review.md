# PR0 · Review và xử lý finding

Ngày 2026-11-03. Scope: inventory, runner và bằng chứng baseline; không sửa UI/runtime sản phẩm.

| Review | Finding có bằng chứng | Cách xử lý và kiểm thật |
|---|---|---|
| Baseline evidence | 10 desktop skeleton; MReview/MSettings chưa có dữ liệu; một số cặp sai tab/state | Chụp lại bằng Playwright headless, đợi chunk/API/ảnh, chọn reference row đúng context; ghép lại 28 cặp. Receipt cuối 28/28, font loaded, DPR 1, không overflow |
| State inventory/simplify | Recipe defaults+patch không mở view cha; schema/caller dễ quên dependency | Context từ điều kiện render, kiểm key/value theo footer, recipe defaults+context+patch; bỏ dead config và giảm lặp validation; extractor --check exit 0 |
| Tweak-context review | 20 row không hiện nội dung và 7 row hiện thiếu: recovery step, member access, server usage, sheet jobs/inbox, metadata/failure dưới fold | Sửa đúng context; 13 row có captureTarget/framing/scroll receipt. Lượt reference cuối 480/480, readback props, font loaded, 0 render error/overflow. Đã xem ảnh recovery copy-fail, S3 key-fail có cả cách sửa, metadata edited; không suy thành visual oracle cho mọi row |
| Capture runner | Onboard bị coi là màn độc lập dù mở CreateChannel; filter ghi đè report đầy đủ | Onboard ghi prerequisite thiếu. Filter cần output riêng; smoke xác nhận bị từ chối và SHA256 report canonical không đổi |
| Capture runner | HTTP lỗi không có board/verdict; fatal/setup lỗi có thể để report cũ hoặc Chrome mồ côi | Gắn board/expected cho response, fail unexpected, report atomic running/completed/aborted, mọi browser setup trong try/finally. Lượt cuối 4 HTTP lỗi dự kiến, 0 unexpected. PID app/reference đã không còn sau khi chạy |
| Capture runner | Gate text >150 không chứng minh ready; cookie clearing làm MAuth mang banner phiên cũ | Không dùng char count. Đợi đúng route, content, không aria-busy, API hữu hạn kết thúc, font/ảnh decode; mỗi board có context mới, MAuth không có cookie/storage |
| Runtime narrowing | API-idle tracker giữ boot request `/api/setup/state` từ page/reload cũ | Tái hiện bằng sequence Review/Main/Template, hai bước sau fail. Không bỏ qua endpoint: chuyển sang context riêng từng board, cùng sequence qua 3/3 rồi full app 28/28 |
| Security boundary | Capture có thể bị cấu hình trỏ production hoặc cookie nằm trong repo | Origin loopback không chứa credential; cookie phải ngoài repo; QA identity phải workflow.qa. Bốn guard smoke đạt expected rejection, không gọi provider |

## Kết luận đúng scope

- Tooling inventory/capture đã được chạy thật, lỗi cũ giữ trong [failure-history](failure-history.json), không đổi thành pass bằng cách bớt target row.
- Matrix vẫn 480 enum/boolean rows, 12 probe số/text, 42 board được phân loại. `MReview` giữ đủ 9 row; đợt45 mobile shared viewer và mobile Editor không bị thêm vào scope.
- `capture-index.json` nối 480 row tới reference receipt; một baseline route nền không chứng minh target state cho mọi row. Matrix `pending` và `visibleStateVerified:false` không bị đổi thành pass.
- Sáu board thiếu target prerequisite là **Onboard, Activity, Dialogs, System, MCompVideo, MSystem**. Preview API hiện `none`/`sources=[]`. Giữ owner phase; đây là gap hiện trạng, không phải package UI đã hoàn thành.
- Chưa full-control Playwright, chưa đầy đủ normal/reduced-motion, chưa slider/font/hit-area gate trên mọi màn, chưa Safari keyboard/dock thật, chưa provider generation proof hay deploy.
- Reference là nội dung tĩnh của artboard đã có trong repository, không phải ảnh tài khoản production; ảnh app chỉ dùng account/data QA. Không commit cookie/password/provider store. Không sửa artboard để né lỗi.
