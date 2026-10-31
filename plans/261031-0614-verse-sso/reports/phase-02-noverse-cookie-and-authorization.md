# Noverse: shared cookie, grant gate, CSRF and worker secrets — report

Server side only (`apps/noverse/**` Go/SQL/deploy, `platform/webguard/**`,
`platform/identity/identitycli/**`, `platform/identity/postgres/pool.go`,
`go.mod`). UI/`apps/noverse/web/**` is owned by the web track and reported
separately.

## What changed

### Cookie + CSRF contract
- `platform/webguard/webguard.go`: `Config.CookieDomain`, `Config.AppOrigins`,
  `ParseCookieDomain` (public-suffix parser, DNS-label boundary vs
  `PUBLIC_URL`), `parseAppOrigins`, `canonicalOrigin`, exact
  `allowedListedOrigin`, `Hosted()`, `LocalSSO()`; `AllowedOrigin` = own origin
  ∪ explicit allowlist; `CrossSiteRequest` rejects `Sec-Fetch-Site: cross-site`
  before `Origin`, and a missing `Origin` keeps the non-browser convention.
  Startup fails on malformed entries; no wildcard/suffix matching; no CORS.
- `platform/webguard/session-cookie.go`: `SessionCookie{Set,Clear,Expire}` —
  Path `/`, HttpOnly, SameSite Strict, `Secure` from the boundary
  (`SECURE_COOKIES`/normalized HTTPS), Domain only when hosted, clear repeats
  the exact Name/Path/Domain, `Expire` retires legacy names at both scopes.

### Noverse wiring
- `apps/noverse/internal/entry/web/gateway.go`: explicit `accountBackend`
  (`backendFile` → `ainovel_session`, `backendPostgres` → `verse_session`) —
  `NewGateway` / `NewPostgresGateway`, never guessed from `Path()`; grant gate
  after `Authenticate`, before feedback/dispatch; `workerEnvPassthrough` +
  `workerEnv` allowlist builder used by local and hosted launches;
  `hostedWorkerEnv` fails closed without `NOVERSE_WORKER_DATABASE_URL` and
  never hands the app DSN to the child.
- `auth.go`: `cookiePolicy()`, `setSession` (expires `ainovel_session` +
  `viderse_session` at both scopes on shared login), `appAccessGranted`
  (`User.Grants[noverse]`), `writeAppAccessDenied`
  (`{code,app,error,user}` 403), `sessionOnlyRoute`, `serveSessionOnly`
  (self-only logout/password/own sessions), `doLogout` (revoke → clear),
  `doRevokeAll` / `doPasswordChange` (`{ok:true,reauthenticate:true}`), grant
  endpoints `PUT|DELETE /api/users/{u}/grants/{app}` with `decodeJSON`
  rejecting unknown fields (so `global_role`/`projects`/`actor` in the body is
  a 400, never a silent widen), `stripFileGrants`.
- `authorization.go`: `passwordNeutralRoute` (exact method+path, mirrors
  `sessionOnlyRoute`); stream re-check now requires the Noverse grant.
- `events.go`: session recheck requires the grant.
- `httperr.go`: `dependencyUnavailable` → 503 for `pgconn`/net/closed-pool
  errors, so a DB failure is never reported as 401/500.
- `recovery.go`, `setup.go`, `notify.go`: same cookie policy; no-grant
  recovery answers 403 after setting the cookie.
- `platform/identity/postgres/pool.go` and
  `apps/noverse/internal/persistence/postgres/pool.go`: `sanitizeErr` now
  returns a `sanitizedError` that unwraps to the cause — text stays redacted,
  classification (`errors.Is/As`) still works.
- `apps/noverse/cmd/ainovel-cli/serve.go`: file backend rejects
  `VERSE_COOKIE_DOMAIN`/`VERSE_APP_ORIGINS`; local SSO requires a loopback bind
  (`ssoLoopbackOnly`) and suppresses `--lan`; `serve user grant|revoke --app`.
- `apps/noverse/internal/bootstrap/storage.go`: `WorkerURL` from
  `NOVERSE_WORKER_DATABASE_URL`, postgres-only, validated.
- `platform/identity/identitycli`: shared operator parser/service (no
  `--actor`, no `--dsn`, no role escalation, no user creation).
- Deploy: `deploy/postgres-init/03-engine-worker-role.sql` rewritten to the
  facts/lease table list with `chapter_versions` SELECT/INSERT only, NOINHERIT,
  membership revocation, converge-on-rerun, explicit `GRANT CONNECT`, and a
  psql `\if` skip when the password variable is absent (initdb hook safe).
  `.env.example`, `deploy/README.md` (role provisioning, env table, SSO/CSRF
  security notes), `dsn-entrypoint.sh` (`worker` DSN target), compose worker
  credentials — all updated.
