# Báo cáo: cutover đăng nhập chung

Hướng dẫn vận hành: [runbook](../../../apps/noverse/deploy/sso-cutover.md).
Chủ hợp đồng thực thi: `identity_cutover.go`, `backup_stats.go`,
`storage_sso.go` và các test tương ứng trong Noverse.

## Hợp đồng cuối

- CLI chỉ dùng `MIGRATION_DATABASE_URL`; apply cần manifest trên đĩa và hai
  digest đã duyệt. Dry-run không ghi DB; backup-only không mở DB.
- Dừng mọi app/worker trước dump và giữ tắt đến hết apply. Dump custom được
  đọc qua cùng handle no-follow, kiểm owner/quyền và SHA-256; inventory mới
  dùng O_EXCL/0600/fsync, không ghi đè lần trước.
- Backup gate là lưới an toàn **count + latest-timestamp**, không chứng minh
  nội dung bằng nhau. Với mọi base table trong schema chứa `users`, số dòng
  COPY và max từng cột `timestamptz` phải khớp aggregate `FROM ONLY`.
  Thay đổi dữ liệu không đổi các thống kê đó có thể lọt qua.
- Identity transaction lấy advisory locks, dòng `auth_invariant_lock` bằng
  `FOR UPDATE NOWAIT`, rồi `LOCK TABLE ONLY <schema.table> IN EXCLUSIVE MODE
  NOWAIT` cho toàn bộ tập bảng trước đối chiếu lại. `FOR UPDATE NOWAIT` chỉ phủ
  khoá dòng, nên transaction đặt thêm `lock_timeout = 1s` để chờ khoá cấp bảng
  cũng thành từ chối 55P03; lỗi khoá khác kèm SQLSTATE.
  Identifier được quote an toàn. SQLSTATE 55P03 dùng chung thông báo tên
  bảng, yêu cầu dừng app/worker rồi thử lại; autovacuum/ANALYZE cũng có thể
  gây xung đột, cần kiểm `pg_locks`/`pg_stat_activity` nếu vẫn bị chặn.
  Không đưa dữ liệu dòng vào lỗi. Role migrate sở hữu bảng được lấy khoá.
- Giữ READ COMMITTED: aggregate nhìn thấy mọi write commit trước khi lấy
  khoá; EXCLUSIVE giữ đến commit ngăn writer thay đổi dữ liệu sau đối chiếu.
  REPEATABLE READ không cần thiết và có thể giữ snapshot cũ trước lúc lấy
  đủ khoá. Kết nối nhàn rỗi không bị phát hiện; khoá không thay thế đóng băng.
- Tài khoản, quyền app, ledger và thu hồi session/pairing commit cùng nhau;
  nguồn file tiếp tục nhập thư viện sau đó. Resume giữ mật khẩu/quyền đã đổi.
- Mỗi identity transaction có dòng `import_runs` riêng, kể cả nguồn rỗng:
  `source_label='sso-cutover-identity'`, `status='verified'`,
  `summary.cutoverBackup={inventoryPath,dumpPath,dumpSha256}`.
  Dòng chỉ tồn tại khi transaction commit; report trỏ qua
  `identity.cutoverRunId`. Resume không ghi đè bản ghi trước, không cần
  migration mới hoặc marker sau commit. Inventory tự nó không chứng minh
  thành công; rollback toàn cutover dùng dump của transaction commit đầu tiên.
- `--backup-dir` tương đối được chuyển bằng `filepath.Abs` trước inventory
  naming. Dump/inventory được chuẩn hoá tuyệt đối trước băm source/run ID
  và ghi summary, nên rollback không phụ thuộc thư mục chạy lệnh sau này.
- Runbook dùng `psql -X` cho role provisioning để `~/.psqlrc` không làm lộ
  SQL đã nội suy mật khẩu; secret chỉ nạp từ env riêng tư/PGPASSFILE.

## Bằng chứng kiểm thử

Regression dùng Postgres và pg_dump/pg_restore thật: writer, dòng invariant
đang bị khoá và khoá bảo trì SHARE UPDATE EXCLUSIVE đều bị từ chối nhanh
trước mọi ghi import; bảng kế thừa đếm riêng. Bản ghi backup commit tồn tại
cho nguồn có dữ liệu/rỗng và không bị ghi đè khi resume. Test đường tương
đối kiểm cả inventory, dump và run ID được chuẩn hoá tuyệt đối; CLI kiểm
report trỏ đúng bản ghi trong DB và dump/inventory của từng lần.

Kiểm đã chạy với Postgres bắt buộc, `GOTOOLCHAIN=go1.26.0`, `GOMAXPROCS=4`,
`nice -n 10`, `go test -p 1 -count=1` cho hai package:
`storageimport` PASS (77.874s), `ainovel-cli` PASS (20.473s).
`go vet -p 1` cả hai package sạch trên Go 1.26.0 và 1.27.0;
`gofmt -l` toàn bộ file Go thay đổi (kể cả test) không xuất đường dẫn trên
cả hai toolchain. Dùng `-p 1` không lockf theo điều phối kiểm E2E song song;
không chạy lại E2E hay race trong phần việc này.

