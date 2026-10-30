# Storage

Phần Lưu trữ của Cài đặt (V22, W13 + §23 + hỏi đáp #10) đã chạy thật: kiểm tra/chuyển thư mục dự án, watch ổ, và dọn dẹp (port `src/server/retention.ts` + `src/editor/media-gc.ts` của yt-factory `2c201e6`).

## Mã

- `internal/storage/` — `Service` (một package một mảng nghiệp vụ): `service.go` (nạp/ghi vị trí, watch, holds), `check.go`→ trong `service.go` (`Check`), `move.go` (rename/chép qua staging, huỷ, phục hồi), `browse.go` (liệt kê + `Open`), `snapshot.go` (hình GET + số đo nền), `volumes*.go` (`Volumes` interface: `statfs`/`diskutil info -plist` + cache), `paths.go`, `attention.go`.
- `internal/retention/` — `retention.go` (preview/run/auto), `candidates.go` (temp-dir, codex-home, export-request, staged-export, media, media-pending), `reclaim.go` (thẻ xoá `src`/`mp4` + cache 30s), `files.go`.
- `internal/web/storage.go` — `MountStorage(g, StorageService, CleanupService)`, wire type khai tại handler.
- `internal/domain/storage.go` — key settings (`storage.location|previous|altRoots|move|clean.lastRunAt|lastSwitch`) và `StoragePrevious` (+`Mode`), `StorageSwitch`.
- `cmd/viderse/storage_wiring.go` — dựng service, `finderOpener`, retention; `serve.go` một dòng gọi `mountStorage(..., activities, bg)`.
- Không thêm bảng: toàn bộ trạng thái nằm trong `settings` (`00001_base.sql`), nên **không có `00005`**. `queries/storage.sql` phục vụ receipt ảnh (dùng bởi retention media).

## Đường dẫn dự án (cho phase sau)

Mọi đường dẫn trong DB là **tương đối** với thư mục dự án; đổi thư mục không cần restart, không sửa DB.

```go
type Service struct{ ... }
func (s *Service) Root() string                              // thư mục hiện tại (tuyệt đối)
func (s *Service) Resolve(rel string) (string, error)        // chặn thoát ra ngoài, trả tuyệt đối
func (s *Service) Candidates(rel string) ([]string, error)   // thư mục cũ còn dữ liệu (đọc dự án cũ)
func (s *Service) GroupPath(group Group) (string, error)     // GroupImages|GroupEditor|GroupSources|GroupExports
func (s *Service) Roots() (current string, all []string)
```

Thư mục gốc luôn có marker `.viderse-root.json`; mặc định `~/.viderse/projects` theo `HOME`.

## Hợp đồng HTTP (§23)

Tất cả sau đăng nhập; lỗi `{error, code}`.

- `GET /api/storage` → `{projects:{path,volume{name,kind,fs,freeBytes},usedBytes,online,offlineSince?,previous?{path,bytes,deleteAfter?,mode},lastSwitch?{mode,at}},stayBytes,suggestions[{path,volume}],usage[{id,bytes}],clean{lastRunAt}}`
  - `previous.mode` = `move` (giữ 7 ngày, có `deleteAfter`) hoặc `new_only` (không `deleteAfter`, **không xoá được** — dữ liệu còn đang đọc); `lastSwitch` cho màn done/donenew.
  - `usage` phục vụ từ bộ nhớ: đo nền lúc mở, mỗi 5 phút, sau mỗi lần chuyển/dọn và mỗi lần `check`. SSE `storage` phát **sau khi** số đo mới xong, nên client tải lại là thấy số đúng. Đo 20k tệp: p50 333 ns / p95 542 ns cho 200 lần `Snapshot`.
- `POST /api/storage/check {path,mode}` → `{ok,createSubdir,checks[{kind,status,text}],needBytes,etaSec}`; `kind` ∈ `space|write|cloud|fs|same_volume`; chọn gốc ổ → `createSubdir:true` ("Viderse Projects").
- `POST /api/storage/move {path,mode:move|new_only}` → `{taskId,done}`; việc nền kind `storage_move`, tiến độ ≤ 4 cập nhật/giây + mốc nhóm.
- `POST /api/storage/move/:taskId/cancel` → `{ok}`.
- `POST /api/storage/previous/delete` → `{ok}`.
- `POST /api/storage/use-default-for-new` → `{ok}`.
- `GET /api/storage/browse?path=` → `{path,roots[{name,path}],dirs[{name,path}]}` (chỉ thư mục; bỏ ẩn/hệ thống; tối đa 500).
- `POST /api/storage/open` → `{ok}` (Finder/xdg-open qua `Opener` tiêm).
- `GET /api/storage/reclaim` → `{items[{id,title,bytes,count,desc}]}`.
- `POST /api/storage/reclaim/:id` → `{id,title,bytes,count,desc}`.
- `POST /api/storage/cleanup/preview` → `{previewId,items[{kind,label,count,bytes}],freedBytes,stoppedProcesses}`.
- `POST /api/storage/cleanup/run {previewId}` → `{removed,freedBytes,stoppedProcesses}`; `previewId` cũ → 409 `preview_required`.

Mã/status: `invalid_path` 400, `not_found`/`missing`/`unknown_kind`/`volume_gone` 404, `cloud_sync` 422, `move_in_progress`/`no_move`/`preview_required`/`not_deletable`/`not_supported` 409.

## Hold cho scheduler

```go
func (s *storage.Service) Holds() []domain.Hold   // rỗng = không giữ
type Queue interface {                            // consumer-side, settings.Service thoả
    QueuePaused(ctx) (bool, error)
    SetQueuePaused(ctx, paused bool) error
}
```

Giá trị: `domain.HoldMoving` (đang chuyển), `domain.HoldOffline` (mất thư mục/ổ), `domain.HoldUseDefault` (ổ cũ rút, cần "Dùng mặc định cho video mới"). Chỉ bật/tắt cờ chung khi service là bên bật (cờ người dùng bật không bị đụng). Scheduler gọi `Holds()` trước mỗi bước mới; `GET /api/status` cũng đã `paused:true` trong lúc giữ.

## Kiểm chứng

- `go test -p 2 ./...` (TEST_DATABASE_URL) — ok, toàn repo.
- `go test -race -p 1 ./internal/storage ./internal/retention ./internal/web` — ok (5.4s/1.6s/50.4s).
- `go test -v` số đo: watch 8,99 µs/nhịp (200 nhịp); move 64 MiB → 3 cập nhật (trần 12, 16,5/s tức chỉ 3 vì <250 ms); `Snapshot` 20k tệp p95 542 ns.
- `go tool sqlc generate` chạy hai lần: hash `gen/models.go` không đổi; `postgres` tests ok.
- gofmt sạch (go1.26.0 và go1.27.0); `go vet` sạch.
- Smoke thật port 18794, DB `viderse_smoke_storage`, HOME `/tmp/viderse-storage/home` (16 MB dữ liệu + 416 MB sau khi phình):
  - `GET /api/storage` → path `/private/tmp/…/.viderse/projects`, usedBytes 15 728 652, usage images 3 MB.
  - `check /Volumes/ViderseSmoke` → same_volume ok (chép 15,7 MB), space ok (cần 17,3 MB / còn 626 MB), write ok, fs APFS ok, `createSubdir:true`.
  - `move` sang ổ APFS ảo (`hdiutil`, fsid khác) → `done:false`, hoàn tất sau ~1 s; đích có `editor/images/sources` + `.viderse-root.json`; thư mục cũ còn nguyên; `previous.deleteAfter` = +7 ngày; `lastSwitch.mode=move`; SSE `event: storage {"rev":32}` + `event: activity` cho task.
  - `move` ngược lại rồi `cancel` sau 350 ms → `{ok:true}`, vị trí và `previous` không đổi, không còn staging.
  - `hdiutil detach` → sau **1–2 s** `online:false`, `offlineSince` đặt, `/api/status` `paused:true` + attention "Không thấy ổ ViderseSmoke" (`href #/settings?section=storage`); `attach` lại → `online:true`, `paused:false`, attention biến mất.
  - `previous/delete` → `{ok:true}`, thư mục cũ mất, `previous` null.
  - `browse?path=/` → roots (Thư mục nhà, Macintosh HD, ổ ngoài) + dirs không có mục ẩn; `open` → `{ok:true}`.
  - `reclaim` → hai thẻ `src`/`mp4` (count 0 vẫn hiện); tạo `TMPDIR/viderse-dl-smoke` 5 MB cũ 6 giờ → preview `{items:[{kind:"temp-dir",…,bytes:5242880}],freedBytes:5242880}` → run `{removed:1,freedBytes:5242880}`, thư mục biến mất; `clean.lastRunAt` được ghi.
  - Dọn dẹp: DB drop, ổ detach, xoá temp — không còn tiến trình nào.

## Khác biệt so với thiết kế

- `deleteAfter` vắng mặt (thay cho chuỗi rỗng) khi không có hạn — client đã theo.
- Thêm mã `not_deletable` (409) và `not_supported` (409) ngoài danh sách §23; `volume_gone` trả 404 (client SettingsWebA đã xử lý banner).
- `usage` không realtime: cache bộ nhớ (5 phút / sau thao tác), đã báo SettingsWebA.
- Không có bảng mới (settings đủ), nên `00005` không dùng.

## Vấn đề mở

- `stayBytes` trong smoke bằng 0 vì config dir tạm chỉ có file nhỏ; số này chỉ để hiển thị "luôn ở máy này".
- `browse` giới hạn 500 mục/ thư mục (chưa phân trang) — đủ cho hộp chọn.
- Scheduler thật chưa tồn tại; `Holds()` đã sẵn để nối.

## Review fixes (review-p2-machine)

Mỗi mục có regression test trong `internal/storage/safety_test.go` /
`internal/retention/safety_test.go` / `internal/web/storage_test.go` /
`internal/persistence/postgres/settings_pg_test.go`.

- **C1 — move không bao giờ ghi đè đích.** `checkConflicts` so toàn cây trước
  khi ghi; `commitEntry` kiểm lại từng tệp lúc merge (kích thước+sha256):
  tệp khác nội dung → `dest_conflict` (409) và dừng; tệp giống hệt (move-back)
  → chỉ bỏ bản staging; thư mục merge đệ quy không ghi đè. Test
  `TestMoveRejectsConflictingDestination`, `TestMoveAllowsIdenticalDestinationFiles`.
- **H1 — nhật ký hai phase.** `storage.move` giờ có `from/to/staging/phase/
  taskId`; phase `commit` được ghi ngay khi dữ liệu đã hoàn chỉnh ở đích (sau
  rename hoặc sau mọi commitEntry) và trước mọi ghi cài đặt. `commitMove`
  (đổi vị trí → ghi previous → xoá nhật ký) idempotent và dùng chung cho
  worker + `recoverMove`. `recoverMove` chạy TRƯỚC `ensureLocation`: nhận
  đích sau rename bị cắt ngang; hoàn tất cutover khi vị trí đã đổi nhưng nhật
  ký còn; giữ nhật ký khi đích không đọc được (ổ rút) thay vì đoán.
  `settlePrevious`/`DeletePrevious` đi qua `deletePath` — root đang dùng
  không thể bị xoá (lỗi `not_deletable`). Test `TestRecoverAdoptsRenamedDestination`,
  `TestRecoverCleansInterruptedCopy`, `TestRecoverFinishesCommittedCutover`,
  `TestRecoverKeepsJournalWhenDestinationUnreachable`,
  `TestActiveRootNeverScheduledForDeletion`.
- **H2 — `validateTarget` cho mọi đường đổi.** Chặn đích lồng/chứa root,
  home, trong config dir, `/System|/Library|/usr|/Volumes|…`, và thư mục
  không-trống không có `.viderse-root.json` (move lẫn new_only). Check mới
  trả hàng `kind:"path"` fail thay vì lỗi chung. Test `TestMoveRejectsUnsafeTargets`.
- **H3 — chỗ lần đổi giữ tới khi cài đặt xong.** `s.moving` đặt trước mọi
  I/O và chỉ nhả sau commitMove/fail — Move/new_only/use_default/deletePrevious
  thứ hai bị `move_in_progress` kể cả cửa sổ kết thúc. Test
  `TestMoveSerializesConcurrentCalls`, `TestNewOnlyBlockedDuringMove`.
- **H4 — worker dưới lifetime service.** Worker chạy dưới `s.lifetime()`
  (không phải ctx request); `Service.Shutdown(ctx)` huỷ move đang chạy và
  chờ `sync.WaitGroup`. Copy/hash/verify/remove đều nhận ctx. `mountStorage`
  trả `*storage.Service`; `svc.Shutdown` là hook join — MachineSetup sở hữu
  điểm join sau `machine.Shutdown`. Test `TestShutdownCancelsAndJoinsMove`.
- **H5 — hold không đụng cờ người dùng.** Hold persist `storage.holds` và
  bật `storage.queuePaused` (khoá riêng); `SettingsStore.QueuePaused` =
  clone.queuePaused OR storage.queuePaused (file postgres của GoCore — một
  đoạn OR + test `TestQueuePausedOrsStorageHoldFlag`). `reconcileHolds` chỉ
  gỡ `HoldMoving` mồ côi sau sập. Test `TestHoldsReconcileAcrossRestart`,
  `TestUserPauseSurvivesHoldRelease`.
- **H6 — volumeCache lock đúng.** Mutex + snapshot bất biến + single-flight
  qua khoá; `byPath` reset khi refresh. Không còn ghi map ngoài khoá.
- **H7 — id pending phải portable.** `pendingMedia` và `reclaimPending`
  lọc `isPortableID` — id `../x` trong JSON trên đĩa bị loại, không bao giờ
  join path. Test `TestPendingRejectsTraversalIDs`,
  `TestPendingMediaFiltersUnsafeIDs`.
- **H8 — receipt lỗi → giữ call-*.** `ScratchDirs` lỗi: mọi thư mục
  `*call-*` trong TmpDir được giữ và lỗi nhóm hiện ở `preview.errors` (wire
  `errors`). Test `TestReceiptsErrorProtectsScratch`.
- **H9 — tham chiếu fail closed + re-check dưới khoá.** `heldMedia` quét
  MỌI root (altRoots), lỗi đọc dir/file tham chiếu (khác ENOENT) → lỗi nhóm
  media (không xoá gì); `media.Remove` re-check `mediaEligibleFresh` dưới
  `locks.lock("media/"+id)` — claim sau lúc dò vẫn thắng (port của đoạn
  heldMediaIds trong media-gc.ts). Test `TestHeldMediaAcrossRoots`,
  `TestMediaFailsClosedOnUnreadableReference`.
- **H10 — run chỉ xoá đúng preview.** `pendingPreview` gắn `{id, root,
  candidates}`: previewId sai → `preview_required`; đổi root sau preview →
  `preview_required`; mục sinh sau preview không bị ăn theo; `Validator`
  re-check trước xoá (export-request còn terminal+quá hạn, staged-export còn
  staged, media còn eligible) — giữ lại báo qua `kept`. Test
  `TestRunDeletesOnlyPreviewedSet`, `TestRunRequiresPreviewAfterRootChange`,
  `TestPreviewIDIsConsumed`, `TestRunKeepsStagedExportThatFinished`.
- **H15 — trạng thái move trong GET.** `GET /api/storage` trả
  `move:{active,taskId,failed{taskId,target,phase,at}}`; failed persist
  `storage.move.lastFailed` và được đổ vào sau reconcile sập. `POST
  /api/storage/move/retry` (server giữ đích) và `POST /api/storage/move/dismiss`.
  Task storage_move failed mang `action{op:"retry"}`. Đã chốt với
  SettingsWebA. Test `TestFailedMoveExposedRetryableDismissable`,
  `TestStorageMoveInfoWireAndActions`.
- **M2 — idle không đi cây.** Bỏ rescan định kỳ: usage đo lúc mở + khi
  `Changed`/check (wake-only loop); sweep 6 giờ của V22 giữ nguyên (đó là
  spec, không phải overhead). Test `TestUsageLoopWalksOnlyOnSignal`.
- **M3 — progress trần thật 4/s.** Mọi send (kể cả đổi nhóm/phase) qua nhịp
  250ms; chỉ một flush cuối qua thẳng. `TestProgressHonoursHardCap`:
  elapsed/250ms + 2.
- **M5 — lỗi mục không nuốt.** Run wire trả `errors[]` (và `kept[]` cho mục
  điều kiện đổi); lỗi nhóm hiện ở preview `errors[]`.
- **L1 (phần tôi).** Bỏ `loaded`, `wePaused`, `usageAge`, QueueHold;
  `pathBytes`/`dirBytes`/`readJSON` gộp vào `internal/fsio` (Bytes,
  RemoveAll có ctx) — storage+retention dùng chung. `mountStorage` không còn
  tạo `activity.Service` (dùng instance chung của serve).

### Đã thông báo cho các slice khác

- MachineSetup: `svc.Shutdown(ctx)` là hook join; `QueuePaused` OR hai cờ
  (tôi đã sửa `postgres/settings.go` — một chỗ nhỏ).
- SettingsWebA: `move.{active,failed}` + `move/retry` + `move/dismiss`
  (đã chốt hình dây).

### Kiểm chứng

- `go test -p 2 ./internal/storage ./internal/retention ./internal/web
  ./internal/persistence/postgres` — ok (3.9/0.4/5.8/2.5s).
- `go test -race -p 1` cùng các package — ok.
- `go test ./archtest` — ok (`internal/fsio` là business layer).
- gofmt sạch trên go1.26.0 và go1.27.0; `go vet` sạch.
- Lỗi còn lại ở `go test ./...` thuộc file đang-sửa của sibling
  (`platforms_wiring_test.go`, `TestSetupHTTPFlow…`) — không phải vùng tôi.
