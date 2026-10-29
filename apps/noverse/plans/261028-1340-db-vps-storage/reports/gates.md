# Red-team và validate gates — 2026-10-28

## Phương thức và giới hạn bằng chứng

Yêu cầu `--hard`: scout sâu, research chỉ Dokploy/goose-pgx/S3/GCS, red-team và validate tự chạy. Runtime có glob/grep/read/edit/write/bash/wait, không có spawn/delegate/task/ask-user surface. Vì vậy reviewer là **cùng planner, bốn lens riêng**, không giả danh bốn subagent hoặc nói đã hỏi kongming. Không có design fork còn bất định cần advisory: hai backend facade, typed relational facts, immutable blobs/fencing, một web replica là phương án đã chốt có lý do ở plan. Gate này xét **kế hoạch**, không chạy implementation tests/build/lint.

Đã đọc skill refs output-standards/plan-organization/planning-rules/cli-integration/validate-workflow/red-team-workflow, verification-roles, red-team-personas và question-framework. `ak plan create` + `add-phase` tạo plan/index+9 files, từng stub đã được đọc trước replacement. Script activation tìm thấy tại `.agentkit/adapters/omp/engineer/sidecars/scripts/set-active-plan.cjs` nhưng node trả thiếu module `ak-prefs-client.cjs`; không giả active state thành công. Không git; branch field khai báo UNVERIFIED.

## Red Team Review

### Findings: 8 (Accept8 / Reject0)

2 Critical,5 High,1 Medium. Mọi finding có codebase evidence, không dựa tên file. Findings được self-adjudicate theo yêu cầu làm gate, không ghi người dùng đã approve. Không có phát hiện nào đòi giảm requested scope.

| # | Lens / verification role | Severity | Failure scenario và evidence | Disposition / mitigation / phase |
|---|---|---|---|---|
| 1 | Assumption / Fact + Scope | Critical | Chỉ “migrate users” làm mất admin create/update/temp/grants/revoke hoặc đổi role/hash sai. Source hiện global admin/member, Projects owner/collab/view (`internal/webauth/store.go:39-41,63-119`), Argon2id (`internal/webauth/password.go:14-74`), admin handler guards (`internal/entry/web/authorization.go:21-22`), temp/password API (`internal/entry/web/auth.go:402-465`) | Accept. Account schema đầy đủ+same-factory parity04, import version3+legacy08; giữ Argon2id, username2 chars, HTTP fields/errors, onboarding/recovery/setup; one-time TTL bổ sung không mất lifecycle |
| 2 | Failure / Flow tracer | Critical | SQL ở gateway nhưng child Host tự NewStore (`internal/host/host.go:175`), reader tự Store (`internal/host/storyread.go:35,518`), editor (`internal/tools/edit_chapter.go:35`) và comments hook (`internal/tools/comments_hook.go:14`) vẫn dùng filesystem | Accept.05 typed repos,06 injected Host/readers,14 reader+8 direct constructors, no-library/read-only-cwd gate; inventory direct+hidden IO |
| 3 | Security / Contract | High | Partial unique owner chỉ chặn2 owner, không chặn0; hai admin hạ quyền trong DB transactions độc lập có thể đều thấy admin khác. Existing file code countAdmin/update guards (`internal/webauth/store.go:603-700`) dựa file write lock, không tự chuyển thành PG invariant | Accept.04 singleton lock+deferred admin constraint; project row lock+deferred exact-one-owner and unique partial; source user disable/soft-delete must transfer first; race tests SQL, không chỉ Go mutex |
| 4 | Security / Fact + Flow | High | `secureRequest` tin arbitrary X-Forwarded-Proto (`internal/entry/web/auth.go:63-64`), `clientIP` chỉ remoteAddr (`:54-59`), route non-api vào static (`gateway.go:482-488`); Docker behind Traefik limiter trộn mọi user hoặc spoof scheme, `/healthz` trả SPA200 | Accept.09 trusted-CIDR forwarding normalization, canonical PUBLIC_URL, hosted Secure mandatory, health before SPA, no public8080;04 DB rate-limit buckets survive restart |
| 5 | Failure / Scope | High | Queue file restart resets running (`internal/audio/queue.go:57-79`); shared audioServices (`internal/entry/web/audio.go:94,125-189`) không thể nhận request actor. Mac worker offline/late ACK có thể chọn audio sai version nếu chỉ đánh done qua callback | Accept.07 durable jobs+lease/fence+source/hash snapshot+current permission check; separate private worker mux, metadata-only shared cache; actual crash/stale-worker tests |
| 6 | Assumption / Contract | High | Current export cache uses size/mtime (`internal/audio/export_result_cache.go:34-50`), local files-based GET (`internal/entry/web/audio.go:1349`); S3 ETag/hash hoặc presign “works everywhere” sẽ bỏ sót same-size corruption, private endpoints và GCS trailers | Accept.03 same conformance local+MinIO, independent download SHA-256, multipart abort, recipe one SDK BaseEndpoint/UsePathStyle, GCS HMAC/checksum compatibility+authenticated streaming fallback; provider production smoke chưa giả pass |
| 7 | Failure / Flow | High | Sau DB writes mới, quay source file snapshot sẽ mất chương/grants/job state; current file authority chapter record Save overwrites latest (`internal/store/chapter_records.go:41-45`) và library writes (`internal/library/files.go:135-139`) không thể tự biết DB delta | Accept.05 immutable history cả file mode,08 reverse-export+roundtrip+restore trước09; rollback app DB-compatible hoặc freeze+latest export, no goose down/no stale source fallback |
| 8 | Scope / Contract verifier | Medium | Mirror “Dir() over SQL” để giữ tools dễ tạo hai authority, lặp file layout trong database. Current facade has19 typed substore (`internal/store/store.go:19-37`) and `.Dir()` file dependency (`internal/tools/edit_chapter.go:35`), không cần thêm general virtual filesystem | Accept.02 facade+typed contracts/two real backends,05 relational tables/keyed artifacts,06 explicit file-only capability/scratch; no Redis/Caddy/SQLite/dual-write/OAuth implementation outside scope |

