# ImageGen — internal/imagegen + image receipts

Chuyển đường vẽ ảnh trả phí từ yt-factory `2c201e6` sang Go: hai provider
(Codex CLI cô lập, Grok/xAI), fallback theo provider, biên nhận Postgres ghi
trước/sau mỗi attempt trả phí, trần ảnh trả phí ngày (D7), và phục hồi crash
không bao giờ vẽ lại mù quáng.

## Nguồn đã port

| yt-factory | Đích Go |
|---|---|
| `src/server/clone/images.ts` (`drawImage`, `generateIsolatedImage`, `prepareIsolatedCall`, `verifyImageReadBoundary`, `classifyImageFailure`, `reconcileImageInvocation`, busy/quota/transient retry) | `internal/imagegen/service.go`, `codex.go`, `imagegen.go` |
| `src/server/clone/codex-image-output.ts` (JSONL parser, PNG validation, capture/rollout/owned-session) | `internal/imagegen/codex_output.go` |
| `src/server/tools/grok.ts` (`drawImage`, aspect, edits, 401/403/429) | `internal/imagegen/grok.go` |
| `src/server/clone/artifacts.ts` (`redactLogLine`), `errors.ts` (CloneError + subclasses) | `internal/imagegen/imagegen.go` |
| journal file `draws/<key>.journal.json` | bảng `image_receipts` (Postgres) + cột `scratch` jsonb |

## Files

- `apps/viderse/internal/imagegen/`: `imagegen.go` (types + classify + redact),
  `service.go` (Service: idempotent lock, settle/recover, fallback, busy + transient
  retry, cap), `codex.go` (CodexProvider + scratch/Reconcile), `grok.go`
  (GrokProvider), `codex_output.go` (parser + PNG + capture), `atomic.go`
  (temp+link atomic non-overwrite writes), `journal.go` (Journal interface),
  `open_nofollow_{unix,other}.go`, `pid_{unix,other}.go`, `smoke_test.go`.
- `apps/viderse/internal/persistence/migrations/00002_image_receipts.sql` —
  PK `(draw_key, attempt)`, `state` check, `charged` mặc định TRUE (thận trọng),
  index `(provider, started_at)`, unique partial index `one_running` trên
  `draw_key` khi `state='running'`, grant guard `viderse_app`.
- `apps/viderse/internal/persistence/postgres/queries/imagegen.sql` → regen
  `gen/imagegen.sql.go` + `gen/models.go` (`go tool sqlc generate`).
- `apps/viderse/internal/persistence/postgres/imagegen.go` —
  `ImageReceiptStore` implements `imagegen.Journal` trên `gen.DBTX`.
- `apps/viderse/internal/persistence/postgres/imagegen_pg_test.go` — pgtest.

## API xuất ra (cho pipeline dùng)

