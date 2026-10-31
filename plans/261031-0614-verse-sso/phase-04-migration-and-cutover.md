# Phase 04 · Nhập tài khoản, backup và runbook chủ máy tự cutover

- **Trạng thái:** pending. **Ước lượng:** 40h.
- **Phụ thuộc:** 01–03 hoàn chỉnh, binary ứng viên chưa mở ghi trên live. **Chặn:** 05 và quyết định vận hành cutover của chủ máy.
- **Owner:** kỹ sư migration; sở hữu storageimport/CLI và runbook. Đây là phase rủi ro cao, không chạy song song với migration/schema của phase khác.

## Bối cảnh đã xác minh

- File identity đang **version 4** (`platform/identity/store.go:26-32`), bao gồm Users/Sessions/Pairings/Audit (`:230-235`). User có hash, temp/recovery, tombstone và metadata (`:61-83`). File writes lấy `.lock` (`:382-419`) rồi atomic replace (`:430-451`).
- Backup v2/v3 hiện ghi O_EXCL, mode 0600, không đè bản gốc (`platform/identity/members.go:186-202,225-241`); test so byte gốc (`platform/identity/migration_test.go:139-145,182-185`). Dùng cùng nguyên tắc cho `.v4.bak`, nhưng bản đã tồn tại **khác digest phải dừng**, không coi là backup hợp lệ.
- Đã có importer Noverse: `runStorageImport` nhận root/accounts/source-id/manifest/dry-run, scan và gọi storageimport.Run (`apps/noverse/cmd/ainovel-cli/storage.go:319-372`). Run có source+manifest ID ổn định và ledger (`apps/noverse/internal/storageimport/importer.go:87-90,115-126,314-319`); session/pairing chủ ý không nhập (`decode.go:510-512`).
- Không được gọi importer cũ rồi mặc định an toàn: `insertUser` dùng ON CONFLICT DO NOTHING (`rows.go:204-218`) có thể che collision username; cùng hàm còn kéo dài temp TTL ít nhất 24h (`:230-245`). Chỉ copy hash qua API CreateUser cũng sai vì API nhận plaintext/hash lại. Phải dùng strict offline writer và giữ TTL v4 nguyên vẹn.
- Noverse `STORE_DRIVER` chọn **cả accounts và facts** (`apps/noverse/cmd/ainovel-cli/serve.go:258-276,98-118`). Nếu đang file mode, chỉ nhập accounts rồi bật Postgres sẽ làm mất đường đọc stories hiện hữu. Cutover phải bao gồm full library/blob importer đã có, không tạo hybrid account selector.

Trong phần bối cảnh, `decode.go`, `rows.go`, `importer.go` thuộc `apps/noverse/internal/storageimport/`.

## Tệp chính xác

- `apps/noverse/cmd/ainovel-cli/{storage.go,main.go}`; mới `storage_sso.go`, `storage_sso_test.go` (tên đề xuất).
- `apps/noverse/internal/storageimport/{decode.go,rows.go,importer.go,scan.go,manifest.go,report.go,importer_test.go}`; mới `identity_cutover.go`, `identity_cutover_test.go`. Tái sử dụng decoder/model v4/UUID/ledger, không làm một thư viện schema JSON thứ hai trong Viderse.
- `apps/noverse/internal/persistence/migrations/00005_import_ledger.sql` **chỉ đọc làm contract**, không sửa migration cũ; nếu ledger hiện hữu không thể biểu diễn hai source account-only thì thêm migration tiếp theo mới, không tạo ad-hoc DDL từ command. Mặc định dùng import_runs/import_items hiện có; Viderse runtime không được grant đọc các bảng này.
- Mới fixtures synthetic dưới `apps/noverse/internal/storageimport/testdata/sso/`: Noverse-v4, Viderse-v4, trùng/tombstone/temp/recovery; không copy tài khoản live. `platform/identity/migration_test.go` giữ test v2/v3 cũ.
- Mới `apps/noverse/deploy/sso-cutover.md` (runbook evergreen khi triển khai), link từ `apps/noverse/deploy/README.md` và `apps/viderse/deploy/README.md`. Runbook chi tiết bắt buộc ở ngay phần dưới của phase này để người triển khai không cần hội thoại.

## CLI đề xuất và giới hạn

Chỉ Noverse binary sở hữu migration command vì migration/schema/importer thuộc Noverse. Viderse có CLI grants, **không import** Noverse để di trú.

