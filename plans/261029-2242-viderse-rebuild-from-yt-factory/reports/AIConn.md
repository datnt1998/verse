# AIConn — Kết nối AI (§15)

## Đã làm

**Domain** — `internal/domain/ai.go`: `Capability`, `ProviderKind` (10 loại), `AIAuth`, `AIStatus`, mã `Reason*` của hợp đồng, `Provider`, `AIModel`, `ModelRef`, `AgentModels`/`DirectorModels`/`ArtistModels`, `Quota`, `DefaultAgentModels()` (chatgpt `gpt-6.1-sol` text+vision, `gpt-image-2` image; grok `grok-imagine-image`; drawFallback = nguồn còn lại).

**Migration `00003_ai_providers.sql`** — bảng `ai_providers` (kind/auth/status có CHECK, `key_enc bytea`, `key_hint`, `catalog_at`, `selected jsonb`), `ai_models` (PK `(provider_id,id)`, `caps`, `context_k`, 3 cột giá, `recommended`, `manual`), unique index "mỗi loại đăng nhập gói một thẻ", seed 2 thẻ quản lý + 3 model mặc định để Trợ lý AI có dữ liệu trước lần lấy danh mục đầu. Grants có guard cho DB chưa có role app.

**Persistence** — `queries/aiconn.sql` + `postgres/aiconn.go` (`AIConnStore`): `ReplaceCatalog` chạy trong transaction, xoá model `NOT manual` rồi upsert, đặt `catalog_at`; model thêm tay sống sót. Status rỗng được mặc định `ready`. sqlc diff sạch.

**secretbox** — AES-256-GCM, khoá 32 byte ở `<cfg>/secret.key` (0700/0600, tạo nguyên tử), nonce 12 byte tiền tố; file hỏng/khác độ dài hoặc ciphertext sai → `ErrWrongKey` (không bao giờ trả rỗng); `Hint()` → `••••7c2e`.

**aiconn** — `aiconn.go` (service, provider list, create/patch/delete, test, check, `RefreshSubscribed`), `agents.go` (validate + `ResolveDirector`/`ResolveArtist` + chuẩn hoá drawFallback), `login.go` (session/mutex đăng nhập, phase, cancel/code/logout, `ChatGPTReady`, `ChatGPTLogin`, `CLITokens`), `catalog.go` (enrich từ `core/models`, suy caps theo tên, recommended), `adapters.go` (OpenAI-compat, Gemini, Anthropic, Ollama, ChatGPT-Codex, Grok), `auth.go` (`CoreAuth` trên `core/auth.Manager`, cờ revoked trong settings để GET đọc local).

**tools** — `CreateIsolatedCodexHome(tok CodexAuthTokens, dest)` dựng `auth.json` **tổng hợp** (`OPENAI_API_KEY:null`, `tokens.id_token=access_token`, `refresh_token:""`, `account_id`, `last_refresh`) — refresh token không rời core/auth nên CLI không thể xoay vòng. Thêm `Codex.DebugModels` + `ParseCodexModelCatalog` cho thẻ ChatGPT.

**imagegen** — `CodexDeps.SourceHome` → `Auth func(ctx)(tools.CodexAuthTokens,error)`; `Ready` lấy đăng nhập từ core/auth; bỏ `DefaultHome` khỏi `CodexBackend`; copy "Cài đặt → Kết nối AI".

**web** — `internal/web/ai.go`: `MountAI`, DTO wire của §15, phân nhánh `/api/ai/providers/<id>[/models|/models/check|/test|/login[/cancel|/code]|/logout]`, lỗi `{error, reason?, …fields}`.

**Composition** — `cmd/viderse/wire_ai.go`: secretbox, `core/auth` store+manager, adapters (Codex catalog qua `toolKit.Codex`, Grok qua `xai.ListModels`), `PaidUsage` = `ImageReceiptStore.Charged`, Notify `ai` + Bump status, `aiAttentionSource` → `statusSvc.RegisterSource` (login_expired/quota_low, href `#/settings?section=connect`), `drawHolder` dựng lại `imagegen.Service` theo `artist.draw`/`drawFallback`, warm catalog ở nền.