```go
type Provider interface {
    Name() string; Model() string; Paid() bool
    Ready(ctx) error
    Draw(ctx, Request, JournalHandle) (Receipt, error)   // điểm tiêu tiền duy nhất
    Reconcile(ctx, scratch, Request) (Receipt, bool, error) // cứu attempt bị ngắt
}

type Request struct { Key, Prompt, Output string; Width, Height int
    Reference string; Deny []string; Timeout time.Duration }

type Receipt struct { Provider, Model, PngSha256, SessionID, CallID,
    RequestID, Attribution, CodexVersion string } // JSON — lưu luôn trong biên nhận

type Journal interface {
    LockKey(ctx, key) (unlock func(), err error)  // advisory lock DB (qua process)
    Begin(ctx, Admission) (JournalEntry, error)   // ghi attempt + kiểm trần NGUYÊN TỬ
    Latest(ctx, key) (*JournalEntry, error)
    Finish(ctx, key, attempt, state, charged, *Receipt, errCode, errText, finishedAt) error // fence: chỉ attempt running của ta
    UpdateScratch(ctx, key, attempt, scratch) error
    Charged(ctx, provider, since) (int64, error)
    Abandon(ctx, key) error
}
type Admission struct { Entry JournalEntry; PaidCap int; DayStart time.Time }
// PaidCap < 0 = không kiểm trần; >= 0 = store tự chặn khi đã dùng đủ.
type JournalEntry struct { Key, Provider, Model, Output, State, ErrorCode,
    ErrorText string; Attempt int; Charged bool; Receipt *Receipt;
    Scratch json.RawMessage; StartedAt; FinishedAt }

type DrawError      { Code, Message, Recovery, RetryAtSource string; RetryAt *time.Time }
type ProviderBusyError / ImageRefusedError   (nhúng DrawError + Unwrap → giữ code/retryAt;
                                              IsProviderBusy/IsImageRefused)
type CapError       { Provider string; Used int64; Limit int; RetryAt time.Time } // trần D7
type CapReachedError{ Provider string; Used int64; Limit int }  // sentinel của store trong Begin
type StoreError     { Err error }        // lỗi biên nhận, không rơi vào fallback
var  ErrAttemptConcluded error            // fence: attempt đã bị chủ khác kết luận
func MarkUnpaid(err error) error         // lỗi chắc chắn trước điểm tiêu phí
func IsUnpaid(err error) bool            // busy/refused/quota/tool_missing/store → true
func ErrCapReached(provider string, used int64, limit int) error // store tự dựng dùng sentinel này

type Service; func NewService(Options) *Service
// Options: Providers (đầu = primary), Fallback *bool (mặc định bật), Journal,
//   DailyPaidImageLimit *int (nil/âm → 50; 0 = không tự tiêu), Now, Sleep,
//   TransientRetries *int (nil → 2: 5s/20s).
// Service.Draw/Abandon/Charged/DailyLimit/ResetQuotaSkips.

func NewCodexProvider(CodexDeps{Codex, Runner, StatusFunc?, SourceHome?,
    PrepareCatalog?, ForbiddenSentinels?, OwnPID?}) *CodexProvider
func NewGrokProvider(GrokDeps{Tokens xai.TokenSource, Client?, BaseURL?,
    ResolveFfmpeg?, Runner tools.Runner (bắt buộc khi payload không phải PNG), ReadyFunc?}) *GrokProvider
func postgres.NewImageReceiptStore(db gen.DBTX) *ImageReceiptStore

// tiện ích: ValidatePng/ValidatePngFile/DecodeImageBase64, GrokAspectRatio,
// CodexImageOutputParser, CaptureImage, FindOwnedSessionImage, ReadImageRollout,
// ProviderLabel, ClassifyImageFailure, ErrScratchInvalid, hằng Receipt* + ErrCode*.
```

## Luật đã chuyển

- **Fallback**: primary trước; fallback khi lỗi ≠ huỷ, output chưa có sẵn,
  secondary usable; xoá output dở trước khi fallback; output có sẵn không bao
  giờ bị xoá để nhường chỗ; cả hai lỗi → code/recovery/retryAt của primary +
  lý do fallback `(Grok cũng lỗi: …)`; busy/refused giữ class.
- **Retry**: provider-busy 4×60s trong `Draw`; transport-tạm-thời
  (`connection reset|temporarily unavailable|502|503`) 2 lần 5s/20s; provider
  quota bị skip in-process tới reset (30 phút nếu không báo mốc).
- **Biên nhận**: `Begin` ghi TRƯỚC lời gọi trả phí (attempt = MAX+1 → retry
  kế thừa chi phí); `Finish` ghi SAU với `charged=false` chỉ khi lỗi được
  `MarkUnpaid` (busy/refused/quota/tool_missing/invalid_request và mọi lỗi
  trước lời gọi: readiness, refs, scratch) — huỷ giữa chừng và forbidden tool
  giữ charged.
  `Latest` chạy trước mọi Draw: `ok` → trả receipt cũ (idempotent);
  `running` → `Reconcile` theo scratch (phiên sống → `conflict`; output
  salvage → adopt `ok`; PNG đích khác byte → `stale`; scratch hỏng → kiểm tra
  file output generic) không thì `failed_unknown` và khoá key — không vẽ lại
  mù; `failed_unknown` đứng cho tới khi `Service.Abandon` tường minh
  (`abandoned` giữ làm bằng chứng).
- **Trần ngày D7**: chỉ provider `Paid()`; đếm `charged` từ nửa đêm địa
  phương; `used >= limit` → `*CapError` với `RetryAt` = nửa đêm hôm sau; 0 =
  không tự tiêu; chặn ngay trong transaction `Begin` (không read-then-insert),
  nhớ skip tới reset; Draw bị chặn không ghi attempt, không gọi provider.
- **Codex isolation**: `requireImageReadiness` (binary + chatgpt login +
  verified incl. `image-tool-allowlist`), prepare cùng luồng bản cũ (CODEX_HOME
  chỉ-auth + allowlist catalog + refs copy + probe biên `/bin/sh -c 'exec 3<"$1"'`
  x3: refs-readable/auth/secret), exec args qua `tools.CodexExecArgs`, env
  exact, timeout clamp 60s–20m (mặc định 10m), sentinel leaks → cancel,
  forbidden tool → `isolation_unverified`.
