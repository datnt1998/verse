# Bước 0 · Bảng kiểm kê trước sửa code

Ngày lập: 2026-11-03. Repository: `verse`; phạm vi sản phẩm `apps/viderse`. **Đây là kiểm kê source, chưa phải bảng đã sửa/đã nghiệm thu.** Chưa sửa code, tạo PR hoặc deploy từ kế hoạch này.

## Quyết định phạm vi
- HOLD SCOPE: giữ nguyên toàn bộ bước0–6, V36–V40, mỗi bước một PR riêng và theo thứ tự. Auto chọn **deep** vì refactor hơn 5 vùng, có schema/API/desktop/mobile; re-scout đúng phase trước khi sửa, không áp dụng một source map cũ cho cả dự án.
- Không thêm feature, không đổi artboard, không triển khai mobile image viewer đợt45 hoặc tự thiết kế mobile Editor.
- Reuse `@noverse/ui`, data/query cache, job images/provenance, queue transaction và channel identity có thật. Không làm lại UI kit, provider/storage hoặc editor engine.
- Code chạm nhiều hơn 8 file là do các màn/backend được gọi tên; không gộp 7 PR để giảm số phase trái yêu cầu. Hai adapter policy Viderse và một desktop viewer là các ranh giới mới có lý do, không thêm service/framework.

## Bảng màn/component ↔ artboard ↔ gói ↔ hành động
Paths dưới `apps/viderse/web/src` trừ khi ghi khác. Artboard là `docs/design/v1/artboards/<tên>.dc.html`; footer `data-props` thắng danh mục tweak cũ. Gói “Nền” dùng trước V36.

