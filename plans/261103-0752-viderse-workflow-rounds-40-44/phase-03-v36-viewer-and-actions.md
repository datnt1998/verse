---
phase: 3
title: "Bước 2 · V36 Trình xem ảnh và nối nút"
status: in-progress
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
| Sửa nguyên nhân lỗi gặp khi smoke API thật | `apps/viderse/internal/media/media.go`, `apps/viderse/internal/persistence/postgres/` (`blobs.go`, `clone.go`, `clone_cutout.go`, `clone_editor.go`), `apps/viderse/internal/pipeline/service.go` | Xoá job trong transaction không vi phạm FK editor, không làm mất media của project khác; cancel bỏ pausedAt cùng status; không thêm API/schema |
| Sửa đọc project / lưu mẫu hiện hữu | `apps/viderse/internal/editor/projects.go`, `apps/viderse/internal/persistence/postgres/cover_trials.go` và regression PostgreSQL tương ứng | Canonical head đổi trong autosave không gây404 giả; giữ404 thật khi head không đổi/bị xoá; INSERT mẫu đủ22placeholder |
| Sửa kit dùng chung | `platform/ui/src/Dialog.tsx` | Lớp centring không nuốt bấm scrim; bấm nội dung không đóng; kiểm consumer Viderse/Noverse |
| Sửa kit dùng chung / mobile sample | `platform/ui/src/Sheet.tsx`, `screens/library/PromptEditor.tsx`, `screens/library/template/CoverSection.tsx` | Theo #81 đã được người dùng chọn A: preview ngay cùng bề mặt, Quay lại giữ draft/focus; trap Tab bỏ qua controls ẩn, không thêm viewer mobile45 |
| Sửa parity phiên bản | `apps/viderse/web/src/screens/review/mobile/MReviewScreen.tsx` và test hiện hữu | Mobile dùng cùng `/images/:id/use` như desktop; không vô tình approve/kick dựng; single-flight, lỗi giữ preview/không rời Duyệt. Lịch sử gồm bản2 AI thật kiểm4 viewport/motion projects |
| Sửa cold graph không đổi UI | `platform/ui/src/confirm-store.ts`, `ConfirmDialog.tsx`, `index.ts`, `apps/viderse/web/src/screens/RouteScreen.tsx` | State/session confirm không kéo toàn bộ dialog/shared controls vào boot; lazy component lúc gọi confirm vốn async, giữ race/session cancellation; source build/size và consumer checks |
| Sửa nguồn tách nền theo #82 đã duyệt | `apps/viderse/internal/pipeline/steps/images/handler.go`, `handler_test.go` | Tách nền từ WebP chuẩn hoá thay vì PNG native. Derivative QA đã lưu được sửa bằng giao dịch CAS/media publication từ chính ảnh hiện có, không sửa ảnh gốc/không thêm lượt AI; validation dựng giữ nguyên |
| Sửa đường action bị mất ở chất lượng | `apps/viderse/web/src/screens/workbench/workbench-logic.ts`, `workbench-logic.test.ts` | Worker nhả activeStep về null sau quality_gate; UI suy từ lỗi score thật để giữ “Vẫn xuất”. Lỗi thiếu công cụ/provider vẫn là sự cố, không được biến thành bỏ qua kiểm định |
| Sửa route Review giữ-alive | `screens/RouteScreen.tsx`, `screens/review/ReviewScreen.tsx`, `screens/review/mobile/MReviewScreen.tsx`, `shell/MobileShell.tsx` và consumer tests | Truyền route pane làm nguồn sự thật; tab Duyệt giữ địa chỉ video/ảnh, video hết chờ duyệt phản hồi và về target hợp lệ, không tác động nhầm video |
| Hoàn tất xác nhận đăng xuất nền dùng chung | `api/session.tsx`, `api/session.test.tsx`, `App.test.tsx`, `screens/auth/AccessDeniedScreen.test.tsx` | Xác nhận rõ thoát cả hai app trước revoke; Huỷ giữ phiên, lỗi giữ xác nhận để thử lại; không đổi Promise<void> của caller |
| Sửa pause khi thay thế generation | `internal/persistence/postgres/queries/clone.sql`, `gen/clone.sql.go`, `clone_editor_delete_pg_test.go` | Supersede atomically xoá paused_at để generation cũ có thể xoá; sqlc1.31.1 sinh lại output, PG lỗi trước/qua sau |
| Tách lỗi vẽ khỏi từ chối nội dung | `screens/review/useReview.ts`, `ReviewScreen.test.tsx`, `mobile/MReviewScreen.tsx`, `MReviewScreen.test.tsx` | `failed` chung không phải `blocked/refused`; quota không bị thẻ “Bộ lọc nội dung” che. Mobile đọc số bản drawing từ `images` đã có trong cùng response, không thêm GET/backend field; không cộng thêm1 hoặc lặp tile đang vẽ |
| Giữ toast dài không che action | `platform/ui/src/Toast.tsx`, `tests/viderse-workflow/viewer-audit.pw.ts` | Overflow/ellipsis đúng kit.css, giữ full accessible text; native geometry check ở cả motion/viewport, không đổi4s lifetime |
| Sửa nền prompt theo #84 đã chọn A | `internal/clone/images/types.go`, `versions.go`, `service.go`, tests; `internal/pipeline/steps/images/model.go`, `draw.go`, tests; `web/src/api/images.ts`, `review/imageView.ts`, `ImageRail.tsx` và các producer Review/Editor/Main/mobile | `baseVersion` rõ ràng cho một ảnh, giữ phần không sửa/refs qua restart; không repurpose `Flight.BaseVersion` race guard; guard live-flight, limit theo provider-card của video, cancelled history readonly, không tự dùng ảnh khác hoặc gửi sai upload ID |

