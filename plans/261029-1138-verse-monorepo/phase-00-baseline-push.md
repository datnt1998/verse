---
title: "Phase 00 — Chốt baseline và đẩy lên GitHub"
status: done
---

# Bối cảnh

Tag `pre-verse` (annotated, chỉ có ở local) trỏ vào `6686262`. Commit này đã qua gate đầy đủ và smoke 3/3 (xem [plan hardening](../261029-0815-pre-verse-hardening/plan.md)). Sau tag, `main` có thêm `917e65a` (`feat(web): move writing style into story and project`). Commit này chỉ đụng `web/`, không có file `.go`, `assets/` hay `evals/`.

Phần còn chưa commit, tính tới 2026-10-29:

- `docs/design/**`: phần anh đang sửa;
- `web/src/screens/audio/Transcript.regen.test.tsx`: file của phiên Codex, chưa được theo dõi;
- plan này và `plans/reports/advise-verse-architecture.{png,svg}`.

`git archive` chỉ lấy những gì đã commit, nên phần nào chưa commit sẽ không sang được `verse`.

# Yêu cầu

- Không tự commit thay phiên Codex hay thay anh.
- `c4` và `main.go` ở gốc là file chưa theo dõi của anh. Chúng không đi theo sang `verse`.
- Chỉ gắn và push tag sau khi CI đã xanh ở đúng commit đó. Tag đã push thì không dời.

# Môi trường test

Dùng chung cho mọi gate trong plan:

```sh
docker compose -p verifycheck -f deploy/compose.test.yml up -d --wait postgres minio
export REQUIRE_POSTGRES_TEST=1 REQUIRE_MINIO_TEST=1 \
  TEST_DATABASE_URL=postgres://postgres:test@127.0.0.1:55432/ainovel_test \
  MINIO_TEST_ENDPOINT=http://127.0.0.1:59000
```

Từ phase 01 trở đi, file compose nằm ở `apps/noverse/deploy/compose.test.yml`.

# Các bước

1. Chốt nội dung:
   - Phiên Codex commit hoặc bỏ `Transcript.regen.test.tsx`.
   - Anh commit hoặc bỏ phần sửa `docs/design/**`.
   - Em commit plan này và hai file sơ đồ, theo đường dẫn cụ thể, sau khi quét secret.
2. Kiểm tree sạch: `git status --porcelain --untracked-files=no` rỗng, và trong danh sách file chưa theo dõi chỉ còn `c4` và `main.go`. Ghi lại `CANDIDATE=$(git rev-parse HEAD)`.
3. Chạy gate trên một snapshot sạch: `git worktree add /tmp/verse-gate $CANDIDATE`, dùng môi trường test ở trên.
   - Định dạng phải được kiểm bằng `gofmt` của đúng bản Go mà CI dùng (`go-version-file: go.mod`, hiện là `go 1.26.0`). `gofmt` 1.27 và 1.26 định dạng khác nhau ở struct literal ẩn danh nằm trong `return`. Run CI `40824347420` đã fail vì đúng lỗi này ở `internal/blobstore/local.go`. Lệnh: `G=$(GOTOOLCHAIN=go$(go mod edit -json | jq -r .Go) go env GOROOT); test -z "$($G/bin/gofmt -l .)" && test -z "$(gofmt -l .)"`. Sau đó chạy `go vet ./...` và `go tool sqlc diff`.
   - `go test -count=1 ./...`
   - `go test -race -count=1 ./internal/host ./internal/store ./internal/tools ./internal/persistence/postgres ./internal/storageimport`
   - Trong `web/`: `npm ci && npm run typecheck && npm test`. Ghi lại số file và số test.
   - Lưu danh sách test Go đã pass, dùng để so khớp ở phase 01:
     `go test -json -count=1 ./... | jq -r 'select(.Action=="pass" and .Test!=null) | .Package+" "+.Test' | sort > tests-before.txt`
   - Chạy lại smoke khi `git diff --name-only 6686262..$CANDIDATE` có file `.go`, hoặc file trong `assets/` hay `evals/`. Khi chạy, build binary từ snapshot: `go build -o /tmp/verse-gate/bin/ainovel-cli-$CANDIDATE ./cmd/ainovel-cli`, rồi gọi binary đó bằng đường dẫn tuyệt đối: `--cases evals/cases/smoke --ci --timeout 60m`. Ghi SHA của binary cùng run id. Tại `917e65a` chưa cần chạy lại.
4. Push `main` (chưa push tag): `git push origin main`. Kiểm `git ls-remote origin refs/heads/main` = `$CANDIDATE`.
5. Chạy CI và giữ đúng run id:
   - `gh workflow run CI --ref main`
   - `RUN=$(gh run list --workflow CI --branch main --event workflow_dispatch -L1 --json databaseId,headSha -q '.[0] | select(.headSha=="'$CANDIDATE'") | .databaseId')`
   - `gh run watch $RUN --exit-status`
6. CI đỏ: sửa trên `noverse`, quay lại bước 2 với commit mới. Tag lúc này chưa được push nên không có gì phải dời.
7. CI xanh: `git tag -f -a pre-verse $CANDIDATE -m "baseline before verse monorepo"`, rồi `git push origin pre-verse`. Tag cũ ở `6686262` chỉ có ở local, nên được ghi đè.
8. Dọn: hạ project `verifycheck`, xoá worktree `/tmp/verse-gate`. Giữ `tests-before.txt` ở `plans/261029-1138-verse-monorepo/reports/`.
9. Ghi kết quả vào file này: `$CANDIDATE`, `$RUN`, số test Go và web.

