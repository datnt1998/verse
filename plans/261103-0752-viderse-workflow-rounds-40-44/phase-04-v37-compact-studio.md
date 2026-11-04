---
phase: 4
title: "Bước 3 · V37 Bàn dựng gọn"
status: in-progress
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

### Preflight trước triển khai
- 04/11/2026: team đã map các owner FE/backend và consumer mobile tối thiểu; chưa sửa source V37, chưa mở PR3.
- [Preflight production chỉ đọc](reports/v37-notification-preflight.json) bằng role `viderse_app`, `default_transaction_read_only=on` và `BEGIN READ ONLY`: không có `notifications.preferences`, bảng suggestions không có dòng. Trạng thái hiện quan sát được là mặc định bật, không có mixed/per-channel cần chọn policy. Migration vẫn phải kiểm lại trong transaction và dừng nếu dữ liệu thay đổi; không dùng receipt này để ghi đè lựa chọn phát sinh sau đó.
- Footer giữ contract attach hiện tại tới PR4 như bước9; không tự đặt template mặc định khi channel còn dữ liệu nhiều template. Bảng/quy tắc cũ đã bỏ UI có thể giữ lịch sử lạnh nếu không còn consumer/route; không thêm API deprecated hoặc `learned` vào prompt mới.

### Thực thi và bằng chứng hiện tại · 04/11/2026
- Source: `5d316eabc0875a71ad380a55a7e582ab17d14cd6`, sửa migration `b493792d689095a20e01992c40c76462348e0882`, giữ video khi đổi phạm vi `93545e71cb6d3357a3147385c25db0e862995d53`. Đã mở [PR #4 dạng nháp](https://github.com/datnt1998/verse/pull/4), chưa merge/deploy. Normal git index của người dùng không đổi; không gồm `gen/models.go` hoặc artboard.
- Full frontend **3703 qua / 4 bỏ qua**, full Go và `go vet` qua. Sáu ca PostgreSQL thật qua, gồm nâng từ schema37 bằng migration-role, giữ preference/history và chặn mở lại loại đề xuất đã bỏ.
- Native desktop + reduced-motion: [ZIP/phát–dừng–tua thật](reports/pr-03/runs/012-reviewed-player-and-zip/owned-ready-studio-restoration.json) **2/2**, [Editor/Template/Main/Việc nền/Thông báo](reports/pr-03/runs/013-reviewed-editor-feedback/interaction-results.json) **10/10**, [Main viewer](reports/pr-03/runs/014-canonical-inspector-viewer-regression/interaction-results.json) **2/2**. Replay dùng media đã lưu, không phải hai lượt render/generate mới; job/project gốc và receipt được bảo vệ.
- Main giữ tab khi nối SSE lại và khi ẩn/hiện bảng; đổi video trở về **Thông tin** theo `Main.dc.html:1341`, không giữ tab trái artboard. Nguồn xuất Editor có thể không có artefact MP4; đọc `output.data` và phát file thật. Nút phím tắt có focus/Tab/Escape/click-toggle thật.
- Điện thoại + reduced-motion: [phần dùng chung](reports/pr-03/runs/016-settled-phone-interactions/interaction-results.json) **6/6** (Template, thông báo và chi tiết Main).
- Main matrix cuối [run034](reports/pr-03/runs/034-canonical-coverage-tokens/owned-main-states-restoration.json) **14/14**: 10 tĩnh +4 đang làm, desktop/reduced-motion, không skip/flaky. Pause/resume, metadata, chọn phạm vi/video, Viết lại không gửi AI, journal, download, offline/Thử lại/nối lại và nền thiếu/phủ kín qua. Clip công khai được phép nhập qua API thật, giải mã và dọn sạch; source/project/receipt không đổi. Thanh phủ token/pill chặn100%, source `d615299e2352718ea51ee8c7976f7b4565286c4b`; frontend3703 qua/4 bỏ qua, focused15/15 và build/typecheck qua.
- Bỏ qua cả ba loại đề xuất qua API thật ở chế độ thường/giảm chuyển động: run040 xác nhận sáu dòng biến mất qua reload, cooldown168giờ; dữ liệu đầu vào được đánh dấu QA, không nhận là bằng chứng producer. Dọn sáu dòng và giữ nguyên dữ liệu gốc. Không tính round45, Safari/iPhone thật hoặc gate release#83 đã qua; Actions bị tắt, không tuyên bố CI xanh.

## Test matrix / verification
| Mức | Scenario | Qua khi |
|---|---|---|
| Critical | Job running→review→ready/attention và reconnect | Cùng job, không toast/progress xong giả, event không duplicate, controls đúng state |
| Critical | 3suggestion kinds accept/dismiss/error; undo ở thao tác có thể hoàn tác | Thay đổi thật đúng target, không mở old plan/composer hoặc hứa hoàn tác việc không gỡ được |
| High | Main mọi state + info/content, data refresh/route change | Đúng default và tab, layout3cột, không reset lựa chọn do polling |
| High | Scene replace/undo + template/sample viewer | Asset/history/transform/keyframe giữ đúng, không copy viewer/prompt logic |
| High | Chọn2video sẵn sàng→tải gói; đổi dòng video | ZIP đúng tệp hai video, hai panel đồng bộ theo ID, không tải cả kho |
| High | Editor15controls và Template sliders/selects/reset/attach | Data/undo/playback/readback thật, popup không đẩy nội dung, sidebar dọc |
| High | Notification switches, background popover, disconnected/empty | Flag mới lưu qua reload, retry có trạng thái, không old recap/plan/0badge |
| High | Legacy notification prefs all-on/all-off/removed-kind/mixed | Decoder/save/readback giữ preference khác; mixed/theo kênh dừng theo backend-contracts.md, không tự bật lại hoặc chọn thay chủ dữ liệu |
| Medium | Live paint + finite transitions + reduced motion | Agent feedback/nhật ký Việt, không shimmer/orb mắc kẹt |

Focused `npm test -w apps/viderse/web -- screens/workbench screens/review components/assistant`; `go test ./apps/viderse/internal/assistant ./apps/viderse/internal/suggest ./apps/viderse/internal/web` nếu còn package sau prune; bỏ tên package đã xoá thay vì giữ file rỗng để test. Typecheck/build và browser tất cả tweaks theo verification.md. Source/API/CLI imports phải compile sau cutover.
PR báo cáo đầy đủ Main/Activity/Dialogs/System và viewer contexts, screenshots/comparison/interaction/motion; dòng V37 gap ghi actual commit, không tuyên bố mobile45.

## Rủi ro, bảo mật và quay lui
Xoá enum/type cũ có thể để history không đọc được: migrate open proposals, render history hợp lệ không làm API panic, không đánh mất tệp/posted state. Không log prompt/credential nhạy cảm vào trace công khai.
Rollback code trước PR4; nếu có migration dữ liệu, áp dụng cửa sổ quiesced và giới hạn restore trong backend-contracts.md. Sau resume writes chỉ forward-fix/recovery được duyệt, không restore backup cũ làm mất receipt/job.

## Checklist
- [x] Re-scout consumers, loại bỏ UI và backend obsolete nguyên tử.
- [x] Main/two tabs/progress/3proposal rows theo board, mọi action nối thật.
- [x] Kiểm các state/tweak và motion trong phạm vi V37, cross-screen viewer; giới hạn bằng chứng ghi rõ trong report.
- [ ] Gửi report/gap V37; merge PR3 trước channel migration.
