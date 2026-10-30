# P3 — ScanCore (luồng quét đối thủ + Trinh sát phía máy chủ)

Trạng thái: DONE_WITH_CONCERNS (xem §5: route HTTP thao tác việc nền không
tồn tại trong tầng web — seam `TaskAction` đã sẵn; không thuộc quyền web của
agent này). Đã xử lý toàn bộ finding vòng 2 (§7). Không commit/push, không smoke
YouTube thật.

## 1. Đã làm

### internal/youtube (chỉ thêm)
- `ref.go`: `ParseChannelRef` chuẩn hoá đầu vào Add — `@handle`, `UC…`, URL
  `/@x`, `/channel/UC…`, `/c/x`, `/user/x` (kể cả thiếu scheme) → ref để
  `ResolveChannelRef`; URL `/watch`, `/shorts`, `/embed`, `/live`, `youtu.be`,
  host khác YouTube → `ok=false` (`invalid_input`). Không đổi `ResolveChannelID`.
- `uploadtime.go`: `FetchUploadTime(ctx, videoID) (*time.Time, bool, error)` gọi
  InnerTube `/player` qua `Client.session/execute` (chung `Pacer`, một request
  mỗi video) trả giờ đăng chính xác từ `ParseUploadTime` và cờ **phụ đề do kênh
  đăng** (`ParseChannelCaptions`: có track `kind != "asr"`). Trang thử thách bot
  (`UploadTimeBlockedError`) được đổi thành `FetchError{Kind: blocked}` để worker
  dừng thăm dò thay vì đếm lỗi từng video. `UploadTime` giữ chữ ký cũ.
- `testdata/player.json` (mới) + nhánh `kindPlayer` trong transport fixture;
  test ref (3 ca) và upload time (4 ca mới: cờ caption, player qua pacer,
  asr-only, bot → blocked). yt-factory không có fixture `/player` (chỉ có raw
  trong `upload-time.test.ts`) nên tự dựng fixture cùng hình dạng.

### internal/scan (mới, viết lại toàn bộ theo review)
- `errors.go`: `Error{Code, Message, Recovery, RetryAt, Existing}`, `CodeOf`;
  `StoreError` + mã `lease_lost/job_stopped/channel_not_found/key_range/limit/
  checkpoint_owned/already_capped/cursor_invalid/channel_exists`; `IsStopped`.
- `store.go`: hợp đồng `Store` (mỗi phương thức một giao dịch): kênh/job/state/
  lease kênh + lease worker, `CommitPage`, `CompleteHeadCheck`, `Status`,
  `StartBackfill`, `ControlBackfill`, `StartDiscovery(replaceStopped)`,
  `SetHistoryCap`, `NextRunnableJob(skip)`, `NextRunnableJobForChannel`,
  `ClaimJob`, backoff/error/paused/capped/complete/cursor-restart fenced,
  `DueChannels`, `NextDeadline`, thăm dò giờ đăng (`NextUploadCandidate`,
  `RecordUploadTime` fenced, `PauseUploads`, `Bump/ResetUploadFailures`),
  `Acquire/Renew/ReleaseWorkerLease`. Hằng: `LeaseTTL 60s`, `Heartbeat 15s`,
  `PageMax 200`, `DiscoveryBlock 10 000`, `MedianSample 150`, `AnchorMax 300`,
  `IdleFloor 1s`, `ProgressInterval 250ms`.
- `service.go`: mặt API/CLI — `Add`, `Remove`, `Status`, `Control(start/pause/
  resume/cancel/check)`, `SetCap`, `RunInline(ctx, channelID) (ran, err)`,
  `TaskAction(ctx, channelID, op)` (cancel→cancel; resume/retry→resume; op lạ →
  `invalid_input`). Đọc `clone.rescanHours` (mặc định 1, kẹp 1–24, NaN/Inf đọc
  như chưa ghi). Ánh xạ lỗi provider/store sang `*scan.Error`; SSE `rivals` qua
  `SetNotifier(func(ref string))`, throttle ≤ 4/s **mỗi kênh** (gom nhịp sau);
  `Stop()` dọn timer. `syncTask` đồng bộ dòng `background_tasks`: running →
  Huỷ, paused → Tiếp tục, waiting(backoff) → Huỷ, failed → Thử lại + error_event
  + `needs_attention`; complete/cancelled/capped → xoá dòng và `ClearTaskLink`;
  dòng bị dọn thì `Start` lại và `LinkTask` lại; claim dùng `adopt` để dòng cũ
  nhận process mới. Progress history `{done: indexed, total: cap}`, chỉ ghi
  ≤ 4/s/kênh; discovery không progress. `TaskAction` resume chịu được
  `already_capped` (giữ nguyên trạng thái capped, không lỗi).
