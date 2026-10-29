---
phase: 1
title: "Nền móng, dọn Writer, shell theo design"
status: completed
priority: P1
effort: 32h
dependsOn: []
---

# Phase 01 · Nền móng, dọn Writer, shell theo design

## Mục tiêu

- Viderse sạch Writer, có schema nền mới.
- Có kênh sự kiện SSE dùng chung với Noverse.
- Brand và kit khớp design. Shell, route và các trạng thái hệ thống đúng artboard.
- Có công cụ đo ngân sách hiệu năng.
- Chưa có tính năng nghiệp vụ; các màn hiện EmptyState theo design.

Gói design: **V0** (phần FE và nền), **V12** (hộp thoại, trạng thái hệ thống), hỏi đáp #1–#4.

## Đầu vào

- Design:
  - `02-stack-verse.md` §2–§4;
  - `05-kit.md`, `kit.css`, `tokens.css`, `03-tokens.md`;
  - `04-components.md` (các mục nhãn V0 và V12), `06-copy.md`;
  - artboard `Brand`, `Kit`, `MKit`, `Dialogs`, `System`.
- Code hiện có:
  - `apps/viderse/web` (khung setup, login, shell);
  - `platform/ui`, `platform/webapp`;
  - `apps/noverse/internal/entry/web/events.go`, `apps/noverse/web/src/data/{events,cache,keys}.ts`.

## Việc

### 1. Gỡ Writer (anh đã chốt)

1. Sao lưu DB: `pg_dump -Fc viderse_go` vào `~/Backups/viderse_go-<ts>.dump`.
2. Xoá rồi tạo lại database `viderse_go` theo README của app (hai role `viderse_app`/`viderse_migrate`). Trong DB chỉ có 2 run Writer chạy thử.
3. Xoá code:
   - `apps/viderse/internal/agent/`, `apps/viderse/internal/script/`;
   - `apps/viderse/assets/` (prompt của Writer);
   - `internal/persistence/postgres/queries/scripts.sql` cùng code sqlc sinh ra;
   - migration `00001_scripts_runs.sql`;
   - route `/api/runs` và `/api/scripts` trong `internal/web/gateway.go`;
   - `WriterRole` và model mặc định trong `internal/bootstrap`.
4. Giữ lệnh `viderse login`, vì chữ đi qua OAuth Codex của `core/auth`.
5. Migration mới bắt đầu từ `00001_base.sql`: bảng `settings` và khung `background_tasks`, `notifications` (§11). Các bảng nghiệp vụ thêm dần theo từng phase, mỗi phase một migration.
6. Sửa `apps/viderse/README.md` và README gốc: bỏ phần Writer, mô tả Viderse mới.

### 2. Sự kiện dùng chung

- **Go:** chuyển phần hub chung của Noverse sang `platform/events`: ring 500 sự kiện, id tăng dần, đọc `Last-Event-ID` hoặc `?lastEventId=`, gửi `reset` khi bị hụt, `hello`, nhịp giữ kết nối. Noverse giữ phần sinh sự kiện, và chuyển sang dùng `platform/events` ngay trong commit này.
- **Web:** chuyển `cache.ts`, `keys.ts` và phần lõi kết nối của `events.ts` (`EventSource`, dự phòng bằng fetch stream, backoff 1/2/4/5 s, `lastEventId`) sang `@noverse/ui/data`.
  - Subpath mới, không có kiểu dữ liệu của app nào.
  - Bảng ánh xạ sự kiện → khoá cache vẫn ở từng app.
  - Noverse đổi import trong cùng commit.
- **Viderse:**
  - `GET /api/events` (SSE) phát sự kiện `{ type, rev, ref }`;
  - `GET /api/status` (§5) cho lần tải đầu, có ETag;
  - client tự poll 3 giây chỉ khi SSE mất.

### 3. Brand, kit, hỏi đáp #1–#4

- `defineBrand` theo 02 §2: `primary #FF4D63`, các màu trạng thái, `accentDeep #E6002E`.
- Logo ống kính, favicon và icon app lấy từ `docs/design/v1/brand/svg` (12-brand).
- Kit thêm 3 thứ, đều chỉ thêm, không đổi gì đang có:
  - `Button variant="quiet"`;
  - `Banner tone="agent"` (nhận `agent` của `AgentOrb`);
  - `toast()` nhận nội dung dạng thẻ 300px (tiêu đề video, đúng một nút việc), `role=alert`, ở lại tới khi người dùng đóng.
- Ghi câu trả lời #1–#4 và các mục #8–#11 vào `11-hoi-dap.md` (xem plan.md).

### 4. Shell và route

- `router.ts` có kiểu, theo bảng 02 §4: `#/`, `#/review`, `#/queue`, `#/channels/<id>`, `#/channels/new`, `#/explore`, `#/new`, `#/library`, `#/templates/<id>`, `#/editor/<projectId>`, `#/settings`, `#/account`. Tham số query có kiểu.
- Shell:
  - `AppHeader` có logo, `MainNav` và `JobsPill`, kèm chấm ở bánh răng và badge ở mục Duyệt;
  - hẹp hơn 700px thì nav cuộn ngang;
  - `MobileShell` ở mức khung (màn điện thoại làm ở phase 09).
