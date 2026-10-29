# GoCore — phase 01 (Go): nền móng Viderse + hub SSE dùng chung

## Đã đổi

**Dọn Writer (Viderse)**
- Xoá `internal/agent/`, `internal/script/`, `assets/` (prompt Writer), `queries/scripts.sql`, `gen/` cũ, migration `00001_scripts_runs.sql`.
- `internal/web`: bỏ `/api/runs`, `/api/runs/{id}`, `/api/scripts`, `/api/scripts/{id}` cùng wire funcs và `StoreView`/`Submitter`.
- `internal/bootstrap`: bỏ `WriterRole`; giữ `Config`/`ResolveRole`/`ModelForRole`/`DSNFromEnv` cho phase sau. `cmd/viderse`: bỏ `loadConfig`, `printWriterLoginHint`, `models.StartPricingRefresh`, `runner`; giữ nguyên `login/logout` (OAuth qua `core/auth`).
- DB: backup `~/Backups/viderse_go-20261029T162547Z.dump` (8938 B, `pg_restore -l` liệt kê `agent_runs`, `scripts`, `goose_db_version`), rồi drop/recreate `viderse_go` owner `viderse_migrate`, role `viderse_app`/`viderse_migrate` giữ nguyên.

**Schema mới**
- `00001_base.sql`: `settings(key text PK, value jsonb, updated_at)`, `background_tasks(...)` và `notifications(...)` theo hợp đồng dữ liệu §11 (gồm `kind/agent/title/object_ref/phase/progress/eta_sec/state/action`, `kind/count/text/href/action_label/read_at`), grant DML cho `viderse_app` có guard. DB mới chỉ áp `00001_base` (kiểm `goose_db_version`).
- `queries/settings.sql`: `GetSetting`, `SetSetting`, `ClearSetting`; sinh lại bằng `go tool sqlc generate` (không sửa tay), `go tool sqlc diff` sạch.

**`platform/events` (hub SSE dùng chung)** — tách từ `apps/noverse/internal/entry/web/events.go`. API xuất khẩu:

```go
const RingSize = 500                        // bộ đệm phát lại
const PingInterval = 15 * time.Second

type Event struct { ID uint64; Type string; Data any; Meta any }
type Visible func(p any, ev Event) bool     // lọc theo principal, áp cả live lẫn replay

type Sub struct { ID string; C chan Event; Tag any; /* p */ }
func (s *Sub) Principal() any               // đọc trong callback WithSub/ForEachSub

type Hub struct{ /* … */ }
func New(visible Visible) *Hub              // visible nil = thấy tất cả
func (h *Hub) Publish(ev Event) uint64      // gán id, vào ring, fan-out, trả id
func (h *Hub) Broadcast(ev Event)           // tức thời: không id, không vào ring
func (h *Hub) Subscribe(p any, lastID uint64) (sub *Sub, replay []Event, stale bool)
func (h *Hub) Unsubscribe(sub *Sub)
func (h *Hub) WithSub(id string, fn func(s *Sub) bool) bool   // khoá hub: đọc Principal, sửa Tag
func (h *Hub) ForEachSub(fn func(s *Sub))                     // khoá hub: quét toàn bộ
func (h *Hub) Update(fn func(p any) (np any, keep bool))      // keep=false → cắt kết nối
func (h *Hub) TrySend(id string, ev Event) bool               // bù backlog, không chặn
func (h *Hub) OnPublish(fn func(Event))     // sau mỗi Publish, ngoài khoá
func (h *Hub) Seq() uint64
func (h *Hub) Close()

func ResumeID(r *http.Request) uint64        // Last-Event-ID → ?lastEventId=
func NewDeadlineWriter(w http.ResponseWriter) io.Writer
var ErrNotFlushable = errors.New(...)

type Hooks struct {                          // đều tuỳ chọn
    Preamble   func(write func(ev Event) bool)  // sau reset/replay, trước vòng live
    AfterFrame func(ev Event) bool
    OnTick     func() bool
}
func (h *Hub) Serve(w, r, sub *Sub, replay []Event, stale bool, hooks Hooks) error
```

`Serve` giữ khung dây: `event: hello` + `{"subscriber":sub.ID}` (không id) → `event: reset` khi `stale` → replay (có `id:`) → Preamble → vòng live + `: ping` mỗi 15 s + deadline ghi 15 s. Không import `platform/identity` hay `apps/*` (archtest xanh).