- `worker.go`: worker một goroutine — lease worker trên `platform_state('youtube')`
  (owner ngẫu nhiên, 60 s, heartbeat 15 s), một trang một việc, discovery trước
  lịch sử (`NextRunnableJob`), đọc lại trang dư đã lưu trước khi theo cursor,
  commit dưới fence kênh, progress throttled ở service, head check đóng discovery
  trong một tx. Lịch: `DueChannels` + `StartDiscovery(false)` (lịch tự động không
  khôi phục việc đã dừng). Khi rảnh: thăm dò giờ đăng 30 video mới nhất/kênh
  dưới fence kênh (fetch huỷ được khi mất lease), nghỉ 2 h sau blocked, 30 phút
  sau 3 lỗi (lưu `scan_schedule`); rồi ngủ tới hạn gần nhất trong (backoff,
  rescan, hết nghỉ thăm dò, hết giờ chờ nền tảng) hoặc wake/ctx/nhịp heartbeat.
  Không chọn việc/không thăm dò khi rảnh; nhịp heartbeat chỉ đọc lại hạn **một
  lần**/tick. Phân loại lỗi (verbatim yt-factory): abort, stopped, deferred,
  rate_limited (tôn trọng Retry-After, đồng thời `RecordBlocked` M6), transient
  (backoff ±20 % trần 30 phút, 5 lần → error), blocked (RecordBlocked + error),
  cursor (2 restart → paused), fatal. `gap_incomplete` khi chạm ordinal 10 000.
  `Run` giữ lease + chờ nhẹ khi process khác giữ; `RunInline` thử lease đúng một
  lần, trả `ran=false` ngay khi serve đang giữ. Mất lease → huỷ ctx + fetch, join
  heartbeat, nhả lease trong 5 s; shutdown chờ worker join xong.
- `recovery.go`: 5 câu RECOVERY_TEXT tiếng Việt nguyên văn (blocked/failures/
  restarts/gap/fatal). `util.go`: token/owner ngẫu nhiên, `SanitizeError`, backoff.

### persistence/postgres (scan.go/queries do ScanStore sở hữu)
Không sửa trực tiếp trong lượt này; đã dùng đúng mặt `scan.Store` và kiểm bằng
test tích hợp. Các finding C1/C2/M1/M3/M4/M5/M8/M9/M10 của review đã được
ScanStore xử lý ở tầng kho (owner lease thật, khoá state trước khi kiểm token,
khối discovery giữ nguyên, baseline lưu cursor, mẫu 30 chọn trước khi lọc,
`RecordUploadTime` fenced, cancelled không chặn check sau, lọc theo kênh trong
SQL).

### cmd/viderse
- `scan_wiring.go`: `scanServiceDeps(pool, settings, activities)` — điểm duy nhất
  biết youtube client, ScanStore, cổng nền tảng (`platformGate`: đọc
  `PlatformStateStore.Pause`, ghi `platforms.Service.RecordBlocked` ở serve /
  cùng câu SQL ở CLI), `newScanService(...)` cho CLI, `mountScan(...)` cho serve
  (SSE `rivals`, worker, `BindWorker`), `runScanWorker` trả kênh done.
- `serve.go`: dựng scan trước `mountRivals`; `scanCtx` riêng; shutdown
  `scanStop()` → chờ `scanWorkerDone` (trong ngân sách 10 s) → `scanSvc.Stop()`
  → machine/bg → đóng pool.

## 2. File
Mới: `internal/scan/{errors,store,service,worker,recovery,util}.go`,
`internal/scan/{scan_test,worker_unit_test,worker_pg_test}.go`,
`internal/youtube/{ref.go,ref_test.go,testdata/player.json}`,
`cmd/viderse/scan_wiring.go` (viết lại).
Sửa: `internal/youtube/{uploadtime.go,uploadtime_test.go,pages_test.go}`,
`cmd/viderse/serve.go`.