```text
ainovel-cli storage sso-import
  --noverse-store file|postgres
  [--noverse-accounts /absolute/snapshot/web-accounts.json]
  --viderse-accounts /absolute/snapshot/viderse/web-accounts.json
  [--root /absolute/frozen-library --owner-map project=username,...]
  --noverse-source-id UUID --viderse-source-id UUID
  --manifest /absolute/private/sso-manifest.json
  --report /absolute/private/sso-report.json
  [--rename-map /absolute/private/rename-map.json]
  [--dry-run | --apply --accept-manifest SHA256 --backup-dir /absolute/private/backup]
  [--accept-global-admins SHA256]
```

- Mặc định dry-run; dry-run đọc DB target để tìm collision nhưng **không** tạo file backup/source/DB row. Chỉ manifest/report mới được ghi ở đường dẫn explicit, mode 0600. Apply bắt buộc digest đã duyệt và backup-dir có receipts dump; không in secret vào report/stdout.
- `file`: noverse-accounts **v4** và root thư viện snapshot bắt buộc; nhập full Noverse facts/blob bằng pipeline hiện có. `postgres`: không nhập lại Noverse file/library, chỉ backfill/kiểm grants hiện có và nhập Viderse. Không coi file thiếu là kho trống.
- Viderse nguồn v4 dùng cùng decoder. Nếu file là v2/v3, preflight ghi version và dừng với hướng dẫn chuyển **bản sao** bằng migration hiện có trước; không mở live để tự upgrade hoặc đoán mất trường. File mới hơn v4/JSON hỏng → lỗi nonzero. Viderse chưa từng có file: owner phải chọn explicit “nguồn Viderse trống” (`--viderse-empty`, thay thế --viderse-accounts), không tự bỏ qua ENOENT.
- DSN target chỉ `MIGRATION_DATABASE_URL`, **không fallback app role** cho command apply này; secret qua env/protected service config, không argv. Identity data migration cần quyền bảo trì; provision role tách riêng chạy bằng DBA.
- Không mặc định đọc `~/.ainovel` hoặc `~/.viderse`. Chỉ chấp nhận absolute paths owner đưa vào. Không tự dừng launchd hoặc chạy update script. Công cụ kiểm source đóng băng/digest/lock, nhưng không được khẳng định có thể tự chứng minh mọi writer trên máy từ một file lock; maintenance window là điều kiện owner xác nhận.

## Collision và mapping: không bao giờ merge âm thầm

Preflight lập inventory **toàn bộ user**, gồm tombstone/locked, của hai file và DB target. Key là username canonical theo `citext`/trim/lowercase; validate ký tự/độ dài như identity. Hash/email/displayName giống nhau **không** phải chứng cứ cùng người.

1. Hai nguồn cùng username, hoặc source đụng target username (kể cả target đã xoá) → report liệt kê source app/source-id/source username/target username và loại conflict; exit nonzero **trước mọi DB write**. Không ON CONFLICT DO NOTHING rồi tuyên bố imported.
2. Ngoại lệ duy nhất cho rerun: ledger xác nhận **cùng source-id, snapshot/mapping digest, record key và target UUID** của lần apply đã commit. Khi đó skip có chủ ý, không ghi đè credential/grant đã được người dùng đổi sau import. Chỉ so password hash giống hoặc ID giống không đủ để skip.
3. Cách xử lý tối thiểu: rename map explicit dạng `{sourceApp,sourceUsername,targetUsername}`; giữ nguyên sourceUsername để audit/provenance, remap references DisabledBy/CreatedBy/Audit actor/owner-map cùng nguồn. Target username vẫn phải chưa chiếm, kể cả tombstone. Preflight lại toàn bộ map; không auto thêm suffix.
4. Không cung cấp auto-merge hay lấy password của app nào thắng. Ngay cả hai account thật sự cùng người, mặc định owner rename thành hai account riêng rồi quyết định account nào dùng và cấp grant/disable account còn lại qua công cụ chuẩn. Mọi thay đổi đó nằm trong maintenance/report được owner chấp thuận, không suy luận của agent.
5. Source UUID khác nhau cho Noverse/Viderse; deterministic user/device/audit IDs dùng namespace source để không đụng. Noverse source-id phải giữ nguyên giữa account import và full library import để membership/facts tham chiếu đúng user. Không tạo thêm user UUID ngẫu nhiên khi resume.

## Dữ liệu giữ lại và thay đổi có chủ ý