- Thành phần V0: `AgentOrb` (ánh xạ 4 trợ lý theo 02 §3), `ChannelAvatar`, `StatusLabel`, `ActionButton`, `Chip`, `Banner`.
- Thành phần V12:
  - `ConfirmDialog` đủ 7 tình huống (gõ để xác nhận, giữ để xác nhận);
  - trạng thái `loading`, `offline`, `not-found`, `empty-workbench`;
  - dải mất kết nối trong ≤ 5 s, nút ghi bị khoá khi mất kết nối.
- `#/dev/kit` (chỉ khi `import.meta.env.DEV`) dựng lại board `Kit` và `MKit` bằng component thật.
- Code splitting: mỗi màn dùng `React.lazy`; shell và Bàn dựng nằm trong chunk đầu.
- Kiểm tra tự động:
  - không có mã màu hex hay bo góc viết cứng ngoài token;
  - tối đa một `ActionButton variant="primary"` mỗi route;
  - script `npm run size -w apps/viderse/web` báo kích thước gzip của chunk đầu, lỗi khi vượt 250 kB.
- Copy: `i18n/vi.ts` lấy từ `06-copy.md` (thuật ngữ, mẫu câu, mã lỗi → câu).

### 5. Bố cục mã

Bố cục dưới đây áp dụng cho các phase sau.

```
apps/viderse/internal/
  bootstrap/ persistence/ domain/ web/        # phase 01
  tools/ accounts/ aiconn/ imagegen/ storage/ # phase 02
  youtube/ scan/                              # phase 03
  channels/ library/ backdrop/                # phase 04
  pipeline/ pipeline/steps/                   # phase 05
  editor/ render/                             # phase 06
  activity/ insight/                          # phase 07
  assistant/                                  # phase 08
apps/viderse/web/src/
  api/ data/ shell/ screens/<route>/ components/ editor/ i18n/ styles/
```

## Tệp

| Hành động | Tệp |
|---|---|
| Xoá | `apps/viderse/internal/{agent,script}/`, `apps/viderse/assets/`, `internal/persistence/migrations/00001_scripts_runs.sql`, `internal/persistence/postgres/{queries/scripts.sql,gen/*}` |
| Tạo | `platform/events/` (hub và test resume), `platform/ui/src/data/` (subpath `data`), `apps/viderse/internal/persistence/migrations/00001_base.sql`, `apps/viderse/internal/domain/`, `apps/viderse/web/src/{shell,screens,components,data}/…`, `apps/viderse/web/scripts/size.mjs` |
| Sửa | `apps/noverse/internal/entry/web/events.go` (dùng `platform/events`), `apps/noverse/web/src/data/*` (dùng `@noverse/ui/data`), `platform/ui/src/{Button,Banner,Toast}.tsx`, `platform/ui/package.json` (exports), `apps/viderse/internal/{web,bootstrap}/`, `apps/viderse/cmd/viderse/serve.go`, `apps/viderse/web/src/{main.tsx,router.ts,App.tsx,i18n/vi.ts}`, `apps/viderse/README.md`, `README.md`, `apps/viderse/docs/design/v1/11-hoi-dap.md`, `10-gap-closure.md` |

## Kiểm thử

- Go:
  - test resume của `platform/events` (chuyển từ `events_resume_test.go`);
  - toàn bộ test Noverse;
  - `archtest`;
  - test ETag và 304 cho `/api/status`.
- Web:
  - test cho kit (`quiet`, `tone="agent"`, toast dạng thẻ);
  - test router (mọi route và tham số);
  - test ranh giới kit với subpath `data`;
  - test của Noverse sau khi đổi import.
- Smoke `viderse serve` trên DB mới: setup → shell → tắt server (dải cam hiện ≤ 5 s) → bật lại (dải tự tắt).
- So `#/dev/kit` với board `Kit`/`MKit`: chụp cùng khung ở Chrome headless.

## Nghiệm thu

- Đạt phần FE trong mục Nghiệm thu của **V0** và **V12** (08-dev-prompt). Các dòng cần backend pipeline (JobsPill theo job thật, tạm dừng job) chuyển sang phase 05 và phase 07.
- `git grep -n -i -E "agent_runs|/api/runs|/api/scripts|WriterRole|writer_system" apps/viderse ':!apps/viderse/docs'` không còn kết quả nào. (`writer` vẫn là khoá `AgentOrb` của Đạo diễn theo 02 §3, nên không grep chữ này.)
- Chunk JS đầu ≤ 250 kB gzip.
- Noverse: test xanh, dist chỉ đổi đúng chỗ import.

## Rủi ro

- **Tách hub SSE làm đổi hành vi Noverse.** Tách nguyên khối, chạy lại test resume và watch. Live Noverse không đổi cho tới khi chủ động `run-live.sh update`.
- **Thêm subpath `data` vào kit vướng luật ranh giới.** Luật hiện chỉ cho import `react` và stylex; mã data không cần thư viện ngoài nên vẫn hợp lệ.
