---
title: "Chuyển Noverse sang monorepo verse"
description: "Dời Noverse vào monorepo verse không mang lịch sử, tách lớp AI core có test ranh giới import, chạy live từ repo mới, dựng khung Viderse bằng Go, rồi đo và nâng cấp core qua eval."
status: done
priority: P1
branch: "main của datnt1998/verse (private, Actions đang tắt)"
tags: [monorepo, refactor, core, auth, eval, infra]
blockedBy: [pre-verse-hardening]
blocks: []
created: 2026-10-29
---

# Kết quả cần đạt

Repo `datnt1998/verse` (một Go module) có cấu trúc như sau:

- `apps/noverse` chạy y hệt bản `pre-verse`.
- `core/` chứa lớp AI dùng chung. Test ranh giới import chặn mọi phụ thuộc ngược.
- Live Noverse trên `127.0.0.1:8790` chạy từ repo mới mà không phải di chuyển dữ liệu.
- `apps/viderse` là khung Go+React chạy được một Writer agent thật.
- Eval đo được các tín hiệu của vòng lặp core. Mọi nâng cấp core đều có A/B với ngưỡng đặt trước.

Repo `noverse` được đóng băng làm tài liệu tham chiếu. Bản Viderse TS (`../viderse`) không bị đụng tới.

Sơ đồ [advise-verse-architecture.svg](../reports/advise-verse-architecture.svg) được vẽ trước khi đối chiếu với code. Chỗ nào sơ đồ khác plan này thì theo plan: `platform/settings`, cổng của Viderse, và SSO.

# Dữ kiện đã kiểm (2026-10-29)

