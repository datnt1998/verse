---
phase: 8
title: "Nhóm 05 · Làm video, Khám phá và Hàng chờ"
status: pending
priority: P1
effort: ""
dependencies: [7]
---
# Phase 08 · Nhóm 05 · Làm video, Khám phá và Hàng chờ
## Mục tiêu / dependency
Nghiệm thu và sửa tới khi khớp **production** nhóm05; chỉ bắt đầu sau phase07 đạt. Áp dụng toàn bộ [hợp đồng nghiệm thu](acceptance-contract.md), không chọn subset dễ. Source authority Workflow/IA/copy/footer không đổi; không sửa artboard.
## Coverage / dữ liệu
- Board/state: NewVideo, MNew, Explore, MExplore, MCompVideo, Queue, MQueue.
- Prerequisites thật: Rival/source videos thật có chương, nhiều link hợp lệ và duplicate, template override, queue nghiệm thu và quota/cửa sổ chạy được duyệt.
- Re-scout source current deployed revision trước sửa; bảng dưới là boundary hiện có, không khẳng định đã có bug ở mọi file. Không sửa file chỉ để đổi nhãn hoặc để inventory trông hoàn tất.
## File inventory / owner
Controller tích hợp duy nhất, owner nhóm này; các nhóm tuần tự vì chung shell, kit, API và production. Không song song click/mutate cùng entity hoặc deploy.
| Action | Path / boundary tuyệt đối | Quy mô/test impact |
|---|---|---|
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/new/NewScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/new/quickCreate.ts | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/explore/ExploreScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/explore/PreviewPanel.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/explore/mobile/MExploreScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/explore/mobile/MVideoScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/queue/QueueScreen.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/screens/queue/MobileQueue.tsx | Component/contract test tương ứng |
| Đọc/sửa khi có lệch | /Users/dat/Projects/verse/apps/viderse/web/src/api/clone.ts | Component/contract test tương ứng |
| Tạo evidence | /Users/dat/Projects/verse/plans/261106-1303-viderse-production-artboard-parity/reports/group-05/ | Matrix rows, ảnh, difference/control ledger và report ngắn |
| Sửa sau đọc | /Users/dat/Projects/verse/apps/viderse/docs/design/v1/10-gap-closure.md; 11-hoi-dap.md cùng thư mục | Một dòng/gói với deployed SHA; chỉ câu hỏi lệch mới khi cần |
## Bước thực hiện
1. Nhập nhanh là dialog PC/màn phủ phone, cùng input empty/pasted/duplicate/error/multi, from topbar/explore, channel/template picker. Một CTA, cancel không enqueue, phím N không xuyên input/dialog.
2. Explore đủ selection/rival menu/preview/scan/search/list errors/offline; bulk selection mở cùng Nhập nhanh, giữ lựa chọn. MCompVideo là ngoại lệ đã chốt: sheet kênh direct add default, Vừa thêm đúng kênh; link phụ mở MNew để override. Link chương mở YouTube đúng mốc.
3. Create qua POST/api/jobs; default/override thực, override không sửa default; duplicate cùng source+kênh trả existing queueItemId hoặc jobId đúng target, source cùng kênh khác vẫn được. Double submit/retry/partial batch không tạo job hoặc thu phí lặp.
4. Queue running/paused/limited/empty, mobile rules/row sheet; workers1–6, cadence theo kênh, không priority/imageWorkers/rescanHours trên client. Manual prepend giữ thứ tự batch, auto tail không preempt running; queue-item→job link và reload theo đúng entity.
5. Đo footer/keyboard/dock trên phone và physical Safari khi có sửa; các paid enqueue/pause/global workers tác động người dùng phải có approved safe action window. Stress>2000queue và race chỉ DB QA riêng; production có safe real job proof không dùng stress thay dữ liệu thật.
6. Mọi mismatch ghi expected/observed vị trí/khoảng cách/font/màu/icon/copy/click. Sửa cause ở owner, focused verification và native smoke, merge main rồi deploy qua phase03, chụp lại đúng production state. Recheck nhóm trước bị ảnh hưởng; intentional difference chỉ khi có số11-hoi-dap đã chốt.
## Verification / test matrix
- Focused khi có code change: npm test -w apps/viderse/web -- screens/new screens/explore screens/queue; npm run typecheck -w apps/viderse/web; npm run build -w apps/viderse/web. Nếu chỉ audit, browser thật là proof, không chạy suite để lấy số xanh. Backend/kit đổi thì focused+broader theo contract; không chạy build/test lúc agent còn sửa.
- Production: browser mở đúng HTTPS, authenticated state; capture reference+production+side-by-side1440×900/390×844 theo counterpart; fonts.ready, image decode, exact state, geometry và motion. Baseline/Playwright QA đang loopback-only KHÔNG phải production test command.
| Mức | Scenario | Oracle |
|---|---|---|
| Critical | Canonical POST/jobs; manual-front/dedup/partial retry; admission snapshot override/default; correct queue→job target. | Request/readback/entity/version đúng; không no-op/side-effect ngoài phạm vi |
| High | Mọi footer value/context và visible controls | Row đầy đủ,0dead,0unverified enabled; disabled có lý do |
| High | Font/tab/menu/sheet/touch | Slider≤2px, không overflow,44px và không đẩy content |
| Medium | Normal/reduced, reopen, resize, route change | Motion/focus/scroll/safe-area đúng; không leftover overlay |
## Checklist
- [ ] Đủ board/tweak/dữ liệu production thật và cặp ảnh; mọi lệch đã sửa hoặc có số quyết định chốt.
- [ ] Control ledger đầy đủ,0không phản hồi,0enabled chưa kiểm; geometry/motion đạt.
- [ ] Commit đã deploy được chứng minh; báo nhóm05 bảng3cột + tối đa6ảnh + số controls/dead/chưa kiểm + URL/SHA; gap mỗi gói một dòng.
## Rủi ro / security / rollback
Không thay dữ liệu/credential người dùng để seed; xác định quyền/chi phí/recovery trước mutation, thiếu thì blocked chứ không pass. Sửa production quay qua phase03 và đúng rollback boundary, không restore DB cũ sau writer reopen. Shared code thay phải invalidate/retest evidence bị ảnh hưởng và proof Safari nếu cần. Báo nhóm ngay khi đạt rồi tiếp tục phase sau, không dừng ở báo cáo trung gian.
