# Chuyển sang đăng nhập chung Noverse ↔ Viderse

Runbook cho **chủ máy tự chạy** khi chuyển hai app sang một kho tài khoản
Postgres (cookie `verse_session`). Không lệnh nào ở đây tự dò `HOME`, tự dừng
dịch vụ hay đụng launchd: bạn quyết định và tự làm từng bước. Diễn tập toàn bộ
trên bản sao (HOME/DB tạm) trước khi làm thật.

Hợp đồng lệnh: `ainovel-cli storage sso-import` (xem `ainovel-cli storage`
để in usage). DSN đích **chỉ** đọc từ `MIGRATION_DATABASE_URL` của role
`noverse_migrate`; không nhận DSN qua argv, không dùng `DATABASE_URL`. Report và
manifest không bao giờ chứa hash mật khẩu, mã tạm, mã khôi phục, token hay DSN.

## Những gì thay đổi với người dùng

- Mọi phiên cũ (`ainovel_session`, `viderse_session`, phiên Postgres cũ, mã QR
  đang chờ) hết hiệu lực. Ai cũng phải đăng nhập lại một lần.
- Mật khẩu, mật khẩu tạm (giữ đúng hạn cũ, không gia hạn), mã khôi phục, trạng
  thái khoá và nhật ký được giữ nguyên.
- Quyền app: tài khoản đến từ Noverse được quyền `noverse`, từ Viderse được
  `viderse`. Mọi quản trị sau cutover là **quản trị cả hai app**: quản trị
  Noverse (trong file hoặc đã có trong DB) có thêm Viderse, quản trị Viderse có
  thêm Noverse. Report liệt kê đủ danh sách (`globalAdmins`) và `--apply` luôn
  cần bạn duyệt digest của nó (`--accept-global-admins`).
- Hai tài khoản trùng tên ở hai app **không bao giờ** được gộp tự động, kể cả
  khi cùng mật khẩu: công cụ dừng và bắt bạn đổi tên một bên.

## A. Chuẩn bị (app vẫn chạy)

1. Ghi lại commit/binary đang chạy, env (không ghi secret), backend Noverse
   thật (`STORE_DRIVER=file` hay `postgres`), thư mục thư viện, blob driver, DB
   đích, URL hai app.
2. Build binary mới của cả hai app. Chạy toàn bộ cổng kiểm thử trên một DB
   clone, rồi restore thử một dump vào DB riêng.
3. Tạo hai UUID nguồn và **giữ nguyên** chúng cho mọi lần chạy (dry-run, apply,
   chạy lại): một cho Noverse (khi nguồn là file), một cho Viderse. Đổi UUID
   giữa chừng là tạo một lần nhập mới, không phải chạy lại.
4. Tạo thư mục riêng `BACKUP` bằng chính user sẽ chạy lệnh nhập
   (`mkdir -m 0700`; lệnh từ chối thư mục của user khác, hay thư mục mà
   group/other đọc hoặc ghi được) và thư mục snapshot riêng.

## B. Đóng băng và sao lưu

1. Tự dừng **mọi** thứ ghi vào tài khoản/thư viện: Noverse, Viderse, worker,
   TUI, lệnh CLI; tắt cơ chế tự khởi động lại; xác nhận không còn tiến trình
   ghi.
2. Nguồn file: tạo `.v4.bak` cạnh từng file gốc (O_EXCL, 0600, giống từng
   byte; không cần DB):

   ```sh
   ainovel-cli storage sso-import --noverse-store file \
     --noverse-accounts /abs/.ainovel/web-accounts.json \
     --noverse-source-id "$NOVERSE_SOURCE" --root /abs/thu-vien \
     --viderse-accounts /abs/.viderse/web-accounts.json \
     --viderse-source-id "$VIDERSE_SOURCE" \
     --backup-only --backup-dir "$BACKUP"
   ```

   Rồi copy nguyên `web-accounts.json` và thư viện vào snapshot (giữ quyền tệp;
   không copy `auth.json` OAuth). Các bước sau chỉ đọc snapshot.
3. File tài khoản phải là **v4**. Gặp lỗi "file phiên bản 2/3": nâng một **bản
   sao** bằng binary cũ (trước đăng nhập chung) của app đó. Trỏ `HOME` tạm vào
   bản sao, chạy app trên loopback và đăng nhập một lần: lần ghi đầu tiên mới
   nâng file, store tự giữ `.v2.bak`/`.v3.bak`. Tắt app, kiểm `"version": 4`
   trong file, rồi trỏ lệnh vào bản sao. Không mở file live để nâng.