| Dữ kiện | Bằng chứng |
|---|---|
| Tag `pre-verse` = `6686262` (chỉ có ở local). Sau đó `main` có thêm `917e65a` (chỉ sửa `web/`, không đụng `.go`, `assets/`, `evals/`). `origin/main` = `ab3aa77`, chậm hơn khoảng 94 commit. Repo `noverse` trên GitHub là private. Repo `datnt1998/verse` chưa có | `git log pre-verse..main`; `gh repo view` |
| Còn chưa commit: phần sửa `docs/design/**` của anh, `web/src/screens/audio/Transcript.regen.test.tsx` của phiên Codex, plan này và hai file sơ đồ. Những thứ này **không đi theo `git archive`** | `git status --short` |
| Có 2 file đang được theo dõi dù khớp `.gitignore`: `config/config.example.jsonc` và `config/rules/README.txt`. Ở repo mới, `git add -A` sẽ bỏ sót chúng | `git ls-files -ci --exclude-standard` |
| `auth/oauth_icons_test.go:18-22` đọc `../../docs/design/v1/icons`. Đây là phụ thuộc qua hệ thống file duy nhất từ core sang tài liệu của app | grep `runtime.Caller(0)` trong các package core |
| Tag `pre-verse` đang có hai file ghi chú dẫn xuất từ mã GPL: `plans/reports/xia-pagekin/01-prompt-pipeline.md` và `04-planning-import.md`. Repo dùng giấy phép Apache-2.0 | `git ls-tree pre-verse`; `LICENSE` |
| Bao đóng core gồm 32 package: `agent/**`, `llm/**`, `models`, `llmretry`, `llmcontract`, `auth`, `codex`, `anthropicoauth`, `xai`. Bao đóng này **không phụ thuộc package nào ngoài chính nó, kể cả trong test**. Có 20 package ngoài core import vào. Module có 89 package | `go list -deps` và `go list -test -deps` |
| `internal/agent` khai báo package `agentcore`, nên khi đặt ở `core/agentcore` thì tên thư mục khớp tên package | `internal/agent/agent.go:1` |
| Có hai chỗ giả định `go.mod` nằm ở gốc app. Cả hai sẽ hỏng khi dùng một module ở gốc `verse` | `internal/blobkey/blobkey_test.go:291-301`; `internal/entry/web/gentypes/main.go:58-74` |
| Cấu hình role nằm trong `internal/bootstrap`. Package này import `library`, `store`, `rules`, `notify`, `persistence/postgres`, nên không dời nguyên vào core được | `bootstrap/config.go:203-232,553-563`; `bootstrap/models.go` |
| Hiện không có package `settings` nào. Cấu hình nằm rải ở `entry/web/proxy.go`, `library.LoadServeConfig` và `bootstrap/storage.go` | glob, `serve.go:204` |
| `webauth` import `library` để dùng `ValidProjectID` và `LegacyProjectID` | `webauth/store.go:25,437`; `members.go:15` |
| Luật admin = owner nằm ở `webauth.User.ProjectRole`, áp cho cả backend file lẫn Postgres | `webauth/store.go:127-133` |
| Hai backend tài khoản lệch nhau ở hai điểm: so khớp username (Postgres dùng `citext`, file so khớp chính xác) và ngưỡng giới hạn đăng nhập (file: 5 lần, 30s→15m, lưu trong RAM; Postgres: 4 lần, 2s→10m, lưu bền) | `store.go:211-214`; `limiter.go:9-14`; `auth_limiter.go` |
| File tài khoản đang ở **v4**, không phải v3 | `webauth/store.go:34` |
| Cookie là `ainovel_session`, host-only, không có `Domain`, `SameSite=Strict`. Cookie không tách theo cổng, nên hai app cùng chạy trên `127.0.0.1` nhận cookie của nhau | `entry/web/auth.go:17,86-92` |
| Kiểm Origin/`Sec-Fetch-Site` chỉ chạy khi có `PUBLIC_URL`. PATCH và DELETE không bị kiểm Content-Type. Khi `TRUSTED_PROXIES` rỗng, luật loopback mất, trái với comment | `entry/web/proxy.go:22-30,235-258`; `gateway.go:614-616` |
| Live chạy file mode trên `127.0.0.1:8790`, không có `tailscale serve`, không có launchd. Process do một phiên omp giữ (cha là `omp __omp_worker_daemon_broker`). Script `bin/run-live.sh` không được git theo dõi (ẩn bằng `.git/info/exclude`) | `ps`; `tailscale serve status`; `git check-ignore` |
| Worktree `noverse-live` ở `917e65a` (Codex deploy ngày 2026-10-29, đã nằm trong `main`). File `.git` của nó trỏ tuyệt đối vào `ainovel-cli/.git` | `git worktree list` |
| Mọi worktree và nhánh phụ đều đã merge vào `main` | `git branch --no-merged main` rỗng |
| Viderse TS có khoảng 111k dòng mã nguồn, 57k dòng test, 11 module agent và 321 commit. Commit cuối ngày 2026-10-24. Trước đó repo không có remote; ngày 2026-10-29 đã đẩy cả 12 nhánh lên repo private `datnt1998/viderse`, sau khi quét secret trên toàn bộ lịch sử bằng regex (0 kết quả) | `gh repo view datnt1998/viderse` |
| Quyết định 0016 của Viderse cấm nhúng harness AI và cấm dùng lại OAuth của gói subscription | `../viderse/docs/decisions/0016-khong-nhung-ai-mo-mcp.md` |
| Eval đã có: `tool_errors` (một số cho cả run), guard lặp lỗi tool (`agents/guard/repeat_failure.go`, ngưỡng 2), circuit breaker (`agent/loop.go:875-884`), `StopAfterToolResult`/`TerminalResult`. Eval chưa có: tách lỗi theo loại; đếm retry, stream idle, compaction ngoài `novel_context`; trạng thái kết thúc subagent; thời lượng run. Code chưa có `submit_result`/`OutputSchema` và chưa có lint schema tĩnh | `internal/eval/collect.go:43-60`; grep |
| Smoke `pre-verse` qua 3/3 trong 67 phút, tốn $1,49, có 3 lỗi tool tự hồi phục, chạy với `--timeout 60m` | run `20261029-101116` |
| CI chỉ chạy khi mở PR hoặc chạy tay | `.github/workflows/ci.yml:3-8` |

# Đối chiếu quyết định đã chốt

