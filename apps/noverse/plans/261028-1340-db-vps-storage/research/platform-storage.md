# Research: Dokploy, goose/pgx, S3 và GCS interop

Đọc tài liệu chính thức trực tiếp ngày 2026-10-28; không nghiên cứu thêm stack ngoài phạm vi yêu cầu. Đây là documentation evidence, **không phải smoke test** của provider hoặc VPS.

## Dokploy

- [Applications](https://docs.dokploy.com/docs/core/applications): build bằng Dockerfile, Environment, Domains, volumes, health-check/Swarm settings, rollback khi deployment save đã bật. Dùng multi-stage Dockerfile hiện có làm gốc; không thêm Caddy. Traefik trong Dokploy quản TLS/domain→container port 8080. Không mở container HTTP trực tiếp ra Internet.
- [Databases](https://docs.dokploy.com/docs/core/databases): hỗ trợ Postgres và kết nối ứng dụng/database trong nền tảng. URL đoán `/docs/core/databases/postgres` trả 404; không lấy trang này làm bằng chứng. Runbook dùng dashboard Dokploy database + internal hostname thực được deployment cung cấp, không hardcode tên service.
- [Backups](https://docs.dokploy.com/docs/core/databases/backups): chọn S3 destination, database, cron/prefix, Test backup. Test backup không chứng minh restore; 09 bắt buộc restore Postgres mới và đối soát blob manifest. Backup database không tự backup volume local hoặc bucket blob.
- Health check và rolling deployment không thay cho migration ordering. Một one-shot migration service/job phải hoàn tất trước readiness. Migrate bằng role riêng, app chỉ kiểm tra schema version; chưa đúng version→not ready, không tự chạy DDL đồng thời mỗi web replica.

## goose và pgx

- [goose Provider](https://pressly.github.io/goose/documentation/provider/): `NewProvider` dùng `database/sql`, hỗ trợ embedded migrations, session locker Postgres, theo mặc định một transaction/migration; không trộn API Provider với globals goose. Dùng SQL migration `-- +goose Up/Down`; PL/pgSQL function bao `StatementBegin/StatementEnd`. Embedded migrations không thuộc writable runtime filesystem.
- [pgxpool docs source](https://raw.githubusercontent.com/jackc/pgx/master/pgxpool/doc.go): native pgx connection pool; app query/transaction bằng `pgxpool`, migration mở `sql.DB` qua `pgx/v5/stdlib`. Pool native và SQL pool migration riêng lifetime, migration pool đóng khi xong. `pkg.go.dev/.../pgxpool` bị reader trả trang trung gian Chess; không dùng làm bằng chứng API detail.
- Chọn pin pgx/v5 và goose/v3 tương thích Go 1.25 của `go.mod:3` khi implement, commit go.sum. Pool max conns/timeouts/env có giới hạn phù hợp một VPS; request context timeout, transaction rollback deferred và không giữ conn qua LLM/S3/ffmpeg. Không dùng connection session variable actor/project trên shared pool. App nhận DSN đã có TLS mode đúng môi trường; internal Dokploy network không đồng nghĩa Internet DSN được phép tắt TLS.
- `pg_trgm` cần extension do migration role tạo; dùng index title/name/content và query đã scope project. Search accent-insensitive không có sẵn chỉ vì cài pg_trgm; không hứa tính năng đó.

## AWS SDK Go v2: endpoint và checksum

- [Endpoint configuration](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html): dùng `s3.Options.BaseEndpoint` + default `EndpointResolverV2`, không hardcode hostname/request path hay thay default resolver S3 toàn bộ; S3 có xử lý virtual hosting riêng. `UsePathStyle` lấy từ env, endpoint URL là base HTTPS không ghép bucket trùng hai lần.
- [S3 checksums](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/s3-checksums.html): SDK S3 từ v1.74.1 mặc định CRC32 khi upload; provider tương thích không có nghĩa hỗ trợ mọi checksum trailer. Dùng SDK config request/response checksum `WHEN_REQUIRED` cho compatible endpoints khi cần theo documented compatibility, giữ SHA-256 nội dung độc lập và verify download; không bỏ integrity. Pin SDK, test hành vi multipart/checksum chứ không theo ETag.
- [R2 Go example](https://developers.cloudflare.com/r2/examples/aws/aws-sdk-go/): `BaseEndpoint=https://<account>.r2.cloudflarestorage.com`, region `auto`, explicit static credentials; `PresignGetObject` và PUT. R2 custom domain là delivery URL, không phải signing endpoint. Signed GET mặc định đi endpoint S3, không thay host sau ký.
- Multipart dùng aws-sdk-go-v2 S3 manager uploader, reader streaming/spooled file có giới hạn, part size >=5 MiB (trừ part cuối), concurrency/size cap cấu hình; abort upload khi context/error, có lifecycle cleanup incomplete uploads. Không giữ toàn bộ audio/export trong RAM. Local backend cùng contract: temp file+fsync+rename trong root và confined key, không giả có URL local từ xa.

## GCS XML/S3 interoperability

- [Interoperability](https://cloud.google.com/storage/docs/interoperability): endpoint XML `storage.googleapis.com`; HMAC access-id/secret, không dùng service-account JSON private key vào S3 secret. HMAC quản qua GCS API/console, bucket private theo IAM.
- [Simple migration](https://cloud.google.com/storage/docs/aws-simple-migration): config endpoint, HMAC, signature v4 và region; ví dụ dùng `us-east-1`. Dùng `S3_REGION=us-east-1` mặc định cho GCS interop, không sao chép `auto` từ R2. Signature v4 và public URL test riêng với Go SDK; tài liệu không tự chứng minh presign mọi cách ký đều tương thích.
- [Multipart uploads](https://cloud.google.com/storage/docs/multipart-uploads): XML multipart có tối đa 10.000 parts, 5 MiB–5 GiB/part (ngoại lệ part cuối), object max 5 TiB; giữ incomplete parts đến complete/abort và hỗ trợ lifecycle `AbortIncompleteMultipartUpload`. GCS XML multipart không hỗ trợ preconditions; **không dùng If-None-Match multipart làm khóa concurrent write**, dùng immutable UUID/hash keys + DB CAS/manifest. Multipart ETag không phải MD5; metadata/size có giới hạn. Không nhầm XML multipart với JSON resumable upload hoặc POST form upload.

## Provider matrix và env

Một driver `s3`, không bảy code path. Mỗi row là **recipe cấu hình**, không tuyên bố đã integration-test provider. Các endpoint bucket/account/region thật lấy từ provider credentials; fixtures local/MinIO là gate bắt buộc, provider được dùng production phải qua smoke trước cutover.

| Provider | S3_ENDPOINT | S3_REGION | FORCE_PATH_STYLE | Credentials/ghi chú |
|---|---|---|---|---|
| AWS S3 | để trống (SDK resolve) | region bucket | false | IAM access/secret, SigV4 |
| Cloudflare R2 | `https://<account>.r2.cloudflarestorage.com` | `auto` | false (test bucket DNS) | R2 access/secret, GET ký bằng endpoint S3 |
| GCS | `https://storage.googleapis.com` | `us-east-1` | true | HMAC keys, XML multipart; smoke checksum/trailer/presign |
| MinIO | `http://minio:9000` chỉ trong test/private network | `us-east-1` hoặc region đã đặt | true | minio access/secret |
| Wasabi | endpoint regional do provider cấp | region bucket | false | Không đoán region từ tên bucket |
| Backblaze B2 | endpoint S3 của bucket | region trong endpoint | false | application key có quyền S3 đúng bucket |
| DigitalOcean Spaces | endpoint region, không phải CDN | region space | false | Spaces access/secret |

Required env contract: `STORAGE_DRIVER`, `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_FORCE_PATH_STYLE`, `S3_PUBLIC_BASE_URL`; local dùng `BLOB_LOCAL_ROOT`. `STORAGE_DRIVER=local|s3` độc lập `STORE_DRIVER=file|postgres`. Parse bool nghiêm ngặt, không log secrets/DSN. Driver S3 bắt buộc bucket/region/credentials; endpoint chỉ cho empty ở AWS. Cho HTTP rõ ràng chỉ MinIO private/test theo validation flag; không âm thầm chuyển HTTPS xuống HTTP.

`S3_PUBLIC_BASE_URL` optional cho **public** asset; private blob luôn authorization + presign/fallback. Không biến cả bucket public vì muốn nghe audio. Nếu provider presign không hỗ trợ hoặc browser không tới endpoint private, server dùng authenticated streaming HEAD/GET với Range, Content-Length, Content-Type, ETag ứng dụng theo SHA-256, Content-Disposition an toàn; không redirect sang URL local hoặc tự sửa host đã ký. Nếu presign trả lỗi trong browser, client retry qua streaming endpoint một lần có quyền, không vòng lặp fallback vô hạn. Cấu hình CORS chỉ origin thật; streaming fallback cho private nội dung và tránh yêu cầu CORS quá rộng.

## Các kiểm chứng vẫn chưa thực hiện

Không truy cập VPS, bucket thật hoặc Postgres/MinIO trong lượt này. Chưa chọn provider production, hostname hoặc secrets. Không có lệnh test/build thực thi. Các giới hạn provider/provider feature ở bảng recipe phải được conformance + smoke xác nhận khi có credentials; riêng local và MinIO bắt buộc cùng suite, không ghi nhà cung cấp khác đã đạt.
