# Two-app SSO smoke suite

End-to-end runner for the shared `verse_session` login between Noverse and
Viderse. It starts **real** `ainovel-cli` and `viderse` binaries against **real**
Postgres runtime roles and drives both UIs and APIs with one Playwright browser
context. No mocks, no test-only auth bypass, no model/engine calls.

```sh
npm run test:sso
```

Prerequisites: Node 22+, npm, Go 1.26.0, PostgreSQL client/server binaries
(`psql`, `initdb`, `pg_ctl`, `pg_dump`, `pg_restore`), macOS `lockf`, and Chromium installed with
`npx playwright install chromium`. Builds acquire `/tmp/verse-heavy.lock`
internally: do **not** wrap the entire command in that same lock.

The first run builds both web bundles and the three Go binaries, so expect a
few minutes; later runs reuse nothing (each run starts from an empty temp dir).

For a held fixture during manual UI checks, after `npm ci` run:

```sh
node_modules/.bin/jiti tests/sso/serve-fixture.ts
```

Wait for `SSO_FIXTURE_READY <path>/state.json`; that private file contains
`urls.noverse`, `urls.viderse` and disposable `users.admin` credentials. Point Vite's
`VITE_API_TARGET` at the relevant backend. Keep credentials out of logs and
screenshots. Ctrl-C stops the fixture and runs the same teardown. This command
provisions its own throwaway cluster; it does not run or claim the smoke tests.

## What it provisions

Everything is throwaway and named with a random suffix:

| Resource | How |
| --- | --- |
| Postgres cluster | `initdb` into the run temp dir, loopback port, trust auth, `dat` superuser. Started/stopped with `pg_ctl`; removed with the temp dir. |
| Identity DB | `verse_sso_<suffix>`, roles from `apps/noverse/deploy/postgres-init/01-roles.sh`, goose migrations via `ainovel-cli storage migrate up` as `noverse_migrate`, then `02-identity-role.sql` (`viderse_identity`) and `03-engine-worker-role.sql` (`noverse_engine_worker`). |
| Viderse data DB | `verse_sso_v_<suffix>`, owner/DDL `viderse_migrate`, DML `viderse_app`, migrated with `viderse migrate up`. |
| Race DB | `verse_sso_<suffix>_race`, same pipeline, used by the concurrent-setup and DB-down scenarios. The cutover scenario provisions a second one (`_imp`) through the same helper and drops a `pg_restore` rehearsal DB (`_rst`) itself. |
| Binaries | `ainovel-cli`, `viderse`, `ssoproxy` (TLS fixture) built into the run temp dir. |
| Servers | Noverse + Viderse in local mode on free loopback ports, plus a second hosted-mode pair (`VERSE_COOKIE_DOMAIN=verse.test`, `SECURE_COOKIES`, `TRUSTED_PROXIES=127.0.0.1/32`, `PUBLIC_URL`) behind `tests/sso/fixture` (TLS proxy with an in-memory throwaway CA). |
| Accounts | First admin through the real `/api/setup/verify` + `/api/setup/admin` flow, then fixture members created via `/api/users` and `/api/users/{u}/grants/{app}`. |

## Teardown

`global-teardown.ts` stops every process group it started (including instances
created by tests), drops the run's databases and roles, stops the private
cluster and deletes the run temp dir. `dir/pids.txt` and `dir/cleanup.sh` in the
run dir are the crash fallback: run `cleanup.sh` if the runner is killed.

## Safety guards

* Refuses to run when the repo or temp root path contains `verse-live`,
  `.ainovel`, or `.viderse`.
* Starts its **own** cluster, so fixed runtime role names never collide with
  the developer or hosted cluster. `TEST_DATABASE_URL` alone does not disable
  this isolation. When `VERSE_SSO_ADMIN_DSN` explicitly selects an existing
  cluster, the fixture refuses to run if any of the six runtime roles already
  exist; it never rotates a pre-existing role's password.
* An existing admin DSN must be loopback (`127.0.0.1`, `::1`, `localhost`) and
  `postgres://`.
* The app pair binds IPv4 loopback; the host-boundary scenario uses `localhost`
  versus `127.0.0.1`. Those names never share a host-only cookie.
* Every child process gets `HOME` inside the run temp dir, never the real one.

## Evidence hygiene

* Traces, videos and screenshots are off; auth bodies are never written to disk.
* Server output is passed through `redact()` before it can reach a failure
  message (DSNs, setup codes, passwords).
* `state.json` holds throwaway credentials for this run only: it lives in a
  `0700` temp dir, is written `0600`, and is deleted on teardown.
* The TLS fixture CA is generated per run, never installed in any trust store;
  the browser uses `ignoreHTTPSErrors` for the rehearsal only.

## Scenarios

`SSOLocalCookiePortScope`, `SSOReverseLogin`, `SSOLocalHostBoundary`,
`SSOHostedCookieNavigation`, `SSONoverseOnlyMember`, `SSOViderseOnlyMember`,
`SSOGlobalAdminAndGrantMutation`, `SSORevocationAcrossBothApps`,
`SSOLogoutEverywhere`, `SSOPasswordChangeRevokesAll`,
`SSOAdminResetAndDisable`, `SSOForcedPasswordBothApps`,
`SSOConcurrentSetupOneAdmin`, `SSOCSRFNonAllowlistedOrigin`,
`SSOCSRFRealBrowserForm`, `SSOReturnTargetBoundary`,
`SSOAlternatingLoginRateLimit`, `SSORoleAndWorkerBoundary`,
`SSOImportedAccountsAndRollback`.

V26/AO assertions cover `apps: [{id,name,origin}]` in successful `/api/me` and
denied-grant responses, session expiry metadata, cross-app end reasons after
logout, password/reset/grant/lock changes, and other-device revocation that
preserves the initiating session. Logout retains the already-revoked cookie
without extending its lifetime, so both app tabs can read the reason; the token
never grants API access and a subsequent login replaces it. The focused Go
regressions additionally cover expired, foreign-secret and garbage cookies and
fail-closed `VERSE_APP_LINKS` configuration.

The cutover scenario drives the real `ainovel-cli storage sso-import` against a
second, freshly migrated identity DB: colliding and clean dry-runs, the
global-admins approval gate, a `pg_dump -Fc` receipt taken as `noverse_migrate`
(PG* env, never a URL argument), apply, an idempotent resume, HTTP checks on
the imported accounts through a freshly started app pair, and a `pg_restore`
rehearsal into a throwaway DB proving the dump is the before-state. The v4
`web-accounts.json` sources are written by the same binary as the TLS proxy in
`-mkaccounts` mode, which goes through the real `platform/identity` file store
(argon2id, flock, atomic replace); per-run passwords reach it only through the
`SSO_FIXTURE_PASSWORDS` env var and the one file-mode session token it prints
on stdout is consumed by the harness and never logged.

File mode requires `--root`, so this account-only scenario supplies an empty
library snapshot and still executes the real library pass; it does not claim
library facts coverage. The optional populated-library check is deliberately
omitted to keep the fixture focused on account cutover and restore.

The resume takes a fresh custom dump after crossing a whole-second boundary:
the backup gate requires an archive creation timestamp strictly newer than
the latest account write. The original before-state dump remains untouched
for the restore rehearsal. Backup directories are mode `0700`; account backups
are asserted byte-equal and mode `0600`. Role passwords use environment
variables; password-bearing SQL is sent on stdin, never process arguments.
