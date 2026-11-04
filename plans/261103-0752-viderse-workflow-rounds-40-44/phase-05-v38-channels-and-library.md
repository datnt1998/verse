---
phase: 5
title: "Bước 4 · V38 Kênh, Thư viện, Template"
status: pending
priority: P1
dependencies: [4]
---

# Bước 4 · V38 — PR riêng, FE/BE/schema cùng cutover

## Mục tiêu và nguồn
Channel chỉ3phần Nguồn & tự động / Template mặc định / Hồ sơ, default sources theo footer. Thư viện ở top-nav, mỗi channel một default template. Automation còn autoFetch, autoApprove ≥85, perDay1–20; bỏ autonomy/priority/learned/performance/rotation. Nguồn V38, Channel/Library/Template/CreateChannel/Onboard; mobile contract tương ứng MChannel/MLibrary/MTemplate/MCreate phải vẫn hoạt động sau PR.
[Backend contract/migration chi tiết](backend-contracts.md), [inventory](reports/inventory.md), [gate](verification.md).

**Gate thiết kế #86 (04/11/2026):** giới hạn1–20 trong phase này là dự kiến theo V38/Channel, chưa phải quyền đổi màn tạo đang cho1–50 (`CreateChannel.dc.html:1271`, `MCreate.dc.html:1081`). Chờ chốt tại `11-hoi-dap.md` #86 trước triển khai giới hạn; không clamp dữ liệu.