## Hợp đồng wire đã chốt (đã gửi SettingsWebB)

`GET/POST /api/ai/providers` (201 khi tạo), `GET /api/ai/providers/:id/models?q=&filter=selected|recommended|all`, `PATCH` `{selected?,apiKey?,baseUrl?}`, `DELETE`, `POST .../test` → `{ok,ms,error?}`, `POST .../models/check` `{model}` → `{ok,ms,caps,contextK,error?}`, `GET/PUT /api/ai/agents`.
Lỗi: `422 auth_failed` (+`status` = HTTP status của provider, +`id`,`keyHint` vì thẻ vẫn được tạo), `409 in_use` (+`agents`), `422 capability_missing` (+`need`,`field`,`model`), `409 managed`, `422 unreachable`, `422 unknown_model`, `400 invalid_kind|missing_key|missing_base_url|invalid_mode`, `409 login_in_progress`, `404 not_subscription`, `422 invalid_code`.
Login: `POST /:id/login {mode?:browser|device}` → 202 `{phase:"login_pending"}`, `GET /:id/login` → `{phase:idle|authorize|code|exchanging|done|failed|cancelled, authUrl?, code?, verifyUrl?, account?{email,plan}, error?}`, `POST /:id/login/cancel`, `POST /:id/login/code {code}`, `POST /:id/logout`.
SSE: type `ai`, ref = provider id hoặc `agents`.

## Kiểm chứng

- `go test ./internal/aiconn ./internal/secretbox ./internal/tools ./internal/imagegen` → ok (tools 27s do suite sẵn có).
- `go test -race -p 1 ./internal/aiconn ./internal/secretbox ./internal/imagegen` → ok (đã sửa 1 data race thật: `LoginStatus` trả con trỏ session sống; nay copy dưới `sessMu`).
- `TEST_DATABASE_URL=… go test -run TestAIConn ./internal/persistence/postgres` → ok (4 ca: seed, round-trip, ReplaceCatalog giữ manual, **plaintext check**).
- `go tool sqlc diff` → sạch. `go build ./internal/{aiconn,secretbox,domain,persistence/postgres}` → ok.
- **Định dạng auth.json**: `VIDERSE_CODEX_FORMAT_CHECK=1 go test -run TestCodexAuthJSONFormatAcceptedByCLI ./internal/tools` → PASS trên `codex-cli 0.162.1` (token giả đúng cấu trúc JWT, không gọi model). Kiểm thêm với credential thật trong `~/.viderse/auth.json` (chỉ đọc, không in): `codex login status` → `Logged in using ChatGPT`, refresh_token rỗng. Ghi nhận: CLI **bắt buộc** có `id_token` và nó phải là JWT 3 đoạn — dùng access token (JWT ChatGPT) là hợp lệ; thiếu `id_token` → `missing field id_token`.
- Không plaintext: test pg đọc thẳng `key_enc` + `string_agg` toàn bộ cột text của 2 bảng, tìm chuỗi khoá và mảnh `VERYSECRET` → không thấy; API chỉ trả `keyHint` (test web khẳng định không có trường `keyEnc`).

## Lệch thiết kế / quyết định

- ~~Chưa nối vào `runServe`~~ — **đã nối** (xem mục "Bổ sung" cuối report): MachineSetup thêm block tools và dòng `wireAI(machineCtx, …)`; smoke 18792 đã chạy.
- `wire_ai.go` là file mới trong `cmd/viderse` (không nằm trong danh sách file sở hữu) — nếu integrator muốn gộp vào `serve.go` thì nội dung chuyển nguyên khối.
- **quota**: §15 không nêu nguồn; core/auth không có API hạn mức. Trường `quota` = ảnh trả phí đã charge hôm nay (biên nhận imagegen) so với trần D7 (`imagegen.DefaultDailyPaidImageLimit` = 50); `spendMonth` bỏ trống. Không bịa số.
- **models/check** với chatgpt/grok: đối chiếu danh mục thay vì gọi thật (một lượt thật tốn hạn mức gói; yêu cầu là "free only"). Với API key: OpenAI-compat `chat/completions` max_tokens=1; Anthropic `/v1/messages` max_tokens=1; Gemini `models.get`; Ollama `/api/show` + `/api/generate` num_predict=1.
- API key provider lỗi khi `test` trả `200 {ok:false,error:"auth_failed"|"unreachable"}` (không 422) — 422 chỉ dùng lúc tạo/đổi khoá.
- `POST /providers` trả 201 (hợp đồng không ghi mã). Thẻ chatgpt/grok: `POST` → 409 `managed`, `DELETE` → 409 `managed`.
- Không ghi dòng §11 cho login: `domain.TaskKind` chưa có loại đăng nhập (chỉ setup/…); bịa kind sẽ sai hợp đồng.
- Nhãn tài khoản (`account.email/plan`) tách từ `Credential.Label()` của core/auth.
- GET không chạm mạng: status thẻ đăng nhập suy từ `LocalStatus` + cờ `ai.revoked.<oauthId>`; cờ chỉ được ghi khi refresh bị từ chối (`invalid_grant`).

