---
phase: 3
title: "Bước 2 · V36 Trình xem ảnh và nối nút"
status: pending
priority: P1
dependencies: [2]
---

# Bước 2 · V36 — PR riêng

## Mục tiêu, nguồn và ranh giới
Mọi entrypoint xem ảnh **desktop** dùng cùng viewer có context thật; không mở rộng MobileImageSheet thành thiết kế đợt45. Nguồn: 08-dev-prompt V36, Workflow quy ước2/5/6, 04-components mục Trình xem ảnh, Main/Review/Editor/Template/Library, SettingsConnect.
[Kiểm kê](reports/inventory.md), [backend scope](backend-contracts.md), [nghiệm thu chung](verification.md) là phần của phase này.

## Kiến trúc và file inventory
| Hành động | File tuyệt đối / phạm vi | Test impact |
|---|---|---|
| Tạo | `/Users/dat/Projects/verse/apps/viderse/web/src/components/image-viewer/ImageViewer.tsx`, `image-viewer.stylex.ts`, `types.ts` | Shared viewer với context typed, không một viewer khác cho mỗi screen |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/workbench/LiveStudio.tsx`, `WorkbenchDesktop.tsx` | Tile click mở đúng ID, nền không nhận shortcut |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/review/ReviewScreen.tsx`, `ImageRail.tsx`, `imageView.ts`, `useImageEditing.ts` | Reuse versions/prompt/refs/actions thay copy logic; giữ polling và optimistic revisions |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/editor/v13/Editor.tsx`, `Inspector.tsx`, `SceneImagePanel.tsx`, `scene-images.ts`; library `template/PreviewPane.tsx`, `CoverSection.tsx`, `PromptPicker.tsx`, `PromptsTab.tsx` | Routed Editor là v13, reuse clipIdFor/sceneImageFor/useSceneRedraws/useSceneVersion/undoSceneSwap; không nối nhầm editor/ui/inspector chưa được route dùng |
| Sửa/xoá feature obsolete | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/settings/`; `/Users/dat/Projects/verse/apps/viderse/internal/web/performance.go`, `admin_gate.go`; client performance key | Bỏ data-key cả desktop/mobile và actual endpoint, giữ mark-posted/export nếu còn dùng |
| Sửa tests | Các test workbench/review/library/editor có consumer, browser suite mới trong verification.md | Regression selection/version/keyboard/close, state/API thật |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/router.ts`, `App.tsx`, `screens/review/ReviewScreen.tsx`, source notification href builders | Typed img address cùng job/video; parse/serialize/canonicalize không strip ảnh đã chọn; browser-history/deep-link tests |

## Thực hiện
1. Re-scout imports và entrypoints ảnh; lập typed context `review | view | view-replace | sample`, phân biệt với prompt mode `view | raw | edit`. ID job/image/version là nguồn sự thật, không index dễ trôi khi polling.
1a. Mở rộng route/paths cho địa chỉ ảnh `img` theo ID ổn định, qua parseHash/routePath và App canonicalization; truyền vào review/viewer selection sau tải ảnh. Click ảnh không phải ảnh đầu, direct link, reload, notification entry, Back/Esc phải quay đúng ảnh/context; ID mất thì phản hồi và chọn fallback hợp lệ, không mutation nhầm ảnh đầu.
2. Viewer tối đa1240×800, ảnh contain không upscale quá natural size, strip đủ ảnh; placeholder đang/chưa vẽ không bấm được, ←/→ bỏ qua. Khi ảnh bị xoá/job đổi, đóng hoặc chọn ảnh hợp lệ có phản hồi, không thao tác nhầm ảnh.
3. Duyệt: mở đúng ảnh từ Bàn dựng→Duyệt và giữ đúng job; versions/prompt/refs/feedback/edit/drop/approve thật. View: ảnh sẵn sàng/đang làm, Góp ý ở cột phải; confirm báo tốn số ảnh + hạn mức thật, kết quả “Đã gửi Hoạ sĩ · bản 2”. View-replace: thêm thay ảnh cảnh bằng command/undo hiện có; sample: ảnh prompt thật, không versions giả, Vẽ thử nối action đang có.
4. Khung đóng bằng ×/Esc/scrim, focus về trigger; trap focus theo overlay kit. Shortcuts viewer không xuyên nền hoặc chạy khi nhập prompt. Confirm đặt trên viewer theo spec, nhưng chỉ có một lớp nhận tương tác; không lồng dialog xác nhận trong dialog khác.
5. Nối `Vẽ lại tất cả` trong Review với confirmCount/expectedRevision thật; không chỉ có nút ở Dialogs/demo. Mọi ảnh mẫu/preview entrypoint có cùng viewer, không mở các popup độc lập còn sót.
6. Bỏ YouTube Data API card và route `/api/youtube/data-key` thực tế; không xoá cả performance.go nếu còn API hợp lệ. Deferred removals learned-rules/suggestions/assistant chỉ xoá cùng final FE consumer ở PR3/PR4 để PR2 không để nút hỏng.
7. Structured prompt mới bỏ learned khi wire viewer; raw prompt cũ là provenance bất biến. Backend chỉ bổ sung contract/action mà V36 cho phép, reuse image APIs; không dựng provider preview giả.

## Ma trận test và acceptance
| Mức | Scenario | Qua khi |
|---|---|---|
| Critical | Mở cùng image từ mọi entrypoint, đổi job/polling trong lúc xem | Đúng ảnh/version/quyền; không gửi sửa nhầm target |
| Critical | Redraw/drop/approve/replace, double-click, quota/network error | Đúng mutation một lần, consequence thật, feedback và readback; undo giữ timeline/keyframes |
| High | View/review/view-replace/sample + strip placeholders | Đủ mode thật, không controls dead, mẫu không hiện history giả |
| High | Esc/scrim/focus/R/E/A/arrows/textarea | Chỉ đúng lớp/ảnh tác động; focus restore, không background shortcut |
| High | Click ảnh thứ3, direct img link/reload/Back/Esc và ảnh bị xoá | Canonicalization giữ img đúng, selection theo ID, trở về đúng context; không reset mặc định ảnh0 |
| Medium | Natural size, ảnh load lỗi/portrait, 1440 và layout hẹp | Không upscale/clipping/overflow, lỗi có đường tiếp tục |

Focused commands: `npm test -w apps/viderse/web -- screens/review screens/workbench screens/library`; `npm run typecheck -w apps/viderse/web`; backend `go test ./apps/viderse/internal/web ./apps/viderse/internal/clone/images` nếu đổi. Browser audit theo verification.md, không lấy test forwarding làm proof.
Mỗi tweak relevant có code/reference/comparison hai viewport và ledger; mobile viewer45 ghi excluded, MReview/data-key mobile vẫn phải kiểm. Cập nhật dòng V36 gap bằng commit thật và mọi lệch còn lại.

## Rủi ro / security / rollback
Preview URL phải qua auth và data owner như hiện tại, không phục vụ path/URL tuỳ ý. Kết quả ảnh có thật và quota có thật; nếu thiếu binding scene→job/media không được đoán bằng filename, ghi câu hỏi trước khi thay contract ngoài package.
Viewer chung dùng cùng mutation/cache owner, không mở subscription trùng. Revert PR trước PR3; giữ render history/data, không huỷ job người dùng.

## Checklist
- [ ] Chốt references, typed contexts và mọi desktop image entrypoint.
- [ ] Hoàn thành viewer/actions/confirmation và bỏ data-key cả hai bố cục.
- [ ] Qua focused tests + build/typecheck + actual UI/state/tweak gates.
- [ ] Gửi comparison/control/motion report, gap V36; merge PR2 trước PR3.