## Thực hiện
1. Re-scout imports và entrypoints ảnh; lập typed context `review | view | view-replace | sample`, phân biệt với prompt mode `view | raw | edit`. ID job/image/version là nguồn sự thật, không index dễ trôi khi polling.
1a. Mở rộng route/paths cho địa chỉ ảnh `img` theo ID ổn định, qua parseHash/routePath và App canonicalization; truyền vào review/viewer selection sau tải ảnh. Click ảnh không phải ảnh đầu, direct link, reload, notification entry, Back/Esc phải quay đúng ảnh/context; ID mất thì phản hồi và chọn fallback hợp lệ, không mutation nhầm ảnh đầu.
2. Viewer tối đa1240×800, ảnh contain không upscale quá natural size, strip đủ ảnh; placeholder đang/chưa vẽ không bấm được, ←/→ bỏ qua. Khi ảnh bị xoá/job đổi, đóng hoặc chọn ảnh hợp lệ có phản hồi, không thao tác nhầm ảnh.
3. Duyệt: mở đúng ảnh từ Bàn dựng→Duyệt và giữ đúng job; versions/prompt/refs/feedback/edit/drop/approve thật. View: ảnh sẵn sàng/đang làm, Góp ý ở cột phải; confirm báo tốn số ảnh + hạn mức thật, kết quả “Đã gửi Hoạ sĩ · bản 2”. View-replace: thêm thay ảnh cảnh bằng command/undo hiện có; sample: ảnh prompt thật, không versions giả, Vẽ thử nối action đang có.
4. Khung đóng bằng ×/Esc/scrim, focus về trigger; trap focus theo overlay kit. Shortcuts viewer không xuyên nền hoặc chạy khi nhập prompt. Callers đóng viewer trước khi hỏi xác nhận, chỉ một lớp modal nhận tương tác; Huỷ khôi phục đúng ảnh/focus, không lồng dialog xác nhận trong dialog khác.
5. Nối `Vẽ lại tất cả` trong Review với confirmCount/expectedRevision thật; không chỉ có nút ở Dialogs/demo. Mọi ảnh mẫu/preview entrypoint có cùng viewer, không mở các popup độc lập còn sót.
6. Bỏ YouTube Data API card và route `/api/youtube/data-key` thực tế; không xoá cả performance.go nếu còn API hợp lệ. Deferred removals learned-rules/suggestions/assistant chỉ xoá cùng final FE consumer ở PR3/PR4 để PR2 không để nút hỏng.
7. Structured prompt mới bỏ learned khi wire viewer; raw prompt cũ là provenance bất biến. Backend chỉ bổ sung contract/action mà V36 cho phép, reuse image APIs; không dựng provider preview giả.
8. Nối thao tác xoá video tới HTTP DELETE có sẵn, không gửi `jobAction('delete')` qua POST. Smoke QA thật ở PR nền đã giữ đúng form pending/lỗi nhưng máy chủ trả “Thao tác làm video không hợp lệ.”; readback job vẫn200. Owner: `screens/workbench/jobActions.tsx`, `api/workbench.ts` đối chiếu `internal/web/clone.go` (DELETE đã có, action switch không nhận delete). Di trú mọi caller và regression consumer/API thật; không thêm backend route hay tính năng ngoài gói. Nghiệm thu: confirm pending chặn đóng/nhấn đôi, thành công readback404, lỗi vẫn giữ form, toast kết quả đúng4s.