## Việc còn lại

Xem mục "Việc còn lại (không chặn)" ở cuối report. (Wiring + smoke đã xong; `internal/web/logs.go` của sibling đã được họ sửa.)

---

# Bổ sung (hoàn tất phạm vi)

## 1. Nối vào `runServe`

MachineSetup đã thêm block tools và **cả hai dòng** của tôi trong `cmd/viderse/serve.go`:

```go
aiSvc, err := wireAI(machineCtx, gateway, pool, settings, hub, statusSvc, cfgDir, toolKit, toolDeps.Runner)
```

`ChatGPT: aiSvc` được truyền vào `toolsvc.Deps` bên trong `mountMachine`. `go build ./cmd/viderse` xanh. Không còn việc nối dở.

## 2. `id_token` của auth.json tổng hợp — bằng chứng

Pinned: `codex-cli 0.162.1` (`codex login status` xác nhận). Nguồn: `openai/codex` (main), các tệp dưới đây.

- `codex-rs/login/src/token_data.rs`: `deserialize_id_token` **parse** id_token thành `IdTokenInfo` (`email`, `chatgpt_plan_type`, `chatgpt_user_id`, `chatgpt_account_id`, `chatgpt_account_is_fedramp`) từ claims `https://api.openai.com/auth` + `.../profile`; sai định dạng JWT ⇒ **cả auth.json không nạp được** (đó là lỗi `invalid ID token format` tôi gặp khi thử token không phải JWT). Hàm **không** kiểm `exp` của id_token — hết hạn không chặn nạp.
- `codex-rs/login/src/auth/manager.rs`, `AuthDotJson::from_external_access_token`: đường chính thức của CLI cho "chỉ có access token + account id" — nó **dựng `id_token` bằng `parse_chatgpt_jwt_claims(access_token)`**, ghi đè `chatgpt_account_id`, và đặt `refresh_token: String::new()`. Tức hình auth.json của ta trùng đúng hình CLI tự sinh cho auth ngoài, kể cả refresh token rỗng.
- Cùng tệp, `proactive_refresh_reason`: CLI chỉ refresh khi `parse_jwt_expiration(access_token) <= now + 5 phút` (`CHATGPT_ACCESS_TOKEN_REFRESH_WINDOW_MINUTES = 5`) hoặc `last_refresh` cũ > 8 ngày. Với refresh token rỗng, lần refresh đó sẽ thất bại ("Your access token could not be refreshed. Please log out and sign in again.").
- Vì vậy tôi thêm `cliRefreshWindow = 15 phút`: `CoreAuth.CLITokens` buộc `Manager.Invalidate` khi token còn dưới 15 phút, nên auth.json luôn có >5 phút dư và **CLI không bao giờ tự refresh trong một lượt vẽ**. (`core/auth` refresh sớm 5 phút; đây là biên an toàn thêm.)

Kết luận: dùng access token làm `id_token` là **đúng** — cùng một JWT, cùng claims, không verify chữ ký cục bộ, và chính CLI làm vậy cho auth ngoài. Không cần `id_token` riêng từ core/auth (kho credential của core/auth không lưu id_token). Không chạy `codex exec`/model call nào.