4. Dump DB Postgres đích **trước khi đổi schema** (và DB Viderse để đối soát)
   để có đường quay lại. Cấu hình kết nối qua
   `PGHOST`/`PGPORT`/`PGDATABASE`/`PGUSER`/`PGPASSFILE` (file 0600), không
   đặt mật khẩu trên dòng lệnh:

   ```sh
   (umask 077; pg_dump --format=custom --file="$BACKUP/noverse-before-schema.dump")
   pg_restore --list "$BACKUP/noverse-before-schema.dump" >/dev/null
   ```

   `pg_restore --list` chỉ kiểm tệp đọc được; restore thật vào một DB tạm mới
   là bằng chứng dump dùng được. Bản này không đủ cho `--apply` (chụp trước
   `migrate up` nên thiếu bảng mới); bản dump mà lệnh nhập kiểm chụp ở bước D.

## C. Schema, role, dry-run

Secret không bao giờ nằm trên dòng lệnh hay shell history, và không ở lại shell
của bạn: chạy từng lệnh trong subshell nạp file 0600, ví dụ
`( set -a; . /secure/sso-migrate.env; exec ainovel-cli storage … )` với
`MIGRATION_DATABASE_URL` (role `noverse_migrate`; mật khẩu để trong
`PGPASSFILE` thay vì trong DSN càng tốt).

1. Áp schema bằng role migrate: `STORE_DRIVER=postgres ainovel-cli storage migrate up`.
2. DBA tạo role theo [`postgres-init/`](postgres-init/): `02-identity-role.sql`
   (`viderse_identity`, mật khẩu từ env `IDENTITY_ROLE_PASSWORD`) và
   `03-engine-worker-role.sql` (`noverse_engine_worker`, env
   `WORKER_ROLE_PASSWORD`), chạy bằng superuser để mật khẩu không vào log
   server. Luôn dùng `psql -X` để bỏ qua `~/.psqlrc` (cấu hình `ECHO` có thể
   in SQL đã nội suy mật khẩu); DSN admin không chứa mật khẩu, dùng
   `PGPASSFILE` 0600. Ví dụ, từ thư mục app:

   ```sh
   ( set -a; . /secure/identity-role.env; set +a
     exec psql -X "$IDENTITY_DSN_ADMIN" -v identity_schema=public \
       -f deploy/postgres-init/02-identity-role.sql )
   ( set -a; . /secure/worker-role.env; set +a
     exec psql -X "$ADMIN_DSN" -v worker_schema=public \
       -f deploy/postgres-init/03-engine-worker-role.sql )
   ```
3. Dry-run (mặc định, không ghi DB, không ghi `.v4.bak`; chỉ ghi manifest và
   report 0600). Env: `MIGRATION_DATABASE_URL`; nguồn file thêm
   `STORAGE_DRIVER`/`BLOB_LOCAL_ROOT` (hoặc cấu hình S3) đúng như `serve` sẽ
   dùng.

   ```sh
   # Noverse đang chạy file
   ainovel-cli storage sso-import --noverse-store file \
     --noverse-accounts /abs/snapshot/.ainovel/web-accounts.json \
     --noverse-source-id "$NOVERSE_SOURCE" --root /abs/snapshot/thu-vien \
     [--owner-map du-an=ten-nguon,…] \
     --viderse-accounts /abs/snapshot/.viderse/web-accounts.json \
     --viderse-source-id "$VIDERSE_SOURCE" \
     --manifest "$BACKUP/sso-manifest.json" --report "$BACKUP/sso-report.json"

   # Noverse đã ở Postgres: chỉ nhập Viderse
   ainovel-cli storage sso-import --noverse-store postgres \
     --viderse-accounts /abs/snapshot/.viderse/web-accounts.json \
     --viderse-source-id "$VIDERSE_SOURCE" \
     --manifest "$BACKUP/sso-manifest.json" --report "$BACKUP/sso-report.json"
   ```

   Viderse chưa từng có tệp tài khoản: dùng `--viderse-empty` thay cho hai cờ
   Viderse. Thiếu tệp không bao giờ được hiểu ngầm là kho trống.