# Gate

| Kiểm tra | Điều kiện qua |
|---|---|
| Tree sạch | Bước 2 cho kết quả rỗng |
| Đúng run CI | `gh run view $RUN --json headSha,headBranch,event,conclusion,jobs` cho thấy: `headSha` = `$CANDIDATE`, `headBranch` = `main`, `event` = `workflow_dispatch`, `conclusion` = `success`. Cả job Go (Postgres+MinIO) lẫn job web đều `success` |
| Tag | `git rev-parse pre-verse^{commit}` = `$CANDIDATE` = `git ls-remote origin refs/heads/main`; `git ls-remote --tags origin pre-verse` có kết quả |
| Danh sách test | `tests-before.txt` có và không rỗng |

# Rollback

- Trước bước 7, chưa có gì cần lùi: chỉ có `main` được push, mà `main` vốn là nhánh làm việc.
- Sau bước 7, tag không được dời. Nếu cần baseline mới, tạo tag mới (ví dụ `pre-verse-2`), rồi cập nhật plan.

# Rủi ro

- Runner GitHub chạy lần đầu với compose Postgres+MinIO, nên có thể lệch môi trường (ffmpeg, cổng). Cách xử lý: đọc log của job, sửa workflow, không nới test.
- Hai test đã biết là flake: `TestExportSecondRunIsFast` và `TestImportPreviewDeadlineCancelsProviderRequest`. Nếu một trong hai fail, chạy lại đúng test đó 3 lần và ghi lại kết quả. Không thêm `t.Skip`.

# Kết quả (2026-10-29, xong)

| Mục | Kết quả |
|---|---|
| Gate cục bộ ở `580903b`, chạy lại ở `c49b37d` | `gofmt` (go1.26.0 và go1.27.3) sạch; `vet` sạch; `sqlc diff` sạch; `go test ./...` với PG17 + MinIO pass 4084 test, 4 test bị skip vì cần môi trường thật (`TestArchitectReconcilesMidpointLiveCopy`, `TestActivityHistoryRealLogCurl`, `TestFindPowerShellPrefersPwsh`, `TestRealVieNeuVolumeOneM4B`); `-race` trên 5 package pass; web 204 file / 1637 test pass |
| `tests-before.txt` | 4084 dòng, sinh ở `c49b37d` |
| Push `main` | `ab3aa77..580903b`, rồi tới `c49b37d` |
| CI `40824347420` (`580903b`) | Fail ở bước Format check: `gofmt` 1.26 và 1.27 định dạng khác nhau ở `internal/blobstore/local.go`. Sửa ở `b436757` |
| CI `40824841772` (`c874856`) | Fail khi khởi động service: `minio/minio` đã bị xoá khỏi Docker Hub (tháng 9/2026), còn quay.io thì bắt đăng nhập. Chuyển `deploy/compose.test.yml` sang `cgr.dev/chainguard/minio:latest` ở `1808cc7` |
| CI `40825496808` (`1808cc7`) | Fail ở `TestExportProgressMonotonicWithFakeFFmpeg`: test phụ thuộc thời gian, vì chữ "đoạn" chỉ được phát qua bộ lọc 250 ms, mà runner Linux nhanh nên cả chương xong trong một nhịp. Sửa ở `c49b37d`: tiêm đồng hồ vào `exportReporter`, thêm `TestExportReporterThrottlesSegmentSteps` |
| CI `40826439285` (`c49b37d`) | **success**: `test (ubuntu-latest)` và `web` đều success |
| Smoke | Binary build từ `c49b37d` (sha256 bắt đầu bằng `0c653775cc7a5b42`). Run `20261029-131144`: `architect_long`, `architect_short`, `writer_first_chapter` đều PASS; gate PASS. Hết 67 phút, $1,48, 1 lỗi tool tự hồi phục. Báo cáo ở `workspace/evals/pre-verse-smoke-c49b37d/` (thư mục bị gitignore). Lần chạy trên `c874856` đã bị dừng giữa chừng vì binary không còn khớp commit ứng viên. Các commit sau `c49b37d` chỉ được chứa tài liệu hoặc `web/`; nếu có file `.go` thì phải chạy lại smoke |
| Commit cuối | `523d4ee`: phần sửa `docs/design/**` của anh (gói AF–AL). `4f307c0`: `Transcript.regen.test.tsx` của phiên Codex, chạy riêng qua 2/2. `04aee55`: `deploy/docker-compose.dokploy.yml` chuyển sang `cgr.dev/chainguard/minio:latest`. Đã chạy `minio` và `minio-init` thật trên volume trống: bucket được tạo, private |
| Gate thay CI | Anh dặn không chạy CI cho đỡ tốn quota, nên gate ở `04aee55` chạy trên máy với **Go 1.26.0**, đúng bản CI dùng. Kết quả: `gofmt` 1.26 và 1.27 sạch; `vet` sạch; `sqlc diff` sạch; `sh -n scripts/install.sh` qua; `go test ./...` với PG17 + MinIO pass 4084 test, danh sách **trùng khớp** `tests-before.txt`; `-race` qua trên `host`, `store`, `tools`, `persistence/postgres`, `storageimport`, `audio`; web `npm ci`, `typecheck` sạch, 205 file / 1639 test pass. Lần chạy CI xanh cuối cùng trên GitHub là ở `c49b37d`; từ đó tới nay không có file `.go` nào thay đổi |
| Tag | `pre-verse` gắn ở commit ghi kết quả này (ngay sau `04aee55`, chỉ thêm tài liệu) |