Rủi ro còn lại (đã đo, không phải phỏng đoán): nếu một lượt vẽ kéo dài quá thời hạn còn lại của access token (>15 phút) thì CLI sẽ thử refresh và báo lỗi rõ ràng — hàng ảnh thấy lỗi, không treo.

## 3. Kiến trúc & hiệu năng

**GET không chạm mạng, không spawn Codex**
- `GET /api/ai/providers`: `ListProviders` + `ListModels` mỗi thẻ + `LocalStatus` (đọc `auth.json`) + 1 truy vấn `Charged` cho mỗi thẻ gói. Không adapter, không token refresh, không CLI.
- `GET /api/ai/agents`: 1 lần đọc settings.
- `GET .../models`: đọc `ai_models` (index `ai_models_provider_idx` + PK `(provider_id,id)`), lọc `q`/`filter` phía server, cắt `limit`.
- `RefreshSubscribed` (Codex `debug models`, xAI `/models`) chỉ chạy **nền lúc khởi động** (context 90 s) và sau login; thất bại chỉ log + bump status.
- Login: một `loginMu` toàn app (`TryLock` → 409), luồng chạy nền có timeout 15 phút.

**Đo thật** (server thật trên 18792, Postgres thật, 4 thẻ; 200 request mỗi phép, sau warmup):
| Endpoint | p50 | p95 | max |
|---|---|---|---|
| `GET /api/ai/providers` | 3.14 ms | **6.19 ms** | 14.03 ms |
| `GET .../models?filter=all&limit=500` (catalog **500 model**) | 5.01 ms | **7.02 ms** | 12.56 ms |
| `GET .../models?q=bulk-49&filter=all` | 4.26 ms | **6.95 ms** | 10.78 ms |

Chứng minh không phụ thuộc provider khi đọc: **tắt cả Ollama và stub OpenAI-compat** rồi đo lại `GET /api/ai/providers` → vẫn 200, p50 5.16 ms / p95 8.29 ms, danh mục 500 model vẫn trả đủ 500.

`?limit=` mặc định 200, trần 500 (danh mục lớn nhất thật ~400). Hình wire không đổi (vẫn là `AiModel[]`).

## 4. Smoke thật (free) — port 18792

Môi trường: `createdb -O viderse_migrate viderse_smoke_aiconn`, `migrate up` (00001–00004 lúc chạy), `HOME=/tmp/viderse-aiconn/home`, `ollama serve` **do tôi tự khởi động** ở `127.0.0.1:11499` (11434 không chạy), stub OpenAI-compat cục bộ ở 18795. Không API-key provider, không model call trả phí.

| Bước | Kết quả |
|---|---|
| Setup admin + login | 200 |
| `POST /api/ai/providers` Ollama (baseUrl 11499) | 201 `{status:"ready", catalogCount:2}` |
| `GET .../models` | 200 `bge-m3:latest` (text), `qwen3-embedding:0.6b` (text) |
| `POST .../models/check {bge-m3:latest}` | 200 `{ok:true, ms:34, caps:["text"], contextK:8}` (Ollama 0.40.11 trả `capabilities:["embedding"]` nhưng `/api/tags` không có cờ đó; caps "text" là suy theo tên — xem ghi chú) |
| `POST .../models/check {khong-co-model-nay}` | 200 `{ok:false, error:"not_found"}` |
| compat provider + key | 201, `keyHint:"••••9999"` |
| `check text-only` / `check image-only` | `ok:true` / `{ok:false,error:"failed"}` |
| Gán Director = `text-only` | OK (sau khi PATCH `selected`) |
| Gán `artist.promptModel` = `text-only` | **422 `{reason:"capability_missing", need:"vision", field:"artist.promptModel", model:"text-only"}`** |
| `POST /api/ai/providers/grok/login {mode:"device"}` | 202 → phase `code` với `code:"EFPR-8ZC2"`, `verifyUrl:"https://accounts.x.ai/oauth2/device?user_code=EFPR-8ZC2"` (dừng, không hoàn tất) |
| `POST .../login/cancel` (body `{}`) | 200; phase `cancelled`; status thẻ `logged_out` |
| `GET /api/ai/providers` | ChatGPT `logged_out` (HOME tạm không có credential) |

