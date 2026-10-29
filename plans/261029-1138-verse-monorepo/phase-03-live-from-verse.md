---
title: "Phase 03 — Live chạy từ repo verse"
status: done (chờ anh xác nhận 2 dòng gate và mốc 7 ngày)
---

# Hiện trạng (2026-10-29)

| Mục | Giá trị |
|---|---|
| Process | `./bin/noverse serve -root /Users/dat/NoverseLive -addr 127.0.0.1:8790`. Process cha là `omp __omp_worker_daemon_broker`; không có launchd, không có `tailscale serve` |
| Mã nguồn | Worktree `/Users/dat/Projects/noverse-live`, nhánh `live`. Ngày 2026-10-29 phiên Codex đã đưa nó lên `917e65a` (pid 50549; trước đó là pid 26602 ở `f10299f`). Dist trong worktree đang bị sửa cục bộ (build lại); stash `live dist rebuilt at 26ccf12` |
| Script | `bin/run-live.sh`: chưa được git theo dõi. Lệnh `update` = `git merge --ff-only main` rồi `go build -o bin/noverse ./cmd/ainovel-cli` |
| Dữ liệu | `$HOME/NoverseLive` (sách); `~/.ainovel`: tài khoản `web-accounts.json` (v4), `auth.json`, `config.json`, `push/` |
| Chế độ | File mode; không có `STORE_DRIVER` |

Tên cookie, `~/.ainovel` và thư mục sách đều giữ nguyên, nên phase này không phải di chuyển dữ liệu. Phiên đăng nhập đang có vẫn còn hiệu lực.

# Các bước

1. Ghi baseline (chỉ đọc):
   - `curl -fsS 127.0.0.1:8790/healthz`;
   - danh sách thư mục sách: `ls $HOME/NoverseLive | sort > /tmp/live-books.txt`;
   - số dòng log lỗi hiện có, nếu có file log.
2. Sao lưu. File này chứa secret, nên đặt quyền trước khi ghi, không đưa vào repo và không in nội dung ra:
   `umask 077 && mkdir -p ~/Backups && tar -czf ~/Backups/.noverse-live-<ts>.tgz.part -C $HOME NoverseLive .ainovel && mv ~/Backups/.noverse-live-<ts>.tgz.part ~/Backups/noverse-live-<ts>.tgz`
3. Đưa script live vào repo: `apps/noverse/scripts/run-live.sh`.
   - Lấy nội dung từ bản hiện tại. Đổi dòng `cd "$(dirname "$0")/.."` thành `cd "$(dirname "$0")/../../.."` (từ `apps/noverse/scripts` lên gốc repo). Đổi bước build thành `go build -o bin/noverse ./apps/noverse/cmd/ainovel-cli`. Binary vẫn là `bin/noverse` tính từ gốc repo, và `.gitignore` ở gốc đã có `/bin/`.
   - Giữ nguyên biến `NOVERSE_LIVE_ROOT`, `NOVERSE_LIVE_ADDR`.
   - Commit: `build: track the live run script`.
4. `git -C /Users/dat/Projects/verse worktree add ../verse-live -b live main`, rồi trong worktree mới chạy `apps/noverse/scripts/run-live.sh update`.
5. Kiểm tương thích dữ liệu, để rollback về binary cũ vẫn an toàn: đọc `git diff <commit của worktree live lúc chuyển>..<commit import của verse>` cho `internal/store`, `internal/webauth`, `internal/library` (đối chiếu theo đường dẫn cũ). Nếu có thay đổi định dạng file trên đĩa (trường mới, `fileVersion` mới, đổi bố cục thư mục), ghi lại. Khi đó rollback phải khôi phục dữ liệu từ bản sao lưu ở bước 2, không chỉ đổi binary.
6. Chuyển:
   - Gửi SIGTERM cho process cũ; chờ thoát và cổng 8790 được nhả (`lsof -iTCP:8790 -sTCP:LISTEN` rỗng).
   - Khởi động bản mới bằng cách đã chốt ở Q9:
     - Giữ cách cũ: chạy một service omp có tên cố định.
     - Hoặc launchd: plist `~/Library/LaunchAgents/dev.verse.noverse-live.plist`, `KeepAlive`, log ra `~/Library/Logs/noverse-live.log`, nạp bằng `launchctl bootstrap gui/$(id -u) <plist>`.
7. Kiểm gate. Fail thì rollback ngay.
8. Giữ nguyên worktree `noverse-live` và binary cũ trong 7 ngày để rollback. Hết 7 ngày, xin anh xác nhận rồi mới `git worktree remove`.

# Gate

| Kiểm tra | Điều kiện qua |
|---|---|
| Đúng binary | `lsof -iTCP:8790 -sTCP:LISTEN -Fp` → `ps -o command= -p <pid>` trỏ vào `verse-live/bin/noverse` |
| Sống | `/healthz` trả 200 trong 10 giây sau khi khởi động |
| Phiên cũ còn hiệu lực | Anh mở live trên thiết bị đang đăng nhập, không bị đưa về màn đăng nhập |
| Dữ liệu | Màn danh sách dự án hiển thị đủ sách trong `/tmp/live-books.txt`. Mở được một chương và danh sách audio của một sách (chỉ đọc) |
| Ổn định | 10 phút đầu không có log mức ERROR mới. Sau 7 ngày không phải rollback |

# Kết quả (2026-10-29)

