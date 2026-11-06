---
phase: 9
title: "Nhóm 06 · Cài đặt"
status: pending
priority: P1
effort: ""
dependencies: [8]
---
# Phase 09 · Nhóm 06 · Cài đặt
## Mục tiêu / dependency
Nghiệm thu và sửa tới khi khớp **production** nhóm06; chỉ bắt đầu sau phase08 đạt. Áp dụng toàn bộ [hợp đồng nghiệm thu](acceptance-contract.md), không chọn subset dễ. Source authority Workflow/IA/copy/footer không đổi; không sửa artboard.
## Coverage / dữ liệu
- Board/state: Settings, MSettings; notification settings instance Activity view=settings.
- Prerequisites thật: Quyền settings thực, provider credentials hợp lệ và action-specific approval, storage/QR/tool states thật có thể dựng an toàn.
- Re-scout source current deployed revision trước sửa; bảng dưới là boundary hiện có, không khẳng định đã có bug ở mọi file. Không sửa file chỉ để đổi nhãn hoặc để inventory trông hoàn tất.
## File inventory / owner
Controller tích hợp duy nhất, owner nhóm này; các nhóm tuần tự vì chung shell, kit, API và production. Không song song click/mutate cùng entity hoặc deploy.
| Action | Path / boundary tuyệt đối | Quy mô/test impact |
|---|---|---|
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/settings/SettingsScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/settings/tools/ToolsSection.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/settings/connect/ConnectSection.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/settings/connect/ProviderCard.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/settings/agents/AgentsSection.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/settings/storage/StorageSection.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/settings/storage/CleanupCard.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/settings/accounts/AccountsSection.test.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/settings/NotificationSettings.tsx | Component/contract test tương ứng |
| Tạo evidence | /Users/dat/Projects/verse/plans/261106-1303-viderse-production-artboard-parity/reports/group-06/ | Matrix rows, ảnh, difference/control ledger và report ngắn |
| Sửa sau đọc | /Users/dat/Projects/verse/apps/viderse/docs/design/v1/10-gap-closure.md; 11-hoi-dap.md cùng thư mục | Một dòng/gói với deployed SHA; chỉ câu hỏi lệch mới khi cần |
## Bước thực hiện
1. Đi đủ section/tools/conn/grok/ai/bili/clean/store và TẤT CẢ footer props còn lại, không dùng README ellipsis làm hết danh sách. Phone list/tools/connect/agents/accounts/storage/QR/sheets/usage, dark-only theo spec, không invent More/Theme.
2. #91 Kiểm tra lại nằm dưới mô tả tools phone; click POST probe + GET status/readback thật PC/phone, không install/download. Nav/icon columns, helper/errors và touch44px khớp cùng default desktop.
3. AI connection add/edit/models/save/cancel/escape/drag/pending/error; một overlay tương tác, save cũ không đóng draft mới. Logout confirm nói đúng machine-scope và lượt đã gửi có thể hoàn tất; không hardcode provider fallback/không gửi duplicate. Kiểm actual credential write chỉ với approval và recovery, không xoá credential sản xuất để lấy clip.
4. Accounts/QR expired/rejected/success, notifications suggestions bool, logs/errors tiếngViệt không secret; storage here/server/S3 states và cleanup dry-run→confirm→result qua runtime thật. Không đổi bucket/provider/global model hoặc xóa media người dùng nếu chưa có quyền; trạng thái không thể dựng an toàn ghi blocked, không mock.
5. Zero YouTube Data API card/string; avatar→settings và mọi link tới storage/logs/members/queue có đích/role thật. Đối chiếu normal/reduced, focus/scrim/scroll; kiểm lại physical Safari nếu overlay đổi.
6. Mọi mismatch ghi expected/observed vị trí/khoảng cách/font/màu/icon/copy/click. Sửa cause ở owner, focused verification và native smoke, merge main rồi deploy qua phase03, chụp lại đúng production state. Recheck nhóm trước bị ảnh hưởng; intentional difference chỉ khi có số11-hoi-dap đã chốt.
## Verification / test matrix
- Focused khi có code change: npm test -w apps/viderse/web -- screens/settings; npm run typecheck -w apps/viderse/web; npm run build -w apps/viderse/web. Nếu chỉ audit, browser thật là proof, không chạy suite để lấy số xanh. Backend/kit đổi thì focused+broader theo contract; không chạy build/test lúc agent còn sửa.
- Production: browser mở đúng HTTPS, authenticated state; capture reference+production+side-by-side1440×900/390×844 theo counterpart; fonts.ready, image decode, exact state, geometry và motion. Baseline/Playwright QA đang loopback-only KHÔNG phải production test command.
| Mức | Scenario | Oracle |
|---|---|---|
| Critical | Machine-scope credentials isolation/approval; real tools readback; stale save/dismiss; cleanup không xoá nhầm; secrets không vào evidence. | Request/readback/entity/version đúng; không no-op/side-effect ngoài phạm vi |
| High | Mọi footer value/context và visible controls | Row đầy đủ,0dead,0unverified enabled; disabled có lý do |
| High | Font/tab/menu/sheet/touch | Slider≤2px, không overflow,44px và không đẩy content |
| Medium | Normal/reduced, reopen, resize, route change | Motion/focus/scroll/safe-area đúng; không leftover overlay |
## Checklist
- [ ] Đủ board/tweak/dữ liệu production thật và cặp ảnh; mọi lệch đã sửa hoặc có số quyết định chốt.
- [ ] Control ledger đầy đủ,0không phản hồi,0enabled chưa kiểm; geometry/motion đạt.
- [ ] Commit đã deploy được chứng minh; báo nhóm06 bảng3cột + tối đa6ảnh + số controls/dead/chưa kiểm + URL/SHA; gap mỗi gói một dòng.
## Rủi ro / security / rollback
Không thay dữ liệu/credential người dùng để seed; xác định quyền/chi phí/recovery trước mutation, thiếu thì blocked chứ không pass. Sửa production quay qua phase03 và đúng rollback boundary, không restore DB cũ sau writer reopen. Shared code thay phải invalidate/retest evidence bị ảnh hưởng và proof Safari nếu cần. Báo nhóm ngay khi đạt rồi tiếp tục phase sau, không dừng ở báo cáo trung gian.
