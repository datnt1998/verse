---
phase: 10
title: "Nhóm 07 · Đăng nhập, Tạo kênh và Hệ thống"
status: pending
priority: P1
effort: ""
dependencies: [9]
---
# Phase 10 · Nhóm 07 · Đăng nhập, Tạo kênh và Hệ thống
## Mục tiêu / dependency
Nghiệm thu và sửa tới khi khớp **production** nhóm07; chỉ bắt đầu sau phase09 đạt. Áp dụng toàn bộ [hợp đồng nghiệm thu](acceptance-contract.md), không chọn subset dễ. Source authority Workflow/IA/copy/footer không đổi; không sửa artboard.
## Coverage / dữ liệu
- Board/state: Auth, MAuth, Onboard, CreateChannel, MCreate, System, MSystem; mọi Dialogs/Activity instance còn lại.
- Prerequisites thật: tài khoản admin/member nghiệm thu được cấp; instance first-run biệt lập cùng release commit cho đúng ngoại lệ#94A (kênh chung rỗng/identity chưa có user), đã chuẩn bị phase01; entity có quyền xoá/thử confirmations. Không nhầm account chưa dùng với instance chưa có dữ liệu.
- Re-scout source current deployed revision trước sửa; bảng dưới là boundary hiện có, không khẳng định đã có bug ở mọi file. Không sửa file chỉ để đổi nhãn hoặc để inventory trông hoàn tất.
## File inventory / owner
Controller tích hợp duy nhất, owner nhóm này; các nhóm tuần tự vì chung shell, kit, API và production. Không song song click/mutate cùng entity hoặc deploy.
| Action | Path / boundary tuyệt đối | Quy mô/test impact |
|---|---|---|
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/auth/LoginScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/auth/ChangePasswordScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/ChannelNewScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/CreateStep.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/DoneStep.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/shell/AppShell.tsx | Component/contract test tương ứng |
| Tạo evidence | /Users/dat/Projects/verse/plans/261106-1303-viderse-production-artboard-parity/reports/group-07/ | Matrix rows, ảnh, difference/control ledger và report ngắn |
| Sửa sau đọc | /Users/dat/Projects/verse/apps/viderse/docs/design/v1/10-gap-closure.md; 11-hoi-dap.md cùng thư mục | Một dòng/gói với deployed SHA; chỉ câu hỏi lệch mới khi cần |
## Bước thực hiện
1. Bổ sung Auth desktop vào matrix mới; mọi screen/kicked/err/role/other/app/step/down/pwerr và footer props còn lại đều có row. MAuth tương ứng không bị giới hạn mobile-only cũ. Chạy first-user setup thật trên instance biệt lập đã duyệt#94A, ghi URL/environment/hash riêng và verified-exception-94; KHÔNG gọi ảnh này production hoặc reset identity chung. Setup lỗi/Recovery chỉ nằm ngoại lệ nếu là first-run không thể dựng an toàn; lỗi runtime khác vẫn phải production. Trước mount desktop, GET /api/setup/state và approval auto-install theo contract.
2. Đăng nhập/đăng xuất/denied/change password/account/menu/devices qua tài khoản nghiệm thu riêng được cấp, không thu hồi session/password của người dùng. Test SSO Noverse link/grant theo policy thật, không tự cấp quyền hay nới cookie/same-origin. Lỗi wrong/limited chỉ nếu có tài khoản/rate-limit window an toàn, không lockout user.
3. Onboard kiểm trên instance thật chưa có kênh toàn hệ thống theo#94A, không dùng account mới ở production đã có kênh. CreateChannel/MCreate còn lại kiểm production: create/done, rivals some/none/empty, template menu và identity running/failed/director. Theo dõi & chọn theo dõi nguồn thật/lưu qua reload; autoFetch/autoApprove/perDay1–20 và template default giống Kênh. Sau create/link/toggle cuối phải đọc cấu hình kết thúc, tắt future admission của entity nghiệm thu khi hết cửa sổ và reconcile jobs/receipts; chỉ giữ bật nếu được duyệt tiếp.
4. System/MSystem loading/offline/notfound/empty và confirmations delVideo/delChannel/redrawAll/restart/clip/typed; offline chỉ browser cá nhân, notfound bằng target không tồn tại, không tắt backend để chụp down. Mỗi dialog và parent source group có ledger đủ, không pass board chỉ từ một nút.
5. Đối chiếu toàn manifest29board và probes: không row không owner/chưa kiểm; zero forbidden UI strings trên mọi reachable state/overlay. Những state không dựng được an toàn còn blocked phải giải quyết bằng prerequisites thật hoặc số quyết định11-hoi-dap chốt trước gọi hoàn tất; không ngầm thu hẹp phạm vi.
6. Sau nhóm07, xác minh final deployed commit và public assets/process, recheck các nhóm trước bị code sau ảnh hưởng. Báo nhóm07 theo cùng format; chỉ khi bảy nhóm đạt, physical Safari hợp lệ và không còn blocker mới ghi toàn workflow hoàn tất.
7. Mọi mismatch ghi expected/observed vị trí/khoảng cách/font/màu/icon/copy/click. Sửa cause ở owner, focused verification và native smoke, merge main rồi deploy qua phase03, chụp lại đúng production state. Recheck nhóm trước bị ảnh hưởng; intentional difference chỉ khi có số11-hoi-dap đã chốt.
## Verification / test matrix
- Focused khi có code change: npm test -w apps/viderse/web -- screens/auth screens/channel shell; npm run typecheck -w apps/viderse/web; npm run build -w apps/viderse/web. Nếu chỉ audit, browser thật là proof, không chạy suite để lấy số xanh. Backend/kit đổi thì focused+broader theo contract; không chạy build/test lúc agent còn sửa.
- Production: browser mở đúng HTTPS, authenticated state; capture reference+production+side-by-side1440×900/390×844 theo counterpart; fonts.ready, image decode, exact state, geometry và motion. Baseline/Playwright QA đang loopback-only KHÔNG phải production test command.
| Mức | Scenario | Oracle |
|---|---|---|
| Critical | Không tác động identity/session người dùng; first-run instance cùng release theo#94; same-origin/grant giữ; full-state completeness không zero-count giả | Request/readback/entity/version đúng; không no-op/side-effect ngoài phạm vi; evidence phân biệt production/ngoại lệ |
| High | Mọi footer value/context và visible controls | Row đầy đủ,0dead,0unverified enabled; disabled có lý do |
| High | Font/tab/menu/sheet/touch | Slider≤2px, không overflow,44px và không đẩy content |
| Medium | Normal/reduced, reopen, resize, route change | Motion/focus/scroll/safe-area đúng; không leftover overlay |
## Checklist
- [ ] Đủ board/tweak/cặp ảnh production; đúng các state first-run có evidence thật riêng theo#94A; mọi lệch đã sửa hoặc có số quyết định chốt, không gọi ảnh ngoại lệ là production.
- [ ] Control ledger đầy đủ,0không phản hồi,0enabled chưa kiểm; geometry/motion đạt.
- [ ] Commit đã deploy được chứng minh; báo nhóm07 bảng3cột + tối đa6ảnh + số controls/dead/chưa kiểm + URL/SHA; gap mỗi gói một dòng.
## Rủi ro / security / rollback
Không thay dữ liệu/credential người dùng để seed; xác định quyền/chi phí/recovery trước mutation, thiếu thì blocked chứ không pass. Sửa production quay qua phase03 và đúng rollback boundary, không restore DB cũ sau writer reopen. Shared code thay phải invalidate/retest evidence bị ảnh hưởng và proof Safari nếu cần. Báo nhóm ngay khi đạt rồi tiếp tục phase sau, không dừng ở báo cáo trung gian.