## 3. Kiểm chứng (đã chạy)
| Lệnh | Kết quả |
|---|---|
| `go test -p 2 -count=1 ./apps/viderse/internal/youtube/... ./apps/viderse/internal/scan/... ./apps/viderse/internal/persistence/...` (`TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test`) | **PASS** — youtube 1.66 s, scan 2.43 s, postgres 79.19 s |
| `go test -race -p 1 -count=1 ./apps/viderse/internal/scan/` (có DB) | **PASS** (3.06 s) |
| `go build ./...` | sạch |
| `go vet ./apps/viderse/...` | sạch |
| `go test ./archtest/` | **PASS** (1.12 s) |
| `(cd apps/viderse && go tool sqlc diff)` | **không diff** |
| gofmt `$(GOTOOLCHAIN=go1.26.0 go env GOROOT)/bin/gofmt -l` và gofmt go1.27.0 trên `internal/scan`, `internal/youtube`, `cmd/viderse` | sạch |

Test của tôi: scan 14 hàm (unit + 2 tích hợp Postgres thật), youtube thêm 7 ca
mới. Các ca then chốt: discovery chạy trước lịch sử trên kho thật (thứ tự lời
gọi `first,next`, cả hai việc đóng, head check + `HistoryEnd`); 5 lỗi liên tiếp
→ error/recovery; 3 lần cursor hỏng → paused; blocked → `RecordBlocked` + error;
thăm dò bị chặn → nghỉ 2 h; mất lease → dừng + nhả lease; service Add trùng →
409 exists, check → discovery running, cap sai → `invalid_cap`, TaskAction
huỷ → cancelled, Remove → not_found.

## 4. Số liệu
- **Nhịp rảnh (2 đối thủ, không việc due)**: worker **0 lần thức để chọn việc,
  0 lần chọn việc/claim/thăm dò mỗi phút**; mỗi phút chỉ có 4 lần ghi heartbeat
  bắt buộc (TTL 60 s) và mỗi tick đọc lại hạn **một** truy vấn
  (`NextDeadline`), cộng 1 lần đọc giờ chờ nền tảng khi vào nhịp rảnh. Không có
  polling mỗi giây. (`TestWorkerIdleDoesNotPoll` mô phỏng 4 tick/phút: biến đếm
  `NextRunnableJob` và thăm dò đứng yên, chỉ `NextDeadline` tăng.)
- **Commit trang** (Postgres local, 20 video/trang): p50 **3.99 ms**, p95
  **8.85 ms**, max **12.91 ms** (`TestScanCommitLatency`).
- Head check: mẫu tối đa 150 hàng mới nhất → median + score cả kênh trong một
  tx; anchors lưu ≤ 300; khối discovery 10 000.

## 5. Sai lệch / hỏi-đáp
1. **Lease worker** dùng `platform_state('youtube')` qua truy vấn riêng trong
   `scan.sql` (cột `scanner_owner` là token CAS), không dùng
   `platforms.Pacer.WithScanner` — helper đó cấp lease 30 phút, không heartbeat,
   không khớp 60 s/15 s. Cổng nền tảng cho giờ chờ dùng đúng kho
   `PlatformStateStore`/`platforms.Service`.
2. **Nhãn dòng Việc nền**: history `label=title`; discovery
   `label=title + " · video mới"` (cùng `object.id=channelID`) để activity không
   gộp hai việc; state capped/complete/cancelled xoá dòng như contract.
3. **Mã lỗi thêm** (đã thống nhất với RivalsAPI): `transient`, `lease_busy`.
4. **Thao tác việc nền (M7/R2-06f của review)**: seam `Service.TaskAction(ctx,
   channelID, op)` đã có, nhưng tầng web hiện **không có route** thao tác việc
   nền (phase 03 contract chỉ có các route `/api/rivals*`; HTTP handler thuộc
   RivalsAPI). Vì vậy không có chỗ mount để nối UI → service trong lượt này.
   Cần một route `POST` (vd `/api/tasks/{id}/op`) do owner web quyết định; seam
   nhận `channelID` = `task.object.id`.
5. **Nhịp heartbeat đọc lại hạn**: chấp nhận 4 truy vấn chỉ-đọc/phút khi rảnh
   (bám tick 15 s bắt buộc) thay vì poll 1 s; con số "≤ 1 lần thức/phút" hiểu
   là **0 lần thức để làm việc**. Nếu muốn 1 truy vấn/phút, giảm tần suất đọc
   hạn xuống mỗi 4 tick (đổi lại việc nhặt job do process khác tạo chậm ≤ 60 s).
6. **Pacer chỉ chia sẻ trong process**: CLI và serve mỗi bên một client/pacer
   trong bộ nhớ; giờ chờ nền tảng/429 thì chia sẻ qua DB và được worker enforce
   trước mọi request. Không có lịch request bền dùng chung giữa hai process —
   ngoài phạm vi contract (không thêm retry ẩn).