| Nguồn | Biến đổi/đích | Điều kiện |
|---|---|---|
| username/name/role, disabled/deleted, timestamps/onboarded | users | Giữ dữ liệu và tombstone; canonical username có report; disabled_by remap sau khi có đủ user |
| permanent Argon2 hash, passwordChangedAt | user_credentials | Chép nguyên hash, không cần biết plaintext, không hash hai lần |
| TempPassword hash/CreatedAt/ExpiresAt/UsedAt/CreatedBy | temporary_passwords | **Không kéo dài TTL**, không làm mã đã dùng thành chưa dùng; preserved must-change |
| Recovery hash/mask/CreatedAt/UsedAt | recovery_codes | Giữ trạng thái single-use; không xuất hash/code ra report |
| Noverse Projects map | projects/project_members | Giữ owner/collab/view theo pipeline library; validate đúng một owner active và tham chiếu thật; không bịa project từ slug thiếu metadata |
| Viderse Projects map | không nhập project Noverse | Bình thường rỗng; nếu nonempty thì report và dừng để owner xác minh, không silently drop/migrate thành dự án Noverse |
| tài khoản Noverse chưa xoá | app_grants(noverse,member) | Gồm locked; không tự cấp Viderse |
| tài khoản Viderse chưa xoá | app_grants(viderse,member) | Không tự cấp Noverse; admin global vẫn có quyền ngầm |
| file audit của hai nguồn | audit_logs + provenance | ID theo source, giữ actor/target/time; Viderse không được đọc history sau nhập |
| file Sessions/Pairings | không nhập bearer state | Đếm invalidated, mọi thiết bị đăng nhập lại; không biến file token thành cookie PG |
| PG sessions/pairings đang có khi noverse-store=postgres | revoke trong cutover identity tx | Buộc login lại, tránh cookie cũ/phiên cũ còn sống sau đổi policy |
| AI OAuth/config (`auth.json`, API keys), Viderse app DB | không chạm | Không phải account migration |

Giữ global_role admin của source, nhưng report phải nêu **tất cả admin Viderse sẽ có toàn quyền Noverse**. Apply có admin từ Viderse cần `--accept-global-admins` bằng digest danh sách được duyệt; tuỳ chọn mapping explicit hạ về member trước import nếu owner chọn. Không âm thầm nâng hay hạ role. Sau mapping phải có ít nhất một admin active nền tảng; tombstone/locked không được tính. Noverse membership invariant cũng phải giữ sau chuyển owner/name.

## Trình tự implementation và tính nguyên tử

1. Tách decoder/write account facts hiện hữu thành helper strict trong storageimport: giữ schema/UUID/ledger hiện có; `insertUser` không nuốt unique violation khác provenance. Sửa đường v4 không gia hạn temp TTL; nếu đường legacy cần grace cũ, chỉ bật explicit cho legacy, không dùng cho v4.
2. Xây preflight hai nguồn + target trong một report: malformed/duplicate/unknown reference/admin promotion/project invariant/changed snapshot. Không nhập một nguồn rồi mới phát hiện nguồn còn lại trùng. Dry-run phải query target, khác với nhánh dry-run importer cũ không mở DB (`storage.go:364-370`).
3. Apply xác nhận manifest, giữ source locks theo thứ tự path ổn định, kiểm digest lần cuối. Tạo `.v4.bak` cho từng file source bằng O_EXCL 0600; fsync file và thư mục. Backup có sẵn phải cùng digest; backup không ghi đủ/ENOSPC/mode lỗi → dừng **trước DB transaction**. Không sửa nội dung source; backup-dir giữ inventory/digest/receipts.
4. Trong một transaction identity: lấy advisory cutover lock + auth_invariant_lock cùng thứ tự với normal account mutations, recheck collision target; insert cả hai source user/credential/recovery/temp/audit/grants và source ledger; revoke PG sessions/pairings nếu áp dụng; commit cùng nhau. Hai apply đồng thời không được nhập hai lượt. Database failure rollback **cả hai nguồn identity**. Không dùng facade CreateUser để nhập hash.
5. Với Noverse file mode, tiếp tục full library/blob importer với **cùng** source namespace/manifest/account mapping và ledger. Account phase cũ phải delegate strict helper/nhận ledger đã commit, không reinsert bằng ON CONFLICT bỏ qua. Library/blob không thể là một transaction với file backup/object store: command chỉ báo hoàn tất sau reconcile toàn bộ; nếu dừng giữa bước, app vẫn offline, report ghi “identity committed, library incomplete”, resume cùng manifest. Không claim toàn cutover atomic.
6. Export report atomically sau mỗi checkpoint. Nếu commit DB xong mà ghi report thất bại, lần chạy lại đọc ledger để báo đúng; không nhập lại vì thiếu report. Không nhận snapshot đổi dưới cùng source-id/manifest.
7. Giữ syntax/import command cũ nhưng migrate account writer và tests để không tồn tại hai collision/TTL conventions. Không xoá tính năng full facts/blob importer đã có.

