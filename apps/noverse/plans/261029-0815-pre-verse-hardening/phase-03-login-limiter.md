---
phase: 3
title: "Giới hạn đăng nhập theo tài khoản không né được"
status: done — 2026-10-29 (commit 104388d)
dependencies: [1]
---

# Phase 03 — H14 và backoff của limiter Postgres

## Yêu cầu

- **H14.** Khoá limiter theo tài khoản dùng username đã chuẩn hoá: cắt khoảng trắng, chuyển chữ thường.
  - Hiện `auth.go` chỉ thêm khoá `user:` khi `ValidUsername` qua. Username viết hoa không qua bước kiểm này nên không có khoá `user:`, trong khi cột `citext` vẫn khớp đúng tài khoản. Kết quả: thử mật khẩu không giới hạn theo tài khoản, chỉ còn giới hạn theo IP.
- **M1, đi kèm.** `postgres/auth_limiter.go` phải tăng dần thời gian khoá giống limiter file, thay vì cố định 2 giây.

## Hướng sửa

1. Chuẩn hoá username một lần ở đầu handler đăng nhập, rồi dùng chính giá trị đó cho cả khoá limiter lẫn bước tra cứu.
2. Sửa limiter Postgres: tăng thời gian khoá theo số lần sai, như `webauth` limiter. Bỏ điều kiện `blocked.After(now)` đang làm reset backoff.

## Test hồi quy

- Sai mật khẩu nhiều lần, mỗi lần đổi kiểu viết hoa của cùng một username: phải nhận `429` theo khoá `user:`.
- Trên Postgres, thời gian chờ của lần khoá thứ n phải lớn hơn lần thứ n−1, cho tới mức trần.

## Kiểm tra

```bash
REQUIRE_POSTGRES_TEST=1 TEST_DATABASE_URL=… go test -count=1 ./internal/entry/web ./internal/persistence/postgres ./internal/webauth
```

## Kết quả

**Sửa:**
- `entry/web/auth.go`: chuẩn hoá username (cắt khoảng trắng, chuyển chữ thường) trước khi dựng khoá `user:` và trước khi ghi audit `RecordSecurityFailure`.
- `Accounts.Login` vẫn nhận nguyên input. Hành vi đăng nhập ở chế độ file không đổi: username viết hoa vẫn không đăng nhập được, nhưng giờ bị tính vào bucket của tài khoản.
- `postgres/auth_limiter.go`: số lần sai cộng dồn qua các lần khoá đã hết hạn, nên mỗi lần khoá sau dài gấp đôi lần trước. Bucket đã quá `expires_at` (20 phút không có lần sai nào) thì đếm lại từ đầu.

**Test hồi quy.** Cả hai fail trước khi sửa và qua sau khi sửa:

| Test | Lỗi trước khi sửa |
|---|---|
| `TestGatewayLoginRateLimitIgnoresUsernameCase` (`entry/web/gateway_test.go`) | `account must be locked after case-variant failures, got 200` |
| `TestAuthLimiterBacksOffAcrossBlocks` (`persistence/postgres/accounts_test.go`) | `failure after an expired block locked for 1.99s, want longer than the previous 1.999s` |

**Còn mở (Low):** `PurgeExpiredLimitBuckets` vẫn chưa có ai gọi, nên các bucket đã hết hạn còn nằm trong bảng. Chúng không còn tác dụng vì đã bị bỏ qua khi đếm.