7. **`RunInline` trả `ran=false` ngay** khi serve giữ lease worker (CLI chỉ xếp
   việc + in trạng thái); RivalsAPI đã bỏ probe lease riêng của họ.
8. Fixture `/player` tự dựng (không có trong yt-factory); fixture `videos-*`,
   `channel-home.json` đã có sẵn từ trước.
9. `Control(resume)` coi `already_capped` là no-op thành công (giữ capped cho
   tới khi nâng cap), không trả lỗi `transient`.

## 6. Vòng 2 (review 2026-10-30) — đã sửa
| Finding | Cách sửa | Test hồi quy |
|---|---|---|
| R2-01 heartbeat race | `setActive` lưu **bản sao** lease cho gia hạn; trang/thăm dò giữ bản của mình, heartbeat chỉ sửa bản của nó | `TestRaceHeartbeatDoesNotTouchPageLease`, `…ProbeLease` (-race) |
| R2-03 idle/past/actionable | `Store.NextWakeup` (một truy vấn: `Actionable` + `Next`) thay `NextDeadline`; hạn đã qua không thành giá trị ngủ; việc actionable đánh thức ngay; fence kênh bận → `idleFenced` ngủ tới hạn fence, không claim-spin | `TestWorkerWakesAtNextDeadline`, `TestWorkerIgnoresPastDeadline`, `TestWorkerIdleDoesNotPoll`, `TestInlineBusyChannelWaitsWithoutSpin` |
| R2-04 gate/pause errors | lỗi đọc giờ chờ nền tảng **lan ra** (không request nào chạy); 429/blocked trong Add ghi bền `RecordBlocked` (Retry-After) | `TestAddPauseReadFailureStopsRequest`, `TestAddRateLimitIsPersisted`, `TestAddBlockedIsPersisted` |
| R2-06 task atomicity | dùng `Store.SyncScanTask(job, channel, spec, state)` — tạo/nhận dòng + nối việc trong một tx (khoá parent→state→job); bỏ Start/LinkTask rời và đường bù Finish | `TestSyncTaskUsesAtomicSyncForAdoption`, `TestSyncTaskStoreErrorLeavesNoOrphan` |
| R2-07 adoption progress | spec mang tiến độ hiện tại (progressNow) khi adoption; kho `AdoptTask` giữ tiến độ nếu spec không có; mọi ghi progress đi qua ngân sách ≤4/s | `TestTaskAdoptionKeepsStoredProgress` (Postgres thật, start/adopt 2 lần vẫn 2/150) |
| R2-08 check≠resume | `Control(check)` chỉ `StartDiscovery(true)`; paused/cancelled giữ nguyên | `TestCheckDoesNotResumePausedHistory` (paused + cancelled) |
| R2-09 ref chặt | `ParseChannelRef`: đúng một đoạn handle/tên, chặn `/`, khoảng trắng, `?`, `#`, `%`, userinfo, port, query/fragment, path lồng; `/channel/UC…` chỉ nhận id hợp lệ + tab con đã biết; vẫn nhận handle quốc tế | 3 hàm test ref (thêm 8 ca sai + international) |
| R2-10 caption rác | track phải là object có `baseUrl` và `kind != asr`; hàng null/thiếu baseUrl bị bỏ | `TestUploadTimeCountsOnlyChannelProvidedCaptions` (thêm ca null/rỗng) |
| R2-11 dead code | xoá `Client.UploadTime`, `storeErr`, `LinkTask` khỏi interface `scan.Store`, `progressNow` chỉ còn cho tạo dòng mới | build/vet sạch |
| R2-D inline spin | fence kênh bận khi inline → ngủ `BusyRetry` có ctx/wake (test-only knob), không claim vòng | `TestInlineBusyChannelWaitsWithoutSpin` |

Kiểm chứng lại sau vòng 2: `go test -p 2 ./internal/scan ./internal/youtube` **PASS**
(3.44 s / 1.78 s), `go test -race -p 1 -count=20 ./internal/scan` **PASS**
(69.10 s, không race/flaky), `go build ./...`, `go vet ./apps/viderse/...`,
`go test ./archtest/`, `sqlc diff`, gofmt 1.26+1.27 sạch.

## 7. Câu hỏi mở
- Ai sở hữu route HTTP thao tác việc nền (`cancel/resume/retry` cho
  `channel_scan`) — RivalsAPI (web) hay phase 07? Seam `TaskAction(channelID,op)`
  đã sẵn và test; không route nào tồn tại trong tầng web hiện tại.