- **Grok**: POST `images/generations` (hay `images/edits` + data-URI khi có
  reference), `b64_json`, timeout 3 phút; 401 → tool_missing; 403 entitlement →
  quota; 429 → busy (retry 60s); timeout/transport → internal/`ProcessFailedError`;
  payload không-PNG → ffmpeg app-managed rồi validate; sanitize secrets trước khi
  đưa vào message.
- **Output**: mọi write là temp-file + `link()` (exclusive — `EEXIST` như
  `wx`), PNG validate head/tail/IHDR/IEND/trần 32MB/8192px/32Mpx, receipt
  provenance + sidecar `*.receipt.json`, không symlink (`O_NOFOLLOW`), không
  quét newest — chỉ session/call định danh.

## Khác biệt chủ động so với bản cũ

- **Trần ngày kiểm NGUYÊN TỬ** trong `Begin` (một transaction: advisory lock
  theo provider → `COUNT(charged)` từ `DayStart` → INSERT), nên hai process
  không thể cùng đọc 49 rồi cùng ghi 50/51. Store trả
  `*CapReachedError{Provider,Used,Limit}`; service đổi thành `*CapError` kèm
  `RetryAt` = nửa đêm địa phương kế tiếp (DST-safe bằng `AddDate`).
- **Một attempt running / key** bằng partial unique index
  `image_receipts_one_running_idx`; `Begin` lần hai → `ErrAttemptConcluded`
  (không ghi attempt, không tiêu). `Finish` chỉ chạm dòng `running` của đúng
  attempt (`execrows`); attempt đã bị chủ khác kết luận → `ErrAttemptConcluded`
  thay vì ghi đè.
- **Độc quyền key qua process** bằng `pg_advisory_lock` giữ trên một kết nối
  pool suốt `settle → draw → retry → fallback`; `Begin` KHÔNG khoá lại key
  (session lock của chính caller + unique index là đủ) để tránh tự chặn mình.
- **`charged` = đã qua điểm tiêu phí**, không phải "thành công": chỉ lỗi
  chắc chắn trước khi gửi/được provider từ chối tường minh (`MarkUnpaid`:
  busy, refused, quota, tool_missing/not-logged-in, invalid_request, lỗi
  scratch/refs trước exec) mới `charged=false`; forbidden tool ngoài allowlist
  và huỷ giữa chừng giữ charged (thận trọng chi phí).
- **Grok transport**: POST không có phản hồi → `internal` charged, KHÔNG tự
  retry (xAI không có idempotency key); 401/403/429 và 4xx khác là miễn phí;
  5xx giữ charged.
- **Ảnh hợp lệ xuất hiện sau lỗi provider** (không có từ trước) được adopt
  (`output-adopted`) và lượt đó kết luận `ok` — không rơi vào dự phòng trả phí
  thứ hai.
- `Reconcile` dọn work **sau** khi cứu output: bản cũ gọi `rm(work)` trước
  `findOwnedSessionImage` nên đường cứu ảnh thực tế chết (mọi restart →
  `failed-unknown`). Scratch mới cho phép cứu thật; khi scratch không đối
  chiếu được, luật generic (file output hợp lệ → adopt) đứng dưới như nhiệm vụ
  yêu cầu, attribution đánh `output-adopted`.
- Trần ngày D7 và lỗi `CapError`/`retryAt` là phần mới của thiết kế
  (yt-factory không có).
- `Service` gom busy/transient retry của cũ-(queue + runImage) vào một chỗ.
- Mọi văn bản provider vào message/biên nhận đi qua `sanitizeProviderText`
  (bearer/sk-, token/cookie/password kw=value, JSON token field) và bị cắt
  500 byte; `errCodeText` che lần cuối trước khi ghi DB.

## Kiểm chứng

