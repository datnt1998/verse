# P0-1. Kế hoạch commit theo cụm

Phạm vi: toàn bộ **1.502** đường dẫn trong `git status --porcelain=v1 -uall` (226 `M`, 41 `D`, 1.235 `??`). Phân cụm phủ đúng 1.502: **1.486 file commit + 16 file không commit**. Người thực hiện chạy lệnh theo đúng thứ tự; mỗi cụm một nhánh + một commit.

Đã kiểm tra trên working tree hiện tại: `go build ./...` và `go vet ./...` đều sạch (exit 0). Chưa build riêng từng commit. Việc "build riêng được" ở từng cụm là **suy ra từ đồ thị import**: mọi cụm xếp sau phụ thuộc của nó. Khi chạy thật, sau mỗi commit nên chạy `go build ./...` trên một checkout sạch (CI của nhánh đó) để xác nhận.

**Cập nhật 25/10, sau khi làm BE:** các lát của `ke-hoach-be.md` thêm tệp mới:
- `assets/packs/**`, `assets/pack*.go`, `assets/voice_test.go`, `assets/writer_golden_test.go`;
- `internal/naolint/`, `internal/store/nao_*.go`, `internal/tools/nao_*.go`, `internal/tools/retention*.go`;
- `internal/rules/{metrics,effective,openings}.go` + `testdata/`;
- `internal/host/exp/narration.go`, `internal/audio/pack.go`, `internal/diag/rules_retention.go`;
- `evals/nao-w0/`, `evals/cases/editor-goi-a/`.

Pathspec theo thư mục của các cụm đã bao những tệp này. Dù vậy, trước khi chạy phải kiểm lại bằng `git status --porcelain -uall`: kết quả phải rỗng sau cụm cuối. Cách khác là gom toàn bộ phần BE thành một cụm riêng `p0-1/be-prd`, đặt sau cụm Host/engine.

Nguyên tắc xếp thứ tự (suy ra từ đồ thị import):

- `internal/bootstrap` import `internal/{auth,anthropicoauth,codex,xai}` và `internal/rules`; `internal/host`, `internal/agents` import `bootstrap` → **Đăng nhập** và **Lớp luật** phải trước **Host/engine**.
- `internal/voice/bootcfg` dùng `bootstrap.TTSConfig` — symbol **mới** của bootstrap — nên file này xếp vào cụm Host/engine, không vào Audio.
- `assets` import `internal/tools` → cụm **Việt hóa** phải đứng sau **Lớp luật**, không đứng đầu như tên gọi.
- `cmd/ainovel-cli` import `internal/entry/web`, `internal/entry/tui`, `internal/voice/*` → cụm CLI đặt **cuối cùng**.
- Chuỗi i18n tiếng Việt trong Go nằm rải theo package (host, tui, diag, web…), không tách theo hunk được; chúng đi theo cụm chứa package đó. Cụm **Việt hóa** giữ phần thuần assets/eval.

## Tóm tắt

| # | Nhánh `p0-1/<slug>` | Cụm | Số file | Build riêng |
|---|---|---|---|---|
| 1 | `tooling-ignore` | `.gitignore` | 1 | n/a |
| 2 | `docs-prd` | Docs/PRD | 14 | n/a |
| 3 | `core-base` | Nền móng lõi | 39 | có |
| 4 | `dang-nhap` | Đăng nhập | 38 | có |
| 5 | `lop-luat` | Lớp luật | 98 | có |
| 6 | `audio` | Audio | 45 | có |
| 7 | `viet-hoa` | Việt hóa | 47 | có |
| 8 | `host-engine` | Host/engine | 148 | có |
| 9 | `web` | Web | 1.031 | có |
| 10 | `tui-cli` | TUI + lệnh CLI + tài liệu gốc | 25 | có |
| — | — | Không commit | 16 | — |

Lệnh tạo nhánh chuẩn mỗi cụm: `git switch -c p0-1/<slug>` từ `main` rồi `git add` + `git commit` (các nhánh xếp chồng: nhánh sau tạo từ nhánh trước nếu muốn mỗi PR build được trên CI tại nhánh đó; hoặc commit tuần tự trên một nhánh `p0-1/all`). Dùng `git add -A <path>` cho mọi cụm để ghi cả file bị xóa (`D`).

## 1. `p0-1/tooling-ignore` — `.gitignore` (1 file)

Commit đầu tiên, tách riêng theo yêu cầu PRD.