4. Đọc toàn bộ report: số tài khoản từng nguồn, `collisions`, `globalAdmins`
   (mỗi người kèm nguồn quyền quản trị) và `globalAdminsDigest`,
   `liveUsersWithoutGrants`, `activeAdminCountAfter`, số phiên và mã QR sẽ bị
   huỷ. Có xung đột (`two_sources`, `target_occupied`, `id_conflict`,
   `changed_snapshot`, `viderse_projects`) thì lệnh trả mã 1 và **chưa ghi
   gì**. Sửa bằng một rename map riêng rồi dry-run lại:

   ```json
   [{"sourceApp": "viderse", "sourceUsername": "linh", "targetUsername": "linh.video"},
    {"sourceApp": "viderse", "sourceUsername": "minh", "targetUsername": "minh", "role": "member"}]
   ```

   `role: "member"` hạ một quản trị nguồn xuống thành viên thay vì để người đó
   thành quản trị cả hai app. Không sửa file nguồn.
5. Ghi lại hai digest **của lần dry-run bạn vừa duyệt**: dòng `Manifest
   sha256` in ra màn hình và `globalAdminsDigest` trong report. Mỗi lần
   dry-run ghi đè manifest, nên đừng tính lại digest từ file lúc apply.

## D. Apply (hai app vẫn tắt)

1. Dump DB đích **ngay trước apply**: sau `migrate up`, sau khi tạo role, khi
   mọi app đã dừng. Dump cả database (không `-t`/`-n`, không `--inserts`),
   định dạng custom, mỗi lần một tên mới, không bao giờ ghi đè bản cũ:

   ```sh
   (umask 077; pg_dump --format=custom \
     --file="$BACKUP/noverse-cutover-$(date -u +%Y%m%dT%H%M%SZ).dump")
   ```

2. Apply bằng hai digest đã duyệt:

   ```sh
   ainovel-cli storage sso-import <đúng các cờ nguồn như dry-run> \
     [--rename-map /abs/rename.json] \
     --manifest "$BACKUP/sso-manifest.json" --report "$BACKUP/sso-report.json" \
     --apply --accept-manifest <Manifest sha256 đã duyệt> \
     --accept-global-admins <globalAdminsDigest đã duyệt> \
     --backup-dir "$BACKUP"
   ```

- Manifest trên đĩa không khớp digest bạn dán (có ai đó chạy dry-run khác sau
  khi bạn duyệt), danh sách quản trị đã đổi, hoặc file manifest không còn:
  lệnh dừng, **không** in digest mới. Chạy lại dry-run và duyệt lại từ đầu
  (report, manifest, danh sách quản trị); không dán giá trị nào chưa duyệt.
- Lệnh xác minh bản dump trong `--backup-dir` qua `pg_restore` (cần có trên
  PATH), đọc đúng một handle đã mở cho mọi bước kiểm và cho SHA-256: đúng
  database đích, có dữ liệu **mọi** bảng của schema tài khoản và đọc hết được.
  Với từng bảng, lệnh so số dòng và giá trị mới nhất (`max`) của mọi cột
  `timestamptz` trong COPY, chỉ tính dòng riêng của bảng (`FROM ONLY`).
  Đây chỉ là **lưới an toàn count + latest-timestamp**, không chứng minh
  nội dung bằng nhau: sửa dữ liệu mà không đổi các thống kê này vẫn có thể
  lọt qua. Bảo đảm bản dump là trạng thái trước nhập phụ thuộc vào việc
  **dừng mọi app/worker trước khi dump** và giữ chúng tắt đến hết apply.
  Kiểm không phụ thuộc đồng hồ hay múi giờ của máy chạy `pg_dump`.
  Dump directory, symlink, tệp hoặc thư mục mà user khác đọc/ghi được
  (cần 0600/0700) đều bị từ chối. Đường dẫn, SHA-256 và phiên bản `pg_restore`
  được ghi vào report và inventory **mới** cho mỗi lần apply
  (`sso-backup-inventory-<UTC>.json`, không ghi đè). `--backup-dir` tương đối
  được chuyển thành đường tuyệt đối trước khi đặt tên inventory; mọi đường
  dump/inventory trong bản ghi rollback và dữ liệu băm run ID đều tuyệt đối.
