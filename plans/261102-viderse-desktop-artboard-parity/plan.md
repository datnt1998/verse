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

- Noverse server và worker native đã triển khai `541c088a`, public entry `index-BRE816z1.js`; Viderse đã triển khai `a61dc0d0`, public entry `index-DKpzOEm5.js`. Cả hai health/readiness 200; máy đọc remote `ready: true`.
- Sau đợt deploy chuẩn, đã ghi nhận truyện gốc `dit-dang-audio/01-truyen-moi` chạy 10/12 chương, khác baseline tạm dừng. Chưa xác định người khởi động; đã hỏi chủ tài khoản, không tự dừng hay tiếp tục truyện.
- Bản chốt `index-BWdKB7dj.js`: build/typecheck/size qua (entry 141,7 KiB, initial JS 191,3 KiB, CSS 33,8 KiB gzip). Ba lượt cold độc lập xấu nhất FCP 768 ms, TBT 26 ms, long task 76 ms, 25 request; CLS 0, không lỗi JavaScript.
- Bộ kiểm hiện tại: app 798 tests, UIKit 239 tests, editor scene 18 tests đều qua; cổng phiên 36 tests qua lại sau khi đổi boot; timeline façade 35 tests qua, gồm preview chỉ đọc không lưu và editor vẫn sửa bản lưu. Toàn bộ gói Go đã qua; PostgreSQL qua ở timeout 10 phút (269,6 giây). Regression lease và toàn gói preview cũng qua.
- Expanded acceptance: Vietnamese visible system copy and progress journal; Agent–User acknowledgement, pending/review/error feedback; 360/390/430 mobile and 1280/1440 desktop. Reuse Noverse interaction patterns without changing user content, provider names or API identifiers.
- Real isolated editor measurement now completes (−16.1 LUFS). The synthetic job correctly remains below the export threshold because it has no complete source/build manifest; no score is fabricated. Native testing exposed a false missing-media blocker: quality now resolves authoritative editor metadata pointers, and remeasurement confirms that blocker is gone.
- Đã đóng lỗi native: template detail/preview tự chờ lease; cue không sáng; motion bị giữ trong reduced-motion; landmark kênh mới; lý do provider hết hạn bị che; cold deep-link prompt; template trắng trang vì thiếu TooltipProvider; preview tự PUT project; popup lọc tràn mép; tên đối thủ dài bị cắt; hộp thoại phím tắt cao hơn viewport.
- Khám phá điện thoại trước sửa: nạp 1.000 video làm DOM lên 15.906 nút và heap 36,68 MiB. Sau virtualize, smoke API thật nạp 5.000 video: 10 thẻ đang gắn, 1.107 nút DOM, heap tăng 5,15 MiB, 60 fps; không sửa DOM/API trong harness.
- Native cuối: mobile 253/253 tại 360/390/430 và hai bộ inset; motion 23 mẫu gồm glide Segmented 320 ms và reduced-motion 0; PC Main/Review 46 hàng; Explore/Channel 239 kiểm tra; assets/settings 47 trạng thái, không PUT hay lỗi JavaScript. Editor 1440 đủ 18 trạng thái, 1280 qua đến export gate và kiểm riêng reduced-motion; dialog cuộn đến nút Đóng nằm trọn viewport. Hai marker prompt bị CSS uppercase là tiêu chí harness cũ, không phải lỗi giao diện.
- Hotfix avatar đã triển khai: smoke public với ảnh thật tại 360/390/430 xác nhận badge nhìn thấy ngoài cạnh trái/dưới và ảnh giữ bo góc 8 px; desktop 1280/1440 không bị che, không lỗi JavaScript hay API ghi. Dữ liệu production đã ở trạng thái “Xử lý” thay vì “Duyệt”; cùng hàng thumbnail đã được kiểm đúng trạng thái hiện tại.
- Smoke public PC và mobile: không tràn ngang hay lỗi JavaScript. Đúng một yêu cầu Director HTTP 200 trả `ok:false/unknown` cho đề nghị chỉ đọc không được hỗ trợ; UI hiện thông báo và gợi ý bằng tiếng Việt, pha chờ có animation 180/800/1.100/2.800 ms. Không thực thi kế hoạch, không sinh ảnh/giọng hay tiếp tục job. Không coi kết quả này là một kế hoạch thành công.
- Đợt sửa Cách đọc trước (`541c088a`) đã kiểm nhánh phân vai `dan`, lưu/đọc lại preset, đổi chế độ và huỷ lựa chọn; chưa kiểm lưu preset vào vai đặc biệt `single`. Báo lỗi mới xác nhận nhánh Một giọng kể vẫn lỗi. Danh mục thật hiện có 25 giọng nhưng `capabilities.preview=false` và không có sample URL, nên chưa đạt đầy đủ luồng nghe thử/chọn giọng.
- Deploy bị chặn bởi Docker hết chỗ; đã được chủ tài khoản duyệt dọn cache build cũ hơn 24 giờ, thu hồi 22,69 GB. Không xoá image, container, volume hay dữ liệu ứng dụng; deploy chính thức sau đó hoàn tất.
- Hotfix icon lịch/phễu trong chip lọc MExplore: trước sửa lệch lên 2,375 px do icon nằm inline trong prefix; chuyển sang slot `icon` của ChipSelect, không thêm offset CSS. Public 360/390/430/1280/1440 qua: độ lệch tâm icon 0 px, mở/chọn ngày và trạng thái, Escape trả focus, hit-test và chống tràn đều đúng; không lỗi JavaScript hay API ghi. Mười test MExplore, typecheck/build và size gate qua. Môi trường QA local chỉ kiểm xong ba kích thước mobile vì desktop gặp onboarding; nghiệm thu đủ năm kích thước dùng production thật, từng trang độc lập để tránh race remount khi đổi breakpoint.
- Giới hạn kiểm chứng: mô phỏng Chrome, chưa kiểm bàn phím Safari trên iPhone thật; dữ liệu canonical khác dữ liệu fixture. Không tuyên bố mọi trạng thái trả phí hay mọi thiết bị đã được kiểm.

