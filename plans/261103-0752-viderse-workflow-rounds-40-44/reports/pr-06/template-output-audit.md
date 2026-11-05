# Tái kiểm Template · Xuất & kênh — PC / mobile

**ReportStatus: DONE (audit nguồn, không phải nghiệm thu UI hay cho phép deploy).**

## Phạm vi và độ tin cậy

- Đúng URL người dùng chỉ định: `#/templates/99d15a24-0d7c-4455-90d0-1346e8250b7a?section=output`. Không thay bằng màn Editor, template khác hay trang tổng quan. Router primary nhận đúng `section=output` tại `apps/viderse/web/src/router.ts:37–38,98–99`; mobile render màn con tại `MTemplateScreen.tsx:223–240`.
- Tổng hợp hai lượt audit PC/mobile độc lập (`TemplateOutputDesktopAudit`, `TemplateOutputMobileAudit`), đối chiếu lại các nhánh chức năng, artboard và hợp đồng V38 bằng đọc nguồn.
- **P** = nguồn production cô lập, base `9d440603`, tại `/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/viderse-prod-measure-origin-atfx6ivo/source`. **C** = primary `/Users/dat/Projects/verse`. P mô tả code UI cũ, **không phải ảnh production đã đăng nhập**. C có sửa đang có trong source; không mặc nhiên là đã deploy/đạt pixel parity.
- **[S]** = bằng chứng nguồn/artboard; **[V?]** = hệ quả thị giác suy ra, chưa đo trên ảnh runtime. Không có authenticated production visual, click audit thực tế hay bằng chứng Safari từ công việc này. Không chạy build/test/lint/formatter; không gọi provider, không ghi template hoặc dữ liệu production, không sửa source/artboard. Chỉ tạo báo cáo này.
- **FullV40 và Safari #93: KEEP.** Báo cáo không miễn scope, không đóng gate và không tuyên bố full V40 release.

### Chỉ dẫn nguồn

Đường dẫn code dưới đây tương đối với `apps/viderse/web/src/` trong P hoặc C:

| Ký hiệu | Chủ sở hữu bằng chứng (link primary) |
|---|---|
| O | [screens/library/template/OutputSection.tsx](../../../../apps/viderse/web/src/screens/library/template/OutputSection.tsx) |
| T | [screens/library/TemplateScreen.tsx](../../../../apps/viderse/web/src/screens/library/TemplateScreen.tsx) |
| M | [screens/library/mobile/MTemplateScreen.tsx](../../../../apps/viderse/web/src/screens/library/mobile/MTemplateScreen.tsx) |
| PV / N | [PreviewPane.tsx](../../../../apps/viderse/web/src/screens/library/template/PreviewPane.tsx) / [TemplateNav.tsx](../../../../apps/viderse/web/src/screens/library/template/TemplateNav.tsx) |
| Draft / LT / MS | [useTemplateDraft.ts](../../../../apps/viderse/web/src/screens/library/template/useTemplateDraft.ts) / [libraryTokens.stylex.ts](../../../../apps/viderse/web/src/screens/library/libraryTokens.stylex.ts) / [mtemplate.stylex.ts](../../../../apps/viderse/web/src/screens/library/mobile/mtemplate.stylex.ts) |
| A / AM | [Template.dc.html](../../../../apps/viderse/docs/design/v1/artboards/Template.dc.html) / [MTemplate.dc.html](../../../../apps/viderse/docs/design/v1/artboards/MTemplate.dc.html) |
| Kit / Q&A | [05-kit.md](../../../../apps/viderse/docs/design/v1/05-kit.md) / [11-hoi-dap.md](../../../../apps/viderse/docs/design/v1/11-hoi-dap.md) |

Số dòng là mốc đọc của audit; C có thể đổi sau tích hợp. Với P, dùng cùng đường dẫn dưới root snapshot nêu trên.

## Kết luận ưu tiên: sai lệch cụ thể

Tất cả dòng sau là **[S]**. Mức độ clipping, xuống dòng, vị trí tuyệt đối trên màn hình là **[V?]**, cần ảnh đối chiếu mới kết luận.