Dọn dẹp: dừng đúng 3 tiến trình tôi khởi động (serve 18792, stub 18795, ollama 11499 — đã xác nhận không còn listen), `dropdb viderse_smoke_aiconn`, xoá `/tmp/viderse-aiconn`. Không chạm DB dev `viderse_go`, không chạm 8787/8790/8787-profile.

Ghi chú: máy chỉ có model Ollama loại embedding nên bước "gán vào Director" dùng provider `openai_compat` cục bộ (stub HTTP real, không phải mock trong mã sản phẩm) để có model text; Ollama thật vẫn được thêm/liệt kê/check.

**Một lỗi phân loại tìm thấy qua smoke và đã sửa**: `models/check` gán mọi lỗi không-auth thành `unreachable`; nay: `auth_failed` / `unreachable` (lỗi kết nối) / `not_found` (404) / `failed` (provider từ chối gọi, vd model không phải chat). Cũng đã sửa: caps rỗng do adapter khai (`[]` = "không làm được gì") không còn bị heuristic đoán đè thành `text`; và phase `code` xoá `authUrl` thừa.

## 5. Định dạng mã

`gofmt -l` trên toàn bộ tệp tôi sửa: sạch với **cả** `GOTOOLCHAIN=go1.26.0` và `go1.27.0`.

## Kiểm chứng cuối

- `go test ./internal/aiconn ./internal/secretbox ./internal/imagegen ./internal/persistence/postgres` (có `TEST_DATABASE_URL`) → ok.
- `go test -race -p 1 ./internal/aiconn ./internal/secretbox ./internal/imagegen` → ok.
- `go test -run TestAI ./internal/web` → ok. `go build ./cmd/viderse` + `go vet` → ok. `go tool sqlc diff` → sạch.
- `VIDERSE_CODEX_FORMAT_CHECK=1 go test -run TestCodexAuthJSONFormatAcceptedByCLI ./internal/tools` → PASS (codex-cli 0.162.1).

## Việc còn lại (không chặn)

- `spendMonth` của thẻ API key: không có API đọc chi tiêu; bỏ trống thay vì bịa.
- Ollama: bản 0.40.11 trả `capabilities` ở `/api/show` nhưng không ở `/api/tags`, nên danh mục hiển thị caps suy theo tên; bấm "Thêm theo tên" (`models/check`) mới có caps chính xác.
- Nếu sau này core/auth lưu `id_token` thật thì truyền thẳng vào `CodexAuthTokens` — hiện chưa cần và không có trong `Credential`.

---

# Sửa theo review Phase 02 (secrets, AI connections)

Nguồn: `reports/Review-p2-secrets.md`. Mỗi mục dưới đây là finding → thay đổi → chỗ kiểm.

## High

### H1 — khoá máy không còn bị thay khi khởi động lạnh song song
`internal/secretbox`: `key()` giờ **đọc → nếu thiếu thì lấy `flock` → đọc lại → sinh → ghi tệp tạm 0600 → rename**. Khoá đã có (kể cả do process khác vừa tạo) không bao giờ bị ghi đè; tệp sai kích thước là lỗi `ErrWrongKey` tường minh (không tạo lại ngầm).
Test: `TestConcurrentFirstUseKeepsOneKey` (8 Box cùng thư mục, mọi ciphertext đều mở được).

### H2 — bộ che bí mật luôn dừng
`internal/aiconn/adapters.go`: bỏ vòng `for` giữ tiền tố ("Bearer "/"sk-") — thay bằng **ba regexp chạy tuần tự** trên chuỗi đã cắt trần 300 ký tự: `Bearer <token>`, `sk-…`, và cặp `x-api-key|api_key|access_token|…: <value>`. Mỗi lượt thay bằng `***` nên không tạo khớp mới.
Test: `TestSanitizeTerminatesOnNestedMatches` (input 20k lần "Bearer sk-secret-value " — trả về trong thời gian tuyến tính), `TestSanitizeRedactsKeysAndHeaders`.