| Quyết định | Dữ kiện mới | Kết luận |
|---|---|---|
| Repo mới `verse` không mang lịch sử; commit đầu là `git archive pre-verse` | Phần sửa chưa commit không đi theo archive. Có 2 file dẫn xuất GPL | **Giữ.** Thêm phase 00: chốt phần sửa, gắn lại tag, push, chạy CI. Loại 2 file GPL khỏi archive |
| Module `github.com/datnt1998/verse`, một `go.mod` ở gốc | Hai chỗ code cần neo lại. `docs/design/v1/22-brief-2510.md:912` và `docs/prd/toi-uu-ainovel-cli-tieng-viet.md:240` ghi là "giữ module path" | **Giữ.** Neo lại 2 chỗ ở M1. README gốc ghi rõ quyết định mới thay quyết định cũ. Không sửa tài liệu thiết kế |
| `core/{agentcore,llmlite,roles,auth}` | Bao đóng sạch, nên M2 chỉ cần dời thư mục và đổi import. Đổi package `llm` thành `llmlite` sẽ phải sửa định danh `llm.X` ở 20 package. `roles` không dời nguyên được | **Hỏi (Q5).** Đề xuất `core/llm`, giữ tên package. Tách `core/roles` ở M4, khi Viderse là người dùng thứ hai |
| OAuth nằm trong core | `auth` có chuỗi runtime riêng của Noverse (`oauth.go:161-164,276-277`, `claudecode.go:21`, `store.go:86`) | **Giữ.** M2 dời nguyên. M4 thêm tham số app: tên lệnh login, đường dẫn kho, tên hiển thị |
| `platform/{identity,settings,notify,logger}` | Chưa có package `settings`. `webauth` phụ thuộc `library`. `pgtest`/`migrate` gắn tên `noverse_*` | **Đổi.** Chỉ dời sang platform thứ mà app thứ hai cần (M4). Bỏ `settings` cho tới khi có code để dời |
| SSO trong MVP: cookie `verse_session`, `grants.json`, admin nền tảng ngầm là admin mọi app | Trái với luật MVP của anh "không liên kết data các app sớm". Đổi tên cookie làm mọi thiết bị phải đăng nhập lại. Cookie dùng chung tên nhưng mỗi app một kho phiên thì các app trên `127.0.0.1` sẽ ghi đè phiên của nhau | **Hỏi (Q6).** Đề xuất M4 dùng chung code identity nhưng mỗi app một kho và một cookie riêng. SSO tách thành bước 4d, chỉ làm khi anh chốt |
| Migrate `web-accounts.json` v3 | File thật đang ở v4 | **Sửa dữ kiện:** v4 |
| Lưu identity: file hay Postgres | Facade `webauth.Store` đã có cả hai backend. Hai backend lệch nhau ở hoa/thường username và ngưỡng limiter | **Chỉ cần chốt khi làm SSO.** Trước SSO, mỗi app dùng backend hiện có của nó |
| DB mỗi app: schema riêng hay database riêng | Migration Noverse gắn cứng `noverse_meta` và role `noverse_migrate`/`noverse_app` | **Đề xuất (Q8): database riêng mỗi app trên cùng một cụm Postgres.** Noverse không phải đổi gì. Viderse Go dùng Postgres; bỏ gợi ý SQLite |
| M3 deploy live từ repo mới | Tên cookie, `~/.ainovel`, `$HOME/NoverseLive` giữ nguyên, nên không phải di chuyển dữ liệu. Script live chưa được git theo dõi. Process phụ thuộc phiên omp | **Giữ.** Đưa script live vào repo `verse`. Cách giữ process: Q9 |
| M4: Viderse dựng lại bằng Go+React, một Writer agent | Bản TS lớn và không có remote. 0016 cấm cách làm core Go đang dùng. Remotion không có bản Go | **Giữ ở mức khung + Writer.** Không thay bản TS. Cần anh xác nhận thay 0016 cho bản Go (Q7) |
| Kiểm Origin ở mọi nơi | Local mode chưa kiểm. PATCH/DELETE chưa kiểm Content-Type. Comment `TRUSTED_PROXIES` sai | **Giữ, làm ở 4a.** Không phụ thuộc quyết định SSO |
| M5 đưa metric core vào eval | Như bảng dữ kiện | **Giữ** |
| M6: guard chặn gọi tool lặp | Guard đã có trong Noverse | **Đổi:** đưa guard lên core và đo, không viết mới |
| Không import hoặc cutover live sang Postgres trước khi xong H10–H13 | Phase 09 của plan `261028-1340-db-vps-storage` vẫn đang làm | **Giữ.** Plan này không đụng hosted, chỉ sửa đường dẫn build |