| Ưu tiên / màn | Chuẩn thiết kế | Production cũ P | Primary C / phần còn thiếu | Sửa nhỏ nhất đúng scope |
|---|---|---|---|---|
| **P1 · PC + mobile: lưu giả thành công** | Autosave phải phản ánh kết quả thật; A:982; AM:923; `01-ia-flows.md:503`. | Draft:84–97 giữ `dirty=true` sau 409/400 field error, có thể `error=null`. T:179–182 vẫn chọn chữ “Đã lưu” từ `saving/error/justSaved`, dù màu có nhận conflict. M:159–164 luôn tick xanh, chỉ phân biệt saving; không có recovery lỗi/conflict ở output. | **Đã có sửa trong source**: T:109–110,217–218 ưu tiên conflict/dirty; M:175–196,235–241 có banner, retry/reload, field errors, live region và khoá lúc reload. | Tích hợp logic trạng thái thật; giữ draft và timestamp server; không hiển thị saved khi dirty/conflicted. Không bỏ 409 hoặc tự ghi đè để làm giao diện “xanh”. |
| **P1 · PC + mobile: rotation ≠ default** | Một default/kênh; “Kênh dùng làm mặc định”, “Đổi ở Kênh”; A:1226–1237; AM:923,997; Workflow:980. | O:34–42 gọi detach;98–104 hiển thị vị trí rotation + “Bỏ khỏi kênh”; `i18n/library.ts:544–550` dạy rotation. | C/O:79–100 đã chuyển sang default row + điều hướng channel/templates; C/M:163–165 nêu tên kênh khi khoá xoá. **Chỉ đúng khi chạy cùng hợp đồng V38**. | Không cherry-pick nhãn default lên backend rotation rồi gọi là đã đổi semantics. Tách hẳn cutover bên dưới. |
| **P1 · PC preview, mobile khi về overview: mất overlay với preview thật** | Overlay stale/building phủ bản xem thử; A:1288–1292,1645–1650. | P/PV:46–64 chỉ đặt overlay ở nhánh **không có projectId**; nhánh player thật bỏ qua overlay này. | **Chưa sửa**: C/PV:49–67 vẫn vậy; dòng71 chỉ thêm câu stale dưới player. | Một frame chung bọc player và overlay; giữ preview cũ bên dưới. Progress/ETA thật hoặc lời trạng thái trung thực, không dùng mẫu 40%/1 phút. Kiểm riêng mobile overview/player, không suy rằng output có preview inline. |
| **P1 · xoá: confirm no-op / đóng sheet khi lỗi** | Confirm nêu hậu quả và chờ thao tác thật; Workflow:977; mobile dùng sheet, không chồng lớp. | P/O:45–54 truyền `onConfirm: () => undefined`, đóng confirm rồi mới xoá. P/M:114–122 nuốt rejection, khiến ConfirmSheet coi lỗi là hoàn tất. | **Đã sửa logic**: C/O:39–47 truyền promise thật; C/M:144–149 để lỗi truyền lên. Shared `platform/ui/src/ConfirmDialog.tsx:22–32` đã có responsive sheet. | Dùng đúng promise và shared confirmation; pending lock, lỗi giữ hộp mở. Còn card xoá thừa trên mobile, xem dòng layout dưới. |
| **P1 · mobile overlay** | Rename/chọn bổ sung dùng bottom sheet: AM:923; `01-ia-flows.md:282,285`; `08-dev-prompt.md:1125`. | M:181–197 dùng Dialog cho rename + AttachChannelDialog. | **Chưa sửa presentation**: C/M:199–219 vẫn Dialog; `AttachChannelDialog.tsx:20–43` vẫn centered Dialog. | Mobile-owned sheet bằng kit hiện có; giữ desktop caller. Có loading/error/empty thật cho danh sách kênh, không coi danh sách chưa tải là rỗng. |
| **P1 fidelity · PC khung** | A:953,977,993–995,1273: rail trái236, padding14×10; header14/24/12; main20/28/32, inner≤700; rail phải372/padding18/divider, scroll nội dung tách header. | N:93 trái232/padding22/14/22/20. T:263 grid gap24 + outer padding28; LT:46 preview280–340, không rail độc lập. | C/T:310–312 + LT:45–46 đã có372, gap0, inner700 và rail padding/divider. C/N:93 vẫn232; T main vẫn cuộn cả header/body. | Giữ cấu trúc C đã đúng, sửa rail trái và ownership scroll; không chỉ đổi token preview rồi gọi full parity. |
| **P1 fidelity · PC fields** | A:125–131,1211–1267: field phẳng, gap10, padding18px dọc, divider dưới trừ cuối; h2 18/600. | O:185–189 dùng gap12, h2 16, card padding14/border/r12. | C/O:160–164 bỏ khung, h2 18 nhưng dùng gap24; vẫn thiếu field padding18/divider. | Style desktop field đúng nhịp; không bê card/grid desktop sang mobile. |
| **P2 · compression PC/mobile** | PC A:1215–1220: 2 cột bằng nhau/gap10,12×14/r12/border1.5,gap trong4. Mobile AM:923:2 cột/gap8,10×12,label12.5,size17,note11.5/1.4. | PC O:200 auto-fit min200/gap8 + RadioCard mặc định có dot; mobile compact metrics gần chuẩn. | C/O:142–145 ẩn dot, dùng12×14 cho cả hai;175 vẫn auto-fit desktop/gap8;176 gap2;178 mobile size18. | Responsive override tại O: desktop fixed2/gap10, mobile giữ10×12/size17. Sửa tone selected/inactive cục bộ, không thay kit toàn cục. |
| **P2 · typography / control mobile** | AM:423–431 track50×30/knob24;754–758 title16; Kit:134–137 touch≥44. | Default Switch36×22 nhưng hitbox đã48×44; MS:9–15 title14. Dock preview sm30; các action phụ chỉ≥44 khi pointer coarse. | C vẫn default Switch và title14; dock/rename/file actions đã tăng hitbox. C/O:173,189 vẫn coarse-only cho một số action kênh/xoá. | Mobile variant track/knob qua kit props; mọi action mobile≥44 cả fine-pointer. Không gắn lỗi “hitbox switch nhỏ”: hitbox đã đủ. |
| **P2 · channel rows** | PC A:1230–1233 gap12,padding8×10,r10,avatar30,name13.5/500; mobile AM:923 avatar34,row padding8×0, tên14.5. | O:206–211 avatar22,row5×8/r7, tên không link. | C/O:183–189 vẫn metrics cũ; avatar34 PC/**40 mobile**, không phải30/34. Đã có “Đổi ở Kênh”. | Dùng danh tính/ảnh thật, link tên và action tới đúng kênh; sửa metric theo breakpoint. Không bịa số video/ETA/tint để giống fixture. |
| **P2 · Bố cục / xoá / file mobile** | A:1241–1249 action cạnh heading +5 icon chips;1262–1265 notice xoá trung tính ngang. AM:923 chỉ4section, không mở editor trên điện thoại; xoá ở More; file2cột/gap8. | P/O:112–118 đặt editor dưới chips và vẫn mở trên mobile;133–143 thêm danger card ở cả mobile; file flex-wrap/gap10. | C/O:103–111 sửa heading/action desktop và ẩn mobile editor;123–134 vẫn danger card thừa;190–193 text-only pill + flex gap10; file44 và chia chỗ đã có. | Giữ mobile helper “vị trí, cỡ chữ **và lớp**…trên máy tính”; bỏ inline danger card mobile, dùng More. PC notice trung tính; chips icon theo kit; mobile file grid2/gap8. |
| **P2 · PC header/nav/preview chi tiết** | A:957–970 selected icon/outline + health ở đáy;982 tick+12.5;983 edit icon;1273–1312 preview rail,title14,dot status,frame16:9/r12,source full-width,card hiệu lực riêng. | N:62,70,86,97–98 thiếu selected icon/outline/bottom anchor/check icons; PV:124–146 preview boxed,title13,frame r9,source hàng ngang; menu T:188–190 chỉ export/duplicate. | C thêm menu library/delete (T:224–227), header density/preview rail/title14/ResponsiveSelect; saved vẫn12/no tick, editor no icon; N và badge/frame/note còn khác. | Hoàn thiện local styles; đừng sửa shared theme. Chỉ hiện health thật từ contract, không dùng ngưỡng “ít clip” tự nghĩ. |
| **P2 · export/duplicate/terms** | A:1253–1258 và Workflow:977: action có feedback; tệp nêu contents/exclusions đúng thật. | P/O:124 download anchor không pending/result; T duplicate không catch; M duplicate **đã có** catch/toast/navigation103–111, không được báo toàn bộ bị unwired. | C/O:30–36,117 + C/T handlers + C/M:135–142 đã có result/error. Duplicate output chưa local pending lock. Nhãn “mẫu video/Biên tập” và mô tả export vẫn lệch artboard. | Tích hợp feedback thật, kiểm double tap. Chốt vocabulary một lần qua Q&A nếu thay; chứng minh contents archive trước khi hứa đúng exclusions. |

**Token hierarchy PC còn lệch [S]:** A:128–131,1218–1220 dùng help12.5/1.55 muted `#8B8B97`, note12/1.5. C/O:163,179 dùng fg3 và caption11.5/fg4; LT:5–15 xác nhận các cỡ này. Không đổi palette global chỉ để sửa một screen.

## Ranh giới sửa an toàn / cutover mặc định

1. **Sửa giao diện cục bộ, không migrate dữ liệu:** spacing, rails, typography, icons, overlay preview thật, mobile sheet/hitbox, vị trí action, feedback lưu/xoá/export. Những thay đổi chức năng feedback vẫn phải giữ promise/API và concurrency thật, không chỉ thay chữ.
2. **Default template là cutover V38 đã có hợp đồng chấp nhận, không phải quyền mới từ yêu cầu tái audit URL.** Xem [phase V38](../../phase-05-v38-channels-and-library.md) (dòng9) và [backend-contracts](../../backend-contracts.md) (dòng15–34). FE+BE+schema phải cùng revision; không đưa default labels lên P rotation và giả đã có default semantics.
3. Chủ backend thực hiện đúng `backend-contracts.md:23–34,50–53`: preflight read-only; maintenance được chấp thuận; quiesce mọi writer + xử lý paid attempts; DB backup/restore proof; map rotation đầu theo position/id (không cursor); giữ template/media/job snapshot; lock/read default+recipe/persist snapshot trong admission transaction. Empty/dangling rotation hoặc perDay>20 là stop, không clamp/fake. Sau mở write không tự restore snapshot cũ.
4. Primary đã có hướng cutover; trạng thái merge ghi ở phase plan không thay cho chứng cứ production đã migrate. Báo cáo này **không chạy/cho phép maintenance, migration, deploy hay ghi template thật**; cũng không thu nhỏ acceptance V38/V40 để coi xong.

### Những điều không được tự suy diễn thành sửa

- Compression đọc `recipe.output.compressExport` (P/O:31; C/O:29), switch và radio cùng sửa boolean thật. `compress=true`/`zip=true` của artboard (A:1400; AM:928) là demo, không phải quyền ghi đè template99d… .
- PC A:1217 là comparison div; mobile AM:923 là clickable radio. Quyết định có giữ PC radio hay chỉ switch cần thống nhất qua Q&A; geometry có thể sửa độc lập, không bỏ keyboard behavior âm thầm.
- “Đặt làm mặc định cho kênh…” artboard mở Kênh; cả hai source mở chooser rồi điều hướng (`AttachChannelDialog.tsx:20–30`). Không bịa kênh đích hoặc thêm set-default mutation vào Output; chốt chooser/direct route nếu cần đổi flow.
- Running counts/ETA, pool ít clip và thumbnail phải có dữ liệu thật. A:1641–1643 chỉ minh hoạ pool8; không tạo threshold dựa trên số kênh. Thiếu contract thì ghi Q&A, không hardcode.
- Kit:3,35,67,116 ưu tiên shared controls: không thu Switch desktop xuống34×20 của markup cũ, không quay chip về26/r7; chip kit hiện28/r8. Mobile track50×30 là visual variant, không lý do viết một switch mới.
- Mobile đúng là overview4hàng → màn con, **không cần4tab desktop**. Main scroll + dock ngoài scroll đã có trong `platform/ui/src/mobile/page.tsx:76–100,168–177`; safe-area/keyboard runtime chưa kiểm. Reduced-motion baseline đã có theo audit mobile, không kết luận thiếu motion support chỉ vì không có ảnh.
- Q&A #38:263–270 cho phép missing-preview/locked reason trung thực; #48:331–334 chốt name-only không làm stale preview. Không ép fake preview hoặc làm stale khi chỉ rename để giống ảnh mẫu.

## Tweak matrix và trạng thái output

| Màn | Giá trị chính thức cần đối chiếu | Cách đọc đúng |
|---|---|---|
| PC1440×900 | A:1396: `section=output`; `preview=fresh/stale/building`; `fewClips=false/true`; `picker=none/add/choose/video`. | **6 core output** =3preview×2health khi picker=none. Đầy đủ Cartesian output là24;18 trường hợp còn lại là backdrop-picker overlay composites, không phải18 form output mới (modal ngoài section conditional ở A:1317; picker đọc toàn cục1567). |
| Mobile390×844 | AM:926: `screen=output`; `preview=fresh/stale/building`; `sheet=none/more`. | **6 output combinations**. Preview nằm overview; trên output trạng thái thể hiện qua dock/summary, không tự thêm player. Full MTemplate30combos là phạm vi khác, không được gọi6output là full mobile pass. |
| Ngoài tweak | Compression on/off; saving/dirty/400/network/409; retry/reload; rename; export/duplicate pending/error/success; delete locked/free/confirm; menu/source picker; play/pause; normal/reduced motion. | Đây là interaction/data states, không bịa thêm enum trong artboard. `none` preview là trạng thái app thật, cần lời giải thích trung thực. |

Fresh: xanh “Khớp cài đặt”, phát được, rebuild phụ. Stale: vàng “Cần dựng lại”, old-preview overlay, rebuild primary. Building: xanh lam “Đang dựng”, progress thật, cancel, source disabled (A:1645–1650). Dock AM:923 có câu stale cố định trong demo; **không copy nguyên câu đó cho mọi runtime state**.

## Trình tự triển khai / ownership đề xuất — chưa thực hiện

Mỗi file chỉ một writer; shared O không giao đồng thời PC/mobile. Không sửa artboard hoặc dữ liệu production để làm ảnh giống nhau.

| Bước / owner duy nhất | File được sở hữu | Điều kiện giao tiếp / acceptance trước tích hợp |
|---|---|---|
| 1 · Integrator | Quyết định patchset/revision; không sửa thay owner đang làm | Chọn rõ nhánh visual-only hay bản V38 đầy đủ. Đọc lại C để không làm mất pending fixes/user changes. Giữ editor revision transport và Settings ngoài scope O/M/PV. |
| 2 · TemplateSharedUI | O + template output copy trong `i18n/library.ts` | Sửa responsive metrics, chips, file layout, mobile visibility, confirm promise. Sở hữu cả breakpoint PC/mobile; không đổi backend/API hay global kit. Chốt vocabulary/chooser trước phần cần quyết định. |
| 3 · DesktopTemplateShell | T + N + PV + token template trong LT | Rails/header/scroll/status, health placement, common preview overlay; chỉ token template, không sửa O hoặc global theme. Giữ real preview/progress và reload/concurrency guards. |
| 4 · MobileTemplateShell | M + MS + mobile-only chooser presentation | Sheet rename/chọn kênh, save recovery/dock/target44. Không sửa O. Nếu phải đổi `AttachChannelDialog.tsx`, file được chuyển độc quyền cho owner này trong bước đó; giữ desktop caller hoặc dùng kit responsive pattern đã duyệt. Không sửa shared Dialog/ConfirmDialog tràn scope. |
| 5 · V38 backend owner, **chỉ khi cutover được thực hiện theo hợp đồng** | Domain/channels/autonomy/admission/repository/query/migration/API+CLI consumers được inventory tại backend-contracts và phase V38 | Một integration owner cho FE+BE+schema, migrate mọi caller, không shim/default-label-only. Không sửa protected `models.go` bằng tay. Maintenance/backup/restore và provider prerequisites là gate thật. |
| 6 · Main/QA sau khi tất cả patch đã land | Acceptance artifacts, không sửa file owner giữa lúc kiểm | Một lượt check/build/test/visual theo plan; không worker nào chạy giữa chừng. Audit hiện tại không thực hiện bước này. |

## Screenshot + click acceptance chính xác — đề xuất, chưa chạy

**Môi trường:** chỉ private QA tách DB/identity/HOME/browser/media; dữ liệu phản ánh template99d… qua sao chép được phép. Production exact route chỉ đọc. Mutation test dùng bản QA/disposable, không nhấn lưu/xoá/duplicate/set-default template production; không gọi AI/provider chỉ để tạo trạng thái. Không dùng DOM mock để chứng minh hành vi backend thật.

1. **Baseline đúng route:**100% zoom, font loaded; ghi revision P/C, provenance dữ liệu, viewport/pointer/motion/safe-area. Chụp PC1440×900 và mobile390×844 ở đầu + cuối output. Dùng tên `pc-output-{fresh|stale|building}-{ready|few}-top/bottom` (6state) và `m-output-{fresh|stale|building}-{none|more}-top/bottom` (6state); với More lấy ảnh overlay đầy đủ. Mỗi state lưu reference / app / comparison cùng props. Các ảnh này chưa tồn tại từ audit này.
2. **Geometry PC:** đo trái236/padding14×10; header14/24/12,title19; inner≤700; rail phải372/padding18/divider; main20/28/32. Scroll center/preview không kéo header đi; không overflow ngang. Fields18px/divider/gap10, compression2cột/gap10; status/source/effect card/chips đúng kit. Tách metric token đã biết khỏi sai lệch pixel thực đo.
3. **Geometry mobile:** header52/title16/inset16, section gap18; compression2cột/gap8,padding10×12,size17; avatar34; file2cột/gap8; không desktop editor/inline danger card/global tabs. Chỉ content scroll; dock/header vẫn thấy, action cuối không bị che. Rename mở keyboard, safe-bottom và narrow fine-pointer vẫn dùng được; đo mọi hit rectangle≥44×44.
4. **Lưu compression trên QA:** switch off/on + từng radio (Space/arrows); switch/selected card/sidebar hoặc summary đồng bộ. Chờ server save thật, reload cả PC/mobile cùng QA template, boolean đúng. Delay/400 field+general/network/409 có ảnh riêng: không saved/tick xanh khi dirty/error/conflict; draft giữ; retry thật; reload có hậu quả rõ và không ghi đè tự động. Name-only không stale preview.
5. **Preview thật:** có `projectId` và stale/building vẫn thấy overlay; play/pause hoạt động khi fresh; source list nổi PC / sheet mobile overview, chọn đúng record; building khoá source và cancel phản ánh server. Không source thì focus/locked reason đúng, không ảnh/ETA/progress giả. Không phát request provider trong audit read-only; active build/cancel chỉ trong QA được phép và không sinh side effect ngoài scope.
6. **Điều hướng:** “Đổi ở Kênh” và tên kênh tới **đúng ID**, tab Template, không mutation. Zero/one/multiple channels wrap tốt. Editor top và heading PC tới cùng template project; unsaved/reloading có reason. Mobile không mở desktop editor; Back/Xem thử về overview đúng ID.
7. **Overlay/actions:** More→rename mở sheet, cancel/Esc/scrim không ghi; focus trả đúng trigger, không overlay chồng. Export tải archive thật + feedback, lỗi không toast success; kiểm contents trước khi chấp nhận copy. Duplicate trên QA mở copy thật; double tap không tạo nhầm nhiều bản. Source/menu nổi không đẩy layout.
8. **Xoá:** template đang dùng khoá kèm tên kênh và bước đổi default đúng semantics backend, zero mutation. Disposable QA template: cancel/Esc zero mutation; confirm pending khoá, lỗi giữ hộp/sheet mở và retry được, chỉ success thật mới đóng/điều hướng. Kiểm cả output PC và More mobile.
9. **Normal/reduced motion:** đại diện switch/radio/More/rename/confirm/toast; reduced motion không spring/slide/press nhưng checked/focus/pending/error còn rõ. Đối chiếu keyboard, URL bar, rotate và dock trên **Safari iPhone thật** theo gate riêng; Chromium emulation và source proof không thay thế bằng chứng đó.

**Kết luận bàn giao:** source chứng minh production UI cũ khác thiết kế đáng kể và có lỗi phản hồi lưu/confirmation/preview cụ thể; primary đã sửa một phần nhưng chưa đạt đầy đủ fidelity. Ưu tiên chức năng P1 rồi responsive metrics; default semantics chỉ đi cùng V38 backend cutover đúng hợp đồng. Không có actual template writes, runtime visual pass, Safari pass hay quyền release mới từ báo cáo này.