### H3 — luồng OAuth thật không còn tự chờ hết giờ; mã dán tay chạy đúng
- `internal/aiconn/auth.go` cầu `manual → auth.ManualSubmission` giờ có context riêng, **luôn bị huỷ và join** trước khi `Login` trả về; huỷ cả khi provider không nhận mã, nên không còn goroutine treo giữ `loginMu`.
- `core/auth/openai_codex.go` (sửa shared, nhỏ): `loginBrowser` đua callback với mã dán y như Anthropic (`waitCodexCode`), validate `state`. Trước đây `opts.Manual` bị bỏ qua ⇒ route `POST /api/ai/providers/chatgpt/login/code` treo vô hạn.
- `Logout` huỷ luồng đang chạy rồi mới lấy `loginMu` để xoá credential — không thể ghi đè ngược.
Smoke thật: `POST …/chatgpt/login` → 202 kèm `authUrl`; `POST …/login/code` → 200 (mã thô được nhận, đúng ngữ nghĩa Anthropic); `POST …/login/cancel` → 200, trạng thái `cancelled`.

### H5 — openai_compat (phần server)
Kind `openai_compat` đòi **cả** `apiKey` và `baseUrl`; `ollama` chỉ `baseUrl`; kind cố định gửi `baseUrl` → 400 `invalid_base_url`. Message cho SettingsWebB: form phải render/submit cả hai.
Smoke thật (201/400 `missing_key`/`missing_base_url`/`invalid_base_url`) — xem dưới.

### H6 — ModelRef có đường chạy thật
`wire_ai.go` thay `drawHolder` (map theo *kind*, gộp mất nguồn thứ hai) bằng `aiExecutors`:
- `imageProvider(ref)` dựng MỘT provider cho MỖI `ModelRef` (chatgpt→Codex, grok→Grok, openai/xai/compat→Images API, openrouter→chat completions, gemini→generateContent/predict), giữ nguyên thứ tự `artist.draw` + `drawFallback`.
- Ref không dựng được **không bị bỏ im lặng**: giữ đúng vị trí bằng `brokenProvider` báo lỗi nêu provider/model.
- `CanText`/`CanDraw` kiểm kind + model + credential; `PutAgents` từ chối 422 `no_executor` khi không có đường chạy.
- `wireAI` trả `aiWiring{Service, Executors}`; `Executors.Drawer()`/`Director()` là consumer thật cho pipeline (không còn holder chết).
Thêm `internal/imagegen/api_images.go`: provider Images API (OpenAI style), OpenRouter, Gemini/Imagen; `MissingKey` miễn phí, journal before draw, ffmpeg transcode như Grok.

## Medium

- **M1** `CLITokens` không còn nuốt lỗi refresh: refresh hỏng → lỗi; token sau refresh không đủ `20' gọi + 5' ngưỡng CLI + 1' chuẩn bị` → lỗi. Hằng `cliRefreshWindow = 26'`.
  Test: `TestCLITokensOnlyIssuesCLISafeTokens` (10' + refresh hỏng; 10' + refresh 2h; 10' + refresh 20'; 1h không refresh).
- **M2** `Options.Lifetime` + `Service.Go/Close`: login và warm catalog bám vòng đời serve; `serve.go` gọi `ai.Service.Close()` sau `machineStop()` trước khi đóng pool/hub.
  Test: `TestCloseCancelsInFlightLogin`.
- **M3** `cfgMu` bọc `PutAgents`, `PatchProvider`, `DeleteProvider`; `PatchProvider` ghi từng trường (`SetSelected` riêng, `UpdateProviderKey` riêng) nên hai tab không ghi đè nhau; Store bỏ `UpdateAiProvider` toàn dòng.
  Test: pg `TestAIConnFieldWritesDoNotClobber` (PATCH selected ∥ PATCH key).
- **M4** GET danh sách: `ListAiProviders`/`ListAiSubscribed` kèm `count(*)` trong một truy vấn; attention chỉ đọc 2 thẻ đăng nhập gói.
  Test: `TestAttentionReadsOnlySubscribedCards` (đếm `ListProviders` = 0).