- `go test -p 2 ./internal/imagegen/... ./internal/persistence/...` → **ok** (unit + pgtest, cần `TEST_DATABASE_URL` cho pgtest).
- `go test -race -p 1 ./internal/imagegen/... ./internal/persistence/postgres/` → **ok**.
- `go build ./...` + `go vet ./...` (apps/viderse) → **clean**.
- `cd apps/viderse && go tool sqlc generate && go tool sqlc diff` → **clean**.
- gofmt go1.26.0 & go1.27.0 → **clean**.
- Ported suites: `image-fallback.test.ts` (8 ca), `grok.test.ts` phần ảnh (8 ca), `codex-image-output.test.ts` (9 ca).
- Ca mới: `DailyLimit` mặc định nil/âm → 50, 0 → không tự tiêu
  (`TestDailyLimitDefault`); trần 49/50/51 + 0 (service, journal in-memory); trần NGUYÊN TỬ +
  ngày mới trên Postgres (`TestReceiptCapOnPostgres`); `Begin` lần hai khi
  attempt còn running → `ErrAttemptConcluded` (`TestReceiptSecondRunningBeginConflicts`);
  fence `Finish` (attempt đã kết luận không bị ghi đè); charged-filter;
  idempotent; crash-recovery (memory + Postgres); reconcile (adopt/stale/conflict);
  adopt-output-thay-vì-fallback; transient retry.

## Smoke trả phí (đúng 1 ảnh)

`VIDERSE_IMAGEGEN_SMOKE=1 TEST_DATABASE_URL=... go test -run TestSmokeOnePaidImageThenCapRefuses -v`:

- `Ready()` pass (codex 0.162.1, chatgpt, isolation verified sau khi
  ToolsRunner sửa `TrimPrefix("/private/")` → `"/private"`).
- Ảnh: `/tmp/viderse-imagegen-smoke/smoke.png` — **1254×1254 PNG, 966,063 bytes**
  (`gpt-image-2`, session `01a188d9-…`, attribution `owned-session-file`).
- Lần vẽ thứ hai: `*imagegen.CapError{provider codex used=1 limit=1 retryAt=2026-10-31 00:00 +0700}` — `runner` không spawn thêm process exec nào.
- ĐÚNG 1 ảnh trả phí thực (ngân sách chủ dự án) + 1 ảnh phát sinh trước đó do
  assertion sai trong lần smoke đầu (đếm exec tính cả probe; file cùng nằm
  trong `/tmp/viderse-imagegen-smoke/smoke.png` và đã bị ghi đè lần hai — chỉ
  một ảnh tồn tại cuối cùng).
- Xác nhận lại artifact sau khi sửa (không gọi lại provider lần nào):
  `smoke.png` = 966,063 bytes, IHDR 1254×1254; sidecar `smoke.png.receipt.json`
  còn nguyên.

## Vấn đề mở

- `CodexProvider.Ready` phụ thuộc `Codex.Status` (probe lần đầu ~2–3 s;
  cache sau đó) — pipeline nên gọi Status một lần lúc khởi động như bản cũ.
- `text-tool-advertisement` trong preflight từng flaky (ok=false → ok=true giữa
  hai lần chạy); không phải phạm vi imagegen nhưng Ready phủ nhận khi nó hỏng —
  theo dõi ở ToolsRunner.
- `grok-imagine-image` chưa được smoke vì chỉ được phép 1 ảnh trả phí (đã dùng
  cho Codex theo yêu cầu).
- Smoke KHÔNG chạy lại sau đợt sửa cuối (hết ngân sách ảnh trả phí). Đường đi
  của smoke được phủ thay thế bằng `TestReceiptCrashRecoveryOnPostgres` +
  `TestReceiptCapOnPostgres` (Service.Draw trên pool thật: LockKey → settle →
  Begin → Finish → trần). Trước khi tin tuyệt đối, chạy lại
  `VIDERSE_IMAGEGEN_SMOKE=1 ... -run TestSmokeOnePaidImageThenCapRefuses` khi
  chủ dự án cho phép thêm một ảnh.
- `ImageReceiptStore.LockKey` cần `*pgxpool.Pool` (session advisory lock). Store
  dựng trên DBTX không phải pool (vd. một transaction đang mở) trả unlock
  no-op — vẫn an toàn nhờ unique index one-running, nhưng hai process có thể
  cùng chạy `settle`/`Draw` cho một key (chỉ một Begin thắng). Composition root
  phải truyền pool.
- Vì `LockKey` giữ một kết nối suốt lượt vẽ, pool phải có ≥2 kết nối (mặc định
  pgxpool đủ); đặt `MaxConns=1` sẽ tự chặn ở `Begin`.
- `Service.Draw` không tự đặt timeout cho toàn bộ chuỗi (busy 4×60s + transient
  2 lần 5s/20s + fallback); caller (pipeline) nên truyền ctx có deadline.