**Noverse chuyển sang hub chung (không còn bản sao)**
- `entry/web/events.go` giờ chỉ còn: `Event` + `visibleTo` (User/Story/Project/AdminsToo), `EventHub` bọc `*events.Hub`, watch sets (`Tag`, `WithSub`/`ForEachSub`), `SetWatch`/`Watched`/`RemoveWatch`, `RefreshUser`/`DropUser`, `OnPublish`, và toàn bộ máy móc live-frame (`liveMux`, `Backlog`, `SetLiveStream`, `AppendLiveEvent`, `ResetLiveEvents`).
- `notify.go` dùng `OnPublish`; `story_delete.go` dùng `RemoveWatch`; `watch.go` dùng `ForEachSub`; `worker.go` dùng `ResumeID`/`NewDeadlineWriter`. Frame trên dây giữ nguyên từng byte (cùng thứ tự, cùng `id:`, cùng comment ping).

**Viderse API mới**
- `GET /api/status` (`internal/web`): body `{"running","queued","paused","reviewCount","attention":[...]}`, `ETag: "<rev>"`, `If-None-Match` khớp → 304 không body.
- `GET /api/events`: SSE theo hợp đồng chung; sự kiện `status` data `{"rev":N}`. Principal = `identity.User` của phiên; `Meta` = `web.eventScope{User}` (rỗng = mọi phiên). Ping kiểm lại phiên, phiên chết thì đóng.
- `internal/status`: `Service` giữ revision, cộng Report của các `Source`, đọc cờ tạm dừng qua `Settings`, `Bump()` phát frame status. **Interface nguồn để phase sau đăng ký:**

```go
// nguồn số liệu (queue, review, tools…): đăng ký bằng Service.RegisterSource
type Source interface { Report(ctx context.Context) (Report, error) }
type Report struct { Running, Queued, ReviewCount int; Attention []Attention }
type Attention struct { Kind domain.AttentionKind `json:"kind"`; Text, Href string }

// phần store status cần (postgres.SettingsStore thoả)
type Settings interface { QueuePaused(ctx context.Context) (bool, error) }
```

  `domain` giữ `KeyQueuePaused = "clone.queuePaused"` (trùng khoá bản TS) và 4 `AttentionKind`. Nguồn lỗi chỉ bị log+bỏ qua; lỗi đọc settings → 500.
- `internal/persistence/postgres`: `OpenPool`, `SettingsStore` (`Get/Set/Clear/QueuePaused/SetQueuePaused`) trên `gen.DBTX`; sqlc code sinh lại. Wire kiểu tách khỏi domain; handler chỉ đọc → gọi service → ghi JSON; ráp toàn bộ ở `cmd/viderse/serve.go`.

## Kiểm thử (đã chạy)