- Có cần giảm truy vấn đọc hạn khi rảnh xuống 1/phút (đổi độ trễ nhặt job
  cross-process) không?
- Khi serve không chạy, CLI xếp việc rồi tự chạy inline; nếu một serve khác
  đang giữ lease, CLI in trạng thái "đã xếp" — đã đúng hợp đồng, có cần thêm
  câu hướng dẫn trong output CLI không?

## 8. Fix round 3 (2026-10-30) — đã sửa
| Finding | Cách sửa | Test hồi quy (đã xác nhận đỏ trước) |
|---|---|---|
| R3-C1 hạn lease worker | `leaseWatch` hẹn giờ đúng `Until` của lease đang giữ, độc lập nhịp heartbeat; giữ `workCtx` cho mọi fetch/commit nên hết hạn huỷ cả fetch đang bay; mỗi lần gia hạn thành công đặt lại hạn; lần gia hạn chạy dưới `context.WithDeadline(ctx, lease.Until)` nên DB treo không thể kéo dài quyền ghi; hết hạn/mất lease → dừng phiên, join heartbeat, nhả lease rồi **quay lại nhận lease** (không kết thúc vòng đời worker); ctx caller huỷ → Run trả nil | `TestWorkerDeadlineExpiryCancelsFetchBetweenTicks` (tick 300ms, hạn 60ms, fetch huỷ <200ms), `TestWorkerRenewalBlockedPastExpiryStops` (renew treo bị chặn ở hạn giữ, commit=0, release=1), `TestWorkerReacquiresLeaseAfterExpiry`, `TestWorkerOnLostLeaseReleasesAndReacquires`. Gỡ `watch.arm` → test hết hạn timeout (đỏ đã xác nhận) |
| R3-M2 worker side | thăm dò gặp fence trả sentinel kèm kênh; vào cùng đường chờ fence như danh sách (`idleFenced`: việc/kênh khác claim được thì đi tiếp, còn lại ngủ tới hạn fence); lặp lại đúng candidate vừa fence → ngủ `busyRetry` có kiểm soát, không chọn lại liên tục | `TestProbeFenceMissWaitsWithoutSpin` (Actionable=true + hạn fence tương lai; cũ: 711 084 lần chọn/130ms → nay ≤ 6), `TestFencedProbeLetsOtherChannelProceed` (A fence, B chạy ngay, không request cho A) |
| R3-M4 service side (progress budget) | một ngân sách duy nhất: dòng mới xin `progressOf` một lần làm khởi tạo; adoption chỉ gửi `spec.Progress` khi ngân sách cho phép (nil ⇒ kho giữ nguyên, 0 ghi); `Tasks.Update` sau `SyncScanTask` chỉ mang phase/action, không ghi lại tiến độ; xoá `progressNow` | `TestAdoptionHonoursProgressBudget` (10 adoption cùng mốc ⇒ ≤1 payload; trước sửa 20), `TestSyncTaskUsesAtomicSyncForAdoption`, `TestProgressBudgetAcrossPageSyncs` (3 lần đồng bộ trang cùng mốc ⇒ 1; sau 250ms ⇒ 2) |
| R3-m1 test barrier | `fakeStore.NextWakeup` đóng `idleEntered` ở lần đọc lịch rảnh đầu; test chờ rào rồi mới chụp bộ đếm (không kéo dài sleep, không nới assertion); `t.Cleanup` luôn cancel + join worker kể cả khi fatal | `TestWorkerIdleDoesNotPoll` chạy `-race -count=5` và chuỗi `-count=20` (trước đây lỗi giả "thăm dò khi rảnh: 0 → 1" ở lần chạy lặp của reviewer) |

Kiểm chứng vòng 3: `go test -race -p 1 -count=20 ./apps/viderse/internal/scan`
**PASS** 65.99 s (có DB, `lockf`/`GOMAXPROCS=4`); `go test -count=1 ./internal/scan`
3.22 s; `go build ./...`, `go vet ./internal/scan ./cmd/viderse`,
`go test ./internal/youtube`, `go tool sqlc diff` sạch.

