---
title: Viderse PC — canonical artboard parity and verified Mac mini release
status: in-progress
priority: P1
branch: main
---

# Outcome

Complete the existing Viderse desktop interface against the published canvas65 artboards, including workflow, UIUX details, shared UIKit and motion. User explicitly rejects mobile-only work or a release report without the actual Mac mini PC interface updated.

Design authority: [design README](../../apps/viderse/docs/design/v1/README.md), then `01-ia-flows.md` for behavior, artboards for geometry, and component/kit/token specs. Preserve current accepted taste; no redesign.

## Design brief

- Register: Product.
- Scene: desktop operator managing multiple video channels; 1440×900 canonical, 1100/1280 breakpoints.
- Direction: exact canonical Viderse shell, content hierarchy and state-specific actions; reuse `@noverse/ui` rather than create another kit.
- Color/type: existing mapped Noverse surfaces, Viderse red accent, pale primary CTA, orange destructive/error, Be Vietnam Pro and JetBrains Mono; exact tokens own values.
- Signature: lens logo and four assistant roles; only meaningful running-state animation.
- Dials: preserve canonical density and 150–200ms transitions; all repeated effects stop for reduced motion.

## Scope / ownership

| Slice | Owning files | Acceptance |
|---|---|---|
| Workbench + Review | `screens/workbench/` and `screens/review/`, excluding mobile folders | Main/Review canonical columns, panels, state actions, metadata/review interactions, image reveal and keyboard |
| Explore + Channels | `screens/explore/` and `screens/channel/`, excluding mobile folders | Explore/CreateChannel/Channel state geometry, controls, linked rivals, autonomy confirmation and identity states |
| Assets + Settings + entry | `screens/library/`, `screens/settings/`, `screens/queue/`, `screens/new/`, `screens/auth/`, `screens/account/`, `screens/setup/`, excluding mobile folders | Library/Template/Settings/Queue/NewVideo/Auth/Onboard states and real routes/actions |
| Editor | `editor/v13/`, `screens/editor/`, affected audio/media boundary | Editor state/panel/inspector/timeline/export-gate parity; real measurement skips only server-confirmed silent video and preserves decode errors |
| Shell + UIKit + motion | desktop `shell/`, `components/`, desktop globals | Shell/Activity/Dialogs/Kit/Brand fidelity; shared primitives, focus, menus, hover/pressed/loading, reduced motion |
| Integration | primary controller | Shared platform changes only if proven necessary; tests/build once after all slices freeze; actual browser verification and sequential official deploy |

## Constraints

- Keep existing mobile repairs and API contracts intact. Shared files have one integration owner; slices message primary instead of editing outside ownership.
- No redraws, retries or user-content changes during audit. Original Noverse story remains paused. The short synthetic voice smoke has completed and its fixture is retired. Only primary may send the separately authorized single Director plan request through the final production PC UI; never run its proposed actions or retry it.
- Existing isolated API fixture and canonical artboard servers are reused; max two owned browser contexts. Current performance capture has exclusive browser/machine window; slices initially inspect/edit source only.
- No build/lint/tests/formatters mid-flight; integration runs focused behavioral gates, full frontend checks and actual served UI once slices freeze.
- No fake artboard data wired into product. Empty/error states come from actual model state. No style/source-only permanent tests.

## Execution / evidence

1. Compare implementation with relevant artboard/state and identify precise discrepancies before changes.
2. Repair within ownership; record source references and exact affected states in `plans/reports/261102-desktop-parity/`.
3. Freeze source; integrate and build. Capture canonical and actual at matching viewport and real supported states; exercise menus, keyboard and transitions. Never label source-only review or DOM counts as visual proof.
4. Run shared/mobile regressions when shared UI changes. Re-run current performance gates after final build.
5. Commit request-only changes using fresh private Git index, push and deploy via the existing Mac mini route. Verify revision, health, actual PC browser surface and original image-approval recovery without a quality bypass.
6. Completion requires all desktop slices and urgent Noverse/Viderse fixes verified on the deployed revision; report remaining physical-device limits explicitly.

## Current state

