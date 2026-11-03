---
phase: 4
title: "Bước 3 · V37 Bàn dựng gọn"
status: pending
priority: P1
dependencies: [3]
---

# Bước 3 · V37 — PR riêng

## Mục tiêu và nguồn
Main/Activity/Dialogs/System dùng chung Bàn dựng gọn. Bỏ Bản tin, ô Nhắn trợ lý, thẻ Kế hoạch, chip Ảnh, Vì sao và Lưu thành quy tắc. Đề xuất chỉ còn3loại thành dòng Chờ bạn; inspector Nội dung đăng/Thông tin, default Thông tin, có Tiến độ. Nguồn V37, Main `state`/`inspector`/`viewer`, Workflow và 01 W6/W8/W9.
Không thêm chatbot, không thêm realtime service. [Kiểm kê](reports/inventory.md), [backend](backend-contracts.md), [gate](verification.md).

## File inventory và reuse
| Hành động | File tuyệt đối / phạm vi | Test impact |
|---|---|---|
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/workbench/WorkbenchDesktop.tsx`, `LiveStudio.tsx`, `panels/WorkbenchPanels.tsx` và panels hiện hữu | 4tab agents/content/source/backdrop → content/info; giữ actions/cache/image watcher |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/shell/AppShell.tsx`, `MainNav.tsx`, `AppHeader.tsx`, `MobileShell.tsx`, `ActivityPopover.tsx`; WorkbenchDesktop inline list/`src/components/agents/`; i18n workbench | Main shell làm owner; không tạo WorkbenchRail/MoreSheet/MobileBar thay source hiện có |
| Xoá khi hết caller | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/workbench/recap/`; composer/plan cards trong `src/components/assistant/`; explain/learn popover ở workbench | Xoá tests tính năng đã bỏ, giữ test tool action còn dùng |
| Sửa/xoá theo reference | `/Users/dat/Projects/verse/apps/viderse/internal/web/recap.go`, `assistant.go`, `learn.go`; `internal/assistant`, `learn`, `suggest`; prompt/override paths liên quan | Remove only obsolete plan/recap/learn and suggestion kinds, giữ API hành động còn sống |
| Sửa | Viewer PR2; `/Users/dat/Projects/verse/apps/viderse/web/src/editor/v13/{Editor,Inspector,SceneImagePanel}.tsx`, `scene-images.ts`, v13 sidebar/transport/timeline; `src/screens/library/template/{PreviewPane,CoverSection,ImagesSection}.tsx` | Routed v13, reuse existing mapping/media-swap/undo; sidebar dọc và mọi controls editor/template nối thật |
| Sửa cùng cutover | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/settings/NotificationSettings.tsx`, `components/learn/SuggestionSettingsRow.tsx`, `api/activity.ts`; `/Users/dat/Projects/verse/apps/viderse/internal/activity/settings.go`, producers/gating | Bỏ recapOnReturn, off[] public → một suggestions bool; stored prefs/strict decoders migrate, unrelated prefs giữ nguyên |

