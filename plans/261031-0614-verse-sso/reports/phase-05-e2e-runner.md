# Two-app SSO runner execution report

## Result

`npm run test:sso` ran end-to-end: **19 / 19 scenarios passed** (Playwright
`list` reporter, single worker, macOS arm64, `dat` workstation, Go 1.26.0,
Playwright 1.64.1 Chromium 149). Real `ainovel-cli`/`viderse` binaries built
from the working tree, real runtime roles (`noverse_migrate`, `noverse_app`,
`viderse_migrate`, `viderse_app`, `viderse_identity`, `noverse_engine_worker`)
on a private `initdb` throwaway cluster. `SSOHostedCookieNavigation` ran the
full local HTTPS parent-domain rehearsal (`tests/sso/fixture` TLS proxy,
`Domain=verse.test` cookies through `--host-resolver-rules`); it is not skipped.
Latest full run against the final runner helpers: **19 passed (2.4m)**, including `SSOImportedAccountsAndRollback`.

## Commands

```sh
npm run test:sso
npx tsc -p tests/sso/tsconfig.json
GOTOOLCHAIN=go1.26.0 go build -o /dev/null ./tests/sso/fixture/
GOTOOLCHAIN=go1.26.0 go vet ./tests/sso/fixture/
GOTOOLCHAIN=go1.27.0 go build -o /dev/null ./tests/sso/fixture/
GOTOOLCHAIN=go1.27.0 go vet ./tests/sso/fixture/
GOTOOLCHAIN=go1.26.0 $(GOTOOLCHAIN=go1.26.0 go env GOROOT)/bin/gofmt -l tests/sso
GOTOOLCHAIN=go1.27.0 $(GOTOOLCHAIN=go1.27.0 go env GOROOT)/bin/gofmt -l tests/sso
```

The suite must not be invoked under `lockf -k /tmp/verse-heavy.lock`: builds
inside the harness acquire that lock themselves and `lockf -k` does not nest.

## Scenario results (observed)