- Message: `chore: bo qua node_modules goc, tsbuildinfo va file rac cuc bo`
- `git add -A .gitignore`
- File `.gitignore` trong working tree đã sửa sẵn để **un-ignore** `internal/entry/web/dist` (`gateway.go` nhúng bằng `go:embed all:dist` → dist bắt buộc commit). Cần thêm block:

```gitignore
# Cache npm/vitest ở gốc repo
/node_modules/
# Build cache TypeScript
*.tsbuildinfo
# File cookie rác do curl sinh ra
/c4
```

- Build riêng: n/a. Sau commit này `git status` tự biến mất 14 file `node_modules/`, `web/tsconfig.tsbuildinfo` và `c4`.

## 2. `p0-1/docs-prd` — Docs/PRD (14 file)

- Message: `docs: them PRD Viet hoa, bo luat goi the loai, script kiem tra va ADR nao truyen`
- `git add -A docs/prd/ docs/adr/`
  - `docs/prd/`: `toi-uu-ainovel-cli-tieng-viet.md`, `nao-truyen.md`, `brief-design.md` và `ke-hoach-be.md` (cả hai thêm sau khi lập kế hoạch), `assets/lo-trinh.png`, `rules/` (10 file: `master-rules-v3.md`, `phu-luc-the-loai-v3.md`, bộ luật A/B/C, `dit-dang-van-phong.md`, `he-thong-san-xuat-lien-tuc.md`, `kiem_tra_truyen.py`, `00-muc-luc.md`, `README.md`) và `p0-1-ke-hoach-commit.md` (file này).
  - `docs/adr/` (1): `0001-nao-truyen-du-lieu-goc-json.md`.
- Build riêng: n/a.

## 3. `p0-1/core-base` — Nền móng lõi (39 file)

Package lá / gần lá mà các cụm sau phụ thuộc; không import gì ngoài nhóm.

- Message: `feat: nen mong loi - domain, thu vien truyen, retry va hop dong LLM`
- `git add -A internal/errs/ internal/utils/ internal/logger/ internal/models/ internal/version/ internal/domain/ internal/library/ internal/llmretry/ internal/llmcontract/`
  - `internal/errs` 1, `internal/utils` 1, `internal/logger` 2, `internal/models` 1, `internal/version` 1, `internal/domain` 7, `internal/library` 18, `internal/llmretry` 4, `internal/llmcontract` 4.
- Build riêng: **có** — domain→errs; llmcontract→llmretry; còn lại không import nội bộ.

## 4. `p0-1/dang-nhap` — Đăng nhập (38 file)

- Message: `feat: dang nhap OAuth cho Codex, xAI, Anthropic va xac thuc web`
- `git add -A internal/auth/ internal/anthropicoauth/ internal/codex/ internal/xai/ platform/identity/ go.mod go.sum`
  - `internal/auth` 14, `internal/anthropicoauth` 7, `internal/codex` 4, `internal/xai` 3, `platform/identity` 8, `go.mod` + `go.sum` 2.
  - `go.mod/go.sum` đi kèm commit này vì gần hết require mới (`rsc.io/qr`, `x/crypto`, `golang-jwt`, `x/term`, `webpush-go`) phục vụ đăng nhập/push; commit trọn cặp để nhất quán.
- Build riêng: **có** — providers→`internal/auth` (cùng cụm); webauth→`internal/library` (cụm 3).

## 5. `p0-1/lop-luat` — Lớp luật (98 file)

- Message: `feat: lop luat, luat nguoi dung, stylestat, chapterfacts va cong cu kiem tra`
- `git add -A internal/rules/ internal/userrules/ internal/stylestat/ internal/chapterfacts/ internal/revision/ internal/flow/ internal/store/ internal/tools/ internal/diag/`
  - `internal/rules` 10, `internal/userrules` 7, `internal/stylestat` 4, `internal/chapterfacts` 2, `internal/revision` 6, `internal/flow` 4, `internal/store` 22, `internal/tools` 31, `internal/diag` 12.
  - `store`, `flow`, `revision` đi kèm (dù hơi "engine") vì `tools`/`revision`/`userrules` import chúng — không tách nếu muốn mỗi cụm build độc lập. `diag` là runtime chẩn đoán luật (`rules_*`).
- Build riêng: **có** — rules→domain+library (cụm 3); userrules→rules+llmcontract+store; tools→mọi dep đã commit trước/trong cụm.

## 6. `p0-1/audio` — Audio (45 file)