Ghi chú hợp đồng với ScanStore (đã thống nhất): `Wakeup{Actionable, Next}`
giữ nguyên; `Actionable` chỉ việc claim được ngay (job queued/due-backoff không
fence, rescan chưa có run discovery sống, upload mẫu 30 chưa đọc + hết nghỉ +
không fence; pause hết hạn một mình không phải việc); `Next` gồm hạn fence của
kênh có việc (không bao giờ hạn đã qua); `NextUploadCandidate` bỏ kênh đang bị
fence; `SyncScanTask` suy trình bày từ job đã khoá (running chỉ bị hạ cấp,
capped/complete/cancelled → `job_stopped` không tạo dòng). Worker chỉ đọc
`Actionable`/`Next`.

## 9. Fix round 4 (2026-10-30) — đã sửa

| Finding | Cách sửa | Test hồi quy / bằng chứng đỏ-trước |
|---|---|---|
| R4-M1 worker side: việc running còn fence làm vòng chọn nóng | Đường danh sách ghi nhớ kênh vừa bị fence (`lastFenced`); nếu lựa chọn kế tiếp trả lại đúng kênh đó thì ngủ một nhịp có kiểm soát (`busyRetry`, thức được bởi wake/heartbeat/ctx) rồi mới thử lại — không còn vòng chọn không hẹn giờ. Nhánh `job == nil && busy` vẫn đi qua `idleFenced` (SQL đã fence-gate retained-running: `Actionable=false`, `Next`=hạn fence) nên khi không còn việc khác thì ngủ tới đúng hạn fence. `lastFenced` được xoá khi claim được fence hoặc sau nhịp chờ. | `TestWorkerFencedListingSelectionStaysBounded` (kho giả trả cùng kênh + `AcquireChannelLease` nil + lần đọc lịch còn `Actionable=true`): trước sửa **155.429 lượt chọn trong 60 ms**; sau sửa ≤4 (đo 2–3), chạy `-count=5` ổn định. Phía SQL/real-store: regression của ScanStore (retained running + fence ngoài → `Actionable=false`, `Next`=hạn fence). |
| R4-M2 service side: state/phase/action rời rạc | Tách `TaskView{State, Phase, Action}` + hàm thuần exported `scan.TaskPresentation(job Job) (TaskView, bool)` (live=false cho complete/cancelled/capped; discovery href = `#/explore?rival=<ChannelID>`). `Store.SyncScanTask` đổi thành `(ctx, jobID, channelID, spec)`; state/phase/action do kho suy từ job ĐANG KHOÁ. Service **mọi** đường ghi (adopt, tiến độ trang, failed/error, control) chỉ gọi `SyncScanTask`; bỏ hẳn nhánh `Tasks.Update` mang ảnh chụp cũ, bỏ `taskSync.adopt`, thu gọn interface `Tasks` còn `Finish/Notify`. Thông báo "Cần xử lý" chỉ phát sau khi sync thành công và view=failed. Discovery href lấy từ ChannelID thay vì Handle (quyết định lead). | `TestTaskPresentationKeepsAcceptedActionMapping` (paused→paused/"Đã tạm dừng"/Tiếp tục; error→failed/LastError/Thử lại; backoff→waiting; terminal→live=false), `TestTaskPresentationDiscoveryAndHistory`, `TestSyncTaskWritesOnlyThroughStore`, `TestSyncTaskUsesAtomicSyncForAdoption`, `TestAdoptionHonoursProgressBudget`, `TestProgressBudgetAcrossPageSyncs`, `TestFailedTaskSyncsOnceAndNotifies`, `TestTerminalJobFinishesTaskWithoutSync` — tất cả khẳng định **không** có `Tasks.Update` rời. Kịch bản thật "pause chen giữa snapshot và lần ghi tiến độ → job=paused, task=paused" nằm ở pg test của ScanStore (cùng seam, reviewer đã đỏ trước); package scan của tôi không có test DB. |
| R4-m1: huỷ caller trong lúc lấy lease | `Run` chuẩn hoá lỗi `context.Canceled/DeadlineExceeded` khi `ctx.Err()!=nil` thành nil (lỗi kho thật vẫn lan ra); lease về cùng lúc ctx huỷ được trả bằng helper `releaseLease` (ngân sách `cleanupBudget`, dùng chung với `serve`) trước khi trả nil. | `TestRunCancelDuringAcquireReturnsNil` (kho chỉ trả sau khi ctx huỷ, có barrier `entered`): trước sửa trả `context canceled`; sau nil. `TestRunReleasesLeaseReturnedWithCancellation` (lease + cancel đồng thời → release=1, nil), `TestRunPropagatesAcquireError`. |