| Màn/component code hiện tại | Artboard / trạng thái cần đi | Gói / PR | Giữ · Sửa · Xoá và bằng chứng |
|---|---|---|---|
| `components/Segmented.tsx`; `platform/ui/src/Tabs.tsx:173-214` | UkControls/UkInputs; chọn, font/resize | Nền / 1 | **Giữ + kiểm/sửa consumer**. Kit đã có ResizeObserver nhóm/nút và fonts.ready, không viết offset bù |
| Kit Selects/Menus/Sheet/Overlays; `components/confirm.tsx` | UkOverlays; Dialogs; scrim/Esc/confirm | Nền / 1 | **Giữ primitive, sửa policy responsive** popover desktop/sheet mobile, hai loại dialog, không chồng lớp tương tác |
| `platform/ui/src/Toast.tsx:147-156`; shell `toastAnchor.ts` | UkFeedback/Toast; pill/card/undo | Nền / 1 | **Sửa giới hạn** Viderse 4s, explicit duration thắng undo5s; không đổi default Noverse |
| `shell/MainNav.tsx:7-20,38-47`; `AppShell.tsx`; `AppHeader.tsx`; `MobileShell.tsx:31-145` | Main + các shell; MMain, MMore/MTop/MNotifications | V37–40 / 3–6 | **Sửa owner hiện có** cho nav/Thư viện/Làm video/More/channel/progress; không tạo MobileBar/MoreSheet/Topbar hay WorkbenchRail chỉ vì source map cũ |
| `screens/workbench/WorkbenchDesktop.tsx:43-67,289-290` | Main/Dialogs/Activity/System | V37 / 3 | **Sửa**, bỏ recap auto-open, composer, plan và popup đề xuất; màn phụ reuse Main đã gọn |
| `screens/workbench/LiveStudio.tsx:42-47,86-129,168-181` | Main state + viewer=view/redraw | V36–37 / 2–3 | **Sửa** mọi tile mở ảnh đúng job/image ID; giữ target-lost guard, suppress key listener nền khi viewer mở |
| Chưa tìm thấy desktop shared viewer trong source; review helpers có sẵn | Main, Review, Editor, Template, Library; view/review/view-replace/sample | V36–37 / 2–3 | **Tạo viewer chung**, reuse review image/provenance/action hooks; không tạo viewer điện thoại đợt45 |
| `screens/review/ReviewScreen.tsx:51-56`; `ImageRail.tsx`; `imageView.ts`; `useImageEditing.ts` | Review compare/confirm/feedback/batch/prompt | V36 / 2 | **Giữ API/hành vi, hợp nhất phần xem**, confirm vẽ lại tất cả; phân biệt prompt mode với viewer mode |
| `screens/workbench/panels/WorkbenchPanels.tsx:25-46` | Main inspector=info/content; state | V37 / 3 | **Sửa** 4tab/agents default → Nội dung đăng + Thông tin, default Thông tin; thêm Tiến độ, không chip Ảnh |
| `components/assistant/*`, `screens/workbench/recap/*`, `components/learn/*`, explain surfaces | Main/Activity/Dialogs sau cắt | V37 / 3 | **Xoá phần obsolete** Bản tin/Nhắn trợ lý/Kế hoạch/Vì sao/Lưu quy tắc. Recap thực nằm dưới workbench; không xoá tool backend còn consumer hành động |
| `screens/channel/ChannelScreen.tsx:36-40,120-146`; router CHANNEL_TABS | Channel tab; MChannel tab | V38 / 4 | **Sửa** bỏ Video, default Nguồn & tự động theo Channel footer sources; còn Nguồn & tự động/Template mặc định/Hồ sơ |
| `screens/channel/ChannelAside.tsx`, `applied.ts`, `ScheduleCard.tsx`, `VideosTab.tsx` | Channel không cột phải/lịch/tab Video | V38 / 4 | **Xoá khi hết references** cả UI/tests/API chỉ cấp cho phần cắt; không xoá stats bốn ô còn trong board |
| `screens/channel/SourcesTab.tsx`; `components/autonomy/*`; mobile `AutonomyMobile.tsx` | Channel autoApprove/link, MChannel sheet | V38 / 4 | **Sửa** Automation3hàng: autoFetch/autoApprove85/perDay; bỏ priority/preset/schedule. **Giữ xác nhận Bỏ liên kết**, không tin suy diễn rằng thiết kế bỏ confirm |
| `screens/channel/TemplatesTab.tsx`; `api/channels.ts`; `api/autonomy.ts` | Channel templates/tplMenu; MChannel defaultTemplate sheet | V38 / 4 | **Sửa** một default và Đổi/Sửa; **xoá** rotation/cursor/public autonomy helpers sau migrate tất cả consumer |
| `screens/channel/SettingsTab.tsx`, `IdentityBlock.tsx`, `CreateStep.tsx`, `ChannelNewScreen.tsx`, `DoneStep.tsx` | Channel settings; CreateChannel/Onboard/MCreate; identity/tplMenu | V38 / 4 | **Giữ identity và lỗi thật, sửa** ba hàng Cách kênh chạy, bỏ learned, giữ language/tag/naming/delete flow |
| `screens/library/LibraryScreen.tsx`; `TemplatesTab.tsx`; `AttachChannelDialog.tsx` | Library tab/menu; Prompt/Overlay | V38 / 4 | **Sửa** bỏ cột trái, top-nav entry; Đặt làm mặc định cho kênh, nhãn kênh dùng, list nổi |
| `screens/library/mobile/MLibraryScreen.tsx`, `MTemplateScreen.tsx` | MLibrary/MPrompt/MOverlay/MBackdrop*/MTemplate | V38 contract / 4; V40 layout / 6 | **Sửa cùng contract PR4** để mobile không vỡ; PR6 hoàn thiện bốn tab bằng nhau, nhãn và menu/sheet |
| `screens/new/NewScreen.tsx:9,115-118`; `plan.ts`; `screens/RouteScreen.tsx`; router.ts | New source=links/explore/step; MNew keyboard | V39 / 5 | **Sửa** trang thành Nhập nhanh dùng chung, một CTA; bỏ priority/chọn video/xem lại; đổi template chỉ video này |
| `screens/explore/ExploreScreen.tsx:255-256`, `PreviewPanel.tsx:115-118`, `bulk.ts`, `RivalSidebar.tsx` | Explore filter popovers/select; CompVideo | V39 / 5 | **Sửa** chip popover, bulk mở cùng dialog với selection; bỏ Vì sao, không enqueue vòng ngoài bypass dialog |
| `screens/explore/mobile/MExploreScreen.tsx:152-153`, `MVideoScreen.tsx:131-145,311-365` | MExplore/MCompVideo; list/sheet/used | V39 contract / 5; V40 layout / 6 | **Sửa** bulk MExplore→Nhập nhanh. **Giữ ngoại lệ thiết kế MCompVideo**: sheet kênh thêm thẳng bằng default, Vừa thêm theo kênh; link phụ MNew để đổi template/thêm link |
| `screens/queue/QueueScreen.tsx`, `MobileQueue.tsx`, `queueView.ts`; `api/clone.ts` | Queue drawer=rules/state; MQueue | V39 / 5 | **Sửa** chỉ workers + nhịp kênh; mọi manual request đầu hàng, auto cuối; giữ pause/row operations đang cần |
| `screens/workbench/mobile/MobileWorkbenchDetail.tsx`, `MobilePlanSheet.tsx`, `MobileRecap.tsx`, `MobileImageSheet.tsx` | MMain/MVideo/MTop/MNotifications | V40 / 6 | **Xoá** recap/composer/plan; **sửa** two tabs/content default/Thông tin sheet/ảnh count. **Giữ không mở rộng MobileImageSheet** đợt45 |
| `screens/review/mobile/MReviewScreen.tsx`; shared imageView prompt parts | MReview viewer/compare/prompt/feedback | V40 / 6 | **Sửa** bỏ Vì sao/learned; chỉ 4 phần prompt; giữ quyền/view/history/sửa prompt thật |
| `screens/settings/SettingsScreen.tsx`; ProviderCard/data-key surface | SettingsConnect/MSettings/MSystem | V36 removal / 2; V40 / 6 | **Xoá** YouTube Data API card cả hai bố cục; **sửa** xác nhận provider logout, theme/nav dropdown trên mobile |
| `screens/editor/EditorScreen.tsx`; `editor/v13/Editor.tsx:26,73,300`, `Inspector.tsx`, `SceneImagePanel.tsx`, `scene-images.ts:37-45,193-229`; template/prompt entrypoints | Editor selected=image, Main viewer, Template peek/Library prompt | V36 wiring / 2; V37 workflow / 3 | **Nối đúng routed Editor v13**, reuse mapping/redraw/media swap/undo hiện có. Không nối nhầm editor/ui/inspector hoặc dựng lifecycle mới |

