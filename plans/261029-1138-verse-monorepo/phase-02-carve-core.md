---
title: "Phase 02 — Tách core/ và test ranh giới import"
status: chờ phase 01, Q5
---

# Bối cảnh

Bao đóng gồm 32 package. Đã kiểm bằng `go list -deps` và `go list -test -deps` trên `pre-verse`: bao đóng này không import package nào ngoài chính nó. Nhưng import sạch không có nghĩa là không đụng hệ thống file. `auth/oauth_icons_test.go:18-22` đọc `../../docs/design/v1/icons/*.svg`, tính từ vị trí file test. Vì vậy, ngoài việc dời thư mục và đổi import, phase này còn phải xử lý đúng một phụ thuộc file này (bước 1).

Có 20 package bên ngoài import vào bao đóng:

- `cmd/ainovel-cli`
- `agents`, `agents/ctxpack`, `agents/guard`
- `arbiter`, `audio`, `bootstrap`, `chapterfacts`, `diag`
- `entry/tui`, `entry/web`
- `eval`, `host`, `host/imp`, `host/sim`
- `ledger`, `revision`, `store`, `tools`, `userrules`

# Ánh xạ

Giữ nguyên tên package, nên định danh trong code không đổi.

| Nguồn (`apps/noverse/internal/…`) | Đích | Package |
|---|---|---|
| `agent/**` (`context`, `subagent`, `tools`, `task`, `schema`, `permission`, …) | `core/agentcore/**` | `agentcore` và các package con giữ tên cũ |
| `llm/**` (`wire`, `retry`, `provider/*`, `internal/testgolden`) | `core/llm/**` | `llm` |
| `models` | `core/models` | `models` |
| `llmretry` | `core/llmretry` | `llmretry` |
| `llmcontract` | `core/llmcontract` | `llmcontract` |
| `auth` | `core/auth` | `auth` |
| `codex`, `anthropicoauth`, `xai` | `core/auth/{codex,anthropicoauth,xai}` | giữ tên cũ |

Không làm trong phase này:

- đổi chuỗi runtime như `ainovel-cli login`, `~/.ainovel/auth.json`, `AINOVEL_CLAUDE_CODE_VERSION`;
- đổi fixture test có từ vựng tiểu thuyết;
- tách `roles`, dời sang `platform/`;
- đưa `agents/guard/repeat_failure.go` lên core (để phase 06).

# Các bước
1. Dời bằng `git mv` theo bảng trên. `go:generate` trong `models/registry.go` dùng đường dẫn tương đối, nên đi theo package.
   Riêng `TestCallbackIconsMatchDesignFiles`:
   - chép 10 file SVG mà test cần vào `core/auth/testdata/icons/`, rồi cho test đọc từ đó;
   - thêm một test phía `apps/noverse` để so từng byte giữa `apps/noverse/docs/design/v1/icons/<tên>.svg` và `core/auth/testdata/icons/<tên>.svg`. Như vậy bất biến "icon trang OAuth khớp file thiết kế" vẫn được giữ, và core không đọc file của app.
2. Đổi import cho từng cặp nguồn→đích bằng cùng cách như phase 01, bước 3. Nguồn: `github.com/datnt1998/verse/apps/noverse/internal/<cũ>`. Đích: `github.com/datnt1998/verse/core/<mới>`. Mỗi mẫu thay phải có lookahead `(?=[/"])`. Nếu thiếu, `internal/agent` sẽ khớp nhầm `internal/agents` (package của Noverse), và `internal/llm` sẽ khớp nhầm `internal/llmretry`.
3. Viết test ranh giới ở `archtest/boundary_test.go` (gốc `verse`, package `archtest`). Test chạy `go list -json -deps -test ./...` rồi áp ba luật:
   - `core/**` chỉ import stdlib, thư viện ngoài và `core/**`.
   - `platform/**` (khi có) chỉ import stdlib, thư viện ngoài, `core/**` và `platform/**`.
   - `apps/<a>/**` không import `apps/<b>/**` với `b ≠ a`.
   
   Luật áp cho cả phụ thuộc trong test. Mỗi vi phạm in ra cặp `importer → imported`.
4. Thêm đối chứng để test không qua một cách vô nghĩa. Tạo một module nhỏ ở `archtest/testdata/violation/`, trong đó có đúng một vi phạm mỗi luật. Test chạy cùng hàm kiểm trên module này và yêu cầu nhận đúng 3 vi phạm đó.
5. Commit: `refactor: move the agent and llm runtime into core`, rồi `test: enforce import boundaries between core, platform and apps`.

# Gate

| Kiểm tra | Điều kiện qua |
|---|---|
| Số package core | `go list ./core/... \| wc -l` = 32 |
| Ranh giới | `go test ./archtest` qua trên repo, và đối chứng nhận đúng 3 vi phạm đã cài |
| Ranh giới có hiệu lực | Thêm tạm `import _ "github.com/datnt1998/verse/apps/noverse/assets"` vào một file trong `core/llm`. `assets` là package công khai, nên luật `internal/` của Go không chặn trước; vi phạm phải do test ranh giới bắt. Kết quả: `go test ./archtest` fail và in đúng cặp `core/llm → apps/noverse/assets`. Gỡ dòng import xong mới commit |
| Gate phase 01 | Qua hết, trừ dòng "Số test không đổi". Dòng đó đổi thành: mọi test trong `tests-before.txt`, sau khi đổi đường dẫn sang vị trí mới, vẫn có mặt và vẫn pass. Test mới (`archtest`, test so icon) được liệt kê riêng |
| Kiểm icon | Sửa tạm một byte trong `apps/noverse/docs/design/v1/icons/login.svg`: test so icon fail. Hoàn tác xong mới commit |
| Smoke eval | Build binary từ commit ứng viên: `go build -o /tmp/verse-m2/ainovel-cli-$(git rev-parse --short HEAD) ./apps/noverse/cmd/ainovel-cli`. Trong `apps/noverse`, gọi binary đó bằng đường dẫn tuyệt đối: `eval --cases evals/cases/smoke --ci --timeout 60m`. Gate PASS 3/3. Ghi SHA của binary cùng run id |
| Ghi số đo | Ghi chi phí, thời lượng và số lỗi tool, so với baseline ($1,49 / 67 phút / 3). Chi phí không phải gate, vì chưa có số đo độ dao động. Case nào FAIL thì chạy lại đúng case đó một lần để tách flake, rồi ghi cả hai kết quả |

# Rollback

Revert hai commit của phase này. Phase không đổi dữ liệu và không đổi hành vi.

# Rủi ro

- **Tên package `context` che stdlib.** Chỗ nào import nó đã có alias từ trước; dời thư mục không đổi điều này.
- **Thay chuỗi khớp nhầm tiền tố.** `internal/agents` (ở lại Noverse) có cùng tiền tố với `internal/agent`; `internal/llmretry` có cùng tiền tố với `internal/llm`. Lookahead ở bước 2 chặn việc này. Gate "số package core = 32" và việc build sẽ bắt nếu vẫn lọt.