### Số liệu / cổng kiểm
- `go test -race -p 1 -count=20 ./apps/viderse/internal/scan` (DB test, heavy lock, GOMAXPROCS=4): **PASS**, 69,020 s.
- `go test -p 2 -count=2 ./internal/scan ./internal/youtube`: PASS (7,091 s / 3,082 s).
- `go build ./...`, `go vet ./apps/viderse/...`, `go test ./archtest/`, `go tool sqlc diff`, gofmt 1.26.0 + 1.27.0: sạch.
- Không sửa migration/SQL; SQL R4-M1 và SyncScanTask do ScanStore sở hữu (đã xác nhận shape qua IRC).

### Deviations / hỏi-đáp
- Guard `lastFenced` dùng `busyRetry` chứ không ngủ tới `NextWakeup.Next`: khi lần đọc lịch còn báo `Actionable=true` thì có thể là việc khác vừa chạy được, ngủ tới `Next` xa sẽ bỏ lỡ; nhịp ngắn vẫn chặn vòng nóng. Đường bình thường (SQL đã sửa) mới là đường ngủ tới hạn fence qua `idleFenced`.
- `TaskPresentation` chỉ mang state/phase/action; tiêu đề/nhãn/loại vẫn do service dựng trong `taskSpecOf` (kho không có tên kênh). Store ghi đè phase/action từ view nên ảnh chụp cũ không thể sống sót.
- Interface `Tasks` bỏ `Start/Update` (không còn dùng); `tasksStub` vẫn giữ bản ghi để test chứng minh 0 lần ghi rời.

## 10. Fix round 5 (2026-10-30) — đã sửa

| Finding | Cách sửa | Test hồi quy / bằng chứng đỏ-trước |
|---|---|---|
| R5-M1: nút của discovery đang backoff lại huỷ lịch sử | `TaskPresentation` tách nhánh backoff theo loại việc: discovery giữ waiting/"Đang chờ thử lại" nhưng hành động là **Mở** trỏ `#/explore?rival=<ChannelID>` (không op); lịch sử vẫn Huỷ. Endpoint huỷ chỉ là thao tác lịch sử nên không bao giờ gắn lên discovery. | `TestTaskPresentationDiscoveryAndHistory` (thuần) + `TestDiscoveryTaskActionsDispatchWithoutHistory` (kho thật): 5 trạng thái discovery (queued/running/backoff/error/paused), bấm đúng action mà dòng phơi ra qua `Service.Control` → lịch sử vẫn `queued`; backoff không có op Huỷ. Đỏ trước: cả hai test fail với `{Label:Huỷ Op:cancel}` / "phơi nút Huỷ". |
| R5-M3 service side: thông báo lỗi phát theo ảnh chụp cũ | Thêm `scan.Notice` + hàm thuần `FailureNotice(title, channelID, view)` (kind/text/href/label giữ nguyên câu chữ cũ) và `TaskSync{TaskID, View, NotificationID}`; `SyncScanTask` trả `TaskSync`. Kho là nơi duy nhất xác định chuyển failed (khoá job + đọc dòng cũ, ghi notification/error_event trong cùng transaction, trả id) — ScanStore sở hữu phần ghi. Service bỏ hẳn `notifyAttention`/`Tasks.Notify` khỏi đường sync: chỉ phát `Changed(res.TaskID)` sau sync thành công và thêm `Changed(res.NotificationID)` khi kho báo có chuyển failed thật. Ảnh chụp lỗi cũ sau resume → NotificationID rỗng → không thông báo; job_stopped → không sync/phát gì. | `TestFailureNoticeText`, `TestFailedSyncPublishesTaskAndNotification` (2 event: task + notice), `TestStaleErrorSnapshotPublishesNoNotification` (chỉ task), `TestSyncTaskStoreErrorLeavesNoOrphan` (0 event). Đếm notification/error_event qua refresh và stale-after-resume là regression real-store của ScanStore trên cùng seam (reviewer đã đỏ trước: count=3; 1→2). |
| R5-M4: mất SSE activity cho task khoẻ | `activity.Service.Changed(ref)` (non-mutating, gọi `emit`). Interface `scan.Tasks` = `Finish` + `Changed`. Sau mọi sync thành công service phát `Changed(res.TaskID)` (+ id thông báo khi có); sync bị từ chối/stopped không phát. Không thêm nhịp publish mới (sync đã ≤1/trang, progress ≤4/s/kênh). | `TestHealthySyncPublishesActivityChange` (activity.Service thật + recording notifier): tạo/pause/resume đều phát đúng task id; `TestChangedPublishesWithoutMutating`; test store-error khẳng định 0 event. Đỏ trước: tắt publish → `events=[]` / `changed=[]`. |