## Audio follow-up — single, quick actions, preset previews

- **Single narration:** sửa backend PostgreSQL nhận đúng vai đặc biệt `single`, giữ lựa chọn sau tải lại, đếm và đọc lại audio cũ đúng phạm vi khi xác nhận; không tạo speaker giả.
- **Chapter quick actions:** đối chiếu MAudio/MVoiceGen hiện hành, đưa thao tác theo trạng thái chương ra vị trí có thể tìm thấy; thao tác mở menu không ghi hay tạo audio, cập nhật/tạo lại chỉ chọn đúng chương.
- **Built-in voices:** danh mục worker thật, nghe thử thật qua worker hosted, trạng thái chờ/dừng/lỗi và quyền truy cập đúng; không bật cờ hỗ trợ khi chưa có đường thực thi.
- Ba slice chỉnh mã độc lập; parent sở hữu tích hợp, schema dùng chung, docs, build/test sau freeze, smoke native và deploy server/worker. Không chạy thêm Director hay tạo lại chương của người dùng.
- Nghiệm thu trên fixture riêng: cả cast và single, lưu/hủy/đổi chế độ; quick actions ở trạng thái audio thiếu/mới/cũ/lỗi/hàng đợi; nghe thử preset thật và mẫu đã tải lên. Khung 360/390/430 và 1280/1440, focus/overlay/reduced motion, bảo toàn audio cũ khi chưa xác nhận. Chưa chốt hoàn thành chỉ vì danh sách giọng hiện ra.

### Bằng chứng bản đã triển khai