- Message: `feat: xuat audio, SRT, chuan hoa so va giong doc OpenAI TTS / VieNeu`
- `git add -A internal/audio/ internal/voice/voice.go internal/voice/service.go internal/voice/service_test.go internal/voice/shared.go internal/voice/openaispeech/ internal/voice/vieneu/ docs/design/v1/10-audio.md docs/design/v1/15-audio-impl.md`
  - `internal/audio` 30; `internal/voice` 13 trừ `bootcfg` (root 4 + `openaispeech` 5 + `vieneu` 4); 2 tài liệu audio trong `docs/design/v1`.
  - `internal/voice/bootcfg/bootcfg.go` **không** vào đây: nó dùng `bootstrap.TTSConfig` — symbol mới chỉ có sau khi commit `internal/bootstrap` — nên xếp sang cụm 8. Vì vậy phải liệt kê path con của `internal/voice` như trên, không dùng `internal/voice/`.
- Build riêng: **có** — audio→library (cụm 3); openaispeech/vieneu→voice (cùng cụm); không file nào trong cụm import package chưa commit.

## 7. `p0-1/viet-hoa` — Việt hóa (47 file)

- Message: `feat: Viet hoa toan bo prompt, go tai nguyen tieng Trung va eval case tieng Viet`
- `git add -A assets/ evals/ ':!assets/testdata/writer-golden.md'`
  - `assets/` 44: xóa `prompts/zh/` (14), `references/zh/` (20), `styles/zh/` (4), `voice_zh.md`; sửa `load.go`, `load_test.go`, `prompts/arbiter-intervention.md`, `references/outline-template.md`.
  - **Không commit việc xóa `assets/testdata/writer-golden.md`.** Hiện không test nào đọc tệp này: test golden đã bị gỡ cùng lúc. P0-6 yêu cầu khôi phục test này và sinh lại golden theo prompt tiếng Việt, nên để tệp cũ nằm trong lịch sử cho P0-6 thay thế. Không cần `git restore`.
  - `evals/cases/smoke/`: 3 case đã Việt hóa. `evals/nao-w0/`: bộ đo W0 của Não truyện (thêm sau khi lập kế hoạch).
- Build riêng: **có** — `assets` import `internal/tools` (cụm 5). Đây là lý do cụm này đứng sau Lớp luật.

## 8. `p0-1/host-engine` — Host/engine (148 file)

- Message: `feat: host/engine, bootstrap, agents, notify va registry lenh dung chung`
- `git add -A internal/host/ internal/bootstrap/ internal/agents/ internal/arbiter/ internal/notify/ internal/comments/ internal/commands/ internal/eval/ internal/entry/startup/ internal/entry/headless/ internal/voice/bootcfg/`
  - `internal/host` 104 (gồm `imp`, `exp`, `sim` và các handler `web_*` — là API surface của host cho web, đi theo host vì import hai chiều), `internal/bootstrap` 9, `internal/agents` 7, `internal/arbiter` 5, `internal/notify` 5 (web push + lifecycle gap30), `internal/comments` 2, `internal/commands` 3 (registry lệnh slash dùng chung TUI/web), `internal/eval` 8, `internal/entry/startup` 3, `internal/entry/headless` 1, `internal/voice/bootcfg` 1.
- Build riêng: **có** — mọi dep (auth cụm 4, rules/store/tools cụm 5, assets cụm 7) đã commit trước hoặc cùng cụm; host không import audio/voice/entry-web.

## 9. `p0-1/web` — Web (1.031 file)

- Message: `feat: giao dien web React, gateway nhung dist va tai lieu thiet ke v1`
- `git add -A web/ internal/entry/web/ docs/design/v1/`
  - `web/` 563 (`src` 540: screens 239, assets 141, ui 46, shell 24, mobile 19, lib 16, data 11, api 11, commands 10, tablet 7, i18n 6, styles 5, root 5; `mock` 12; `public` 5; `package.json`, `package-lock.json`, `tsconfig.json`, `vite.config.ts`, `index.html`, `README.md` — `tsconfig.tsbuildinfo` đã bị `.gitignore` chặn).
  - `internal/entry/web/` 274, **bao gồm `dist/` 185 file build sẵn, nhúng bằng `go:embed` — bắt buộc commit**.
  - `docs/design/v1/` 194 còn lại (trừ 2 file audio ở cụm 6; `git add` lặp lại path đã commit không sao).
- Build riêng: **có** — `entry/web` import ~30 package, tất cả đã commit ở cụm 3–8.

## 10. `p0-1/tui-cli` — TUI + lệnh CLI + tài liệu gốc (25 file)