## Ma trận test và acceptance
| Mức | Scenario | Qua khi |
|---|---|---|
| Critical | Mở cùng image từ mọi entrypoint, đổi job/polling trong lúc xem | Đúng ảnh/version/quyền; không gửi sửa nhầm target |
| Critical | Redraw/drop/approve/replace, double-click, quota/network error | Đúng mutation một lần, consequence thật, feedback và readback; undo giữ timeline/keyframes |
| High | View/review/view-replace/sample + strip placeholders | Đủ mode thật, không controls dead, mẫu không hiện history giả |
| High | Esc/scrim/focus/R/E/A/arrows/textarea | Chỉ đúng lớp/ảnh tác động; focus restore, không background shortcut |
| High | Click ảnh thứ3, direct img link/reload/Back/Esc và ảnh bị xoá | Canonicalization giữ img đúng, selection theo ID, trở về đúng context; không reset mặc định ảnh0 |
| Medium | Natural size, ảnh load lỗi/portrait, 1440 và layout hẹp | Không upscale/clipping/overflow, lỗi có đường tiếp tục |

Focused commands (từ root repo): `npm test -w apps/viderse/web -- screens/review screens/workbench screens/library`; `npm run typecheck -w apps/viderse/web`; backend `go -C apps/viderse test ./internal/web ./internal/media ./internal/pipeline/... ./internal/editor ./internal/persistence/postgres` với PostgreSQL QA được cô lập và `REQUIRE_POSTGRES_TEST=1`. Browser audit theo verification.md; không lấy test forwarding làm proof.
Mỗi tweak relevant có code/reference/comparison hai viewport và ledger; mobile viewer45 ghi excluded, MReview/data-key mobile vẫn phải kiểm. Cập nhật dòng V36 gap bằng commit thật và mọi lệch còn lại.

## Rủi ro / security / rollback
Preview URL phải qua auth và data owner như hiện tại, không phục vụ path/URL tuỳ ý. Kết quả ảnh có thật và quota có thật; nếu thiếu binding scene→job/media không được đoán bằng filename, ghi câu hỏi trước khi thay contract ngoài package.
Viewer chung dùng cùng mutation/cache owner, không mở subscription trùng. Revert PR trước PR3; giữ render history/data, không huỷ job người dùng.