| Scenario | Result | Key observed statuses |
| --- | --- | --- |
| SSOLocalCookiePortScope | PASS | one host-only `verse_session` on 127.0.0.1, SameSite=Strict, HttpOnly, not Secure; second page no login form; `/api/me` 200 + `/api/library` 200 + `/api/status` 200 |
| SSOReverseLogin | PASS | Viderse login → Noverse `/api/me` 200, identical user+grants; `e2eproj` visible via membership |
| SSOLocalHostBoundary | PASS | `http://localhost:<port>/api/me` → 401; cookie domain `127.0.0.1`, no leading dot |
| SSOHostedCookieNavigation | PASS | `Domain=.verse.test`, Secure+Strict+HttpOnly; sibling `viderse.verse.test` opens sessioned; `khac.other.test` → 401; forged `X-Forwarded-Proto: http` still gets Secure cookie; foreign Origin → 403 |
| SSONoverseOnlyMember | PASS | Viderse `/api/me` + `/api/status` → 403 `{code:"app_access_denied", app:"viderse"}`; cookie stays; Noverse 200; `/api/events` → 403; `/api/me/sessions` 200 (neutral) |
| SSOViderseOnlyMember | PASS | Noverse login → 403 `app_access_denied` (not 401, cookie set); `/api/library` 403; Viderse 200 |
| SSOGlobalAdminAndGrantMutation | PASS | admin effective grants `{noverse:admin, viderse:admin}`, 0 `app_grants` rows; member PUT → 403; grant → target sessions 401 both apps; CLI grant idempotent (no revoke, no audit); CLI revoke → 401 then login 403; 2 audit rows (actor boss + system); project membership unchanged |
| SSORevocationAcrossBothApps | PASS | `/api/sessions/{id}/revoke` from Noverse → both apps 401; repeated from Viderse → both 401 |
| SSOLogoutEverywhere | PASS | `/api/logout` on Viderse → both apps 401; `/api/me/sessions/revoke-all` `{reauthenticate:true}` from either app → all device jars 401 on both |
| SSOPasswordChangeRevokesAll | PASS | change on Noverse and on Viderse → `{reauthenticate:true}`, every old session 401 on both apps, old password generic 401, new password 200 |
| SSOAdminResetAndDisable | PASS | admin reset → sessions 401 both apps, temp login forces change; disable → 401 both apps, login 401; re-enable does not resurrect sessions |
| SSOForcedPasswordBothApps | PASS | identical contract both apps: `/api/me` `mustChangePassword:true`, business GET/POST → 403 `{code:"password_change_required", mustChangePassword:true}`; change with consumed temp → 200; relogin flag false |
| SSOConcurrentSetupOneAdmin | PASS | fresh identity DB, both codes accepted verify; concurrent `/api/setup/admin` → exactly `[200, 409]`; `SELECT count(*)=1`; loser `/api/setup` → `{needed:false}`; dropped DB → 503 on both apps |
| SSOCSRFNonAllowlistedOrigin | PASS | third-port/foreign/`null`/lookalike Origin on POST/PUT/PATCH/DELETE → 403 `yêu cầu khác origin bị từ chối` before handler, no CORS header, no forged user; allowlisted sibling passes CSRF layer; same on Viderse |
| SSOCRSFRealBrowserForm | PASS | auto-submitted form from a third loopback origin → POST `/api/users` 403, no `Access-Control-Allow-Origin`, no account created |
| SSOReturnTargetBoundary | PASS | `#//evil.example`, `#/login?next=https://evil.example`, `?next=//evil.example`, `#/x?next=javascript:…` all stay on `http://127.0.0.1` origin; no token in URL; hash router works |
| SSOAlternatingLoginRateLimit | PASS | unknown user == wrong-password (identical 401 body); 4 alternating fails → 429 `retryAfter` on BOTH apps; `login_rate_limits` rows present; budget unchanged across Noverse restart; next fail yields longer block; success clears |
| SSORoleAndWorkerBoundary | PASS | `viderse_identity`: `users` readable, `projects/stories/project_members` + `global_role/status` writes denied; `noverse_engine_worker`: all account tables denied, `story_leases`/`projects` readable; real worker child served `/snapshot` 200, visible in `pg_stat_activity` as `noverse_engine_worker`, env = worker DSN + `AINOVEL_STORY_SCOPE` only |
| SSOImportedAccountsAndRollback | PASS | collision dry-run exit 1 (`two_sources`), users=0/no `.v4.bak`; clean dry-run exit 0, both source admins inventoried; wrong admin approval exit 1 (`approval_mismatch`), users=0/no backups; real custom-format dump as `noverse_migrate`; apply exit 0, 5 users, byte-equal 0600 account backups, inventory dump SHA matches; fresh-dump resume exit 0, 5 skips/0 inserts, unchanged member password hash; both admins log in on both apps with admin/admin effective grants; each member's own app 200/other app 403 `app_access_denied`; temporary-password login 200/business GET 403 `password_change_required`; old file session 401 on both apps; wrong-password and unknown-user generic 401 bodies equal; original dump restored into new DB has 0 users, restore DB dropped |

## App bug found and fixed

**`platform/webguard/webguard.go:190`** — `ParseCookieDomain` used
`strings.ContainsAny(domain, "/:@?#&= \\t")`. The literal held a
double-escaped `\\t` (backslash + `t`), so `ContainsAny` forbade the letter
`t`: **any** `VERSE_COOKIE_DOMAIN` containing `t` (`verse.test`, every `.net`)
failed with `không phải tên miền (cấm scheme, port, userinfo, wildcard)`.