- Noverse gateway và máy đọc native cùng `3d874df0a31dcbba660bb911845d0211e9fbf1f1`, qua `~/server/bin/build-deploy.sh`; bundle live `index-BfTGoPDP.js`. Migration `00022_job_priority.sql` đã đi qua lượt deploy.
- Máy đọc live `ready`, `capabilities.preview=true`, danh mục thật 25 preset. Mẫu Hải Đăng trả HTTP 200, WAV mono PCM16/48 kHz, 222.764 byte, 2,32 giây. Nút nghe Adam trong UI dùng GET thật; CDP đọc HTMLAudio `duration=currentTime=2.72`, `ended=true`, `readyState=4`.
- Fixture riêng: gán dẫn truyện Hải Đăng; chọn single Adam trong UI, lưu thành công, huỷ chọn Mai Anh không phát sinh PUT, mở lại vẫn Adam. Đổi sang Phân vai trên PC lưu cả `mode`/`voiceMode=cast`, giữ dẫn truyện Hải Đăng và single Adam. Không có job tạo chương; fixture đã xoá và danh sách truyện gốc không đổi.
- Menu chương “Ca đêm lệch” đã có nghe/xuất riêng/chia sẻ/cập nhật/tạo lại; mở menu và huỷ xác nhận không ghi dữ liệu. Form xuất khoanh đúng chương 1. Khung mobile 360/390/430 và PC 1280/1440 không tràn ngang; nút `⋯` 44 px, nút đóng menu nằm trong viewport sau animation. Đã bật preference giảm chuyển động trong browser; chưa có kiểm chứng iPhone Safari vật lý hoặc bảng chia sẻ hệ điều hành.
- 210 test web audio và build/typecheck qua. Lượt race rộng trên snapshot đầu: các gói audio, PostgreSQL, worker, migration qua; các regression sau review về ưu tiên/chọn đúng attempt/thử lại failed và HTTP cũng qua, có bằng chứng lỗi trước sửa → qua sau sửa. Nhóm web `Audio|TTS|Tts|Preview|Worker|Single|Priority` trên snapshot cuối với race qua. **Không tuyên bố toàn suite xanh:** toàn gói web với race timeout 20 phút; lượt đầu dùng nhầm DB QA có public schema cũng gây lỗi fixture, đã tách DB rỗng và dọn DB kiểm thử riêng.
- Người dùng đã chọn **Cập nhật riêng chương 1**, cho phép AI soạn lại phần đổi và native TTS chỉ chương 1; không cho điều khiển hàng đợi viết. Đã bấm **Cập nhật chương này** thật, HTTP 200 và chỉ tạo một job `update` cho chương 1. Job về `state=error` với lỗi SQL23505 do các ID đoạn cũ đổi thứ tự đụng ràng buộc ordinal tức thời; không có lỗi kẹt trạng thái job (observer ban đầu nhận sai enum `err` thay vì `error`, đã sửa và dừng observer cũ).
- Vá `3cd876cfd12f56db4c5a636073b54d967b4f1384` đã triển khai thành công cho gateway và máy đọc native: migration `00023_audio_segment_order.sql` cho hoãn uniqueness, `SaveScriptBundleTx` kiểm tra thứ tự cuối trước khi trả kết quả. Bốn regression đổi chỗ/chèn/rollback duplicate/CAS cũ qua; nhóm race audio/preparation/PostgreSQL qua. CLI thật áp dụng toàn bộ 23 migration trên DB smoke rỗng; constraint thực tế deferrable, initially immediate.
- Đã bấm **Thử lại** thật đúng job chương 1 sau deploy, HTTP 200; job không còn báo SQL23505 nhưng dừng ở bước soạn vì OpenAI trả `usage_limit_reached` / HTTP 429. Chưa có audio mới, chưa tuyên bố đường MP3 chương 1 hết 404. Cấu hình global dùng `openai-codex/gpt-6.1-sol`, không có model dự phòng; xAI có tài khoản được đánh dấu kết nối nhưng chưa kiểm chứng hạn mức hoặc khả năng gọi hiện tại. Không tự đổi cấu hình AI dùng chung hay điều khiển hàng đợi viết; cần quyết định của chủ tài khoản trước khi đổi provider.
- Đã dừng server QA, xoá helper local/container/host và truyện QA. Tab kiểm tra retry được đóng khi kết thúc lần kiểm tra; không giữ Chrome test chờ hạn mức. Chưa kiểm chứng native clip tải lên; không suy diễn kết quả preset thành hỗ trợ giọng clone trên hosted.

Rollback: use the previous revision through the owning deployment scripts; preserve PostgreSQL and media, never reset user state or regenerate images.