## Bằng chứng thực thi hiện tại
- [Báo cáo V36](reports/pr-02/viewer-and-actions.md) giữ số test, screenshot, control ledger và các gate chưa đạt; không coi báo cáo audit cũ là pass của nguồn mới.
- Đã chứng minh HTTP DELETE/readback404 của video thường và paused→stop→delete; PG kiểm rollback/shared media. UI sau xoá và thay/Undo đã qua lượt kiểm tra tập trung cuối.
- Lỗi race GET project trong cửa sổ autosave được sửa bằng xác minh identity canonical head; chỉ đọc lại khi head thực sự chuyển, không lặp404 thật hoặc che lỗi khác. PG interleaving và smoke PUT/GET thật có receipt.
- **Design #81 đã chốt A:** Prompt mobile2/2 và Cover desktop/mobile4/4 qua; preview ngay cùng bề mặt, Quay lại giữ lựa chọn/draft/focus. Lượt28 bổ sung Esc mobile Cover và Huỷ/Esc/scrim desktop xác nhận, khôi phục đúng ảnh và không gửi mutation. Không thêm viewer mobile45.
- Hai gate cold phone≤200KiB và độ trễ Khám phá giữ mở tới trước deploy cuối theo #83 đã được người dùng duyệt; không giảm ngưỡng hoặc giấu thất bại. Các gate UI/chức năng V36 vẫn bắt buộc trước merge. iPhone/Safari thật chưa có bằng chứng.
- Người dùng cho phép **tối đa2 ảnh QA thật** qua kết nối provider hiện có. QA chỉ nhận access token còn hạn, không sao chép refresh token, không logout/ghi kho production; tắt fallback để giới hạn lượt. Không tự tăng hạn mức nếu lỗi.
- Đã dùng đủ **2/2 ảnh QA thật**: một bản cảnh1920×1080 WebP và một ảnh trial bìa PNG native1672×941. Không được gọi thêm ảnh. Hai bản lịch sử kiểm4/4 desktop/mobile normal/reduced; trial bìa ready và preview4/4. Credential QA đã gỡ; không nhận refresh, kho production không đổi.
- Source tree `8406d47d0414fc174f20bfaee3e3d15077323180` qua typecheck từ archive riêng; build production qua. Entry141,7/150KiB và CSS34,7/40KiB qua; cold phone202,7/200KiB còn mở theo #83.
- #82 đã vá nguồn tách nền và sửa đúng derivative QA qua CAS từ ảnh đã có, giữ originals và không thêm AI. Job thực sự hoàn tất; HTTP200 MP4630653byte, ffprobe H.2641920×1080 +AAC,6giây. Không phải migration chung cho dữ liệu production.
- `jobStage` nhận cả score/export quality_gate, không biến tool/provider/codec lỗi thành quality. Focused Review/mobile/quality28/28 qua; native “Vẫn xuất”202 có readback succeeded và MP4 decode. PostgreSQL hiện hành10/10 qua gồm supersede paused generation.
- Lượt28 route/target/cancel14qua10skip có tên; lượt30 binary nhúng logoutCancel4/4 qua, giữ `/api/me` và0logout. Full tại source844 trước #84: Viderse3723qua/4skip, Noverse1894/1894, kit261/261; lịch sử thất bại giữ nguyên. Full Viderse sau #84 là3755qua/4skip, không tái gán receipt cũ cho nguồn mới.
- Source `ee94b6f3` và followup đã kiểm `84458549b6250b8098026dbda5e14e74d6015781` đã đẩy nhánh `refactor/viderse-v36-viewer`, PR nháp [#3](https://github.com/datnt1998/verse/pull/3) mở; chưa merge. Source fingerprints, toàn bộ generated dist và evidence gọn được đưa cùng followup, không stage artboard/user changes.
- Template nhúng lượt54 **16/16 qua** gồm trial bìa đã có,0AI; Review nhúng lượt55 có24hàng captured/2N/A gồm raw/draft/ref/cutout/batch/confirm và motion. Settings lượt52 **14qua/2skip mobile recheck có owner PR6**; provider thật lượt51 **8/8**, toast không che CTA. Source/replay guard và credential cleanup qua.
- **#84A đã tích hợp:** source `52d3871225e4a6c8dd0799c0e5d151692463d600`, giữ đúng bản đang xem qua `baseVersion`/checkpoint/worker. [Source hashes và gates](reports/pr-02/selected-source-integration.json), [review](reports/pr-02/selected-source-review.md). Full3755qua/4skip Canvas được ghi tên, typechecks/build qua; native61 bốn context và HTTP62 sáu ca từ chối qua,0provider. Không tự dùng bản, không làm viewer45. PR2 còn ledger/tweak acceptance trước merge.
- **Sidebar Cài đặt:** source `1108e3932d877efe5297505a61d175f4b97a764b` sửa cột icon/nhãn/trailing và hàng Luồng chạy; [bằng chứng hình học](reports/pr-02/sidebar-layout-repair.json). Lượt87 kiểm16/16 desktop/mobile normal/reduced, gồm Tab/Enter, viền focus và bảo toàn nguyên bản các kết nối có sẵn; không suy ra mobile manual-model đã hoàn tất.
- **Ảnh dọc trong viewer:** source `d65b0b93a7fb6ab11e6a62b638321babb7751888` sửa contain theo kích thước stage. [Receipt](reports/pr-02/portrait-containment-repair.json) giữ lỗi trước sửa (ảnh cao1024 trong stage574,28) và lượt88 qua2/2: ảnh287,14×574,28, không cắt/không upscale. Fixture ảnh tự tạo được publish bằng media API thật rồi gỡ; không thêm lượt AI. Generic Editor lượt86 thay ảnh/Undo2/2 và Settings lượt87 không thay thế bằng chứng clone history hoặc các gap PR6.
- **Team sửa hai lỗi cuối đã tìm thấy:** source `a704a2a236cc6af8a526c4b824dc954958b6d25f` gắn tick theo ID và giữ đầy đủ nhãn bộ chọn tham chiếu. [Receipt](reports/pr-02/review-team-integration.json): full3756qua/4skip, typecheck/build/native build qua; native92 có50capture và kiểm approval identity/label geometry/keyboard/slider≤2px, raw/draft/refs/cutout/batch/confirm; source/replay giữ nguyên,0provider. Template93 qua2/2. Comparison `Review.prompt.edit` đã dùng draft thật, rail từ đầu. Đang review34comparison và bổ sung ledger control, chưa đóng checklist hoặc merge.
- **Model/bàn phím/tham chiếu:** source `098742e1af49505838bce0eea73979e1c8a7ccd9`, full3764qua/4skip và build nhúng qua; Review37/37 có hai hồi quy lỗi-trước/qua-sau cho thêm→gỡ và thêm trùng. Native1072/2 kiểm draft trả về0thay đổi/0API; native1088/8 Review/Settings ở cả viewport/motion, không ghi ngoài quyền. Native109 UseVersion HTTP200 + inverse có job/steps/intents/artifacts/bindings khôi phục, source/project gốc giữ nguyên và image_receipts0→0. Lượt111 tiếp tục bắt race địa chỉ sau duyệt; focused50/50 qua sau sửa cả desktop/mobile nhưng full/build/native xác nhận còn chạy, chưa đánh dấu phase đạt.
- **Nghiệm thu nguồn `11f1cd36`:** full3766qua/4skip, typechecks/build nhúng qua; native138model autosave2/2, Review14052capture/50observation, safe controls1418/8 PC/mobile vàrefs2/2, Review actions14212/12, Main dùng bản1452/2 vàviewer1462/2. [Receipt](reports/pr-02/integrated-acceptance.json), [xử lý đối chiếu](reports/pr-02/visual-review-disposition.md). Người dùng đã chốt **#85A giữ thẻ nguồn vẽ đầy đủ**. Không gọi0dead toàn app, không gọi provider hoàn tất từ queue/cancel, không triển khai mobile45; fallback thành công cần credential nguồn thứ hai. PR3–PR6 và hai release gate#83 còn mở.


## Checklist
- [x] Chốt references, typed contexts và mọi desktop image entrypoint.
- [x] Hoàn thành viewer/actions/confirmation và bỏ data-key cả hai bố cục.
- [x] Qua focused tests + build/typecheck + actual UI/state/tweak gates trong phạm vi V36; giới hạn và owner các phần sau được ghi rõ trong report.
- [ ] Gửi comparison/control/motion report, gap V36; merge PR2 trước PR3.

## Thực hiện bổ sung #84 · đúng bản đang xem

1. Contract `baseVersion?:number` chỉ áp một ảnh. Không có trường: giữ chính sách nền riêng hiện hành của normal/batch; có trường: không tự rơi về bản đang dùng nếu sai. Kiểm positive/existence/state/single-image trước ghi, chỉ dùng failed-version nếu có prompt snapshot hợp lệ.
2. Backend giữ nền được chọn trong draft/checkpoint; phần không sửa và refs (kể cả rỗng) theo nền. Refs chỉnh sửa phải cập nhật mô tả tham chiếu; wrapper an toàn/công cụ vẫn dựng cho provider hiện hành. Không đổi ý nghĩa `Flight.BaseVersion` kiểm người dùng đổi ảnh trong lúc vẽ.
3. Frontend mọi producer single-image Review edit/feedback, mobile hiện hữu, Editor và Main viewer gửi đúng nền đã chọn. Draft lấy text/refs đang thấy; Cancel/error giữ draft; chọn bản/SSE trong lúc xác nhận không đổi payload đã chốt. Không đưa raw wrapper vào editable scene, không thêm mobile viewer45 hoặc làm sớm UI4phần V40.
4. Regression có used1/selected2 khác nhau: giữ nguyên style/avoid/refs của2, chỉ scene đã sửa thay; explicit refs[] không tự thêm refs1. Backend serialize/reload/đổi used version; invalid request giữ revision/manifest. Native UI chụp trước/sau edit/ref và request target đúng; không gửi thêm ảnh AI quá2/2.
5. Hai slice chỉ sửa file thuộc phạm vi đã chia, không chạy build/test/formatter giữa chừng; controller tích hợp rồi focused → broader + compiled native smoke, review và cập nhật gap bằng commit thật. Không merge chỉ vì user đã chọn A.
6. Rà soát độc lập đã thêm các boundary bắt buộc: provider-card override không rơi về chuỗi chung; failed row có flight còn sống vẫn conflict; hướng cũ dính Scene không gửi lại; refs rỗng không còn mô tả cũ. HTTP thật lượt58 đã kiểm400/404/409/422 cùng no-mutation/readback. Frontend phải giữ raw upload media ID, bỏ cancelled khỏi nguồn mặc định nhưng giữ history, Grok lấy lần lỗi cuối qua baseVersion và không gửi edit trong lúc drawing/busy. Chạy lại focused/full affected/build/native sau tích hợp cuối; không gán lượt3736 hoặc native56/57 thành pass cho các sửa frontend sau đó.
7. Kết quả nguồn cuối: refs upload identity, dirty-source guard, fresh refs/no-base, cancelled readonly và async upload target fencing đã tích hợp; Review30/mobile11/LiveStudio12/Workbench5/editor25 qua. Native61 dùng cancelled history không prompt và vẫn đọc đủ rich data: producer phải trả `parts:[]`, regression đã lỗi trước/qua sau. Go checks/build dùng overlay HEAD duy nhất cho model generated ngoài scope của người dùng. Phone budget hiện203.1/200KiB FAIL, không nới gate; cần giảm thật trước deploy cuối.

