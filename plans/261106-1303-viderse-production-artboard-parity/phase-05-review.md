---
phase: 5
title: "Nhóm 02 · Duyệt"
status: pending
priority: P1
effort: ""
dependencies: [4]
---
# Phase 05 · Nhóm 02 · Duyệt
## Mục tiêu / dependency
Nghiệm thu và sửa tới khi khớp **production** nhóm02; chỉ bắt đầu sau phase04 đạt. Áp dụng toàn bộ [hợp đồng nghiệm thu](acceptance-contract.md), không chọn subset dễ. Source authority Workflow/IA/copy/footer không đổi; không sửa artboard.
## Coverage / dữ liệu
- Board/state: Review, MReview; Dialogs redraw-all và MSystem redrawAll.
- Prerequisites thật: Ảnh/version/raw và structured prompt thật, review job, failed/blocked và chi phí paid action được duyệt.
- Re-scout source current deployed revision trước sửa; bảng dưới là boundary hiện có, không khẳng định đã có bug ở mọi file. Không sửa file chỉ để đổi nhãn hoặc để inventory trông hoàn tất.
## File inventory / owner
Controller tích hợp duy nhất, owner nhóm này; các nhóm tuần tự vì chung shell, kit, API và production. Không song song click/mutate cùng entity hoặc deploy.
| Action | Path / boundary tuyệt đối | Quy mô/test impact |
|---|---|---|
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/review/ReviewScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/review/ImageRail.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/review/imageView.ts | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/review/useImageEditing.ts | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/review/useReview.ts | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/review/mobile/MReviewScreen.tsx | Component/contract test tương ứng |
| Tạo evidence | /Users/dat/Projects/verse/plans/261106-1303-viderse-production-artboard-parity/reports/group-02/ | Matrix rows, ảnh, difference/control ledger và report ngắn |
| Sửa sau đọc | /Users/dat/Projects/verse/apps/viderse/docs/design/v1/10-gap-closure.md; 11-hoi-dap.md cùng thư mục | Một dòng/gói với deployed SHA; chỉ câu hỏi lệch mới khi cần |
## Bước thực hiện
1. Đi đủ confirm/state/view/score/prompt/refs của Review và state/score của MReview; chọn ảnh/phiên bản/compare/full-cutout theo dữ liệu thật. Không thêm mobile shared viewer đợt45.
2. Prompt modern chỉ Cảnh/Phong cách/Tham chiếu/Tránh; legacy raw giữ đúng văn bản đã gửi, không gán wrapper cũ vào Cảnh. Sửa trên selected baseVersion, giữ refs kể cả rỗng và provenance, không ngầm đổi bản đang dùng.
3. Bấm chọn ảnh/version, keyboard shortcuts theo context/quyền, prompt view/raw/edit, add/remove refs, feedback/cancel/dismiss và batch approval/redraw nếu được phép. #92 skip→route change→undo/reload phải giữ snapshot, không approve/redraw/quota.
4. Paid redraw thực tế chỉ khi có entity/quota/approval; lưu request/receipt đúng một lần, chờ kết quả ảnh thật và readback version/score. Failed/blocked/reveal và pending không có fake percent; button disabled có lý do tiếngViệt. Không có quyền chi phí thì đánh blocked phần đó, QA khác không thay production.
5. Kiểm sheet handoff/focus/scroll lock/normal-reduced; nếu sửa ảnh hưởng Safari lặp physical gate trước deploy. Không còn Vì sao? hoặc Ghi chú đã học trong mọi state/overlay.
6. Mọi mismatch ghi expected/observed vị trí/khoảng cách/font/màu/icon/copy/click. Sửa cause ở owner, focused verification và native smoke, merge main rồi deploy qua phase03, chụp lại đúng production state. Recheck nhóm trước bị ảnh hưởng; intentional difference chỉ khi có số11-hoi-dap đã chốt.
## Verification / test matrix
- Focused khi có code change: npm test -w apps/viderse/web -- screens/review; npm run typecheck -w apps/viderse/web; npm run build -w apps/viderse/web. Nếu chỉ audit, browser thật là proof, không chạy suite để lấy số xanh. Backend/kit đổi thì focused+broader theo contract; không chạy build/test lúc agent còn sửa.
- Production: browser mở đúng HTTPS, authenticated state; capture reference+production+side-by-side1440×900/390×844 theo counterpart; fonts.ready, image decode, exact state, geometry và motion. Baseline/Playwright QA đang loopback-only KHÔNG phải production test command.
| Mức | Scenario | Oracle |
|---|---|---|
| Critical | Selected version khác used version; refs rỗng; Undo đúng snapshot qua route/reload; pending/conflict không ghi đè hoặc thu phí kép. | Request/readback/entity/version đúng; không no-op/side-effect ngoài phạm vi |
| High | Mọi footer value/context và visible controls | Row đầy đủ,0dead,0unverified enabled; disabled có lý do |
| High | Font/tab/menu/sheet/touch | Slider≤2px, không overflow,44px và không đẩy content |
| Medium | Normal/reduced, reopen, resize, route change | Motion/focus/scroll/safe-area đúng; không leftover overlay |
## Checklist
- [ ] Đủ board/tweak/dữ liệu production thật và cặp ảnh; mọi lệch đã sửa hoặc có số quyết định chốt.
- [ ] Control ledger đầy đủ,0không phản hồi,0enabled chưa kiểm; geometry/motion đạt.
- [ ] Commit đã deploy được chứng minh; báo nhóm02 bảng3cột + tối đa6ảnh + số controls/dead/chưa kiểm + URL/SHA; gap mỗi gói một dòng.
## Rủi ro / security / rollback
Không thay dữ liệu/credential người dùng để seed; xác định quyền/chi phí/recovery trước mutation, thiếu thì blocked chứ không pass. Sửa production quay qua phase03 và đúng rollback boundary, không restore DB cũ sau writer reopen. Shared code thay phải invalidate/retest evidence bị ảnh hưởng và proof Safari nếu cần. Báo nhóm ngay khi đạt rồi tiếp tục phase sau, không dừng ở báo cáo trung gian.
