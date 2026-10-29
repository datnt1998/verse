---
title: "Phase 01 — Dời Noverse vào apps/noverse"
status: done
---

# Bối cảnh

Phase này chỉ dời code và đổi đường dẫn. Hành vi không đổi. Các tên runtime giữ nguyên:

- binary `ainovel-cli`;
- thư mục `~/.ainovel`;
- cookie `ainovel_session`;
- header `X-Ainovel-*`, biến môi trường `AINOVEL_*`;
- tên DB và role `noverse_*`, bảng `noverse_meta`.

Module đổi từ `github.com/datnt1998/noverse` thành `github.com/datnt1998/verse`. Code nằm ở `github.com/datnt1998/verse/apps/noverse/...`.

# Ánh xạ thư mục

| Nguồn (`pre-verse`) | Đích (`verse`) | Ghi chú |
|---|---|---|
| `go.mod`, `go.sum` | gốc | Giữ nguyên cả file, kể cả dòng `tool` của sqlc. Sau đó `go mod edit -module github.com/datnt1998/verse` |
| `LICENSE`, `third_party/NOTICE` | gốc | Giữ nguyên nội dung |
| `AGENTS.md`, `.agentkit/` | gốc | Giữ nguyên |
| `.gitattributes` | gốc | Giữ nguyên |
| `.github/` (cả cây: `workflows/{ci,docker,release}.yml`, `scripts/gen-changelog.sh`) | gốc | Sửa đường dẫn ở bước 5. `release.yml:25-26` gọi `.github/scripts/gen-changelog.sh`, nên thư mục script phải nằm ở gốc |
| `.goreleaser.yml` | gốc | `main: ./apps/noverse/cmd/ainovel-cli`; `files`: `apps/noverse/README.md`, `LICENSE`, `third_party/NOTICE`; `release.github.name: verse` |
| `.dockerignore` | gốc | Thêm tiền tố `apps/noverse/` cho các mẫu riêng của app; `.ainovel` → `**/.ainovel` |
| `.gitignore` | `apps/noverse/.gitignore` | Mẫu có `/` ở đầu được tính từ thư mục chứa file, nên không phải sửa. Gốc có `.gitignore` ngắn: `.DS_Store`, `.env`, `go.work`, `/bin/` |
| `Dockerfile` | `apps/noverse/Dockerfile` | Build context là gốc `verse` (bước 6) |
| `cmd/ internal/ assets/ web/ deploy/ docs/ evals/ scripts/ config/` | `apps/noverse/…` | `docs/` phải đi cùng app, vì `assets/pack_test.go:14` và `internal/ledger/ledger_test.go:92` đọc `docs/prd/rules`, còn `internal/auth/oauth_icons_test.go:22` đọc `docs/design/v1/icons`, đều theo đường dẫn tương đối |
| `README.md`, `HUONG_DAN_SU_DUNG.md`, `config.example.jsonc`, `docker-compose.yml`, `sqlc.yaml` | `apps/noverse/` | `sqlc.yaml` dùng đường dẫn tương đối, nên đặt ở `apps/noverse` là đúng |
| `plans/` (trừ plan này) | `apps/noverse/plans/` | Loại 2 file GPL (Q4) |
| `plans/261029-1138-verse-monorepo/` | `plans/` ở gốc | Plan cấp `verse` |
| — | `README.md` ở gốc (mới) | Mô tả bố cục, cách build từng app, nguồn import `datnt1998/noverse@<commit>`, và quyết định đổi module path thay cho ghi chú "giữ module path" trong tài liệu cũ |

Có 2 file đang được theo dõi dù khớp `.gitignore`: `config/config.example.jsonc` và `config/rules/README.txt` (kết quả `git ls-files -ci --exclude-standard`). Ở repo mới, `git add -A` sẽ bỏ sót chúng, nên phải `git add -f`.

# Các bước

1. Sau khi anh duyệt: `gh repo create datnt1998/verse --private`, rồi tạo `/Users/dat/Projects/verse`.
2. Giải nén archive vào `apps/noverse`, loại 2 file GPL:
   `git -C ../ainovel-cli archive pre-verse | tar -x -C apps/noverse --exclude 'plans/reports/xia-pagekin/01-prompt-pipeline.md' --exclude 'plans/reports/xia-pagekin/04-planning-import.md'`
   Sau đó dời các mục cấp gốc ra ngoài theo bảng trên. Stage bằng `git add -A`, cộng thêm `git add -f` cho 2 file ở trên.