- Message: `feat: lenh login/serve/voice, TUI cap nhat, README va config mau`
- `git add -A internal/entry/tui/ cmd/ainovel-cli/ README.md HUONG_DAN_SU_DUNG.md config.example.jsonc config/config.example.jsonc`
  - `internal/entry/tui` 17 (xóa `command_registry.go`, thêm `commands_test.go`), `cmd/ainovel-cli` 4 (`main.go` sửa + `login.go`, `serve.go`, `voice.go` mới), README/HUONG_DAN 2, config mẫu 2.
- Build riêng: **có** — cmd import entry/web + entry/tui + voice/bootcfg (đã commit); tui import auth/commands/host… đều đã commit. **Commit khép kín: sau nó `go build ./...` xanh toàn cục.**

## File KHÔNG commit (16)

| Đường dẫn | Lý do |
|---|---|
| `node_modules/.vite/vitest/` (2 file) | Cache vitest ở gốc repo — chặn bằng `/node_modules/` |
| `node_modules/.vitest-cache/` (12 file) | Cache vitest — chặn bằng `/node_modules/` |
| `web/tsconfig.tsbuildinfo` | Build cache TS — chặn bằng `*.tsbuildinfo` |
| `c4` | Cookie jar do curl sinh ra, file rác cục bộ — xóa tay, chặn bằng `/c4` |

Lưu ý: `internal/entry/web/dist/**` (185 file) **phải commit**, không nằm trong danh sách này — nhúng vào binary qua `go:embed`.

## Kết quả quét bí mật

Quét toàn bộ 1.235 file untracked với pattern khóa API/token/private key (`sk-*`, `ghp_*`, `AKIA*`, `AIza*`, `BEGIN ... PRIVATE KEY`, JWT, literal `access_token`/`password`/`client_secret`…): **không có bí mật thật**. Các hit đều là giả/mô phỏng:

- `sk-saved-*`, `sk-secret-*`, `sk-stub-*` trong `internal/entry/web/*_test.go` — fixture test.
- `codexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"` trong `internal/auth/openai_codex.go` — OAuth client ID công khai của Codex CLI, giống upstream.
- VAPID **public** key trong `web/mock/server.ts` — khóa công khai mô phỏng; private key sinh lúc chạy tại `~/.ainovel/push/vapid.json` (ngoài repo).
- `c4` chỉ chứa header cookie file, không có cookie nào.

## CI: workflow đã có, chỉ thêm job web

`.github/workflows/ci.yml` đã có từ commit `77931a2`. Workflow chạy trên PR và khi push `main`, ma trận ubuntu và windows, gồm các bước:

- kiểm định dạng `gofmt -l .`;
- `go vet`, `go test -count=1 ./...`;
- `go test -race` trên `internal/host`, `internal/store`, `internal/tools`.

Không viết lại workflow này, chỉ thêm job `web` vào cuối:

```yaml
  web:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: web/package-lock.json
      - run: npm ci
      - run: npx tsc --noEmit -p .
      - run: npx vitest run
      - run: npm run build
```

Commit riêng: `p0-1/tooling-ci`, thông điệp `ci: thêm job kiểm web`.

**Chặn trước khi merge:** bước `gofmt -l .` hiện báo đỏ ở `internal/entry/tui/panels_outline.go`. Đây là tệp PO đang sửa, agent không được động vào. PO chạy `gofmt -w internal/entry/tui/panels_outline.go` trước khi commit cụm chứa tệp đó, nếu không thì `main` sẽ đỏ ngay ở commit đầu.

**Bật chặn merge khi đỏ:** vào GitHub › Settings › Branches, thêm luật cho `main`, bắt buộc qua check `CI / test` (cả hai OS) và `CI / web`. Phần này làm trên GitHub, không nằm trong repo.

## Cách kiểm sau khi commit xong

1. `git status --porcelain=v1 -uall` → **rỗng**: 16 file không commit đã bị `.gitignore` nuốt.
2. `go build ./... && go vet ./... && go test ./...` → xanh (build/vet đã xác nhận trên working tree trước khi lập kế hoạch).
3. `cd web && npm ci && npx tsc --noEmit -p . && npx vitest run` → xanh.
4. `go run ./cmd/ainovel-cli --version` chạy được; `ainovel-cli serve` mở được web UI vì `internal/entry/web/dist` đã vào repo.
5. Trên GitHub: PR của từng nhánh `p0-1/*` phải xanh theo workflow trên trước khi merge; `main` xanh sau commit cuối.