- Trong transaction, lệnh lấy dòng `auth_invariant_lock` bằng
  `FOR UPDATE NOWAIT`, rồi khoá mọi base table của schema chứa `users` bằng
  `EXCLUSIVE MODE NOWAIT` trước đối chiếu thống kê và danh sách quản trị;
  khoá giữ đến commit. Transaction đặt `lock_timeout = 1s` để cả khoá cấp bảng
  (DDL còn mở, `VACUUM FULL`, `LOCK TABLE` tay) cũng không chờ vô hạn. Dòng
  invariant hoặc bảng đang bị giữ khoá xung đột khiến lệnh từ chối trong khoảng
  một giây, báo tên bảng và yêu cầu
  `dừng mọi app/worker kết nối database rồi thử lại`; lỗi khoá khác kèm
  SQLSTATE (không kèm dữ liệu).
  Autovacuum/ANALYZE cũng có thể giữ khoá xung đột dù app đã tắt: có thể thử
  lại sau khi tác vụ đó kết thúc; nếu vẫn bị chặn, kiểm tra
  `pg_locks`/`pg_stat_activity` để xác định kết nối giữ khoá.
  Lỗi khoá luôn huỷ transaction; không tự dừng hay huỷ tác vụ của database.
  Kết nối chỉ đang nhàn rỗi không bị phát hiện; vẫn phải tự dừng mọi app.
  Thống kê hoặc danh sách quản trị lệch thì huỷ cả transaction.
- Sau kiểm dump và **trước khi mở transaction**, lệnh khoá file nguồn
  (`<file>.lock`; còn writer giữ khoá thì dừng), kiểm/tạo `.v4.bak`
  (bản có sẵn phải là tệp thường, riêng tư 0600), rồi nhập
  **trong một transaction**: tài khoản hai nguồn, quyền app, nhật ký, sổ
  nguồn gốc, huỷ mọi phiên và mã QR cũ, bảo đảm còn ít nhất một quản trị đang
  hoạt động. Lỗi ở đây không để lại tài khoản nào.
- Nguồn file: sau khi transaction tài khoản commit, lệnh nhập tiếp toàn bộ thư
  viện/blob với cùng UUID nguồn. Report `identity_committed_library_incomplete`
  nghĩa là chạy lại **đúng lệnh** để tiếp tục; không đổi UUID, không xoá sổ
  nguồn gốc.
- Chạy lại lệnh đã xong là an toàn: tài khoản đã nhập được bỏ qua, mật khẩu
  người dùng đổi sau đó không bị ghi đè. Lần apply trước đã ghi DB, nên trước
  mỗi lần chạy lại `--apply` (kể cả để nhập tiếp thư viện) hãy dump thêm một
  bản **tên mới** theo bước 1. Điểm quay lại là dump được ghi cho transaction
  tài khoản đã commit đầu tiên, không phải inventory tạo đầu tiên; xem mục F.
- Mã thoát: 0 = đã lên kế hoạch/đã xong; 1 = xung đột, lỗi cờ/env, transaction
  bị huỷ, hoặc thư viện chưa nhập xong.

Đối soát trước khi bật app: số tài khoản (kể cả đã xoá), quyền app, nhật ký,
chủ/ thành viên dự án và (nguồn file) toàn bộ truyện/blob khớp report. Thư viện
chưa nhập xong thì **không** đổi `STORE_DRIVER`.

## E. Bật lại và kiểm

1. Env runtime, không còn DSN migrate:
   - Noverse: `STORE_DRIVER=postgres`, `DATABASE_URL` = `noverse_app`,
     `NOVERSE_WORKER_DATABASE_URL` = `noverse_engine_worker`.
   - Viderse: `VIDERSE_DATABASE_URL` = `viderse_app`,
     `VERSE_IDENTITY_DATABASE_URL` = `viderse_identity` trên DB của Noverse.
   - Cả hai: cùng `VERSE_APP_ORIGINS`. Local: bind `127.0.0.1`, không đặt
     `VERSE_COOKIE_DOMAIN`; cookie không phân biệt cổng nên không mở app lạ trên
     cùng host bằng trình duyệt này. Hosted: `PUBLIC_URL` riêng từng app,
     `SECURE_COOKIES=true`, cùng `VERSE_COOKIE_DOMAIN` (domain cha),
     `TRUSTED_PROXIES` đúng Traefik; cổng HTTP của app không publish ra
     host, chỉ Traefik ra ngoài.
