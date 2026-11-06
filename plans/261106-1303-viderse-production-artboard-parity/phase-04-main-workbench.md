---
phase: 4
title: "Nhóm 01 · Bàn dựng và Editor"
status: pending
priority: P1
effort: ""
dependencies: [3]
---
# Phase 04 · Nhóm 01 · Bàn dựng và Editor
## Mục tiêu / dependency
Nghiệm thu và sửa tới khi khớp **production** nhóm01; chỉ bắt đầu sau phase03 đạt. Áp dụng toàn bộ [hợp đồng nghiệm thu](acceptance-contract.md), không chọn subset dễ. Source authority Workflow/IA/copy/footer không đổi; không sửa artboard.
## Coverage / dữ liệu
- Board/state: Main, MMain, Activity; Editor desktop; Dialogs/MSystem instance thao tác video.
- Prerequisites thật: Job thật đủ trạng thái/ảnh/score/metadata; ba suggestion types, ít nhất hai ready files và editor project thật được phép sửa.
- Re-scout source current deployed revision trước sửa; bảng dưới là boundary hiện có, không khẳng định đã có bug ở mọi file. Không sửa file chỉ để đổi nhãn hoặc để inventory trông hoàn tất.
## File inventory / owner
Controller tích hợp duy nhất, owner nhóm này; các nhóm tuần tự vì chung shell, kit, API và production. Không song song click/mutate cùng entity hoặc deploy.
| Action | Path / boundary tuyệt đối | Quy mô/test impact |
|---|---|---|
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/workbench/WorkbenchDesktop.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/workbench/LiveStudio.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/workbench/panels/WorkbenchPanels.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/workbench/mobile/MobileWorkbench.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/workbench/mobile/MobileWorkbenchList.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/workbench/mobile/MobileWorkbenchDetail.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/editor/EditorScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/editor/v13/ | Editor thực: scene-image/session/quality/export; không chỉ route wrapper |
| Tạo evidence | /Users/dat/Projects/verse/plans/261106-1303-viderse-production-artboard-parity/reports/group-01/ | Matrix rows, ảnh, difference/control ledger và report ngắn |
| Sửa sau đọc | /Users/dat/Projects/verse/apps/viderse/docs/design/v1/10-gap-closure.md; 11-hoi-dap.md cùng thư mục | Một dòng/gói với deployed SHA; chỉ câu hỏi lệch mới khi cần |
## Bước thực hiện
1. Đăng nhập production và mở phạm vi tất cả/kênh: Chờ bạn chứa đúng ba loại đề xuất dạng dòng; không dock/thẻ Đề xuất riêng, recap, composer/dải agent cũ. Chọn dòng video đổi đồng bộ mọi panel theo entity, mặc định Nội dung đăng đúng board.
2. Đi mọi stage images/review/render/issue/quality/done/paused, metadata pending/writing/failed/ai/edited/rewrite/history, viewer và progress; build từ job thật. Hai tab Nội dung đăng/Thông tin trên PC; mobile Nội dung đăng/Ảnh và Thông tin sheet theo board, không buộc cấu trúc tab desktop lên phone.
3. Kiểm tiến độ/log thật, running shimmer/reduced/paused, full/cutout/provenance/history, clipboard và tải gói hai video có tệp thật. Đề xuất ba loại phải có action/readback/undo nếu đảo được, không coi nút có toast là thành công.
4. Gắn Editor25row cũ vào nhóm này: image viewer và selected version, panel/focus, fix/measuring/export-gate/exporting/export-done/conflict/narrow. Giữ CAS X-Editor-Revision/If-Match, tab có unsaved work, playback/seek/download thật; không tự render lại hay chạy AI chỉ để chụp. Phone entry chỉ Mở trên máy tính theo spec.
5. Control ledger gồm chọn nhiều/download/clipboard/progress links/notification popover/channel switch/shortcuts và confirmations tương ứng. Mỗi state dựng được chụp đúng board/counterpart; real state thiếu dữ liệu ghi blocked, không dùng sample.
6. Mọi mismatch ghi expected/observed vị trí/khoảng cách/font/màu/icon/copy/click. Sửa cause ở owner, focused verification và native smoke, merge main rồi deploy qua phase03, chụp lại đúng production state. Recheck nhóm trước bị ảnh hưởng; intentional difference chỉ khi có số11-hoi-dap đã chốt.
## Verification / test matrix
- Focused khi có code change: npm test -w apps/viderse/web -- screens/workbench editor/v13; npm run typecheck -w apps/viderse/web; npm run build -w apps/viderse/web. vitest.editor.config.ts sở hữu src/editor/v13 và UI/bridge/engine/pages; mở rộng filter đúng module đổi, chạy test:engine khi engine/node contract đổi. Nếu chỉ audit, browser thật là proof, không chạy suite để lấy số xanh. Backend/kit đổi thì focused+broader theo contract; không chạy build/test lúc agent còn sửa.
- Production: browser mở đúng HTTPS, authenticated state; capture reference+production+side-by-side1440×900/390×844 theo counterpart; fonts.ready, image decode, exact state, geometry và motion. Baseline/Playwright QA đang loopback-only KHÔNG phải production test command.
| Mức | Scenario | Oracle |
|---|---|---|
| Critical | Suggestion action readback; video selection isolation; Editor conflict giữ draft và version đúng; download/seek tệp thật. | Request/readback/entity/version đúng; không no-op/side-effect ngoài phạm vi |
| High | Mọi footer value/context và visible controls | Row đầy đủ,0dead,0unverified enabled; disabled có lý do |
| High | Font/tab/menu/sheet/touch | Slider≤2px, không overflow,44px và không đẩy content |
| Medium | Normal/reduced, reopen, resize, route change | Motion/focus/scroll/safe-area đúng; không leftover overlay |
## Checklist
- [ ] Đủ board/tweak/dữ liệu production thật và cặp ảnh; mọi lệch đã sửa hoặc có số quyết định chốt.
- [ ] Control ledger đầy đủ,0không phản hồi,0enabled chưa kiểm; geometry/motion đạt.
- [ ] Commit đã deploy được chứng minh; báo nhóm01 bảng3cột + tối đa6ảnh + số controls/dead/chưa kiểm + URL/SHA; gap mỗi gói một dòng.
## Rủi ro / security / rollback
Không thay dữ liệu/credential người dùng để seed; xác định quyền/chi phí/recovery trước mutation, thiếu thì blocked chứ không pass. Sửa production quay qua phase03 và đúng rollback boundary, không restore DB cũ sau writer reopen. Shared code thay phải invalidate/retest evidence bị ảnh hưởng và proof Safari nếu cần. Báo nhóm ngay khi đạt rồi tiếp tục phase sau, không dừng ở báo cáo trung gian.