- Baseline: `/healthz` 200; danh sách sách ghi ở `/tmp/live-books.txt`; `web-accounts.json` v4, 1 user, 4 phiên.
- Sao lưu: `~/Backups/noverse-live-20261029-163638.tgz` (khoảng 2,8 GB, quyền `0600`, thư mục `0700`).
- Script: `apps/noverse/scripts/run-live.sh` (commit "build: track the live run script"); `exec` binary bằng đường dẫn tuyệt đối để `ps` chỉ rõ worktree.
- Worktree `../verse-live`, nhánh `live`. Lúc chuyển ở `94a6925`; sau đó `run-live.sh update` lên `51708aa` (gồm kiểm Origin, `platform/identity`, sự kiện core).
- Tương thích dữ liệu: `917e65a..pre-verse` không đổi `internal/store`, `internal/webauth`, `internal/library`. So tag JSON và `fileVersion` của `webauth` với `platform/identity`, và của `store`: không trường nào bị bỏ hay thêm. Thay đổi duy nhất trên đĩa: session log (`meta/sessions/**.jsonl`) có thêm dòng `{"_event": …}`. Binary cũ chỉ đọc các file này ở `diag` (xuất chẩn đoán) và `eval`; resume không đọc chúng. Rollback về binary cũ vẫn an toàn; bản xuất chẩn đoán của binary cũ chỉ hiện thêm vài dòng rỗng.
- Chuyển: dừng service omp `noverse-live41` (pid 50549, binary cũ), cổng 8790 được nhả, nạp `~/Library/LaunchAgents/dev.verse.noverse-live.plist` bằng `launchctl bootstrap`. PATH trong plist có `~/.local/bin` (uv) và `/opt/homebrew/bin` (ffmpeg, rg), giống PATH của process cũ ở những binary app gọi.

| Gate | Kết quả |
|---|---|
| Đúng binary | Cổng 8790 do `/Users/dat/Projects/verse-live/bin/noverse serve -root /Users/dat/NoverseLive -addr 127.0.0.1:8790` giữ, process cha là launchd (ppid 1) |
| Sống | `/healthz` 200 ngay sau khi khởi động, cả lúc chuyển lẫn sau lần update |
| Phiên cũ còn hiệu lực | Cookie, file tài khoản và 4 phiên giữ nguyên; `/api/projects` không cookie trả 401 như cũ. **Cần anh mở live trên thiết bị đang đăng nhập để xác nhận** |
| Dữ liệu | Qua. Danh sách thư mục sách sau khi chuyển trùng `/tmp/live-books.txt`. Kiểm đọc trên bản sao APFS của `NoverseLive` và `.ainovel` (bỏ `auth.json`, `web-token` và tài khoản; dùng một admin tạm). Binary cũ `917e65a` và binary live `ccfa802` lần lượt đọc qua API thật và cho kết quả **giống hệt nhau**: dự án `tron-audio` có 2 truyện; truyện 1 xong 64/64 chương (94.296 từ), audio tập 1 có 519 đoạn, 1872 giây; truyện 2 xong 28/36 chương (32.330 từ), audio 2559 đoạn, 8406 giây. Mở được chương cuối của mỗi truyện. Bản sao đã xoá; `NoverseLive` không có file nào đổi sau bản sao lưu lúc chuyển |
| Ổn định | Log `~/Library/Logs/noverse-live.log` không có dòng ERROR/panic sau khi chuyển và sau lần update. Mốc 7 ngày: tới 2026-11-05 |

Worktree cũ `/Users/dat/Projects/noverse-live` và binary cũ giữ tới 2026-11-05 để rollback.

Cập nhật 2026-10-29 lên `ce3f221` (sau khi merge 4b, phase 05 và Viderse Go) bằng `run-live.sh update` + `launchctl kickstart -k`: pid mới có ppid 1, `/healthz` 200, `/api/projects` không cookie 401, POST từ origin cổng khác hoặc host lạ (DNS rebinding) đều 403, danh sách sách và `web-accounts.json` (v4, 1 user, 4 phiên) giữ nguyên, log không có dòng lỗi.

# Rollback

1. Dừng bản mới theo đúng cách đang giữ nó:
   - launchd: `launchctl bootout gui/$(id -u)/dev.verse.noverse-live`. Chỉ kill process thì `KeepAlive` sẽ khởi động lại nó.
   - service omp: dừng service đó.
2. Chờ cổng 8790 được nhả (`lsof -iTCP:8790 -sTCP:LISTEN` rỗng).
3. Chạy `/Users/dat/Projects/noverse-live/bin/run-live.sh`. Binary cũ vẫn nằm ở đó.
4. Nếu bước 5 đã ghi nhận thay đổi định dạng dữ liệu, khôi phục `NoverseLive` và `.ainovel` từ bản sao lưu trước khi chạy binary cũ.

Không có thay đổi định dạng thì cả quá trình mất dưới 1 phút. Muốn chuyển sang bản mới lần nữa thì làm lại bước 6.

# Rủi ro

- **Phiên omp đang giữ live kết thúc ngay giữa lúc chuyển.** Cách giảm: chốt Q9 trước, rồi chuyển trong một lần.
- **`dist` trong worktree cũ đã được build lại cục bộ**, nên có thể khác với `dist` đã commit. Gate "`dist` khớp nguồn" ở phase 01 bảo đảm `dist` của `verse` đúng với nguồn web. Nếu giao diện khác bản đang chạy, đó là khác biệt giữa dist cục bộ và nguồn, không phải lỗi do dời repo. Ghi lại nếu gặp.
- **Ghi đè tài khoản.** Bản cũ và bản mới không được chạy cùng lúc trên cùng `~/.ainovel`. Bước 6 phải dừng hẳn bản cũ trước.