### Decision deltas và Whole-Plan Consistency Sweep

1. Identity contract là **admin/member + owner/collab/view + Argon2id**, không bcrypt/global collaborator; hash recovery giữ Argon2id/mask, không convert khi thiếu plaintext.
2. UUID user cấp deterministic từ source+username; source không có UUID user. Username length2, status API disabled mapped SQL locked, giữ metadata/onboarding.
3. Exact-one-owner không chỉ partial unique; at-least-one-active-admin cần serialized mutation, không count race.
4. Engine child tại gateway715, Host.New175, reader35;14 reader calls plus one WithStore factory branch,8 direct constructor set; pure prose matching thay filesystem editor.
5. Actual comments `.ainovel/gop-y/<n>.json` + `_events.json`, actual statuses pending/sent/fixed/lost/unsure/done; audio `vai.json`, `ban-doc`, `chuong`, `ung-vien`, cache flat synth-key.
6. Blob DDL thuộc00002 sau projects, không standalone trước parent; keyed artifacts/story_leases trong00003; workers/jobs/audio00004, import ledger00005.
7. Post-write rollback needs latest data reverse-export; file history new version required; session invalidation explicitly notified, no fabricated missing historic revisions.
8. One public mux8080/Traefik + private worker8081 tailnet; Secure/proxy health order; bare S3 SDK metadata ETag not integrity proof; physical backup key/blob manifests required.

Sweep bằng ranged reads toàn bộ index,9 phase và reports; grep tìm các giá trị cũ, sửa body/DDL/commands đồng thời. Mentions các giá trị cũ còn lại chỉ dưới dạng **cấm/cảnh báo sai lầm**, không chỉ thị thực thi. New API/test/table identifiers đều marked proposed/deliverables. Không tạo migration00002_blobs riêng; không ghi unauthenticated `/api/me` curl200; backup commands dùng PGSERVICE không secret DSN argv. Technical contradictions remaining:0 known; unconfirmed user assumptions được giữ riêng, không gọi “confirmed”.

## Validation Log

### Tier: Full; bốn vai trò tự thực hiện

| Vai trò | Kiểm chứng thực tế | Kết quả kế hoạch |
|---|---|---|
| Fact Checker | Glob source/test files; re-grep Store/Host/reader/gateway/auth/audio/worker constructors và direct os calls; read current account format/hash/permission functions; official docs reads | Existing evidence cited; new files/symbols labelled proposed. Branch unverified due no-git request. Không invent claim-count hoặc số test pass |
| Flow Tracer | serve.go→NewGateway→ServeHTTP guards→handler; login→Argon2→hash recheck→session; launchWorker→RunWorker→Host.New→store; tools→ProseEdit/comments/ledger; audioService→reader+NewService→queue; revision Scan→accept | Failure points named with actual control flow in source inventory §1; ordering preserved, health placed before static |
| Scope Auditor | Gateway created serve.go65, Host per-run run_worker.go36, shared audioServices sync.Map94, account store process-owned149, factories/pools shared vs actor scoped | No request actor on shared store/cache; service-only clients share, tx/request/run scopes explicit; source snapshot/file mode isolated |
| Contract Verifier |19 facade fields,5 hosted web direct NewStore calls +3 host calls,14 reader calls, NewGateway test constructor unchanged, account HTTP JSON/routes/roles/errors, requested8 storage env keys | Enumerated caller set; no universal rename; DDL/API same names; local constructor remains offline API; account JSON4 và project format3 được phân biệt |