- Noverse và Viderse đã triển khai bản `ec3d2aed`; Noverse public nạp `index-CqmhAx93.js`, Viderse nạp `index-BWdKB7dj.js`. Máy đọc đã khôi phục sau lỗi launchd handoff; `/api/tts/status` trả `ready: true`.
- Truyện gốc `dit-dang-audio/01-truyen-moi` hiện đang chạy 10/12 chương, khác baseline tạm dừng trước deploy. Chưa xác định người khởi động; đã hỏi chủ tài khoản, không tự dừng hay tiếp tục truyện.
- Bản chốt `index-BWdKB7dj.js`: build/typecheck/size qua (entry 141,7 KiB, initial JS 191,3 KiB, CSS 33,8 KiB gzip). Ba lượt cold độc lập xấu nhất FCP 768 ms, TBT 26 ms, long task 76 ms, 25 request; CLS 0, không lỗi JavaScript.
- Bộ kiểm hiện tại: app 798 tests, UIKit 239 tests, editor scene 18 tests đều qua; cổng phiên 36 tests qua lại sau khi đổi boot; timeline façade 35 tests qua, gồm preview chỉ đọc không lưu và editor vẫn sửa bản lưu. Toàn bộ gói Go đã qua; PostgreSQL qua ở timeout 10 phút (269,6 giây). Regression lease và toàn gói preview cũng qua.
- Expanded acceptance: Vietnamese visible system copy and progress journal; Agent–User acknowledgement, pending/review/error feedback; 360/390/430 mobile and 1280/1440 desktop. Reuse Noverse interaction patterns without changing user content, provider names or API identifiers.
- Real isolated editor measurement now completes (−16.1 LUFS). The synthetic job correctly remains below the export threshold because it has no complete source/build manifest; no score is fabricated. Native testing exposed a false missing-media blocker: quality now resolves authoritative editor metadata pointers, and remeasurement confirms that blocker is gone.
- Đã đóng lỗi native: template detail/preview tự chờ lease; cue không sáng; motion bị giữ trong reduced-motion; landmark kênh mới; lý do provider hết hạn bị che; cold deep-link prompt; template trắng trang vì thiếu TooltipProvider; preview tự PUT project; popup lọc tràn mép; tên đối thủ dài bị cắt; hộp thoại phím tắt cao hơn viewport.
- Khám phá điện thoại trước sửa: nạp 1.000 video làm DOM lên 15.906 nút và heap 36,68 MiB. Sau virtualize, smoke API thật nạp 5.000 video: 10 thẻ đang gắn, 1.107 nút DOM, heap tăng 5,15 MiB, 60 fps; không sửa DOM/API trong harness.
- Native cuối: mobile 253/253 tại 360/390/430 và hai bộ inset; motion 23 mẫu gồm glide Segmented 320 ms và reduced-motion 0; PC Main/Review 46 hàng; Explore/Channel 239 kiểm tra; assets/settings 47 trạng thái, không PUT hay lỗi JavaScript. Editor 1440 đủ 18 trạng thái, 1280 qua đến export gate và kiểm riêng reduced-motion; dialog cuộn đến nút Đóng nằm trọn viewport. Hai marker prompt bị CSS uppercase là tiêu chí harness cũ, không phải lỗi giao diện.
- Hotfix avatar ở hàng duyệt mobile: cho badge tràn khỏi thumbnail nhưng giữ bo góc riêng cho ảnh. Smoke native 360/390/430 xác nhận điểm vẽ và hit-test ngoài cạnh trái/dưới đều nhìn thấy; desktop 1280/1440 không bị che, không lỗi JavaScript hay API ghi. Bản build `index-BuCmsTN2.js`, đang chờ triển khai.
- Giới hạn kiểm chứng: mô phỏng Chrome, chưa kiểm bàn phím Safari trên iPhone thật; dữ liệu canonical khác dữ liệu fixture. Không gọi sinh ảnh/giọng hay thực thi kế hoạch Director trong native. Chờ kiểm production và đúng một yêu cầu lập kế hoạch đã được cho phép.

Rollback: use the previous revision through the owning deployment scripts; preserve PostgreSQL and media, never reset user state or regenerate images.