- **M5** `Models(ctx, id, q, filter, cursor, limit)` trả `ModelPage{Models, Total, HasMore}`; lọc q/filter + keyset + limit **trong SQL**; `total` là tổng bộ lọc. Wire: `{models,total,nextCursor}` (cursor = id cuối trang).
  Test: `TestModelsPaginationKeepsOrderAndTotal`, web `TestAIModelsPaginationWire`.
- **M9** Ollama: capability lấy từ `/api/show` (`completion`/`vision`) ngay lúc lấy danh mục; model chỉ embedding có caps rỗng; chỉ khi `/api/show` không khai capabilities mới rơi về heuristic families.
  Test: `TestOllamaEmbeddingOnlyModelHasNoTextCap`, `TestOllamaShowWithoutCapabilitiesFallsBack`.
- **M10** paid smoke `imagegen/smoke_test.go` dùng `CoreAuth.CLITokens` thay vì home CLI.
- **M11** `TestProvider` trả lỗi khi `SetStatus`/`ReplaceCatalog` hỏng (không còn "xong" giả).
- **M12** `UpsertAiModel ... manual = ai_models.manual OR EXCLUDED.manual` — model thêm tay không bị hạ cấp khi lần refresh sau cũng trả nó.
  Test: pg `TestAIConnManualFlagSurvivesCatalogRefresh`.

## Low

- **L1** `ProviderView` không còn nhúng `domain.Provider` (không còn `EncKey` trong kiểu trả về); dựng qua `viewOf`. Web có wire riêng cho agents (`aiAgentsWire`) và trang model.
  Test: web `TestAIProviderWireNeverExposesCiphertext` (không có `keyEnc`/`encKey`/`apiKey`).
- **L2** bỏ `SanitizeLoginError`, interface `Tasks` không dùng, `keyHintOf` trùng; `Hint` của secretbox là bản duy nhất.

## SSRF / redirect
Ngoài allowlist baseUrl ở H5: client mang credential dùng `aiconn.NewCredentialClient` → `SameOriginRedirect` (cùng scheme+host, tối đa 5 lần, không hạ https→http) nên khoá không bị forward sang host khác qua redirect.
Test: `TestSameOriginRedirect`; `TestBaseURLPolicy` (userinfo/query/scheme sai bị từ chối, loopback/LAN được phép).

## Không thuộc phần này
H4/M6/M7/M8 (Bilibili, pause, lỗi nền tảng) là của `Platforms`; H5 phía UI (`AddProvider.tsx`) là của `SettingsWebB` — đã gửi hợp đồng.

## Kiểm chứng
```
go build ./...                                   # ok
go vet ./internal/aiconn ./internal/imagegen ./internal/secretbox ./internal/persistence/postgres   # sạch
go test -p 2 ./internal/aiconn ./internal/secretbox ./internal/imagegen ./internal/persistence/postgres   # ok
go test -race -p 1 ./internal/aiconn ./internal/secretbox ./internal/imagegen                     # ok
go test -run TestAI ./internal/web               # ok
go tool sqlc diff                                # sạch
gofmt -l (go1.26.0 và go1.27.0) trên các tệp đã sửa  # sạch
```

Smoke thật (cổng 18792, DB `viderse_smoke_aiconn2`, HOME tạm, stub OpenAI-compatible ở 18794, chỉ gọi local):
```
compat no key  → 400 {reason:missing_key}
compat no url  → 400 {reason:missing_base_url}
compat create  → 201 {status:ready, catalogCount:7, keyHint:"••••7c2e"}
GET models?limit=3 → 3 trang (3/3/1), total 7, nextCursor null ở trang cuối
PUT agents artist.promptModel=local-model-000 → 422 {reason:capability_missing, need:"vision", field:"artist.promptModel"}
POST provider openai + baseUrl → 400 {reason:invalid_base_url}
GET /api/ai/providers → 3 thẻ, không có ciphertext
```
Bí mật: `SELECT count(*) … encode(key_enc,'escape') LIKE '%VERYSECRET%'` → 0; `key_hint="••••7c2e"`, `length(key_enc)=51`; `grep -c 'VERYSECRET\|sk-local' serve.log` → 0; file tài khoản cũng 0.
Dọn: server + stub đã dừng, DB đã drop, thư mục tạm đã xoá.