## Thực hiện
1. Re-scout đúng owners bằng references; kiểm route subscriptions trước xoá recap/plan/explain. Không xoá toàn bộ assistant service chỉ vì composer biến mất; các nút cần action thật vẫn gọi existing operation.
2. Main chuyển inspector thành hai tab; default info khi vào, không reset tab người dùng đang chọn do SSE/poll refresh. Nội dung đăng gom cover/title/description/tags, sao chép và thêm/bỏ tag thật; Thông tin gom bốn trợ lý/nguồn/template/video nền và số đang có.
3. VideoList là các dòng bấm được đổi đồng bộ cả hai panel theo job; Chọn nhiều hiện checkbox và sticky “Tải gói N video”, chỉ job/tệp hợp lệ, tải ZIP thật đúng hai video đã chọn. Tiến độ gộp 2 dòng nhật ký + “Nhật ký · N bước”; đang dựng có progress thật, sẵn sàng có điểm/tệp, video không bộ ảnh có summary, ảnh thành phẩm16:9, footer Template mặc định. Không tạo phần trăm giả.
4. Bỏ recap auto-open, composer, plan và các lý do/learn UI, chip Ảnh và override ảnh/prompt **cấp video** đã bỏ. Không xoá feedback/sửa prompt **cấp ảnh** hay raw prompt history mà V36/V40 vẫn cần.
5. Đề xuất: map3dòng thực ở Main (video nổi, template cần sửa, dọn tệp) tới current enum; kiểm source before deleting other kinds. Hiện cùng nhóm Chờ bạn, có thao tác thật/undo nếu đảo được; không popup/thẻ kế hoạch mới.
6. Xoá backend recap/plan-generation, learned rules và kinds bị bỏ khi last consumer được migrate. Prune image/prompt video-level exceptions chỉ khi còn là phần feature đã cắt; giữ image intent per-scene, export và job actions. Migrate mobile consumers tối thiểu cùng PR để không import/API lỗi; final layout mobile thuộc PR6.
7. Reuse viewer PR2 cho ảnh/editor/sample; cross-fade phiên bản mới có motion token và reduced-motion. Activity/Dialogs/System chỉ là trạng thái cùng shell, không ba page clone.
8. Editor giữ thanh trái dọc; nối undo/redo, popover Phím tắt, về đầu/phát/tới cuối, mute, ±0,1s, Nghe lại, tách/xoá, zoom timeline và ẩn từng track. Đối chiếu 15 controls theo artboard; không chỉ viewer pass là xong Editor.
9. Template: hai ô chọn video là floating select (dựng từ video đã làm và video vẽ thử bìa); Độ mạnh/Tốc độ kéo thay dữ liệu thật; Chọn ảnh/Đặt lại/Bỏ hiệu ứng/Thêm vào kênh/Bỏ khỏi kênh có feedback. PR3 giữ hành vi attach đang có tới PR4 cutover default, không tạo rotation mới.
10. Việc nền popover và mọi nút bên trong, NotificationSettings switches phải hoạt động. Xoá flag Bản tin; Đề xuất của trợ lý một flag cho3loại. Dialog shortcuts bỏ / Nhắn trợ lý/Gửi tin nhắn, N Làm video nối ở PR5. Loading/disconnected/retry details/empty shell theo Main mới; chưa có video hiện Bốn trợ lý, Template mặc định và ẩn badge Duyệt khi0.
11. Notification contract cùng PR: xoá recapOnReturn, thay per-kind/per-channel off[] bằng một bool và migrate state/gating. Live3kinds đồng nhất bật hoặc tắt thì giữ trạng thái đó; mixed/per-channel không thể bảo toàn → preflight dừng, ghi hỏi11-hoi-dap trước khi chọn policy. Không prune enum rồi để stored off[] khiến save validate lỗi, không tự bật lại prefs người dùng.

## Test matrix / verification
| Mức | Scenario | Qua khi |
|---|---|---|
| Critical | Job running→review→ready/attention và reconnect | Cùng job, không toast/progress xong giả, event không duplicate, controls đúng state |
| Critical | 3suggestion kinds accept/dismiss/undo/error | Thay đổi thật đúng target, không mở old plan/composer |
| High | Main mọi state + info/content, data refresh/route change | Đúng default và tab, layout3cột, không reset lựa chọn do polling |
| High | Scene replace/undo + template/sample viewer | Asset/history/transform/keyframe giữ đúng, không copy viewer/prompt logic |
| High | Chọn2video sẵn sàng→tải gói; đổi dòng video | ZIP đúng tệp hai video, hai panel đồng bộ theo ID, không tải cả kho |
| High | Editor15controls và Template sliders/selects/reset/attach | Data/undo/playback/readback thật, popup không đẩy nội dung, sidebar dọc |
| High | Notification switches, background popover, disconnected/empty | Flag mới lưu qua reload, retry có trạng thái, không old recap/plan/0badge |
| High | Legacy notification prefs all-on/all-off/removed-kind/mixed | Contract decoder/save/readback đúng; unrelated prefs giữ; mixed stop có câu hỏi, không tự reset |
| Medium | Live paint + finite transitions + reduced motion | Agent feedback/nhật ký Việt, không shimmer/orb mắc kẹt |

Focused `npm test -w apps/viderse/web -- screens/workbench screens/review components/assistant`; `go test ./apps/viderse/internal/assistant ./apps/viderse/internal/suggest ./apps/viderse/internal/web` nếu còn package sau prune; bỏ tên package đã xoá thay vì giữ file rỗng để test. Typecheck/build và browser tất cả tweaks theo verification.md. Source/API/CLI imports phải compile sau cutover.
PR báo cáo đầy đủ Main/Activity/Dialogs/System và viewer contexts, screenshots/comparison/interaction/motion; dòng V37 gap ghi actual commit, không tuyên bố mobile45.

## Rủi ro, bảo mật và quay lui
Xoá enum/type cũ có thể để history không đọc được: migrate open proposals, render history hợp lệ không làm API panic, không đánh mất tệp/posted state. Không log prompt/credential nhạy cảm vào trace công khai.
Rollback code trước PR4; nếu có migration dữ liệu, áp dụng cửa sổ quiesced và giới hạn restore trong backend-contracts.md. Sau resume writes chỉ forward-fix/recovery được duyệt, không restore backup cũ làm mất receipt/job.

## Checklist
- [ ] Re-scout consumers, loại bỏ UI và backend obsolete nguyên tử.
- [ ] Main/two tabs/progress/3proposal rows theo board, mọi action nối thật.
- [ ] Kiểm mọi state/tweak, motion và các cross-screen viewer contexts.
- [ ] Gửi report/gap V37; merge PR3 trước channel migration.