### Số liệu / cổng kiểm
- `go test -race -p 1 -count=20 ./apps/viderse/internal/scan` (DB, heavy lock, GOMAXPROCS=4): **PASS**, 82,445 s (gồm e2e discovery-dispatch và publish).
- `go test ./apps/viderse/internal/activity`: PASS (0,614 s).
- `go test -p 2 -count=2 ./internal/scan ./internal/activity ./internal/youtube`: PASS (8,453 s / 0,293 s / 3,304 s).
- `go build ./...`, `go vet ./apps/viderse/...`, `archtest`, `go tool sqlc diff`, gofmt 1.26.0 + 1.27.0: sạch.

### Deviations / hỏi-đáp
- Service không tự dựng thông báo nữa; `FailureNotice` thuần do kho gọi trong transaction (đúng yêu cầu "kho là nơi xác định transition"). Service chỉ phát id thông báo do kho trả về.
- Trường `View` của `TaskSync` hiện service không đọc (chỉ kho ghi/đối chiếu), nhưng giữ trong kiểu theo hợp đồng lead để nơi gọi khác có thể dùng.
- Sau lượt race chỉ có thay đổi gofmt trong test và thêm test activity.Changed; không đổi mã sản phẩm.

### Bổ sung sau smoke thật — throttle cho publish Việc nền (2026-10-30)
- Tách bộ giới hạn SSE per-ref thành `refThrottle` (notifyAt/pending/timer/send, cùng `ProgressInterval`, leading + trailing flush) dùng **hai** instance: `rivals` (send = `SetNotifier`) và `tasks` (send = `Tasks.Changed`, phủ cả TaskID và NotificationID). `Stop()` dừng cả hai.
- Regression đỏ-trước: `TestTaskPublishThrottledLeadingAndTrailing` — 10 lần đồng bộ một task trong ~1s trước sửa phát **10** Changed; sau sửa **2** (leading + đúng một nhịp trailing sau bùng, ≤4). `TestRivalsPublishThrottledLeadingAndTrailing` giữ hành vi rivals. E2E `TestHealthySyncPublishesActivityChange` chờ nhịp trailing thay vì đòi phát ngay.
- `go test -race -p 1 -count=20 ./apps/viderse/internal/scan` (DB, heavy lock): **PASS**, 118,835 s; `go build ./...`, vet, gofmt 1.26.0 + 1.27.0 sạch.

### Sửa lại throttle SSE (sau smoke thật) — chuyển sang tầng web
- Lý do: cửa sổ 250 ms theo từng ref ở scan vẫn cho cùng một task ref tạo 5 frame `activity` trong 200 ms khi luồng `rivals` bắn ngay trên cùng kênh SSE; trần phải tính trên cả loại sự kiện, không theo ref.
- `web.Notifier` gộp nhịp cho cả luồng của một loại: frame đầu ngay, các thay đổi trong 250 ms gộp thành đúng một frame trailing; ref trailing = ref cuối nếu mọi ref gộp bằng nhau, ngược lại rỗng; `rev` tăng dần. Constructor nội bộ `newNotifier(hub, typ, now, after)` cho test tiêm đồng hồ/hẹn giờ; chỉ `time.AfterFunc` một lần mỗi cửa sổ, không goroutine thường trực.
- Bỏ `refThrottle` khỏi scan (cả hai instance) và test tương ứng; scan phát sự kiện `rivals` và `Tasks.Changed` trực tiếp.
- Wiring: `serve.go` tạo đúng một `notifyActivity` và truyền vào `activity.New` + `mountMachine` (callback setup dùng lại), không còn notifier activity thứ hai.
- Regression đỏ-trước: `TestNotifierBurstCoalescesToLeadingAndTrailing` — 20 lần đổi trong 100 ms trước sửa phát **20** frame; sau sửa **2** (leading rev1 + trailing rev2 cùng ref). `TestNotifierMixedRefsTrailingEmpty` (trailing ref rỗng khi ref trộn), `TestNotifierSpacedCallsPublishImmediately` (cách >250 ms phát ngay).
- Cổng kiểm: scan `-race -count=20` **PASS** 85,414 s; web `-race` **PASS** 80,875 s; activity `-race` **PASS** 1,565 s; `go build ./...`, vet, gofmt 1.26.0 + 1.27.0 sạch.
