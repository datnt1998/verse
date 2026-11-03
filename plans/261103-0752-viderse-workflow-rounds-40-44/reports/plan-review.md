# Rà soát kế hoạch · 2026-11-03

## Trạng thái
Kế hoạch và kiểm kê đã soạn, chưa sửa code sản phẩm/DB, chưa tạo PR/commit/deploy. Bốn reviewer độc lập đọc plan và xác minh source; không agent nào chạy test/build/lint hoặc thao tác production. Các sửa sau review là sửa kế hoạch theo bằng chứng, không thay quyết định sản phẩm người dùng đã chốt.

## Red Team Review
**10 finding: 1 Critical, 8 High, 1 Medium; nhận10, bác0.** Nhóm các dòng dưới đúng finding của reviewer; các sửa wording/links không được tính thêm finding. Phát hiện về preview building là consistency delta từ kiểm chứng source, không blocker mới.

| Lens / vai xác minh | Finding / mức | Bằng chứng source | Disposition và nơi sửa |
|---|---|---|---|
| Scope / Contract verifier | Notification preference contract chưa migrate — High | `web/src/api/activity.ts:163-171,198-200`; `internal/activity/settings.go:19-38,66-73,113-127`; V37 `08-dev-prompt.md:1026-1028,1044` | Nhận: PR3 FE/BE/storage/producers cùng bỏ recapOnReturn/off[]→bool, giữ prefs khác, mixed/per-channel preflight stop |
| Scope / Contract verifier | Chỉ bỏ key chưa retire collector — High | `cmd/viderse/clone_wiring.go:313-314`; `performance_wiring.go:25-75`; `internal/performance/service.go:387-450`; `internal/web/performance.go:52-63` | Nhận: PR4 final consumer cutover retire Run/read/apply; giữ MarkPosted/export metadata/shared source pacer |
| Assumptions / Scope auditor | Nối nhầm Editor cũ, mất reuse scene lifecycle — High | `screens/editor/EditorScreen.tsx:5`; `editor/v13/Editor.tsx:26,73,300`; `scene-images.ts:37-45,193-229`; `Inspector.tsx:221-231` | Nhận: PR2/PR3 đổi inventory sang v13 và reuse clip mapping/version/swap/undo, không tạo lifecycle khác |
| Assumptions / Scope auditor | img route bị canonicalization strip — High | `router.ts:7-8,77-79,129-134`; `App.tsx:39-43`; `ReviewScreen.tsx:53`; `imageView.ts:100-103` | Nhận: PR2 typed address/parse/serialize/canonicalize + selection-ID/deep-link/reload/Back regression |
| Assumptions / Scope auditor | Ép MCompVideo đi Nhập nhanh trái thiết kế — High | `01-ia-flows.md:290`; `MVideoScreen.tsx:131-145,311-365` | Nhận: PR5/PR6 direct channel-sheet + Vừa thêm giữ; secondary MNew; dùng chung jobs contract, không đổi design |
| Failure / Flow tracer | Snapshot boundary và AutoPick migration sai/mơ hồ — High | `clone_wiring.go:229-235`; `pipeline/service.go:374-380`; `postgres/clone_jobs.go:31-54`; `queue/autopick.go:62-65`; `domain/channels.go:76-80`; `queries/clone.sql:37-40` | Nhận: snapshot default/recipe cùng admission transaction/lock với default update; job đã persist bất biến; autoFetch=AutoPick AND effective pick, boundary race/truth-table |
| Failure / Flow tracer | Global front và duplicate target chưa đủ trên capped queue — High | `queries/queue.sql:12-16`; `queue/service.go:272-288,362-367,743-747`; `schedule.go:68-75,191-210,345-347`; `restart.go:19-45`; `domain/clone-request.go:57-61`; `workbench_projection.go:56-67` | Nhận: PR5 global ordering trước cap, bounds full-set/mọi writer, >2000/3channel regression; duplicate DTO queueItemId + actual mapping→job |
| Failure / Flow tracer | Restore snapshot sau release có thể mất job/receipt và thu phí kép — Critical | `postgres/image_draws.go:16-31,176-197`; `imagegen/service.go:145-177,320-351,420-424`; `web/clone_images.go:247-260`; `cmd/viderse/clone_wiring.go:128-154` | Nhận: maintenance phê duyệt, quiesce/reconcile trước backup xuyên read-only acceptance; sau resume không auto restore, forward-fix/recovery riêng giữ receipts |
| Security / Fact checker | QA DB/account không tách provider credential machine-scope — High | `cmd/viderse/wire_ai.go:376-382`; `core/auth/store.go:81-101`; `bootstrap/bootstrap.go:16-29`; `aiconn/login.go:255-276`; `aiconn/auth.go:162-167` | Nhận: PR0 isolation receipt app+identity DSN/HOME/config/auth/media/blob/browser, trước mọi write/logout/draw; chưa tách thì block |
| Security / Fact checker | Source map có file phantom/recap sai thư mục — Medium | `shell/MobileShell.tsx:31-145`; `AppShell.tsx:4-6,28-30`; `AppHeader.tsx:36`; `workbench/recap/Recap.tsx:4`; `WorkbenchDesktop.tsx:20,33,305-306`; `api/library.ts:30,129-140` | Nhận: dùng actual AppHeader/MobileShell/ActivityPopover/workbench/recap; không tạo shell mới. Preview contract building đã có, bỏ historical-gap assumption |