- `go test -p 2 ./platform/... ./archtest/... ./apps/viderse/internal/{web,status,domain,persistence/...} ./apps/viderse/cmd/...` với `TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test` → **tất cả ok** (gồm 6 ca settings trên Postgres thật: khoá thiếu, sai kiểu, ghi đè, clear no-op, `QueuePaused` mặc định false rồi round-trip, đúng khoá `clone.queuePaused`).
- `go build ./apps/viderse/... ./apps/noverse/...` sạch; `go vet` sạch; `gofmt -l` rỗng với cả go1.26.0 và go1.27.0 trên mọi file đã sửa.
- `(cd apps/viderse && go tool sqlc diff)` → không diff.
- Smoke `viderse serve` (binary build riêng, `VIDERSE_ADDR=127.0.0.1:18789`, HOME tạm) trên DB mới: `migrate up` áp `00001_base`; setup qua curl (`/api/setup` → `/api/setup/verify` → `/api/setup/admin`) tạo admin + cookie phiên; `GET /api/status` → 200 `Etag: "0"` body đúng hình; lặp với `If-None-Match: "0"` → **304, không body**; `GET /api/events` → `event: hello` + `data: {"subscriber":"…"}`; đặt `clone.queuePaused=true` trong DB → `paused:true`, xoá → `false`. Server đã dừng, HOME tạm/cookie/binary đã xoá, cổng 18789 không còn ai giữ.
- `go test -race -p 1 ./platform/events/` → **ok (1.6 s), không race**.
- `go test -race -p 1 ./apps/noverse/internal/entry/web/` → ban đầu FAIL ở `TestLibraryRootSwitch`: race trên seam `Gateway.transport` (test ghi, `streamWorkerJobs` đọc). Đã tái hiện trên cây trước thay đổi (`git worktree add /tmp/verse-head HEAD` → cùng race) và **đã sửa** (mục Follow-up #3).
- Noverse `internal/entry/web` lần chạy đầu: 1 fail do **tôi chép sai** `SetLiveStream` vào trong nhánh vượt `liveStreamCap` của `appendStream` (bản gốc đặt ngoài). Đã sửa và chạy lại: `go test -p 2 ./apps/noverse/internal/entry/web/ ./apps/noverse/internal/persistence/migrate/ ./platform/events/ ./archtest/` → **exit 0, không fail** (164 s).
- `internal/persistence/migrate` (Noverse) fail khi chạy bằng DSN local (`dat@`) vì `TestMigrateApplicationRoleCannotDDL` hardcode userinfo `://postgres:test@` của DSN CI; chạy với DSN dạng CI thì pass (đã kiểm). Không liên quan thay đổi của tôi.

## Việc còn lại / lưu ý

- **README chưa sửa** (ngoài phạm vi của tôi): `apps/viderse/README.md` vẫn mô tả Writer, `/api/runs`, `/api/scripts`, `agent_runs`, `scripts`. Acceptance grep `apps/viderse ':!docs'` chỉ còn các dòng này → integrator sửa README thì grep rỗng. Ngoài README, grep sạch (gồm cả `apps/viderse/web`).
- `apps/viderse/internal/web/dist/` và `apps/viderse/web/` đang được ViderseWeb build lại trong cùng cây làm việc.
- `repo` local không có role `postgres`; tôi đã tạo **tạm** `postgres/test` để chạy Noverse suite đúng DSN CI (test `TestMigrateApplicationRoleCannotDDL` hardcode userinfo đó) và **đã drop lại** (`DROP OWNED` trên `postgres`/`ainovel_test` rồi `DROP ROLE`; `pg_roles` không còn `postgres`).
- `paused` nằm ngoài revision: tính năng nào ghi `clone.queuePaused` phải gọi `status.Bump()` để ETag/SSE đổi; chưa có writer nào trong wave này nên chưa nối.
- `/api/status` chưa đăng ký nguồn nào → số 0 và `attention: []` là giá trị thật.
- `apps/viderse/internal/tools` có 12 test fail do agent ToolsRunner đang sửa slice đó (không thuộc phạm vi tôi).


## Follow-up (integrator yêu cầu)

**1. ETag theo nội dung (`internal/web`)**
`getStatus` render snapshot → `json.Marshal` → `ETag` = 16 hex đầu SHA-256 của chính bytes body; so khớp `If-None-Match` theo RFC 9110 (`*`, danh sách phẩy, bỏ `W/`). Revision vẫn chỉ dùng cho sự kiện SSE. Nhờ vậy: restart (revision về 0) không còn trả 304 cho body cũ, và một trường đổi mà không `Bump` vẫn đổi ETag.
Test mới: `TestGetStatusETagFollowsContent` (cùng nội dung → cùng ETag; gateway+service mới = restart → vẫn cùng ETag; `paused` đổi không Bump → ETag khác + 200), `TestGetStatusIfNoneMatchReturns304` (khớp mạnh/yếu/danh sách/`*` → 304 rỗng; không khớp → 200). Test cũ `TestGetStatusETag304` (kỳ vọng ETag `"0"`/`"1"` theo revision) được thay.

**2. Nối lại SSE sau restart**
`platform/events.Subscribe` đã coi `lastID > Seq()` là stale (trả `reset`, không replay) — có test `TestResumeBeyondSequenceEmitsReset`. Hành vi trước khi tách y hệt (`git show HEAD:apps/noverse/internal/entry/web/events.go` → `if lastID > h.seq … stale = true`). Thêm test ở tầng Noverse `TestGlobalEventsResumeAfterRestartEmitsReset` (`Last-Event-ID: 99` trên hub mới → `hello` rồi `reset`, không có frame `story.changed`).

**3. Race `TestLibraryRootSwitch`**
Tái hiện được trên HEAD (`/tmp/verse-head`, `-run TestLibraryRootSwitch -race` → `DATA RACE`), nên là race có sẵn. Sửa tại gốc, thay đổi nhỏ nhất: thêm `transportMu sync.RWMutex` + `workerTransport()`/`setTransport()` trong `gateway.go`; 4 chỗ đọc production (`context_window_sync.go`, `gateway.go` ×2, `jobs.go`) đi qua `workerTransport()`; 14 chỗ gán trong test (`gap_test.go` ×5, `gateway_test.go` ×2, `story_delete_test.go` ×2, `input_wedge_test.go`, `ledger_api_test.go`, `nao_api_test.go`, `security_review_test.go`, `worker_cocreate_proposal_test.go`) đổi sang `tg.g.setTransport(...)`. `go test -race -run TestLibraryRootSwitch` → ok.

**4. Dọn `internal/bootstrap`**
Xoá `Config`, `LoadConfig`, `ResolveRole`, `ResolveReasoningEffort`, `ModelForRole`, `DefaultConfigPath`, `defaultProvider/defaultModel` và các import `encoding/json`, `core/llm`, `core/llmconfig`, `core/roles`. Giữ `init()` (danh tính core/auth), `DefaultConfigDir`, `DSNFromEnv`; doc comment cập nhật (chọn model sẽ đọc từ cài đặt kết nối AI lưu trong DB, xây trên `core/llmconfig`).

**Kiểm thử sau follow-up**
- `go build ./apps/viderse/... ./apps/noverse/...`, `go vet` các gói liên quan: sạch; `gofmt -l` rỗng với go1.26.0 và go1.27.0.
- `go test ./apps/viderse/internal/web ./apps/viderse/internal/status ./apps/viderse/internal/persistence/... ./platform/events ./archtest` → ok (web 1.6 s, persistence 0.8 s).
- `go test -run 'TestGlobalEventsResume|TestStoryLiveResume|TestLiveFrames|TestSetWatchBackfills' ./apps/noverse/internal/entry/web/` → ok.
- `go test -race -p 1 ./apps/noverse/internal/entry/web/ ./platform/events/` → **ok, không race** (1052 s).


## Follow-up round 2 (R1/R2/R3 + low-priority)

**R1+R2 — backfill nguyên khối (platform/events + noverse SetWatch)**
Hub mới: `Backfill(id string, fill func(s *Sub) ([]Event, bool)) bool` — fill
chạy dưới khoá hub (đọc Principal hiện hành, sửa Tag), batch trả về được lọc
lại qua `visible` với principal đó rồi enqueue nguyên khối; Publish/Broadcast
không chen vào giữa. `SetWatch` của Noverse giờ làm tất cả trong một Backfill:
kiểm user → đổi watch → đọc live backlog → enqueue. Lock order cố định
hub → liveMu (Backlog chỉ được gọi trong callback giữ hub lock). Trước đây
WithSub và TrySend tách hai critical section: RefreshUser hạ quyền giữa chừng
lọt frame bị cấm; worker delta xen vào giữa batch phá thứ tự clear→delta.
Test: `TestBackfillUsesCurrentPrincipalVisibility` (principal bị Update đổi
giữa subscribe và backfill → frame bị cấm không gửi), `TestBackfillBatchIsAtomicAgainstBroadcast` (32 lần, batch + live luôn liền nhau),
`TestBackfillFillRejects`, Noverse `TestSetWatchBackfillRespectsRefreshedPermissions`
(RefreshUser thu hồi đọc "p" mà không khoá tài khoản → zero frame) và
`TestSetWatchBackfillAndLiveDeltaReconstructInOrder` (16 lần đua SetWatch với
worker delta, text rebuilt luôn = "AB").

**R3 server side — ping interval tuỳ app**
`platform/events` thêm `Option`/`PingEvery(d)`: `New(visible, opts...)`; hub
giữ `pingInterval` (mặc định `PingInterval` = 15 s — Noverse không đổi), Serve
tick bằng `h.pingInterval`. Viderse truyền `PingEvery(2*time.Second)`; comment
trong `web/events.go` ghi hợp đồng client: không byte quá ~3.5 s ⇒ coi là
mất kết nối (KitShared/ViderseWeb cài phía client).

**Low priority**
- (a) `status.Snapshot`/`Attention`/`Report` không còn JSON tag — kiểu thuần
  domain; `internal/web` có `statusWire`/`attentionWire` + `statusToWire`,
  `getStatus` marshal wire DTO.
- (b) `init()` trong `internal/bootstrap` bị xoá: `auth.Configure` chỉ ghi
  atomic đọc lười (`core/auth/options.go` — không yêu cầu init), nên đổi
  thành `bootstrap.ConfigureIdentity()` gọi tường minh đầu `main()`.
- (c) Composition giữ ở `cmd/viderse` — không đổi.

**Kiểm thử**
- `go test -p 2` trên entry/web (full), platform/events, viderse web/status/
  bootstrap/persistence/cmd, archtest → ok (158 s).
- `go test -race -p 1 ./platform/events/` → ok; `-race` trên watch/live/
  resume/LibraryRootSwitch/job-broadcast/resume-gap/permission tests của
  entry/web → ok (11 s).
- `gofmt -l` rỗng (go1.26.0 + go1.27.0).
- Lưu ý: `apps/viderse/internal/imagegen` hiện không build (import lỗi tạm
  trong file đang làm dở của agent ImageGen — ngoài phạm vi của tôi).