## Runbook chủ máy tự chạy (đưa nguyên nội dung được kiểm vào deploy/sso-cutover.md)

**Cảnh báo:** CLI mới chưa tồn tại ở thời điểm viết plan. Các lệnh dưới là hợp đồng cần implementation trước khi chủ máy dùng. Engineer chỉ diễn tập trong HOME/DB tạm, không chạy lên đường dẫn hoặc dịch vụ live.

### A. Chuẩn bị trước cửa sổ bảo trì

1. Chủ máy ghi nhận binary/commit cũ, env không secret và backend Noverse thật; xác định library root/blob driver/DB target, hai URL và domain. Không dựa vào giá trị cổng minh hoạ trong plan. Chuẩn bị cấu hình runtime mới nhưng chưa bật.
2. Build binary ứng viên, chạy toàn bộ gate 05 trên DB clone; restore thử dump vào một DB tách biệt. Ghi phiên bản `pg_dump`/Postgres tương thích; dự trù dung lượng backup, không lưu backup vào git.
3. Tạo thư mục backup private 0700 và snapshot private cho mỗi source. Owner tự dừng **cả Noverse/Viderse và engine writers/TUI/CLI ghi** theo cơ chế họ đang dùng, tạm vô hiệu tự restart rồi xác nhận không còn ghi. Không cung cấp agent lệnh launchctl thao tác live.
4. Nếu nguồn file: owner copy nguyên web-accounts.json và library/blob metadata cần thiết vào snapshot, giữ permissions/digests, không copy OAuth auth.json. Command có chế độ backup-only **đề xuất** (`storage sso-import --backup-only` với explicit source paths/backup-dir, không DSN/DB writes) để owner tạo `web-accounts.json.v4.bak` O_EXCL/0600 byte-equal **cạnh từng file nguồn gốc** khi mọi writer đã dừng, đúng cơ chế v2/v3. Sau đó nhập từ snapshot cũng có `.v4.bak` riêng; inventory liên kết digest nguồn gốc–backup–snapshot, không chỉ backup một bản sao rồi quên bảo vệ nguồn gốc.

### B. Backup DB trước schema và trước import

Owner cấu hình `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSFILE` qua secret store/file 0600 của role backup. Không đặt DSN/password trên argv hay shell history. `BACKUP` là thư mục explicit do owner đã tạo:

```sh
pg_dump --format=custom --file="$BACKUP/noverse-before-sso.dump"
pg_restore --list "$BACKUP/noverse-before-sso.dump"
```

Backup Viderse app DB riêng bằng cùng quy trình sau khi chọn đúng PGDATABASE/PGUSER; dù không thay schema nghiệp vụ, dump làm mốc đối soát. DBA lưu role/ACL cần tái tạo bằng quy trình backup cluster hiện có, không nhúng password role vào report. Ghi checksum dump và restore thử vào DB disposable; `pg_restore --list` **không thay thế** restore rehearsal.

### C. Expand schema, preflight, provision

1. Owner chạy `ainovel-cli storage migrate up` bằng MIGRATION_DATABASE_URL target Noverse; giữ STORE_DRIVER=postgres cho command đó. Không cấp DSN migrate cho app runtime.
2. DBA chạy `02-identity-role.sql` và `03-engine-worker-role.sql`, cấp login secret qua kênh riêng, kiểm SQL negative privileges bằng từng role. Noverse runtime giữ noverse_app, child dùng noverse_engine_worker; Viderse runtime identity dùng viderse_identity, data vẫn viderse_app.
3. Chạy storage sso-import --dry-run với hai source-id ổn định, snapshot paths, backend explicit và report/manifest. File mode thêm root+owner-map/blob config/target. Apply chỉ nhập từ snapshot đã đóng băng; backup-only ở A.4 là ngoại lệ owner chọn explicit file nguồn gốc. Không command nào tự dò HOME hoặc tự dừng dịch vụ.
4. Đọc toàn report: counts, collision/tombstone, admin mở rộng quyền, TTL, memberships, số session/pairing invalidated, snapshot/dump digest. Có conflict → dừng, chưa mở app; sửa rename map trên file riêng, không chỉnh live source, chạy preflight mới. DB đã có Noverse accounts không được bị coi là source file cần merge lại.