- `go.mod`: `golang.org/x/net`, `github.com/jackc/puddle/v2`,
  `gopkg.in/yaml.v3` promoted to direct `go mod tidy` dependencies.

## Decisions

- **Admin cross-user revoke preserved.** `serveAccount` keeps
  `owner == user.Username || user.Role == admin`; the no-grant branch
  (`serveSessionOnly`) is self-only. Because a global admin always carries an
  effective grant, admins never reach the no-grant branch — the spec's "the
  no-grant branch may only touch its own sessions" and "an admin with a grant
  keeps the ability" are both satisfied.
- **Password change is one implementation.** `serveAccount`'s
  `/api/me/password` now delegates to `doPasswordChange`, so the granted path
  clears the cookie and returns `reauthenticate:true` (both backends revoke
  every session).
- **File-mode grants are `null`.** `User()` fills `fileGrants` internally so
  the gate stays inert, but publishing `{"noverse":"member"}` would fabricate a
  shared-store contract. File-mode user payloads emit `"grants": null`
  (distinct from `{}` = shared store, no rows). Version hashing
  (`resourceVersion`/`currentResource`) hashes the same stripped view, so
  per-user If-Match tokens still match.
- **Worker env is an allowlist, not `os.Environ()`.** Provider/model config is
  file-based under `HOME`; no API-key env is read on the worker path.
- **Worker role has no default privileges and no sequence grants** (facts keys
  are UUIDs generated in Go); `chapter_versions` is append-only.

## Named tests (all passing)

| Test | File | Result |
|---|---|---|
| `TestSharedCookiePolicy` | `platform/webguard/session-cookie_test.go` | PASS |
| `TestSSOOriginAllowlistMatrix` (config) | `platform/webguard/session-cookie_test.go` | PASS |
| `TestSSOOriginAllowlistMatrix` (HTTP: POST/PUT/PATCH/DELETE × local/hosted × own/allowlisted/non-allowlisted/foreign/null/missing × same-origin/same-site/cross-site/missing, DB-unchanged) | `apps/noverse/internal/entry/web/sso_grants_test.go` | PASS |
| `TestCookieDomainConfig`, `TestAppOriginsConfig` | `platform/webguard/session-cookie_test.go` | PASS |
| `TestSSOLocalBindRequiresLoopback` | `apps/noverse/cmd/ainovel-cli/serve_sso_test.go` | PASS |
| `TestHostedAppHTTPPortIsNotPublished` | `apps/noverse/cmd/ainovel-cli/serve_sso_test.go` | PASS |
| `TestAppGrantHTTPRequiresGlobalAdmin` | `apps/noverse/internal/entry/web/sso_grants_test.go` | PASS |
| `TestGrantCLIOperatorBoundary`, `TestGrantCLIPostgresOperator` | `apps/noverse/cmd/ainovel-cli/serve_sso_test.go` | PASS |
| `TestGrantCLIParse` (grammar incl. actor/DSN/role rejection) | `platform/identity/identitycli/grants_test.go` | PASS |
| `TestSharedLogoutRevokesBeforeClear` (incl. DB-down not a fake success) | `apps/noverse/internal/entry/web/sso_grants_test.go` | PASS |
| `TestSSOLoginClearsLegacyCookies` (both names, both scopes, no reuse, no legacy read) | `apps/noverse/internal/entry/web/sso_grants_test.go` | PASS |
| `TestForcedPasswordGateEveryBusinessRoute` | `apps/noverse/internal/entry/web/sso_grants_test.go` | PASS |
| `TestHostedWorkerEnvExcludesIdentitySecrets` | `apps/noverse/internal/entry/web/sso_grants_test.go` | PASS |
| `TestWorkerRoleCannotReadIdentity`, `TestWorkerRoleScriptSkipsWithoutPassword` | `apps/noverse/internal/persistence/postgres/worker_role_test.go` | PASS |
| `TestIdentityErrorsAndLogsRedactSecrets` (HTTP layer: 401/403/503 bodies + server log, no password/token/DSN; DB-down 503 keeps the cookie) | `apps/noverse/internal/entry/web/sso_grants_test.go` | PASS |
| `TestIdentityErrorsAndLogsRedactSecrets` (repository layer) | `platform/identity/postgres/redact_test.go` | PASS |
| `TestNoverseGrantGateMatrix` (handler matrix: granted member, no-grant 403 with `user.grants`, admin, writes/SSE/admin surfaces blocked, DB unchanged, self-only session revoke, no-grant logout still revokes) | `apps/noverse/internal/entry/web/sso_grants_test.go` | PASS |
| `TestAccountsBackendLoginMeMemberLogout` | `apps/noverse/internal/entry/web/accounts_backend_test.go` | PASS (fixed: helpers sent `ainovel_session` to a Postgres gateway) |
| `TestResourceVersionUsersItemMatchesUpdate` | `apps/noverse/internal/entry/web/version_test.go` | PASS (grants stripped before hashing) |

