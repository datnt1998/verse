---
phase: 6
title: "Bước 5 · V39 Làm video Nhập nhanh và hàng chờ"
status: in-progress
priority: P1
dependencies: [5]
---

# Bước 5 · V39 — PR riêng

## Mục tiêu và nguồn
Một dialog/sheet Nhập nhanh cho link hoặc selection Khám phá, shortcut N; một primary; template override “chỉ video này”. Mọi video user thêm lên đầu hàng CHỜ, auto-fetch ở cuối theo nhịp; không ưu tiên toggle. Explore filters popover desktop/sheet mobile; run rules chỉ Số luồng + nhịp kênh. Nguồn V39, New/Explore/Queue/Dialogs, MNew/MExplore/MCompVideo/MQueue.
[Backend cutover](backend-contracts.md) quy định `/api/jobs`, dedup, order và snapshot; [verification](verification.md) áp dụng mọi control.

## File inventory
| Hành động | File tuyệt đối / phạm vi | Test impact |
|---|---|---|
| Tái cấu trúc | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/new/NewScreen.tsx`, `plan.ts`, styles/tests; `src/screens/RouteScreen.tsx`, `router.ts` | Trang riêng→dialog owner trong shell; parser/validation thật reuse, bỏ bước Chọn/Xem lại/priority |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/shell/AppShell.tsx`, `MainNav.tsx`, `AppHeader.tsx`, `MobileShell.tsx` | Một owner Nhập nhanh/N/CTA, return location/channel/context; không tạo shell parallel |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/explore/ExploreScreen.tsx`, `PreviewPanel.tsx`, `bulk.ts`, mobile `MExploreScreen.tsx`, `MVideoScreen.tsx` | Desktop/bulk selection→same dialog; **MCompVideo giữ channel-sheet direct add** + Vừa thêm + secondary MNew; bỏ Vì sao, shared canonical API |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/web/src/screens/queue/QueueScreen.tsx`, `MobileQueue.tsx`, `queueView.ts`; `src/api/clone.ts`; `screens/channel/latest.ts` | Không flag priority/imageWorkers/rescanHours, new contract mọi caller |
| Sửa | `/Users/dat/Projects/verse/apps/viderse/internal/web/queue.go`, `clone.go`, `domain/clone-request.go`, `clone_queue.go`, pipeline `queue/{plan,service,schedule,restart,autopick}.go`; `persistence/postgres/queries/queue.sql`, queue/clone stores và clone_wiring | Snapshot tại admission, duplicate queued target, selection-before-cap/global min và mọi ordering writer |
| Xoá | Old priority helper/control và tests incidental/wording cho trang new/old route; creation routes cũ khi hết caller | Không alias public `/clone/enqueue` hoặc `/clone` POST cũ, không xoá job-operation endpoints |

## Thực hiện
1. Re-scout consumer refs trong web/CLI/machine routes/tests. Dialog state thuộc shell/session, open từ N hoặc CTA hoặc selection; đóng trả về đúng màn, input không bị polling reset. N không chạy trong textarea/contenteditable, khi confirm/dialog khác đang mở.
2. Một/nhiều link có parser/invalid/duplicate/partial feedback thật. Desktop Explore và mobile MExplore bulk mở dialog cùng selection. **Ngoại lệ theo 01 §6.2: MCompVideo** sheet chọn kênh thêm thẳng bằng default kênh, cập nhật “Vừa thêm”; link phụ “Đổi template hoặc thêm link khác” mở MNew với nguồn đã chọn. Hai đường dùng cùng jobs/dedup/front contract, không ép đường phụ thành bắt buộc. Bỏ bước Chọn video/Xem lại và priority; summary read-only chỉ khi artboard có.
3. Template điền default kênh, đổi kênh reset về default mới; override có “chỉ video này”, templateId explicit không PATCH channel. **Recipe snapshot chỉ tại job admission transaction** theo backend-contracts.md, không ở enqueue; queued request giữ optional explicitID, existing job snapshot bất biến. Single preview/batch×/explore-source, success close+toast+Việc nền; Cancel/Esc/×/scrim không enqueue.
4. POST `/api/jobs` canonical; override hợp lệ library thắng default, không rotation. Atomic dedup trả jobId hoặc **queueItemId** cho source/channel đã chờ; Mở ở Bàn dựng định vị queued item thật và theo mapping persist khi chuyển thành job. Migrate web/CLI/machine/strict decoders/tests rồi xoá old creation paths; không tạo job sớm/giả chỉ để có ID.
5. Prepend block toàn cục, serialize enqueue/front/restart và bounds aggregate trên toàn eligible set. Sửa query ListQueuedItems: global manual priority được chọn **trước LIMIT/workset**, không sort sau channel-sorted LIMIT2000; giữ bounded scheduler/cadence, không thêm global queue limit. Auto cuối, jobs đang chạy không preempt, không sửa created_at/lease giả order.
6. Rules JSON/public patch chỉ workers1–6 cho phần cấu hình chung; imageConcurrency3/scan1h server fixed. Pause/row operations và per-channel cadence hợp lệ giữ. Remove hidden old setting reads để config cũ không tiếp tục thay quy tắc mới.
7. Desktop filter chips là popover không đẩy layout; mobile dùng sheet. Bỏ Vì sao cả preview và detail. URL/state filter giữ đúng tiêu chí, clear filter cập nhật ngay và readback đúng GET.
8. Route `/new` không còn screen riêng: chuyển mọi internal entrypoint sang dialog; legacy browser hash nếu cần phải được nhận vào cùng canonical shell/dialog, không giữ screen/route alias có chức năng độc lập. Ghi rõ behavior trong route tests, không redirect về màn sai rồi mất input/selection.