### D. Apply và đối soát khi app còn tắt

1. Owner chạy lại cùng command với `--apply --accept-manifest <digest> --backup-dir <dir>` và admin-list digest nếu cần. Tool verify `.v4.bak` byte-equal; failure trước/giữa apply → xem checkpoint, không xoá ledger hoặc đổi source-id để chạy lại.
2. Chạy reconcile theo report: user (kể cả tombstone)/credential/temp/recovery/grants/audit counts, project owner/membership, full Noverse facts/blob inventory; dữ liệu không liên quan không đổi. Dùng synthetic fixture hoặc owner login để kiểm hash, không xuất hash ra report.
3. File mode: full library/blob import phải complete và reconcile trước khi đổi STORE_DRIVER. Không bật Postgres với accounts-only target rồi coi thiếu story là UI bug.
4. Đặt env runtime: Noverse STORE_DRIVER=postgres, DATABASE_URL=noverse_app, NOVERSE_WORKER_DATABASE_URL=worker role; Viderse VIDERSE_DATABASE_URL=data role và VERSE_IDENTITY_DATABASE_URL=identity role; cả hai cùng CookieDomain/AppOrigins/Secure đúng topology. Migration DSNs không có trong runtime.

### E. Smoke đóng cửa và mở ghi

1. Owner khởi động hai app ứng viên bằng cấu hình mới, chưa mở public traffic/ghi nghiệp vụ ngoài smoke; readiness phải thật sự kiểm được DB. Dọn cookie cũ `ainovel_session`/`viderse_session` bằng browser hoặc expired Set-Cookie đúng scope, **không nhận chúng làm fallback**.
2. Mọi người đăng nhập lại. Thực hiện bảng smoke phase 05: hai chiều SSO, no-grant, revoke/logout/password, CSRF allowlist/non-allowlist, role negative và một thao tác facts Noverse/metadata Viderse. Không chạy model tốn tiền để chứng minh SSO.
3. Cổng mở ghi: report import/reconcile complete, backup restore rehearsal pass, smoke pass, owner xác nhận mapping/admin list. Ghi timestamp cutover và giữ binary/env/dump/snapshot cũ private. Chỉ lúc này mở traffic và cơ chế restart theo vận hành owner.

### F. Rollback có/không có dữ liệu mới

- **Chưa mở ghi hoặc import fail:** giữ hai app tắt. Identity tx thất bại thì không có account rows mới; library stage thất bại có ledger và có thể resume. Nếu rollback thay vì resume, restore target DB vào **DB khác**, đối chiếu rồi chuyển DSN/binary cũ; với nguồn file giữ nguyên source và dùng `.v4.bak` byte-equal nếu cần. Không `goose down`, không drop DB đang chứa dữ liệu, không chạy lại import bằng source-id mới.
- **Sau mở ghi:** đóng băng cả hai app; tạo dump/snapshot hậu-cutover trước restore. Ưu tiên quay binary tương thích schema additive, giữ Postgres làm nguồn sự thật. Không restore old DB/file mù quáng: sẽ hồi sinh password/session/grant đã thu hồi và mất facts mới. Lập diff user/password status/grants/membership/audit và dữ liệu app phát sinh, chốt reconcile/export trước khi quay file. Nếu không có đường reverse account export được kiểm, **không** tuyên bố rollback về file không mất dữ liệu; giữ maintenance, sửa forward trên DB hiện tại hoặc owner chấp nhận điểm phục hồi đã ghi rõ.
- Khi quay binary cookie cũ, expire `verse_session` cả host-only và Domain scope đã phát, revoke sessions liên quan ở kho vẫn dùng; người dùng đăng nhập lại. Không để hai phiên bản ghi hai kho song song.

## Ma trận validation