3. Đổi module bằng một lệnh. Lệnh này đổi cả import lẫn chuỗi trong code, kể cả file sinh ra như `models_generated.go` và `gen/`:
   `grep -rl --include='*.go' 'github.com/datnt1998/noverse' . | xargs perl -pi -e 's#github\.com/datnt1998/noverse(?=[/"])#github.com/datnt1998/verse/apps/noverse#g'`
   Sau đó chạy lệnh `gofmt` hai phiên bản như phase 00, bước 3, để bắt file lỗi format.
4. Neo lại hai chỗ đang tìm `go.mod`:
   - `internal/blobkey/blobkey_test.go` `repoRoot`: vẫn lấy `filepath.Dir(filepath.Dir(wd))`, nhưng kiểm thư mục `cmd/ainovel-cli` thay cho `go.mod`. Đổi tên hàm thành `appRoot`.
   - `internal/entry/web/gentypes/main.go` `outputPath`: đi ngược lên tới thư mục đầu tiên có `web/package.json`, thay cho `go.mod`. Test drift chứng minh file đích là `apps/noverse/web/src/api/types.gen.ts`.
5. Workflow:
   - `ci.yml`:
     - job web dùng `working-directory: apps/noverse/web` và `cache-dependency-path: apps/noverse/web/package-lock.json`;
     - `sh -n apps/noverse/scripts/install.sh`;
     - compose dùng `-f apps/noverse/deploy/compose.test.yml`;
     - bước `go tool sqlc diff` dùng `working-directory: apps/noverse`;
     - bước race chạy `./apps/noverse/internal/{host,store,tools}`;
     - `gofmt`, `vet` và `test` vẫn chạy ở gốc.
   - `docker.yml`: dùng `images: ghcr.io/${{ github.repository }}/noverse`, `file: apps/noverse/Dockerfile`, `context: .`.
   - `release.yml`: đường dẫn `.github/scripts/gen-changelog.sh` vẫn đúng vì `.github/` nằm ở gốc. Secret của release (`GEMINI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`) chưa có trong repo mới; chỉ phát hành sau khi anh đặt secret.
6. `apps/noverse/Dockerfile` (context = gốc `verse`):
   - Stage web: `WORKDIR /src/apps/noverse/web`; `COPY apps/noverse/web/package.json apps/noverse/web/package-lock.json ./`; `COPY apps/noverse/web/packages/ui/package.json packages/ui/package.json`; `COPY apps/noverse/web/ ./`. Vite xuất ra `../internal/entry/web/dist`, tức `/src/apps/noverse/internal/entry/web/dist`.
   - Stage builder: `COPY go.mod go.sum ./`; `COPY . .`; `COPY --from=web /src/apps/noverse/internal/entry/web/dist ./apps/noverse/internal/entry/web/dist`; build `./apps/noverse/cmd/ainovel-cli`.
   - Stage runtime giữ nguyên.
7. Compose và script:
   - `apps/noverse/docker-compose.yml`: `context: ../..`, `dockerfile: apps/noverse/Dockerfile`.
   - `apps/noverse/deploy/docker-compose.dokploy.yml`: `context: ../../..`, `dockerfile: apps/noverse/Dockerfile`.
   - `apps/noverse/scripts/install.sh`: đổi `REPO="datnt1998/verse"`, đổi URL raw thành `.../verse/main/apps/noverse/scripts/install.sh`.
   - `apps/noverse/internal/version/info.go:14`: đổi `ReleaseRepo` thành `"datnt1998/verse"`, vì lệnh tự cập nhật lấy release từ repo này, và từ phase này release được goreleaser đẩy lên `verse`. Sửa comment ở dòng 11 cho khớp. `update_test.go` truyền repo qua tham số nên không phải sửa. Binary cũ đã cài vẫn hỏi `noverse`. Repo đó đóng băng, nên các binary cũ sẽ không nhận được bản cập nhật mới nữa; ghi chú điều này trong `apps/noverse/README.md`.
   - `apps/noverse/deploy/README.md`: thêm đường dẫn compose mới cho nguồn Dokploy. Đây là tài liệu vận hành, nên cần cập nhật.