## Hợp đồng/backend liên quan
Xem [bản đồ backend](../backend-contracts.md): tên data-key thực `/api/youtube/data-key`; create route thực `/api/clone/enqueue`; thiếu `/api/jobs`; resolver template đang rotation; manual pick hiện FIFO; unique source key + owner lock đã có và cần giữ khi đổi front semantics.
Notification preferences phải migrate cùng PR3; collector đọc hiệu quả sau đăng và insight routes được retire cùng final FE consumer ở PR4, không chỉ ẩn key card. QA mutation phải tách HOME/identity/storage, không chỉ app DB.

## Những kết luận đã kiểm lại, không đưa thành câu hỏi giả
- Channel default tab đã được `Channel.dc.html:1149` chốt là sources; không hỏi người dùng chọn lại.
- `04-components.md:384-389` đã liệt kê đủ mode review/view/view-replace/sample; mode view/raw/edit trong hook là prompt, không phải mâu thuẫn thiết kế.
- `08-dev-prompt.md` V38 yêu cầu confirm bỏ liên kết; giữ confirm, không xoá theo một suy diễn scout.
- Query `RenameTemplate` cho đổi tên template dùng chung; guard chỉ-kênh-này ở `NameChannelStandardTemplates` là bảo vệ auto-naming, không phải quyền RenameTemplate. Không bỏ guard vì thấy MT/QH cùng template.
- Tiếng Việt áp dụng nhãn/copy/feedback; tiêu đề video nguồn và prompt nguyên văn không bị dịch giả (`06-copy.md:321`).

## Chưa phải bằng chứng đã nghiệm thu
- Đã chụp **480 reference tweaks**, baseline **28 màn code thật** ở viewport artboard tương ứng, tạo ảnh đặt cạnh và smoke 3 control MReview. Xem [báo cáo baseline](pr-00/baseline.md). Không gọi baseline là target match hoặc full-control Playwright pass.
- Đã GET-only preflight production: 1 kênh, 2 template, 0 nhịp ngoài 1–20, 0 rotation rỗng/dangling, 0 notification exception, 0 queued/running. Chưa đọc snapshot DB hoặc migrate; xem [receipt và giới hạn](pr-00/legacy-preflight.json).
- Editor/template đã có owner nguồn như bảng; phải lấy references mới trước PR2/PR3, không tạo API/action giả khi data contract chưa cấp.
- Current preview API đã có `building` (`api/library.ts:30,129-140`); historical gap tại 10-gap-closure:326 không còn là bằng chứng thiếu API. PR0 kiểm runtime builder và seed completed QA video hợp lệ; chỉ block theo prerequisite thực sự thiếu, không thêm service/API thay thế.
- Inventory/QA đã có commit `d245db8c`; chưa tạo PR/merge/deploy hoặc tuyên bố V36–V40 hoàn tất. Các trạng thái chưa map/seed, preview hoàn tất, full click/motion audit và Safari thật vẫn pending.
