---
phase: 6
title: "Nhóm 03 · Kênh"
status: pending
priority: P1
effort: ""
dependencies: [5]
---
# Phase 06 · Nhóm 03 · Kênh
## Mục tiêu / dependency
Nghiệm thu và sửa tới khi khớp **production** nhóm03; chỉ bắt đầu sau phase05 đạt. Áp dụng toàn bộ [hợp đồng nghiệm thu](acceptance-contract.md), không chọn subset dễ. Source authority Workflow/IA/copy/footer không đổi; không sửa artboard.
## Coverage / dữ liệu
- Board/state: Channel, MChannel; Dialogs unlink/delete-channel và MSystem delChannel (chủ instance).
- Prerequisites thật: Kênh nghiệm thu mới/cũ, hai template thật, rival có thể link/unlink và permission cho mutation.
- Re-scout source current deployed revision trước sửa; bảng dưới là boundary hiện có, không khẳng định đã có bug ở mọi file. Không sửa file chỉ để đổi nhãn hoặc để inventory trông hoàn tất.
## File inventory / owner
Controller tích hợp duy nhất, owner nhóm này; các nhóm tuần tự vì chung shell, kit, API và production. Không song song click/mutate cùng entity hoặc deploy.
| Action | Path / boundary tuyệt đối | Quy mô/test impact |
|---|---|---|
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/ChannelScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/SourcesTab.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/TemplatesTab.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/SettingsTab.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/AutomationCard.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/PaceStepper.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/IdentityBlock.tsx | Component/contract test tương ứng |
| Tạo evidence | /Users/dat/Projects/verse/plans/261106-1303-viderse-production-artboard-parity/reports/group-03/ | Matrix rows, ảnh, difference/control ledger và report ngắn |
| Sửa sau đọc | /Users/dat/Projects/verse/apps/viderse/docs/design/v1/10-gap-closure.md; 11-hoi-dap.md cùng thư mục | Một dòng/gói với deployed SHA; chỉ câu hỏi lệch mới khi cần |
## Bước thực hiện
1. Chỉ ba phần Nguồn & tự động/Template mặc định/Hồ sơ, sources mặc định. Đi mọi tab/channel/autoApprove/tplMenu/identity/link/fresh và mobile sheet. Không làm tên production thành MT/CL/QH; dùng kênh thật cùng trạng thái, nhãn dài.
2. Đổi default template trên entity nghiệm thu, reload PC/phone và đọc API; shared template nhiều kênh, không rotation/cursor/autonomy cũ. Đổi template cho video không sửa default kênh. Delete default đang được dùng phải bị chặn đúng contract.
3. Automation chỉ autoFetch/autoApprove/perDay1–20; switch/stepper lưu thật, không clamp dữ liệu legacy. Tự duyệt chỉ mọi ảnh≥85; thiếu score không qua. Tắt tự lấy không tự bật lại khi reload; nhịp auto không chặn manual.
4. Liên kết/bỏ liên kết đối thủ, xác nhận, profile/name/tag/language và identity generating/failed/retry có readback thật; không lộ ID. Confirm delete chỉ thực thi trên kênh nghiệm thu được phép, không xóa kênh người dùng.
5. Popover/sheet đổi template/link không đẩy nội dung, cùng lựa chọn/default. Chụp fresh channel và lỗi thực có kiểm soát; không tạo failure bằng sửa DB.
5a. Nghiệm thu Dặn trợ lý về kênh theo quyết định#87–#89 đã chốt: đủ bốn người nhận, PC composer và mobile Hồ sơ→Sheet Nhập nhanh; lưu/readback/reload không gọi model. Video/lượt mới tiêu thụ đúng chỉ dẫn snapshot, job đang chạy bất biến. Kiểm định báo chỉ dẫn ngoài khả năng và không tự duyệt như đã đáp ứng; không thêm xử lý âm thanh. Đối chiếu consumer thật và retest Bàn dựng/Duyệt bị ảnh hưởng.
5b. Đóng vòng đời mutation sau link/create/toggle CUỐI: link nguồn đầu có thể bật lại autoFetch, nên kiểm API terminal config, tắt future admission của kênh nghiệm thu khi cửa sổ hết và reconcile queue/running/receipts. Chỉ giữ automation bật khi có quyền tiếp tục; không pause kênh khác hoặc báo nhóm đạt khi còn work ngoài quyền.
6. Mọi mismatch ghi expected/observed vị trí/khoảng cách/font/màu/icon/copy/click. Sửa cause ở owner, focused verification và native smoke, merge main rồi deploy qua phase03, chụp lại đúng production state. Recheck nhóm trước bị ảnh hưởng; intentional difference chỉ khi có số11-hoi-dap đã chốt.
## Verification / test matrix
- Focused khi có code change: npm test -w apps/viderse/web -- screens/channel; npm run typecheck -w apps/viderse/web; npm run build -w apps/viderse/web. Nếu chỉ audit, browser thật là proof, không chạy suite để lấy số xanh. Backend/kit đổi thì focused+broader theo contract; không chạy build/test lúc agent còn sửa.
- Production: browser mở đúng HTTPS, authenticated state; capture reference+production+side-by-side1440×900/390×844 theo counterpart; fonts.ready, image decode, exact state, geometry và motion. Baseline/Playwright QA đang loopback-only KHÔNG phải production test command.
| Mức | Scenario | Oracle |
|---|---|---|
| Critical | Default persistence/shared refs; perDay biên1/20; autoApprove84/85/missing; unlink cancel không ghi; no customer deletion. | Request/readback/entity/version đúng; không no-op/side-effect ngoài phạm vi |
| Critical | Dặn trợ lý #87–#89 đủ bốn người nhận | Save không model; job mới dùng snapshot; job cũ giữ; unsupported không autoApprove |
| High | Mọi footer value/context và visible controls | Row đầy đủ,0dead,0unverified enabled; disabled có lý do |
| High | Font/tab/menu/sheet/touch | Slider≤2px, không overflow,44px và không đẩy content |
| Medium | Normal/reduced, reopen, resize, route change | Motion/focus/scroll/safe-area đúng; không leftover overlay |
## Checklist
- [ ] Đủ board/tweak/dữ liệu production thật và cặp ảnh; mọi lệch đã sửa hoặc có số quyết định chốt.
- [ ] Control ledger đầy đủ,0không phản hồi,0enabled chưa kiểm; geometry/motion đạt.
- [ ] Commit đã deploy được chứng minh; báo nhóm03 bảng3cột + tối đa6ảnh + số controls/dead/chưa kiểm + URL/SHA; gap mỗi gói một dòng.
## Rủi ro / security / rollback
Không thay dữ liệu/credential người dùng để seed; xác định quyền/chi phí/recovery trước mutation, thiếu thì blocked chứ không pass. Sửa production quay qua phase03 và đúng rollback boundary, không restore DB cũ sau writer reopen. Shared code thay phải invalidate/retest evidence bị ảnh hưởng và proof Safari nếu cần. Báo nhóm ngay khi đạt rồi tiếp tục phase sau, không dừng ở báo cáo trung gian.