# Điều kiện trước khi bắt đầu

- Anh trả lời các câu hỏi chặn phase 00 và 01: Q1–Q4.
- Phase 00 đạt gate. CI xanh trên GitHub tại commit được gắn tag.
- Không cutover hosted sang Postgres trong suốt plan này.

# Cách chạy gate trên máy (anh yêu cầu 2026-10-29)

CI đang tắt để giữ quota, nên mọi gate chạy trên máy anh (10 nhân, 16 GB). Để máy không lag và không tràn RAM:

- Mỗi lúc chỉ chạy **một** việc nặng: `go build/vet/test`, `-race`, `npm`/vitest, `docker build`. Khi có nhiều agent cùng làm, mọi lệnh nặng đi qua khoá chung `lockf -k /tmp/verse-heavy.lock <lệnh>` để không bao giờ chạy song song; agent con dùng thêm `GOMAXPROCS=2 -p 1`. Eval smoke không tính là việc nặng: process chủ yếu chờ model qua mạng (đo lúc chạy: 0% CPU, khoảng 10 MB RSS), nên được chạy cùng một việc nặng khác.
- `go test`: `nice -n 10`, `GOMAXPROCS=4`, `-p 2`. Riêng `-race` dùng `-p 1`.
- Web: `npm test -- --maxWorkers=2`.
- `gofmt`: chỉ chạy trên các file đã đổi, không quét cả cây.
- Postgres/MinIO test (`verifycheck`) chỉ bật khi chạy test cần chúng; xong là `docker compose -p verifycheck down -v`.

# Các giai đoạn

| Giai đoạn | Kết quả | Chặn bởi | Trạng thái |
|---|---|---|---|
| 00 [Chốt baseline và đẩy lên GitHub](./phase-00-baseline-push.md) | Tree sạch, tag ở commit cuối, gate xanh (CI cuối ở `c49b37d`, gate tương đương chạy trên máy ở `04aee55`) | — | xong |
| 01 [Dời Noverse vào `apps/noverse`](./phase-01-move-noverse.md) | Repo `verse` qua toàn bộ gate cũ; số test không đổi | 00 | xong (CI hoãn, Actions tắt) |
| 02 [Tách `core/` và test ranh giới](./phase-02-carve-core.md) | 32 package nằm ở `core/`; test ranh giới có kiểm chứng đối chứng; smoke eval 3/3 | 01 | xong |
| 03 [Live chạy từ `verse`](./phase-03-live-from-verse.md) | Cổng 8790 chạy binary build từ `verse` dưới launchd; phiên đăng nhập cũ còn hiệu lực; rollback dưới 1 phút | 02 | Xong. Dữ liệu đã kiểm: binary cũ và mới đọc ra giống hệt. Anh đã xác nhận phiên cũ trên thiết bị. Ngày 2026-10-29 anh bỏ mốc 7 ngày: live đã dựng lại từ `main` và live cũ đã gỡ. Quay về một commit `verse` vẫn nhanh, còn quay về repo cũ thì phải dựng lại worktree |
| 04 [An toàn request, identity, khung Viderse](./phase-04-security-identity-viderse.md) | 4a an toàn request; 4b identity dùng chung code; 4c Viderse + Writer chạy thật. Không làm 4d (SSO) | 03 | xong. Smoke của 4b chỉ có `architect_long` ×3 (anh cắt eval) |
| 05 [Metric core trong eval](./phase-05-core-metrics.md) | Report có các trường metric mới; baseline có số đo độ dao động | 02 | xong; baseline rút gọn còn `architect_long` ×3 theo quyết định của anh. Baseline lộ một lỗi đo retry trước stream, đã sửa |
| 06 [Nâng cấp core có A/B](./phase-06-core-upgrades.md) | Mỗi nâng cấp có giả thuyết, ngưỡng đặt trước và kết quả A/B | 05 | 2 nâng cấp đã merge **không qua A/B**, theo quyết định của anh; có test thay thế, ghi trong file A/B |

Phase 05 làm song song được với 03 và 04. Phạm vi của 05: `core/agentcore` (phát sự kiện) và `apps/noverse/internal/{eval,store,agents,host}` (cầu nối và collector). Bước 4b cũng sửa `agents` (dời `ParseThinkingLevel`) và `bootstrap`. Với mỗi file mà hai phase cùng sửa, chỉ một người được giữ file đó tại mỗi thời điểm.