## Test matrix
| Mức | Scenario | Qua khi |
|---|---|---|
| Critical | User A đã chờ, user [B,C] thêm, auto D, đang chạy E | Pending B,C,A,D; E không bị restart; thứ tự cùng batch ổn định |
| Critical |2requests cùng source/channel hoặc khác channel | Cùng cặp chỉ một item/job, trả existing; khác channel hợp lệ; không race min position |
| Critical | template override/default đổi/restart | Snapshot đúng, default channel không đổi, worker dùng đúng template |
| High | Single/multi/explore links, invalid/mixed duplicate, retry | Giữ các mục chưa xong, không enqueue trùng, feedback rõ thật |
| High | N/input/overlay/cancel, network fail/double submit | Một dialog/mutation tối đa một lần, không mất source context |
| High | workers0/1/6/7 và removed JSON fields | Validation/admin đúng, không còn hidden setting ảnh hưởng |
| Medium | Filter menu dài, keyboard mobile, role switch/tab | Đúng overlay/44px/viewport và cùng chức năng desktop/mobile |
| High | MCompVideo direct add rồi secondary MNew | Đúng default/front, Vừa thêm theo kênh, duplicate không tạo thêm; secondary giữ video và cho override riêng |
| Critical | ≥3channel, >2000pending, newest manual ở channel sort cuối | Vẫn lấy manual mới đầu tiên, bounds không lấy từ capped subset, front/restart cùng lock |
| High | Default B commit trước admission sau worker đã đọc A; explicit T | Job mới dùng B, explicit T dùng T, job snapshot A cũ giữ; không snapshot ngoài lock |
| High | Duplicate khi queue paused/busy, jobId null, sau admission/reload | Queued workbench target đúng queueItemId và mapping sang job, không link null/general Queue |

Focused `npm test -w apps/viderse/web -- screens/new screens/explore screens/queue api`; `go test ./apps/viderse/internal/pipeline/queue ./apps/viderse/internal/web ./apps/viderse/internal/persistence/postgres`; race queue transaction/dedup với DB QA. Typecheck/build và full affected contracts; CLI thật enqueue một URL và batch trên QA nếu CLI public contract đổi. Browser all New/Explore/Queue relevant tweaks và mobile contract, evidence theo verification.md, cập nhật V39 gap.

## Rủi ro, bảo mật và rollback
Không cho frontend tự chọn auto-origin hoặc bypass cadence; request input là user-manual, internal auto path riêng vẫn giữ. Không đổi ACL hay metadata lookup/URL validation. Không gửi link riêng tư/credential vào report.
Rollback FE+BE nguyên tử; snapshot/order migration phải backup và không rewrite job đang chạy. Nếu không thể serialize front allocation, PR chưa đạt, không đổi lời copy “đầu hàng” thành “trước auto” để che thiếu.

## Checklist
- [ ] Một Nhập nhanh entrypoint, link/explore/default override/cancel đúng.
- [ ] Cutover `/api/jobs` + all callers; dedup/front ordering/rules đúng thực tế.
- [ ] Qua concurrency/API/CLI/UI/motion checks cả hai bố cục.
- [ ] Gửi comparison/control report + gap V39, merge PR5 trước PR6.