- **Unit/file:** exact v4 decode; truncated/newer/missing version; invalid username/hash/TTL/ref; case collision; tombstone collision; rename remap; Viderse project nonempty; byte-equal `.v4.bak`, O_EXCL, wrong prior backup, permission/ENOSPC; dry-run không source/DB write; report không secrets.
- **Postgres:** hai nguồn + target collisions fail toàn bộ identity; UUID/source ledger stable; rerun sau user đổi password không ghi đè; crash before commit rollback cả hai; commit xong report lỗi resume đúng; hai apply race; preserve used/expired temp/recovery; disabled_by reference không mất; grants đúng app; source admin approval; PG old sessions revoked.
- **Importer/full cutover:** file Noverse v4 có project/story/blob + Viderse file → full import → cả hai login/shared cookie; project permission giữ nguyên; không kéo dài TTL; kill giữa library/blob stage và resume không duplicate; reconcile không báo complete khi thiếu facts.
- **Rehearsal:** owner-runbook trên HOME/DB/snapshot tạm, dump và restore thật, rollback trước mở ghi, mô phỏng thay password/grants/facts sau mở ghi để chứng minh restore cũ phải bị chặn nếu chưa đối soát.
- Main chạy `REQUIRE_POSTGRES_TEST=1 go test ./apps/noverse/internal/storageimport/... ./apps/noverse/cmd/ainovel-cli/... ./platform/identity/...` với TEST_DATABASE_URL disposable. Không dùng `ACCOUNTS_V3_SRC` hoặc tài khoản người dùng thật cho tests.

## Kiểm bảo mật bắt buộc và test có tên

- `TestSSOCutoverInvalidatesOldSessions`: file bearer không nhập, PG session/pairing cũ revoked trong tx; rollback không để account mới nhưng phiên cũ vẫn hiệu lực. Cookie cũ hai tên không được backend mới nhận.
- `TestSSOUsernameCollisionFailsBeforeWrites`: case/tombstone/two-source/target conflict, không merge qua hash giống; report đủ nguồn và không DB write trước approve.
- `TestSSOAdminPromotionRequiresExplicitApproval`: Viderse admin mở quyền nền tảng chỉ apply với report digest được owner duyệt; role hạ trong mapping cũng explicit.
- `TestMigrationReportContainsNoSecrets`: report/manifest/log/error không chứa Argon2 string, recovery/temp hash, raw token, plaintext password hoặc DSN credentials; backup riêng 0600 vẫn chứa facts gốc cần bảo vệ.
- `TestSSOBackupIsExactAndNeverOverwritten`, `TestSSOResumePreservesPostImportPassword`: backup đúng byte/O_EXCL, rerun không hồi sinh credential/used temp hoặc grant đã thu hồi.
- `TestSSORollbackRequiresPostCutoverFreeze`: diễn tập có password/grant/facts mới; runbook không restore backup cũ trong lúc writers còn chạy và không tuyên bố rollback không mất dữ liệu.

Runbook phải nói rõ: **local** bind hai app127.0.0.1 và không dùng browser profile SSO với server local không tin cậy vì cookie bỏ qua cổng; **hosted** private container bind0.0.0.0 được nhưng HTTP app port không publish ra host/public, chỉ Traefik public, Domain+Secure bắt buộc. Giữ0600/0700 cho source/dump/report, không log command/env secret. Mở traffic chỉ sau security-reviewer phase05 đạt0 blocker/major.

## Rủi ro

| Rủi ro | Xác suất × tác động | Giảm thiểu và tín hiệu dừng |
|---|---|---|
| Trùng tên bị merge/skip im lặng | Cao × Rất cao | Inventory toàn nguồn+DB, strict unique/provenance, fail trước writes; zero unresolved collisions là gate |
| Bật PG làm Noverse file mất stories | Cao × Rất cao | File mode bắt buộc full library/blob import và reconcile, không accounts-only cutover |
| Admin Viderse thành admin Noverse ngoài ý muốn | Vừa × Rất cao | Report danh sách và explicit digest approval/mapping hạ role; thiếu approval không apply |
| Legacy importer hồi sinh mật khẩu tạm | Cao × Cao | V4 giữ exact expiry/used, không default grace; test used/expired |
| Source còn writer hoặc crash giữa DB/object store | Vừa × Cao | Owner maintenance, locks/digests, checkpoint/resume; không mở traffic cho tới complete |
| Restore snapshot cũ sau đổi password/grant | Vừa × Rất cao | Freeze + hậu-cutover dump + diff; ưu tiên forward fix, không hứa reverse export chưa có |

Phase chỉ hoàn tất khi command thật, tests và runbook diễn tập đạt; không đánh dấu xong bằng một script copy file hoặc account-only scaffold.