## Validation Log
### Quyết định được xác nhận bằng repo / brief
Không hỏi người dùng chọn lại các điều đã có nguồn: channel default=sources; POST/api/jobs theo brief; 4viewer contexts có trong kit; raw prompt/source title giữ ngôn ngữ thật; V40 mobile exception theo §6.2; default fixture không overwrite production.
Không tự đặt policy legacy: rotation rỗng/perDay>20/mixed notification prefs hoặc timing logout khác design là stop condition tại preflight và phải hỏi qua `11-hoi-dap.md` nếu thực sự gặp. Hiện chưa query production nên không bịa số câu hỏi hay nói đã có dữ liệu lỗi.

### Verification results và giới hạn
- Tier: Full — Fact Checker, Flow Tracer, Scope Auditor, Contract Verifier qua4reviewer; không tuyên bố mọi consumer đã được enumerate vĩnh viễn, từng PR phải references lại.
- `ak plan validate <plan-dir> --json` sau final sweep: `valid: true`, `errors: null`.
- `go list` thực tế thành công cho9package dự kiến web/channels/autonomy/queue/postgres/images/aiconn/assistant/suggest; đây là xác minh đường test, **không phải test pass hoặc build pass**.
- Manifest scripts đã đọc ở root, Viderse web, platform/ui; SQLC config đã đọc, query owner sửa lại đúng `postgres/queries/library.sql`.
- Source/read checks đã xác minh các điểm chịu lực: kit RO/fonts/overlays/toast; channel tabs/default; actual create/data-key routes; v13 scene lifecycle; image route; MComp exception; building preview; notification preference format; collector lifecycle; queue capped fetch; AutoPick gate; snapshot outside-admission; credential-home scope. Không bịa con số15claims×7phase đã đạt.
- Browser **reference Main** ở1440×900 đã mở: font loaded, overflow=false; chưa chạy code UI/full tweak comparisons/Playwright click audit. Đây là kế hoạch, không phải nghiệm thu tính năng.
- Đã đóng tab/browser test và dừng static reference server8041; không tạo worktree hoặc chạm Chrome user.

### Whole-Plan Consistency Sweep
Decision deltas:10finding ở trên + preview-building historical-gap correction + bổ sung Editor15controls/Template/ZIP/notification/empty states từ checklist V37. Rà plan.md,7phase,verification,backend-contracts,inventory sau sửa; tìm stale phantom source owners, old inspector, queue snapshot/enqueue-admission ambiguity, MComp forced-dialog, off[] legacy, performance collector và unrestricted restore. Các tên cũ chỉ được giữ khi nói rõ obsolete/negative assertion/nguồn cần migrate.
Đã đọc lại12tài liệu cuối; kiểm31local Markdown links,0missing;7phase pending có checklist và dependency chain1→2→3→4→5→6→7 không chu kỳ; index58dòng, không còn scaffold/phase filename cũ. Sweep deltas không còn mâu thuẫn chưa giải quyết. Task hydration7execution items đã map phase files, đều blocked chờ lựa chọn triển khai và PR trước merge, không tự chạy code. CLI journal validate trả ok; server8041 không còn listener, browser đã đóng.

## Handoff
Bảy PR vẫn pending; bắt đầu PR0 sau người dùng chọn triển khai. Không tự auto-cook/auto-deploy. Triển khai theo plan file, tái tạo task view nếu phiên mới, giữ blocked gates thay vì bỏ acceptance khó.
