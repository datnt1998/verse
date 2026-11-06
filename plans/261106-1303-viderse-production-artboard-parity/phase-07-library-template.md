---
phase: 7
title: "Nhóm 04 · Thư viện và Template"
status: pending
priority: P1
effort: ""
dependencies: [6]
---
# Phase 07 · Nhóm 04 · Thư viện và Template
## Mục tiêu / dependency
Nghiệm thu và sửa tới khi khớp **production** nhóm04; chỉ bắt đầu sau phase06 đạt. Áp dụng toàn bộ [hợp đồng nghiệm thu](acceptance-contract.md), không chọn subset dễ. Source authority Workflow/IA/copy/footer không đổi; không sửa artboard.
## Coverage / dữ liệu
- Board/state: Library, Template, MLibrary, MTemplate; Prompt/Overlay/Backdrop là state trong board theo footer.
- Prerequisites thật: Template nghiệm thu riêng và dùng chung, prompt/overlay/backdrop/media thật, completed source đủ tạo preview và bộ import được cấp.
- Re-scout source current deployed revision trước sửa; bảng dưới là boundary hiện có, không khẳng định đã có bug ở mọi file. Không sửa file chỉ để đổi nhãn hoặc để inventory trông hoàn tất.
## File inventory / owner
Controller tích hợp duy nhất, owner nhóm này; các nhóm tuần tự vì chung shell, kit, API và production. Không song song click/mutate cùng entity hoặc deploy.
| Action | Path / boundary tuyệt đối | Quy mô/test impact |
|---|---|---|
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/library/LibraryScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/library/TemplatesTab.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/library/AttachChannelDialog.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/library/TemplateScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/library/template/ImagesSection.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/library/template/OutputSection.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/library/template/PreviewPane.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/library/mobile/MLibraryScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/library/mobile/MTemplateScreen.tsx | Component/contract test tương ứng |
| Tạo evidence | /Users/dat/Projects/verse/plans/261106-1303-viderse-production-artboard-parity/reports/group-04/ | Matrix rows, ảnh, difference/control ledger và report ngắn |
| Sửa sau đọc | /Users/dat/Projects/verse/apps/viderse/docs/design/v1/10-gap-closure.md; 11-hoi-dap.md cùng thư mục | Một dòng/gói với deployed SHA; chỉ câu hỏi lệch mới khi cần |
## Bước thực hiện
1. Bốn tab template/prompt/video nền/lớp phủ, mobile bằng nhau; Library top-nav không cột trái cũ. Đi mọi Library tab/prompt/menu/nguon/import/kenh và Template section/preview/fewClips/picker; mobile list/screens/sheets/empty theo footer.
2. Kiểm gắn default cho kênh/readback nhiều kênh và nhãn Kênh dùng làm mặc định; menu xoá template đang dùng bị chặn đúng FK. Prompt built-in/mine/new quyền sửa đúng, ảnh bìa/mẫu dùng viewer thật.
3. Đọc template-output-audit trong plan cũ trước sửa Output; so vị trí/nhãn/control thật, không coi hotfix Joker là đã khớp toàn bộ Xuất & kênh. Ba chế độ ảnh chỉ serialize active fields, giữ draft khi đổi mode, không gửi none kèm count gây GET Editor400.
4. Tự lưu/readback/reload, preview fresh→stale→building→fresh phải qua builder thật; phát preview, clips/nền/overlay/import hợp lệ và lỗi/partial/offline theo luồng thật. Không rename/edit shared template của người dùng; bản nghiệm thu riêng.
5. Import/upload/delete/paid sample cần kiểm file/hash/quota/phạm vi trước bấm; cancel không ghi. Chỉ màn dựng lại được có ảnh production, các blocked không gọi pass.
6. Mọi mismatch ghi expected/observed vị trí/khoảng cách/font/màu/icon/copy/click. Sửa cause ở owner, focused verification và native smoke, merge main rồi deploy qua phase03, chụp lại đúng production state. Recheck nhóm trước bị ảnh hưởng; intentional difference chỉ khi có số11-hoi-dap đã chốt.
## Verification / test matrix
- Focused khi có code change: npm test -w apps/viderse/web -- screens/library; npm run typecheck -w apps/viderse/web; npm run build -w apps/viderse/web. Nếu chỉ audit, browser thật là proof, không chạy suite để lấy số xanh. Backend/kit đổi thì focused+broader theo contract; không chạy build/test lúc agent còn sửa.
- Production: browser mở đúng HTTPS, authenticated state; capture reference+production+side-by-side1440×900/390×844 theo counterpart; fonts.ready, image decode, exact state, geometry và motion. Baseline/Playwright QA đang loopback-only KHÔNG phải production test command.
| Mức | Scenario | Oracle |
|---|---|---|
| Critical | Autosave/version và active image mode; preview thực thay recipe; default FK/shared template; partial import không duplicate. | Request/readback/entity/version đúng; không no-op/side-effect ngoài phạm vi |
| High | Mọi footer value/context và visible controls | Row đầy đủ,0dead,0unverified enabled; disabled có lý do |
| High | Font/tab/menu/sheet/touch | Slider≤2px, không overflow,44px và không đẩy content |
| Medium | Normal/reduced, reopen, resize, route change | Motion/focus/scroll/safe-area đúng; không leftover overlay |
## Checklist
- [ ] Đủ board/tweak/dữ liệu production thật và cặp ảnh; mọi lệch đã sửa hoặc có số quyết định chốt.
- [ ] Control ledger đầy đủ,0không phản hồi,0enabled chưa kiểm; geometry/motion đạt.
- [ ] Commit đã deploy được chứng minh; báo nhóm04 bảng3cột + tối đa6ảnh + số controls/dead/chưa kiểm + URL/SHA; gap mỗi gói một dòng.
## Rủi ro / security / rollback
Không thay dữ liệu/credential người dùng để seed; xác định quyền/chi phí/recovery trước mutation, thiếu thì blocked chứ không pass. Sửa production quay qua phase03 và đúng rollback boundary, không restore DB cũ sau writer reopen. Shared code thay phải invalidate/retest evidence bị ảnh hưởng và proof Safari nếu cần. Báo nhóm ngay khi đạt rồi tiếp tục phase sau, không dừng ở báo cáo trung gian.
