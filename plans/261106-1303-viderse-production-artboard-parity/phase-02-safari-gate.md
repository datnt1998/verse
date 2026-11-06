---
phase: 2
title: "Gate Safari iPhone thật #93"
status: pending
priority: P1
effort: ""
dependencies: [1]
---
# Phase 02 · Safari iPhone thật trước deploy
## Mục tiêu / quyết định
Người dùng chọn **Tự test iPhone trong1ngày** ngày06/11. KEEP: không có waiver, hết1ngày không tự deploy. Đồng hồ bắt đầu khi giao preview truy cập được của candidate cuối cùng; ghi thời điểm giao và hạn cụ thể vào report.
## File inventory
- Đọc:/Users/dat/Projects/verse/plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-06/physical-safari-handoff.md (checklist và route LAN).
- Sửa:/Users/dat/Projects/verse/apps/viderse/docs/design/v1/11-hoi-dap.md (#93), handoff trên (URL/source/scope thật nếu thay).
- Tạo trong plan này: reports/safari-physical.md và clip đã redaction; chỉ record bằng chứng người dùng cung cấp.
## Bước thực hiện
1. Kiểm owner/service/port8045 hiện tại trước reuse; không mặc định preview24bf453d còn sống hoặc khớp candidate. Source/assets hash phải map với main merged V40; nếu khác cần rebuild preview từ candidate và kiểm route truy cập. Preview chỉ dữ liệu mẫu rõ nhãn, không auth/env/production API/provider, không dùng preview chứng nhận backend.
2. Giao URL LAN thực và checklist, xác nhận iPhone cùngLAN. Nếu không truy cập, xử lý route an toàn; không tự public tunnel/DNS hoặc hạ auth. Ghi thời điểm nhận preview.
3. Người dùng ghi model iPhone/iOS/Safari, portrait/landscape; quay liên tục: Nội dung đăng mở/đóng keyboard và cuộn; Duyệt→góp ý/vẽ lại và dismiss; prompt→sửa chỉ dẫn handoff; URL bar co/mở; xoay dọc-ngang-dọc; dock/safe area. Lặp ít nhất3vòng normal, rồi bật Giảm chuyển động và lặp. Không submit paid action trên preview.
4. Ghi PASS/FAIL từng dòng, clip và revision. Chromium/WebKit desktop không thay iPhone. Lỗi: quay về phase01 sửa, giao preview mới và lặp test phần liên quan; không hứa deploy vào hạn1ngày nếu chưa đạt.
5. Cập nhật#93 đạt chỉ khi đủ clip/results trên đúng artifact; giữ lịch sử quyết định. Dừng service QA do phiên sở hữu sau handoff/test hoàn tất, kiểm PID/command trước SIGTERM; không kill service của người khác.
## Verification / success
Checklist handoff5mục có kết quả+clip, không CTA bị che, dock kẹt, overflow, overlay chồng/focus/scroll lock. Source final/candidate không khác phần Safari vừa test. Đây là physical layout gate, production acceptance vẫn ở phase04–10.
## Checklist
- [ ] Giao preview đúng candidate, ghi giờ bắt đầu/hạn1ngày.
- [ ] Nhận clip/device metadata và5mục checklist đạt; cập nhật#93 theo evidence.
- [ ] Xác nhận artifact không đổi phần liên quan; cho phép sang phase03, không tự deploy từ planning.
## Rủi ro / rollback
Không có thiết bị/operator/clip hoặc mạngLAN không tới=blocked, không coi timeout là waiver. Không thay runtime production; chỉ phục hồi/dừng preview đúng owner. Nếu hotfix sau này đổi keyboard/dock/overlay, proof tương ứng phải làm lại trước rollout.