# Review độc lập (2026-10-29)

Một reviewer độc lập đã đối chiếu 27 anchor với mã nguồn; các anchor đều đúng. Reviewer tìm ra 15 lỗi trong plan, 10 lỗi mức High. Tất cả đã được sửa trong các file phase:

- thiếu `MINIO_TEST_ENDPOINT`;
- push tag trước khi CI xanh;
- Dockerfile thiếu đích copy `dist` và đích build;
- thiếu `.github/scripts`;
- test icon OAuth đọc file của app;
- script live neo sai thư mục;
- rollback dưới launchd;
- smoke gọi nhầm binary trong `PATH`;
- thiếu cầu nối sự kiện vào session log;
- `RoleConfig` thiếu `Provider`;
- gate CI không gắn với run id;
- so khớp tuyệt đối danh sách test sau khi đã thêm test;
- phản ví dụ ranh giới bị luật `internal/` của Go chặn trước;
- quyền file sao lưu;
- `SECURE_COOKIES` thiếu `PUBLIC_URL`.

# Điều kiện hoàn thành

- Mọi gate của từng phase ghi lại lệnh đã chạy và kết quả ngay trong file phase.
- Test ranh giới import có trong `go test ./...` của CI và fail với fixture vi phạm.
- Live 8790 chạy từ `verse` ít nhất 7 ngày không phải rollback. Sau đó repo `noverse` mới được đóng băng (README ghi chú, không push thêm).

# Rủi ro

- **Phần sửa dở bị bỏ lại.** Archive chỉ lấy những gì đã commit. Cách giảm: gate phase 00 yêu cầu tree sạch.
- **Đổi import ở 737 file làm rơi file.** Cách giảm: đo số file Go còn chuỗi `datnt1998/noverse` (phải bằng 0); so số test pass trước và sau.
- **Live phụ thuộc phiên omp.** Phiên đó kết thúc thì live chết, trước hay sau M3 đều vậy. Xem Q9.
- **Chi phí eval.** Mỗi smoke khoảng $1,5 và 67 phút. A/B với N=3 trên 17 case khoảng $3,1 mỗi nhánh (baseline cũ). M6 phải giới hạn số case cho mỗi nâng cấp.
- **Viderse TS không có remote.** Ngoài phạm vi plan, nhưng mất máy là mất hết. Xem Q10.

# Quyết định đã chốt — 2026-10-29

Anh duyệt toàn bộ đề xuất.

| # | Quyết định | Áp dụng ở |
|---|---|---|
| Q1 | Chỉ archive sau khi phần sửa `docs/design/**` của anh và `Transcript.regen.test.tsx` của phiên Codex đã được commit (hoặc bỏ). `c4` và `main.go` ở gốc không đi theo | 00 |
| Q2 | Push `main`, chạy CI bằng tay. CI xanh mới gắn và push tag `pre-verse` | 00 |
| Q3 | Tạo repo private `datnt1998/verse`; module `github.com/datnt1998/verse` | 01 |
| Q4 | Bỏ 2 file ghi chú dẫn xuất GPL (`xia-pagekin/01-prompt-pipeline.md`, `04-planning-import.md`); giữ phần còn lại của `xia-pagekin` | 01 |
| Q5 | Dùng `core/llm` và giữ tên package `llm`, không dùng `llmlite`. Tách `core/roles` ở M4 | 02, 04 |
| Q6 | Không làm SSO trong MVP. Ở M4, mỗi app có kho tài khoản và cookie riêng, chỉ dùng chung code identity. Bước 4d không làm | 04 |
| Q7 | Viderse bản Go thay quyết định 0016: nhúng agent core và dùng OAuth subscription. Có đăng nhập, dùng code identity chung | 04 |
| Q8 | Mỗi app một database trên cùng cụm Postgres. Viderse Go dùng DB `viderse_go` | 04 |
| Q9 | Live chạy dưới launchd | 03 |
| Q10 | Đặt remote private cho Viderse TS ngay. Câu hỏi Soverse (làm gì, gồm những agent nào) vẫn để ngỏ, nằm ngoài plan | ngoài plan |