2. Khởi động hai app, chưa mở cho người dùng. Kiểm bằng tài khoản thật của bạn:
   đăng nhập ở app này rồi mở app kia không phải đăng nhập lại; tài khoản thiếu
   quyền thấy màn "chưa được cấp quyền"; đăng xuất/đổi mật khẩu ở một app thì
   app kia đòi đăng nhập lại; cookie cũ không còn đăng nhập được. Kịch bản tự
   động tương ứng: `npm run test:sso` (chạy trên DB tạm, không chạm dữ liệu
   thật).
3. Chỉ mở cho người dùng khi report đã đối soát, restore thử dump đạt, smoke
   đạt. Ghi lại thời điểm chuyển; giữ binary, env, dump và snapshot cũ ở nơi
   riêng.

## F. Quay lại

- **Chưa mở cho người dùng** (hoặc apply lỗi): giữ hai app tắt. Lỗi
  transaction tài khoản thì DB không đổi. Inventory được tạo **trước**
  transaction nên tự nó không chứng minh đã commit; `--report` cũng bị ghi đè
  khi chạy lại. Mỗi identity transaction commit có một dòng `import_runs`
  với `source_label='sso-cutover-identity'`, `status='verified'` và
  `summary.cutoverBackup` chứa `inventoryPath`, `dumpPath`, `dumpSha256`.
  Dòng này được ghi trong chính transaction, kể cả khi không có nguồn tài
  khoản để nhập. Report của lần đó trỏ đến dòng qua `identity.cutoverRunId`;
  resume tạo dòng riêng, không sửa bản ghi trước.

  Nếu report đã bị ghi đè, dùng kết nối bảo trì (không mật khẩu trên argv,
  `PGPASSFILE` 0600) để tìm các transaction **đã commit**:

  ```sh
  psql -X -c "SELECT id, started_at, summary->'cutoverBackup' AS backup
    FROM import_runs WHERE source_label='sso-cutover-identity'
    AND status='verified' ORDER BY started_at, id"
  ```

  Muốn quay lại toàn bộ cutover, chọn bản ghi commit đầu tiên và giữ inventory
  mà `inventoryPath` chỉ đến; restore `dumpPath` của bản ghi đó vào một
  **DB khác** (trạng thái ngay trước apply, đã có schema mới), hoặc dùng
  `noverse-before-schema.dump` (trước `migrate up`). Trước khi restore, so
  `shasum -a 256 <tệp>` với `dumpSha256` trong bản ghi DB và SHA-256 trong
  inventory; lệch thì dừng. Nếu chưa có bản ghi commit nào, không suy ra
  thành công từ inventory; dùng dump đã xác minh trước lần thử bị huỷ.
  Đối chiếu DB mới, rồi trỏ binary/DSN cũ vào đó; nguồn file vẫn nguyên,
  `.v4.bak` giống từng byte nếu cần. Không `goose down`, không drop DB đang
  có dữ liệu, không nhập lại bằng UUID nguồn mới.
- **Đã mở cho người dùng**: dừng cả hai app và dump trạng thái hiện tại trước
  mọi restore. Restore bản cũ sẽ hồi sinh mật khẩu đã đổi, phiên và quyền đã
  thu hồi, và mất dữ liệu mới. Không có lệnh xuất ngược tài khoản về file, nên
  không có đường quay về file "không mất dữ liệu": ưu tiên sửa tiếp trên DB
  hiện tại (schema chỉ thêm, binary cũ tương thích).
- **Không quay về file tài khoản cũ sau khi đã mở cho người dùng.** File cũ
  không biết các thay đổi bảo mật sau cutover: tài khoản đã khoá, mật khẩu đã
  đổi hay đặt lại, quyền đã thu hồi đều "sống lại". Nếu buộc phải quay, trước
  khi bật bản cũ hãy áp lại thủ công từng thay đổi đó lên file (khoá lại tài
  khoản, cấp mật khẩu tạm mới cho người đã đổi/đặt lại mật khẩu, bỏ quyền đã
  thu hồi) dựa trên nhật ký quản trị trong DB.
- Khi chạy lại binary cũ: xoá `verse_session` ở cả host-only lẫn domain đã dùng
  và huỷ phiên trong kho đang dùng; không để hai phiên bản cùng ghi hai kho.