### Phase gate checklist

Mỗi dòng xác nhận **plan contains executable requirement**, không nói code đã thực hiện.

| Phase | Dataflow / deps / owner | Fail-before unit/integration/E2E | Risks+rollback+observable done | Commands / source evidence |
|---|---|---|---|---|
|01 | Source→manifest→fixtures; no blocker; Storage lead | Layout/accounts/authority fixtures | Complete path classification and safe fixture | go test store/webauth/library/web, Store19 fields and schema source |
|02 | Config→factory→scoped facade→backend;01; Repository owner | Factory/tx/CAS/migration lock | No fake SQL filesystem, pool/lifetime safety | Explicit tests + storage migrate up/status, pgx/goose research |
|03 | Stream→hash→local/multipart→download/presign;02; Blob owner | One conformance both local+MinIO; copy corruption/resume | Immutable keys/orphans/private fallback | docker compose+conformance/copy commands; real cache/download callers |
|04 | Guard→validate/hash→DB tx+audit→session;02/03; Identity owner | Lifecycle parity + concurrency admin/owner + permission matrix | Temp one-time/role/isolation preserved | Real auth/member/recovery symbols + DB/HTTP tests |
|05 | Typed input→relational facts/history/events;04; Domain owner |19 domains, immutable/CAS/FK/search/reanchor | No metadata omitted or cross-tenant pointer | Store/comments/library/web test commands; record/outline facts |
|06 | Backend story identity→lease→Host/tools→fenced tx;05; Engine owner | No-library engine roundtrip and lost-lease cases | Hidden IO removed, no literary behavior changes | tools/revision/host/web and race commands; callsite inventory |
|07 | Enqueue→claim→Mac→blob→fenced ACK→jobs UI;03/06; Audio owner | Crash/reclaim/cancel/stale and actual media/VieNeu | Tailnet creds, resource bounds, offline degradation | Existing audio integration + new worker/DB tests |
|08 | Snapshot→decode→manifest→import→reconcile→restore/export;04–07; Migration owner | Twice/interrupt/source drift/roundtrip | No source overwrite/no stale rollback | Explicit import twice, reconcile, export, pg_dump/restore |
|09 | Traefik→trusted proxy→app→PG/blob and private worker;08; Deployment owner | Proxy/health/container/ports/browser/restore | Secure settings, actual backup and cutover | Clean Docker/web/go/e2e/curl commands, current Dockerfile/Vite/embed |

### Critical questions / recorded decisions

0 questions actually sent; no ask-user capability. Không giả custom input/answers.

| Chủ đề | Phương án | Trạng thái / rationale |
|---|---|---|
| Database | Postgres primary (selected theo shared decision); SQLite rejected | Locked decision, env DSN makes ops adjustable |
| Worker | Mac pulling over Tailscale (assumed); alternate CPU host with same API | **Giả định — anh xác nhận**, no VPS GPU requirement |
| Offline | Two backends (assumed); dropping file requires later user choice | **Giả định — anh xác nhận**, maintenance cost explicit |
| Blob | local or one S3 SDK driver; provider endpoint/region recipes | Locked scope; production provider credentials unavailable, conformance commands defined |
| Row security | Explicit scoped repo/IDOR matrix/composite FK; RLS considered not selected now | Documented trade-off avoids pool actor state and system-job policy ambiguity, no claim FK=enforced authorization |
| Auth compatibility | Keep roles/hash/endpoints/errors/user lifecycle; add TTL/one-use and DB races protection | Grounded in latest code, not user interview confirmation |
| Rollout | All domains staged, production authority switches only after08/09 | No live hybrid/dual-write rollback ambiguity |

### Recommendation

Plan deliverable meets requested scope and is ready for review/implementation handoff, **not deployed/tested**. Before production, confirm3 assumptions and obtain operational DSNs/hostnames/bucket/HMAC/Tailscale ACL, then main verifier executes listed tests once after implementation. No known unresolved architectural contradiction. Independent reviewer gate remains not exercised because runtime lacks delegation; do not misreport as done by separate agents.