8. Commit đầu: `chore: import noverse from datnt1998/noverse@<commit>` (bước 2). Commit thứ hai: `build: anchor app paths for the verse monorepo` (bước 3–7). Tách đôi để phần dời thuần tuý xem lại được riêng.
9. Push lên `verse`. Chạy CI bằng tay và kiểm theo đúng run id như phase 00, bước 5.

# Gate

Chạy trên một bản clone sạch của `verse`, với môi trường test của phase 00 (compose ở `apps/noverse/deploy/compose.test.yml`).

| Kiểm tra | Điều kiện qua |
|---|---|
| Đủ file | Lấy danh sách `git ls-tree -r --name-only pre-verse`, bỏ 2 file GPL, ánh xạ sang đường dẫn mới theo bảng trên, rồi so với `git ls-files`. Hai danh sách phải bằng nhau, không tính `README.md` và `.gitignore` mới ở gốc |
| Build | `go build ./...` và `go vet ./...` không lỗi; `gofmt` theo bản Go của `go.mod` và `gofmt` cục bộ đều cho kết quả rỗng (lệnh ở phase 00, bước 3) |
| Không sót module cũ | `grep -rl --include='*.go' 'github.com/datnt1998/noverse' .` rỗng. `grep -rl --include='*.go' 'datnt1998/noverse' .` chỉ còn `apps/noverse/internal/version/update_test.go` (chuỗi repo truyền làm tham số test) |
| sqlc | `(cd apps/noverse && go tool sqlc diff)` không có diff |
| Test Go | `go test -count=1 ./...` qua |
| Số test không đổi | Tạo danh sách pass như phase 00, đổi tiền tố `github.com/datnt1998/verse/apps/noverse/` thành `github.com/datnt1998/noverse/`, rồi `diff` với `tests-before.txt`: **rỗng**. So khớp tuyệt đối chỉ áp dụng ở phase này |
| Race | `go test -race` trên `host`, `store`, `tools`, `persistence/postgres`, `storageimport` qua |
| Kiểu TS sinh ra | `go generate ./apps/noverse/internal/entry/web` xong thì `git status --porcelain` rỗng |
| Web | Trong `apps/noverse/web`: `npm ci && npm run typecheck && npm test`. Số file và số test bằng số đã ghi ở phase 00 |
| `dist` khớp nguồn | `npm run build` xong thì `git status --porcelain apps/noverse/internal/entry/web/dist` rỗng. Nếu Vite build không ổn định (hash khác nhau giữa hai lần build từ cùng nguồn), ghi lại bằng chứng rồi bỏ dòng này. Không bỏ khi chưa có bằng chứng |
| Image | `docker build -f apps/noverse/Dockerfile -t verse-noverse:m1 .` qua; container chạy với HOME tạm thì `curl -fsS 127.0.0.1:<port>/healthz` trả 200 |
| Binary | `go build -o /tmp/verse-m1/ainovel-cli ./apps/noverse/cmd/ainovel-cli`. Chạy `serve` với `HOME` và `-root` tạm trên `127.0.0.1:8791`: `/healthz` trả 200, `GET /` trả HTML của SPA. Xong thì dừng process và xoá thư mục tạm |
| Đường release | `go run github.com/goreleaser/goreleaser/v2@latest check` qua (máy không cài sẵn goreleaser); `sh -n .github/scripts/gen-changelog.sh` qua |
| CI | Run CI trên `datnt1998/verse` có `headSha` đúng commit thứ hai và `conclusion=success` |

# Rollback

Repo `noverse` không bị đụng tới. Bỏ repo `verse` là quay về như cũ.

# Chạy thử (2026-10-29, ở `c49b37d`, trong `/tmp/verse-dry`, không push)

Đã chạy bước 2–3 và lệnh `go mod edit`. Kết quả:

- Archive đã loại đúng 2 file GPL; 4 file còn lại của `xia-pagekin` vẫn giữ.
- Lệnh thay chuỗi để lại đúng 2 file còn chuỗi `datnt1998/noverse`. Đó là chuỗi repo release (`version/info.go`, `update_test.go`), không phải đường dẫn import. Vì vậy gate "Không sót module cũ" ở trên phải loại trừ 2 file này (sau khi sửa `info.go`, chỉ còn `update_test.go`).
- `gofmt` sạch, `go build ./...` qua, `go vet ./...` sạch.
- `(cd apps/noverse && go tool sqlc diff)` sạch.
- `go test ./...` (không có PG/MinIO) fail đúng 2 test đã dự đoán ở bước 4: `TestNoKeyLiteralsOutsideBlobkey` (tìm `go.mod` ở gốc app) và `TestGeneratedTypesAreCurrent` (đọc `web/` tính từ gốc module). Không có test nào khác fail.

# Kết quả (2026-10-29)

Ba commit trên `main` của `verse`: `0cc9f98` (import từ `5f7a25e` = `pre-verse`), `17dd6df` (neo đường dẫn), `76a0e80` (build lại bundle web nhúng).

| Gate | Kết quả |
|---|---|
| Đủ file | Khớp bảng ánh xạ (kiểm ở commit đầu) |
| Build | `gofmt` 1.26.0 và 1.27.0 rỗng; `go build ./...` và `go vet ./...` sạch |
| Không sót module cũ | Không file `.go` nào còn import `github.com/datnt1998/noverse`; chuỗi `datnt1998/noverse` trong Go chỉ còn ở `update_test.go` |
| sqlc | Không có diff |
| Test Go | 4084 pass, 0 fail, có Postgres và MinIO (`REQUIRE_POSTGRES_TEST=1`, `REQUIRE_MINIO_TEST=1`) |
| Số test không đổi | `diff` với `tests-before.txt` rỗng. Phải đổi tiền tố ở **mọi chỗ** trong dòng, không chỉ đầu dòng: subtest `TestAllDiagnosticRulesVietnamese/<tên hàm>` lấy tên qua `runtime.FuncForPC`, nên tên subtest chứa cả module path |
| Race | Qua trên `host/...`, `store/...`, `tools/...`, `persistence/postgres`, `storageimport`, `audio` |
| Kiểu TS sinh ra | `go generate` không đổi `types.gen.ts` |
| Web | `npm ci`, `typecheck` sạch; 205 file / 1639 test pass, bằng phase 00 |
| `dist` khớp nguồn | Bundle commit ở `pre-verse` đã cũ hơn nguồn web: build ở `pre-verse` cho hash khác bản commit, và hai lần build liên tiếp cho cùng hash. Vì build ổn định, không bỏ dòng gate này mà commit lại bundle (`76a0e80`). Sau commit, `npm run build` để `git status` rỗng |
| Image | `docker build` qua; container (HOME là tmpfs) trả `/healthz` 200 và trang SPA |
| Binary | `serve` trên `127.0.0.1:8791` với HOME/root tạm: `/healthz` 200, `/` trả SPA với hash bundle mới. Đã dừng process và xoá thư mục tạm |
| Đường release | `goreleaser check` (v2.19.0) qua; `sh -n` qua cho `install.sh` và `gen-changelog.sh` |
| CI | **Hoãn theo yêu cầu của anh (tiết kiệm quota).** Repo `datnt1998/verse` (private) tắt Actions trước lần push đầu (`actions/permissions.enabled=false`); sau khi push, `actions/runs.total_count = 0`. Các gate cục bộ ở trên thay cho run CI. Khi bật lại Actions, chạy CI một lần ở commit mới nhất |

# Rủi ro

- **Có test so chuỗi đường dẫn package** (ví dụ lọc stack trace). Lệnh ở bước 3 đổi cả chuỗi, và gate so danh sách test sẽ bắt test bị mất.
- **Tag release.** Từ nay `v*` trong `verse` là release của Noverse. Khi app khác cần release, phải chốt lại cách đặt tag. Hiện chưa có nhu cầu.
- **`-buildvcs`.** Binary lấy thông tin VCS từ repo `verse`. Các biến `main.version`, `main.commit`, `main.date` vẫn được gán qua `ldflags` như cũ.