## File inventory
| Hành động | File tuyệt đối / phạm vi | Test impact |
|---|---|---|
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/channel/ChannelScreen.tsx`, `SourcesTab.tsx`, `TemplatesTab.tsx`, `SettingsTab.tsx`, `CreateStep.tsx`, `ChannelNewScreen.tsx`, `DoneStep.tsx` | Bỏ tab/cột cũ; giữ bốn stats/identity; create3hàng |
| Xoá khi hết refs | Channel `VideosTab.tsx`, `ChannelAside.tsx`, `applied.ts`, `ScheduleCard.tsx`; autonomy desktop/mobile components | Xoá tests obsolete; mobile import useAutonomySave phải cutover cùng PR |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/library/LibraryScreen.tsx`, `TemplatesTab.tsx`, `AttachChannelDialog.tsx`, template detail/mobile counterparts | Single default/shared refs/dropdown nổi, không còn library side rail |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/router.ts`, `shell/MainNav.tsx`, `AppHeader.tsx`, `MobileShell.tsx`, `api/channels.ts`, autonomy/learned APIs và i18n | Không route/caller videos/autonomy còn sống; canonical source tab, không file shell phantom |
| Sửa + migration mới | `/Users/dat/Projects/verse/apps/viderse/internal/domain/channels.go`; `internal/channels/`, `internal/autonomy/`, `pipeline/queue/plan.go`, `autopick.go`, postgres stores/queries | Bool fixed threshold; default FK; perDay1–20; snapshot job không đổi |
| Regenerate, không sửa tay | `apps/viderse/sqlc.yaml` → `internal/persistence/postgres/gen/` | Compile repository/interfaces/fakes/CLI |
| Sửa/xoá collector obsolete | `/Users/dat/Projects/verse/apps/viderse/cmd/viderse/clone_wiring.go`, `performance_wiring.go`; `internal/performance/service.go`, `web/performance.go`; live UI/API consumers | Không còn Run/public-page/DataAPI metrics reads hoặc insight apply routes; giữ mark-posted/export metadata và shared source pacer |

## Thực hiện
1. Preflight và backup/restore proof theo backend-contracts.md. Rotation đầu → default; không lấy cursor. Dữ liệu >20, empty/dangling template: dừng và hỏi bằng số trong 11-hoi-dap, không clamp/fake default. Không sửa data production chỉ để giống fixture.
2. Thêm FK default, chuyển effective autonomy→bool bằng truth table, giữ auto-fetch đã có hiệu lực; perDay/validation1–20. Migrate query refs/default counts/delete restrictions/autonaming guards và scheduler/template resolver. Bỏ cursor/weight/rotation/public autonomy/priority khi hết consumer.
3. Mọi ảnh trong video phải scored ≥85 mới auto-approve khi switch bật; ảnh đang vẽ/chưa score/bỏ/lỗi không tự approved. 84/85 boundary và nhiều ảnh trái nhau phải có regression.
   - Bỏ cấu hình mức tự chủ **không bỏ tự sửa lỗi**: `Workflow.dc.html:980` chốt “Máy luôn tự thử sửa lỗi1lần”. Giữ đường tự sửa/thử lại một lần và chống lặp qua worker restart; bỏ cấu hình `onError/belowQuality` công khai, không xoá luôn AutoFix khỏi runner vì suy luận từ việc bỏ enum.
4. Nguồn & tự động: source list, liên kết, autoFetch/autoApprove/perDay; không cột Đang áp dụng hoặc lịch đăng. Bỏ liên kết phải confirm theo số video/ảnh hưởng thật, Huỷ không request ghi; undo chỉ khi có thao tác nghịch khả dụng.
5. Tab Template một default, Đổi list nổi + Sửa template. Job snapshot đã persist giữ; queued explicit IDs giữ, không override thì default/recipe được snapshot trong admission transaction theo backend-contracts.md, default update cùng lock. Profile giữ naming/language/tag/identity/delete. Retire runtime collector và performance read/apply routes cùng final FE consumer, không chỉ xoá key card; giữ posted/export metadata.
6. Library top-nav, bỏ side rail. Menu “Đặt làm mặc định cho kênh…”, “Mặc định của…” lấy refs mới, có thể shared2channel. Guard auto-naming không tự đổi tên template shared/đã người dùng đổi. Delete default đang dùng phải giải thích channel nào, không mất FK.
7. CreateChannel/Onboard/MCreate có đúng3hàng Cách kênh chạy; giữ lỗi identity thật và tiếp tục sau lỗi. Chuyển mobile API consumers ngay trong PR4; PR6 chỉ final layout/tweak/animation, không để mobile dùng old contract đến PR6.
8. Fixture QA riêng: MT/QH=Mẫu chuẩn · Tách nền; CL=Truyện kể đêm. Không cấy tên/ID minh hoạ vào ứng dụng production.

## Ma trận test
| Mức | Scenario | Qua khi |
|---|---|---|
| Critical | Upgrade từ rotation/autonomy legacy; worker đang chạy | Mapping đúng, job snapshot giữ, không mất media hoặc requeue job |
| Critical | AutoApprove false/true, score84/85/missing/mixed | Chỉ toàn bộ ≥85 và toggle bật mới tự qua |
| Critical | Template shared/default đổi/xoá/mất quyền | FK/refs đúng, override không đổi default, không rename template shared tự động |
| High | perDay0/1/20/21, existing>20; empty rotation | New writes đúng range; preflight stop có thông báo, không silent clamp |
| High | Create channel identity error/retry/cancel, switch save fail | Giữ dữ liệu, backend readback đúng, mobile cùng chức năng |
| Medium | Dropdown dài, tab nguồn mặc định,4stats/avatar | Desktop list nổi, mobile sheet, avatar tròn, no horizontal/vertical dock bug |
| High | Retired post-publication feature ở startup và HTTP | Không goroutine/reader chạy ẩn, không read/apply routes; MarkPosted/export/source pacing vẫn hoạt động thật |
| High | Default đổi đúng lúc admission + AutoPick=false/pick enabled | Job snapshot theo commit boundary, không bật tự lấy đã tắt |

Focused `npm test -w apps/viderse/web -- screens/channel screens/library`; `go test ./apps/viderse/internal/channels ./apps/viderse/internal/autonomy ./apps/viderse/internal/pipeline/queue ./apps/viderse/internal/persistence/postgres`. Full Viderse Go compile/tests và SQLC generate; migration CLI thật từ rỗng và từ snapshot cũ trên DB QA riêng. Typecheck/build và all relevant artboard tweaks/screenshots/controls theo verification.md.

## Rủi ro / bảo mật / rollback
DB/FE/backend phải cùng revision. Không widen quyền set default/template/delete, workers/admin vẫn admin. Chỉ backup owner được truy cập snapshot.
Rollback schema chỉ restore backup đã kiểm khi mọi writer còn quiesced liên tục từ backup tới acceptance. Sau resume không restore snapshot cũ: forward-fix/recovery riêng có reconcile job/paid receipt. Xem backend-contracts.md; không giả Down lossless hoặc giữ alias để né cutover.

## Checklist
- [ ] Hoàn tất preflight, mapping/truth table và stop conditions.
- [ ] Cutover schema/backend/query/caller kể cả mobile và CLI.
- [ ] Channel/library/template/create theo board, fixtures chỉ QA.
- [ ] Qua migration/API/score-boundary/UI/motion checks; gửi report và gap V38, merge PR4.
