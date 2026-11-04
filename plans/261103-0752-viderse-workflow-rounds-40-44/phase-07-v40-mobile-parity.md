---
phase: 7
title: "Bước 6 · V40 Điện thoại và nghiệm thu toàn luồng"
status: pending
priority: P1
dependencies: [6]
---

# Bước 6 · V40 — PR riêng

## Mục tiêu, nguồn và exclusion
Hoàn thiện mọi màn <768px theo 01-ia-flows §6.2 khối **Đợt44**, không chỉ ẩn phần desktop. Nhãn, chức năng, lựa chọn và mặc định giống desktop sau V36–V39. Source V40, MMain/MVideo/MReview/MChannel/MLibrary/MTemplate/MNew/MExplore/MCompVideo/MQueue/MMore/MSettings/MSystem/MCreate/MNotifications/MTop và các màn phụ được §6.2 giữ nguyên.
**Không thiết kế/triển khai mobile shared image viewer đợt45**, không tự làm mobile Editor. Giữ MReview và MobileImageSheet hiện có, sửa nội dung/feedback được V40 chỉ rõ. [Inventory](reports/inventory.md), [verification](verification.md), [backend scope](backend-contracts.md).

## File inventory
| Hành động | File tuyệt đối / phạm vi | Test impact |
|---|---|---|
| Sửa/xoá obsolete | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/workbench/mobile/MobileWorkbench.tsx`, `MobileWorkbenchList.tsx`, `MobileWorkbenchDetail.tsx`, `MobilePlanSheet.tsx`, `MobileRecap.tsx`, `MobileImageSheet.tsx` | MMain bỏ recap; MVideo2tabs + Info sheet, content default, bỏ composer/plan/why/video overrides |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/review/mobile/MReviewScreen.tsx`; shared `imageView.ts`/prompt contracts | Bỏ Vì sao/learned; 4parts, feedback/version/compare vẫn đúng |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/` mobile styles/branches, library mobile MLibrary/MTemplate/MPrompt/MOverlay/MBackdrop screens | Default template, Automation, fourequal tabs, no rotation/autonomy/schedule |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/new/`, `screens/explore/mobile/`, `screens/queue/MobileQueue.tsx` và sheets | Nhập nhanh keyboard-safe, source selection/filter/action parity |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/shell/MobileShell.tsx`, `AppHeader.tsx`, `AppShell.tsx`, `ActivityPopover.tsx`; settings/auth/mobile copy/style | Actual shell owners của More/tabbar/topbar, không tạo file MobileBar/MoreSheet/Topbar phantom |
| Backend hạn chế | `/Users/dat/Projects/verse/apps/viderse/internal/aiconn/login.go`, `agents.go`, `internal/imagegen/service.go` draw-key/receipt/fallback owner | Không new mobile API/lifecycle; reuse actual idempotency/quota, không double-bill |

## Thực hiện
1. Re-scout từng mobile branch và đối chiếu manifest. Không thay shared desktop defaults để khớp mobile rồi làm desktop lệch. Desktop cũng phải giữ gate sau mỗi mobile change.
2. MMain bỏ recap. MVideo chỉ Nội dung đăng/Ảnh, mặc định Nội dung đăng; Thông tin thành sheet gộp trợ lý/nguồn/video nền; tab Ảnh mang count, redraw chỉ trong context thật được thiết kế, không mới hoá viewer45.
3. MChannel sources/templates/settings, Automation3hàng, một default. MLibrary4tabs bằng nhau, ⋯ đặt default; MTemplate “Kênh dùng làm mặc định”. Create3hàng và default fixtures QA khớp desktop; không overwrite channel thật.
4. MNew Nhập nhanh theo PR5, nhiều link/explore/template override/cancel; keyboard không che CTA và footer không kẹt trên cao sau dismiss. MExplore bulk mở MNew; MCompVideo giữ sheet kênh direct add/default và Vừa thêm, link phụ MNew để override; chương mở YouTube đúng mốc. MQueue chỉ workers + per-channel cadence; bỏ why/Ưu tiên.
5. MMore đúng4hàng theo artboard, Theme có3lựa chọn dùng shared data/policy. MSettings và desktop không có data-key card. Đăng xuất ChatGPT/Grok dùng confirm nói ảnh hưởng thật và API hiện có; đang drawing nếu fail credential phải chuyển fallback theo existing chain, không gửi lại request đã thành công/đang được provider xử lý một cách mù. Nếu copy yêu cầu thời điểm chuyển không thể được API bảo đảm, ghi câu hỏi vào 11-hoi-dap trước sửa copy/hành vi.
5a. Gate parity phát hiện khi nghiệm thu Settings V36: desktop có “Kiểm tra lại” công cụ, mobile hiện không có ở cả code và MSettings. Hai ca mobile recheck trong lượt47 được ghi skip, **không phải đã hoàn thành**. Trước cutover PR6, đối chiếu Workflow/§6.2 và ghi hỏi11-hoi-dap nếu cần thêm vị trí control khác artboard; không tự thêm bố cục hoặc bỏ chức năng parity. Chốt xong phải kiểm mobile native POST `/api/tools/check`, readback và không install.
6. MReview và desktop Review bỏ Vì sao, Ghi chú đã học và câu mức Cân bằng; shared submitted prompt chỉ Cảnh/Phong cách/Tham chiếu/Tránh. Prompt raw/nguồn giữ ngôn ngữ thực, mọi nhãn/error/progress/notification Việt, không stringify raw exception lên màn. CreateChannel/MCreate nút Theo dõi & chọn phải nối action thật và giữ lựa chọn sau tải lại.
7. Motion đủ mở/đóng/chuyển tab/agent feedback/live progress/ảnh mới/toast/undo; dùng token có sẵn và reduced-motion. Dock/tabbar bám safe area/visualViewport theo shared shell, tránh height 100vh cứng. Safari thiết bị thật là gate cho bug keyboard/bottom nav, Chromium chỉ là coverage bổ sung.
8. Sweep toàn UI source/callers lần cuối cho removed terms/contracts; không sửa docs/artboard hoặc seed giả để làm số đo pass. Phần chưa có API/provider thật phải có blocker và owner, không nghiệm thu bằng mock.

## Test matrix
| Mức | Scenario | Qua khi |
|---|---|---|
| Critical | Mobile thao tác same entity rồi desktop reload và ngược lại | Nhãn/option/default/data/action cùng contract, không hai feature sets |
| Critical | Logout provider khi idle/drawing/fallback unavailable | Confirm đúng, credentials không lộ, không duplicate billing/result; failure có đường tiếp tục |
| Critical | Keyboard open→scroll→dismiss, rotate/URL bar trên Safari | CTA tới được, dock không kẹt cao, scroll/focus khôi phục |
| High | MVideo/MReview prompt4parts/version/compare/redraw | Đúng ảnh/version, actions thật; không learned/why/plan |
| High | Every MChannel/MLibrary/MTemplate/MNew/MExplore/MQueue/MMore tweak | Đủ manifest row + control ledger,44px, overlay trong viewport |
| Medium | Normal/reduced motion, repeated sheet openclose/route changes | Không leftover scrim/focus/scroll lock/animation or duplicate notifications |

Focused `npm test -w apps/viderse/web -- mobile screens/new screens/queue shell screens/settings`; typecheck/build, broad `npm test -w apps/viderse/web` sau integrate; `npm run test:engine -w apps/viderse/web` nếu shared editor imports đổi. Backend focused aiconn/draw tests nếu sửa plus actual configured QA provider smoke; không suy API202 thành ảnh đã xong.
Playwright toàn state manifest ở 1440×900/390×844 và boundary sizes theo verification.md; screenshots/reference/comparison, control ledger 0 unresponsive, slider≤2px, motion trace, real Safari keyboard proof. V40 gap ghi commit/date/limits thật.

## Rủi ro / security / rollback
Provider credential là machine-scope, **QA human account/app DB không đủ**. Trước logout/draw phải qua isolation receipt HOME/config/auth.DefaultStore, app+identity DSNs, media/blob và browser storage như verification.md; thiếu thì block, không xoá credential thật. Không đổi provider production, kill Chrome user hoặc thiết kế mobile45.
Revert PR6 quay lại source sau PR5, giữ migrations/canonical contracts mới; không quay về fields cũ chỉ để UI mobile compile.

## Checklist
- [ ] Map và hoàn thiện mọi mobile screen/state trong §6.2, cùng capability/default desktop.
- [ ] Prompt4parts, logout confirm/fallback, motion/feedback/dock/keyboard đúng.
- [ ] Full removed-feature and caller sweep; all controlled interactions and actual smoke passed.
- [ ] Gửi bảng đã sửa/còn lệch + comparisons + trace/control evidence + gap V40; chỉ chốt full sau mọi gate, không auto deploy trong planning.