## Gates

```sh
go mod tidy                     # ok (3 direct deps promoted)
go build ./...                  # ok
go vet ./...                    # ok
go test -p 2 ./archtest/... ./platform/webguard/... ./platform/identity/... \
  ./apps/noverse/internal/bootstrap/... ./apps/noverse/cmd/ainovel-cli/...   # all ok
go test -p 2 ./...              # only apps/noverse/internal/persistence/migrate
                                #   TestMigrateApplicationRoleCannotDDL (known,
                                #   pre-existing, out of scope)
go tool sqlc -f apps/noverse/sqlc.yaml generate                       # no drift
go tool sqlc -f platform/identity/postgres/sqlc.yaml generate         # no drift
gofmt -l <changed files>        # clean under go1.26.0 and go1.27.0
```
Postgres env for every run: `TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test REQUIRE_POSTGRES_TEST=1`.

Full-suite result: `go test -p 2 ./...` fails **only**
`apps/noverse/internal/persistence/migrate` →
`--- FAIL: TestMigrateApplicationRoleCannotDDL`, the documented pre-existing
failure. Every other package returns `ok`.

## Real-binary smoke (127.0.0.1:18800, temp HOME, DB `sso_smoke_p2`)

```sh
go build -o /tmp/ainovel-sso-smoke ./apps/noverse/cmd/ainovel-cli
createdb sso_smoke_p2
STORE_DRIVER=postgres DATABASE_URL=$DSN MIGRATION_DATABASE_URL=$DSN \
  /tmp/ainovel-sso-smoke storage migrate up      # 00007…00012 applied
HOME=/tmp/sso-smoke-home STORE_DRIVER=postgres DATABASE_URL=$DSN \
  NOVERSE_WORKER_DATABASE_URL=$DSN BLOB_LOCAL_ROOT=/tmp/sso-smoke-blobs \
  HTTP_ADDR=127.0.0.1:18800 /tmp/ainovel-sso-smoke serve
```

Trimmed evidence:

```
Mã thiết lập: FSZ5-EVHZ
POST /api/setup/verify {"code":"FSZ5-EVHZ"} → {"setupToken":"cd70…"}
POST /api/setup/admin → 200
Set-Cookie: ainovel_session=; … Max-Age=0        (legacy expired, host-only)
Set-Cookie: viderse_session=; … Max-Age=0        (legacy expired)
Set-Cookie: verse_session=b223e61f-….b93152d2…; Path=/; Expires=Mon, 30 Nov 2026; Max-Age=2591999; HttpOnly; SameSite=Strict
POST /api/login (admin) → 200, new verse_session id (no reuse)
POST /api/me  Origin: https://evil.example → 403 {"error":"yêu cầu khác origin bị từ chối"}
POST /api/users {"username":"ghost"} (foreign Origin) → 403, no user created
POST /api/users {…"sibling"…} → 201 grants {"noverse":"member"}
PUT /api/users/sibling/grants/noverse {"role":"admin"} → 400 (member only)
DELETE /api/users/sibling/grants/noverse → 200 grants {}
POST /api/login (sibling) → 403 app_access_denied + Set-Cookie verse_session
GET /api/me (that cookie) → 403 {"code":"app_access_denied","app":"noverse",…"user":{…}}
GET /api/projects (no grant) → 403 app_access_denied
POST /api/logout (no grant, {}) → 200;  GET /api/me after → 401
serve user grant sibling --app viderse  → "Đã cấp quyền dùng viderse cho \"sibling\"."
serve user revoke sibling --app viderse → "Đã thu hồi quyền dùng viderse cho \"sibling\"."
serve user grant sibling                → exit 1 ("thiếu --app noverse|viderse")
```

Cleanup: server stopped, `dropdb sso_smoke_p2`, temp HOME/blobs/log/binary
removed; nothing listening on 18800 afterwards.

## Deviations

- The no-grant `POST /api/logout` smoke initially returned 400
  (`Content-Type phải là application/json`) because the curl call omitted the
  JSON body; the pre-existing contract requires `{}` for every POST. With `{}`
  it returns 200 and revokes the session (also covered by
  `TestNoverseGrantGateMatrix`).
- `TestAppGrantHTTPRequiresGlobalAdmin` expects the forged-body case to be a
  400 rather than "silently ignored": `decodeJSON` rejects unknown fields, so
  `global_role`/`projects`/`actor` never reach the mutation at all.
- The `/api/me/password` handling was previously duplicated in
  `serveAccount`; it now delegates to `doPasswordChange` (same wire contract as
  the no-grant branch).