- Request: start `ainovel-cli serve` or `viderse serve` with `PUBLIC_URL=https://noverse.verse.test:PORT` and `VERSE_COOKIE_DOMAIN=verse.test`.
- Expected: config accepted; hosted cookie `Domain=verse.test`.
- Actual before the fix: server exits, `Lỗi serve: cấu hình proxy: VERSE_COOKIE_DOMAIN="verse.test" không phải tên miền …`.
- Fix: the literal now holds a real tab (`"/:@?#&= \t"`); the per-label `[a-z0-9-]` check stays the authority. Regression cases in `TestCookieDomainConfig` (`verse.net`, `studio.test`, `tet.vn` accepted; an embedded tab refused).
- The fixture's hosted parent domain moved from the temporary `verse.priv`/`other.priv` to the reserved `verse.test`/`other.test`, so the E2E run exercises the fixed path.

## Harness issues fixed during bring-up (no product changes)

- `Set-Cookie` seeding used `headers.get()` (collapsed); now `getSetCookie()`.
- `CREATE ROLE`-style guard moved behind a private `initdb` cluster, so the
  developer cluster's pre-existing runtime roles are never touched.
- Setup-code parsing reads unredacted child output; `srv.log()` stays redacted.
- `provisionRaceDB` reuses the full 01-roles.sh + migrations + 02/03 role SQL
  pipeline; teardown kills worker-spawned pgids via `pids.txt`, drops only
  `verse_sso_<suffix>` databases, drops roles only when this run created them.
- `psqlAs` now connects via PG* env vars instead of a URL argument, so role
  DSNs never appear on a command line.

## Cleanup verification

After the passing run, a scoped check of the OS temp root found **0**
`verse-sso-*` directories. A captured `ps -axo pid=,command=` inspection,
reporting counts only, found **0** app/proxy/worker/private-Postgres processes
whose executable or data path belonged to those roots. The private cluster's
data directory was absent, so its databases and runtime roles did not persist.
The manual debug database `sso_imp_dbg` was separately checked in
`pg_database`: count **0**. Manual fixture/debug paths were also absent.

## Evidence hygiene

Traces/videos/screenshots are disabled; `state.json` is private and deleted.
The suite consumes the file-store token in memory and uses boolean assertions
for bearer/password-hash comparisons; server failure output is redacted.
Role passwords reach the SQL scripts through environment variables, and
password-bearing provisioning SQL travels on stdin rather than argv.

**Execution concern:** an earlier ad-hoc fixture smoke command used disposable
passwords in its command text and printed its disposable session token. Those
credentials and their backing files were destroyed; no live account or data
was involved. No secret values are copied into this report. Consequently the
strict requirement that *no* secrets appeared in *any* stdout/argv during the
entire assignment was not met, even though the final suite path avoids that
exposure.

## Imported-account rehearsal details

The Go fixture uses only the public `platform/identity` file-store API to
create the v4 sources, including a real argon2id temporary password and one
live member session. The colliding Viderse variant is independent of the clean
source. Passwords are generated per suite run and passed only via environment.
The imported pair reuses the suite's fresh Viderse data DB and starts new
Noverse/Viderse processes against the separate imported identity DB.

File mode requires `--root`; an empty library snapshot exercises the real
library pass without populated library fixtures. The optional library facts
check was omitted. The original before-state dump is retained unchanged for
restore; a second fresh dump is taken before resume after crossing a
whole-second boundary, matching the strict archive-freshness gate.

The first full attempt observed 18 passes and the new scenario failing at the
initial apply (exit 1), while importer backup verification was being updated.
A separate temporary-database rehearsal with the current importer then
observed dry-run/apply exits 0. A full run passed 19/19 in 16.2m; after the
runner's password-in-argv cleanup, the final complete run observed:

```text
Running 19 tests using 1 worker
  19 passed (2.4m)
```

TypeScript, fixture builds/vet on both Go toolchains, and both explicit
toolchain `gofmt -l` commands completed successfully with no output.
Additional diagnosis used `go run ./tests/sso/fixture -mkaccounts`, `createdb`,
`storage migrate up`, `storage sso-import` dry-run/apply, `pg_dump`, and
`dropdb` against the temporary manual rehearsal; credential-bearing command
details are intentionally omitted.
